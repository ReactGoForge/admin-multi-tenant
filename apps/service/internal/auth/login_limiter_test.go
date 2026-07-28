package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// memoryLoginLimitStore 在单元测试中模拟 Redis 计数、锁定和故障。
type memoryLoginLimitStore struct {
	counts map[string]int
	locks  map[string]time.Duration
	err    error
}

// Locked 返回测试键的锁定时长，或模拟 Redis 异常。
func (store *memoryLoginLimitStore) Locked(_ context.Context, key string) (time.Duration, error) {
	if store.err != nil {
		return 0, store.err
	}
	return store.locks[key], nil
}

// Increment 增加测试计数，并在达到阈值时写入锁定时长。
func (store *memoryLoginLimitStore) Increment(_ context.Context, key string, limit int, _ time.Duration, lockDuration time.Duration) (time.Duration, error) {
	if store.err != nil {
		return 0, store.err
	}
	if ttl := store.locks[key]; ttl > 0 {
		return ttl, nil
	}
	store.counts[key]++
	if store.counts[key] >= limit {
		delete(store.counts, key)
		store.locks[key] = lockDuration
		return lockDuration, nil
	}
	return 0, nil
}

// Delete 清除测试账号的失败计数和锁定。
func (store *memoryLoginLimitStore) Delete(_ context.Context, key string) error {
	if store.err != nil {
		return store.err
	}
	delete(store.counts, key)
	delete(store.locks, key)
	return nil
}

// Close 结束内存 Store，不需要释放资源。
func (store *memoryLoginLimitStore) Close() error { return nil }

// newMemoryLoginLimiter 创建启用状态的内存登录限流器。
func newMemoryLoginLimiter() (*LoginLimiter, *memoryLoginLimitStore) {
	store := &memoryLoginLimitStore{counts: map[string]int{}, locks: map[string]time.Duration{}}
	return &LoginLimiter{enabled: true, store: store}, store
}

// TestLoginLimiterThresholdsAndAccountClear 验证账号第五次、IP 第二十次触发锁定且成功可清除账号计数。
func TestLoginLimiterThresholdsAndAccountClear(t *testing.T) {
	limiter, _ := newMemoryLoginLimiter()
	ctx := context.Background()
	for attempt := 1; attempt <= loginAccountLimit; attempt++ {
		ttl, err := limiter.RecordCredentialFailure(ctx, "Admin", "192.0.2.1")
		if err != nil {
			t.Fatalf("RecordCredentialFailure() 返回错误: %v", err)
		}
		if attempt < loginAccountLimit && ttl != 0 {
			t.Fatalf("账号第 %d 次失败提前锁定: %v", attempt, ttl)
		}
		if attempt == loginAccountLimit && ttl != loginLockDuration {
			t.Fatalf("账号第五次锁定 = %v，期望 %v", ttl, loginLockDuration)
		}
	}

	secondLimiter, _ := newMemoryLoginLimiter()
	for attempt := 1; attempt <= loginIPLimit; attempt++ {
		ttl, err := secondLimiter.RecordIPFailure(ctx, "198.51.100.8")
		if err != nil {
			t.Fatalf("RecordIPFailure() 返回错误: %v", err)
		}
		if attempt == loginIPLimit && ttl != loginLockDuration {
			t.Fatalf("IP 第二十次锁定 = %v，期望 %v", ttl, loginLockDuration)
		}
	}

	clearLimiter, clearStore := newMemoryLoginLimiter()
	if _, err := clearLimiter.RecordCredentialFailure(ctx, "admin", "203.0.113.3"); err != nil {
		t.Fatal(err)
	}
	if err := clearLimiter.ClearAccount(ctx, "admin"); err != nil {
		t.Fatal(err)
	}
	if clearStore.counts[loginAccountKey("admin")] != 0 || clearStore.counts[loginIPKey("203.0.113.3")] != 1 {
		t.Fatalf("成功清理范围错误: %#v", clearStore.counts)
	}
	accountKey := loginAccountKey("Admin")
	ipKey := loginIPKey("192.0.2.1")
	accountDigest := strings.TrimPrefix(accountKey, loginLimiterKeyRoot+"account:")
	ipDigest := strings.TrimPrefix(ipKey, loginLimiterKeyRoot+"ip:")
	if len(accountDigest) != 64 || len(ipDigest) != 64 || strings.Contains(ipKey, "192.0.2.1") {
		t.Fatal("限流 Redis Key 不应包含账号或 IP 明文")
	}
}

// TestLoginLimiterStoreFailureReturnsServiceUnavailable 验证运行期 Redis 异常安全失败为 503。
func TestLoginLimiterStoreFailureReturnsServiceUnavailable(t *testing.T) {
	limiter, store := newMemoryLoginLimiter()
	store.err = errors.New("redis unavailable")
	handler := newTestHandler(&testEmployeeStore{})
	loginLogs := &recordingLoginStore{}
	handler.ConfigureLoginSecurity(limiter, loginLogs)
	recorder := performJSONRequest(newTestRouter(handler), http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"secret123"}`, "")
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":50003`) {
		t.Fatalf("Redis 异常响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if len(loginLogs.entries) != 1 || !strings.Contains(string(loginLogs.entries[0].Metadata), `"reason":"security_unavailable"`) {
		t.Fatalf("安全服务异常登录日志 = %#v", loginLogs.entries)
	}
}

// TestLockedLoginReturnsRetryAfterAndLimitedLog 验证已锁定账号返回 429、十五分钟提示和限流日志。
func TestLockedLoginReturnsRetryAfterAndLimitedLog(t *testing.T) {
	limiter, store := newMemoryLoginLimiter()
	store.locks[loginAccountKey("admin")] = loginLockDuration
	handler := newTestHandler(&testEmployeeStore{})
	loginLogs := &recordingLoginStore{}
	handler.ConfigureLoginSecurity(limiter, loginLogs)
	recorder := performJSONRequest(newTestRouter(handler), http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"secret123"}`, "")
	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "900" {
		t.Fatalf("限流响应 = %d Retry-After=%s %s", recorder.Code, recorder.Header().Get("Retry-After"), recorder.Body.String())
	}
	if len(loginLogs.entries) != 1 || !strings.Contains(string(loginLogs.entries[0].Metadata), `"reason":"rate_limited"`) {
		t.Fatalf("限流登录日志 = %#v", loginLogs.entries)
	}
}
