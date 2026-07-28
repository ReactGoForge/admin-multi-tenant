package rbac

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"

	"github.com/gin-gonic/gin"
)

type testDepartmentStore struct {
	departments  []PlatformDepartment
	err          error
	lastScope    managementScope
	lastID       uint64
	lastMutation DepartmentMutation
}

// ListDepartments 返回测试预设的部门列表并记录范围。
func (store *testDepartmentStore) ListDepartments(_ context.Context, scope managementScope) ([]PlatformDepartment, error) {
	store.lastScope = scope
	return store.departments, store.err
}

// CreateDepartment 记录部门创建请求。
func (store *testDepartmentStore) CreateDepartment(_ context.Context, scope managementScope, mutation DepartmentMutation) error {
	store.lastScope = scope
	store.lastMutation = mutation
	return store.err
}

// UpdateDepartment 记录部门编辑请求。
func (store *testDepartmentStore) UpdateDepartment(_ context.Context, scope managementScope, departmentID uint64, mutation DepartmentMutation) error {
	store.lastScope = scope
	store.lastID = departmentID
	store.lastMutation = mutation
	return store.err
}

// DeleteDepartment 记录部门删除请求。
func (store *testDepartmentStore) DeleteDepartment(_ context.Context, scope managementScope, departmentID uint64) error {
	store.lastScope = scope
	store.lastID = departmentID
	return store.err
}

// newDepartmentTestRouter 注册部门 Handler 测试路由。
func newDepartmentTestRouter(store *testDepartmentStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDepartmentHandler(store)
	router.GET("/departments", handler.ListPlatformDepartments)
	router.GET("/tenant/departments", tenantDepartmentContextMiddleware(), handler.ListTenantDepartments)
	router.POST("/departments", handler.CreatePlatformDepartment)
	router.PATCH("/departments/:departmentId", handler.UpdatePlatformDepartment)
	router.DELETE("/departments/:departmentId", handler.DeletePlatformDepartment)
	return router
}

// tenantDepartmentContextMiddleware 写入租户部门测试所需的认证上下文。
func tenantDepartmentContextMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		tenantID := uint64(88)
		context.Set("authenticated_employee", auth.Employee{ID: 9, Scope: "tenant", TenantID: &tenantID})
		context.Set("authenticated_token_identity", auth.TokenIdentity{EmployeeID: 9, Scope: "tenant", TenantID: &tenantID, Mode: "normal"})
		context.Next()
	}
}

// performDepartmentRequest 执行带请求体的部门测试请求。
func performDepartmentRequest(router http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestListPlatformDepartmentsResponseAndEmpty 验证部门字符串 ID、负责人、统计和空数组响应。
func TestListPlatformDepartmentsResponseAndEmpty(t *testing.T) {
	parentID := uint64(9007199254740993)
	leaderID := uint64(9007199254740995)
	leaderName := "平台员工"
	store := &testDepartmentStore{departments: []PlatformDepartment{{
		ID: 9007199254740997, ParentID: &parentID, Name: "平台运营部",
		LeaderEmployeeID: &leaderID, LeaderName: &leaderName,
		EmployeeCount: 3, Sort: 10, Status: 1,
	}}}
	router := newDepartmentTestRouter(store)
	recorder := performRBACRequest(router, "/departments")
	if recorder.Code != http.StatusOK {
		t.Fatalf("部门列表响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.lastScope.Name != "platform" || store.lastScope.TenantID != nil {
		t.Fatalf("平台部门范围 = %#v", store.lastScope)
	}
	for _, expected := range []string{
		`"id":"9007199254740997"`, `"parentId":"9007199254740993"`,
		`"leader":{"id":"9007199254740995","name":"平台员工"}`,
		`"employeeCount":3`, `"status":"enabled"`,
	} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("部门响应缺少 %s: %s", expected, recorder.Body.String())
		}
	}

	store.departments = []PlatformDepartment{}
	recorder = performRBACRequest(router, "/departments")
	if !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("部门空数组响应 = %s", recorder.Body.String())
	}
}

// TestListPlatformDepartmentsNullLeaderAndError 验证无效负责人返回 null，查询失败返回 500。
func TestListPlatformDepartmentsNullLeaderAndError(t *testing.T) {
	leaderID := uint64(8)
	store := &testDepartmentStore{departments: []PlatformDepartment{{
		ID: 1, Name: "无负责人部门", LeaderEmployeeID: &leaderID, Status: 0,
	}}}
	router := newDepartmentTestRouter(store)
	recorder := performRBACRequest(router, "/departments")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"leader":null`) || !strings.Contains(recorder.Body.String(), `"status":"disabled"`) {
		t.Fatalf("部门空负责人响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	store.err = errors.New("database unavailable")
	recorder = performRBACRequest(router, "/departments")
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":50000`) {
		t.Fatalf("部门查询失败响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestListTenantDepartmentsUsesAuthenticatedTenantScope 验证租户部门列表只使用认证上下文中的租户。
func TestListTenantDepartmentsUsesAuthenticatedTenantScope(t *testing.T) {
	store := &testDepartmentStore{}
	recorder := performRBACRequest(newDepartmentTestRouter(store), "/tenant/departments")
	if recorder.Code != http.StatusOK {
		t.Fatalf("租户部门列表响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.lastScope.Name != "tenant" || store.lastScope.TenantID == nil || *store.lastScope.TenantID != 88 {
		t.Fatalf("租户部门范围 = %#v", store.lastScope)
	}
}

// TestCreatePlatformDepartmentValidationAndMutation 验证创建请求校验和 DTO 转换。
func TestCreatePlatformDepartmentValidationAndMutation(t *testing.T) {
	store := &testDepartmentStore{}
	router := newDepartmentTestRouter(store)
	body := `{"parentId":"11","name":" 平台运营部 ","leaderEmployeeId":"12","sort":9}`
	recorder := performDepartmentRequest(router, http.MethodPost, "/departments", body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("创建部门响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.lastMutation.ParentID == nil || *store.lastMutation.ParentID != 11 || store.lastMutation.LeaderEmployeeID == nil || *store.lastMutation.LeaderEmployeeID != 12 {
		t.Fatalf("创建部门 ID 转换 = %#v", store.lastMutation)
	}
	if store.lastMutation.Name != "平台运营部" || store.lastMutation.Status != 1 {
		t.Fatalf("创建部门字段 = %#v", store.lastMutation)
	}

	recorder = performDepartmentRequest(router, http.MethodPost, "/departments", `{"name":"","status":"enabled"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法部门创建响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestUpdateAndDeletePlatformDepartmentErrors 验证编辑、删除路径参数和业务错误映射。
func TestUpdateAndDeletePlatformDepartmentErrors(t *testing.T) {
	store := &testDepartmentStore{err: errManagementNotFound}
	router := newDepartmentTestRouter(store)
	body := `{"name":"平台运营部","status":"enabled"}`
	recorder := performDepartmentRequest(router, http.MethodPatch, "/departments/42", body)
	if recorder.Code != http.StatusNotFound || store.lastID != 42 {
		t.Fatalf("编辑不存在部门响应 = %d %s id=%d", recorder.Code, recorder.Body.String(), store.lastID)
	}

	store.err = errManagementConflict
	recorder = performDepartmentRequest(router, http.MethodDelete, "/departments/42", "")
	if recorder.Code != http.StatusConflict || store.lastID != 42 {
		t.Fatalf("删除冲突部门响应 = %d %s id=%d", recorder.Code, recorder.Body.String(), store.lastID)
	}
}
