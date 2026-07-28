package rbac

import (
	"context"
)

// RoleApplication 定义 Role Handler 依赖的业务服务能力。
type RoleApplication interface {
	ListPlatformRoles(context.Context, EmployeeActor, PlatformRoleQuery) ([]PlatformRole, int64, error)
	ListTenantRoles(context.Context, EmployeeActor, managementScope, PlatformRoleQuery) ([]PlatformRole, int64, error)
	PlatformRoleDetail(context.Context, EmployeeActor, uint64) (PlatformRoleDetail, error)
	TenantRoleDetail(context.Context, EmployeeActor, managementScope, uint64) (TenantRoleDetail, error)
	PlatformPermissionOptions(context.Context, EmployeeActor) (PlatformPermissionOptions, error)
	ListPlatformRoleEmployees(context.Context, EmployeeActor, uint64, PlatformEmployeeQuery) ([]PlatformEmployee, int64, error)
	ListTenantRoleEmployees(context.Context, EmployeeActor, managementScope, uint64, PlatformEmployeeQuery) ([]PlatformEmployee, int64, error)
	CreateRole(context.Context, EmployeeActor, managementScope, RoleMutation) error
	UpdateRole(context.Context, EmployeeActor, managementScope, uint64, RoleMutation) error
	AssignRolePermissions(context.Context, EmployeeActor, managementScope, uint64, RolePermissionMutation) error
	SetRoleStatus(context.Context, EmployeeActor, managementScope, uint64, uint8) error
	DeleteRole(context.Context, EmployeeActor, managementScope, uint64) error
}

// RoleMutation 描述角色新增和编辑接口经过校验后的字段。
type RoleMutation struct {
	Name                  string
	Description           *string
	PermissionIDs         []uint64
	PlatformPermissionIDs []uint64
	TenantPermissionIDs   []uint64
	Status                uint8
}

// RolePermissionMutation 描述角色授权接口经过校验后的权限 ID。
type RolePermissionMutation struct {
	PermissionIDs         []uint64
	PlatformPermissionIDs []uint64
	TenantPermissionIDs   []uint64
}

// PlatformRoleDetail 描述平台角色详情的内部结果。
type PlatformRoleDetail struct {
	Role                  PlatformRole
	PlatformPermissionIDs []uint64
	TenantPermissionIDs   []uint64
	PlatformMenus         []PlatformMenu
	TenantMenus           []PlatformMenu
}

// TenantRoleDetail 描述租户角色详情的内部结果。
type TenantRoleDetail struct {
	Role          PlatformRole
	PermissionIDs []uint64
	Menus         []PlatformMenu
}

// PlatformPermissionOptions 描述平台角色创建时可选择的权限树。
type PlatformPermissionOptions struct {
	PlatformMenus []PlatformMenu
	TenantMenus   []PlatformMenu
}

// RoleService 编排角色业务规则和事务边界。
type RoleService struct {
	store RoleDataStore
}

// NewRoleService 使用角色数据访问能力创建角色服务。
func NewRoleService(store RoleDataStore) *RoleService {
	return &RoleService{store: store}
}

