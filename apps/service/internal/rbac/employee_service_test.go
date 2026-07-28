package rbac

import (
	"context"
	"errors"
	"testing"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
)

type testEmployeeServiceStore struct {
	authority          delegationAuthority
	rolesWithin        map[uint64]bool
	protected          bool
	deleteRow          employeeDeleteRow
	validateRolesError error
	createCalls        int
	updateCalls        int
	replaceCalls       int
	deleteCalls        int
}

// LoadDelegationAuthority 返回测试预设的向下授权边界。
func (store *testEmployeeServiceStore) LoadDelegationAuthority(context.Context, auth.Employee, auth.TokenIdentity, managementScope, bool) (delegationAuthority, error) {
	if store.authority.PlatformPermissions == nil {
		return newDelegationAuthority(true), nil
	}
	return store.authority, nil
}

// RolesWithinDelegationAuthority 返回测试预设的角色授权结果。
func (store *testEmployeeServiceStore) RolesWithinDelegationAuthority(_ context.Context, _ managementScope, roleIDs []uint64, _ delegationAuthority) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(roleIDs))
	for _, roleID := range roleIDs {
		allowed, exists := store.rolesWithin[roleID]
		result[roleID] = !exists || allowed
	}
	return result, nil
}

// ListEmployees 是 Service 写流程测试未使用的占位实现。
func (store *testEmployeeServiceStore) ListEmployees(context.Context, managementScope, PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	return nil, 0, nil
}

// ListEmployeeOptions 是 Service 写流程测试未使用的占位实现。
func (store *testEmployeeServiceStore) ListEmployeeOptions(context.Context, managementScope) (PlatformEmployeeOptions, error) {
	return PlatformEmployeeOptions{}, nil
}

// ValidateDepartment 默认允许测试部门。
func (store *testEmployeeServiceStore) ValidateDepartment(context.Context, managementScope, *uint64, *uint64) error {
	return nil
}

// IsProtectedEmployee 返回测试预设保护状态。
func (store *testEmployeeServiceStore) IsProtectedEmployee(context.Context, managementScope, uint64) (bool, error) {
	return store.protected, nil
}

// EnsureEmployeeExists 默认认为测试员工存在。
func (store *testEmployeeServiceStore) EnsureEmployeeExists(context.Context, managementScope, uint64) error {
	return nil
}

// UpdateEmployee 记录员工更新调用。
func (store *testEmployeeServiceStore) UpdateEmployee(context.Context, managementScope, uint64, EmployeeUpdate) (bool, error) {
	store.updateCalls++
	return true, nil
}

// ResetEmployeePassword 默认认为密码更新成功。
func (store *testEmployeeServiceStore) ResetEmployeePassword(context.Context, managementScope, uint64, string) (bool, error) {
	return true, nil
}

// SetEmployeeStatus 默认认为状态更新成功。
func (store *testEmployeeServiceStore) SetEmployeeStatus(context.Context, managementScope, uint64, uint8) (bool, error) {
	return true, nil
}

// WithEmployeeTransaction 直接使用当前测试 Store 模拟事务 Store。
func (store *testEmployeeServiceStore) WithEmployeeTransaction(ctx context.Context, fn func(EmployeeTransactionStore) error) error {
	return fn(store)
}

// ValidateEmployeeRoles 返回测试预设角色校验错误。
func (store *testEmployeeServiceStore) ValidateEmployeeRoles(context.Context, managementScope, []uint64) error {
	return store.validateRolesError
}

// CreateEmployee 记录员工创建调用。
func (store *testEmployeeServiceStore) CreateEmployee(context.Context, managementScope, EmployeeCreate) (uint64, error) {
	store.createCalls++
	return 100, nil
}

// ReplaceEmployeeRoles 记录角色替换调用。
func (store *testEmployeeServiceStore) ReplaceEmployeeRoles(context.Context, managementScope, uint64, []uint64) error {
	store.replaceCalls++
	return nil
}

// ListEmployeeRoleIDs 返回当前员工已有角色。
func (store *testEmployeeServiceStore) ListEmployeeRoleIDs(context.Context, managementScope, uint64) ([]uint64, error) {
	return []uint64{99}, nil
}

// FindEmployeeForDelete 返回测试预设的删除前员工行。
func (store *testEmployeeServiceStore) FindEmployeeForDelete(context.Context, managementScope, uint64) (employeeDeleteRow, error) {
	return store.deleteRow, nil
}

