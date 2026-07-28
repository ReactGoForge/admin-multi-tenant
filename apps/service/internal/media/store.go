package media

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound        = errors.New("图片数据不存在")
	ErrConflict        = errors.New("图片数据冲突")
	ErrInvalidOwner    = errors.New("图片归属不合法")
	ErrImageReferenced = errors.New("图片仍被品牌设置引用")
)

// ImageAsset 描述数据库中的图片元数据及列表展示所需的关联字段。
type ImageAsset struct {
	ID                   uint64    `gorm:"column:id"`
	TenantID             *uint64   `gorm:"column:tenant_id"`
	TenantName           *string   `gorm:"->;column:tenant_name"`
	CategoryID           *uint64   `gorm:"column:category_id"`
	CategoryName         *string   `gorm:"->;column:category_name"`
	OriginalName         string    `gorm:"column:original_name"`
	ObjectKey            string    `gorm:"column:object_key"`
	MIMEType             string    `gorm:"column:mime_type"`
	SizeBytes            uint64    `gorm:"column:size_bytes"`
	UploadedByEmployeeID uint64    `gorm:"column:uploaded_by_employee_id"`
	CreatedAt            time.Time `gorm:"column:created_at"`
}

// ImageCategory 描述平台或单个租户拥有的自定义图片分类。
type ImageCategory struct {
	ID       uint64  `gorm:"column:id"`
	TenantID *uint64 `gorm:"column:tenant_id"`
	Name     string  `gorm:"column:name"`
	IsShared bool    `gorm:"column:is_shared"`
}

