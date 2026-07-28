package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"path/filepath"
)

var ErrMediaUnavailable = errors.New("图片存储服务不可用")

// ImageUpload 描述 Handler 已校验完成的图片上传内容。
type ImageUpload struct {
	OwnerTenantID        *uint64
	CategoryID           *uint64
	OriginalName         string
	Data                 []byte
	MIMEType             string
	Extension            string
	UploadedByEmployeeID uint64
}

// AvatarUpload 描述 Handler 已校验完成的头像上传内容。
type AvatarUpload struct {
	EmployeeID   uint64
	EmployeeName string
	Scope        string
	TenantID     *uint64
	OriginalName string
	Data         []byte
	MIMEType     string
	Extension    string
}

// MiniappUserAvatarUpload 描述已通过校验的小程序用户头像。
type MiniappUserAvatarUpload struct {
	UserID       uint64
	OriginalName string
	Data         []byte
	MIMEType     string
	Extension    string
}

// ImagePage 描述图片列表查询结果和分页信息。
type ImagePage struct {
	Items    []ImageAsset
	URLs     map[uint64]string
	Page     int
	PageSize int
	Total    int64
}

// ImageUploadResult 描述图片上传后的元数据和临时访问地址。
type ImageUploadResult struct {
	Asset      ImageAsset
	PreviewURL string
}

// AvatarUploadResult 描述头像替换后的审计目标和临时访问地址。
type AvatarUploadResult struct {
	AvatarURL    string
	EmployeeID   uint64
	EmployeeName string
}

// PublicImageObject 描述公开品牌图片代理需要写回 HTTP 响应的对象流。
type PublicImageObject struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

// serviceStore 定义媒体 Service 需要的数据库访问能力。
type serviceStore interface {
	FindEmployeeAvatar(context.Context, uint64) (*ImageAsset, error)
	ReplaceEmployeeAvatar(context.Context, uint64, string, *uint64, *ImageAsset) (string, error)
	ReplaceMiniappUserAvatar(context.Context, uint64, string) (string, error)
	GetPlatformSettings(context.Context) (*BasicSettings, error)
	UpdatePlatformSettings(context.Context, string, *uint64) error
	GetTenantSettings(context.Context, uint64) (*BasicSettings, error)
	UpdateTenantSettings(context.Context, uint64, string, *uint64) error
	FindPublicImage(context.Context, uint64) (*ImageAsset, error)
	ListImages(context.Context, *uint64, bool, *uint64, string, int, int) ([]ImageAsset, int64, error)
	CreateImage(context.Context, *ImageAsset) error
	UpdateImageName(context.Context, uint64, *uint64, string) error
	UpdateImageCategory(context.Context, uint64, *uint64, *uint64) error
	DeleteImageMetadata(context.Context, uint64, *uint64) (string, error)
	ListCategories(context.Context, *uint64, bool) ([]ImageCategory, error)
	CreateCategory(context.Context, *ImageCategory) error
	UpdateCategory(context.Context, uint64, *uint64, string) error
	DeleteCategory(context.Context, uint64, *uint64) error
	ListTenantOptions(context.Context) ([]TenantOption, error)
}

// Service 编排媒体业务规则、对象存储访问和数据库写入补偿。
type Service struct {
	store   serviceStore
	storage Storage
}

// NewService 使用媒体数据库和对象存储能力创建业务服务。
func NewService(store serviceStore, storage Storage) *Service {
	return &Service{store: store, storage: storage}
}

// StorageReady 返回当前对象存储是否可用于读写或签发临时地址。
func (service *Service) StorageReady() bool {
	return service != nil && service.storage != nil && service.storage.Ready()
}

// AvatarURL 为当前用户接口签发覆盖后台会话周期的头像临时地址。
func (service *Service) AvatarURL(ctx context.Context, imageID uint64) (string, error) {
	if !service.StorageReady() {
		return "", ErrNotFound
	}
	asset, err := service.store.FindEmployeeAvatar(ctx, imageID)
	if err != nil {
		return "", err
	}
	return service.storage.PresignedGet(ctx, asset.ObjectKey, avatarExpiry)
}

// MiniappUserAvatarURL 为小程序用户私有头像签发临时访问地址。
func (service *Service) MiniappUserAvatarURL(ctx context.Context, objectKey string) (string, error) {
	if !service.StorageReady() || objectKey == "" {
		return "", ErrMediaUnavailable
	}
	return service.storage.PresignedGet(ctx, objectKey, avatarExpiry)
}

