package media

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// TestServiceUploadImageCleansObjectWhenMetadataFails 验证图片元数据写入失败后会尽力清理新对象。
func TestServiceUploadImageCleansObjectWhenMetadataFails(t *testing.T) {
	store := &fakeMediaStore{createImageErr: ErrInvalidOwner}
	storage := &fakeMediaStorage{}
	service := NewService(store, storage)

	_, err := service.UploadImage(context.Background(), ImageUpload{OriginalName: "logo.png", Data: []byte("image"), MIMEType: "image/png", Extension: ".png", UploadedByEmployeeID: 1})
	if !errors.Is(err, ErrInvalidOwner) {
		t.Fatalf("UploadImage() error = %v", err)
	}
	if len(storage.deletedKeys) != 1 {
		t.Fatalf("对象清理次数 = %d, want 1", len(storage.deletedKeys))
	}
}

// TestServiceUploadImageKeepsMetadataWhenPresignFails 验证元数据成功后预签名失败不会删除已入库对象。
func TestServiceUploadImageKeepsMetadataWhenPresignFails(t *testing.T) {
	store := &fakeMediaStore{}
	storage := &fakeMediaStorage{presignErr: errors.New("sign failed")}
	service := NewService(store, storage)

	_, err := service.UploadImage(context.Background(), ImageUpload{OriginalName: "logo.png", Data: []byte("image"), MIMEType: "image/png", Extension: ".png", UploadedByEmployeeID: 1})
	if !errors.Is(err, ErrMediaUnavailable) {
		t.Fatalf("UploadImage() error = %v", err)
	}
	if store.createdAsset == nil {
		t.Fatal("预签名失败前应已保存元数据")
	}
	if len(storage.deletedKeys) != 0 {
		t.Fatalf("预签名失败不应删除已入库对象，deleted=%v", storage.deletedKeys)
	}
}

// TestServiceDeleteImageIgnoresStorageCleanupFailure 验证删除元数据成功后对象清理失败不会回滚业务成功。
func TestServiceDeleteImageIgnoresStorageCleanupFailure(t *testing.T) {
	store := &fakeMediaStore{deleteMetadataKey: "platform/2026/07/logo.png"}
	storage := &fakeMediaStorage{deleteErr: errors.New("delete failed")}
	service := NewService(store, storage)

	if err := service.DeleteImage(context.Background(), 10, nil); err != nil {
		t.Fatalf("DeleteImage() error = %v", err)
	}
	if len(storage.deletedKeys) != 1 || storage.deletedKeys[0] != store.deleteMetadataKey {
		t.Fatalf("对象清理记录 = %v", storage.deletedKeys)
	}
}

// TestServiceListImagesReturnsUnavailableWhenPresignFails 验证列表预览地址签发失败时返回媒体不可用错误。
func TestServiceListImagesReturnsUnavailableWhenPresignFails(t *testing.T) {
	store := &fakeMediaStore{listAssets: []ImageAsset{{ID: 1, ObjectKey: "platform/2026/07/logo.png"}}}
	storage := &fakeMediaStorage{presignErr: errors.New("sign failed")}
	service := NewService(store, storage)

	_, err := service.ListImages(context.Background(), nil, false, nil, "", 1, 10)
	if !errors.Is(err, ErrMediaUnavailable) {
		t.Fatalf("ListImages() error = %v", err)
	}
}

// TestUploadMiniappUserAvatarReplacesAndCleansOldObject 验证小程序头像替换成功后清理旧对象。
func TestUploadMiniappUserAvatarReplacesAndCleansOldObject(t *testing.T) {
	store := &fakeMediaStore{oldMiniappAvatarKey: "miniapp-users/old.png"}
	storage := &fakeMediaStorage{}
	service := NewService(store, storage)

	avatarURL, err := service.UploadMiniappUserAvatar(context.Background(), MiniappUserAvatarUpload{
		UserID: 4, OriginalName: "avatar.png", Data: []byte("image"), MIMEType: "image/png", Extension: ".png",
	})
	if err != nil || avatarURL == "" || !strings.HasPrefix(store.newMiniappAvatarKey, "miniapp-users/") {
		t.Fatalf("头像替换结果 url=%q key=%q err=%v", avatarURL, store.newMiniappAvatarKey, err)
	}
	if len(storage.deletedKeys) != 1 || storage.deletedKeys[0] != store.oldMiniappAvatarKey {
		t.Fatalf("旧头像清理记录 = %v", storage.deletedKeys)
	}
}

