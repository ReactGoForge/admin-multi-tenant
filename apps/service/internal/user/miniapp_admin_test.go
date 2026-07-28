package user

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type testMiniappSettingsStore struct {
	appID   string
	tenant  *TenantOption
	savedID string
}

// GetMiniappAppID 返回测试 AppID。
func (store *testMiniappSettingsStore) GetMiniappAppID(_ context.Context) (string, error) {
	return store.appID, nil
}

// SaveMiniappAppID 记录测试保存值。
func (store *testMiniappSettingsStore) SaveMiniappAppID(_ context.Context, appID string) error {
	store.savedID = appID
	return nil
}

// FindTenantOption 返回测试租户。
func (store *testMiniappSettingsStore) FindTenantOption(_ context.Context, _ uint64) (*TenantOption, error) {
	return store.tenant, nil
}

type testMiniappCodeGenerator struct {
	images [][]byte
	calls  int
}

// GenerateUnlimitedCode 返回测试 PNG 原始字节并记录调用次数。
func (generator *testMiniappCodeGenerator) GenerateUnlimitedCode(_ context.Context, _, _ string) ([]byte, error) {
	index := generator.calls
	generator.calls++
	if len(generator.images) == 0 {
		return testMiniappPNG("default"), nil
	}
	if index >= len(generator.images) {
		index = len(generator.images) - 1
	}
	return generator.images[index], nil
}

type testMiniappCodeCache struct {
	ready           bool
	objects         map[string][]byte
	getErr          error
	putErr          error
	puts            int
	lastContentType string
}

// Ready 返回测试缓存是否可用。
func (cache *testMiniappCodeCache) Ready() bool {
	return cache.ready
}

// Get 从测试缓存读取对象。
func (cache *testMiniappCodeCache) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	if cache.getErr != nil {
		return nil, 0, cache.getErr
	}
	content, ok := cache.objects[key]
	if !ok {
		return nil, 0, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(content)), int64(len(content)), nil
}

// Put 将对象写入测试缓存。
func (cache *testMiniappCodeCache) Put(_ context.Context, key, contentType string, _ int64, reader io.Reader) error {
	if cache.putErr != nil {
		return cache.putErr
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if cache.objects == nil {
		cache.objects = make(map[string][]byte)
	}
	cache.objects[key] = content
	cache.puts++
	cache.lastContentType = contentType
	return nil
}

// testMiniappPNG 创建带合法 PNG 签名的测试内容。
func testMiniappPNG(value string) []byte {
	return append(append([]byte{}, wechatPNGSignature...), []byte(value)...)
}

// testMiniappJPEG 创建带合法 JPEG 签名的测试内容。
func testMiniappJPEG(value string) []byte {
	return append(append([]byte{}, wechatJPEGSignature...), []byte(value)...)
}

// testMiniappDataURL 返回测试图片对应的数据地址。
func testMiniappDataURL(content []byte) string {
	image, _ := miniappCodeDataURL(content)
	return image
}

// TestMiniappSettingsNeverExposeSecret 验证微信配置接口仅返回密钥配置状态。
func TestMiniappSettingsNeverExposeSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &testMiniappSettingsStore{appID: "wx123"}
	handler := NewMiniappAdminHandler(NewMiniappAdminService(store, &testMiniappCodeGenerator{}, true))
	router := gin.New()
	router.GET("/settings", handler.GetSettings)
	router.PUT("/settings", handler.UpdateSettings)
	response := performUserRequest(router, http.MethodGet, "/settings", "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"appId":"wx123"`) || !strings.Contains(response.Body.String(), `"secretConfigured":true`) || strings.Contains(strings.ToLower(response.Body.String()), "appsecret") {
		t.Fatalf("微信配置响应 = %d %s", response.Code, response.Body.String())
	}
	response = performUserRequest(router, http.MethodPut, "/settings", `{"appId":" wx456 "}`, "")
	if response.Code != http.StatusOK || store.savedID != "wx456" {
		t.Fatalf("微信配置保存 = %d %s, %q", response.Code, response.Body.String(), store.savedID)
	}
}

