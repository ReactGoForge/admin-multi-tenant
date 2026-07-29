package rbac

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// tenantRecord 映射租户管理查询和写入所需的数据库字段。
type tenantRecord struct {
	ID              uint64  `gorm:"column:id"`
	Name            string  `gorm:"column:name"`
	Remark          *string `gorm:"column:remark"`
	IconURL         *string `gorm:"column:icon_url"`
	Status          uint8   `gorm:"column:status"`
	OwnerEmployeeID *uint64 `gorm:"column:owner_employee_id"`
	OwnerName       *string `gorm:"column:owner_name;->"`
	LoginAccount    *string `gorm:"column:login_account;->"`
}

// tenantOwnerRecord 映射租户所有者校验所需的数据库字段。
type tenantOwnerRecord struct {
	ID              uint64  `gorm:"column:id"`
	Status          uint8   `gorm:"column:status"`
	OwnerEmployeeID *uint64 `gorm:"column:owner_employee_id"`
}

// ListTenants 按分页和筛选条件查询平台租户列表。
func (store *GormStore) ListTenants(ctx context.Context, query TenantQuery) ([]PlatformTenant, int64, error) {
	base := store.db.WithContext(ctx).Table("tenants AS t")
	if query.Name != "" {
		base = base.Where("LOCATE(?, t.name) > 0", query.Name)
	}
	if query.Status != nil {
		base = base.Where("t.status = ?", *query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]tenantRecord, 0)
	err := base.
		Select("t.id", "t.name", "t.remark", "t.icon_url", "t.status", "t.owner_employee_id", "e.name AS owner_name", "e.login_account").
		Joins("LEFT JOIN employees AS e ON e.id = t.owner_employee_id").
		Order("t.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	tenants := make([]PlatformTenant, 0, len(rows))
	for _, row := range rows {
		tenants = append(tenants, PlatformTenant{
			ID: row.ID, Name: row.Name, Remark: row.Remark, IconURL: row.IconURL,
			Status: row.Status, OwnerEmployeeID: row.OwnerEmployeeID,
			OwnerName: row.OwnerName, LoginAccount: row.LoginAccount,
		})
	}
	return tenants, total, nil
}

// WithTenantTransaction 在单个数据库事务内执行租户生命周期写流程。
func (store *GormStore) WithTenantTransaction(ctx context.Context, fn func(TenantTransactionStore) error) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GormStore{db: tx})
	})
}

// CreateTenantRecord 写入租户主记录并返回新租户 ID。
func (store *GormStore) CreateTenantRecord(ctx context.Context, name string) (uint64, error) {
	row := tenantRecord{Name: name, Status: 1}
	if err := store.db.WithContext(ctx).Table("tenants").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// CreateTenantOwnerRole 写入租户企业管理员角色并返回角色 ID。
func (store *GormStore) CreateTenantOwnerRole(ctx context.Context, tenantID uint64) (uint64, error) {
	key := tenantOwnerSystemKey
	role := roleInsertRow{Scope: "tenant", TenantID: &tenantID, Name: tenantOwnerDisplayName, SystemKey: &key, Status: 1}
	if err := store.db.WithContext(ctx).Table("roles").Create(&role).Error; err != nil {
		return 0, err
	}
	return role.ID, nil
}

// AssignDefaultTenantOwnerPermissions 写入企业管理员默认权限快照。
func (store *GormStore) AssignDefaultTenantOwnerPermissions(ctx context.Context, tenantID, roleID uint64) error {
	return assignDefaultTenantOwnerPermissions(store.db.WithContext(ctx), tenantID, roleID)
}

// CreateTenantOwnerEmployee 写入租户所有者员工并返回员工 ID。
func (store *GormStore) CreateTenantOwnerEmployee(ctx context.Context, tenantID uint64, input TenantCreateInput, passwordHash string) (uint64, error) {
	employee := employeeInsertRow{Scope: "tenant", TenantID: &tenantID, Name: input.OwnerName, LoginAccount: input.LoginAccount, PasswordHash: passwordHash, Status: 1}
	if err := store.db.WithContext(ctx).Table("employees").Create(&employee).Error; err != nil {
		return 0, err
	}
	return employee.ID, nil
}

// CreateTenantOwnerRelation 写入租户所有者与企业管理员角色关联。
func (store *GormStore) CreateTenantOwnerRelation(ctx context.Context, tenantID, ownerEmployeeID, ownerRoleID uint64) error {
	return store.db.WithContext(ctx).Table("employee_roles").Create(map[string]any{"scope": "tenant", "tenant_id": tenantID, "employee_id": ownerEmployeeID, "role_id": ownerRoleID}).Error
}

// SetTenantOwner 回写租户所有者员工 ID。
func (store *GormStore) SetTenantOwner(ctx context.Context, tenantID, ownerEmployeeID uint64) error {
	return store.db.WithContext(ctx).Table("tenants").Where("id = ?", tenantID).Update("owner_employee_id", ownerEmployeeID).Error
}

// FindTenantOwner 查询租户所有者字段，可按删除场景锁定租户行。
func (store *GormStore) FindTenantOwner(ctx context.Context, tenantID uint64, forUpdate bool) (*tenantOwnerRecord, error) {
	query := store.db.WithContext(ctx).Table("tenants").Select("id", "status", "owner_employee_id").Where("id = ?", tenantID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var tenant tenantOwnerRecord
	result := query.Take(&tenant)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errManagementNotFound
	}
	return &tenant, result.Error
}

// EnsureTenantOwnerEmployee 确认租户所有者员工仍属于该租户范围。
func (store *GormStore) EnsureTenantOwnerEmployee(ctx context.Context, tenantID, ownerEmployeeID uint64) error {
	var count int64
	err := store.db.WithContext(ctx).Table("employees").
		Where("id = ? AND scope = ? AND tenant_id = ?", ownerEmployeeID, "tenant", tenantID).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return errManagementConflict
	}
	return nil
}