// ListPlatformRoles 返回平台角色分页列表，并标记权限是否可配置。
func (service *RoleService) ListPlatformRoles(ctx context.Context, actor EmployeeActor, query PlatformRoleQuery) ([]PlatformRole, int64, error) {
	query.IncludeSuperAdmin = actor.PlatformSuperAdmin
	query.VisibleProtectedEmployeeID = platformActorEmployeeID(actor)
	roles, total, err := service.store.ListPlatformRoles(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	if err := service.markRoleConfigurability(ctx, actor, managementScope{Name: "platform"}, roles); err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// ListTenantRoles 返回租户角色分页列表，并标记权限是否可配置。
func (service *RoleService) ListTenantRoles(ctx context.Context, actor EmployeeActor, scope managementScope, query PlatformRoleQuery) ([]PlatformRole, int64, error) {
	roles, total, err := service.store.ListRoles(ctx, scope, query)
	if err != nil {
		return nil, 0, err
	}
	if err := service.markRoleConfigurability(ctx, actor, scope, roles); err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// PlatformRoleDetail 返回平台角色详情、有效平台权限和租户代管权限。
func (service *RoleService) PlatformRoleDetail(ctx context.Context, actor EmployeeActor, roleID uint64) (PlatformRoleDetail, error) {
	scope := managementScope{Name: "platform"}
	role, err := service.store.FindPlatformRole(ctx, roleID, actor.PlatformSuperAdmin, platformActorEmployeeID(actor))
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	if role == nil {
		return PlatformRoleDetail{}, errManagementNotFound
	}
	roles := []PlatformRole{*role}
	if err := service.markRoleConfigurability(ctx, actor, scope, roles); err != nil {
		return PlatformRoleDetail{}, err
	}
	role = &roles[0]
	platformPermissionIDs, err := service.store.ListPlatformRolePermissionIDs(ctx, roleID)
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	tenantPermissionIDs, err := service.store.ListPlatformRoleTenantPermissionIDs(ctx, roleID)
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	platformMenus, tenantMenus, authority, err := service.loadPlatformAndTenantMenus(ctx, actor, scope)
	if err != nil {
		return PlatformRoleDetail{}, err
	}
	if role.SystemKey != nil && *role.SystemKey == platformSuperAdminKey {
		platformPermissionIDs = allMenuIDs(platformMenus, false)
		tenantPermissionIDs = allMenuIDs(tenantMenus, true)
	}
	if role.PermissionConfigurable {
		platformMenus = filterDelegableMenus(platformMenus, authority.PlatformPermissions, authority.Unrestricted, false)
		tenantMenus = filterDelegableMenus(tenantMenus, authority.TenantPermissions, authority.Unrestricted, false)
	}
	return PlatformRoleDetail{Role: *role, PlatformPermissionIDs: platformPermissionIDs, TenantPermissionIDs: tenantPermissionIDs, PlatformMenus: platformMenus, TenantMenus: tenantMenus}, nil
}

// TenantRoleDetail 返回租户角色详情和可分配权限树。
func (service *RoleService) TenantRoleDetail(ctx context.Context, actor EmployeeActor, scope managementScope, roleID uint64) (TenantRoleDetail, error) {
	role, err := service.store.FindRole(ctx, scope, roleID)
	if err != nil {
		return TenantRoleDetail{}, err
	}
	roles := []PlatformRole{*role}
	if err := service.markRoleConfigurability(ctx, actor, scope, roles); err != nil {
		return TenantRoleDetail{}, err
	}
	role = &roles[0]
	permissionIDs, err := service.store.ListRolePermissionIDs(ctx, scope, roleID)
	if err != nil {
		return TenantRoleDetail{}, err
	}
	menus, err := service.store.ListMenus(ctx, "tenant", true)
	if err != nil {
		return TenantRoleDetail{}, err
	}
	if role.PermissionConfigurable {
		authority, err := service.currentAuthority(ctx, actor, scope)
		if err != nil {
			return TenantRoleDetail{}, err
		}
		allowTenantMenuPermissions := canAssignTenantMenuPermissions(scope, role.SystemKey, actor.PlatformSuperAdmin)
		menus = filterDelegableMenus(menus, authority.TenantPermissions, authority.Unrestricted, allowTenantMenuPermissions)
	}
	return TenantRoleDetail{Role: *role, PermissionIDs: permissionIDs, Menus: menus}, nil
}

// PlatformPermissionOptions 返回平台角色表单可分配的平台和租户权限树。
func (service *RoleService) PlatformPermissionOptions(ctx context.Context, actor EmployeeActor) (PlatformPermissionOptions, error) {
	scope := managementScope{Name: "platform"}
	platformMenus, tenantMenus, authority, err := service.loadPlatformAndTenantMenus(ctx, actor, scope)
	if err != nil {
		return PlatformPermissionOptions{}, err
	}
	return PlatformPermissionOptions{
		PlatformMenus: filterDelegableMenus(platformMenus, authority.PlatformPermissions, authority.Unrestricted, false),
		TenantMenus:   filterDelegableMenus(tenantMenus, authority.TenantPermissions, authority.Unrestricted, false),
	}, nil
}

// ListPlatformRoleEmployees 返回平台角色关联员工分页列表。
func (service *RoleService) ListPlatformRoleEmployees(ctx context.Context, actor EmployeeActor, roleID uint64, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	role, err := service.store.FindPlatformRole(ctx, roleID, actor.PlatformSuperAdmin, platformActorEmployeeID(actor))
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		return nil, 0, errManagementNotFound
	}
	query.RoleID = &roleID
	query.VisibleProtectedEmployeeID = platformActorEmployeeID(actor)
	employees, total, err := service.store.ListEmployees(ctx, managementScope{Name: "platform"}, query)
	if err != nil {
		return nil, 0, err
	}
	if err := service.markEmployeeRoles(ctx, actor, managementScope{Name: "platform"}, employees); err != nil {
		return nil, 0, err
	}
	return employees, total, nil
}

// ListTenantRoleEmployees 返回租户角色关联员工分页列表。
func (service *RoleService) ListTenantRoleEmployees(ctx context.Context, actor EmployeeActor, scope managementScope, roleID uint64, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	if _, err := service.store.FindRole(ctx, scope, roleID); err != nil {
		return nil, 0, err
	}
	query.RoleID = &roleID
	employees, total, err := service.store.ListEmployees(ctx, scope, query)
	if err != nil {
		return nil, 0, err
	}
	if err := service.markEmployeeRoles(ctx, actor, scope, employees); err != nil {
		return nil, 0, err
	}
	return employees, total, nil
}

// CreateRole 创建角色并在同一事务中写入权限关联。
func (service *RoleService) CreateRole(ctx context.Context, actor EmployeeActor, scope managementScope, mutation RoleMutation) error {
	return service.store.WithRoleTransaction(ctx, func(tx RoleTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		permissionIDs := roleDirectPermissionIDs(scope, mutation.PermissionIDs, mutation.PlatformPermissionIDs)
		if err := tx.ValidateRolePermissions(ctx, scope, permissionIDs, false); err != nil {
			return err
		}
		if err := validatePermissionIDsWithinAuthorityWithStore(ctx, tx, scope, permissionIDs, authority); err != nil {
			return err
		}
		roleID, err := tx.CreateRole(ctx, scope, mutation)
		if err != nil {
			return err
		}
		if err := tx.ReplaceRolePermissions(ctx, scope, roleID, permissionIDs); err != nil {
			return err
		}
		if scope.Name != "platform" {
			return nil
		}
		if err := tx.ValidateManagedRolePermissions(ctx, mutation.TenantPermissionIDs); err != nil {
			return err
		}
		if err := validatePermissionIDsWithinAuthorityWithStore(ctx, tx, managementScope{Name: "tenant"}, mutation.TenantPermissionIDs, authority); err != nil {
			return err
		}
		return tx.ReplaceManagedPermissions(ctx, roleID, mutation.TenantPermissionIDs)
	})
}

// UpdateRole 更新非内置角色的名称和描述。
func (service *RoleService) UpdateRole(ctx context.Context, actor EmployeeActor, scope managementScope, roleID uint64, mutation RoleMutation) error {
	return service.store.WithRoleTransaction(ctx, func(tx RoleTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, []uint64{roleID}, authority); err != nil {
			return err
		}
		if err := tx.EnsureCustomRole(ctx, scope, roleID); err != nil {
			return err
		}
		changed, err := tx.UpdateRole(ctx, scope, roleID, mutation)
		if err != nil {
			return normalizeWriteError(err)
		}
		if !changed {
			return errManagementNotFound
		}
		return nil
	})
}

// AssignRolePermissions 替换角色权限，并保留平台角色租户代管权限的同事务写入。
func (service *RoleService) AssignRolePermissions(ctx context.Context, actor EmployeeActor, scope managementScope, roleID uint64, mutation RolePermissionMutation) error {
	return service.store.WithRoleTransaction(ctx, func(tx RoleTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, []uint64{roleID}, authority); err != nil {
			return err
		}
		allowTenantMenuPermissions, err := tx.EnsureAssignableRole(ctx, scope, roleID, actor.PlatformSuperAdmin)
		if err != nil {
			return err
		}
		permissionIDs := roleDirectPermissionIDs(scope, mutation.PermissionIDs, mutation.PlatformPermissionIDs)
		if err := tx.ValidateRolePermissions(ctx, scope, permissionIDs, allowTenantMenuPermissions); err != nil {
			return err
		}
		if err := validatePermissionIDsWithinAuthorityWithStore(ctx, tx, scope, permissionIDs, authority); err != nil {
			return err
		}
		if err := tx.ReplaceRolePermissions(ctx, scope, roleID, permissionIDs); err != nil {
			return err
		}
		if scope.Name != "platform" {
			return nil
		}
		if err := tx.ValidateManagedRolePermissions(ctx, mutation.TenantPermissionIDs); err != nil {
			return err
		}
		if err := validatePermissionIDsWithinAuthorityWithStore(ctx, tx, managementScope{Name: "tenant"}, mutation.TenantPermissionIDs, authority); err != nil {
			return err
		}
		return tx.ReplaceManagedPermissions(ctx, roleID, mutation.TenantPermissionIDs)
	})
}

