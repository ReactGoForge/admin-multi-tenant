package rbac

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// PlatformRoleQuery 描述平台角色列表已经校验过的分页和筛选条件。
type PlatformRoleQuery struct {
	Page                       int
	PageSize                   int
	Name                       string
	Type                       string
	Status                     *uint8
	IncludeSuperAdmin          bool
	VisibleProtectedEmployeeID *uint64
}

// PlatformRole 描述平台角色列表与详情需要的只读字段。
type PlatformRole struct {
	ID                     uint64    `gorm:"column:id"`
	Name                   string    `gorm:"column:name"`
	Description            *string   `gorm:"column:description"`
	Type                   string    `gorm:"column:role_type"`
	SystemKey              *string   `gorm:"column:system_key"`
	Status                 uint8     `gorm:"column:status"`
	EmployeeCount          int64     `gorm:"column:employee_count"`
	PermissionCount        int64     `gorm:"column:permission_count"`
	PermissionConfigurable bool      `gorm:"-"`
	CreatedAt              time.Time `gorm:"column:created_at"`
}

// PlatformMenu 描述菜单管理与角色权限树需要的菜单节点。
type PlatformMenu struct {
	ID               uint64  `gorm:"column:id"`
	ParentID         *uint64 `gorm:"column:parent_id"`
	Name             string  `gorm:"column:name"`
	Type             string  `gorm:"column:node_type"`
	Scope            string  `gorm:"column:scope"`
	Path             *string `gorm:"column:path"`
	Component        *string `gorm:"column:component"`
	Icon             *string `gorm:"column:icon"`
	PermissionCode   *string `gorm:"column:permission_code"`
	TenantAssignable uint8   `gorm:"column:tenant_assignable"`
	Sort             uint32  `gorm:"column:sort"`
	Visible          uint8   `gorm:"column:visible"`
	Status           uint8   `gorm:"column:status"`
}

// RoleStore 定义平台角色只读接口需要的查询能力。
type RoleStore interface {
	delegationAuthorityStore
	ListPlatformRoles(context.Context, PlatformRoleQuery) ([]PlatformRole, int64, error)
	FindPlatformRole(context.Context, uint64, bool, *uint64) (*PlatformRole, error)
	ListPlatformRolePermissionIDs(context.Context, uint64) ([]uint64, error)
	ListPlatformRoleTenantPermissionIDs(context.Context, uint64) ([]uint64, error)
	ListMenus(context.Context, string, bool) ([]PlatformMenu, error)
}

// RoleDataStore 定义 Role Service 需要的数据访问能力。
type RoleDataStore interface {
	RoleStore
	ListRoles(context.Context, managementScope, PlatformRoleQuery) ([]PlatformRole, int64, error)
	FindRole(context.Context, managementScope, uint64) (*PlatformRole, error)
	ListRolePermissionIDs(context.Context, managementScope, uint64) ([]uint64, error)
	ListEmployees(context.Context, managementScope, PlatformEmployeeQuery) ([]PlatformEmployee, int64, error)
	WithRoleTransaction(context.Context, func(RoleTransactionStore) error) error
}

// RoleTransactionStore 定义 Role 事务内需要的数据访问能力。
type RoleTransactionStore interface {
	delegationAuthorityStore
	ValidateRolePermissions(context.Context, managementScope, []uint64, bool) error
	ValidateManagedRolePermissions(context.Context, []uint64) error
	CreateRole(context.Context, managementScope, RoleMutation) (uint64, error)
	EnsureCustomRole(context.Context, managementScope, uint64) error
	EnsureAssignableRole(context.Context, managementScope, uint64, bool) (bool, error)
	UpdateRole(context.Context, managementScope, uint64, RoleMutation) (bool, error)
	SetRoleStatus(context.Context, managementScope, uint64, uint8) (bool, error)
	ReplaceRolePermissions(context.Context, managementScope, uint64, []uint64) error
	ReplaceManagedPermissions(context.Context, uint64, []uint64) error
	RoleEmployeeCount(context.Context, managementScope, uint64) (int64, error)
	DeleteRole(context.Context, managementScope, uint64) error
}

// ListPlatformRoleTenantPermissionIDs 查询平台角色当前关联的全部启用租户菜单节点 ID。
func (store *GormStore) ListPlatformRoleTenantPermissionIDs(ctx context.Context, roleID uint64) ([]uint64, error) {
	permissionIDs := make([]uint64, 0)
	err := store.db.WithContext(ctx).
		Table("platform_role_tenant_menus AS prtm").
		Joins("JOIN menus AS m ON m.id = prtm.menu_id").
		Where("prtm.role_id = ? AND m.scope = ? AND m.tenant_assignable = ? AND m.status = ?", roleID, "tenant", 1, 1).
		Order("m.sort ASC, m.id ASC").
		Pluck("m.id", &permissionIDs).Error
	return permissionIDs, err
}

