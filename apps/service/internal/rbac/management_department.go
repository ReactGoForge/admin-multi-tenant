package rbac

import "gorm.io/gorm"

// validateDepartment 校验父部门存在且不会形成循环。
func validateDepartment(db *gorm.DB, scope managementScope, departmentID *uint64, currentID *uint64) error {
	if departmentID == nil {
		return nil
	}
	if currentID != nil && *departmentID == *currentID {
		return errManagementConflict
	}
	var count int64
	if err := scopedTable(db.Table("departments"), scope).Where("id = ?", *departmentID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errManagementNotFound
	}
	if currentID != nil {
		// 业务约束：编辑部门时沿 parent_id 向上遍历；一旦遇到自身，说明会形成循环层级。
		parent := departmentID
		for parent != nil {
			if *parent == *currentID {
				return errManagementConflict
			}
			var row struct {
				ParentID *uint64 `gorm:"column:parent_id"`
			}
			result := scopedTable(db.Table("departments"), scope).Select("parent_id").Where("id = ?", *parent).Take(&row)
			if result.Error != nil {
				return result.Error
			}
			parent = row.ParentID
		}
	}
	return nil
}