// SetRoleStatus 更新非内置角色状态。
func (service *RoleService) SetRoleStatus(ctx context.Context, actor EmployeeActor, scope managementScope, roleID uint64, status uint8) error {
	return service.store.WithRoleTransaction(ctx, func(tx RoleTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, []uint64{roleID}, authority); err != nil {
			return err
		}
		if err := tx.EnsureCustomRole(ctx, scope, roleID); err != nil {
			return err
		}
		changed, err := tx.SetRoleStatus(ctx, scope, roleID, status)
		if err != nil {
			return normalizeWriteError(err)
		}
		if !changed {
			return errManagementNotFound
		}
		return nil
	})
}

// DeleteRole 删除无员工关联的自定义角色和权限关联。
func (service *RoleService) DeleteRole(ctx context.Context, actor EmployeeActor, scope managementScope, roleID uint64) error {
	return service.store.WithRoleTransaction(ctx, func(tx RoleTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, []uint64{roleID}, authority); err != nil {
			return err
		}
		if err := tx.EnsureCustomRole(ctx, scope, roleID); err != nil {
			return err
		}
		count, err := tx.RoleEmployeeCount(ctx, scope, roleID)
		if err != nil {
			return err
		}
		if count > 0 {
			return errManagementConflict
		}
		if err := tx.ReplaceRolePermissions(ctx, scope, roleID, nil); err != nil {
			return err
		}
		if scope.Name == "platform" {
			if err := tx.ReplaceManagedPermissions(ctx, roleID, nil); err != nil {
				return err
			}
		}
		return tx.DeleteRole(ctx, scope, roleID)
	})
}

