package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
)

const readinessCheckTimeout = 3 * time.Second

// newReadinessCheck 创建只检查 MySQL 和已启用认证 Redis 的就绪探针。
func newReadinessCheck(sqlDB *sql.DB, captcha *auth.CaptchaManager, limiter *auth.LoginLimiter) httpapi.ReadinessCheck {
	return func(ctx context.Context) error {
		checkContext, cancel := context.WithTimeout(ctx, readinessCheckTimeout)
		defer cancel()
		if err := sqlDB.PingContext(checkContext); err != nil {
			return fmt.Errorf("MySQL 未就绪: %w", err)
		}
		if err := captcha.Ready(checkContext); err != nil {
			return fmt.Errorf("验证码服务未就绪: %w", err)
		}
		if err := limiter.Ready(checkContext); err != nil {
			return fmt.Errorf("登录限流服务未就绪: %w", err)
		}
		return nil
	}
}
