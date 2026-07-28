package httpapi

import (
	"context"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ReadinessCheck 定义就绪检查需要验证必要外部依赖的最小能力。
type ReadinessCheck func(context.Context) error

// Routes 汇总 HTTP 服务需要的日志记录器和各入口路由配置。
//
// 路由按公开接口、小程序、后台公共接口、平台工作空间和租户工作空间分组，
// 避免服务新增功能时继续扩大单个平铺配置结构。
type Routes struct {
	RequestRecorder       logging.RequestRecorder
	AuditRecorder         logging.AuditRecorder
	DevelopmentHTTPLogger *logging.DevelopmentHTTPLogger
	RequestLogMode        logging.RequestLogMode
	Readiness             ReadinessCheck
	Public                PublicRoutes
	Miniapp               MiniappRoutes
	Admin                 AdminRoutes
	Platform              PlatformRoutes
	Tenant                TenantRoutes
}

// NewRouter 创建 Gin 路由，并按固定顺序挂载全局中间件与业务入口。
func NewRouter(routes Routes) *gin.Engine {
	// Go 学习提示：gin.New 只创建空 Engine；全局中间件、404/405 和业务路由都需要显式注册。
	// Use 的先后顺序就是请求经过中间件的先后顺序。
	router := gin.New()
	router.HandleMethodNotAllowed = true
	if routes.DevelopmentHTTPLogger != nil {
		router.Use(routes.DevelopmentHTTPLogger.Middleware())
	}
	if routes.RequestRecorder != nil {
		router.Use(logging.Middleware(routes.RequestRecorder, routes.RequestLogMode))
	}
	router.Use(Recovery())
	router.NoRoute(func(context *gin.Context) {
		WriteError(context, ErrorCodeEndpointNotFound)
	})
	router.NoMethod(func(context *gin.Context) {
		WriteError(context, ErrorCodeMethodNotAllowed)
	})

	router.GET("/ping", func(context *gin.Context) {
		WriteSuccess(context, http.StatusOK, gin.H{"message": "pong"})
	})
	router.GET("/healthz", func(context *gin.Context) {
		WriteSuccess(context, http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(context *gin.Context) {
		if routes.Readiness == nil || routes.Readiness(context.Request.Context()) != nil {
			WriteError(context, ErrorCodeServiceNotReady)
			return
		}
		WriteSuccess(context, http.StatusOK, gin.H{"status": "ready"})
	})
	registerPublicRoutes(router, routes.Public)

	// Go 学习提示：Group 只添加公共路径前缀并继承父级中间件，不会立即处理请求。
	// 真正的 GET、POST 等注册发生在各 register 函数中。
	admin := router.Group("/api/admin")
	registerAdminAuthRoutes(admin, routes.Admin)
	registerMiniappRoutes(router, routes.Miniapp)

	// 安全边界：后台业务路由必须先认证，再记录成功写操作的审计日志，最后进入各资源权限中间件。
	protectedAdmin := admin.Group("")
	protectedAdmin.Use(routes.Admin.Authenticate)
	if routes.AuditRecorder != nil {
		protectedAdmin.Use(logging.AuditMiddleware(routes.AuditRecorder))
	}
	registerAdminCommonRoutes(protectedAdmin, routes.Admin)
	registerPlatformRoutes(protectedAdmin.Group("/platform"), routes.Platform)
	registerTenantRoutes(protectedAdmin.Group("/tenant"), routes.Tenant)

	return router
}