// TestUploadMiniappUserAvatarCleansNewObjectWhenDatabaseFails 验证数据库替换失败时补偿删除新头像。
func TestUploadMiniappUserAvatarCleansNewObjectWhenDatabaseFails(t *testing.T) {
	store := &fakeMediaStore{replaceMiniappAvatarErr: ErrNotFound}
	storage := &fakeMediaStorage{}
	service := NewService(store, storage)

	_, err := service.UploadMiniappUserAvatar(context.Background(), MiniappUserAvatarUpload{
		UserID: 4, OriginalName: "avatar.webp", Data: []byte("image"), MIMEType: "image/webp", Extension: ".webp",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UploadMiniappUserAvatar() error = %v", err)
	}
	if len(storage.deletedKeys) != 1 || storage.deletedKeys[0] != store.newMiniappAvatarKey {
		t.Fatalf("新头像补偿清理记录 = %v, key=%q", storage.deletedKeys, store.newMiniappAvatarKey)
	}
}

type fakeMediaStore struct {
	createImageErr          error
	replaceMiniappAvatarErr error
	oldMiniappAvatarKey     string
	newMiniappAvatarKey     string
	createdAsset            *ImageAsset
	deleteMetadataKey       string
	listAssets              []ImageAsset
}

// FindEmployeeAvatar 返回测试头像元数据。
func (store *fakeMediaStore) FindEmployeeAvatar(context.Context, uint64) (*ImageAsset, error) {
	return &ImageAsset{ObjectKey: "avatar.webp"}, nil
}

// ReplaceEmployeeAvatar 记录测试头像元数据替换。
func (store *fakeMediaStore) ReplaceEmployeeAvatar(context.Context, uint64, string, *uint64, *ImageAsset) (string, error) {
	return "", nil
}

// ReplaceMiniappUserAvatar 记录测试小程序用户头像替换。
func (store *fakeMediaStore) ReplaceMiniappUserAvatar(_ context.Context, _ uint64, objectKey string) (string, error) {
	store.newMiniappAvatarKey = objectKey
	return store.oldMiniappAvatarKey, store.replaceMiniappAvatarErr
}

// GetPlatformSettings 返回测试平台品牌设置。
func (store *fakeMediaStore) GetPlatformSettings(context.Context) (*BasicSettings, error) {
	return &BasicSettings{Name: "ReactGoForge Admin"}, nil
}

// UpdatePlatformSettings 记录测试平台品牌设置更新。
func (store *fakeMediaStore) UpdatePlatformSettings(context.Context, string, *uint64) error {
	return nil
}

// GetTenantSettings 返回测试租户品牌设置。
func (store *fakeMediaStore) GetTenantSettings(context.Context, uint64) (*BasicSettings, error) {
	return &BasicSettings{Name: "租户"}, nil
}

// UpdateTenantSettings 记录测试租户品牌设置更新。
func (store *fakeMediaStore) UpdateTenantSettings(context.Context, uint64, string, *uint64) error {
	return nil
}

// FindPublicImage 返回测试公开品牌图片元数据。
func (store *fakeMediaStore) FindPublicImage(context.Context, uint64) (*ImageAsset, error) {
	return &ImageAsset{ObjectKey: "public.png", MIMEType: "image/png"}, nil
}

// ListImages 返回测试图片列表。
func (store *fakeMediaStore) ListImages(context.Context, *uint64, bool, *uint64, string, int, int) ([]ImageAsset, int64, error) {
	return store.listAssets, int64(len(store.listAssets)), nil
}

// CreateImage 记录测试图片元数据写入。
func (store *fakeMediaStore) CreateImage(_ context.Context, asset *ImageAsset) error {
	store.createdAsset = asset
	asset.ID = 100
	return store.createImageErr
}

// UpdateImageName 记录测试图片名称更新。
func (store *fakeMediaStore) UpdateImageName(context.Context, uint64, *uint64, string) error {
	return nil
}

// UpdateImageCategory 记录测试图片分类更新。
func (store *fakeMediaStore) UpdateImageCategory(context.Context, uint64, *uint64, *uint64) error {
	return nil
}

// DeleteImageMetadata 返回测试删除后的对象键。
func (store *fakeMediaStore) DeleteImageMetadata(context.Context, uint64, *uint64) (string, error) {
	return store.deleteMetadataKey, nil
}

// ListCategories 返回测试图片分类列表。
func (store *fakeMediaStore) ListCategories(context.Context, *uint64, bool) ([]ImageCategory, error) {
	return nil, nil
}

// CreateCategory 记录测试图片分类创建。
func (store *fakeMediaStore) CreateCategory(_ context.Context, category *ImageCategory) error {
	category.ID = 1
	return nil
}

// UpdateCategory 记录测试图片分类更新。
func (store *fakeMediaStore) UpdateCategory(context.Context, uint64, *uint64, string) error {
	return nil
}

// DeleteCategory 记录测试图片分类删除。
func (store *fakeMediaStore) DeleteCategory(context.Context, uint64, *uint64) error {
	return nil
}

// ListTenantOptions 返回测试租户选项。
func (store *fakeMediaStore) ListTenantOptions(context.Context) ([]TenantOption, error) {
	return nil, nil
}

type fakeMediaStorage struct {
	presignErr  error
	deleteErr   error
	deletedKeys []string
}

// Ready 表示测试对象存储可用。
func (storage *fakeMediaStorage) Ready() bool { return true }

// Put 记录测试对象写入。
func (storage *fakeMediaStorage) Put(context.Context, string, string, int64, io.Reader) error {
	return nil
}

// PresignedGet 返回测试预签名地址。
func (storage *fakeMediaStorage) PresignedGet(_ context.Context, key string, _ time.Duration) (string, error) {
	if storage.presignErr != nil {
		return "", storage.presignErr
	}
	return "https://example.test/" + strings.TrimPrefix(key, "/"), nil
}

// Get 返回测试对象流。
func (storage *fakeMediaStorage) Get(context.Context, string) (*Object, error) {
	return &Object{Body: io.NopCloser(strings.NewReader("image")), ContentType: "image/png", Size: 5}, nil
}

// Delete 记录测试对象删除。
func (storage *fakeMediaStorage) Delete(_ context.Context, key string) error {
	storage.deletedKeys = append(storage.deletedKeys, key)
	return storage.deleteErr
}