// UploadMiniappUserAvatar 保存小程序用户头像对象并原子替换用户头像键。
func (service *Service) UploadMiniappUserAvatar(ctx context.Context, upload MiniappUserAvatarUpload) (string, error) {
	if !service.StorageReady() {
		return "", ErrMediaUnavailable
	}
	key, err := createMiniappUserAvatarObjectKey(upload.Extension)
	if err != nil {
		return "", err
	}
	if err := service.storage.Put(ctx, key, upload.MIMEType, int64(len(upload.Data)), bytes.NewReader(upload.Data)); err != nil {
		return "", ErrMediaUnavailable
	}
	oldObjectKey, err := service.store.ReplaceMiniappUserAvatar(ctx, upload.UserID, key)
	if err != nil {
		service.cleanupObject(ctx, key, "小程序头像写入失败后的对象清理失败")
		return "", err
	}
	if oldObjectKey != "" {
		service.cleanupObject(ctx, oldObjectKey, "小程序旧头像替换后的对象清理失败")
	}
	avatarURL, err := service.storage.PresignedGet(ctx, key, avatarExpiry)
	if err != nil {
		return "", ErrMediaUnavailable
	}
	return avatarURL, nil
}

// UploadCurrentEmployeeAvatar 保存新头像、切换员工引用，并尽力清理失败或旧对象。
func (service *Service) UploadCurrentEmployeeAvatar(ctx context.Context, upload AvatarUpload) (*AvatarUploadResult, error) {
	if !service.StorageReady() {
		return nil, ErrMediaUnavailable
	}
	key, err := createObjectKey(upload.TenantID, upload.Extension)
	if err != nil {
		return nil, err
	}
	if err := service.storage.Put(ctx, key, upload.MIMEType, int64(len(upload.Data)), bytes.NewReader(upload.Data)); err != nil {
		return nil, ErrMediaUnavailable
	}
	avatarURL, err := service.storage.PresignedGet(ctx, key, avatarExpiry)
	if err != nil {
		service.cleanupObject(ctx, key, "头像临时地址签发失败后的对象清理失败")
		return nil, ErrMediaUnavailable
	}
	asset := &ImageAsset{
		TenantID:             upload.TenantID,
		OriginalName:         filepath.Base(upload.OriginalName),
		ObjectKey:            key,
		MIMEType:             upload.MIMEType,
		SizeBytes:            uint64(len(upload.Data)),
		UploadedByEmployeeID: upload.EmployeeID,
	}
	oldObjectKey, err := service.store.ReplaceEmployeeAvatar(ctx, upload.EmployeeID, upload.Scope, upload.TenantID, asset)
	if err != nil {
		service.cleanupObject(ctx, key, "头像元数据写入失败后的对象清理失败")
		return nil, err
	}
	if oldObjectKey != "" {
		service.cleanupObject(ctx, oldObjectKey, "旧头像元数据删除后的对象清理失败")
	}
	return &AvatarUploadResult{AvatarURL: avatarURL, EmployeeID: upload.EmployeeID, EmployeeName: upload.EmployeeName}, nil
}

// GetPlatformBasicSettings 返回平台品牌名称和图标摘要。
func (service *Service) GetPlatformBasicSettings(ctx context.Context) (*BasicSettings, error) {
	return service.store.GetPlatformSettings(ctx)
}

// UpdatePlatformBasicSettings 保存平台品牌名称和图标引用。
func (service *Service) UpdatePlatformBasicSettings(ctx context.Context, name string, iconImageID *uint64) error {
	return service.store.UpdatePlatformSettings(ctx, name, iconImageID)
}

// GetTenantBasicSettings 返回指定可信租户品牌名称和图标摘要。
func (service *Service) GetTenantBasicSettings(ctx context.Context, tenantID uint64) (*BasicSettings, error) {
	return service.store.GetTenantSettings(ctx, tenantID)
}

// UpdateTenantBasicSettings 保存指定可信租户品牌名称和图标引用。
func (service *Service) UpdateTenantBasicSettings(ctx context.Context, tenantID uint64, name string, iconImageID *uint64) error {
	return service.store.UpdateTenantSettings(ctx, tenantID, name, iconImageID)
}

// PublicPlatformBrand 返回公开平台品牌名称和图标图片 ID。
func (service *Service) PublicPlatformBrand(ctx context.Context) (*BasicSettings, error) {
	return service.store.GetPlatformSettings(ctx)
}

// PublicImage 读取当前被品牌引用的私有对象，供 Handler 代理输出。
func (service *Service) PublicImage(ctx context.Context, imageID uint64) (*PublicImageObject, error) {
	if !service.StorageReady() {
		return nil, ErrMediaUnavailable
	}
	asset, err := service.store.FindPublicImage(ctx, imageID)
	if err != nil {
		return nil, err
	}
	object, err := service.storage.Get(ctx, asset.ObjectKey)
	if err != nil {
		return nil, ErrMediaUnavailable
	}
	return &PublicImageObject{Body: object.Body, ContentType: asset.MIMEType, Size: object.Size}, nil
}

