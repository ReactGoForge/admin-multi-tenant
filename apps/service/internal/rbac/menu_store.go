package rbac

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

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
