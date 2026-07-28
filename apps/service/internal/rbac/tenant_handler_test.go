package rbac

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type testTenantApp struct {
	tenants      []PlatformTenant
	total        int64
	err          error
	lastQuery    TenantQuery
	lastTenantID uint64
	lastCreate   TenantCreateInput
	lastUpdate   TenantUpdateInput
	lastStatus   uint8
}

// ListTenants 返回测试预设的租户列表并记录查询条件。
func (app *testTenantApp) ListTenants(_ context.Context, query TenantQuery) ([]PlatformTenant, int64, error) {
	app.lastQuery = query
	return app.tenants, app.total, app.err
}

// CreateTenant 记录租户创建输入。
func (app *testTenantApp) CreateTenant(_ context.Context, input TenantCreateInput) error {
	app.lastCreate = input
	return app.err
}

// UpdateTenant 记录租户编辑输入。
func (app *testTenantApp) UpdateTenant(_ context.Context, tenantID uint64, input TenantUpdateInput) error {
	app.lastTenantID = tenantID
	app.lastUpdate = input
	return app.err
}

// ResetTenantOwnerPassword 记录租户所有者密码重置目标。
func (app *testTenantApp) ResetTenantOwnerPassword(_ context.Context, tenantID uint64, _ string) error {
	app.lastTenantID = tenantID
	return app.err
}

// SetTenantStatus 记录租户状态更新目标。
func (app *testTenantApp) SetTenantStatus(_ context.Context, tenantID uint64, status uint8) error {
	app.lastTenantID = tenantID
	app.lastStatus = status
	return app.err
}

// DeleteTenant 记录租户删除目标。
func (app *testTenantApp) DeleteTenant(_ context.Context, tenantID uint64) error {
	app.lastTenantID = tenantID
	return app.err
}

// newTenantTestRouter 注册租户 Handler 测试路由。
func newTenantTestRouter(app *testTenantApp) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewTenantHandler(app)
	router.GET("/tenants", handler.ListTenants)
	router.POST("/tenants", handler.CreateTenant)
	router.PATCH("/tenants/:tenantId", handler.UpdateTenant)
	router.PUT("/tenants/:tenantId/owner-password", handler.ResetTenantOwnerPassword)
	router.PATCH("/tenants/:tenantId/status", handler.SetTenantStatus)
	router.DELETE("/tenants/:tenantId", handler.DeleteTenant)
	return router
}

// performTenantRequest 执行租户 Handler 测试请求。
func performTenantRequest(router http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestListTenantsResponseAndFilters 验证租户列表分页筛选、字符串 ID 和空数组兼容。
func TestListTenantsResponseAndFilters(t *testing.T) {
	ownerID := uint64(9007199254740993)
	ownerName := "企业管理员"
	account := "tenant-owner"
	remark := "内部备注"
	app := &testTenantApp{
		tenants: []PlatformTenant{{
			ID: 9007199254740995, Name: "测试租户", Remark: &remark, Status: 1,
			OwnerEmployeeID: &ownerID, OwnerName: &ownerName, LoginAccount: &account,
		}},
		total: 1,
	}
	router := newTenantTestRouter(app)
	recorder := performTenantRequest(router, http.MethodGet, "/tenants?page=2&pageSize=20&name=%20租户%20&status=enabled", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("租户列表响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if app.lastQuery.Page != 2 || app.lastQuery.PageSize != 20 || app.lastQuery.Name != "租户" || app.lastQuery.Status == nil || *app.lastQuery.Status != 1 {
		t.Fatalf("租户筛选解析错误: %#v", app.lastQuery)
	}
	for _, expected := range []string{`"id":"9007199254740995"`, `"ownerEmployeeId":"9007199254740993"`, `"ownerName":"企业管理员"`, `"status":"enabled"`, `"pageSize":20`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("租户响应缺少 %s: %s", expected, recorder.Body.String())
		}
	}

	app.tenants = []PlatformTenant{}
	recorder = performTenantRequest(router, http.MethodGet, "/tenants", "")
	if !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("租户空数组响应 = %s", recorder.Body.String())
	}
}

// TestTenantMutationsValidateAndMapErrors 验证租户写接口 DTO 转换和业务错误映射。
func TestTenantMutationsValidateAndMapErrors(t *testing.T) {
	app := &testTenantApp{}
	router := newTenantTestRouter(app)
	recorder := performTenantRequest(router, http.MethodPost, "/tenants", `{"name":" 测试租户 ","ownerName":" 管理员 ","loginAccount":" owner ","password":"123456"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("创建租户响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if app.lastCreate.Name != "测试租户" || app.lastCreate.OwnerName != "管理员" || app.lastCreate.LoginAccount != "owner" {
		t.Fatalf("创建租户字段未清理: %#v", app.lastCreate)
	}

	remark := " 备注 "
	recorder = performTenantRequest(router, http.MethodPatch, "/tenants/42", `{"name":"租户","loginAccount":"owner","remark":"`+remark+`"}`)
	if recorder.Code != http.StatusOK || app.lastTenantID != 42 || app.lastUpdate.Remark == nil || *app.lastUpdate.Remark != "备注" {
		t.Fatalf("编辑租户响应或字段错误 = %d %s %#v", recorder.Code, recorder.Body.String(), app.lastUpdate)
	}

	app.err = errManagementConflict
	recorder = performTenantRequest(router, http.MethodDelete, "/tenants/42", "")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("删除冲突响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	app.err = errors.New("database unavailable")
	recorder = performTenantRequest(router, http.MethodGet, "/tenants", "")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("租户查询失败响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestTenantRequestValidation 验证非法路径、状态和密码不会进入业务服务。
func TestTenantRequestValidation(t *testing.T) {
	app := &testTenantApp{}
	router := newTenantTestRouter(app)
	paths := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/tenants?page=0", ""},
		{http.MethodPost, "/tenants", `{"name":"租户","ownerName":"管理员","loginAccount":"owner","password":"12345"}`},
		{http.MethodPut, "/tenants/abc/owner-password", `{"password":"123456"}`},
		{http.MethodPatch, "/tenants/42/status", `{"status":"unknown"}`},
	}
	for _, item := range paths {
		recorder := performTenantRequest(router, item.method, item.path, item.body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("非法请求 %s %s 响应 = %d %s", item.method, item.path, recorder.Code, recorder.Body.String())
		}
	}
}