// TestTenantMiniappCodeReturnsDataURL 验证租户小程序码使用统一 JSON 响应返回图片。
func TestTenantMiniappCodeReturnsDataURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &testMiniappSettingsStore{appID: "wx123", tenant: &TenantOption{ID: 9, Name: "租户", Status: 1}}
	generator := &testMiniappCodeGenerator{images: [][]byte{testMiniappJPEG("first"), testMiniappJPEG("second")}}
	handler := NewMiniappAdminHandler(NewMiniappAdminService(store, generator, false))
	router := gin.New()
	router.GET("/tenants/:tenantId/code", handler.TenantMiniappCode)
	router.POST("/tenants/:tenantId/code", handler.TenantMiniappCode)
	response := performUserRequest(router, http.MethodGet, "/tenants/9/code", "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), testMiniappDataURL(testMiniappJPEG("first"))) || !strings.Contains(response.Body.String(), `"extension":"jpg"`) {
		t.Fatalf("小程序码响应 = %d %s", response.Code, response.Body.String())
	}
	response = performUserRequest(router, http.MethodPost, "/tenants/9/code", "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), testMiniappDataURL(testMiniappJPEG("second"))) || generator.calls != 2 {
		t.Fatalf("重新生成响应 = %d %s，调用次数 = %d", response.Code, response.Body.String(), generator.calls)
	}
}

// TestTenantMiniappCodeUsesCacheAndRegenerates 验证普通查看复用缓存，强制刷新覆盖缓存。
func TestTenantMiniappCodeUsesCacheAndRegenerates(t *testing.T) {
	store := &testMiniappSettingsStore{appID: "wx123", tenant: &TenantOption{ID: 9, Name: "租户", Status: 1}}
	generator := &testMiniappCodeGenerator{images: [][]byte{testMiniappPNG("first"), testMiniappPNG("second")}}
	cache := &testMiniappCodeCache{ready: true, objects: make(map[string][]byte)}
	service := NewMiniappAdminService(store, generator, true)
	service.ConfigureMiniappCodeCache(cache, "develop")

	first, firstExtension, err := service.TenantMiniappCode(context.Background(), 9, false)
	if err != nil {
		t.Fatalf("首次生成错误 = %v", err)
	}
	second, secondExtension, err := service.TenantMiniappCode(context.Background(), 9, false)
	if err != nil || second != first || firstExtension != "png" || secondExtension != "png" || generator.calls != 1 || cache.puts != 1 || cache.lastContentType != "image/png" {
		t.Fatalf("缓存读取 = %q, %v，调用次数 = %d，写入次数 = %d", second, err, generator.calls, cache.puts)
	}
	regenerated, regeneratedExtension, err := service.TenantMiniappCode(context.Background(), 9, true)
	if err != nil || regenerated == first || regeneratedExtension != "png" || generator.calls != 2 || cache.puts != 2 {
		t.Fatalf("强制刷新 = %q, %v，调用次数 = %d，写入次数 = %d", regenerated, err, generator.calls, cache.puts)
	}
}

// TestTenantMiniappCodeCacheFailureFallsBackToWechat 验证缓存异常不阻止微信生成结果。
func TestTenantMiniappCodeCacheFailureFallsBackToWechat(t *testing.T) {
	store := &testMiniappSettingsStore{appID: "wx123", tenant: &TenantOption{ID: 9, Name: "租户", Status: 1}}
	generator := &testMiniappCodeGenerator{images: [][]byte{testMiniappPNG("generated")}}
	cache := &testMiniappCodeCache{
		ready:   true,
		objects: make(map[string][]byte),
		getErr:  errors.New("read failed"),
		putErr:  errors.New("write failed"),
	}
	service := NewMiniappAdminService(store, generator, true)
	service.ConfigureMiniappCodeCache(cache, "develop")

	image, extension, err := service.TenantMiniappCode(context.Background(), 9, false)
	if err != nil || image != testMiniappDataURL(testMiniappPNG("generated")) || extension != "png" || generator.calls != 1 {
		t.Fatalf("缓存异常降级 = %q, %v，调用次数 = %d", image, err, generator.calls)
	}
}

// TestMiniappCodeCacheKeyIsolatesRequestIdentity 验证 AppID、版本和租户场景分别隔离缓存。
func TestMiniappCodeCacheKeyIsolatesRequestIdentity(t *testing.T) {
	base := miniappCodeCacheKey("wx123", "develop", "9")
	for _, other := range []string{
		miniappCodeCacheKey("wx456", "develop", "9"),
		miniappCodeCacheKey("wx123", "trial", "9"),
		miniappCodeCacheKey("wx123", "develop", "10"),
	} {
		if other == base {
			t.Fatalf("缓存键未隔离: %q", base)
		}
	}
}