// UpdateTenantAndOwner 更新租户基础信息和当前所有者登录账号。
func (store *GormStore) UpdateTenantAndOwner(ctx context.Context, tenantID, ownerEmployeeID uint64, input TenantUpdateInput) error {
	db := store.db.WithContext(ctx)
	if err := db.Table("tenants").Where("id = ?", tenantID).Updates(map[string]any{"name": input.Name, "remark": input.Remark}).Error; err != nil {
		return err
	}
	return db.Table("employees").Where("id = ?", ownerEmployeeID).Update("login_account", input.LoginAccount).Error
}

// UpdateTenantOwnerPassword 更新租户所有者密码哈希并保持原有会话字段语义。
func (store *GormStore) UpdateTenantOwnerPassword(ctx context.Context, tenantID, ownerEmployeeID uint64, passwordHash string) (bool, error) {
	result := store.db.WithContext(ctx).Table("employees").
		Where("id = ? AND scope = ? AND tenant_id = ?", ownerEmployeeID, "tenant", tenantID).
		Update("password_hash", passwordHash)
	return result.RowsAffected > 0, result.Error
}

// UpdateTenantStatus 更新租户状态并返回是否有记录被数据库报告为更新。
func (store *GormStore) UpdateTenantStatus(ctx context.Context, tenantID uint64, status uint8) (bool, error) {
	result := store.db.WithContext(ctx).Table("tenants").Where("id = ?", tenantID).Update("status", status)
	return result.RowsAffected > 0, result.Error
}

// EnsureTenantExists 确认租户主记录存在。
func (store *GormStore) EnsureTenantExists(ctx context.Context, tenantID uint64) error {
	var count int64
	if err := store.db.WithContext(ctx).Table("tenants").Where("id = ?", tenantID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errManagementNotFound
	}
	return nil
}

// ValidateEmptyTenantForDelete 校验租户仅包含初始化所有者、企业管理员角色和默认关联。
func (store *GormStore) ValidateEmptyTenantForDelete(ctx context.Context, tenantID, ownerEmployeeID uint64) (uint64, error) {
	scope := managementScope{Name: "tenant", TenantID: &tenantID}
	return validateEmptyTenantForDelete(store.db.WithContext(ctx), scope, ownerEmployeeID)
}

// DeleteTenantBootstrapRecords 按外键约束顺序删除空租户初始化数据。
func (store *GormStore) DeleteTenantBootstrapRecords(ctx context.Context, tenantID, ownerEmployeeID, ownerRoleID uint64) error {
	scope := managementScope{Name: "tenant", TenantID: &tenantID}
	db := store.db.WithContext(ctx)
	if err := scopedTable(db.Table("role_menus"), scope).Where("role_id = ?", ownerRoleID).Delete(nil).Error; err != nil {
		return err
	}
	if err := scopedTable(db.Table("employee_roles"), scope).Where("employee_id = ?", ownerEmployeeID).Delete(nil).Error; err != nil {
		return err
	}
	if err := db.Table("tenants").Where("id = ?", tenantID).Update("owner_employee_id", gorm.Expr("NULL")).Error; err != nil {
		return err
	}
	if err := scopedTable(db.Table("employees"), scope).Where("id = ?", ownerEmployeeID).Delete(nil).Error; err != nil {
		return err
	}
	if err := scopedTable(db.Table("roles"), scope).Where("id = ?", ownerRoleID).Delete(nil).Error; err != nil {
		return err
	}
	return db.Table("tenants").Where("id = ?", tenantID).Delete(nil).Error
}

// validateEmptyTenantForDelete 校验租户删除前不存在初始化数据之外的业务引用。
func validateEmptyTenantForDelete(db *gorm.DB, scope managementScope, ownerEmployeeID uint64) (uint64, error) {
	if scope.TenantID == nil {
		return 0, errManagementConflict
	}
	var count int64
	if err := scopedTable(db.Table("departments"), scope).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 0 {
		return 0, errManagementConflict
	}
	if err := scopedTable(db.Table("employees"), scope).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 1 {
		return 0, errManagementConflict
	}
	if err := scopedTable(db.Table("employees"), scope).Where("id = ?", ownerEmployeeID).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 1 {
		return 0, errManagementConflict
	}
	var ownerRole roleInsertRow
	result := scopedTable(db.Table("roles"), scope).
		Select("id", "system_key").
		Where("system_key = ?", tenantOwnerSystemKey).
		Take(&ownerRole)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return 0, errManagementConflict
	}
	if result.Error != nil {
		return 0, result.Error
	}
	if err := scopedTable(db.Table("roles"), scope).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 1 {
		return 0, errManagementConflict
	}
	if err := scopedTable(db.Table("employee_roles"), scope).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 1 {
		return 0, errManagementConflict
	}
	if err := scopedTable(db.Table("employee_roles"), scope).Where("employee_id = ? AND role_id = ?", ownerEmployeeID, ownerRole.ID).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 1 {
		return 0, errManagementConflict
	}
	if err := db.Table("tenant_users").Where("tenant_id = ?", *scope.TenantID).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 0 {
		return 0, errManagementConflict
	}
	if err := db.Table("image_assets").Where("tenant_id = ?", *scope.TenantID).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 0 {
		return 0, errManagementConflict
	}
	if err := db.Table("image_categories").Where("tenant_id = ?", *scope.TenantID).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != 0 {
		return 0, errManagementConflict
	}
	return ownerRole.ID, nil
}
