package httpapi

import "github.com/gin-gonic/gin"

// MiniappRoutes 描述小程序登录、认证和当前用户接口。
type MiniappRoutes struct {
	Login        gin.HandlerFunc
	Authenticate gin.HandlerFunc
	Current      gin.HandlerFunc
	Avatar       gin.HandlerFunc
}

// registerMiniappRoutes 挂载小程序公开登录和受保护的当前用户接口。
func registerMiniappRoutes(router *gin.Engine, routes MiniappRoutes) {
	miniapp := router.Group("/api/miniapp")
	miniapp.POST("/auth/login", routes.Login)

	protectedMiniapp := miniapp.Group("")
	protectedMiniapp.Use(routes.Authenticate)
	protectedMiniapp.GET("/me", routes.Current)
	protectedMiniapp.POST("/profile/avatar", routes.Avatar)
}
