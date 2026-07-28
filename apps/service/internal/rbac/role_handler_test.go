package rbac

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"

	"github.com/gin-gonic/gin"
)

type testRoleStore struct {
	roles                          []PlatformRole
	total                          int64
	role                           *PlatformRole
	permissionIDs                  []uint64
	menus                          []PlatformMenu
	menusByScope                   map[string][]PlatformMenu
	lastQuery                      PlatformRoleQuery
	lastRoleID                     uint64
	lastIncludeSuperAdmin          bool
	lastVisibleProtectedEmployeeID *uint64
	listCalls                      int
	listError                      error
	findError                      error
	permissionIDsError             error
	menusError                     error
	employeeStore                  *testEmployeeStore
}

// LoadDelegationAuthority 返回角色只读测试使用的全量授权边界。
func (store *testRoleStore) LoadDelegationAuthority(_ context.Context, _ auth.Employee, _ auth.TokenIdentity, _ managementScope, _ bool) (delegationAuthority, error) {
	return newDelegationAuthority(true), nil
}

// RolesWithinDelegationAuthority 把测试角色统一标记为处于授权范围内。
func (store *testRoleStore) RolesWithinDelegationAuthority(_ context.Context, _ managementScope, roleIDs []uint64, _ delegationAuthority) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(roleIDs))
	for _, roleID := range roleIDs {
		result[roleID] = true
	}
	return result, nil
}

// FindPlatformRole 返回测试预设的平台角色详情并记录角色 ID。
func (store *testRoleStore) FindPlatformRole(_ context.Context, roleID uint64, includeSuperAdmin bool, visibleProtectedEmployeeID *uint64) (*PlatformRole, error) {
	store.lastRoleID = roleID
	store.lastIncludeSuperAdmin = includeSuperAdmin
	store.lastVisibleProtectedEmployeeID = visibleProtectedEmployeeID
	return store.role, store.findError
}

// ListPlatformRolePermissionIDs 返回测试预设的有效角色权限 ID。
func (store *testRoleStore) ListPlatformRolePermissionIDs(_ context.Context, _ uint64) ([]uint64, error) {
	return store.permissionIDs, store.permissionIDsError
}

// ListPlatformRoleTenantPermissionIDs 返回测试预设的平台角色租户权限 ID。
func (store *testRoleStore) ListPlatformRoleTenantPermissionIDs(_ context.Context, _ uint64) ([]uint64, error) {
	return store.permissionIDs, store.permissionIDsError
}

// ListMenus 返回测试预设的角色权限树菜单节点。
func (store *testRoleStore) ListMenus(_ context.Context, scope string, _ bool) ([]PlatformMenu, error) {
	if store.menusByScope != nil {
		return store.menusByScope[scope], store.menusError
	}
	return store.menus, store.menusError
}

// ListPlatformRoles 返回测试预设的平台角色分页结果并模拟 Service 可见性处理。
func (store *testRoleStore) ListPlatformRoles(_ context.Context, actor EmployeeActor, query PlatformRoleQuery) ([]PlatformRole, int64, error) {
	query.IncludeSuperAdmin = actor.PlatformSuperAdmin
	query.VisibleProtectedEmployeeID = platformActorEmployeeID(actor)
	store.listCalls++
	store.lastQuery = query
	return store.roles, store.total, store.listError
}

// ListTenantRoles 返回测试预设的租户角色分页结果。
func (store *testRoleStore) ListTenantRoles(_ context.Context, _ EmployeeActor, _ managementScope, query PlatformRoleQuery) ([]PlatformRole, int64, error) {
	store.listCalls++
	store.lastQuery = query
	return store.roles, store.total, store.listError
}

