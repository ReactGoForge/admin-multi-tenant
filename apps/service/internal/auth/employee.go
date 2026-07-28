package auth

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Employee 描述登录与认证流程读取的现有员工字段。
type Employee struct {
	ID              uint64  `gorm:"column:id"`
	Scope           string  `gorm:"column:scope"`
	TenantID        *uint64 `gorm:"column:tenant_id"`
	Name            string  `gorm:"column:name"`
	LoginAccount    string  `gorm:"column:login_account"`
	PasswordHash    string  `gorm:"column:password_hash"`
	ActiveSessionID *string `gorm:"column:active_session_id"`
	AvatarImageID   *uint64 `gorm:"column:avatar_image_id"`
	Phone           *string `gorm:"column:phone"`
	Status          uint8   `gorm:"column:status"`
}

// Role 描述当前员工关联的启用角色。
type Role struct {
	ID        uint64  `gorm:"column:id"`
	Name      string  `gorm:"column:name"`
	SystemKey *string `gorm:"column:system_key"`
}

// Tenant 描述认证流程需要检查的现有租户字段。
type Tenant struct {
	ID          uint64  `gorm:"column:id"`
	Name        string  `gorm:"column:name"`
	IconURL     *string `gorm:"column:icon_url"`
	IconImageID *uint64 `gorm:"column:icon_image_id"`
	Status      uint8   `gorm:"column:status"`
}

// PlatformBrand 描述当前用户接口返回的全平台品牌信息。
type PlatformBrand struct {
	Name        string  `gorm:"column:name"`
	IconImageID *uint64 `gorm:"column:icon_image_id"`
}

// NavigationMenu 描述当前工作空间导航需要的启用菜单节点。
type NavigationMenu struct {
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
}

// EmployeeStore 定义登录、会话激活和当前用户接口所需的最小数据能力。
// Go 学习提示：这是一个接口，只声明“能做什么”而不规定“怎么做”。Handler 因此不依赖
// GORM 细节，测试也可以用简单的假对象实现这些方法。
type EmployeeStore interface {
	FindByLogin(context.Context, string) (*Employee, error)
	FindByID(context.Context, uint64) (*Employee, error)
	ActivateSession(context.Context, uint64, string) error
	UpdateBasicProfile(context.Context, uint64, *string) error
	ChangePassword(context.Context, uint64, string) error
	FindTenantByID(context.Context, uint64) (*Tenant, error)
	ListRoles(context.Context, Employee) ([]Role, error)
	ListPermissions(context.Context, Employee, []uint64) ([]string, error)
	ListManagedPermissions(context.Context, []uint64, bool) ([]string, error)
	FindPlatformBrand(context.Context) (*PlatformBrand, error)
	ListNavigationMenus(context.Context, string) ([]NavigationMenu, error)
}

// GormEmployeeStore 使用 GORM 查询现有 RBAC 表并激活员工后台会话。
type GormEmployeeStore struct {
	db *gorm.DB
}

// NewEmployeeStore 创建基于当前数据库连接的员工认证数据对象。
func NewEmployeeStore(db *gorm.DB) *GormEmployeeStore {
	return &GormEmployeeStore{db: db}
}

// FindByLogin 按全局唯一登录账号读取员工认证字段。
func (store *GormEmployeeStore) FindByLogin(ctx context.Context, loginAccount string) (*Employee, error) {
	var employee Employee
	err := store.db.WithContext(ctx).
		Table("employees").
		Select("id", "scope", "tenant_id", "name", "login_account", "password_hash", "active_session_id", "avatar_image_id", "phone", "status").
		Where("login_account = ?", loginAccount).
		Take(&employee).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &employee, err
}

// FindByID 按员工 ID 读取最新认证状态与工作空间归属。
func (store *GormEmployeeStore) FindByID(ctx context.Context, employeeID uint64) (*Employee, error) {
	var employee Employee
	err := store.db.WithContext(ctx).
		Table("employees").
		Select("id", "scope", "tenant_id", "name", "login_account", "password_hash", "active_session_id", "avatar_image_id", "phone", "status").
		Where("id = ?", employeeID).
		Take(&employee).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &employee, err
}

// ActivateSession 覆盖写入启用员工当前唯一有效的后台会话标识。
func (store *GormEmployeeStore) ActivateSession(ctx context.Context, employeeID uint64, sessionID string) error {
	// 安全边界：每次登录覆盖 active_session_id，使旧 JWT 即使尚未到期也会因会话标识不匹配而失效。
	result := store.db.WithContext(ctx).
		Table("employees").
		Where("id = ? AND status = ?", employeeID, 1).
		Update("active_session_id", sessionID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("员工会话激活失败")
	}
	return nil
}

// UpdateBasicProfile 更新当前启用员工本人的可空手机号。
func (store *GormEmployeeStore) UpdateBasicProfile(ctx context.Context, employeeID uint64, phone *string) error {
	result := store.db.WithContext(ctx).
		Table("employees").
		Where("id = ? AND status = ?", employeeID, 1).
		Updates(map[string]any{"phone": phone})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}

	// MySQL 对新旧值完全相同时可能返回零行；回读确认员工仍启用且资料已经是目标值。
	employee, err := store.FindByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if employee == nil || employee.Status != 1 || !sameOptionalString(employee.Phone, phone) {
		return errors.New("员工基本资料更新失败")
	}
	return nil
}

