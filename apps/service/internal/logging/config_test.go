package logging

import (
	"testing"
	"time"
)

// TestLoadConfigUsesDefaults 验证日志环境变量缺省时保持当前采集策略。
func TestLoadConfigUsesDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("SYSTEM_REQUEST_LOG_MODE", "")
	t.Setenv("SYSTEM_EVENT_DB_ENABLED", "")
	t.Setenv("SYSTEM_LOG_RETENTION_DAYS", "")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("读取默认日志配置失败: %v", err)
	}
	if config.RequestMode != RequestLogModeMutationAndError || !config.EventDBEnabled || config.Retention != 30*24*time.Hour {
		t.Fatalf("默认日志配置错误: %+v", config)
	}
	if config.DevelopmentHTTPEnabled {
		t.Fatal("APP_ENV 缺失时不应启用开发 HTTP 文件日志")
	}
}

// TestLoadConfigAcceptsOverrides 验证日志采集模式、事件入库和保留天数可以覆盖。
func TestLoadConfigAcceptsOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("SYSTEM_REQUEST_LOG_MODE", "all")
	t.Setenv("SYSTEM_EVENT_DB_ENABLED", "false")
	t.Setenv("SYSTEM_LOG_RETENTION_DAYS", "90")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("读取自定义日志配置失败: %v", err)
	}
	if config.RequestMode != RequestLogModeAll || config.EventDBEnabled || config.Retention != 90*24*time.Hour {
		t.Fatalf("自定义日志配置错误: %+v", config)
	}
	if !config.DevelopmentHTTPEnabled {
		t.Fatal("APP_ENV=development 时应启用开发 HTTP 文件日志")
	}
}

// TestLoadConfigDisablesDevelopmentHTTPOutsideDevelopment 验证非开发环境不会启用文件正文日志。
func TestLoadConfigDisablesDevelopmentHTTPOutsideDevelopment(t *testing.T) {
	for _, environment := range []string{"production", "test", "local", "Development"} {
		t.Run(environment, func(t *testing.T) {
			t.Setenv("APP_ENV", environment)
			config, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() 返回错误: %v", err)
			}
			if config.DevelopmentHTTPEnabled {
				t.Fatalf("APP_ENV=%q 时不应启用开发 HTTP 文件日志", environment)
			}
		})
	}
}

// TestLoadConfigRejectsInvalidValues 验证非法日志配置会阻止服务继续启动。
func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "请求模式", key: "SYSTEM_REQUEST_LOG_MODE", value: "errors"},
		{name: "事件开关", key: "SYSTEM_EVENT_DB_ENABLED", value: "yes"},
		{name: "保留天数下限", key: "SYSTEM_LOG_RETENTION_DAYS", value: "0"},
		{name: "保留天数上限", key: "SYSTEM_LOG_RETENTION_DAYS", value: "3651"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("SYSTEM_REQUEST_LOG_MODE", "")
			t.Setenv("SYSTEM_EVENT_DB_ENABLED", "")
			t.Setenv("SYSTEM_LOG_RETENTION_DAYS", "")
			t.Setenv(test.key, test.value)
			if _, err := LoadConfig(); err == nil {
				t.Fatalf("非法配置 %s=%s 未被拒绝", test.key, test.value)
			}
		})
	}
}