// PlatformRoleDetail 返回测试预设的平台角色详情。
func (store *testRoleStore) PlatformRoleDetail(_ context.Context, actor EmployeeActor, roleID uint64) (PlatformRoleDetail, error) {
	role, err := store.FindPlatformRole(context.Background(), roleID, actor.PlatformSuperAdmin, platformActorEmployeeID(actor))
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	if role == nil {
		return PlatformRoleDetail{}, errManagementNotFound
	}
	platformMenus, err := store.ListMenus(context.Background(), "platform", true)
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	tenantMenus, err := store.ListMenus(context.Background(), "tenant", true)
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	platformPermissionIDs, err := store.ListPlatformRolePermissionIDs(context.Background(), roleID)
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	tenantPermissionIDs, err := store.ListPlatformRoleTenantPermissionIDs(context.Background(), roleID)
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	if role.SystemKey != nil && *role.SystemKey == platformSuperAdminKey {
		platformPermissionIDs = allMenuIDs(platformMenus, false)
		tenantPermissionIDs = allMenuIDs(tenantMenus, true)
	}
	return PlatformRoleDetail{Role: *role, PlatformPermissionIDs: platformPermissionIDs, TenantPermissionIDs: tenantPermissionIDs, PlatformMenus: platformMenus, TenantMenus: tenantMenus}, nil
}

// TenantRoleDetail 返回测试预设的租户角色详情。
func (store *testRoleStore) TenantRoleDetail(_ context.Context, _ EmployeeActor, _ managementScope, _ uint64) (TenantRoleDetail, error) {
	return TenantRoleDetail{}, nil
}

// PlatformPermissionOptions 返回测试预设的平台角色权限选项。
func (store *testRoleStore) PlatformPermissionOptions(_ context.Context, _ EmployeeActor) (PlatformPermissionOptions, error) {
	platformMenus, err := store.ListMenus(context.Background(), "platform", true)
	if err != nil {
		return PlatformPermissionOptions{}, err
	}
	tenantMenus, err := store.ListMenus(context.Background(), "tenant", true)
	if err != nil {
		return PlatformPermissionOptions{}, err
	}
	return PlatformPermissionOptions{PlatformMenus: platformMenus, TenantMenus: tenantMenus}, nil
}

// ListPlatformRoleEmployees 返回测试预设的平台角色员工列表。
func (store *testRoleStore) ListPlatformRoleEmployees(_ context.Context, actor EmployeeActor, roleID uint64, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	if _, err := store.FindPlatformRole(context.Background(), roleID, actor.PlatformSuperAdmin, platformActorEmployeeID(actor)); err != nil {
		return nil, 0, err
	}
	query.RoleID = &roleID
	query.VisibleProtectedEmployeeID = platformActorEmployeeID(actor)
	return store.employeeStore.ListPlatformEmployees(context.Background(), query)
}

// ListTenantRoleEmployees 返回测试预设的租户角色员工列表。
func (store *testRoleStore) ListTenantRoleEmployees(context.Context, EmployeeActor, managementScope, uint64, PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	return nil, 0, nil
}

// CreateRole 记录测试不覆盖的角色创建调用。
func (store *testRoleStore) CreateRole(context.Context, EmployeeActor, managementScope, RoleMutation) error {
	return nil
}

// UpdateRole 记录测试不覆盖的角色编辑调用。
func (store *testRoleStore) UpdateRole(context.Context, EmployeeActor, managementScope, uint64, RoleMutation) error {
	return nil
}

// AssignRolePermissions 记录测试不覆盖的角色授权调用。
func (store *testRoleStore) AssignRolePermissions(context.Context, EmployeeActor, managementScope, uint64, RolePermissionMutation) error {
	return nil
}

// SetRoleStatus 记录测试不覆盖的角色状态调用。
func (store *testRoleStore) SetRoleStatus(context.Context, EmployeeActor, managementScope, uint64, uint8) error {
	return nil
}

// DeleteRole 记录测试不覆盖的角色删除调用。
func (store *testRoleStore) DeleteRole(context.Context, EmployeeActor, managementScope, uint64) error {
	return nil
}

