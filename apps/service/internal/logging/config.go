package logging

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSystemLogRetentionDays = 30
	maximumSystemLogRetentionDays = 3650
)

// RequestLogMode 描述系统请求日志写入数据库的采集范围。
type RequestLogMode string

const (
	// RequestLogModeMutationAndError 仅保存写请求和失败查询。
	RequestLogModeMutationAndError RequestLogMode = "mutation_and_error"
	// RequestLogModeAll 保存除固定排除项以外的全部 API 请求。
	RequestLogModeAll RequestLogMode = "all"
	// RequestLogModeOff 关闭系统请求日志数据库写入。
	RequestLogModeOff RequestLogMode = "off"
)

// Config 描述系统请求日志、运行事件和保留时间的运行配置。
type Config struct {
	RequestMode            RequestLogMode
	EventDBEnabled         bool
	Retention              time.Duration
	DevelopmentHTTPEnabled bool
}

// LoadConfig 从环境变量读取日志配置，并拒绝无法识别或超出范围的值。
func LoadConfig() (Config, error) {
	config := Config{
		RequestMode:            RequestLogModeMutationAndError,
		EventDBEnabled:         true,
		Retention:              defaultSystemLogRetentionDays * 24 * time.Hour,
		DevelopmentHTTPEnabled: strings.TrimSpace(os.Getenv("APP_ENV")) == "development",
	}

	if rawMode := strings.TrimSpace(os.Getenv("SYSTEM_REQUEST_LOG_MODE")); rawMode != "" {
		mode := RequestLogMode(rawMode)
		if mode != RequestLogModeMutationAndError && mode != RequestLogModeAll && mode != RequestLogModeOff {
			return Config{}, fmt.Errorf("SYSTEM_REQUEST_LOG_MODE 只接受 mutation_and_error、all 或 off")
		}
		config.RequestMode = mode
	}

	if rawEnabled := strings.TrimSpace(os.Getenv("SYSTEM_EVENT_DB_ENABLED")); rawEnabled != "" {
		enabled, err := strconv.ParseBool(rawEnabled)
		if err != nil || (rawEnabled != "true" && rawEnabled != "false") {
			return Config{}, fmt.Errorf("SYSTEM_EVENT_DB_ENABLED 只接受 true 或 false")
		}
		config.EventDBEnabled = enabled
	}

	if rawDays := strings.TrimSpace(os.Getenv("SYSTEM_LOG_RETENTION_DAYS")); rawDays != "" {
		days, err := strconv.Atoi(rawDays)
		if err != nil || days < 1 || days > maximumSystemLogRetentionDays {
			return Config{}, fmt.Errorf("SYSTEM_LOG_RETENTION_DAYS 必须是 1 到 %d 之间的整数", maximumSystemLogRetentionDays)
		}
		config.Retention = time.Duration(days) * 24 * time.Hour
	}

	return config, nil
}