// HasAnySystemRole 默认认为员工没有系统角色。
func (store *testEmployeeServiceStore) HasAnySystemRole(context.Context, managementScope, uint64) (bool, error) {
	return false, nil
}

// IsTenantOwnerEmployee 默认认为员工不是租户所有者。
func (store *testEmployeeServiceStore) IsTenantOwnerEmployee(context.Context, managementScope, uint64) (bool, error) {
	return false, nil
}

// HasEmployeeDeleteReference 默认认为员工没有删除引用。
func (store *testEmployeeServiceStore) HasEmployeeDeleteReference(context.Context, managementScope, uint64) (bool, error) {
	return false, nil
}

// DeleteEmployeeRoles 默认认为角色关联删除成功。
func (store *testEmployeeServiceStore) DeleteEmployeeRoles(context.Context, managementScope, uint64) error {
	return nil
}

// DeleteEmployee 记录员工删除调用。
func (store *testEmployeeServiceStore) DeleteEmployee(context.Context, managementScope, uint64) error {
	store.deleteCalls++
	return nil
}

// TestEmployeeServiceAllowsSelfUpdate 验证同范围普通会话可更新本人基础资料，平台受保护员工本人也可更新。
func TestEmployeeServiceAllowsSelfUpdate(t *testing.T) {
	tenantID := uint64(8)
	tests := []struct {
		name      string
		actor     EmployeeActor
		scope     managementScope
		protected bool
	}{
		{
			name: "租户员工本人",
			actor: EmployeeActor{
				Employee: auth.Employee{ID: 9, Scope: "tenant", TenantID: &tenantID},
				Identity: auth.TokenIdentity{EmployeeID: 9, Scope: "tenant", TenantID: &tenantID, Mode: "normal"},
			},
			scope: managementScope{Name: "tenant", TenantID: &tenantID},
		},
		{
			name: "平台受保护员工本人",
			actor: EmployeeActor{
				Employee:           auth.Employee{ID: 9, Scope: "platform"},
				Identity:           auth.TokenIdentity{EmployeeID: 9, Scope: "platform", Mode: "normal"},
				PlatformSuperAdmin: true,
			},
			scope:     managementScope{Name: "platform"},
			protected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &testEmployeeServiceStore{protected: test.protected}
			service := NewEmployeeService(store)
			err := service.UpdateEmployee(
				context.Background(),
				test.actor,
				test.scope,
				9,
				EmployeeMutation{Name: "本人", LoginAccount: "self", Status: 1},
			)
			if err != nil || store.updateCalls != 1 {
				t.Fatalf("本人基础资料应允许更新一次: err=%v updateCalls=%d", err, store.updateCalls)
			}
		})
	}
}

// TestEmployeeServiceRejectsOtherProtectedEmployeeUpdate 验证平台受保护员工仍不能被其他员工编辑。
func TestEmployeeServiceRejectsOtherProtectedEmployeeUpdate(t *testing.T) {
	store := &testEmployeeServiceStore{protected: true}
	service := NewEmployeeService(store)
	actor := EmployeeActor{
		Employee:           auth.Employee{ID: 1, Scope: "platform"},
		Identity:           auth.TokenIdentity{EmployeeID: 1, Scope: "platform", Mode: "normal"},
		PlatformSuperAdmin: true,
	}
	err := service.UpdateEmployee(
		context.Background(),
		actor,
		managementScope{Name: "platform"},
		9,
		EmployeeMutation{Name: "受保护员工", LoginAccount: "protected", Status: 1},
	)
	if !errors.Is(err, errManagementProtected) || store.updateCalls != 0 {
		t.Fatalf("其他受保护员工仍应禁止编辑: err=%v updateCalls=%d", err, store.updateCalls)
	}
}

