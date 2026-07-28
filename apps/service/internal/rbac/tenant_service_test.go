package rbac

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type testTenantStore struct {
	tx                 *testTenantTx
	updateStatusCalled bool
	statusChanged      bool
	ensureCalled       bool
	err                error
}

// ListTenants 返回空租户列表。
func (store *testTenantStore) ListTenants(context.Context, TenantQuery) ([]PlatformTenant, int64, error) {
	return nil, 0, store.err
}

// UpdateTenantStatus 记录状态更新调用并返回预设结果。
func (store *testTenantStore) UpdateTenantStatus(context.Context, uint64, uint8) (bool, error) {
	store.updateStatusCalled = true
	return store.statusChanged, store.err
}

// EnsureTenantExists 记录租户存在性兜底检查。
func (store *testTenantStore) EnsureTenantExists(context.Context, uint64) error {
	store.ensureCalled = true
	return store.err
}

// WithTenantTransaction 使用测试事务对象执行租户事务。
func (store *testTenantStore) WithTenantTransaction(ctx context.Context, fn func(TenantTransactionStore) error) error {
	if store.tx == nil {
		store.tx = &testTenantTx{}
	}
	return fn(store.tx)
}

type testTenantTx struct {
	owner                      tenantOwnerRecord
	ownerRoleID                uint64
	createTenantCalled         bool
	createOwnerRoleCalled      bool
	assignPermissionsCalled    bool
	createOwnerEmployeeCalled  bool
	createOwnerRelationCalled  bool
	setOwnerCalled             bool
	ensureOwnerEmployeeCalled  bool
	updateTenantAndOwnerCalled bool
	updatePasswordChanged      bool
	validateEmptyCalled        bool
	deleteBootstrapCalled      bool
	err                        error
}

// CreateTenantRecord 记录租户主记录创建调用。
func (tx *testTenantTx) CreateTenantRecord(context.Context, string) (uint64, error) {
	tx.createTenantCalled = true
	return 11, tx.err
}

// CreateTenantOwnerRole 记录企业管理员角色创建调用。
func (tx *testTenantTx) CreateTenantOwnerRole(context.Context, uint64) (uint64, error) {
	tx.createOwnerRoleCalled = true
	return 12, tx.err
}

// AssignDefaultTenantOwnerPermissions 记录默认权限快照写入调用。
func (tx *testTenantTx) AssignDefaultTenantOwnerPermissions(context.Context, uint64, uint64) error {
	tx.assignPermissionsCalled = true
	return tx.err
}

// CreateTenantOwnerEmployee 记录所有者员工创建调用。
func (tx *testTenantTx) CreateTenantOwnerEmployee(context.Context, uint64, TenantCreateInput, string) (uint64, error) {
	tx.createOwnerEmployeeCalled = true
	return 13, tx.err
}

// CreateTenantOwnerRelation 记录所有者角色关联创建调用。
func (tx *testTenantTx) CreateTenantOwnerRelation(context.Context, uint64, uint64, uint64) error {
	tx.createOwnerRelationCalled = true
	return tx.err
}

// SetTenantOwner 记录租户所有者回写调用。
func (tx *testTenantTx) SetTenantOwner(context.Context, uint64, uint64) error {
	tx.setOwnerCalled = true
	return tx.err
}

// FindTenantOwner 返回测试预设的租户所有者字段。
func (tx *testTenantTx) FindTenantOwner(context.Context, uint64, bool) (*tenantOwnerRecord, error) {
	if tx.err != nil {
		return nil, tx.err
	}
	return &tx.owner, nil
}

// EnsureTenantOwnerEmployee 记录所有者员工范围校验调用。
func (tx *testTenantTx) EnsureTenantOwnerEmployee(context.Context, uint64, uint64) error {
	tx.ensureOwnerEmployeeCalled = true
	return tx.err
}

// UpdateTenantAndOwner 记录租户和所有者账号更新调用。
func (tx *testTenantTx) UpdateTenantAndOwner(context.Context, uint64, uint64, TenantUpdateInput) error {
	tx.updateTenantAndOwnerCalled = true
	return tx.err
}

// UpdateTenantOwnerPassword 记录密码更新调用并返回预设更新结果。
func (tx *testTenantTx) UpdateTenantOwnerPassword(context.Context, uint64, uint64, string) (bool, error) {
	return tx.updatePasswordChanged, tx.err
}

