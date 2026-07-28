package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	base64Captcha "github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

type memoryCaptchaStore struct {
	values    map[string]string
	ttl       time.Duration
	setCalls  int
	getCalls  int
	closed    bool
	available bool
}

// Set 在测试内存中保存验证码答案和有效期。
func (store *memoryCaptchaStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	store.setCalls++
	if !store.available {
		return errors.New("store unavailable")
	}
	store.values[key] = value
	store.ttl = ttl
	return nil
}

// GetDel 在测试内存中模拟 Redis 的原子读取删除行为。
func (store *memoryCaptchaStore) GetDel(_ context.Context, key string) (string, error) {
	store.getCalls++
	if !store.available {
		return "", errors.New("store unavailable")
	}
	value, exists := store.values[key]
	if !exists {
		return "", redis.Nil
	}
	delete(store.values, key)
	return value, nil
}

// Close 记录测试验证码存储已经关闭。
func (store *memoryCaptchaStore) Close() error {
	store.closed = true
	return nil
}

// newMemoryCaptchaManager 创建使用可控随机源和内存存储的验证码管理器。
func newMemoryCaptchaManager(store *memoryCaptchaStore) *CaptchaManager {
	return &CaptchaManager{
		enabled: true,
		store:   store,
		driver:  base64Captcha.NewDriverDigit(42, 120, captchaLength, 0.5, 30),
		random:  bytes.NewReader(make([]byte, 28)),
	}
}

// TestCaptchaGenerateAndConsume 验证图片生成、120 秒有效期和一次性消费。
func TestCaptchaGenerateAndConsume(t *testing.T) {
	store := &memoryCaptchaStore{values: make(map[string]string), available: true}
	manager := newMemoryCaptchaManager(store)

	result, err := manager.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() 返回错误: %v", err)
	}
	if result.ID == "" || !strings.HasPrefix(result.Image, "data:image/png;base64,") {
		t.Fatalf("Generate() 返回无效结果: %#v", result)
	}
	if result.ExpiresIn != 120 || store.ttl != captchaTTL {
		t.Fatalf("验证码有效期 = %d / %s，期望 120 秒", result.ExpiresIn, store.ttl)
	}
	if err := manager.Verify(context.Background(), result.ID, "0000"); err != nil {
		t.Fatalf("Verify() 首次校验失败: %v", err)
	}
	if err := manager.Verify(context.Background(), result.ID, "0000"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("Verify() 重复校验错误 = %v，期望 ErrCaptchaInvalid", err)
	}
}

// TestCaptchaWrongAnswerConsumesValue 验证错误答案同样立即消费验证码。
func TestCaptchaWrongAnswerConsumesValue(t *testing.T) {
	store := &memoryCaptchaStore{values: make(map[string]string), available: true}
	manager := newMemoryCaptchaManager(store)
	result, err := manager.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() 返回错误: %v", err)
	}

	if err := manager.Verify(context.Background(), result.ID, "9999"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("错误答案返回 = %v，期望 ErrCaptchaInvalid", err)
	}
	if err := manager.Verify(context.Background(), result.ID, "0000"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("错误后重复校验返回 = %v，期望 ErrCaptchaInvalid", err)
	}
}

// TestCaptchaExpiredAndUnavailable 验证过期键和 Redis 故障得到不同错误。
func TestCaptchaExpiredAndUnavailable(t *testing.T) {
	store := &memoryCaptchaStore{values: make(map[string]string), available: true}
	manager := newMemoryCaptchaManager(store)
	if err := manager.Verify(context.Background(), "expired", "0000"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("过期验证码返回 = %v，期望 ErrCaptchaInvalid", err)
	}

	store.available = false
	if err := manager.Verify(context.Background(), "any", "0000"); !errors.Is(err, ErrCaptchaUnavailable) {
		t.Fatalf("存储故障返回 = %v，期望 ErrCaptchaUnavailable", err)
	}
}

// TestCaptchaDisabledDoesNotUseStore 验证关闭验证码时不访问或关闭 Redis 存储。
func TestCaptchaDisabledDoesNotUseStore(t *testing.T) {
	manager, err := NewCaptchaManager(context.Background(), Config{CaptchaEnabled: false})
	if err != nil {
		t.Fatalf("NewCaptchaManager() 返回错误: %v", err)
	}

	if _, err := manager.Generate(context.Background()); err != nil {
		t.Fatalf("Generate() 返回错误: %v", err)
	}
	if err := manager.Verify(context.Background(), "", ""); err != nil {
		t.Fatalf("Verify() 返回错误: %v", err)
	}
	if manager.store != nil {
		t.Fatal("关闭验证码时不应初始化 Redis 存储")
	}
}
