package rbac

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"

	"github.com/gin-gonic/gin"
)

type testEmployeeStore struct {
	employees    []PlatformEmployee
	total        int64
	options      PlatformEmployeeOptions
	lastQuery    PlatformEmployeeQuery
	listCalls    int
	listError    error
	optionsError error
}

// LoadDelegationAuthority 返回测试使用的全量向下授权边界。
func (store *testEmployeeStore) LoadDelegationAuthority(_ context.Context, _ auth.Employee, _ auth.TokenIdentity, _ managementScope, _ bool) (delegationAuthority, error) {
	return newDelegationAuthority(true), nil
}

// RolesWithinDelegationAuthority 把测试角色统一标记为可分配。
func (store *testEmployeeStore) RolesWithinDelegationAuthority(_ context.Context, _ managementScope, roleIDs []uint64, _ delegationAuthority) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(roleIDs))
	for _, roleID := range roleIDs {
		result[roleID] = true
	}
	return result, nil
}

// ListPlatformEmployees 返回测试预设的平台员工分页结果并记录查询条件。
func (store *testEmployeeStore) ListPlatformEmployees(_ context.Context, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	store.listCalls++
	store.lastQuery = query
	return store.employees, store.total, store.listError
}

// ListPlatformEmployeeOptions 返回测试预设的平台角色和部门选项。
func (store *testEmployeeStore) ListPlatformEmployeeOptions(_ context.Context) (PlatformEmployeeOptions, error) {
	return store.options, store.optionsError
}

// ListEmployees 返回测试预设的员工分页结果并模拟 Service 写入平台本人可见 ID。
func (store *testEmployeeStore) ListEmployees(_ context.Context, actor EmployeeActor, scope managementScope, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	if scope.Name == "platform" {
		query.VisibleProtectedEmployeeID = platformActorEmployeeID(actor)
	}
	store.listCalls++
	store.lastQuery = query
	return store.employees, store.total, store.listError
}

// ListEmployeeOptions 返回测试预设的角色和部门选项。
func (store *testEmployeeStore) ListEmployeeOptions(_ context.Context, _ EmployeeActor, _ managementScope) (PlatformEmployeeOptions, error) {
	for index := range store.options.Roles {
		store.options.Roles[index].Assignable = true
	}
	return store.options, store.optionsError
}

// CreateEmployee 是 Handler 测试未覆盖写流程的占位实现。
func (store *testEmployeeStore) CreateEmployee(context.Context, EmployeeActor, managementScope, EmployeeMutation) error {
	return nil
}

// UpdateEmployee 是 Handler 测试未覆盖写流程的占位实现。
func (store *testEmployeeStore) UpdateEmployee(context.Context, EmployeeActor, managementScope, uint64, EmployeeMutation) error {
	return nil
}

// AssignEmployeeRoles 是 Handler 测试未覆盖写流程的占位实现。
func (store *testEmployeeStore) AssignEmployeeRoles(context.Context, EmployeeActor, managementScope, uint64, []uint64) error {
	return nil
}

// ResetEmployeePassword 是 Handler 测试未覆盖写流程的占位实现。
func (store *testEmployeeStore) ResetEmployeePassword(context.Context, EmployeeActor, managementScope, uint64, string) error {
	return nil
}

// SetEmployeeStatus 是 Handler 测试未覆盖写流程的占位实现。
func (store *testEmployeeStore) SetEmployeeStatus(context.Context, EmployeeActor, managementScope, uint64, uint8) error {
	return nil
}

// DeleteEmployee 是 Handler 测试未覆盖写流程的占位实现。
func (store *testEmployeeStore) DeleteEmployee(context.Context, EmployeeActor, managementScope, uint64) error {
	return nil
}

