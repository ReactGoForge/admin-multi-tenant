package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	base64Captcha "github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

const (
	captchaKeyPrefix = "admin-multi-tenant:captcha:admin:"
	captchaLength    = 4
	captchaTTL       = 120 * time.Second
	redisPingTimeout = 5 * time.Second
)

var (
	// ErrCaptchaInvalid 表示验证码不存在、已过期或与输入不一致。
	ErrCaptchaInvalid = errors.New("验证码错误或已过期")
	// ErrCaptchaUnavailable 表示验证码存储暂时不可用。
	ErrCaptchaUnavailable = errors.New("验证码服务暂时不可用")
)

// captchaStore 定义验证码管理器需要的最小存储能力。
// Go 学习提示：调用方依赖接口而不是 Redis 具体类型，既便于替换实现，也便于单元测试使用内存假实现。
type captchaStore interface {
	Set(context.Context, string, string, time.Duration) error
	GetDel(context.Context, string) (string, error)
	Close() error
}

// redisCaptchaStore 使用 Redis 实现验证码的限时保存和一次性读取。
type redisCaptchaStore struct {
	client *redis.Client
}

// Set 将验证码答案按指定有效期写入 Redis。
func (store *redisCaptchaStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return store.client.Set(ctx, key, value, ttl).Err()
}

// GetDel 原子读取并删除验证码，保证每个验证码最多校验一次。
func (store *redisCaptchaStore) GetDel(ctx context.Context, key string) (string, error) {
	// 安全边界：GETDEL 是一条原子命令，读取成功的同时立即删除，
	// 并发请求也不能重复使用同一个验证码。
	return store.client.GetDel(ctx, key).Result()
}

// Ping 检查验证码 Redis 连接当前是否可用。
func (store *redisCaptchaStore) Ping(ctx context.Context) error {
	return store.client.Ping(ctx).Err()
}

// Close 关闭验证码使用的 Redis 客户端。
func (store *redisCaptchaStore) Close() error {
	return store.client.Close()
}

// CaptchaManager 负责生成数字验证码图片，并通过一次性 Redis 数据完成校验。
type CaptchaManager struct {
	enabled bool
	store   captchaStore
	driver  *base64Captcha.DriverDigit
	random  io.Reader
}

// CaptchaResult 描述一次验证码生成结果。
type CaptchaResult struct {
	ID        string
	Image     string
	ExpiresIn int
}

// NewCaptchaManager 按认证配置创建验证码管理器，并在启用时验证 Redis 连接。
func NewCaptchaManager(ctx context.Context, config Config) (*CaptchaManager, error) {
	manager := &CaptchaManager{
		enabled: config.CaptchaEnabled,
		driver:  base64Captcha.NewDriverDigit(42, 120, captchaLength, 0.5, 30),
		random:  rand.Reader,
	}
	if !config.CaptchaEnabled {
		return manager, nil
	}

	// 业务约束：只有明确启用验证码时才创建和检查 Redis，关闭时本地启动不依赖 Redis。
	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddress,
		Password: config.RedisPassword,
	})
	pingContext, cancel := context.WithTimeout(ctx, redisPingTimeout)
	defer cancel()
	if err := client.Ping(pingContext).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("Redis 连接验证失败: %w", err)
	}
	manager.store = &redisCaptchaStore{client: client}
	return manager, nil
}

// Enabled 返回当前进程是否启用了验证码校验。
func (manager *CaptchaManager) Enabled() bool {
	return manager.enabled
}

// Ready 检查启用验证码时依赖的 Redis；未启用时不产生外部连接。
func (manager *CaptchaManager) Ready(ctx context.Context) error {
	if manager == nil || !manager.enabled {
		return nil
	}
	checker, ok := manager.store.(interface {
		Ping(context.Context) error
	})
	if !ok {
		return ErrCaptchaUnavailable
	}
	if err := checker.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}
	return nil
}

// Generate 使用安全随机数生成验证码标识和答案，并返回对应 PNG Data URL。
func (manager *CaptchaManager) Generate(ctx context.Context) (CaptchaResult, error) {
	if !manager.enabled {
		return CaptchaResult{}, nil
	}

	// 安全边界：验证码 ID 和答案都使用 crypto/rand，而不是可预测的普通伪随机数。
	idBytes := make([]byte, 24)
	if _, err := io.ReadFull(manager.random, idBytes); err != nil {
		return CaptchaResult{}, fmt.Errorf("生成验证码标识失败: %w", err)
	}
	id := base64.RawURLEncoding.EncodeToString(idBytes)

	digitBytes := make([]byte, captchaLength)
	if _, err := io.ReadFull(manager.random, digitBytes); err != nil {
		return CaptchaResult{}, fmt.Errorf("生成验证码答案失败: %w", err)
	}
	for index := range digitBytes {
		digitBytes[index] = '0' + digitBytes[index]%10
	}
	answer := string(digitBytes)

	item, err := manager.driver.DrawCaptcha(answer)
	if err != nil {
		return CaptchaResult{}, fmt.Errorf("绘制验证码失败: %w", err)
	}
	if err := manager.store.Set(ctx, captchaKeyPrefix+id, answer, captchaTTL); err != nil {
		return CaptchaResult{}, fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}

	return CaptchaResult{
		ID:        id,
		Image:     item.EncodeB64string(),
		ExpiresIn: int(captchaTTL / time.Second),
	}, nil
}

// Verify 原子消费验证码并使用常量时间比较输入答案。
func (manager *CaptchaManager) Verify(ctx context.Context, id, code string) error {
	if !manager.enabled {
		return nil
	}

	// 即使答案错误，GetDel 也已经消费验证码，防止攻击者对同一答案反复尝试。
	answer, err := manager.store.GetDel(ctx, captchaKeyPrefix+id)
	if errors.Is(err, redis.Nil) {
		return ErrCaptchaInvalid
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}
	// 安全边界：常量时间比较减少根据响应耗时推测正确答案的机会。
	if len(answer) != len(code) || subtle.ConstantTimeCompare([]byte(answer), []byte(code)) != 1 {
		return ErrCaptchaInvalid
	}
	return nil
}

// Close 释放验证码管理器持有的 Redis 客户端；关闭验证码时不执行任何连接操作。
func (manager *CaptchaManager) Close() error {
	if manager.store == nil {
		return nil
	}
	return manager.store.Close()
}