// TenantOption 描述平台图片管理按租户筛选所需的最小数据。
type TenantOption struct {
	ID   uint64 `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

// BasicSettings 描述平台或租户品牌设置及当前图标摘要。
type BasicSettings struct {
	Name             string
	IconImageID      *uint64
	LegacyIconURL    *string
	IconOriginalName *string
}

// Store 使用 GORM 持久化媒体元数据、分类和品牌图片引用。
type Store struct {
	db *gorm.DB
}

// NewStore 创建媒体数据库访问对象。
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// ListImages 按可信所有者、共享范围、分类和名称分页读取图片元数据。
func (store *Store) ListImages(ctx context.Context, tenantID *uint64, sharedOnly bool, categoryID *uint64, name string, page int, pageSize int) ([]ImageAsset, int64, error) {
	// 业务约束：tenantID 为 nil 表示平台所有者，非 nil 表示指定租户；所有查询都先固定所有者范围。
	query := store.db.WithContext(ctx).
		Table("image_assets AS ia").
		Joins("LEFT JOIN tenants AS t ON t.id = ia.tenant_id").
		Joins("LEFT JOIN image_categories AS ic ON ic.id = ia.category_id").
		Where("NOT EXISTS (SELECT 1 FROM employees avatar_owner WHERE avatar_owner.avatar_image_id = ia.id)")
	if tenantID == nil {
		query = query.Where("ia.tenant_id IS NULL")
	} else {
		query = query.Where("ia.tenant_id = ?", *tenantID)
	}
	if sharedOnly {
		query = query.Where("ic.is_shared = 1")
	}
	if categoryID != nil {
		if *categoryID == 0 {
			query = query.Where("ia.category_id IS NULL")
		} else {
			query = query.Where("ia.category_id = ?", *categoryID)
		}
	}
	if keyword := strings.TrimSpace(name); keyword != "" {
		query = query.Where("ia.original_name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	assets := make([]ImageAsset, 0)
	err := query.Select("ia.id, ia.tenant_id, t.name AS tenant_name, ia.category_id, ic.name AS category_name, ia.original_name, ia.object_key, ia.mime_type, ia.size_bytes, ia.uploaded_by_employee_id, ia.created_at").
		Order("ia.created_at DESC, ia.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&assets).Error
	return assets, total, err
}

// FindImage 按 ID 读取单张图片及其归属。
func (store *Store) FindImage(ctx context.Context, imageID uint64) (*ImageAsset, error) {
	var asset ImageAsset
	err := store.db.WithContext(ctx).Table("image_assets").Where("id = ?", imageID).Take(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &asset, err
}

// CreateImage 保存已经成功上传到对象存储的图片元数据。
func (store *Store) CreateImage(ctx context.Context, asset *ImageAsset) error {
	// 业务约束：图片和分类必须属于同一所有者，校验与元数据写入放在同一事务中。
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if asset.CategoryID != nil {
			if err := validateCategoryOwner(tx, *asset.CategoryID, asset.TenantID); err != nil {
				return err
			}
		}
		return tx.Table("image_assets").Create(asset).Error
	})
}

// UpdateImageCategory 校验图片和分类同属一个所有者后修改分类。
func (store *Store) UpdateImageCategory(ctx context.Context, imageID uint64, ownerTenantID *uint64, categoryID *uint64) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, err := findImageForOwner(tx, imageID, ownerTenantID)
		if err != nil {
			return err
		}
		if categoryID != nil {
			if err := validateCategoryOwner(tx, *categoryID, ownerTenantID); err != nil {
				return err
			}
		}
		return tx.Table("image_assets").Where("id = ?", asset.ID).Update("category_id", categoryID).Error
	})
}

// UpdateImageName 校验图片归属后修改用于展示和搜索的图片名称。
func (store *Store) UpdateImageName(ctx context.Context, imageID uint64, ownerTenantID *uint64, name string) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, err := findImageForOwner(tx, imageID, ownerTenantID)
		if err != nil {
			return err
		}
		return tx.Table("image_assets").Where("id = ?", asset.ID).Update("original_name", name).Error
	})
}

// DeleteImageMetadata 在确认图片未被品牌引用且归属正确后删除元数据，并返回对象键供尽力清理。
func (store *Store) DeleteImageMetadata(ctx context.Context, imageID uint64, ownerTenantID *uint64) (string, error) {
	objectKey := ""
	// Go 学习提示：objectKey 在事务回调外声明，回调成功后把结果带回 Handler；
	// 如果任何步骤返回 error，GORM 回滚数据库删除，调用方不会清理对象。
	err := store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, err := findImageForOwner(tx, imageID, ownerTenantID)
		if err != nil {
			return err
		}
		var references int64
		if err := tx.Raw("SELECT (SELECT COUNT(*) FROM platform_settings WHERE icon_image_id = ?) + (SELECT COUNT(*) FROM tenants WHERE icon_image_id = ?) + (SELECT COUNT(*) FROM employees WHERE avatar_image_id = ?) AS reference_count", imageID, imageID, imageID).Scan(&references).Error; err != nil {
			return err
		}
		if references > 0 {
			return ErrImageReferenced
		}
		if err := tx.Table("image_assets").Where("id = ?", imageID).Delete(&ImageAsset{}).Error; err != nil {
			return err
		}
		objectKey = asset.ObjectKey
		return nil
	})
	return objectKey, err
}

// ReplaceEmployeeAvatar 在一个事务内创建头像图片、替换员工引用并删除旧头像元数据。
func (store *Store) ReplaceEmployeeAvatar(ctx context.Context, employeeID uint64, scope string, tenantID *uint64, asset *ImageAsset) (string, error) {
	oldObjectKey := ""
	err := store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var employee struct {
			Scope         string  `gorm:"column:scope"`
			TenantID      *uint64 `gorm:"column:tenant_id"`
			AvatarImageID *uint64 `gorm:"column:avatar_image_id"`
		}
		if err := tx.Table("employees").Clauses(clause.Locking{Strength: "UPDATE"}).Select("scope", "tenant_id", "avatar_image_id").Where("id = ? AND status = 1", employeeID).Take(&employee).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if employee.Scope != scope || !sameOwner(employee.TenantID, tenantID) || !sameOwner(asset.TenantID, tenantID) {
			return ErrInvalidOwner
		}
		if err := tx.Table("image_assets").Create(asset).Error; err != nil {
			return err
		}
		if err := tx.Table("employees").Where("id = ?", employeeID).Update("avatar_image_id", asset.ID).Error; err != nil {
			return err
		}
		if employee.AvatarImageID == nil {
			return nil
		}
		var oldAsset ImageAsset
		if err := tx.Table("image_assets").Where("id = ?", *employee.AvatarImageID).Take(&oldAsset).Error; err != nil {
			return err
		}
		oldObjectKey = oldAsset.ObjectKey
		return tx.Table("image_assets").Where("id = ?", oldAsset.ID).Delete(&ImageAsset{}).Error
	})
	return oldObjectKey, err
}

// ReplaceMiniappUserAvatar 锁定启用用户并替换其私有头像对象键。
func (store *Store) ReplaceMiniappUserAvatar(ctx context.Context, userID uint64, newObjectKey string) (string, error) {
	oldObjectKey := ""
	err := store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current struct {
			AvatarObjectKey *string `gorm:"column:avatar_url"`
		}
		err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("avatar_url").Where("id = ? AND status = 1", userID).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if current.AvatarObjectKey != nil {
			oldObjectKey = *current.AvatarObjectKey
		}
		return tx.Table("users").Where("id = ? AND status = 1", userID).Update("avatar_url", newObjectKey).Error
	})
	return oldObjectKey, err
}

// FindEmployeeAvatar 读取当前员工引用的头像对象元数据。
func (store *Store) FindEmployeeAvatar(ctx context.Context, imageID uint64) (*ImageAsset, error) {
	var asset ImageAsset
	err := store.db.WithContext(ctx).Table("image_assets AS ia").Where("ia.id = ?", imageID).
		Where("EXISTS (SELECT 1 FROM employees avatar_owner WHERE avatar_owner.avatar_image_id = ia.id)").Take(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &asset, err
}

// ListCategories 读取指定所有者的全部分类，并可限制为平台共享分类。
func (store *Store) ListCategories(ctx context.Context, tenantID *uint64, sharedOnly bool) ([]ImageCategory, error) {
	query := store.db.WithContext(ctx).Table("image_categories")
	if tenantID == nil {
		query = query.Where("tenant_id IS NULL")
	} else {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	if sharedOnly {
		query = query.Where("is_shared = 1")
	}
	categories := make([]ImageCategory, 0)
	err := query.Order("name ASC, id ASC").Find(&categories).Error
	return categories, err
}

// CreateCategory 新增当前所有者的图片分类。
func (store *Store) CreateCategory(ctx context.Context, category *ImageCategory) error {
	if err := store.db.WithContext(ctx).Table("image_categories").Create(category).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return ErrConflict
		}
		return err
	}
	return nil
}

// UpdateCategory 校验归属后修改分类名称。
func (store *Store) UpdateCategory(ctx context.Context, categoryID uint64, ownerTenantID *uint64, name string) error {
	query := ownerQuery(store.db.WithContext(ctx).Table("image_categories"), ownerTenantID).Where("id = ? AND is_shared = 0", categoryID)
	result := query.Update("name", name)
	if result.Error != nil {
		if strings.Contains(strings.ToLower(result.Error.Error()), "duplicate") {
			return ErrConflict
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCategory 校验归属后删除分类；数据库外键会将分类下图片自动转入未分类。
func (store *Store) DeleteCategory(ctx context.Context, categoryID uint64, ownerTenantID *uint64) error {
	result := ownerQuery(store.db.WithContext(ctx).Table("image_categories"), ownerTenantID).Where("id = ? AND is_shared = 0", categoryID).Delete(&ImageCategory{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTenantOptions 读取平台图片选择器按租户筛选所需的全部租户选项。
func (store *Store) ListTenantOptions(ctx context.Context) ([]TenantOption, error) {
	options := make([]TenantOption, 0)
	err := store.db.WithContext(ctx).Table("tenants").Select("id", "name").Order("name ASC, id ASC").Scan(&options).Error
	return options, err
}

// GetPlatformSettings 读取全平台唯一品牌名称与图标摘要。
func (store *Store) GetPlatformSettings(ctx context.Context) (*BasicSettings, error) {
	var result struct {
		Name             string  `gorm:"column:name"`
		IconImageID      *uint64 `gorm:"column:icon_image_id"`
		IconOriginalName *string `gorm:"column:icon_original_name"`
	}
	err := store.db.WithContext(ctx).Table("platform_settings AS ps").
		Select("ps.name, ps.icon_image_id, ia.original_name AS icon_original_name").
		Joins("LEFT JOIN image_assets AS ia ON ia.id = ps.icon_image_id").Where("ps.id = 1").Take(&result).Error
	if err != nil {
		return nil, err
	}
	return &BasicSettings{Name: result.Name, IconImageID: result.IconImageID, IconOriginalName: result.IconOriginalName}, nil
}

// UpdatePlatformSettings 校验图标来自图片库后更新平台品牌。
func (store *Store) UpdatePlatformSettings(ctx context.Context, name string, iconImageID *uint64) error {
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if iconImageID != nil {
			var count int64
			if err := tx.Table("image_assets").Where("id = ?", *iconImageID).
				Where("NOT EXISTS (SELECT 1 FROM employees avatar_owner WHERE avatar_owner.avatar_image_id = image_assets.id)").
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrInvalidOwner
			}
		}
		return tx.Table("platform_settings").Where("id = 1").Updates(map[string]any{"name": name, "icon_image_id": iconImageID}).Error
	})
}

// GetTenantSettings 读取可信租户的名称、兼容图标地址和图片图标摘要。
func (store *Store) GetTenantSettings(ctx context.Context, tenantID uint64) (*BasicSettings, error) {
	var result struct {
		Name             string  `gorm:"column:name"`
		IconURL          *string `gorm:"column:icon_url"`
		IconImageID      *uint64 `gorm:"column:icon_image_id"`
		IconOriginalName *string `gorm:"column:icon_original_name"`
	}
	err := store.db.WithContext(ctx).Table("tenants AS t").
		Select("t.name, t.icon_url, t.icon_image_id, ia.original_name AS icon_original_name").
		Joins("LEFT JOIN image_assets AS ia ON ia.id = t.icon_image_id").Where("t.id = ?", tenantID).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &BasicSettings{Name: result.Name, IconImageID: result.IconImageID, LegacyIconURL: result.IconURL, IconOriginalName: result.IconOriginalName}, nil
}

// UpdateTenantSettings 校验图标来自平台共享图库或当前租户图库后更新租户品牌。
func (store *Store) UpdateTenantSettings(ctx context.Context, tenantID uint64, name string, iconImageID *uint64) error {
	// 业务约束：租户只能引用自己的图片，或平台明确放入共享分类的图片。
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant struct {
			IconImageID *uint64 `gorm:"column:icon_image_id"`
		}
		if err := tx.Table("tenants").Select("icon_image_id").Where("id = ?", tenantID).Take(&tenant).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if iconImageID != nil {
			var asset ImageAsset
			if err := tx.Table("image_assets AS ia").Where("ia.id = ?", *iconImageID).
				Where("NOT EXISTS (SELECT 1 FROM employees avatar_owner WHERE avatar_owner.avatar_image_id = ia.id)").Take(&asset).Error; err != nil {
				return ErrInvalidOwner
			}
			if asset.TenantID != nil && *asset.TenantID != tenantID {
				return ErrInvalidOwner
			}
			if asset.TenantID == nil && (tenant.IconImageID == nil || *tenant.IconImageID != *iconImageID) {
				var sharedCount int64
				if err := tx.Table("image_assets AS ia").
					Joins("JOIN image_categories AS ic ON ic.id = ia.category_id").
					Where("ia.id = ? AND ia.tenant_id IS NULL AND ic.is_shared = 1", *iconImageID).
					Count(&sharedCount).Error; err != nil {
					return err
				}
				if sharedCount == 0 {
					return ErrInvalidOwner
				}
			}
		}
		result := tx.Table("tenants").Where("id = ?", tenantID).Updates(map[string]any{"name": name, "icon_image_id": iconImageID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// FindPublicImage 仅在图片正被平台或某个租户品牌引用时返回对象元数据。
func (store *Store) FindPublicImage(ctx context.Context, imageID uint64) (*ImageAsset, error) {
	var asset ImageAsset
	err := store.db.WithContext(ctx).Table("image_assets AS ia").
		Where("ia.id = ?", imageID).
		Where("EXISTS (SELECT 1 FROM platform_settings ps WHERE ps.icon_image_id = ia.id) OR EXISTS (SELECT 1 FROM tenants t WHERE t.icon_image_id = ia.id)").
		Take(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &asset, err
}

// findImageForOwner 按图片 ID 与所有者精确校验图片归属。
func findImageForOwner(db *gorm.DB, imageID uint64, ownerTenantID *uint64) (*ImageAsset, error) {
	// 安全边界：只按图片 ID 查询不足以实现多租户隔离，必须同时追加所有者条件。
	var asset ImageAsset
	query := ownerQuery(db.Table("image_assets"), ownerTenantID).Where("id = ?", imageID).
		Where("NOT EXISTS (SELECT 1 FROM employees avatar_owner WHERE avatar_owner.avatar_image_id = image_assets.id)")
	if err := query.Take(&asset).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &asset, nil
}

// sameOwner 比较两个可空租户 ID 是否代表同一个平台或租户所有者。
func sameOwner(left, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// validateCategoryOwner 校验分类与目标图片属于同一平台或租户所有者。
func validateCategoryOwner(db *gorm.DB, categoryID uint64, ownerTenantID *uint64) error {
	var count int64
	if err := ownerQuery(db.Table("image_categories"), ownerTenantID).Where("id = ?", categoryID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrInvalidOwner
	}
	return nil
}

// ownerQuery 将可空租户所有者转换为精确的数据范围条件。
func ownerQuery(query *gorm.DB, tenantID *uint64) *gorm.DB {
	if tenantID == nil {
		return query.Where("tenant_id IS NULL")
	}
	return query.Where("tenant_id = ?", *tenantID)
}