// newRBACTestRouter 注册平台员工只读接口，供 Gin httptest 验证 Handler。
func newRBACTestRouter(store *testEmployeeStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set("authenticated_employee", auth.Employee{ID: 100, Scope: "platform"})
		context.Set("authenticated_token_identity", auth.TokenIdentity{EmployeeID: 100, Scope: "platform", Mode: "normal"})
		context.Next()
	})
	handler := NewHandler(store)
	router.GET("/employees", handler.ListPlatformEmployees)
	router.GET("/employees/options", handler.PlatformEmployeeOptions)
	return router
}

// newAuthenticatedRBACTestRouter 注入已认证的平台员工上下文，用于验证受保护员工仅向本人放行。
func newAuthenticatedRBACTestRouter(store *testEmployeeStore, employeeID uint64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set("authenticated_employee", auth.Employee{ID: employeeID, Scope: "platform"})
		context.Set("authenticated_token_identity", auth.TokenIdentity{EmployeeID: employeeID, Scope: "platform", Mode: "normal"})
		context.Next()
	})
	handler := NewHandler(store)
	router.GET("/employees", handler.ListPlatformEmployees)
	return router
}

// performRBACRequest 执行平台 RBAC 测试请求并返回响应记录器。
func performRBACRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestListPlatformEmployeesDefaultsAndResponse 验证默认分页、字符串 ID、空角色数组和员工响应字段。
func TestListPlatformEmployeesDefaultsAndResponse(t *testing.T) {
	departmentID := uint64(9007199254740993)
	departmentName := "平台运营部"
	store := &testEmployeeStore{
		employees: []PlatformEmployee{{
			ID:             9007199254740995,
			Name:           "平台员工",
			LoginAccount:   "platform_user",
			DepartmentID:   &departmentID,
			DepartmentName: &departmentName,
			Roles:          []EmployeeRole{},
			Status:         1,
			CreatedAt:      time.Date(2026, 7, 20, 12, 30, 0, 0, time.Local),
		}},
		total: 1,
	}
	recorder := performRBACRequest(newRBACTestRouter(store), "/employees")

	if recorder.Code != http.StatusOK {
		t.Fatalf("员工列表响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.lastQuery.Page != 1 || store.lastQuery.PageSize != 10 {
		t.Fatalf("默认分页 = page:%d pageSize:%d", store.lastQuery.Page, store.lastQuery.PageSize)
	}
	for _, expected := range []string{
		`"id":"9007199254740995"`,
		`"id":"9007199254740993"`,
		`"roles":[]`,
		`"phone":null`,
		`"status":"enabled"`,
		`"pageSize":10`,
	} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("员工列表缺少 %s: %s", expected, recorder.Body.String())
		}
	}
}

