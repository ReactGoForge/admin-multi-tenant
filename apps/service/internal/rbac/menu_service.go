package rbac

import (
	"context"
	"regexp"
	"strings"
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
