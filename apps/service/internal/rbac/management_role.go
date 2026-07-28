package rbac

import (
	"errors"

	"gorm.io/gorm"
)

// validatePermissions 校验权限节点范围、启用状态、租户可分配标记及角色类型限制。
func validatePermissions(db *gorm.DB, scope managementScope, permissionIDs []uint64, allowTenantMenuPermissions bool) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	// 业务约束：只有启用且范围匹配的菜单或操作节点可以保存为角色权限。
	query := db.Table("menus").Where("id IN ? AND scope = ? AND status = ?", permissionIDs, scope.Name, 1)
	if scope.Name == "tenant" {
		query = query.Where("tenant_assignable = ?", 1)
		if !allowTenantMenuPermissions {
			query = query.Where("permission_code IS NULL OR permission_code NOT LIKE ?", tenantMenuPermissionPrefix+"%")
		}
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(permissionIDs)) {
		return errManagementConflict
	}
	return nil
}

// validateManagedPermissions 校验平台角色选择的节点均为可分配租户权限。
func validateManagedPermissions(db *gorm.DB, permissionIDs []uint64) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	var count int64
	err := db.Table("menus").
		Where("id IN ? AND scope = ? AND status = ? AND tenant_assignable = ?", permissionIDs, "tenant", 1, 1).
		Where("permission_code IS NULL OR permission_code NOT LIKE ?", tenantMenuPermissionPrefix+"%").
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != int64(len(permissionIDs)) {
		return errManagementConflict
	}
	return nil
}

// replaceRolePermissions 替换角色菜单权限关联。
func replaceRolePermissions(tx *gorm.DB, scope managementScope, roleID uint64, permissionIDs []uint64) error {
	if err := scopedTable(tx.Table("role_menus"), scope).Where("role_id = ?", roleID).Delete(nil).Error; err != nil {
		return err
	}
	for _, menuID := range permissionIDs {
		values := map[string]any{"scope": scope.Name, "role_id": roleID, "menu_id": menuID}
		if scope.TenantID != nil {
			values["tenant_id"] = *scope.TenantID
		}
		if err := tx.Table("role_menus").Create(values).Error; err != nil {
			return err
		}
	}
	return nil
}

// replaceManagedPermissions 替换平台角色进入租户后的全部权限关联。
func replaceManagedPermissions(tx *gorm.DB, roleID uint64, permissionIDs []uint64) error {
	if err := tx.Table("platform_role_tenant_menus").Where("role_id = ?", roleID).Delete(nil).Error; err != nil {
		return err
	}
	for _, menuID := range permissionIDs {
		if err := tx.Table("platform_role_tenant_menus").Create(map[string]any{"role_id": roleID, "menu_id": menuID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ensureCustomRole 确认角色存在且不是内置角色。
func ensureCustomRole(db *gorm.DB, scope managementScope, roleID uint64) error {
	var row struct {
		SystemKey *string `gorm:"column:system_key"`
	}
	result := scopedTable(db.Table("roles"), scope).Select("system_key").Where("id = ?", roleID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return errManagementNotFound
	}
	if result.Error != nil {
		return result.Error
	}
	if row.SystemKey != nil {
		return errManagementProtected
	}
	return nil
}

// ensureAssignableRole 确认角色允许配置权限，并返回是否允许授予租户菜单管理权限。
func ensureAssignableRole(db *gorm.DB, scope managementScope, roleID uint64, platformSuperAdmin bool) (bool, error) {
	var row struct {
		SystemKey *string `gorm:"column:system_key"`
	}
	result := scopedTable(db.Table("roles"), scope).Select("system_key").Where("id = ?", roleID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, errManagementNotFound
	}
	if result.Error != nil {
		return false, result.Error
	}
	if err := validateRolePermissionAssignment(scope, row.SystemKey, platformSuperAdmin); err != nil {
		return false, err
	}
	return canAssignTenantMenuPermissions(scope, row.SystemKey, platformSuperAdmin), nil
}

// canAssignTenantMenuPermissions 判断目标角色是否为平台超级管理员正在配置的企业管理员。
func canAssignTenantMenuPermissions(scope managementScope, systemKey *string, platformSuperAdmin bool) bool {
	return scope.Name == "tenant" && systemKey != nil && *systemKey == tenantOwnerSystemKey && platformSuperAdmin
}

// validateRolePermissionAssignment 校验指定类型角色是否允许当前操作者配置权限。
func validateRolePermissionAssignment(scope managementScope, systemKey *string, platformSuperAdmin bool) error {
	if systemKey == nil {
		return nil
	}
	if scope.Name == "platform" && *systemKey == "platform_admin" {
		if platformSuperAdmin {
			return nil
		}
		return errManagementForbidden
	}
	if scope.Name == "tenant" && *systemKey == tenantOwnerSystemKey {
		if platformSuperAdmin {
			return nil
		}
		return errManagementForbidden
	}
	return errManagementProtected
}
