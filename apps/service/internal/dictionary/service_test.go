package dictionary

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type serviceTestStore struct {
	types      []Type
	typeRow    Type
	itemRow    Item
	itemCount  int64
	err        error
	events     []string
	enabled    bool
	typeID     uint64
	itemID     uint64
	typeInput  TypeMutation
	typeUpdate TypeUpdate
	itemInput  ItemMutation
	itemUpdate ItemUpdate
}

// ListTypes 返回测试预设字典并记录启用过滤参数。
func (store *serviceTestStore) ListTypes(_ context.Context, enabledOnly bool) ([]Type, error) {
	store.events = append(store.events, "list_types")
	store.enabled = enabledOnly
	return store.types, store.err
}

// CreateType 记录测试中的字典字段新增内容。
func (store *serviceTestStore) CreateType(_ context.Context, mutation TypeMutation) error {
	store.events = append(store.events, "create_type")
	store.typeInput = mutation
	return store.err
}

// WithTransaction 使用测试 Store 自身模拟事务数据访问能力。
func (store *serviceTestStore) WithTransaction(_ context.Context, action func(TransactionStore) error) error {
	store.events = append(store.events, "begin")
	if err := action(store); err != nil {
		store.events = append(store.events, "rollback")
		return err
	}
	store.events = append(store.events, "commit")
	return nil
}

// FindTypeForUpdate 记录测试中的字典字段加锁读取。
func (store *serviceTestStore) FindTypeForUpdate(_ context.Context, typeID uint64) (Type, error) {
	store.events = append(store.events, "lock_type")
	store.typeID = typeID
	if store.err != nil {
		return Type{}, store.err
	}
	return store.typeRow, nil
}

// CountItems 返回测试预设字典项数量。
func (store *serviceTestStore) CountItems(_ context.Context, typeID uint64) (int64, error) {
	store.events = append(store.events, "count_items")
	store.typeID = typeID
	if store.err != nil {
		return 0, store.err
	}
	return store.itemCount, nil
}

// UpdateType 记录测试中的字典字段更新内容。
func (store *serviceTestStore) UpdateType(_ context.Context, typeID uint64, update TypeUpdate) error {
	store.events = append(store.events, "update_type")
	store.typeID, store.typeUpdate = typeID, update
	return store.err
}

// DeleteType 记录测试中的字典字段删除 ID。
func (store *serviceTestStore) DeleteType(_ context.Context, typeID uint64) error {
	store.events = append(store.events, "delete_type")
	store.typeID = typeID
	return store.err
}

// CreateItem 记录测试中的字典项新增内容。
func (store *serviceTestStore) CreateItem(_ context.Context, typeID uint64, mutation ItemMutation) error {
	store.events = append(store.events, "create_item")
	store.typeID, store.itemInput = typeID, mutation
	return store.err
}

// FindItemForUpdate 记录测试中的字典项加锁读取。
func (store *serviceTestStore) FindItemForUpdate(_ context.Context, typeID, itemID uint64) (Item, error) {
	store.events = append(store.events, "lock_item")
	store.typeID, store.itemID = typeID, itemID
	if store.err != nil {
		return Item{}, store.err
	}
	return store.itemRow, nil
}

// UpdateItem 记录测试中的字典项更新内容。
func (store *serviceTestStore) UpdateItem(_ context.Context, typeID, itemID uint64, update ItemUpdate) error {
	store.events = append(store.events, "update_item")
	store.typeID, store.itemID, store.itemUpdate = typeID, itemID, update
	return store.err
}

// DeleteItem 记录测试中的字典项删除 ID。
func (store *serviceTestStore) DeleteItem(_ context.Context, typeID, itemID uint64) error {
	store.events = append(store.events, "delete_item")
	store.typeID, store.itemID = typeID, itemID
	return store.err
}

