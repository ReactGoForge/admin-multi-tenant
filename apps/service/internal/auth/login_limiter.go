package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	loginFailureWindow   = 10 * time.Minute
	loginLockDuration    = 15 * time.Minute
	loginAccountLimit    = 5
	loginIPLimit         = 20
	loginLimiterKeyRoot  = "admin-multi-tenant:login-limit:admin:"
	loginLimiterPingWait = 5 * time.Second
)

var errLoginSecurityUnavailable = errors.New("登录安全服务暂时不可用")

// loginLimitStore 定义登录限流所需的原子计数、锁定查询和清理能力。
type loginLimitStore interface {
	Locked(context.Context, string) (time.Duration, error)
	Increment(context.Context, string, int, time.Duration, time.Duration) (time.Duration, error)
	Delete(context.Context, string) error
	Close() error
}

// redisLoginLimitStore 使用 Redis 原子脚本维护失败窗口和锁定时间。
type redisLoginLimitStore struct {
	client *redis.Client
}

var incrementLoginFailureScript = redis.NewScript(`
local lockTTL = redis.call('PTTL', KEYS[2])
if lockTTL > 0 then
  return lockTTL
end
local attempts = redis.call('INCR', KEYS[1])
if attempts == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
if attempts >= tonumber(ARGV[1]) then
  redis.call('SET', KEYS[2], '1', 'PX', ARGV[3])
  redis.call('DEL', KEYS[1])
  return tonumber(ARGV[3])
end
return 0
`)

// Locked 返回指定账号或 IP 当前剩余锁定时间。
func (store *redisLoginLimitStore) Locked(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := store.client.PTTL(ctx, key+":lock").Result()
	if errors.Is(err, redis.Nil) || ttl < 0 {
		return 0, nil
	}
	return ttl, err
}

// Increment 原子增加失败次数，并在达到阈值时创建短期锁。
func (store *redisLoginLimitStore) Increment(ctx context.Context, key string, limit int, window, lockDuration time.Duration) (time.Duration, error) {
	value, err := incrementLoginFailureScript.Run(ctx, store.client, []string{key + ":failures", key + ":lock"}, limit, window.Milliseconds(), lockDuration.Milliseconds()).Int64()
	if err != nil {
		return 0, err
	}
	return time.Duration(value) * time.Millisecond, nil
}

// Delete 清除指定账号的失败计数和锁定状态。
func (store *redisLoginLimitStore) Delete(ctx context.Context, key string) error {
	return store.client.Del(ctx, key+":failures", key+":lock").Err()
}

// Ping 检查登录限流 Redis 连接当前是否可用。
func (store *redisLoginLimitStore) Ping(ctx context.Context) error {
	return store.client.Ping(ctx).Err()
}

// Close 关闭登录限流持有的 Redis 客户端。
func (store *redisLoginLimitStore) Close() error {
	return store.client.Close()
}

// LoginLimiter 按账号和客户端 IP 控制后台登录失败频率。
type LoginLimiter struct {
	enabled bool
	store   loginLimitStore
}

// NewLoginLimiter 根据认证配置创建登录限流器，并在启用时验证 Redis 可用性。
func NewLoginLimiter(ctx context.Context, config Config) (*LoginLimiter, error) {
	limiter := &LoginLimiter{enabled: config.LoginRateLimitEnabled}
	if !limiter.enabled {
		return limiter, nil
	}
	client := redis.NewClient(&redis.Options{Addr: config.RedisAddress, Password: config.RedisPassword})
	pingContext, cancel := context.WithTimeout(ctx, loginLimiterPingWait)
	defer cancel()
	if err := client.Ping(pingContext).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("登录限流 Redis 连接验证失败: %w", err)
	}
	limiter.store = &redisLoginLimitStore{client: client}
	return limiter, nil
}

// Check 检查账号或 IP 是否已被锁定，并返回最长剩余时间。
func (limiter *LoginLimiter) Check(ctx context.Context, account, clientIP string) (time.Duration, error) {
	if limiter == nil || !limiter.enabled {
		return 0, nil
	}
	accountTTL, err := limiter.store.Locked(ctx, loginAccountKey(account))
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errLoginSecurityUnavailable, err)
	}
	ipTTL, err := limiter.store.Locked(ctx, loginIPKey(clientIP))
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errLoginSecurityUnavailable, err)
	}
	return maxDuration(accountTTL, ipTTL), nil
}

// Ready 检查启用登录限流时依赖的 Redis；未启用时不产生外部连接。
func (limiter *LoginLimiter) Ready(ctx context.Context) error {
	if limiter == nil || !limiter.enabled {
		return nil
	}
	checker, ok := limiter.store.(interface {
		Ping(context.Context) error
	})
	if !ok {
		return errLoginSecurityUnavailable
	}
	if err := checker.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", errLoginSecurityUnavailable, err)
	}
	return nil
}

// RecordIPFailure 只累计当前 IP 的失败次数，适用于验证码错误。
func (limiter *LoginLimiter) RecordIPFailure(ctx context.Context, clientIP string) (time.Duration, error) {
	if limiter == nil || !limiter.enabled {
		return 0, nil
	}
	ttl, err := limiter.store.Increment(ctx, loginIPKey(clientIP), loginIPLimit, loginFailureWindow, loginLockDuration)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errLoginSecurityUnavailable, err)
	}
	return ttl, nil
}

// RecordCredentialFailure 同时累计账号和 IP，并返回任一维度触发的最长锁定时间。
func (limiter *LoginLimiter) RecordCredentialFailure(ctx context.Context, account, clientIP string) (time.Duration, error) {
	if limiter == nil || !limiter.enabled {
		return 0, nil
	}
	accountTTL, err := limiter.store.Increment(ctx, loginAccountKey(account), loginAccountLimit, loginFailureWindow, loginLockDuration)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errLoginSecurityUnavailable, err)
	}
	ipTTL, err := limiter.store.Increment(ctx, loginIPKey(clientIP), loginIPLimit, loginFailureWindow, loginLockDuration)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errLoginSecurityUnavailable, err)
	}
	return maxDuration(accountTTL, ipTTL), nil
}

// ClearAccount 在登录成功后清除当前账号的失败计数和锁定状态。
func (limiter *LoginLimiter) ClearAccount(ctx context.Context, account string) error {
	if limiter == nil || !limiter.enabled {
		return nil
	}
	if err := limiter.store.Delete(ctx, loginAccountKey(account)); err != nil {
		return fmt.Errorf("%w: %v", errLoginSecurityUnavailable, err)
	}
	return nil
}

// Close 释放登录限流持有的 Redis 客户端。
func (limiter *LoginLimiter) Close() error {
	if limiter == nil || limiter.store == nil {
		return nil
	}
	return limiter.store.Close()
}

// loginAccountKey 对标准化账号做摘要，避免 Redis Key 暴露登录账号。
func loginAccountKey(account string) string {
	return loginLimiterKeyRoot + "account:" + loginLimitDigest(strings.ToLower(strings.TrimSpace(account)))
}

// loginIPKey 对客户端 IP 做摘要，避免 Redis Key 暴露网络地址。
func loginIPKey(clientIP string) string {
	return loginLimiterKeyRoot + "ip:" + loginLimitDigest(strings.TrimSpace(clientIP))
}

// loginLimitDigest 返回限流键使用的固定长度 SHA-256 十六进制摘要。
func loginLimitDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// maxDuration 返回两个时长中的较大值。
func maxDuration(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}
