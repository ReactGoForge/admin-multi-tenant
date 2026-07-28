package media

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Object 描述从对象存储读取的图片流及其响应元数据。
type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

// Storage 定义媒体模块实际使用的最小对象存储能力。
// Go 学习提示：接口只描述“能做什么”，MinioStorage 与 UnavailableStorage 都可以作为实现传给 Handler。
type Storage interface {
	Ready() bool
	Put(context.Context, string, string, int64, io.Reader) error
	PresignedGet(context.Context, string, time.Duration) (string, error)
	Get(context.Context, string) (*Object, error)
	Delete(context.Context, string) error
}

// MinioStorage 使用私有 MinIO Bucket 存取图片，并使用对外地址签发临时访问 URL。
type MinioStorage struct {
	// 业务约束：internal 使用服务端可达地址执行真实读写，public 使用浏览器可达地址签发临时 URL。
	internal  *minio.Client
	public    *minio.Client
	bucket    string
	available atomic.Bool
}

// NewMinioStorage 创建 MinIO 客户端并检查或创建私有 Bucket；失败时返回错误但不泄露密钥。
func NewMinioStorage(ctx context.Context, config Config) (*MinioStorage, error) {
	if !config.Complete() {
		return nil, fmt.Errorf("MinIO 配置不完整")
	}
	internal, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 内部客户端失败: %w", err)
	}
	public, err := minio.New(config.PublicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.PublicUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 对外客户端失败: %w", err)
	}
	storage := &MinioStorage{internal: internal, public: public, bucket: config.Bucket}
	// Go 学习提示：atomic.Bool 允许多个 goroutine 安全读取可用状态，不需要额外互斥锁。
	exists, err := internal.BucketExists(ctx, config.Bucket)
	if err != nil {
		return storage, fmt.Errorf("检查 MinIO Bucket 失败: %w", err)
	}
	if !exists {
		if err := internal.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{}); err != nil {
			return storage, fmt.Errorf("创建 MinIO Bucket 失败: %w", err)
		}
	}
	storage.available.Store(true)
	return storage, nil
}

// Ready 返回本进程启动时对象存储与 Bucket 是否初始化成功。
func (storage *MinioStorage) Ready() bool {
	return storage != nil && storage.available.Load()
}

// Put 将已校验的图片流写入私有 Bucket。
func (storage *MinioStorage) Put(ctx context.Context, key string, contentType string, size int64, reader io.Reader) error {
	_, err := storage.internal.PutObject(ctx, storage.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// PresignedGet 为图片生成短期有效的私有对象读取地址。
func (storage *MinioStorage) PresignedGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	// 安全边界：私有 Bucket 不直接公开；Presigned URL 只在 expiry 时间内允许读取指定对象。
	value, err := storage.public.PresignedGetObject(ctx, storage.bucket, key, expiry, url.Values{})
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

// Get 读取公开品牌接口已确认可访问的私有图片对象。
func (storage *MinioStorage) Get(ctx context.Context, key string) (*Object, error) {
	object, err := storage.internal.GetObject(ctx, storage.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, err
	}
	return &Object{Body: object, ContentType: info.ContentType, Size: info.Size}, nil
}

// Delete 删除指定对象；调用方负责在数据库事务完成后执行尽力清理。
func (storage *MinioStorage) Delete(ctx context.Context, key string) error {
	return storage.internal.RemoveObject(ctx, storage.bucket, key, minio.RemoveObjectOptions{})
}

// UnavailableStorage 在配置缺失或初始化失败时稳定返回不可用状态。
type UnavailableStorage struct{}

// Ready 表示当前没有可用对象存储。
func (UnavailableStorage) Ready() bool { return false }

// Put 拒绝不可用状态下的对象写入。
func (UnavailableStorage) Put(context.Context, string, string, int64, io.Reader) error {
	return fmt.Errorf("图片存储服务不可用")
}

// PresignedGet 拒绝不可用状态下的临时地址签发。
func (UnavailableStorage) PresignedGet(context.Context, string, time.Duration) (string, error) {
	return "", fmt.Errorf("图片存储服务不可用")
}

// Get 拒绝不可用状态下的对象读取。
func (UnavailableStorage) Get(context.Context, string) (*Object, error) {
	return nil, fmt.Errorf("图片存储服务不可用")
}

// Delete 拒绝不可用状态下的对象删除。
func (UnavailableStorage) Delete(context.Context, string) error {
	return fmt.Errorf("图片存储服务不可用")
}
