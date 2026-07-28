package dictionary

import (
	"context"
	"errors"
)

var (
	ErrNotFound  = errors.New("dictionary resource not found")
	ErrConflict  = errors.New("dictionary resource conflict")
	ErrProtected = errors.New("system dictionary resource is protected")
)

// TypeMutation 描述经过 HTTP 校验的字典字段写入内容。
type TypeMutation struct {
	Code   string
	Name   string
	Remark *string
	Sort   uint32
	Status uint8
}

// ItemMutation 描述经过 HTTP 校验的字典项写入内容。
type ItemMutation struct {
	Label  string
	Value  string
	Sort   uint32
	Status uint8
}

// TypeUpdate 描述 Store 需要执行的字典字段更新内容。
type TypeUpdate struct {
	Code   *string
	Name   string
	Remark *string
	Sort   uint32
	Status *uint8
}

// ItemUpdate 描述 Store 需要执行的字典项更新内容。
type ItemUpdate struct {
	Label  string
	Value  *string
	Sort   uint32
	Status *uint8
}

// DataStore 定义字典 Service 需要的数据访问能力。
type DataStore interface {
	ListTypes(context.Context, bool) ([]Type, error)
	CreateType(context.Context, TypeMutation) error
	WithTransaction(context.Context, func(TransactionStore) error) error
}

// TransactionStore 定义字典事务内需要的数据访问能力。
type TransactionStore interface {
	FindTypeForUpdate(context.Context, uint64) (Type, error)
	CountItems(context.Context, uint64) (int64, error)
	UpdateType(context.Context, uint64, TypeUpdate) error
	DeleteType(context.Context, uint64) error
	CreateItem(context.Context, uint64, ItemMutation) error
	FindItemForUpdate(context.Context, uint64, uint64) (Item, error)
	UpdateItem(context.Context, uint64, uint64, ItemUpdate) error
	DeleteItem(context.Context, uint64, uint64) error
}

// Service 编排字典业务规则和事务边界。
type Service struct {
	store DataStore
}

// NewService 使用字典数据能力创建字典服务。
func NewService(store DataStore) *Service { return &Service{store: store} }

// ListOptions 返回所有启用字典字段及启用字典项。
func (service *Service) ListOptions(ctx context.Context) ([]Type, error) {
	return service.store.ListTypes(ctx, true)
}

// ListTypes 返回包含禁用数据的完整字典管理列表。
func (service *Service) ListTypes(ctx context.Context) ([]Type, error) {
	return service.store.ListTypes(ctx, false)
}

// CreateType 新增一个可完整维护的自定义字典字段。
func (service *Service) CreateType(ctx context.Context, mutation TypeMutation) error {
	return service.store.CreateType(ctx, mutation)
}

// UpdateType 更新字典字段，并保护系统字段的编码和状态。
func (service *Service) UpdateType(ctx context.Context, typeID uint64, mutation TypeMutation) error {
	return service.store.WithTransaction(ctx, func(tx TransactionStore) error {
		typeRow, err := tx.FindTypeForUpdate(ctx, typeID)
		if err != nil {
			return err
		}
		update := TypeUpdate{Name: mutation.Name, Remark: mutation.Remark, Sort: mutation.Sort}
		if !typeRow.IsSystem {
			update.Code = &mutation.Code
			update.Status = &mutation.Status
		}
		return tx.UpdateType(ctx, typeID, update)
	})
}

// DeleteType 删除无字典项的自定义字典字段。
func (service *Service) DeleteType(ctx context.Context, typeID uint64) error {
	return service.store.WithTransaction(ctx, func(tx TransactionStore) error {
		typeRow, err := tx.FindTypeForUpdate(ctx, typeID)
		if err != nil {
			return err
		}
		if typeRow.IsSystem {
			return ErrProtected
		}
		count, err := tx.CountItems(ctx, typeID)
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrConflict
		}
		return tx.DeleteType(ctx, typeID)
	})
}

// CreateItem 为自定义字典字段新增字典项。
func (service *Service) CreateItem(ctx context.Context, typeID uint64, mutation ItemMutation) error {
	return service.store.WithTransaction(ctx, func(tx TransactionStore) error {
		typeRow, err := tx.FindTypeForUpdate(ctx, typeID)
		if err != nil {
			return err
		}
		if typeRow.IsSystem {
			return ErrProtected
		}
		return tx.CreateItem(ctx, typeID, mutation)
	})
}

// UpdateItem 更新字典项，并只允许系统字典项修改文案和排序。
func (service *Service) UpdateItem(ctx context.Context, typeID, itemID uint64, mutation ItemMutation) error {
	return service.store.WithTransaction(ctx, func(tx TransactionStore) error {
		typeRow, err := tx.FindTypeForUpdate(ctx, typeID)
		if err != nil {
			return err
		}
		if _, err := tx.FindItemForUpdate(ctx, typeID, itemID); err != nil {
			return err
		}
		update := ItemUpdate{Label: mutation.Label, Sort: mutation.Sort}
		if !typeRow.IsSystem {
			update.Value = &mutation.Value
			update.Status = &mutation.Status
		}
		return tx.UpdateItem(ctx, typeID, itemID, update)
	})
}

// DeleteItem 删除自定义字典字段下的指定字典项。
func (service *Service) DeleteItem(ctx context.Context, typeID, itemID uint64) error {
	return service.store.WithTransaction(ctx, func(tx TransactionStore) error {
		typeRow, err := tx.FindTypeForUpdate(ctx, typeID)
		if err != nil {
			return err
		}
		if typeRow.IsSystem {
			return ErrProtected
		}
		return tx.DeleteItem(ctx, typeID, itemID)
	})
}
