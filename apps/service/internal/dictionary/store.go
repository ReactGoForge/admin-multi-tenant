package dictionary

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Type 描述一个全局字典字段及其字典内容。
type Type struct {
	ID       uint64  `gorm:"column:id;primaryKey"`
	Code     string  `gorm:"column:code"`
	Name     string  `gorm:"column:name"`
	Remark   *string `gorm:"column:remark"`
	Sort     uint32  `gorm:"column:sort"`
	Status   uint8   `gorm:"column:status"`
	IsSystem bool    `gorm:"column:is_system"`
	Items    []Item  `gorm:"foreignKey:DictionaryTypeID"`
}

// TableName 指定字典字段模型对应的真实表名。
func (Type) TableName() string { return "dictionary_types" }

// Item 描述一个字典字段下的稳定值和展示文案。
type Item struct {
	ID               uint64 `gorm:"column:id;primaryKey"`
	DictionaryTypeID uint64 `gorm:"column:dictionary_type_id"`
	Label            string `gorm:"column:label"`
	Value            string `gorm:"column:value"`
	Sort             uint32 `gorm:"column:sort"`
	Status           uint8  `gorm:"column:status"`
}

// TableName 指定字典项模型对应的真实表名。
func (Item) TableName() string { return "dictionary_items" }

// Store 使用 GORM 访问全局字典表。
type Store struct {
	db *gorm.DB
}

// transactionStore 使用同一个 GORM 事务访问全局字典表。
type transactionStore struct {
	db *gorm.DB
}

// NewStore 使用现有数据库连接创建字典存储。
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// ListTypes 按排序读取字典字段及字典项，可选择只返回启用数据。
func (store *Store) ListTypes(ctx context.Context, enabledOnly bool) ([]Type, error) {
	// Go 学习提示：GORM 查询是链式构造的；Where、Order、Preload 返回带有新增条件的 *gorm.DB。
	// Preload 用额外查询填充 Type.Items，避免 Handler 再逐个查询字典项。
	query := store.db.WithContext(ctx).Order("sort ASC, id ASC")
	preload := func(db *gorm.DB) *gorm.DB {
		if enabledOnly {
			db = db.Where("status = ?", 1)
		}
		return db.Order("sort ASC, id ASC")
	}
	if enabledOnly {
		query = query.Where("status = ?", 1)
	}
	var types []Type
	if err := query.Preload("Items", preload).Find(&types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

// CreateType 新增一个可完整维护的自定义字典字段。
func (store *Store) CreateType(ctx context.Context, mutation TypeMutation) error {
	typeRow := Type{Code: mutation.Code, Name: mutation.Name, Remark: mutation.Remark, Sort: mutation.Sort, Status: mutation.Status, IsSystem: false}
	return normalizeWriteError(store.db.WithContext(ctx).Create(&typeRow).Error)
}

// WithTransaction 在单个数据库事务中执行字典写流程。
func (store *Store) WithTransaction(ctx context.Context, action func(TransactionStore) error) error {
	// Go 学习提示：Transaction 回调返回 nil 时提交，返回 error 时自动回滚。
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return action(&transactionStore{db: tx})
	})
}

// FindTypeForUpdate 加锁读取字典字段，供事务内写操作校验保护规则。
func (store *transactionStore) FindTypeForUpdate(ctx context.Context, typeID uint64) (Type, error) {
	var typeRow Type
	if err := store.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&typeRow, typeID).Error; err != nil {
		return Type{}, normalizeFindError(err)
	}
	return typeRow, nil
}

// CountItems 统计指定字典字段下的字典项数量。
func (store *transactionStore) CountItems(ctx context.Context, typeID uint64) (int64, error) {
	var count int64
	if err := store.db.WithContext(ctx).Model(&Item{}).Where("dictionary_type_id = ?", typeID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// UpdateType 更新事务内已经完成业务校验的字典字段。
func (store *transactionStore) UpdateType(ctx context.Context, typeID uint64, update TypeUpdate) error {
	updates := map[string]any{"name": update.Name, "remark": update.Remark, "sort": update.Sort}
	if update.Code != nil {
		updates["code"] = *update.Code
	}
	if update.Status != nil {
		updates["status"] = *update.Status
	}
	return normalizeWriteError(store.db.WithContext(ctx).Model(&Type{}).Where("id = ?", typeID).Updates(updates).Error)
}

// DeleteType 删除事务内已经确认可删除的字典字段。
func (store *transactionStore) DeleteType(ctx context.Context, typeID uint64) error {
	return normalizeWriteError(store.db.WithContext(ctx).Delete(&Type{}, typeID).Error)
}

// CreateItem 新增事务内已经确认可创建的字典项。
func (store *transactionStore) CreateItem(ctx context.Context, typeID uint64, mutation ItemMutation) error {
	item := Item{DictionaryTypeID: typeID, Label: mutation.Label, Value: mutation.Value, Sort: mutation.Sort, Status: mutation.Status}
	return normalizeWriteError(store.db.WithContext(ctx).Create(&item).Error)
}

// FindItemForUpdate 加锁读取指定字典项，确保更新前目标存在且属于当前字典字段。
func (store *transactionStore) FindItemForUpdate(ctx context.Context, typeID, itemID uint64) (Item, error) {
	var item Item
	if err := store.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND dictionary_type_id = ?", itemID, typeID).First(&item).Error; err != nil {
		return Item{}, normalizeFindError(err)
	}
	return item, nil
}

// UpdateItem 更新事务内已经完成业务校验的字典项。
func (store *transactionStore) UpdateItem(ctx context.Context, typeID, itemID uint64, update ItemUpdate) error {
	updates := map[string]any{"label": update.Label, "sort": update.Sort}
	if update.Value != nil {
		updates["value"] = *update.Value
	}
	if update.Status != nil {
		updates["status"] = *update.Status
	}
	return normalizeWriteError(store.db.WithContext(ctx).Model(&Item{}).Where("id = ? AND dictionary_type_id = ?", itemID, typeID).Updates(updates).Error)
}

// DeleteItem 删除事务内已经确认可删除的字典项。
func (store *transactionStore) DeleteItem(ctx context.Context, typeID, itemID uint64) error {
	result := store.db.WithContext(ctx).Where("id = ? AND dictionary_type_id = ?", itemID, typeID).Delete(&Item{})
	if result.Error != nil {
		return normalizeWriteError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// normalizeFindError 将数据库未找到错误转换为稳定业务错误。
func normalizeFindError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// normalizeWriteError 将唯一约束和外键冲突转换为稳定业务错误。
func normalizeWriteError(err error) error {
	if err == nil {
		return nil
	}
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && (mysqlError.Number == 1062 || mysqlError.Number == 1451 || mysqlError.Number == 1452) {
		return ErrConflict
	}
	return err
}
