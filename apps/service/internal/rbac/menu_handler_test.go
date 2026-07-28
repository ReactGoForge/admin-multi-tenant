package rbac

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type testMenuStore struct {
	menus        []PlatformMenu
	lastScope    string
	calls        int
	err          error
	lastMutation MenuMutation
	lastMenuID   uint64
	lastStatus   uint8
}

// CreateMenu 记录测试菜单创建请求。
func (store *testMenuStore) CreateMenu(_ context.Context, mutation MenuMutation) error {
	store.lastMutation = mutation
	return store.err
}

// UpdateMenu 记录测试菜单编辑请求。
func (store *testMenuStore) UpdateMenu(_ context.Context, menuID uint64, mutation MenuMutation) error {
	store.lastMenuID = menuID
	store.lastMutation = mutation
	return store.err
}

// SetMenuStatus 记录测试菜单状态请求。
func (store *testMenuStore) SetMenuStatus(_ context.Context, menuID uint64, status uint8) error {
	store.lastMenuID = menuID
	store.lastStatus = status
	return store.err
}

// DeleteMenu 记录测试菜单删除请求。
func (store *testMenuStore) DeleteMenu(_ context.Context, menuID uint64) error {
	store.lastMenuID = menuID
	return store.err
}

// ListMenus 返回测试预设的菜单节点并记录范围。
func (store *testMenuStore) ListMenus(_ context.Context, scope string) ([]PlatformMenu, error) {
	store.calls++
	store.lastScope = scope
	return store.menus, store.err
}

// performMenuMutationRequest 执行菜单写接口测试请求。
func performMenuMutationRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// newMenuTestRouter 注册平台菜单只读 Handler 测试路由。
func newMenuTestRouter(store *testMenuStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewMenuHandler(store)
	router.GET("/menus", handler.ListPlatformMenus)
	router.POST("/menus", handler.CreatePlatformMenu)
	router.PATCH("/menus/:menuId", handler.UpdatePlatformMenu)
	router.PATCH("/menus/:menuId/status", handler.SetPlatformMenuStatus)
	router.DELETE("/menus/:menuId", handler.DeletePlatformMenu)
	return router
}

// TestListPlatformMenusScopesAndResponse 验证双范围菜单查询、字符串 ID、空值和布尔字段响应。
func TestListPlatformMenusScopesAndResponse(t *testing.T) {
	parentID := uint64(9007199254740993)
	path := "/platform/system/roles"
	store := &testMenuStore{menus: []PlatformMenu{{
		ID: 9007199254740995, ParentID: &parentID, Name: "角色管理",
		Type: "menu", Scope: "platform", Path: &path,
		TenantAssignable: 0, Sort: 20, Visible: 1, Status: 0,
	}}}
	router := newMenuTestRouter(store)
	for _, scope := range []string{"platform", "tenant"} {
		recorder := performRBACRequest(router, "/menus?scope="+scope)
		if recorder.Code != http.StatusOK {
			t.Fatalf("菜单范围 %s 响应 = %d %s", scope, recorder.Code, recorder.Body.String())
		}
		if store.lastScope != scope {
			t.Fatalf("菜单查询范围 = %s", store.lastScope)
		}
	}
	for _, expected := range []string{
		`"id":"9007199254740995"`, `"parentId":"9007199254740993"`,
		`"component":null`, `"tenantAssignable":false`, `"visible":true`,
		`"status":"disabled"`,
	} {
		if recorder := performRBACRequest(router, "/menus?scope=platform"); !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("菜单响应缺少 %s: %s", expected, recorder.Body.String())
		}
	}
}

