package user

import "testing"

// TestLoadWechatConfigAcceptsMiniappCodeEnvironment 验证小程序码版本只接受微信支持的三个环境值。
func TestLoadWechatConfigAcceptsMiniappCodeEnvironment(t *testing.T) {
	for _, envVersion := range []string{"develop", "trial", "release"} {
		t.Run(envVersion, func(t *testing.T) {
			t.Setenv("WECHAT_MINIAPP_APP_SECRET", "secret")
			t.Setenv("WECHAT_MINIAPP_CODE_ENV_VERSION", envVersion)
			config, err := LoadWechatConfig()
			if err != nil {
				t.Fatalf("LoadWechatConfig() 返回错误: %v", err)
			}
			if config.AppSecret != "secret" || config.MiniappCodeEnvVersion != envVersion {
				t.Fatalf("微信配置不正确: %#v", config)
			}
		})
	}
}

// TestLoadWechatConfigRejectsMissingOrInvalidMiniappCodeEnvironment 验证缺失或非法版本时拒绝启动配置。
func TestLoadWechatConfigRejectsMissingOrInvalidMiniappCodeEnvironment(t *testing.T) {
	for _, envVersion := range []string{"", "production", "test"} {
		t.Run(envVersion, func(t *testing.T) {
			t.Setenv("WECHAT_MINIAPP_CODE_ENV_VERSION", envVersion)
			if _, err := LoadWechatConfig(); err == nil {
				t.Fatalf("版本 %q 应被拒绝", envVersion)
			}
		})
	}
}