// newRoleTestRouter 注册平台角色只读 Handler 测试路由。
func newRoleTestRouter(roleStore *testRoleStore, employeeStore *testEmployeeStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set("authenticated_employee", auth.Employee{ID: 100, Scope: "platform"})
		context.Set("authenticated_token_identity", auth.TokenIdentity{EmployeeID: 100, Scope: "platform", Mode: "normal"})
		context.Next()
	})
	roleStore.employeeStore = employeeStore
	handler := NewRoleHandler(roleStore)
	router.GET("/roles", handler.ListPlatformRoles)
	router.GET("/roles/permission-options", handler.PlatformRolePermissionOptions)
	router.GET("/roles/:roleId/employees", handler.ListPlatformRoleEmployees)
	router.GET("/roles/:roleId", handler.PlatformRoleDetail)
	return router
}

// newSuperAdminRoleTestRouter 注入已认证的平台所有者上下文，验证仅本人可见的角色和员工查询条件。
func newSuperAdminRoleTestRouter(roleStore *testRoleStore, employeeStore *testEmployeeStore, employeeID uint64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set("authenticated_employee", auth.Employee{ID: employeeID, Scope: "platform"})
		context.Set("authenticated_token_identity", auth.TokenIdentity{EmployeeID: employeeID, Scope: "platform", Mode: "normal"})
		context.Set("authenticated_platform_super_admin", true)
		context.Next()
	})
	roleStore.employeeStore = employeeStore
	handler := NewRoleHandler(roleStore)
	router.GET("/roles", handler.ListPlatformRoles)
	router.GET("/roles/:roleId/employees", handler.ListPlatformRoleEmployees)
	router.GET("/roles/:roleId", handler.PlatformRoleDetail)
	return router
}

