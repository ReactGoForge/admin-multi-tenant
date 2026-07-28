package rbac

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var permissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(?::[a-z][a-z0-9-]*)+$`)

var registeredMenuPages = map[string]map[string]string{
	"platform": {
		"/platform":                    "router/modules/platform-index.tsx",
		"/platform/tenants":            "pages/platform/tenants/index.tsx",
		"/platform/images":             "pages/platform/images/index.tsx",
		"/platform/system/employees":   "pages/platform/system/employees/index.tsx",
		"/platform/system/roles":       "pages/platform/system/roles/index.tsx",
		"/platform/system/menus":       "pages/platform/system/menus/index.tsx",
		"/platform/system/departments": "pages/platform/system/departments/index.tsx",
		"/platform/system/basic":       "pages/platform/system/basic/index.tsx",
		"/platform/system/miniapp":     "pages/platform/system/miniapp/index.tsx",
		"/platform/system/fields":      "pages/platform/system/fields/index.tsx",
		"/platform/users":              "pages/platform/users/index.tsx",
		"/platform/logs/system":        "pages/platform/logs/system/index.tsx",
		"/platform/logs/operations":    "pages/platform/logs/operations/index.tsx",
	},
	"tenant": {
		"/tenant":                    "router/modules/tenant-index.tsx",
		"/tenant/system/employees":   "pages/tenant/system/employees/index.tsx",
		"/tenant/images":             "pages/tenant/images/index.tsx",
		"/tenant/system/roles":       "pages/tenant/system/roles/index.tsx",
		"/tenant/system/menus":       "pages/tenant/system/menus/index.tsx",
		"/tenant/system/departments": "pages/tenant/system/departments/index.tsx",
		"/tenant/system/basic":       "pages/tenant/system/basic/index.tsx",
		"/tenant/users":              "pages/tenant/users/index.tsx",
		"/tenant/logs/operations":    "pages/tenant/logs/operations/index.tsx",
	},
}

// MenuMutation 描述已经通过 HTTP 基础校验的菜单写入字段。
type MenuMutation struct {
	Scope            string
	ParentID         *uint64
	Name             string
	Type             string
	Path             *string
	Component        *string
	Icon             *string
	PermissionCode   *string
	TenantAssignable uint8
	Sort             uint32
	Visible          uint8
	Status           uint8
}

// menuInsertRow 映射菜单创建时写入数据库的全部字段。
type menuInsertRow struct {
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

// MenuDataStore 定义 Menu Service 需要的数据访问能力。
type MenuDataStore interface {
	ListMenus(context.Context, string, bool) ([]PlatformMenu, error)
	WithMenuTransaction(context.Context, func(MenuTransactionStore) error) error
}

// MenuTransactionStore 定义 Menu 写事务内需要的数据访问能力。
type MenuTransactionStore interface {
	FindMenuForUpdate(context.Context, uint64) (*PlatformMenu, error)
	CreateMenu(context.Context, MenuMutation) (uint64, error)
	UpdateMenu(context.Context, uint64, MenuMutation) error
	SetMenuStatus(context.Context, uint64, uint8) error
	DeleteMenu(context.Context, uint64) error
	MenuChildCount(context.Context, uint64, bool) (int64, error)
	RoleMenuRelationCount(context.Context, uint64) (int64, error)
	ManagedMenuRelationCount(context.Context, uint64) (int64, error)
	AssignMenuToTenantOwners(context.Context, uint64) error
}

// MenuService 编排菜单业务规则、事务边界和持久化错误归一化。
type MenuService struct {
	store MenuDataStore
}

// NewMenuService 使用菜单数据访问能力创建菜单服务。
func NewMenuService(store MenuDataStore) *MenuService {
	return &MenuService{store: store}
}

// ListMenus 返回指定菜单范围的全部节点。
func (service *MenuService) ListMenus(ctx context.Context, scope string) ([]PlatformMenu, error) {
	return service.store.ListMenus(ctx, scope, false)
}

// CreateMenu 校验菜单字段和父节点后创建菜单节点。
func (service *MenuService) CreateMenu(ctx context.Context, mutation MenuMutation) error {
	if !validateMenuNodeFields(mutation) {
		return errManagementInvalid
	}
	return service.store.WithMenuTransaction(ctx, func(tx MenuTransactionStore) error {
		if err := service.validateMenuParent(ctx, tx, mutation, nil); err != nil {
			return err
		}
		menuID, err := tx.CreateMenu(ctx, mutation)
		if err != nil {
			return normalizeWriteError(err)
		}
		if mutation.Scope == "tenant" && mutation.TenantAssignable == 1 && isDefaultTenantOwnerPermission(mutation.PermissionCode) {
			return tx.AssignMenuToTenantOwners(ctx, menuID)
		}
		return nil
	})
}

// UpdateMenu 校验不可变字段、层级和核心节点保护后更新菜单。
func (service *MenuService) UpdateMenu(ctx context.Context, menuID uint64, mutation MenuMutation) error {
	if !validateMenuNodeFields(mutation) {
		return errManagementInvalid
	}
	return service.store.WithMenuTransaction(ctx, func(tx MenuTransactionStore) error {
		existing, err := tx.FindMenuForUpdate(ctx, menuID)
		if err != nil {
			return err
		}
		if existing.Scope != mutation.Scope || existing.Type != mutation.Type || existing.Status != mutation.Status {
			return errManagementConflict
		}
		if isProtectedPlatformMenu(*existing) && !protectedMenuFieldsUnchanged(*existing, mutation) {
			return errManagementProtected
		}
		if err := service.validateMenuParent(ctx, tx, mutation, &menuID); err != nil {
			return err
		}
		if err := tx.UpdateMenu(ctx, menuID, mutation); err != nil {
			return normalizeWriteError(err)
		}
		if existing.Scope == "tenant" && existing.TenantAssignable == 0 && mutation.TenantAssignable == 1 && isDefaultTenantOwnerPermission(mutation.PermissionCode) {
			return tx.AssignMenuToTenantOwners(ctx, menuID)
		}
		return nil
	})
}

// SetMenuStatus 校验父子状态和核心节点保护后更新菜单状态。
func (service *MenuService) SetMenuStatus(ctx context.Context, menuID uint64, status uint8) error {
	return service.store.WithMenuTransaction(ctx, func(tx MenuTransactionStore) error {
		menu, err := tx.FindMenuForUpdate(ctx, menuID)
		if err != nil {
			return err
		}
		if menu.Status == status {
			return nil
		}
		if status == 0 {
			if isProtectedPlatformMenu(*menu) {
				return errManagementProtected
			}
			enabledChildren, err := tx.MenuChildCount(ctx, menuID, true)
			if err != nil {
				return err
			}
			if enabledChildren > 0 {
				return errManagementConflict
			}
		} else if menu.ParentID != nil {
			parent, err := tx.FindMenuForUpdate(ctx, *menu.ParentID)
			if err != nil {
				return err
			}
			if parent.Scope != menu.Scope || parent.Status != 1 {
				return errManagementConflict
			}
		}
		return tx.SetMenuStatus(ctx, menuID, status)
	})
}

// DeleteMenu 保守删除无子节点、无角色关联且非核心的菜单节点。
func (service *MenuService) DeleteMenu(ctx context.Context, menuID uint64) error {
	return service.store.WithMenuTransaction(ctx, func(tx MenuTransactionStore) error {
		menu, err := tx.FindMenuForUpdate(ctx, menuID)
		if err != nil {
			return err
		}
		if isProtectedPlatformMenu(*menu) {
			return errManagementProtected
		}
		children, err := tx.MenuChildCount(ctx, menuID, false)
		if err != nil {
			return err
		}
		roleRelations, err := tx.RoleMenuRelationCount(ctx, menuID)
		if err != nil {
			return err
		}
		managedRelations, err := tx.ManagedMenuRelationCount(ctx, menuID)
		if err != nil {
			return err
		}
		if children > 0 || roleRelations > 0 || managedRelations > 0 {
			return errManagementConflict
		}
		return tx.DeleteMenu(ctx, menuID)
	})
}

// validateMenuNodeFields 校验目录、页面菜单和操作权限的字段组合。
func validateMenuNodeFields(mutation MenuMutation) bool {
	switch mutation.Type {
	case "directory":
		return mutation.Path == nil && mutation.Component == nil && mutation.PermissionCode == nil
	case "menu":
		if mutation.Path == nil || mutation.Component == nil || mutation.PermissionCode == nil || !permissionCodePattern.MatchString(*mutation.PermissionCode) {
			return false
		}
		component, exists := registeredMenuPages[mutation.Scope][*mutation.Path]
		if !exists || component != *mutation.Component {
			return false
		}
		if mutation.Scope == "platform" && isPlatformProtectedPermissionGroup(mutation.PermissionCode) {
			return (*mutation.Path == "/platform/system/menus" && strings.HasPrefix(*mutation.PermissionCode, "platform:menu:")) ||
				(*mutation.Path == "/platform/system/fields" && strings.HasPrefix(*mutation.PermissionCode, "platform:field:")) ||
				(*mutation.Path == "/platform/system/miniapp" && strings.HasPrefix(*mutation.PermissionCode, "platform:miniapp:")) ||
				(*mutation.Path == "/platform/logs/system" && *mutation.PermissionCode == "platform:system-log:view")
		}
		return true
	case "permission":
		return mutation.ParentID != nil && mutation.Path == nil && mutation.Component == nil && mutation.Icon == nil && mutation.PermissionCode != nil && permissionCodePattern.MatchString(*mutation.PermissionCode) && mutation.Visible == 0
	default:
		return false
	}
}

// WithMenuTransaction 在单个数据库事务内执行 Menu 写流程。
func (store *GormStore) WithMenuTransaction(ctx context.Context, fn func(MenuTransactionStore) error) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GormStore{db: tx})
	})
}

// CreateMenu 写入菜单节点并返回新菜单 ID。
func (store *GormStore) CreateMenu(ctx context.Context, mutation MenuMutation) (uint64, error) {
	row := menuInsertRow{
		ParentID: mutation.ParentID, Name: mutation.Name, Type: mutation.Type, Scope: mutation.Scope,
		Path: mutation.Path, Component: mutation.Component, Icon: mutation.Icon, PermissionCode: mutation.PermissionCode,
		TenantAssignable: mutation.TenantAssignable, Sort: mutation.Sort, Visible: mutation.Visible, Status: mutation.Status,
	}
	if err := store.db.WithContext(ctx).Table("menus").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// UpdateMenu 更新菜单节点的可编辑字段。
func (store *GormStore) UpdateMenu(ctx context.Context, menuID uint64, mutation MenuMutation) error {
	updates := map[string]any{
		"parent_id": mutation.ParentID, "name": mutation.Name, "path": mutation.Path,
		"component": mutation.Component, "icon": mutation.Icon, "permission_code": mutation.PermissionCode,
		"tenant_assignable": mutation.TenantAssignable, "sort": mutation.Sort, "visible": mutation.Visible,
	}
	return store.db.WithContext(ctx).Table("menus").Where("id = ?", menuID).Updates(updates).Error
}

// SetMenuStatus 更新菜单节点状态。
func (store *GormStore) SetMenuStatus(ctx context.Context, menuID uint64, status uint8) error {
	return store.db.WithContext(ctx).Table("menus").Where("id = ?", menuID).Update("status", status).Error
}

// DeleteMenu 删除菜单节点。
func (store *GormStore) DeleteMenu(ctx context.Context, menuID uint64) error {
	return store.db.WithContext(ctx).Table("menus").Where("id = ?", menuID).Delete(nil).Error
}

// FindMenuForUpdate 读取菜单写操作需要的完整字段。
func (store *GormStore) FindMenuForUpdate(ctx context.Context, menuID uint64) (*PlatformMenu, error) {
	var menu PlatformMenu
	err := store.db.WithContext(ctx).Table("menus").Where("id = ?", menuID).Take(&menu).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errManagementNotFound
	}
	return &menu, err
}

// MenuChildCount 查询指定菜单节点的直接子节点数量。
func (store *GormStore) MenuChildCount(ctx context.Context, menuID uint64, enabledOnly bool) (int64, error) {
	var count int64
	query := store.db.WithContext(ctx).Table("menus").Where("parent_id = ?", menuID)
	if enabledOnly {
		query = query.Where("status = ?", 1)
	}
	return count, query.Count(&count).Error
}

// RoleMenuRelationCount 查询普通角色关联指定菜单节点的数量。
func (store *GormStore) RoleMenuRelationCount(ctx context.Context, menuID uint64) (int64, error) {
	var count int64
	err := store.db.WithContext(ctx).Table("role_menus").Where("menu_id = ?", menuID).Count(&count).Error
	return count, err
}

// ManagedMenuRelationCount 查询平台角色代管权限关联指定菜单节点的数量。
func (store *GormStore) ManagedMenuRelationCount(ctx context.Context, menuID uint64) (int64, error) {
	var count int64
	err := store.db.WithContext(ctx).Table("platform_role_tenant_menus").Where("menu_id = ?", menuID).Count(&count).Error
	return count, err
}

// AssignMenuToTenantOwners 把可分配租户节点自动授予全部企业管理员。
func (store *GormStore) AssignMenuToTenantOwners(ctx context.Context, menuID uint64) error {
	return assignMenuToTenantOwners(store.db.WithContext(ctx), menuID)
}

// validateMenuParent 校验父节点范围、类型、启用状态和循环关系。
func (service *MenuService) validateMenuParent(ctx context.Context, store MenuTransactionStore, mutation MenuMutation, currentID *uint64) error {
	if mutation.ParentID == nil {
		if mutation.Type == "permission" {
			return errManagementConflict
		}
		return nil
	}
	if currentID != nil && *mutation.ParentID == *currentID {
		return errManagementConflict
	}
	parent, err := store.FindMenuForUpdate(ctx, *mutation.ParentID)
	if err != nil {
		return err
	}
	if parent.Scope != mutation.Scope || (mutation.Type == "permission" && parent.Type != "menu") || (mutation.Type != "permission" && parent.Type != "directory") {
		return errManagementConflict
	}
	if mutation.Scope == "platform" && mutation.Type == "permission" {
		parentReserved := isPlatformProtectedPermissionGroup(parent.PermissionCode)
		mutationReserved := isPlatformProtectedPermissionGroup(mutation.PermissionCode)
		if parentReserved != mutationReserved {
			return errManagementConflict
		}
		if parentReserved && !sameReservedPermissionGroup(parent.PermissionCode, mutation.PermissionCode) {
			return errManagementConflict
		}
	}
	if mutation.Status == 1 && parent.Status != 1 {
		return errManagementConflict
	}
	// 与部门树相同，编辑菜单时沿父节点向上检查，防止把节点挂到自己的子树下。
	for currentID != nil && parent.ParentID != nil {
		if *parent.ParentID == *currentID {
			return errManagementConflict
		}
		parent, err = store.FindMenuForUpdate(ctx, *parent.ParentID)
		if err != nil {
			return err
		}
		if parent.Scope != mutation.Scope {
			return errManagementConflict
		}
	}
	return nil
}

// sameReservedPermissionGroup 校验保留权限父子节点属于同一菜单或字段分组。
func sameReservedPermissionGroup(parentCode, childCode *string) bool {
	if parentCode == nil || childCode == nil {
		return false
	}
	for _, prefix := range []string{"platform:menu:", "platform:field:", "platform:miniapp:", "platform:system-log:"} {
		if strings.HasPrefix(*parentCode, prefix) {
			return strings.HasPrefix(*childCode, prefix)
		}
	}
	return false
}

// isPlatformProtectedPermissionGroup 判断权限是否属于需要保持父子编码一致的核心分组。
func isPlatformProtectedPermissionGroup(permissionCode *string) bool {
	return permissionCode != nil && (strings.HasPrefix(*permissionCode, "platform:menu:") || strings.HasPrefix(*permissionCode, "platform:field:") || strings.HasPrefix(*permissionCode, "platform:miniapp:") || *permissionCode == "platform:system-log:view")
}

// isProtectedPlatformMenu 判断节点是否属于防止管理入口自锁的核心菜单权限。
func isProtectedPlatformMenu(menu PlatformMenu) bool {
	return menu.Scope == "platform" && menu.PermissionCode != nil && strings.HasPrefix(*menu.PermissionCode, "platform:menu:")
}

// protectedMenuFieldsUnchanged 校验核心菜单的关键定位和权限字段未被修改。
func protectedMenuFieldsUnchanged(existing PlatformMenu, mutation MenuMutation) bool {
	return equalUint64Pointer(existing.ParentID, mutation.ParentID) && equalStringPointer(existing.Path, mutation.Path) &&
		equalStringPointer(existing.Component, mutation.Component) && equalStringPointer(existing.PermissionCode, mutation.PermissionCode) &&
		existing.TenantAssignable == mutation.TenantAssignable && existing.Visible == mutation.Visible
}

// equalUint64Pointer 比较两个可选 BIGINT 值。
func equalUint64Pointer(left, right *uint64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

// equalStringPointer 比较两个可选字符串值。
func equalStringPointer(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}
