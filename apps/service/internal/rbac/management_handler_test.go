package rbac

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

// TestNormalizeEmployeeRequest 验证员工写入字段和密码边界。
func TestNormalizeEmployeeRequest(t *testing.T) {
	request := employeeMutationRequest{Name: " 测试员工 ", LoginAccount: " staff ", Password: "123456", Status: "enabled"}
	if !normalizeEmployeeRequest(&request, true) {
		t.Fatal("合法员工请求应通过校验")
	}
	if request.Name != "测试员工" || request.LoginAccount != "staff" {
		t.Fatalf("文本字段未正确清理: %#v", request)
	}
	request.Password = "12345"
	if normalizeEmployeeRequest(&request, true) {
		t.Fatal("少于 6 位的初始密码不应通过校验")
	}
}

// TestNormalizeTenantSettings 验证租户登录账号和备注的清理和限制。
func TestNormalizeTenantSettings(t *testing.T) {
	remark := "  内部备注  "
	update := tenantUpdateRequest{Name: " 测试租户 ", LoginAccount: " tenant-owner ", Remark: &remark}
	if !normalizeTenantUpdateRequest(&update) || update.Name != "测试租户" || update.LoginAccount != "tenant-owner" || update.Remark == nil || *update.Remark != "内部备注" {
		t.Fatalf("租户编辑字段清理失败: %#v", update)
	}
	if normalizeTenantUpdateRequest(&tenantUpdateRequest{Name: "测试租户"}) {
		t.Fatal("空登录账号不应通过校验")
	}
}

// TestParseIDs 验证 BIGINT 字符串数组去重并拒绝非法值。
func TestParseIDs(t *testing.T) {
	empty, valid := parseIDs([]string{})
	if !valid || len(empty) != 0 {
		t.Fatalf("空权限数组应允许清空角色权限: %#v, %v", empty, valid)
	}
	values, valid := parseIDs([]string{"2", "1", "2"})
	if !valid || len(values) != 2 || values[0] != 2 || values[1] != 1 {
		t.Fatalf("ID 解析结果不正确: %#v, %v", values, valid)
	}
	if _, valid = parseIDs([]string{"0"}); valid {
		t.Fatal("零 ID 不应通过校验")
	}
}

// TestValidateRolePermissionAssignment 验证受支持的内置角色只允许平台超级管理员配置。
func TestValidateRolePermissionAssignment(t *testing.T) {
	platformScope := managementScope{Name: "platform"}
	tenantScope := managementScope{Name: "tenant"}
	platformAdmin := "platform_admin"
	tenantOwner := tenantOwnerSystemKey
	if err := validateRolePermissionAssignment(platformScope, nil, false); err != nil {
		t.Fatalf("自定义角色应允许原有授权者配置: %v", err)
	}
	if err := validateRolePermissionAssignment(platformScope, &platformAdmin, true); err != nil {
		t.Fatalf("平台超级管理员应能配置内置平台管理员: %v", err)
	}
	if !errors.Is(validateRolePermissionAssignment(platformScope, &platformAdmin, false), errManagementForbidden) {
		t.Fatal("普通平台授权者配置内置平台管理员应返回无权限")
	}
	if err := validateRolePermissionAssignment(tenantScope, &tenantOwner, true); err != nil {
		t.Fatalf("平台超级管理员应能配置企业管理员权限: %v", err)
	}
	if !errors.Is(validateRolePermissionAssignment(tenantScope, &tenantOwner, false), errManagementForbidden) {
		t.Fatal("非平台超级管理员配置企业管理员应返回无权限")
	}
	if !canAssignTenantMenuPermissions(tenantScope, &tenantOwner, true) {
		t.Fatal("平台超级管理员配置企业管理员时应允许租户菜单管理权限")
	}
	if canAssignTenantMenuPermissions(tenantScope, nil, true) || canAssignTenantMenuPermissions(platformScope, &platformAdmin, true) || canAssignTenantMenuPermissions(tenantScope, &tenantOwner, false) {
		t.Fatal("租户自定义角色、平台角色和普通租户身份均不应允许租户菜单管理权限")
	}
}