// TestPlatformRolePermissionOptionsKeepAssignableMenus 验证平台与租户权限树不再移除原保留节点。
func TestPlatformRolePermissionOptionsKeepAssignableMenus(t *testing.T) {
	permissionCode := "platform:system-log:view"
	store := &testRoleStore{menus: []PlatformMenu{{
		ID: 1044, Name: "系统日志", Type: "menu", Scope: "platform",
		PermissionCode: &permissionCode, Status: 1,
	}}}
	recorder := performRBACRequest(newRoleTestRouter(store, &testEmployeeStore{}), "/roles/permission-options")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"permissionCode":"platform:system-log:view"`) {
		t.Fatalf("角色权限选项响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestListPlatformRolesDefaultsFiltersAndResponse 验证角色默认分页、筛选与稳定响应字段。
func TestListPlatformRolesDefaultsFiltersAndResponse(t *testing.T) {
	systemKey := "platform_admin"
	description := "平台日常管理"
	store := &testRoleStore{
		roles: []PlatformRole{{
			ID: 9007199254740993, Name: "平台管理员", Description: &description,
			Type: "system", SystemKey: &systemKey, Status: 1,
			EmployeeCount: 2, PermissionCount: 28,
			CreatedAt: time.Date(2026, 7, 20, 13, 0, 0, 0, time.Local),
		}},
		total: 1,
	}
	employeeStore := &testEmployeeStore{}
	recorder := performRBACRequest(newRoleTestRouter(store, employeeStore), "/roles")
	if recorder.Code != http.StatusOK {
		t.Fatalf("角色列表响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.lastQuery.Page != 1 || store.lastQuery.PageSize != 10 {
		t.Fatalf("默认分页错误: %#v", store.lastQuery)
	}
	for _, expected := range []string{
		`"id":"9007199254740993"`, `"type":"system"`,
		`"systemKey":"platform_admin"`, `"employeeCount":2`,
		`"permissionCount":28`, `"permissionConfigurable":false`, `"pageSize":10`,
	} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("角色列表缺少 %s: %s", expected, recorder.Body.String())
		}
	}

	values := url.Values{
		"page": {"2"}, "pageSize": {"20"}, "name": {" 管理员 "},
		"type": {"custom"}, "status": {"disabled"},
	}
	recorder = performRBACRequest(newRoleTestRouter(store, employeeStore), "/roles?"+values.Encode())
	if recorder.Code != http.StatusOK {
		t.Fatalf("角色筛选响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.lastQuery.Page != 2 || store.lastQuery.PageSize != 20 || store.lastQuery.Name != "管理员" || store.lastQuery.Type != "custom" || store.lastQuery.Status == nil || *store.lastQuery.Status != 0 {
		t.Fatalf("角色筛选解析错误: %#v", store.lastQuery)
	}
}

// TestPlatformSuperAdminRoleVisibility 验证只有平台所有者请求会放行超级管理员角色及本人的关联员工。
func TestPlatformSuperAdminRoleVisibility(t *testing.T) {
	systemKey := platformSuperAdminKey
	roleStore := &testRoleStore{
		roles: []PlatformRole{{ID: 1001, Name: "平台超级管理员", Type: "system", SystemKey: &systemKey, Status: 1}},
		role:  &PlatformRole{ID: 1001, Name: "平台超级管理员", Type: "system", SystemKey: &systemKey, Status: 1},
		total: 1,
	}
	employeeStore := &testEmployeeStore{}
	router := newSuperAdminRoleTestRouter(roleStore, employeeStore, 1)
	recorder := performRBACRequest(router, "/roles")
	if recorder.Code != http.StatusOK || !roleStore.lastQuery.IncludeSuperAdmin {
		t.Fatalf("平台所有者角色列表未放行超级管理员: status=%d query=%#v", recorder.Code, roleStore.lastQuery)
	}
	if roleStore.lastQuery.VisibleProtectedEmployeeID == nil || *roleStore.lastQuery.VisibleProtectedEmployeeID != 1 {
		t.Fatalf("平台所有者员工统计未限定本人: %#v", roleStore.lastQuery)
	}

	recorder = performRBACRequest(router, "/roles/1001/employees")
	if recorder.Code != http.StatusOK || employeeStore.lastQuery.VisibleProtectedEmployeeID == nil || *employeeStore.lastQuery.VisibleProtectedEmployeeID != 1 {
		t.Fatalf("超级管理员角色员工未限定本人: status=%d query=%#v", recorder.Code, employeeStore.lastQuery)
	}

	ordinaryStore := &testRoleStore{roles: []PlatformRole{}, total: 0}
	performRBACRequest(newRoleTestRouter(ordinaryStore, &testEmployeeStore{}), "/roles")
	if ordinaryStore.lastQuery.IncludeSuperAdmin || ordinaryStore.lastQuery.VisibleProtectedEmployeeID == nil || *ordinaryStore.lastQuery.VisibleProtectedEmployeeID != 100 {
		t.Fatalf("普通请求不应放行超级管理员角色: %#v", ordinaryStore.lastQuery)
	}
}

// TestPlatformSuperAdminRoleDetailUsesDynamicPermissions 验证超级管理员详情按当前启用菜单展示动态全权限。
func TestPlatformSuperAdminRoleDetailUsesDynamicPermissions(t *testing.T) {
	systemKey := platformSuperAdminKey
	store := &testRoleStore{
		role:          &PlatformRole{ID: 1001, Name: "平台超级管理员", Type: "system", SystemKey: &systemKey, Status: 1},
		permissionIDs: []uint64{9999},
		menusByScope: map[string][]PlatformMenu{
			"platform": {{ID: 1001, Name: "平台权限", Scope: "platform", Status: 1}},
			"tenant": {
				{ID: 2001, Name: "可代管权限", Scope: "tenant", TenantAssignable: 1, Status: 1},
				{ID: 2002, Name: "不可代管权限", Scope: "tenant", TenantAssignable: 0, Status: 1},
			},
		},
	}
	recorder := performRBACRequest(newSuperAdminRoleTestRouter(store, &testEmployeeStore{}, 1), "/roles/1001")
	if recorder.Code != http.StatusOK {
		t.Fatalf("超级管理员角色详情响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	for _, expected := range []string{`"platformPermissionIds":["1001"]`, `"tenantPermissionIds":["2001"]`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("超级管理员动态权限缺少 %s: %s", expected, recorder.Body.String())
		}
	}
	if !store.lastIncludeSuperAdmin || store.lastVisibleProtectedEmployeeID == nil || *store.lastVisibleProtectedEmployeeID != 1 {
		t.Fatalf("超级管理员详情可见条件错误: include=%v employee=%v", store.lastIncludeSuperAdmin, store.lastVisibleProtectedEmployeeID)
	}
}

// TestListPlatformRolesRejectsInvalidQuery 验证角色非法分页、类型和状态统一返回 400。
func TestListPlatformRolesRejectsInvalidQuery(t *testing.T) {
	paths := []string{
		"/roles?page=0", "/roles?pageSize=101", "/roles?type=unknown",
		"/roles?type=", "/roles?status=unknown",
	}
	for _, path := range paths {
		store := &testRoleStore{}
		recorder := performRBACRequest(newRoleTestRouter(store, &testEmployeeStore{}), path)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10001`) {
			t.Fatalf("非法参数 %s 响应 = %d %s", path, recorder.Code, recorder.Body.String())
		}
		if store.listCalls != 0 {
			t.Fatalf("非法参数 %s 不应查询数据库", path)
		}
	}
}

