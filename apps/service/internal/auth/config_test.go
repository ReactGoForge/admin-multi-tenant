package auth

import (
	"os"
	"strings"
	"testing"
)

// TestLoadConfigDefaults 验证验证码默认启用并使用约定的本地 Redis 地址。
func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", minimumJWTBytes))
	previousEnabled, enabledExists := os.LookupEnv("CAPTCHA_ENABLED")
	if err := os.Unsetenv("CAPTCHA_ENABLED"); err != nil {
		t.Fatalf("清理 CAPTCHA_ENABLED 失败: %v", err)
	}
	t.Cleanup(func() {
		if enabledExists {
			_ = os.Setenv("CAPTCHA_ENABLED", previousEnabled)
			return
		}
		_ = os.Unsetenv("CAPTCHA_ENABLED")
	})
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "redis-password")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() 返回错误: %v", err)
	}
	if !config.CaptchaEnabled {
		t.Fatal("CaptchaEnabled = false，期望 true")
	}
	if !config.LoginRateLimitEnabled {
		t.Fatal("LoginRateLimitEnabled = false，期望 true")
	}
	if config.RedisAddress != "127.0.0.1:16379" {
		t.Fatalf("RedisAddress = %q，期望 127.0.0.1:16379", config.RedisAddress)
	}
}

// TestLoadConfigCaptchaDisabled 验证同时关闭验证码和登录限流时完全忽略 Redis 配置。
func TestLoadConfigCaptchaDisabled(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", minimumJWTBytes))
	t.Setenv("APP_ENV", "development")
	t.Setenv("CAPTCHA_ENABLED", "false")
	t.Setenv("LOGIN_RATE_LIMIT_ENABLED", "false")
	t.Setenv("REDIS_HOST", "invalid host")
	t.Setenv("REDIS_PORT", "invalid")
	t.Setenv("REDIS_PASSWORD", "")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() 返回错误: %v", err)
	}
	if config.CaptchaEnabled {
		t.Fatal("CaptchaEnabled = true，期望 false")
	}
	if config.LoginRateLimitEnabled {
		t.Fatal("LoginRateLimitEnabled = true，期望 false")
	}
	if config.RedisAddress != "" || config.RedisPassword != "" {
		t.Fatal("关闭验证码时不应读取 Redis 配置")
	}
}

// TestLoadConfigRejectsDisabledRateLimitInProduction 验证非本地环境不能关闭登录限流。
func TestLoadConfigRejectsDisabledRateLimitInProduction(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", minimumJWTBytes))
	t.Setenv("APP_ENV", "production")
	t.Setenv("CAPTCHA_ENABLED", "false")
	t.Setenv("LOGIN_RATE_LIMIT_ENABLED", "false")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("生产环境关闭登录限流未返回错误")
	}
}

// TestLoadConfigRejectsInvalidValues 验证不安全密钥和非法布尔值会阻止启动。
func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name           string
		secret         string
		captchaEnabled string
	}{
		{name: "密钥过短", secret: "short", captchaEnabled: "false"},
		{name: "验证码开关非法", secret: strings.Repeat("s", minimumJWTBytes), captchaEnabled: "1"},
		{name: "验证码开关大小写非法", secret: strings.Repeat("s", minimumJWTBytes), captchaEnabled: "TRUE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", test.secret)
			t.Setenv("CAPTCHA_ENABLED", test.captchaEnabled)
			if _, err := LoadConfig(); err == nil {
				t.Fatal("LoadConfig() 未返回预期错误")
			}
		})
	}
}
