package rbac

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const platformSuperAdminKey = "platform_super_admin"

// PlatformEmployeeQuery 描述平台员工列表已经校验过的分页与筛选条件。
type PlatformEmployeeQuery struct {
	Page                       int
	PageSize                   int
	Name                       string
	LoginAccount               string
	DepartmentID               *uint64
	RoleID                     *uint64
	Status                     *uint8
	VisibleProtectedEmployeeID *uint64
}

// EmployeeRole 描述员工列表中展示的角色。
type EmployeeRole struct {
	ID         uint64
	Name       string
	Assignable bool
}

// PlatformEmployee 描述平台员工列表需要返回的只读字段。
type PlatformEmployee struct {
	ID             uint64
	Name           string
	LoginAccount   string
	DepartmentID   *uint64
	DepartmentName *string
	Roles          []EmployeeRole
	Phone          *string
	Status         uint8
	CreatedAt      time.Time
}

// EmployeeOption 描述平台员工筛选用的角色或部门选项。
type EmployeeOption struct {
	ID         uint64
	Name       string
	Status     uint8
	Assignable bool
}

// PlatformEmployeeOptions 汇总平台员工筛选所需的角色与部门。
type PlatformEmployeeOptions struct {
	Roles       []EmployeeOption
	Departments []EmployeeOption
}

// EmployeeStore 定义平台员工只读接口需要的最小查询能力。
type EmployeeStore interface {
	ListPlatformEmployees(context.Context, PlatformEmployeeQuery) ([]PlatformEmployee, int64, error)
	ListPlatformEmployeeOptions(context.Context) (PlatformEmployeeOptions, error)
}

// GormStore 使用 GORM 执行现有 RBAC 表的只读查询。
type GormStore struct {
	db *gorm.DB
}

// NewStore 创建平台 RBAC 只读查询对象。
func NewStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