// TestListPlatformEmployeesPassesCurrentEmployeeVisibility 验证平台员工列表仅把当前登录员工 ID 作为受保护记录放行条件。
func TestListPlatformEmployeesPassesCurrentEmployeeVisibility(t *testing.T) {
	store := &testEmployeeStore{}
	recorder := performRBACRequest(newAuthenticatedRBACTestRouter(store, 1001), "/employees")
	if recorder.Code != http.StatusOK {
		t.Fatalf("平台所有者员工列表响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.lastQuery.VisibleProtectedEmployeeID == nil || *store.lastQuery.VisibleProtectedEmployeeID != 1001 {
		t.Fatalf("本人可见员工 ID 未传入查询: %#v", store.lastQuery)
	}

	ordinaryStore := &testEmployeeStore{}
	performRBACRequest(newRBACTestRouter(ordinaryStore), "/employees")
	if ordinaryStore.lastQuery.VisibleProtectedEmployeeID == nil || *ordinaryStore.lastQuery.VisibleProtectedEmployeeID != 100 {
		t.Fatalf("普通认证身份应只放行本人受保护记录: %#v", ordinaryStore.lastQuery)
	}
}

// TestListPlatformEmployeesFilters 验证员工分页和全部筛选参数会被正确解析。
func TestListPlatformEmployeesFilters(t *testing.T) {
	values := url.Values{
		"page":         {"2"},
		"pageSize":     {"20"},
		"name":         {" 平台员工 "},
		"loginAccount": {" account "},
		"departmentId": {"11"},
		"roleId":       {"22"},
		"status":       {"disabled"},
	}
	store := &testEmployeeStore{}
	recorder := performRBACRequest(newRBACTestRouter(store), "/employees?"+values.Encode())

	if recorder.Code != http.StatusOK {
		t.Fatalf("筛选响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	query := store.lastQuery
	if query.Page != 2 || query.PageSize != 20 || query.Name != "平台员工" || query.LoginAccount != "account" {
		t.Fatalf("文本或分页筛选解析错误: %#v", query)
	}
	if query.DepartmentID == nil || *query.DepartmentID != 11 || query.RoleID == nil || *query.RoleID != 22 {
		t.Fatalf("ID 筛选解析错误: %#v", query)
	}
	if query.Status == nil || *query.Status != 0 {
		t.Fatalf("状态筛选解析错误: %#v", query.Status)
	}
}

// TestListPlatformEmployeesRejectsInvalidQuery 验证非法分页、筛选 ID 和状态统一返回 400。
func TestListPlatformEmployeesRejectsInvalidQuery(t *testing.T) {
	paths := []string{
		"/employees?page=0",
		"/employees?page=",
		"/employees?page=" + strconv.Itoa(int(^uint(0)>>1)) + "&pageSize=100",
		"/employees?pageSize=101",
		"/employees?departmentId=0",
		"/employees?roleId=-1",
		"/employees?status=unknown",
	}
	for _, path := range paths {
		store := &testEmployeeStore{}
		recorder := performRBACRequest(newRBACTestRouter(store), path)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10001`) {
			t.Fatalf("非法参数 %s 响应 = %d %s", path, recorder.Code, recorder.Body.String())
		}
		if store.listCalls != 0 {
			t.Fatalf("非法参数 %s 不应查询数据库，调用次数 = %d", path, store.listCalls)
		}
	}
}

// TestPlatformEmployeeOptionsAndStoreErrors 验证筛选选项字符串 ID、空数组以及数据库错误响应。
func TestPlatformEmployeeOptionsAndStoreErrors(t *testing.T) {
	store := &testEmployeeStore{
		options: PlatformEmployeeOptions{
			Roles:       []EmployeeOption{{ID: 9007199254740993, Name: "平台管理员", Status: 1}},
			Departments: []EmployeeOption{},
		},
	}
	router := newRBACTestRouter(store)
	optionsRecorder := performRBACRequest(router, "/employees/options")
	if optionsRecorder.Code != http.StatusOK || !strings.Contains(optionsRecorder.Body.String(), `"id":"9007199254740993"`) || !strings.Contains(optionsRecorder.Body.String(), `"assignable":true`) || !strings.Contains(optionsRecorder.Body.String(), `"departments":[]`) {
		t.Fatalf("筛选选项响应 = %d %s", optionsRecorder.Code, optionsRecorder.Body.String())
	}

	store.listError = errors.New("database unavailable")
	listRecorder := performRBACRequest(router, "/employees")
	if listRecorder.Code != http.StatusInternalServerError || !strings.Contains(listRecorder.Body.String(), `"code":50000`) {
		t.Fatalf("列表查询错误响应 = %d %s", listRecorder.Code, listRecorder.Body.String())
	}
	store.optionsError = errors.New("database unavailable")
	optionsErrorRecorder := performRBACRequest(router, "/employees/options")
	if optionsErrorRecorder.Code != http.StatusInternalServerError || !strings.Contains(optionsErrorRecorder.Body.String(), `"code":50000`) {
		t.Fatalf("选项查询错误响应 = %d %s", optionsErrorRecorder.Code, optionsErrorRecorder.Body.String())
	}
}