// ListPlatformRoles 按分页、筛选条件和当前查看者可见范围查询平台角色。
func (store *GormStore) ListPlatformRoles(ctx context.Context, query PlatformRoleQuery) ([]PlatformRole, int64, error) {
	baseQuery := store.platformRoleQuery(ctx, query.IncludeSuperAdmin)
	if query.Name != "" {
		baseQuery = baseQuery.Where("LOCATE(?, r.name) > 0", query.Name)
	}
	if query.Type == "system" {
		baseQuery = baseQuery.Where("r.system_key IS NOT NULL")
	} else if query.Type == "custom" {
		baseQuery = baseQuery.Where("r.system_key IS NULL")
	}
	if query.Status != nil {
		baseQuery = baseQuery.Where("r.status = ?", *query.Status)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	roles := make([]PlatformRole, 0)
	selectSQL, selectArgs := platformRoleSelectSQL(query.VisibleProtectedEmployeeID)
	if err := baseQuery.
		Select(selectSQL, selectArgs...).
		Order("r.id ASC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&roles).Error; err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// ListRoles 按工作空间分页读取角色和关联统计。
func (store *GormStore) ListRoles(ctx context.Context, scope managementScope, query PlatformRoleQuery) ([]PlatformRole, int64, error) {
	base := scopedAlias(store.db.WithContext(ctx).Table("roles AS r"), scope, "r")
	if query.Name != "" {
		base = base.Where("LOCATE(?, r.name) > 0", query.Name)
	}
	if query.Type == "system" {
		base = base.Where("r.system_key IS NOT NULL")
	} else if query.Type == "custom" {
		base = base.Where("r.system_key IS NULL")
	}
	if query.Status != nil {
		base = base.Where("r.status = ?", *query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	roles := make([]PlatformRole, 0)
	tenantClause := "IS NULL"
	tenantArgs := make([]any, 0, 2)
	if scope.TenantID != nil {
		tenantClause = "= ?"
		tenantArgs = append(tenantArgs, *scope.TenantID, *scope.TenantID)
	}
	selectSQL := `r.id,
		r.name,
		r.description,
		CASE WHEN r.system_key IS NULL THEN 'custom' ELSE 'system' END AS role_type,
		r.system_key,
		r.status,
		r.created_at,
		(
			SELECT COUNT(*)
			FROM employee_roles AS counted_er
			WHERE counted_er.role_id = r.id
				AND counted_er.scope = '` + scope.Name + `'
				AND counted_er.tenant_id ` + tenantClause + `
		) AS employee_count,
		(
			SELECT COUNT(*)
			FROM role_menus AS counted_rm
			JOIN menus AS counted_m ON counted_m.id = counted_rm.menu_id AND counted_m.scope = counted_rm.scope
			WHERE counted_rm.role_id = r.id
				AND counted_rm.scope = '` + scope.Name + `'
				AND counted_rm.tenant_id ` + tenantClause + `
				AND counted_m.status = 1
		) AS permission_count`
	if err := base.
		Select(selectSQL, tenantArgs...).
		Order("r.id ASC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&roles).Error; err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// FindPlatformRole 按 ID 和当前查看者可见范围查询平台角色。
func (store *GormStore) FindPlatformRole(ctx context.Context, roleID uint64, includeSuperAdmin bool, visibleProtectedEmployeeID *uint64) (*PlatformRole, error) {
	var role PlatformRole
	selectSQL, selectArgs := platformRoleSelectSQL(visibleProtectedEmployeeID)
	err := store.platformRoleQuery(ctx, includeSuperAdmin).
		Select(selectSQL, selectArgs...).
		Where("r.id = ?", roleID).
		Take(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &role, err
}

// FindRole 查询工作空间内指定角色。
func (store *GormStore) FindRole(ctx context.Context, scope managementScope, roleID uint64) (*PlatformRole, error) {
	var role PlatformRole
	result := scopedAlias(store.db.WithContext(ctx).Table("roles AS r"), scope, "r").
		Select("r.id", "r.name", "r.description", "CASE WHEN r.system_key IS NULL THEN 'custom' ELSE 'system' END AS role_type", "r.system_key", "r.status", "r.created_at", "0 AS employee_count", "0 AS permission_count").
		Where("r.id = ?", roleID).
		Take(&role)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errManagementNotFound
	}
	return &role, result.Error
}

// ListPlatformRolePermissionIDs 查询角色当前关联的全部启用平台菜单节点 ID。
func (store *GormStore) ListPlatformRolePermissionIDs(ctx context.Context, roleID uint64) ([]uint64, error) {
	permissionIDs := make([]uint64, 0)
	err := store.db.WithContext(ctx).
		Table("role_menus AS rm").
		Joins("JOIN menus AS m ON m.id = rm.menu_id").
		Where("rm.role_id = ? AND rm.scope = ? AND rm.tenant_id IS NULL AND m.scope = ? AND m.status = ?", roleID, "platform", "platform", 1).
		Order("m.sort ASC, m.id ASC").
		Pluck("m.id", &permissionIDs).Error
	return permissionIDs, err
}

// ListRolePermissionIDs 查询角色权限节点 ID。
func (store *GormStore) ListRolePermissionIDs(ctx context.Context, scope managementScope, roleID uint64) ([]uint64, error) {
	ids := make([]uint64, 0)
	query := scopedAlias(store.db.WithContext(ctx).Table("role_menus AS rm"), scope, "rm").
		Joins("JOIN menus AS m ON m.id = rm.menu_id AND m.scope = rm.scope").
		Where("rm.role_id = ?", roleID)
	return ids, query.Order("rm.menu_id ASC").Pluck("rm.menu_id", &ids).Error
}

// WithRoleTransaction 在单个数据库事务内执行 Role 写流程。
func (store *GormStore) WithRoleTransaction(ctx context.Context, fn func(RoleTransactionStore) error) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GormStore{db: tx})
	})
}

// ValidateRolePermissions 校验权限节点范围、启用状态、租户可分配标记及角色类型限制。
func (store *GormStore) ValidateRolePermissions(ctx context.Context, scope managementScope, permissionIDs []uint64, allowTenantMenuPermissions bool) error {
	return validatePermissions(store.db.WithContext(ctx), scope, permissionIDs, allowTenantMenuPermissions)
}

// ValidateManagedRolePermissions 校验平台角色选择的节点均为可分配租户权限。
func (store *GormStore) ValidateManagedRolePermissions(ctx context.Context, permissionIDs []uint64) error {
	return validateManagedPermissions(store.db.WithContext(ctx), permissionIDs)
}

// CreateRole 写入角色主记录并返回新角色 ID。
func (store *GormStore) CreateRole(ctx context.Context, scope managementScope, mutation RoleMutation) (uint64, error) {
	row := roleInsertRow{Scope: scope.Name, TenantID: scope.TenantID, Name: mutation.Name, Description: mutation.Description, Status: mutation.Status}
	if err := store.db.WithContext(ctx).Table("roles").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// EnsureCustomRole 确认角色存在且不是内置角色。
func (store *GormStore) EnsureCustomRole(ctx context.Context, scope managementScope, roleID uint64) error {
	return ensureCustomRole(store.db.WithContext(ctx), scope, roleID)
}

// EnsureAssignableRole 确认角色允许配置权限，并返回是否允许授予租户菜单管理权限。
func (store *GormStore) EnsureAssignableRole(ctx context.Context, scope managementScope, roleID uint64, platformSuperAdmin bool) (bool, error) {
	return ensureAssignableRole(store.db.WithContext(ctx), scope, roleID, platformSuperAdmin)
}

// UpdateRole 更新角色基础信息并返回是否有记录被更新。
func (store *GormStore) UpdateRole(ctx context.Context, scope managementScope, roleID uint64, mutation RoleMutation) (bool, error) {
	result := scopedTable(store.db.WithContext(ctx).Table("roles"), scope).
		Where("id = ?", roleID).
		Updates(map[string]any{"name": mutation.Name, "description": mutation.Description})
	return result.RowsAffected > 0, result.Error
}

// SetRoleStatus 更新角色状态并返回是否有记录被更新。
func (store *GormStore) SetRoleStatus(ctx context.Context, scope managementScope, roleID uint64, status uint8) (bool, error) {
	result := scopedTable(store.db.WithContext(ctx).Table("roles"), scope).
		Where("id = ?", roleID).
		Update("status", status)
	return result.RowsAffected > 0, result.Error
}

// ReplaceRolePermissions 替换角色菜单权限关联。
func (store *GormStore) ReplaceRolePermissions(ctx context.Context, scope managementScope, roleID uint64, permissionIDs []uint64) error {
	return replaceRolePermissions(store.db.WithContext(ctx), scope, roleID, permissionIDs)
}

// ReplaceManagedPermissions 替换平台角色进入租户后的全部权限关联。
func (store *GormStore) ReplaceManagedPermissions(ctx context.Context, roleID uint64, permissionIDs []uint64) error {
	return replaceManagedPermissions(store.db.WithContext(ctx), roleID, permissionIDs)
}

// RoleEmployeeCount 查询角色当前关联员工数量。
func (store *GormStore) RoleEmployeeCount(ctx context.Context, scope managementScope, roleID uint64) (int64, error) {
	var count int64
	err := scopedTable(store.db.WithContext(ctx).Table("employee_roles"), scope).
		Where("role_id = ?", roleID).
		Count(&count).Error
	return count, err
}

// DeleteRole 删除角色主记录。
func (store *GormStore) DeleteRole(ctx context.Context, scope managementScope, roleID uint64) error {
	return scopedTable(store.db.WithContext(ctx).Table("roles"), scope).
		Where("id = ?", roleID).
		Delete(nil).Error
}

// ListMenus 按范围查询菜单节点，可选择只返回启用节点。
func (store *GormStore) ListMenus(ctx context.Context, scope string, enabledOnly bool) ([]PlatformMenu, error) {
	query := store.db.WithContext(ctx).
		Table("menus").
		Select("id", "parent_id", "name", "node_type", "scope", "path", "component", "icon", "permission_code", "tenant_assignable", "sort", "visible", "status").
		Where("scope = ?", scope)
	if enabledOnly {
		query = query.Where("status = ?", 1)
	}
	menus := make([]PlatformMenu, 0)
	if err := query.Order("sort ASC, id ASC").Scan(&menus).Error; err != nil {
		return nil, err
	}
	return menus, nil
}

// platformRoleQuery 创建限定平台范围的角色基础查询，并按身份决定是否排除超级管理员。
func (store *GormStore) platformRoleQuery(ctx context.Context, includeSuperAdmin bool) *gorm.DB {
	query := store.db.WithContext(ctx).
		Table("roles AS r").
		Where("r.scope = ? AND r.tenant_id IS NULL", "platform")
	if !includeSuperAdmin {
		query = query.Where("r.system_key IS NULL OR r.system_key <> ?", platformSuperAdminKey)
	}
	return query
}

// platformRoleSelectSQL 返回平台角色列表与详情共用的只读字段、本人可见员工统计及动态权限统计。
func platformRoleSelectSQL(visibleProtectedEmployeeID *uint64) (string, []any) {
	employeeVisibilitySQL := `NOT EXISTS (
					SELECT 1
					FROM employee_roles AS owner_er
					JOIN roles AS owner_r ON owner_r.id = owner_er.role_id
					WHERE owner_er.employee_id = counted_er.employee_id
						AND owner_er.scope = 'platform'
						AND owner_er.tenant_id IS NULL
						AND owner_r.scope = 'platform'
						AND owner_r.tenant_id IS NULL
						AND owner_r.system_key = 'platform_super_admin'
				)`
	selectArgs := make([]any, 0, 1)
	if visibleProtectedEmployeeID != nil {
		employeeVisibilitySQL = "(counted_er.employee_id = ? OR " + employeeVisibilitySQL + ")"
		selectArgs = append(selectArgs, *visibleProtectedEmployeeID)
	}
	return `r.id,
		r.name,
		r.description,
		CASE WHEN r.system_key IS NULL THEN 'custom' ELSE 'system' END AS role_type,
		r.system_key,
		r.status,
		r.created_at,
		(
			SELECT COUNT(*)
			FROM employee_roles AS counted_er
			WHERE counted_er.role_id = r.id
				AND counted_er.scope = 'platform'
				AND counted_er.tenant_id IS NULL
				AND ` + employeeVisibilitySQL + `
		) AS employee_count,
		CASE WHEN r.system_key = 'platform_super_admin' THEN (
			SELECT COUNT(*) FROM menus AS platform_m
			WHERE platform_m.scope = 'platform' AND platform_m.status = 1
		) + (
			SELECT COUNT(*) FROM menus AS tenant_m
			WHERE tenant_m.scope = 'tenant' AND tenant_m.status = 1 AND tenant_m.tenant_assignable = 1
		) ELSE (
			SELECT COUNT(*)
			FROM role_menus AS counted_rm
			JOIN menus AS counted_m ON counted_m.id = counted_rm.menu_id
			WHERE counted_rm.role_id = r.id
				AND counted_rm.scope = 'platform'
				AND counted_rm.tenant_id IS NULL
				AND counted_m.scope = 'platform'
				AND counted_m.status = 1
		) + (
			SELECT COUNT(*)
			FROM platform_role_tenant_menus AS counted_prtm
			JOIN menus AS counted_tm ON counted_tm.id = counted_prtm.menu_id
			WHERE counted_prtm.role_id = r.id
				AND counted_tm.scope = 'tenant'
				AND counted_tm.tenant_assignable = 1
				AND counted_tm.status = 1
		) END AS permission_count`, selectArgs
}