// ListImages 读取图片元数据并为每张图片签发短期预览地址。
func (service *Service) ListImages(ctx context.Context, ownerTenantID *uint64, sharedOnly bool, categoryID *uint64, name string, page int, pageSize int) (*ImagePage, error) {
	if !service.StorageReady() {
		return nil, ErrMediaUnavailable
	}
	assets, total, err := service.store.ListImages(ctx, ownerTenantID, sharedOnly, categoryID, name, page, pageSize)
	if err != nil {
		return nil, err
	}
	urls := make(map[uint64]string, len(assets))
	for _, asset := range assets {
		previewURL, signErr := service.storage.PresignedGet(ctx, asset.ObjectKey, presignedExpiry)
		if signErr != nil {
			return nil, ErrMediaUnavailable
		}
		urls[asset.ID] = previewURL
	}
	return &ImagePage{Items: assets, URLs: urls, Page: page, PageSize: pageSize, Total: total}, nil
}

// UploadImage 写入对象存储和图片元数据，数据库失败时尽力删除新对象。
func (service *Service) UploadImage(ctx context.Context, upload ImageUpload) (*ImageUploadResult, error) {
	if !service.StorageReady() {
		return nil, ErrMediaUnavailable
	}
	key, err := createObjectKey(upload.OwnerTenantID, upload.Extension)
	if err != nil {
		return nil, err
	}
	if err := service.storage.Put(ctx, key, upload.MIMEType, int64(len(upload.Data)), bytes.NewReader(upload.Data)); err != nil {
		return nil, ErrMediaUnavailable
	}
	asset := &ImageAsset{
		TenantID:             upload.OwnerTenantID,
		CategoryID:           upload.CategoryID,
		OriginalName:         filepath.Base(upload.OriginalName),
		ObjectKey:            key,
		MIMEType:             upload.MIMEType,
		SizeBytes:            uint64(len(upload.Data)),
		UploadedByEmployeeID: upload.UploadedByEmployeeID,
	}
	if err := service.store.CreateImage(ctx, asset); err != nil {
		service.cleanupObject(ctx, key, "图片元数据写入失败后的对象清理失败")
		return nil, err
	}
	previewURL, err := service.storage.PresignedGet(ctx, key, presignedExpiry)
	if err != nil {
		return nil, ErrMediaUnavailable
	}
	return &ImageUploadResult{Asset: *asset, PreviewURL: previewURL}, nil
}

// UpdateImageName 修改单张图片的展示名称。
func (service *Service) UpdateImageName(ctx context.Context, imageID uint64, ownerTenantID *uint64, name string) error {
	return service.store.UpdateImageName(ctx, imageID, ownerTenantID, name)
}

// UpdateImageCategory 修改单张图片的分类。
func (service *Service) UpdateImageCategory(ctx context.Context, imageID uint64, ownerTenantID *uint64, categoryID *uint64) error {
	return service.store.UpdateImageCategory(ctx, imageID, ownerTenantID, categoryID)
}

// DeleteImage 删除图片元数据后尽力清理对象存储。
func (service *Service) DeleteImage(ctx context.Context, imageID uint64, ownerTenantID *uint64) error {
	objectKey, err := service.store.DeleteImageMetadata(ctx, imageID, ownerTenantID)
	if err != nil {
		return err
	}
	if service.StorageReady() {
		service.cleanupObject(ctx, objectKey, "图片元数据删除后的对象清理失败")
	}
	return nil
}

// ListCategories 返回指定所有者的分类列表。
func (service *Service) ListCategories(ctx context.Context, ownerTenantID *uint64, sharedOnly bool) ([]ImageCategory, error) {
	return service.store.ListCategories(ctx, ownerTenantID, sharedOnly)
}

// CreateCategory 新增指定所有者的图片分类。
func (service *Service) CreateCategory(ctx context.Context, ownerTenantID *uint64, name string) (*ImageCategory, error) {
	category := &ImageCategory{TenantID: ownerTenantID, Name: name}
	if err := service.store.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// UpdateCategory 修改指定所有者的图片分类名称。
func (service *Service) UpdateCategory(ctx context.Context, categoryID uint64, ownerTenantID *uint64, name string) error {
	return service.store.UpdateCategory(ctx, categoryID, ownerTenantID, name)
}

// DeleteCategory 删除指定所有者的图片分类。
func (service *Service) DeleteCategory(ctx context.Context, categoryID uint64, ownerTenantID *uint64) error {
	return service.store.DeleteCategory(ctx, categoryID, ownerTenantID)
}

// ListTenantOptions 返回平台图片管理的租户筛选选项。
func (service *Service) ListTenantOptions(ctx context.Context) ([]TenantOption, error) {
	return service.store.ListTenantOptions(ctx)
}

// cleanupObject 尽力删除对象存储中的孤立对象，失败时只记录固定文案。
func (service *Service) cleanupObject(ctx context.Context, objectKey string, message string) {
	if objectKey == "" || service.storage == nil {
		return
	}
	if err := service.storage.Delete(ctx, objectKey); err != nil {
		log.Print(message)
	}
}