// TestPlatformRoleDetailSuccessAndNotFound 验证角色详情字符串 ID、空数组和不存在响应。
func TestPlatformRoleDetailSuccessAndNotFound(t *testing.T) {
	systemKey := "platform_admin"
	store := &testRoleStore{
		role:          &PlatformRole{ID: 1002, Name: "平台管理员", Type: "system", SystemKey: &systemKey, Status: 1},
		permissionIDs: []uint64{1001, 9007199254740993},
		menus:         []PlatformMenu{{ID: 1001, Name: "租户管理", Type: "menu", Scope: "platform", Status: 1}},
	}
	router := newRoleTestRouter(store, &testEmployeeStore{})
	recorder := performRBACRequest(router, "/roles/1002")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"platformPermissionIds":["1001","9007199254740993"]`) || !strings.Contains(recorder.Body.String(), `"tenantPermissionIds":["1001","9007199254740993"]`) || !strings.Contains(recorder.Body.String(), `"platformMenus":[{"id":"1001"`) || !strings.Contains(recorder.Body.String(), `"tenantMenus":[{"id":"1001"`) {
		t.Fatalf("角色详情响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	store.role = nil
	recorder = performRBACRequest(router, "/roles/1001")
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"code":40001`) {
		t.Fatalf("角色不存在响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = performRBACRequest(router, "/roles/invalid")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法角色 ID 响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	store.role = &PlatformRole{ID: 1002, Name: "平台管理员", Type: "system", Status: 1}
	store.permissionIDs = []uint64{}
	store.menus = []PlatformMenu{}
	recorder = performRBACRequest(router, "/roles/1002")
	if !strings.Contains(recorder.Body.String(), `"platformPermissionIds":[]`) || !strings.Contains(recorder.Body.String(), `"tenantPermissionIds":[]`) || !strings.Contains(recorder.Body.String(), `"platformMenus":[]`) || !strings.Contains(recorder.Body.String(), `"tenantMenus":[]`) {
		t.Fatalf("角色详情空数组响应 = %s", recorder.Body.String())
	}
}

// TestListPlatformRoleEmployeesAndStoreErrors 验证角色员工分页复用角色筛选并覆盖查询错误。
func TestListPlatformRoleEmployeesAndStoreErrors(t *testing.T) {
	roleStore := &testRoleStore{role: &PlatformRole{ID: 1002, Name: "平台管理员", Type: "system", Status: 1}}
	employeeStore := &testEmployeeStore{employees: []PlatformEmployee{}, total: 0}
	router := newRoleTestRouter(roleStore, employeeStore)
	recorder := performRBACRequest(router, "/roles/1002/employees?page=2&pageSize=20")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("角色员工响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if employeeStore.lastQuery.RoleID == nil || *employeeStore.lastQuery.RoleID != 1002 || employeeStore.lastQuery.Page != 2 || employeeStore.lastQuery.PageSize != 20 {
		t.Fatalf("角色员工查询条件错误: %#v", employeeStore.lastQuery)
	}

	roleStore.findError = errors.New("database unavailable")
	recorder = performRBACRequest(router, "/roles/1002")
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":50000`) {
		t.Fatalf("角色详情错误响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	roleStore.findError = nil
	roleStore.listError = errors.New("database unavailable")
	recorder = performRBACRequest(router, "/roles")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("角色列表错误响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}
