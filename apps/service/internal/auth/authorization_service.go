package auth

import (
	"context"
)

// AuthenticationResult 描述一次后台 Token 认证后可写入请求上下文的可信身份。
type AuthenticationResult struct {
	Employee  Employee
	Identity  TokenIdentity
	TenantID  *uint64
	Workspace string
}

// AuthorizationService 负责认证身份和实时权限判断。
type AuthorizationService struct {
	tokens    *TokenManager
	employees EmployeeStore
}

// NewAuthorizationService 创建认证与权限判断服务。
func NewAuthorizationService(tokens *TokenManager, employees EmployeeStore) *AuthorizationService {
	return &AuthorizationService{tokens: tokens, employees: employees}
}

// Authenticate 校验 Token 与数据库中的最新员工、租户和会话状态。
func (service *AuthorizationService) Authenticate(ctx context.Context, rawToken string) (AuthenticationResult, error) {
	identity, err := service.tokens.Parse(rawToken)
	if err != nil {
		return AuthenticationResult{}, ServiceError{Kind: serviceErrorUnauthorized}
	}
	employee, err := service.employees.FindByID(ctx, identity.EmployeeID)
	if err != nil {
		return AuthenticationResult{}, ServiceError{Kind: serviceErrorInternal}
	}
	if employee == nil || employee.Status != 1 || !identityMatchesEmployee(identity, *employee) {
		return AuthenticationResult{}, ServiceError{Kind: serviceErrorUnauthorized}
	}

	tenantID := employee.TenantID
	if identity.Mode == "managed" {
		tenantID = identity.TenantID
	}
	if tenantID != nil {
		tenant, tenantErr := service.employees.FindTenantByID(ctx, *tenantID)
		if tenantErr != nil {
			return AuthenticationResult{}, ServiceError{Kind: serviceErrorInternal}
		}
		if tenant == nil || tenant.Status != 1 {
			return AuthenticationResult{}, ServiceError{Kind: serviceErrorUnauthorized}
		}
	}
	if identity.Mode == "managed" {
		allowed, permissionErr := service.CanEnterTenant(ctx, *employee)
		if permissionErr != nil {
			return AuthenticationResult{}, ServiceError{Kind: serviceErrorInternal}
		}
		if !allowed {
			return AuthenticationResult{}, ServiceError{Kind: serviceErrorForbidden}
		}
	}

	workspace := employee.Scope
	if identity.Mode == "managed" {
		workspace = "tenant"
	}
	return AuthenticationResult{Employee: *employee, Identity: identity, TenantID: tenantID, Workspace: workspace}, nil
}

// CanEnterTenant 实时确认平台员工仍具有进入租户的权限。
func (service *AuthorizationService) CanEnterTenant(ctx context.Context, employee Employee) (bool, error) {
	roles, err := service.employees.ListRoles(ctx, employee)
	if err != nil {
		return false, err
	}
	roleIDs := make([]uint64, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
		if role.SystemKey != nil && *role.SystemKey == "platform_super_admin" {
			return true, nil
		}
	}
	permissions, err := service.employees.ListPermissions(ctx, employee, roleIDs)
	if err != nil {
		return false, err
	}
	for _, permission := range permissions {
		if permission == "platform:tenant:enter" {
			return true, nil
		}
	}
	return false, nil
}

// RequirePermission 实时校验当前员工是否拥有指定工作空间权限。
func (service *AuthorizationService) RequirePermission(ctx context.Context, employee Employee, identity TokenIdentity, workspace, permissionCode string) (bool, bool, error) {
	managedTenant := identity.Mode == "managed" && workspace == "tenant"
	if (!managedTenant && employee.Scope != workspace) || (identity.Mode == "managed" && workspace != "tenant") {
		return false, false, nil
	}

	roles, err := service.employees.ListRoles(ctx, employee)
	if err != nil {
		return false, false, err
	}
	roleIDs := make([]uint64, 0, len(roles))
	managedSuperAdmin := false
	platformSuperAdmin := false
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
		if role.SystemKey == nil {
			continue
		}
		if employee.Scope == "platform" && *role.SystemKey == "platform_super_admin" {
			platformSuperAdmin = true
			if !managedTenant {
				return true, true, nil
			}
			managedSuperAdmin = true
		}
	}

	var permissions []string
	if managedTenant {
		permissions, err = service.employees.ListManagedPermissions(ctx, roleIDs, managedSuperAdmin)
	} else {
		permissions, err = service.employees.ListPermissions(ctx, employee, roleIDs)
	}
	if err != nil {
		return false, platformSuperAdmin, err
	}
	for _, permission := range permissions {
		if permission == permissionCode {
			return true, platformSuperAdmin, nil
		}
	}
	return false, platformSuperAdmin, nil
}

// RequirePlatformSuperAdmin 实时校验当前员工是否为启用的平台超级管理员。
func (service *AuthorizationService) RequirePlatformSuperAdmin(ctx context.Context, employee Employee, identity TokenIdentity) (bool, error) {
	if employee.Scope != "platform" || identity.Mode != "normal" {
		return false, nil
	}
	roles, err := service.employees.ListRoles(ctx, employee)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role.SystemKey != nil && *role.SystemKey == "platform_super_admin" {
			return true, nil
		}
	}
	return false, nil
}

// identityMatchesEmployee 检查 Token 中的员工范围和租户归属是否仍与数据库一致。
func identityMatchesEmployee(identity TokenIdentity, employee Employee) bool {
	if employee.ActiveSessionID == nil || *employee.ActiveSessionID != identity.SessionID {
		return false
	}
	if identity.Mode == "managed" {
		return employee.ID == identity.EmployeeID && employee.Scope == "platform" && employee.TenantID == nil && identity.Scope == "platform" && identity.TenantID != nil
	}
	if employee.ID != identity.EmployeeID || employee.Scope != identity.Scope || (employee.Scope != "platform" && employee.Scope != "tenant") {
		return false
	}
	if employee.Scope == "platform" {
		return employee.TenantID == nil && identity.TenantID == nil
	}
	if employee.TenantID == nil || identity.TenantID == nil {
		return false
	}
	return *employee.TenantID == *identity.TenantID
}
