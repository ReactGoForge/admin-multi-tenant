package rbac

import (
	"context"
	"errors"
	"testing"
)

type testMenuServiceStore struct {
	menus                []PlatformMenu
	findMenus            map[uint64]PlatformMenu
	createID             uint64
	createErr            error
	updateErr            error
	statusErr            error
	deleteErr            error
	childCount           int64
	enabledChildCount    int64
	roleRelations        int64
	managedRelations     int64
	assignErr            error
	transactions         int
	created              bool
	updated              bool
	statusUpdated        bool
	deleted              bool
	assignedTenantOwners bool
	lastListScope        string
	lastListEnabledOnly  bool
	lastStatus           uint8
	lastAssignedMenuID   uint64
}

// ListMenus 返回测试预设的菜单列表并记录过滤条件。
func (store *testMenuServiceStore) ListMenus(_ context.Context, scope string, enabledOnly bool) ([]PlatformMenu, error) {
	store.lastListScope = scope
	store.lastListEnabledOnly = enabledOnly
	return store.menus, nil
}

// WithMenuTransaction 记录事务调用并直接执行测试闭包。
func (store *testMenuServiceStore) WithMenuTransaction(ctx context.Context, fn func(MenuTransactionStore) error) error {
	store.transactions++
	return fn(store)
}

// FindMenuForUpdate 返回测试预设的菜单节点。
func (store *testMenuServiceStore) FindMenuForUpdate(_ context.Context, menuID uint64) (*PlatformMenu, error) {
	menu, ok := store.findMenus[menuID]
	if !ok {
		return nil, errManagementNotFound
	}
	return &menu, nil
}

// CreateMenu 记录菜单创建调用。
func (store *testMenuServiceStore) CreateMenu(_ context.Context, _ MenuMutation) (uint64, error) {
	store.created = true
	return store.createID, store.createErr
}

// UpdateMenu 记录菜单更新调用。
func (store *testMenuServiceStore) UpdateMenu(_ context.Context, _ uint64, _ MenuMutation) error {
	store.updated = true
	return store.updateErr
}

// SetMenuStatus 记录菜单状态更新调用。
func (store *testMenuServiceStore) SetMenuStatus(_ context.Context, _ uint64, status uint8) error {
	store.statusUpdated = true
	store.lastStatus = status
	return store.statusErr
}

// DeleteMenu 记录菜单删除调用。
func (store *testMenuServiceStore) DeleteMenu(context.Context, uint64) error {
	store.deleted = true
	return store.deleteErr
}

// MenuChildCount 返回测试预设的子节点数量。
func (store *testMenuServiceStore) MenuChildCount(_ context.Context, _ uint64, enabledOnly bool) (int64, error) {
	if enabledOnly {
		return store.enabledChildCount, nil
	}
	return store.childCount, nil
}

// RoleMenuRelationCount 返回测试预设的普通角色关联数量。
func (store *testMenuServiceStore) RoleMenuRelationCount(context.Context, uint64) (int64, error) {
	return store.roleRelations, nil
}

// ManagedMenuRelationCount 返回测试预设的平台代管关联数量。
func (store *testMenuServiceStore) ManagedMenuRelationCount(context.Context, uint64) (int64, error) {
	return store.managedRelations, nil
}

// AssignMenuToTenantOwners 记录企业管理员默认授权调用。
func (store *testMenuServiceStore) AssignMenuToTenantOwners(_ context.Context, menuID uint64) error {
	store.assignedTenantOwners = true
	store.lastAssignedMenuID = menuID
	return store.assignErr
}

// TestMenuServiceListUsesFullMenuScope 验证菜单服务列表读取全部状态节点。
func TestMenuServiceListUsesFullMenuScope(t *testing.T) {
	store := &testMenuServiceStore{}
	service := NewMenuService(store)
	if _, err := service.ListMenus(context.Background(), "tenant"); err != nil {
		t.Fatalf("菜单列表错误 = %v", err)
	}
	if store.lastListScope != "tenant" || store.lastListEnabledOnly {
		t.Fatalf("菜单列表条件 scope=%s enabledOnly=%v", store.lastListScope, store.lastListEnabledOnly)
	}
}

// TestMenuServiceRejectsInvalidNodeFields 验证非法字段组合不会进入写事务。
func TestMenuServiceRejectsInvalidNodeFields(t *testing.T) {
	store := &testMenuServiceStore{}
	service := NewMenuService(store)
	err := service.CreateMenu(context.Background(), MenuMutation{Scope: "platform", Type: "menu", Name: "未知页面", Status: 1})
	if !errors.Is(err, errManagementInvalid) || store.transactions != 0 {
		t.Fatalf("非法菜单字段 err=%v transactions=%d", err, store.transactions)
	}
}