// employeeRow 映射员工列表主查询返回的数据库字段。
type employeeRow struct {
	ID             uint64    `gorm:"column:id"`
	Name           string    `gorm:"column:name"`
	LoginAccount   string    `gorm:"column:login_account"`
	DepartmentID   *uint64   `gorm:"column:department_id"`
	DepartmentName *string   `gorm:"column:department_name"`
	Phone          *string   `gorm:"column:phone"`
	Status         uint8     `gorm:"column:status"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

// employeeRoleRow 映射当前页员工关联角色的批量查询结果。
type employeeRoleRow struct {
	EmployeeID uint64 `gorm:"column:employee_id"`
	ID         uint64 `gorm:"column:id"`
	Name       string `gorm:"column:name"`
}

// ListPlatformEmployees 按筛选条件分页查询平台员工，并批量补充当前页角色。
func (store *GormStore) ListPlatformEmployees(ctx context.Context, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	return store.ListEmployees(ctx, managementScope{Name: "platform"}, query)
}

// ListEmployees 按筛选条件分页查询指定工作空间员工，并批量补充当前页角色。
func (store *GormStore) ListEmployees(ctx context.Context, scope managementScope, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	baseQuery := scopedAlias(store.db.WithContext(ctx).
		Table("employees AS e").
		Select("e.id"), scope, "e")
	if scope.Name == "platform" {
		protectedEmployeeFilter := `NOT EXISTS (
			SELECT 1
			FROM employee_roles AS excluded_er
			JOIN roles AS excluded_r ON excluded_r.id = excluded_er.role_id
			WHERE excluded_er.employee_id = e.id
				AND excluded_er.scope = ?
				AND excluded_er.tenant_id IS NULL
				AND excluded_r.scope = ?
				AND excluded_r.tenant_id IS NULL
				AND excluded_r.system_key = ?
		)`
		if query.VisibleProtectedEmployeeID != nil {
			baseQuery = baseQuery.Where(
				"(e.id = ? OR "+protectedEmployeeFilter+")",
				*query.VisibleProtectedEmployeeID,
				"platform",
				"platform",
				platformSuperAdminKey,
			)
		} else {
			baseQuery = baseQuery.Where(protectedEmployeeFilter, "platform", "platform", platformSuperAdminKey)
		}
	}

	if query.Name != "" {
		baseQuery = baseQuery.Where("LOCATE(?, e.name) > 0", query.Name)
	}
	if query.LoginAccount != "" {
		baseQuery = baseQuery.Where("LOCATE(?, e.login_account) > 0", query.LoginAccount)
	}
	if query.DepartmentID != nil {
		baseQuery = baseQuery.Where("e.department_id = ?", *query.DepartmentID)
	}
	if query.RoleID != nil {
		roleQuery := `EXISTS (
			SELECT 1
			FROM employee_roles AS filtered_er
			JOIN roles AS filtered_r ON filtered_r.id = filtered_er.role_id
			WHERE filtered_er.employee_id = e.id
				AND filtered_er.scope = ?
				AND filtered_er.role_id = ?
				AND filtered_r.scope = ?
		`
		args := []any{scope.Name, *query.RoleID, scope.Name}
		if scope.TenantID == nil {
			roleQuery += " AND filtered_er.tenant_id IS NULL AND filtered_r.tenant_id IS NULL"
		} else {
			roleQuery += " AND filtered_er.tenant_id = ? AND filtered_r.tenant_id = ?"
			args = append(args, *scope.TenantID, *scope.TenantID)
		}
		roleQuery += ")"
		baseQuery = baseQuery.Where(roleQuery, args...)
	}
	if query.Status != nil {
		baseQuery = baseQuery.Where("e.status = ?", *query.Status)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []employeeRow
	offset := (query.Page - 1) * query.PageSize
	join := "LEFT JOIN departments AS d ON d.id = e.department_id AND d.scope = ?"
	joinArgs := []any{scope.Name}
	if scope.TenantID == nil {
		join += " AND d.tenant_id IS NULL"
	} else {
		join += " AND d.tenant_id = ?"
		joinArgs = append(joinArgs, *scope.TenantID)
	}
	if err := baseQuery.
		Select("e.id", "e.name", "e.login_account", "e.department_id", "d.name AS department_name", "e.phone", "e.status", "e.created_at").
		Joins(join, joinArgs...).
		Order("e.id DESC").
		Limit(query.PageSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	employees := make([]PlatformEmployee, 0, len(rows))
	employeeIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		employeeIDs = append(employeeIDs, row.ID)
		employees = append(employees, PlatformEmployee{
			ID:             row.ID,
			Name:           row.Name,
			LoginAccount:   row.LoginAccount,
			DepartmentID:   row.DepartmentID,
			DepartmentName: row.DepartmentName,
			Roles:          make([]EmployeeRole, 0),
			Phone:          row.Phone,
			Status:         row.Status,
			CreatedAt:      row.CreatedAt,
		})
	}
	if len(employeeIDs) == 0 {
		return employees, total, nil
	}

	var roleRows []employeeRoleRow
	roleDB := scopedAlias(store.db.WithContext(ctx).
		Table("employee_roles AS er").
		Select("er.employee_id", "r.id", "r.name").
		Joins("JOIN roles AS r ON r.id = er.role_id AND r.scope = er.scope").
		Where("er.employee_id IN ?", employeeIDs), scope, "er")
	if scope.TenantID == nil {
		roleDB = roleDB.Where("r.tenant_id IS NULL")
	} else {
		roleDB = roleDB.Where("r.tenant_id = ?", *scope.TenantID)
	}
	if err := roleDB.
		Order("er.employee_id ASC, r.id ASC").
		Scan(&roleRows).Error; err != nil {
		return nil, 0, err
	}

	employeeIndexes := make(map[uint64]int, len(employees))
	for index, employee := range employees {
		employeeIndexes[employee.ID] = index
	}
	for _, roleRow := range roleRows {
		index, exists := employeeIndexes[roleRow.EmployeeID]
		if !exists {
			continue
		}
		employees[index].Roles = append(employees[index].Roles, EmployeeRole{ID: roleRow.ID, Name: roleRow.Name})
	}

	return employees, total, nil
}

// ListPlatformEmployeeOptions 查询平台员工筛选使用的角色与部门，并排除平台超级管理员角色。
func (store *GormStore) ListPlatformEmployeeOptions(ctx context.Context) (PlatformEmployeeOptions, error) {
	return store.ListEmployeeOptions(ctx, managementScope{Name: "platform"})
}

// ListEmployeeOptions 查询指定工作空间员工筛选使用的角色与部门。
func (store *GormStore) ListEmployeeOptions(ctx context.Context, scope managementScope) (PlatformEmployeeOptions, error) {
	options := PlatformEmployeeOptions{
		Roles:       make([]EmployeeOption, 0),
		Departments: make([]EmployeeOption, 0),
	}
	roles := scopedTable(store.db.WithContext(ctx).
		Table("roles"), scope).
		Select("id", "name", "status").
		Where("system_key IS NULL")
	if scope.Name == "platform" {
		roles = scopedTable(store.db.WithContext(ctx).
			Table("roles"), scope).
			Select("id", "name", "status").
			Where("system_key IS NULL OR system_key <> ?", platformSuperAdminKey)
	}
	if err := roles.Order("id ASC").
		Scan(&options.Roles).Error; err != nil {
		return PlatformEmployeeOptions{}, err
	}
	if err := scopedTable(store.db.WithContext(ctx).
		Table("departments"), scope).
		Select("id", "name", "status").
		Order("sort ASC, id ASC").
		Scan(&options.Departments).Error; err != nil {
		return PlatformEmployeeOptions{}, err
	}
	return options, nil
}

// WithEmployeeTransaction 在单个数据库事务内执行 Employee 写流程。
func (store *GormStore) WithEmployeeTransaction(ctx context.Context, fn func(EmployeeTransactionStore) error) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GormStore{db: tx})
	})
}

// ValidateDepartment 校验可选部门属于当前工作空间。
func (store *GormStore) ValidateDepartment(ctx context.Context, scope managementScope, departmentID *uint64, currentID *uint64) error {
	return validateDepartment(store.db.WithContext(ctx), scope, departmentID, currentID)
}

// ValidateEmployeeRoles 校验角色属于当前范围且不是受保护的所有者角色。
func (store *GormStore) ValidateEmployeeRoles(ctx context.Context, scope managementScope, roleIDs []uint64) error {
	var count int64
	query := scopedTable(store.db.WithContext(ctx).Table("roles"), scope).Where("id IN ?", roleIDs)
	if scope.Name == "platform" {
		query = query.Where("system_key IS NULL OR system_key = ?", "platform_admin")
	} else {
		query = query.Where("system_key IS NULL")
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(roleIDs)) {
		return errManagementProtected
	}
	return nil
}

// CreateEmployee 写入员工主记录并返回新员工 ID。
func (store *GormStore) CreateEmployee(ctx context.Context, scope managementScope, create EmployeeCreate) (uint64, error) {
	row := employeeInsertRow{Scope: scope.Name, TenantID: scope.TenantID, DepartmentID: create.DepartmentID, Name: create.Name, LoginAccount: create.LoginAccount, PasswordHash: create.PasswordHash, Phone: create.Phone, Status: create.Status}
	if err := store.db.WithContext(ctx).Table("employees").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// UpdateEmployee 更新员工基础资料并返回是否有记录被更新。
func (store *GormStore) UpdateEmployee(ctx context.Context, scope managementScope, employeeID uint64, update EmployeeUpdate) (bool, error) {
	result := scopedTable(store.db.WithContext(ctx).Table("employees"), scope).
		Where("id = ?", employeeID).
		Updates(map[string]any{"name": update.Name, "login_account": update.LoginAccount, "phone": update.Phone, "department_id": update.DepartmentID})
	return result.RowsAffected > 0, result.Error
}

// ResetEmployeePassword 更新员工密码哈希并返回是否有记录被更新。
func (store *GormStore) ResetEmployeePassword(ctx context.Context, scope managementScope, employeeID uint64, passwordHash string) (bool, error) {
	result := scopedTable(store.db.WithContext(ctx).Table("employees"), scope).
		Where("id = ?", employeeID).
		Update("password_hash", passwordHash)
	return result.RowsAffected > 0, result.Error
}

// SetEmployeeStatus 更新员工状态并返回是否有记录被更新。
func (store *GormStore) SetEmployeeStatus(ctx context.Context, scope managementScope, employeeID uint64, status uint8) (bool, error) {
	result := scopedTable(store.db.WithContext(ctx).Table("employees"), scope).
		Where("id = ?", employeeID).
		Update("status", status)
	return result.RowsAffected > 0, result.Error
}

// EnsureEmployeeExists 确认员工仍存在于当前工作空间。
func (store *GormStore) EnsureEmployeeExists(ctx context.Context, scope managementScope, employeeID uint64) error {
	return ensureScopedExists(store.db.WithContext(ctx), "employees", scope, employeeID)
}

// ReplaceEmployeeRoles 替换员工角色关联。
func (store *GormStore) ReplaceEmployeeRoles(ctx context.Context, scope managementScope, employeeID uint64, roleIDs []uint64) error {
	tx := store.db.WithContext(ctx)
	if err := scopedTable(tx.Table("employee_roles"), scope).Where("employee_id = ?", employeeID).Delete(nil).Error; err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		values := map[string]any{"scope": scope.Name, "employee_id": employeeID, "role_id": roleID}
		if scope.TenantID != nil {
			values["tenant_id"] = *scope.TenantID
		}
		if err := tx.Table("employee_roles").Create(values).Error; err != nil {
			return err
		}
	}
	return nil
}

// IsProtectedEmployee 判断员工是否关联内置所有者角色。
func (store *GormStore) IsProtectedEmployee(ctx context.Context, scope managementScope, employeeID uint64) (bool, error) {
	var count int64
	protectedKey := platformSuperAdminKey
	if scope.Name == "tenant" {
		protectedKey = tenantOwnerSystemKey
	}
	query := store.db.WithContext(ctx).Table("employee_roles er").
		Joins("JOIN roles r ON r.id=er.role_id").
		Where("er.employee_id=? AND er.scope=? AND r.scope=? AND r.system_key = ?", employeeID, scope.Name, scope.Name, protectedKey)
	if scope.TenantID == nil {
		query = query.Where("er.tenant_id IS NULL AND r.tenant_id IS NULL")
	} else {
		query = query.Where("er.tenant_id=? AND r.tenant_id=?", *scope.TenantID, *scope.TenantID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListEmployeeRoleIDs 查询员工当前角色关联。
func (store *GormStore) ListEmployeeRoleIDs(ctx context.Context, scope managementScope, employeeID uint64) ([]uint64, error) {
	roleIDs := make([]uint64, 0)
	err := scopedTable(store.db.WithContext(ctx).Table("employee_roles"), scope).
		Where("employee_id = ?", employeeID).
		Pluck("role_id", &roleIDs).Error
	return roleIDs, err
}

// FindEmployeeForDelete 锁定删除前需要校验的员工字段。
func (store *GormStore) FindEmployeeForDelete(ctx context.Context, scope managementScope, employeeID uint64) (employeeDeleteRow, error) {
	var employee employeeDeleteRow
	result := scopedTable(store.db.WithContext(ctx).Table("employees"), scope).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "status").
		Where("id = ?", employeeID).
		Take(&employee)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return employeeDeleteRow{}, errManagementNotFound
	}
	return employee, result.Error
}

// HasAnySystemRole 判断员工是否持有任意系统内置角色。
func (store *GormStore) HasAnySystemRole(ctx context.Context, scope managementScope, employeeID uint64) (bool, error) {
	var count int64
	query := store.db.WithContext(ctx).Table("employee_roles er").
		Joins("JOIN roles r ON r.id=er.role_id").
		Where("er.employee_id=? AND er.scope=? AND r.scope=? AND r.system_key IS NOT NULL", employeeID, scope.Name, scope.Name)
	if scope.TenantID == nil {
		query = query.Where("er.tenant_id IS NULL AND r.tenant_id IS NULL")
	} else {
		query = query.Where("er.tenant_id=? AND r.tenant_id=?", *scope.TenantID, *scope.TenantID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsTenantOwnerEmployee 判断员工是否仍被租户所有者字段引用。
func (store *GormStore) IsTenantOwnerEmployee(ctx context.Context, scope managementScope, employeeID uint64) (bool, error) {
	if scope.Name != "tenant" || scope.TenantID == nil {
		return false, nil
	}
	var count int64
	if err := store.db.WithContext(ctx).Table("tenants").Where("id = ? AND owner_employee_id = ?", *scope.TenantID, employeeID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasEmployeeDeleteReference 检查会阻止员工物理删除的业务引用。
func (store *GormStore) HasEmployeeDeleteReference(ctx context.Context, scope managementScope, employeeID uint64) (bool, error) {
	var departmentLeaderCount int64
	if err := scopedTable(store.db.WithContext(ctx).Table("departments"), scope).Where("leader_employee_id = ?", employeeID).Count(&departmentLeaderCount).Error; err != nil {
		return false, err
	}
	if departmentLeaderCount > 0 {
		return true, nil
	}
	var imageUploaderCount int64
	if err := store.db.WithContext(ctx).Table("image_assets").Where("uploaded_by_employee_id = ?", employeeID).Count(&imageUploaderCount).Error; err != nil {
		return false, err
	}
	return imageUploaderCount > 0, nil
}

// DeleteEmployeeRoles 删除员工角色关联。
func (store *GormStore) DeleteEmployeeRoles(ctx context.Context, scope managementScope, employeeID uint64) error {
	return scopedTable(store.db.WithContext(ctx).Table("employee_roles"), scope).
		Where("employee_id = ?", employeeID).
		Delete(nil).Error
}

// DeleteEmployee 删除员工主记录。
func (store *GormStore) DeleteEmployee(ctx context.Context, scope managementScope, employeeID uint64) error {
	return scopedTable(store.db.WithContext(ctx).Table("employees"), scope).
		Where("id = ?", employeeID).
		Delete(nil).Error
}

// validateEmployee 校验可选员工属于当前工作空间。
func validateEmployee(db *gorm.DB, scope managementScope, employeeID *uint64) error {
	if employeeID == nil {
		return nil
	}
	var count int64
	if err := scopedTable(db.Table("employees"), scope).Where("id = ?", *employeeID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errManagementNotFound
	}
	return nil
}
