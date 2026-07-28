package database

import (
	"strings"
	"testing"
)

// setRequiredEnvironment 为数据库配置测试设置一组完整的基础环境变量。
func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("MYSQL_HOST", "")
	t.Setenv("MYSQL_PORT", "")
	t.Setenv("MYSQL_USER", "admin_multi_tenant")
	t.Setenv("MYSQL_PASSWORD", "local_password")
	t.Setenv("MYSQL_DATABASE", "admin_multi_tenant")
}

// TestLoadConfigUsesLocalDefaults 验证未指定地址时会使用本地 MySQL 默认地址和必要参数。
func TestLoadConfigUsesLocalDefaults(t *testing.T) {
	setRequiredEnvironment(t)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() 返回错误: %v", err)
	}

	if config.Host != defaultHost {
		t.Fatalf("Host = %q，期望 %q", config.Host, defaultHost)
	}
	if config.Port != defaultPort {
		t.Fatalf("Port = %q，期望 %q", config.Port, defaultPort)
	}
	dsn := config.dsn()
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Fatal("DSN 未启用 utf8mb4")
	}
	if !strings.Contains(dsn, "parseTime=true") {
		t.Fatal("DSN 未启用 parseTime")
	}
}

// TestLoadConfigAcceptsCustomAddress 验证数据库地址可以通过环境变量覆盖。
func TestLoadConfigAcceptsCustomAddress(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MYSQL_HOST", "mysql.internal")
	t.Setenv("MYSQL_PORT", "3307")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() 返回错误: %v", err)
	}

	if config.Host != "mysql.internal" || config.Port != "3307" {
		t.Fatalf("地址 = %s:%s，期望 mysql.internal:3307", config.Host, config.Port)
	}
}

// TestLoadConfigRejectsMissingValues 验证缺少必需的数据库配置时会返回错误。
func TestLoadConfigRejectsMissingValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "缺少用户", key: "MYSQL_USER"},
		{name: "缺少密码", key: "MYSQL_PASSWORD"},
		{name: "缺少数据库", key: "MYSQL_DATABASE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv(test.key, "")

			if _, err := LoadConfig(); err == nil {
				t.Fatalf("清空 %s 后 LoadConfig() 未返回错误", test.key)
			}
		})
	}
}

// TestLoadConfigRejectsRootUser 验证应用数据库连接禁止使用 root 账号。
func TestLoadConfigRejectsRootUser(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MYSQL_USER", "ROOT")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("root 用户未被拒绝")
	}
}

// TestLoadConfigRejectsInvalidPort 验证超出合法范围的 MySQL 端口会被拒绝。
func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MYSQL_PORT", "70000")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("非法端口未被拒绝")
	}
}