// TestListPlatformMenusRejectsInvalidScopeAndErrors 验证缺少或非法范围返回 400，查询失败返回 500。
func TestListPlatformMenusRejectsInvalidScopeAndErrors(t *testing.T) {
	for _, path := range []string{"/menus", "/menus?scope=", "/menus?scope=unknown"} {
		store := &testMenuStore{}
		recorder := performRBACRequest(newMenuTestRouter(store), path)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10001`) {
			t.Fatalf("非法菜单范围 %s 响应 = %d %s", path, recorder.Code, recorder.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("非法菜单范围 %s 不应查询数据库", path)
		}
	}
	store := &testMenuStore{err: errors.New("database unavailable")}
	recorder := performRBACRequest(newMenuTestRouter(store), "/menus?scope=platform")
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":50000`) {
		t.Fatalf("菜单查询失败响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestCreatePlatformMenuValidatesStaticPages 验证菜单创建只接受已注册静态页面。
func TestCreatePlatformMenuValidatesStaticPages(t *testing.T) {
	validBody := `{"scope":"platform","parentId":"1002","name":"角色管理","type":"menu","path":"/platform/system/roles","component":"pages/platform/system/roles/index.tsx","permissionCode":"platform:role:view","tenantAssignable":false,"sort":20,"visible":true,"status":"enabled"}`
	store := &testMenuStore{}
	recorder := performMenuMutationRequest(newMenuTestRouter(store), http.MethodPost, "/menus", validBody)
	if recorder.Code != http.StatusCreated || store.lastMutation.Path == nil || *store.lastMutation.Path != "/platform/system/roles" {
		t.Fatalf("合法静态页面响应 = %d %s, mutation=%#v", recorder.Code, recorder.Body.String(), store.lastMutation)
	}

	homeBody := `{"scope":"platform","name":"首页","type":"menu","path":"/platform","component":"router/modules/platform-index.tsx","permissionCode":"platform:home:view","tenantAssignable":false,"sort":0,"visible":true,"status":"enabled"}`
	store = &testMenuStore{}
	recorder = performMenuMutationRequest(newMenuTestRouter(store), http.MethodPost, "/menus", homeBody)
	if recorder.Code != http.StatusCreated || store.lastMutation.Path == nil || *store.lastMutation.Path != "/platform" {
		t.Fatalf("平台首页静态页面响应 = %d %s, mutation=%#v", recorder.Code, recorder.Body.String(), store.lastMutation)
	}

	invalidBody := strings.Replace(validBody, "/platform/system/roles", "/platform/unknown", 1)
	store = &testMenuStore{err: errManagementInvalid}
	recorder = performMenuMutationRequest(newMenuTestRouter(store), http.MethodPost, "/menus", invalidBody)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法静态页面响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestTenantMenuPermissionIsAssignable 验证租户菜单查看权限可标记为租户角色可分配。
func TestTenantMenuPermissionIsAssignable(t *testing.T) {
	body := `{"scope":"tenant","name":"首页","type":"menu","path":"/tenant","component":"router/modules/tenant-index.tsx","permissionCode":"tenant:home:view","tenantAssignable":true,"sort":0,"visible":true,"status":"enabled"}`
	store := &testMenuStore{}
	recorder := performMenuMutationRequest(newMenuTestRouter(store), http.MethodPost, "/menus", body)
	if recorder.Code != http.StatusCreated || store.lastMutation.TenantAssignable != 1 || store.lastMutation.Path == nil || *store.lastMutation.Path != "/tenant" {
		t.Fatalf("租户菜单权限响应 = %d %s, mutation=%#v", recorder.Code, recorder.Body.String(), store.lastMutation)
	}
}

// TestMenuMutationEndpointsValidateIDsAndStatus 验证菜单编辑、状态和删除接口的基础参数。
func TestMenuMutationEndpointsValidateIDsAndStatus(t *testing.T) {
	directoryBody := `{"scope":"tenant","name":"业务中心","type":"directory","tenantAssignable":true,"sort":10,"visible":true,"status":"enabled"}`
	store := &testMenuStore{}
	router := newMenuTestRouter(store)
	recorder := performMenuMutationRequest(router, http.MethodPatch, "/menus/9", directoryBody)
	if recorder.Code != http.StatusOK || store.lastMenuID != 9 || store.lastMutation.Scope != "tenant" || !strings.Contains(recorder.Body.String(), `"data":null`) {
		t.Fatalf("菜单编辑响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = performMenuMutationRequest(router, http.MethodPatch, "/menus/9/status", `{"status":"disabled"}`)
	if recorder.Code != http.StatusOK || store.lastStatus != 0 || !strings.Contains(recorder.Body.String(), `"data":null`) {
		t.Fatalf("菜单状态响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = performMenuMutationRequest(router, http.MethodDelete, "/menus/invalid", "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法菜单 ID 响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}