// TestMenuServiceCreateAssignsTenantOwnerDefaults 验证可分配租户默认权限会同步授予企业管理员。
func TestMenuServiceCreateAssignsTenantOwnerDefaults(t *testing.T) {
	permissionCode := "tenant:employee:view"
	store := &testMenuServiceStore{createID: 88, findMenus: map[uint64]PlatformMenu{9: {ID: 9, Scope: "tenant", Type: "menu", Status: 1}}}
	service := NewMenuService(store)
	err := service.CreateMenu(context.Background(), MenuMutation{Scope: "tenant", Type: "permission", ParentID: menuUint64Pointer(9), Name: "查看员工", PermissionCode: &permissionCode, TenantAssignable: 1, Status: 1})
	if err != nil || !store.created || !store.assignedTenantOwners || store.lastAssignedMenuID != 88 {
		t.Fatalf("租户默认授权 err=%v created=%v assigned=%v menuID=%d", err, store.created, store.assignedTenantOwners, store.lastAssignedMenuID)
	}
}

// TestMenuServiceUpdateProtectsImmutableAndCoreFields 验证编辑禁止修改不可变字段和核心菜单关键字段。
func TestMenuServiceUpdateProtectsImmutableAndCoreFields(t *testing.T) {
	permissionCode := "platform:menu:view"
	path := "/platform/system/menus"
	component := "pages/platform/system/menus/index.tsx"
	store := &testMenuServiceStore{findMenus: map[uint64]PlatformMenu{1: {ID: 1, Scope: "platform", Type: "menu", Status: 1, Path: &path, Component: &component, PermissionCode: &permissionCode}}}
	service := NewMenuService(store)
	changedPermission := "platform:menu:create"
	err := service.UpdateMenu(context.Background(), 1, MenuMutation{Scope: "platform", Type: "menu", Name: "菜单管理", Path: &path, Component: &component, PermissionCode: &changedPermission, Status: 1, Visible: 1})
	if !errors.Is(err, errManagementProtected) || store.updated {
		t.Fatalf("核心菜单保护 err=%v updated=%v", err, store.updated)
	}

	tenantPath := "/tenant/system/roles"
	tenantComponent := "pages/tenant/system/roles/index.tsx"
	tenantPermissionCode := "tenant:role:view"
	err = service.UpdateMenu(context.Background(), 1, MenuMutation{Scope: "tenant", Type: "menu", Name: "角色管理", Path: &tenantPath, Component: &tenantComponent, PermissionCode: &tenantPermissionCode, TenantAssignable: 1, Status: 1, Visible: 1})
	if !errors.Is(err, errManagementConflict) {
		t.Fatalf("不可变字段修改错误 = %v", err)
	}
}

// TestMenuServiceStatusProtectsParentAndChildren 验证启停状态遵守核心菜单、父节点和子节点约束。
func TestMenuServiceStatusProtectsParentAndChildren(t *testing.T) {
	parentID := uint64(1)
	permissionCode := "tenant:role:view"
	store := &testMenuServiceStore{findMenus: map[uint64]PlatformMenu{
		1: {ID: 1, Scope: "tenant", Type: "directory", Status: 0},
		2: {ID: 2, Scope: "tenant", Type: "menu", ParentID: &parentID, Status: 0, PermissionCode: &permissionCode},
	}}
	service := NewMenuService(store)
	err := service.SetMenuStatus(context.Background(), 2, 1)
	if !errors.Is(err, errManagementConflict) || store.statusUpdated {
		t.Fatalf("父级停用时启用菜单 err=%v statusUpdated=%v", err, store.statusUpdated)
	}

	store = &testMenuServiceStore{enabledChildCount: 1, findMenus: map[uint64]PlatformMenu{2: {ID: 2, Scope: "tenant", Type: "menu", Status: 1, PermissionCode: &permissionCode}}}
	service = NewMenuService(store)
	err = service.SetMenuStatus(context.Background(), 2, 0)
	if !errors.Is(err, errManagementConflict) || store.statusUpdated {
		t.Fatalf("存在启用子节点时停用 err=%v statusUpdated=%v", err, store.statusUpdated)
	}
}

// TestMenuServiceDeleteProtectsCoreAndRelations 验证核心节点或存在关联时拒绝删除。
func TestMenuServiceDeleteProtectsCoreAndRelations(t *testing.T) {
	permissionCode := "platform:menu:view"
	store := &testMenuServiceStore{findMenus: map[uint64]PlatformMenu{1: {ID: 1, Scope: "platform", PermissionCode: &permissionCode}}}
	service := NewMenuService(store)
	err := service.DeleteMenu(context.Background(), 1)
	if !errors.Is(err, errManagementProtected) || store.deleted {
		t.Fatalf("核心菜单删除 err=%v deleted=%v", err, store.deleted)
	}

	permissionCode = "tenant:employee:view"
	store = &testMenuServiceStore{roleRelations: 1, findMenus: map[uint64]PlatformMenu{2: {ID: 2, Scope: "tenant", PermissionCode: &permissionCode}}}
	service = NewMenuService(store)
	err = service.DeleteMenu(context.Background(), 2)
	if !errors.Is(err, errManagementConflict) || store.deleted {
		t.Fatalf("存在关联删除 err=%v deleted=%v", err, store.deleted)
	}
}

// menuUint64Pointer 返回菜单测试用 BIGINT 指针。
func menuUint64Pointer(value uint64) *uint64 {
	return &value
}
