package user

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// MiniappAdminApplication 定义微信配置和小程序码 Handler 依赖的业务服务能力。
type MiniappAdminApplication interface {
	GetSettings(context.Context) (MiniappSettings, error)
	UpdateSettings(context.Context, string) error
	TenantMiniappCode(context.Context, uint64, bool) (string, string, error)
}

// MiniappAdminDataStore 定义微信配置和小程序码 Service 需要的数据能力。
type MiniappAdminDataStore interface {
	GetMiniappAppID(context.Context) (string, error)
	SaveMiniappAppID(context.Context, string) error
	FindTenantOption(context.Context, uint64) (*TenantOption, error)
}

// MiniappSettings 描述平台微信小程序配置的可公开字段。
type MiniappSettings struct {
	AppID            string
	SecretConfigured bool
}

// miniappCodeCache 定义小程序码缓存实际使用的最小对象存储能力。
type miniappCodeCache interface {
	Ready() bool
	Put(context.Context, string, string, int64, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, int64, error)
}

// MiniappAdminService 编排平台微信小程序配置和租户小程序码生成。
type MiniappAdminService struct {
	store            MiniappAdminDataStore
	codes            MiniappCodeGenerator
	secretConfigured bool
	codeCache        miniappCodeCache
	codeEnvVersion   string
	codeMutex        sync.Mutex
}

// NewMiniappAdminService 使用配置数据和微信小程序码能力创建业务服务。
func NewMiniappAdminService(store MiniappAdminDataStore, codes MiniappCodeGenerator, secretConfigured bool) *MiniappAdminService {
	return &MiniappAdminService{store: store, codes: codes, secretConfigured: secretConfigured}
}

// ConfigureMiniappCodeCache 配置小程序码使用的私有对象存储和目标版本。
func (service *MiniappAdminService) ConfigureMiniappCodeCache(cache miniappCodeCache, envVersion string) {
	service.codeCache = cache
	service.codeEnvVersion = strings.TrimSpace(envVersion)
}

// GetSettings 返回 AppID 与服务器密钥配置状态，不读取或返回密钥内容。
func (service *MiniappAdminService) GetSettings(ctx context.Context) (MiniappSettings, error) {
	appID, err := service.store.GetMiniappAppID(ctx)
	if err != nil {
		return MiniappSettings{}, err
	}
	return MiniappSettings{AppID: appID, SecretConfigured: service.secretConfigured}, nil
}

// UpdateSettings 保存全平台唯一微信小程序 AppID。
func (service *MiniappAdminService) UpdateSettings(ctx context.Context, appID string) error {
	return service.store.SaveMiniappAppID(ctx, appID)
}

// TenantMiniappCode 读取或强制重新生成携带指定租户场景值的小程序码。
func (service *MiniappAdminService) TenantMiniappCode(ctx context.Context, tenantID uint64, regenerate bool) (string, string, error) {
	tenant, err := service.store.FindTenantOption(ctx, tenantID)
	if err != nil {
		return "", "", err
	}
	if tenant == nil {
		return "", "", errTenantNotFound
	}
	appID, err := service.store.GetMiniappAppID(ctx)
	if err != nil || strings.TrimSpace(appID) == "" {
		return "", "", errWechatUnavailable
	}
	appID = strings.TrimSpace(appID)
	scene := strconv.FormatUint(tenantID, 10)
	cacheKey := miniappCodeCacheKey(appID, service.codeEnvVersion, scene)
	if !regenerate {
		if content, ok := service.loadMiniappCodeCache(ctx, cacheKey); ok {
			image, extension := miniappCodeDataURL(content)
			return image, extension, nil
		}
	}

	// 首次缓存未命中和手动重新生成串行执行；进入锁后再次检查，避免并发首次查看重复调用微信。
	service.codeMutex.Lock()
	defer service.codeMutex.Unlock()
	if !regenerate {
		if content, ok := service.loadMiniappCodeCache(ctx, cacheKey); ok {
			image, extension := miniappCodeDataURL(content)
			return image, extension, nil
		}
	}
	content, err := service.codes.GenerateUnlimitedCode(ctx, appID, scene)
	if err != nil {
		return "", "", err
	}
	service.saveMiniappCodeCache(ctx, cacheKey, content)
	image, extension := miniappCodeDataURL(content)
	return image, extension, nil
}

// miniappCodeCacheKey 根据会影响太阳码内容的稳定参数生成私有对象键。
func miniappCodeCacheKey(appID, envVersion, scene string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{"v2", appID, envVersion, scene}, "\x00")))
	return fmt.Sprintf("miniapp-codes/v2/%s.image", hex.EncodeToString(digest[:]))
}

// loadMiniappCodeCache 从私有对象存储读取并校验小程序码图片。
func (service *MiniappAdminService) loadMiniappCodeCache(ctx context.Context, key string) ([]byte, bool) {
	if service.codeCache == nil || !service.codeCache.Ready() {
		return nil, false
	}
	body, size, err := service.codeCache.Get(ctx, key)
	if err != nil || body == nil {
		if body != nil {
			_ = body.Close()
		}
		return nil, false
	}
	if size > wechatMaximumBodyBytes {
		_ = body.Close()
		return nil, false
	}
	defer body.Close()
	content, err := io.ReadAll(io.LimitReader(body, wechatMaximumBodyBytes+1))
	if _, _, ok := miniappCodeImageFormat(content); err != nil || len(content) > wechatMaximumBodyBytes || !ok {
		return nil, false
	}
	return content, true
}

// saveMiniappCodeCache 尽力把微信生成成功的图片按真实 MIME 保存到私有对象存储。
func (service *MiniappAdminService) saveMiniappCodeCache(ctx context.Context, key string, content []byte) {
	if service.codeCache == nil || !service.codeCache.Ready() {
		return
	}
	contentType, _, ok := miniappCodeImageFormat(content)
	if !ok {
		return
	}
	_ = service.codeCache.Put(ctx, key, contentType, int64(len(content)), bytes.NewReader(content))
}

// miniappCodeDataURL 将已校验的图片转为管理后台可直接展示的数据和扩展名。
func miniappCodeDataURL(content []byte) (string, string) {
	contentType, extension, _ := miniappCodeImageFormat(content)
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(content), extension
}