// markRoleConfigurability 批量标记角色权限是否允许当前操作者配置。
func (service *RoleService) markRoleConfigurability(ctx context.Context, actor EmployeeActor, scope managementScope, roles []PlatformRole) error {
	authority, err := service.currentAuthority(ctx, actor, scope)
	if err != nil {
		return err
	}
	return markRolePermissionConfigurability(ctx, service.store, scope, roles, authority, actor.PlatformSuperAdmin)
}

// markEmployeeRoles 批量标记员工角色是否允许当前操作者分配。
func (service *RoleService) markEmployeeRoles(ctx context.Context, actor EmployeeActor, scope managementScope, employees []PlatformEmployee) error {
	authority, err := service.currentAuthority(ctx, actor, scope)
	if err != nil {
		return err
	}
	return markEmployeeRoleAssignability(ctx, service.store, scope, employees, authority)
}

// currentAuthority 读取当前请求的实时向下授权边界。
func (service *RoleService) currentAuthority(ctx context.Context, actor EmployeeActor, scope managementScope) (delegationAuthority, error) {
	return service.currentAuthorityWithStore(ctx, service.store, actor, scope)
}

// currentAuthorityWithStore 从指定 Store 读取当前请求的实时向下授权边界。
func (service *RoleService) currentAuthorityWithStore(ctx context.Context, store delegationAuthorityStore, actor EmployeeActor, scope managementScope) (delegationAuthority, error) {
	return store.LoadDelegationAuthority(ctx, actor.Employee, actor.Identity, scope, actor.PlatformSuperAdmin)
}

