package httpapi

import "github.com/gin-gonic/gin"

// PublicRoutes 描述不需要登录即可访问的公开接口。
type PublicRoutes struct {
	PlatformBrand gin.HandlerFunc
	Image         gin.HandlerFunc
}

// registerPublicRoutes 挂载平台品牌和公开品牌图片接口。
func registerPublicRoutes(router *gin.Engine, routes PublicRoutes) {
	public := router.Group("/api/public")
	public.GET("/platform-brand", routes.PlatformBrand)
	public.GET("/images/:imageId", routes.Image)
}