// TestEmployeeServiceStillRejectsSelfSensitiveOperations 验证本人基础资料放开后仍不能自分配角色、重置密码、停用或删除。
func TestEmployeeServiceStillRejectsSelfSensitiveOperations(t *testing.T) {
	tenantID := uint64(8)
	store := &testEmployeeServiceStore{}
	service := NewEmployeeService(store)
	actor := EmployeeActor{
		Employee: auth.Employee{ID: 9, Scope: "tenant", TenantID: &tenantID},
		Identity: auth.TokenIdentity{EmployeeID: 9, Scope: "tenant", TenantID: &tenantID, Mode: "normal"},
	}
	scope := managementScope{Name: "tenant", TenantID: &tenantID}
	tests := []struct {
		name     string
		run      func() error
		expected error
	}{
		{
			name:     "分配角色",
			run:      func() error { return service.AssignEmployeeRoles(context.Background(), actor, scope, 9, []uint64{1}) },
			expected: errManagementForbidden,
		},
		{
			name:     "重置密码",
			run:      func() error { return service.ResetEmployeePassword(context.Background(), actor, scope, 9, "password") },
			expected: errManagementForbidden,
		},
		{
			name:     "停用",
			run:      func() error { return service.SetEmployeeStatus(context.Background(), actor, scope, 9, 0) },
			expected: errManagementForbidden,
		},
		{
			name:     "删除",
			run:      func() error { return service.DeleteEmployee(context.Background(), actor, scope, 9) },
			expected: errManagementProtected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, test.expected) {
				t.Fatalf("本人敏感操作应被拒绝: err=%v", err)
			}
		})
	}
	if store.replaceCalls != 0 || store.deleteCalls != 0 {
		t.Fatalf("本人敏感操作不应进入写入: replace=%d delete=%d", store.replaceCalls, store.deleteCalls)
	}
}

// TestEmployeeServiceCreateStopsBeforeWriteWhenRolesInvalid 验证角色校验失败时不会创建员工或角色关联。
func TestEmployeeServiceCreateStopsBeforeWriteWhenRolesInvalid(t *testing.T) {
	store := &testEmployeeServiceStore{validateRolesError: errManagementProtected}
	service := NewEmployeeService(store)
	actor := EmployeeActor{Employee: auth.Employee{ID: 1, Scope: "platform"}, Identity: auth.TokenIdentity{EmployeeID: 1, Scope: "platform", Mode: "normal"}, PlatformSuperAdmin: true}
	err := service.CreateEmployee(context.Background(), actor, managementScope{Name: "platform"}, EmployeeMutation{Name: "员工", LoginAccount: "staff", Password: "123456", RoleIDs: []uint64{1}, Status: 1})
	if !errors.Is(err, errManagementProtected) || store.createCalls != 0 || store.replaceCalls != 0 {
		t.Fatalf("角色非法时不应写员工或角色关联: err=%v create=%d replace=%d", err, store.createCalls, store.replaceCalls)
	}
}

// TestEmployeeServiceAssignRejectsCurrentRoleOutsideAuthority 验证移除越权角色前也会先校验当前角色边界。
func TestEmployeeServiceAssignRejectsCurrentRoleOutsideAuthority(t *testing.T) {
	store := &testEmployeeServiceStore{rolesWithin: map[uint64]bool{99: false, 2: true}}
	service := NewEmployeeService(store)
	actor := EmployeeActor{Employee: auth.Employee{ID: 1, Scope: "platform"}, Identity: auth.TokenIdentity{EmployeeID: 1, Scope: "platform", Mode: "normal"}, PlatformSuperAdmin: false}
	err := service.AssignEmployeeRoles(context.Background(), actor, managementScope{Name: "platform"}, 2, []uint64{2})
	if !errors.Is(err, errManagementForbidden) || store.replaceCalls != 0 {
		t.Fatalf("当前越权角色不应被静默移除: err=%v replace=%d", err, store.replaceCalls)
	}
}

// TestEmployeeServiceDeleteRequiresDisabledEmployee 验证删除员工前必须先停用。
func TestEmployeeServiceDeleteRequiresDisabledEmployee(t *testing.T) {
	store := &testEmployeeServiceStore{deleteRow: employeeDeleteRow{ID: 2, Status: 1}}
	service := NewEmployeeService(store)
	actor := EmployeeActor{Employee: auth.Employee{ID: 1, Scope: "platform"}, Identity: auth.TokenIdentity{EmployeeID: 1, Scope: "platform", Mode: "normal"}, PlatformSuperAdmin: true}
	err := service.DeleteEmployee(context.Background(), actor, managementScope{Name: "platform"}, 2)
	if !errors.Is(err, errManagementConflict) || store.deleteCalls != 0 {
		t.Fatalf("启用员工不应被删除: err=%v delete=%d", err, store.deleteCalls)
	}
}