// loadPlatformAndTenantMenus 读取平台和租户权限树以及当前授权边界。
func (service *RoleService) loadPlatformAndTenantMenus(ctx context.Context, actor EmployeeActor, scope managementScope) ([]PlatformMenu, []PlatformMenu, delegationAuthority, error) {
	authority, err := service.currentAuthority(ctx, actor, scope)
	if err != nil {
		return nil, nil, delegationAuthority{}, err
	}
	platformMenus, err := service.store.ListMenus(ctx, "platform", true)
	if err != nil {
		return nil, nil, delegationAuthority{}, err
	}
	tenantMenus, err := service.store.ListMenus(ctx, "tenant", true)
	if err != nil {
		return nil, nil, delegationAuthority{}, err
	}
	return platformMenus, tenantMenus, authority, nil
}

// roleDirectPermissionIDs 返回当前工作空间应写入 role_menus 的权限 ID。
func roleDirectPermissionIDs(scope managementScope, permissionIDs []uint64, platformPermissionIDs []uint64) []uint64 {
	if scope.Name == "platform" {
		return platformPermissionIDs
	}
	return permissionIDs
}

// allMenuIDs 返回启用菜单节点 ID，可按租户可分配标记过滤。
func allMenuIDs(menus []PlatformMenu, tenantAssignableOnly bool) []uint64 {
	ids := make([]uint64, 0, len(menus))
	for _, menu := range menus {
		if tenantAssignableOnly && menu.TenantAssignable != 1 {
			continue
		}
		ids = append(ids, menu.ID)
	}
	return ids
}

// validatePermissionIDsWithinAuthorityWithStore 校验待写入菜单节点不会扩大当前操作者权限边界。
func validatePermissionIDsWithinAuthorityWithStore(ctx context.Context, store RoleTransactionStore, scope managementScope, permissionIDs []uint64, authority delegationAuthority) error {
	if authority.Unrestricted || len(permissionIDs) == 0 {
		return nil
	}
	menus, err := storeMenusForScope(ctx, store, scope.Name)
	if err != nil {
		return err
	}
	permissions := make([]string, 0, len(permissionIDs))
	permissionIDSet := make(map[uint64]struct{}, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		permissionIDSet[permissionID] = struct{}{}
	}
	for _, menu := range menus {
		if _, exists := permissionIDSet[menu.ID]; exists && menu.PermissionCode != nil {
			permissions = append(permissions, *menu.PermissionCode)
		}
	}
	if !permissionsWithinAuthority(permissions, authority.permissionsForScope(scope.Name)) {
		return errManagementForbidden
	}
	return nil
}

// storeMenusForScope 通过事务 Store 读取指定范围的菜单节点。
func storeMenusForScope(ctx context.Context, store RoleTransactionStore, scope string) ([]PlatformMenu, error) {
	if menuStore, ok := store.(interface {
		ListMenus(context.Context, string, bool) ([]PlatformMenu, error)
	}); ok {
		return menuStore.ListMenus(ctx, scope, false)
	}
	return nil, errManagementForbidden
}
