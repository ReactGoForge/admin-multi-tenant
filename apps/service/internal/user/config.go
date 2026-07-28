package user

import (
	"fmt"
	"os"
	"strings"
)

// WechatConfig 描述小程序登录调用微信服务所需的后台配置。
type WechatConfig struct {
	AppSecret             string
	MiniappCodeEnvVersion string
}

// LoadWechatConfig 从环境变量读取小程序密钥和固定的小程序码版本。
func LoadWechatConfig() (WechatConfig, error) {
	envVersion := strings.TrimSpace(os.Getenv("WECHAT_MINIAPP_CODE_ENV_VERSION"))
	switch envVersion {
	case "develop", "trial", "release":
	default:
		return WechatConfig{}, fmt.Errorf("WECHAT_MINIAPP_CODE_ENV_VERSION 必须是 develop、trial 或 release")
	}
	return WechatConfig{
		AppSecret:             strings.TrimSpace(os.Getenv("WECHAT_MINIAPP_APP_SECRET")),
		MiniappCodeEnvVersion: envVersion,
	}, nil
}

// SecretConfigured 判断微信小程序密钥是否已经通过服务器环境注入。
func (config WechatConfig) SecretConfigured() bool {
	return config.AppSecret != ""
}
