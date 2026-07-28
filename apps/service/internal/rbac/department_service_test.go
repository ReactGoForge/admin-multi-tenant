package rbac

import (
	"context"
	"errors"
	"testing"
)

type testDepartmentServiceStore struct {
	validateDepartmentErr error
	validateLeaderErr     error
	createErr             error
	updateChanged         bool
	updateErr             error
	ensureErr             error
	childCount            int64
	employeeCount         int64
	deleteChanged         bool
	deleteErr             error
	created               bool
	updated               bool
	deleted               bool
}

// ListDepartments 返回测试预设的空部门列表。
func (store *testDepartmentServiceStore) ListDepartments(context.Context, managementScope) ([]PlatformDepartment, error) {
	return []PlatformDepartment{}, nil
}

// ValidateDepartment 返回测试预设的父部门校验结果。
func (store *testDepartmentServiceStore) ValidateDepartment(context.Context, managementScope, *uint64, *uint64) error {
	return store.validateDepartmentErr
}

// ValidateDepartmentLeader 返回测试预设的负责人校验结果。
func (store *testDepartmentServiceStore) ValidateDepartmentLeader(context.Context, managementScope, *uint64) error {
	return store.validateLeaderErr
}

// CreateDepartment 记录部门创建调用。
func (store *testDepartmentServiceStore) CreateDepartment(context.Context, managementScope, DepartmentMutation) error {
	store.created = true
	return store.createErr
}

// UpdateDepartment 记录部门更新调用。
func (store *testDepartmentServiceStore) UpdateDepartment(context.Context, managementScope, uint64, DepartmentMutation) (bool, error) {
	store.updated = true
	return store.updateChanged, store.updateErr
}

// EnsureDepartmentExists 返回测试预设的存在性校验结果。
func (store *testDepartmentServiceStore) EnsureDepartmentExists(context.Context, managementScope, uint64) error {
	return store.ensureErr
}

// DepartmentChildCount 返回测试预设的子部门数量。
func (store *testDepartmentServiceStore) DepartmentChildCount(context.Context, managementScope, uint64) (int64, error) {
	return store.childCount, nil
}

// DepartmentEmployeeCount 返回测试预设的部门员工数量。
func (store *testDepartmentServiceStore) DepartmentEmployeeCount(context.Context, managementScope, uint64) (int64, error) {
	return store.employeeCount, nil
}

// DeleteDepartment 记录部门删除调用。
func (store *testDepartmentServiceStore) DeleteDepartment(context.Context, managementScope, uint64) (bool, error) {
	store.deleted = true
	return store.deleteChanged, store.deleteErr
}

// TestDepartmentServiceCreateStopsBeforeWriteWhenParentInvalid 验证父部门无效时不会写入部门。
func TestDepartmentServiceCreateStopsBeforeWriteWhenParentInvalid(t *testing.T) {
	store := &testDepartmentServiceStore{validateDepartmentErr: errManagementNotFound}
	service := NewDepartmentService(store)
	err := service.CreateDepartment(context.Background(), managementScope{Name: "platform"}, DepartmentMutation{Name: "平台运营部", Status: 1})
	if !errors.Is(err, errManagementNotFound) || store.created {
		t.Fatalf("创建部门父级校验结果 err=%v created=%v", err, store.created)
	}
}

// TestDepartmentServiceCreateStopsBeforeWriteWhenLeaderInvalid 验证负责人无效时不会写入部门。
func TestDepartmentServiceCreateStopsBeforeWriteWhenLeaderInvalid(t *testing.T) {
	store := &testDepartmentServiceStore{validateLeaderErr: errManagementNotFound}
	service := NewDepartmentService(store)
	err := service.CreateDepartment(context.Background(), managementScope{Name: "platform"}, DepartmentMutation{Name: "平台运营部", Status: 1})
	if !errors.Is(err, errManagementNotFound) || store.created {
		t.Fatalf("创建部门负责人校验结果 err=%v created=%v", err, store.created)
	}
}

// TestDepartmentServiceUpdateEnsuresExistenceWhenUnchanged 验证更新未命中时继续执行存在性校验。
func TestDepartmentServiceUpdateEnsuresExistenceWhenUnchanged(t *testing.T) {
	store := &testDepartmentServiceStore{ensureErr: errManagementNotFound}
	service := NewDepartmentService(store)
	err := service.UpdateDepartment(context.Background(), managementScope{Name: "platform"}, 9, DepartmentMutation{Name: "平台运营部", Status: 1})
	if !errors.Is(err, errManagementNotFound) || !store.updated {
		t.Fatalf("更新部门存在性校验 err=%v updated=%v", err, store.updated)
	}
}

// TestDepartmentServiceDeleteProtectsNonEmptyDepartment 验证存在子部门或员工时拒绝删除。
func TestDepartmentServiceDeleteProtectsNonEmptyDepartment(t *testing.T) {
	service := NewDepartmentService(&testDepartmentServiceStore{childCount: 1, deleteChanged: true})
	err := service.DeleteDepartment(context.Background(), managementScope{Name: "platform"}, 9)
	if !errors.Is(err, errManagementConflict) {
		t.Fatalf("存在子部门删除错误 = %v", err)
	}

	store := &testDepartmentServiceStore{employeeCount: 1, deleteChanged: true}
	service = NewDepartmentService(store)
	err = service.DeleteDepartment(context.Background(), managementScope{Name: "platform"}, 9)
	if !errors.Is(err, errManagementConflict) || store.deleted {
		t.Fatalf("存在员工删除错误 err=%v deleted=%v", err, store.deleted)
	}
}

// TestDepartmentServiceDeleteMapsMissingDepartment 验证删除未命中时返回资源不存在。
func TestDepartmentServiceDeleteMapsMissingDepartment(t *testing.T) {
	store := &testDepartmentServiceStore{}
	service := NewDepartmentService(store)
	err := service.DeleteDepartment(context.Background(), managementScope{Name: "platform"}, 9)
	if !errors.Is(err, errManagementNotFound) || !store.deleted {
		t.Fatalf("删除不存在部门错误 err=%v deleted=%v", err, store.deleted)
	}
}
