package rbac

import "context"

// ListDepartments 查询部门、有效负责人和部门员工数量。
func (store *GormStore) ListDepartments(ctx context.Context, scope managementScope) ([]PlatformDepartment, error) {
	departments := make([]PlatformDepartment, 0)
	query := scopedAlias(store.db.WithContext(ctx).Table("departments AS d"), scope, "d")
	tenantCondition := "IS NULL"
	args := []any{scope.Name}
	if scope.TenantID != nil {
		tenantCondition = "= ?"
		args = append(args, *scope.TenantID)
	}
	selectSQL := `d.id,
			d.parent_id,
			d.name,
			d.leader_employee_id,
			leader.name AS leader_name,
			d.sort,
			d.status,
			(
				SELECT COUNT(*)
				FROM employees AS counted_e
				WHERE counted_e.department_id = d.id
					AND counted_e.scope = ?
					AND counted_e.tenant_id ` + tenantCondition + `
			) AS employee_count`
	joinSQL := "LEFT JOIN employees AS leader ON leader.id = d.leader_employee_id AND leader.scope = ? AND leader.tenant_id " + tenantCondition
	err := query.
		Select(selectSQL, args...).
		Joins(joinSQL, args...).
		Order("d.sort ASC, d.id ASC").
		Scan(&departments).Error
	if err != nil {
		return nil, err
	}
	return departments, nil
}

// ListPlatformDepartments 查询平台部门、有效平台负责人和部门员工数量。
func (store *GormStore) ListPlatformDepartments(ctx context.Context) ([]PlatformDepartment, error) {
	return store.ListDepartments(ctx, managementScope{Name: "platform"})
}

// ValidateDepartmentLeader 校验可选负责人属于当前工作空间。
func (store *GormStore) ValidateDepartmentLeader(ctx context.Context, scope managementScope, employeeID *uint64) error {
	return validateEmployee(store.db.WithContext(ctx), scope, employeeID)
}

// CreateDepartment 写入部门主记录。
func (store *GormStore) CreateDepartment(ctx context.Context, scope managementScope, mutation DepartmentMutation) error {
	row := departmentInsertRow{
		Scope:            scope.Name,
		TenantID:         scope.TenantID,
		ParentID:         mutation.ParentID,
		Name:             mutation.Name,
		LeaderEmployeeID: mutation.LeaderEmployeeID,
		Sort:             mutation.Sort,
		Status:           mutation.Status,
	}
	return store.db.WithContext(ctx).Table("departments").Create(&row).Error
}

// UpdateDepartment 更新部门基础字段并返回是否有记录被更新。
func (store *GormStore) UpdateDepartment(ctx context.Context, scope managementScope, departmentID uint64, mutation DepartmentMutation) (bool, error) {
	result := scopedTable(store.db.WithContext(ctx).Table("departments"), scope).
		Where("id = ?", departmentID).
		Updates(map[string]any{
			"parent_id":          mutation.ParentID,
			"name":               mutation.Name,
			"leader_employee_id": mutation.LeaderEmployeeID,
			"sort":               mutation.Sort,
			"status":             mutation.Status,
		})
	return result.RowsAffected > 0, result.Error
}

// EnsureDepartmentExists 确认部门仍存在于当前工作空间。
func (store *GormStore) EnsureDepartmentExists(ctx context.Context, scope managementScope, departmentID uint64) error {
	return ensureScopedExists(store.db.WithContext(ctx), "departments", scope, departmentID)
}

// DepartmentChildCount 查询指定部门的直接子部门数量。
func (store *GormStore) DepartmentChildCount(ctx context.Context, scope managementScope, departmentID uint64) (int64, error) {
	var count int64
	err := scopedTable(store.db.WithContext(ctx).Table("departments"), scope).
		Where("parent_id = ?", departmentID).
		Count(&count).Error
	return count, err
}

// DepartmentEmployeeCount 查询指定部门下的员工数量。
func (store *GormStore) DepartmentEmployeeCount(ctx context.Context, scope managementScope, departmentID uint64) (int64, error) {
	var count int64
	err := scopedTable(store.db.WithContext(ctx).Table("employees"), scope).
		Where("department_id = ?", departmentID).
		Count(&count).Error
	return count, err
}

// DeleteDepartment 删除部门主记录并返回是否有记录被删除。
func (store *GormStore) DeleteDepartment(ctx context.Context, scope managementScope, departmentID uint64) (bool, error) {
	result := scopedTable(store.db.WithContext(ctx).Table("departments"), scope).
		Where("id = ?", departmentID).
		Delete(nil)
	return result.RowsAffected > 0, result.Error
}