// sameOptionalString 判断两个可空文本是否表达相同数据库值。
func sameOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// ChangePassword 更新当前员工密码哈希并清空唯一有效会话，使全部旧 Token 立即失效。
func (store *GormEmployeeStore) ChangePassword(ctx context.Context, employeeID uint64, passwordHash string) error {
	result := store.db.WithContext(ctx).Table("employees").Where("id = ? AND status = ?", employeeID, 1).
		Updates(map[string]any{"password_hash": passwordHash, "active_session_id": nil})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("员工密码修改失败")
	}
	return nil
}

// FindTenantByID 读取租户名称与最新启用状态。
func (store *GormEmployeeStore) FindTenantByID(ctx context.Context, tenantID uint64) (*Tenant, error) {
	var tenant Tenant
	err := store.db.WithContext(ctx).
		Table("tenants").
		Select("id", "name", "icon_url", "icon_image_id", "status").
		Where("id = ?", tenantID).
		Take(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &tenant, err
}

// FindPlatformBrand 读取全平台唯一品牌名称和图标引用。
func (store *GormEmployeeStore) FindPlatformBrand(ctx context.Context) (*PlatformBrand, error) {
	var brand PlatformBrand
	err := store.db.WithContext(ctx).Table("platform_settings").Select("name", "icon_image_id").Where("id = ?", 1).Take(&brand).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &brand, err
}

// ListRoles 读取员工在当前工作空间中关联的全部启用角色。
func (store *GormEmployeeStore) ListRoles(ctx context.Context, employee Employee) ([]Role, error) {
	query := store.db.WithContext(ctx).
		Table("employee_roles AS er").
		Select("r.id", "r.name", "r.system_key").
		Joins("JOIN roles AS r ON r.id = er.role_id").
		Where("er.employee_id = ? AND er.scope = ? AND r.scope = ? AND r.status = ?", employee.ID, employee.Scope, employee.Scope, 1)
	if employee.TenantID == nil {
		query = query.Where("er.tenant_id IS NULL AND r.tenant_id IS NULL")
	} else {
		query = query.Where("er.tenant_id = ? AND r.tenant_id = ?", *employee.TenantID, *employee.TenantID)
	}

	var roles []Role
	if err := query.Order("r.id ASC").Scan(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// ListPermissions 按员工当前启用角色读取数据库实时权限。
func (store *GormEmployeeStore) ListPermissions(ctx context.Context, employee Employee, roleIDs []uint64) ([]string, error) {
	var permissions []string
	if len(roleIDs) == 0 {
		return permissions, nil
	}

	// 安全边界：权限从当前启用的角色和菜单实时查询，撤销角色或停用菜单后无需等待 JWT 过期。
	query := store.db.WithContext(ctx).
		Table("role_menus AS rm").
		Distinct("m.permission_code").
		Joins("JOIN menus AS m ON m.id = rm.menu_id").
		Where("rm.role_id IN ? AND rm.scope = ? AND m.scope = ? AND m.status = ? AND m.permission_code IS NOT NULL", roleIDs, employee.Scope, employee.Scope, 1)
	if employee.TenantID == nil {
		query = query.Where("rm.tenant_id IS NULL")
	} else {
		query = query.Where("rm.tenant_id = ?", *employee.TenantID)
	}
	if err := query.Order("m.permission_code ASC").Pluck("m.permission_code", &permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

// ListManagedPermissions 读取平台角色进入任意租户后可使用的实时租户权限。
func (store *GormEmployeeStore) ListManagedPermissions(ctx context.Context, roleIDs []uint64, superAdmin bool) ([]string, error) {
	permissions := make([]string, 0)
	query := store.db.WithContext(ctx).
		Table("menus AS m").
		Distinct("m.permission_code").
		Where("m.scope = ? AND m.tenant_assignable = ? AND m.status = ? AND m.permission_code IS NOT NULL", "tenant", 1, 1)
	if !superAdmin {
		if len(roleIDs) == 0 {
			return permissions, nil
		}
		query = query.Joins("JOIN platform_role_tenant_menus AS prtm ON prtm.menu_id = m.id").Where("prtm.role_id IN ?", roleIDs)
	}
	if err := query.Order("m.permission_code ASC").Pluck("m.permission_code", &permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

// ListNavigationMenus 查询当前工作空间全部启用菜单定义，由前端再按实时权限过滤。
func (store *GormEmployeeStore) ListNavigationMenus(ctx context.Context, scope string) ([]NavigationMenu, error) {
	menus := make([]NavigationMenu, 0)
	err := store.db.WithContext(ctx).
		Table("menus").
		Select("id", "parent_id", "name", "node_type", "scope", "path", "component", "icon", "permission_code", "tenant_assignable", "sort", "visible").
		Where("scope = ? AND status = ?", scope, 1).
		Order("sort ASC, id ASC").
		Scan(&menus).Error
	return menus, err
}
