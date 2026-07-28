package auth

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultRedisHost = "127.0.0.1"
	defaultRedisPort = "16379"
	minimumJWTBytes  = 32
)

// Config 描述后台登录、JWT 与可选验证码服务所需的配置。
type Config struct {
	JWTSecret             string
	CaptchaEnabled        bool
	LoginRateLimitEnabled bool
	RedisAddress          string
	RedisPassword         string
}

// LoadConfig 从环境变量读取认证配置，并拒绝不安全或无法解析的值。
func LoadConfig() (Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if len([]byte(secret)) < minimumJWTBytes {
		return Config{}, fmt.Errorf("JWT_SECRET 必填且至少需要 %d 字节", minimumJWTBytes)
	}

	captchaEnabled, err := optionalBoolean("CAPTCHA_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	loginRateLimitEnabled, err := optionalBoolean("LOGIN_RATE_LIMIT_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	appEnvironment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnvironment == "" {
		appEnvironment = "development"
	}
	if !loginRateLimitEnabled && appEnvironment != "development" && appEnvironment != "local" && appEnvironment != "test" {
		return Config{}, fmt.Errorf("LOGIN_RATE_LIMIT_ENABLED 只能在本地开发或测试环境关闭")
	}

	config := Config{
		JWTSecret:             secret,
		CaptchaEnabled:        captchaEnabled,
		LoginRateLimitEnabled: loginRateLimitEnabled,
	}
	if !captchaEnabled && !loginRateLimitEnabled {
		return config, nil
	}

	host := strings.TrimSpace(os.Getenv("REDIS_HOST"))
	if host == "" {
		host = defaultRedisHost
	}
	port := strings.TrimSpace(os.Getenv("REDIS_PORT"))
	if port == "" {
		port = defaultRedisPort
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("REDIS_PORT 必须是 1 到 65535 之间的端口号")
	}
	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		return Config{}, fmt.Errorf("验证码或登录限流启用时缺少 REDIS_PASSWORD")
	}

	config.RedisAddress = net.JoinHostPort(host, port)
	config.RedisPassword = password
	return config, nil
}

// optionalBoolean 读取只允许小写 true 或 false 的可选布尔环境变量。
func optionalBoolean(name string, defaultValue bool) (bool, error) {
	rawValue, exists := os.LookupEnv(name)
	if !exists {
		return defaultValue, nil
	}
	value := strings.TrimSpace(rawValue)
	parsed, err := strconv.ParseBool(value)
	if err != nil || (value != "true" && value != "false") {
		return false, fmt.Errorf("%s 只接受 true 或 false", name)
	}
	return parsed, nil
}