// ValidateEmptyTenantForDelete 记录空租户删除校验调用。
func (tx *testTenantTx) ValidateEmptyTenantForDelete(context.Context, uint64, uint64) (uint64, error) {
	tx.validateEmptyCalled = true
	return tx.ownerRoleID, tx.err
}

// DeleteTenantBootstrapRecords 记录初始化数据删除调用。
func (tx *testTenantTx) DeleteTenantBootstrapRecords(context.Context, uint64, uint64, uint64) error {
	tx.deleteBootstrapCalled = true
	return tx.err
}

// TestTenantServiceCreateUsesSingleBootstrapFlow 验证创建租户时执行完整初始化事务步骤。
func TestTenantServiceCreateUsesSingleBootstrapFlow(t *testing.T) {
	tx := &testTenantTx{}
	service := NewTenantService(&testTenantStore{tx: tx})
	err := service.CreateTenant(context.Background(), TenantCreateInput{Name: "租户", OwnerName: "管理员", LoginAccount: "owner", Password: "123456"})
	if err != nil {
		t.Fatalf("创建租户服务错误: %v", err)
	}
	if !tx.createTenantCalled || !tx.createOwnerRoleCalled || !tx.assignPermissionsCalled || !tx.createOwnerEmployeeCalled || !tx.createOwnerRelationCalled || !tx.setOwnerCalled {
		t.Fatalf("租户初始化事务步骤不完整: %#v", tx)
	}
}

// TestCreateTenantRecordDoesNotWriteJoinedFields 验证创建租户时不会把列表联表字段写入 tenants。
func TestCreateTenantRecordDoesNotWriteJoinedFields(t *testing.T) {
	database, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(database)
	if _, err := store.CreateTenantRecord(context.Background(), "测试租户"); err != nil {
		t.Fatal(err)
	}
	statement := database.Statement.SQL.String()
	if strings.Contains(statement, "owner_name") || strings.Contains(statement, "login_account") {
		t.Fatalf("创建租户 SQL 不应包含联表字段: %s", statement)
	}
}

// TestTenantServiceStatusSameValueChecksExistence 验证相同状态未更新行时仍执行存在性检查。
func TestTenantServiceStatusSameValueChecksExistence(t *testing.T) {
	store := &testTenantStore{statusChanged: false}
	service := NewTenantService(store)
	if err := service.SetTenantStatus(context.Background(), 42, 1); err != nil {
		t.Fatalf("相同状态租户应在存在时成功: %v", err)
	}
	if !store.updateStatusCalled || !store.ensureCalled {
		t.Fatalf("相同状态未执行存在性兜底检查: %#v", store)
	}
}

// TestTenantServiceOwnerConsistency 验证编辑和重置密码都要求租户存在有效所有者。
func TestTenantServiceOwnerConsistency(t *testing.T) {
	service := NewTenantService(&testTenantStore{tx: &testTenantTx{owner: tenantOwnerRecord{ID: 42}}})
	if !errors.Is(service.UpdateTenant(context.Background(), 42, TenantUpdateInput{Name: "租户", LoginAccount: "owner"}), errManagementConflict) {
		t.Fatal("缺少所有者的租户编辑应返回冲突")
	}
	if !errors.Is(service.ResetTenantOwnerPassword(context.Background(), 42, "123456"), errManagementConflict) {
		t.Fatal("缺少所有者的密码重置应返回冲突")
	}
}

// TestTenantServiceDeleteGuards 验证删除租户必须停用且通过空租户校验后才删除初始化数据。
func TestTenantServiceDeleteGuards(t *testing.T) {
	ownerID := uint64(13)
	activeService := NewTenantService(&testTenantStore{tx: &testTenantTx{owner: tenantOwnerRecord{ID: 42, Status: 1, OwnerEmployeeID: &ownerID}}})
	if !errors.Is(activeService.DeleteTenant(context.Background(), 42), errManagementConflict) {
		t.Fatal("启用租户不应允许删除")
	}

	tx := &testTenantTx{owner: tenantOwnerRecord{ID: 42, Status: 0, OwnerEmployeeID: &ownerID}, ownerRoleID: 12}
	service := NewTenantService(&testTenantStore{tx: tx})
	if err := service.DeleteTenant(context.Background(), 42); err != nil {
		t.Fatalf("空停用租户删除应成功: %v", err)
	}
	if !tx.validateEmptyCalled || !tx.deleteBootstrapCalled {
		t.Fatalf("删除空租户未执行校验和删除流程: %#v", tx)
	}
}