// TestDefaultTenantOwnerPermissionExcludesMenuManagement 验证企业管理员默认权限排除菜单管理分组。
func TestDefaultTenantOwnerPermissionExcludesMenuManagement(t *testing.T) {
	menuPermission := "tenant:menu:view"
	futureMenuPermission := "tenant:menu:edit"
	employeePermission := "tenant:employee:view"
	if !isTenantMenuPermission(&menuPermission) || !isTenantMenuPermission(&futureMenuPermission) {
		t.Fatal("tenant:menu:* 应统一识别为租户菜单管理权限")
	}
	if isDefaultTenantOwnerPermission(&menuPermission) || isDefaultTenantOwnerPermission(&futureMenuPermission) {
		t.Fatal("菜单管理权限不应默认授予企业管理员")
	}
	if isTenantMenuPermission(&employeePermission) || isTenantMenuPermission(nil) || !isDefaultTenantOwnerPermission(&employeePermission) || !isDefaultTenantOwnerPermission(nil) {
		t.Fatal("普通租户权限和目录节点应默认授予企业管理员")
	}
}

// TestPermissionsWithinAuthority 验证多角色权限并集只允许继续授予已有权限子集。
func TestPermissionsWithinAuthority(t *testing.T) {
	authority := permissionCodeSet([]string{"platform:role:create", "platform:role:permission"})
	if !permissionsWithinAuthority([]string{"platform:role:create"}, authority) {
		t.Fatal("自身已有权限子集应允许继续授予")
	}
	if permissionsWithinAuthority([]string{"platform:role:create", "platform:system-log:view"}, authority) {
		t.Fatal("自身没有的日志权限不应允许继续授予")
	}
}

// TestFilterDelegableMenus 验证可编辑权限树只保留可授予节点及其必要祖先。
func TestFilterDelegableMenus(t *testing.T) {
	employeeView := "platform:employee:view"
	employeeCreate := "platform:employee:create"
	systemLogView := "platform:system-log:view"
	menus := []PlatformMenu{
		{ID: 1001, Name: "权限管理", Type: "directory", Scope: "platform", Status: 1},
		{ID: 1002, ParentID: uint64Pointer(1001), Name: "员工管理", Type: "menu", Scope: "platform", PermissionCode: &employeeView, Status: 1},
		{ID: 1003, ParentID: uint64Pointer(1002), Name: "新增员工", Type: "permission", Scope: "platform", PermissionCode: &employeeCreate, Status: 1},
		{ID: 1004, Name: "系统日志", Type: "menu", Scope: "platform", PermissionCode: &systemLogView, Status: 1},
	}
	filtered := filterDelegableMenus(menus, permissionCodeSet([]string{employeeView, employeeCreate}), false, false)
	if len(filtered) != 3 || filtered[0].ID != 1001 || filtered[1].ID != 1002 || filtered[2].ID != 1003 {
		t.Fatalf("权限树过滤结果错误: %#v", filtered)
	}
}

// TestCurrentScopedEmployee 验证普通同范围会话保护本人，代管会话不会因数字 ID 相同而误判。
func TestCurrentScopedEmployee(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenantID := uint64(8)
	normalContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	normalContext.Set("authenticated_employee", auth.Employee{ID: 9, Scope: "tenant", TenantID: &tenantID})
	normalContext.Set("authenticated_token_identity", auth.TokenIdentity{EmployeeID: 9, Scope: "tenant", TenantID: &tenantID, Mode: "normal"})
	if !isCurrentScopedEmployee(normalContext, managementScope{Name: "tenant", TenantID: &tenantID}, 9) {
		t.Fatal("普通租户会话应识别当前员工本人")
	}

	managedContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	managedContext.Set("authenticated_employee", auth.Employee{ID: 9, Scope: "platform"})
	managedContext.Set("authenticated_token_identity", auth.TokenIdentity{EmployeeID: 9, Scope: "platform", TenantID: &tenantID, Mode: "managed"})
	if isCurrentScopedEmployee(managedContext, managementScope{Name: "tenant", TenantID: &tenantID}, 9) {
		t.Fatal("平台代管员工与租户员工 ID 相同也不应判定为本人")
	}
}

// uint64Pointer 返回测试菜单父子关系使用的无符号整数指针。
func uint64Pointer(value uint64) *uint64 {
	return &value
}

// TestNormalizeWriteError 验证数据库约束错误不会泄露为内部错误。
func TestNormalizeWriteError(t *testing.T) {
	duplicate := &mysql.MySQLError{Number: 1062, Message: "duplicate sensitive value"}
	if !errors.Is(normalizeWriteError(duplicate), errManagementConflict) {
		t.Fatal("唯一约束错误应转换为稳定冲突错误")
	}
	original := errors.New("query failed")
	if !errors.Is(normalizeWriteError(original), original) {
		t.Fatal("未知数据库错误应保留原始原因")
	}
}
