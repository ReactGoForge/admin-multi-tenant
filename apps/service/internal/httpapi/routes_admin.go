package httpapi

import "github.com/gin-gonic/gin"

// AdminRoutes 描述后台登录认证和所有后台工作空间共用的接口。
type AdminRoutes struct {
	Captcha           gin.HandlerFunc
	Login             gin.HandlerFunc
	Authenticate      gin.HandlerFunc
	CurrentUser       gin.HandlerFunc
	ProfileBasic      gin.HandlerFunc
	ProfilePassword   gin.HandlerFunc
	ProfileAvatar     gin.HandlerFunc
	DictionaryOptions gin.HandlerFunc
}

// registerAdminAuthRoutes 挂载不需要后台登录态的验证码和登录接口。
func registerAdminAuthRoutes(admin *gin.RouterGroup, routes AdminRoutes) {
	adminAuth := admin.Group("/auth")
	adminAuth.GET("/captcha", routes.Captcha)
	adminAuth.POST("/login", routes.Login)
}

// registerAdminCommonRoutes 挂载认证后由平台和租户工作空间共用的接口。
func registerAdminCommonRoutes(admin *gin.RouterGroup, routes AdminRoutes) {
	admin.GET("/me", routes.CurrentUser)
	admin.PUT("/profile/basic", routes.ProfileBasic)
	admin.PUT("/profile/password", routes.ProfilePassword)
	admin.POST("/profile/avatar", routes.ProfileAvatar)
	admin.GET("/dictionary-options", routes.DictionaryOptions)
}