// TestServiceListAndCreate 验证查询过滤参数和自定义字典新增直接交给 Store。
func TestServiceListAndCreate(t *testing.T) {
	store := &serviceTestStore{}
	service := NewService(store)
	if _, err := service.ListOptions(context.Background()); err != nil || !store.enabled {
		t.Fatalf("启用字典查询 = %v enabled=%v", err, store.enabled)
	}
	if _, err := service.ListTypes(context.Background()); err != nil || store.enabled {
		t.Fatalf("管理字典查询 = %v enabled=%v", err, store.enabled)
	}

	mutation := TypeMutation{Code: "business_type", Name: "业务类型", Sort: 10, Status: 1}
	if err := service.CreateType(context.Background(), mutation); err != nil || store.typeInput != mutation {
		t.Fatalf("字典字段新增 = %v input=%+v", err, store.typeInput)
	}
}

// TestServiceTypeRules 验证系统字典字段保护、非空删除保护和事务访问顺序。
func TestServiceTypeRules(t *testing.T) {
	mutation := TypeMutation{Code: "new_code", Name: "新名称", Sort: 20, Status: 0}
	store := &serviceTestStore{typeRow: Type{ID: 7, IsSystem: true}}
	service := NewService(store)
	if err := service.UpdateType(context.Background(), 7, mutation); err != nil {
		t.Fatalf("系统字典字段更新 = %v", err)
	}
	if store.typeUpdate.Code != nil || store.typeUpdate.Status != nil || store.typeUpdate.Name != "新名称" || store.typeUpdate.Sort != 20 {
		t.Fatalf("系统字典字段保护失效: %+v", store.typeUpdate)
	}
	if !reflect.DeepEqual(store.events, []string{"begin", "lock_type", "update_type", "commit"}) {
		t.Fatalf("系统字典字段更新顺序 = %#v", store.events)
	}

	store = &serviceTestStore{typeRow: Type{ID: 8, IsSystem: false}, itemCount: 1}
	if err := NewService(store).DeleteType(context.Background(), 8); !errors.Is(err, ErrConflict) {
		t.Fatalf("非空自定义字典删除错误 = %v", err)
	}

	store = &serviceTestStore{typeRow: Type{ID: 9, IsSystem: true}}
	if err := NewService(store).DeleteType(context.Background(), 9); !errors.Is(err, ErrProtected) {
		t.Fatalf("系统字典字段删除错误 = %v", err)
	}
}

// TestServiceItemRules 验证系统字典项保护、自定义字典项更新和事务访问顺序。
func TestServiceItemRules(t *testing.T) {
	mutation := ItemMutation{Label: "展示名", Value: "stable_value", Sort: 30, Status: 0}
	store := &serviceTestStore{typeRow: Type{ID: 7, IsSystem: true}, itemRow: Item{ID: 11, DictionaryTypeID: 7}}
	service := NewService(store)
	if err := service.UpdateItem(context.Background(), 7, 11, mutation); err != nil {
		t.Fatalf("系统字典项更新 = %v", err)
	}
	if store.itemUpdate.Value != nil || store.itemUpdate.Status != nil || store.itemUpdate.Label != "展示名" || store.itemUpdate.Sort != 30 {
		t.Fatalf("系统字典项保护失效: %+v", store.itemUpdate)
	}
	if !reflect.DeepEqual(store.events, []string{"begin", "lock_type", "lock_item", "update_item", "commit"}) {
		t.Fatalf("系统字典项更新顺序 = %#v", store.events)
	}

	store = &serviceTestStore{typeRow: Type{ID: 8, IsSystem: false}}
	if err := NewService(store).CreateItem(context.Background(), 8, mutation); err != nil {
		t.Fatalf("自定义字典项新增 = %v", err)
	}
	if store.itemInput != mutation {
		t.Fatalf("自定义字典项新增输入 = %+v", store.itemInput)
	}

	store = &serviceTestStore{typeRow: Type{ID: 9, IsSystem: true}}
	if err := NewService(store).CreateItem(context.Background(), 9, mutation); !errors.Is(err, ErrProtected) {
		t.Fatalf("系统字典项新增错误 = %v", err)
	}
	if err := NewService(store).DeleteItem(context.Background(), 9, 11); !errors.Is(err, ErrProtected) {
		t.Fatalf("系统字典项删除错误 = %v", err)
	}
}
