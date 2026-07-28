package httpapi

import "github.com/gin-gonic/gin"

// PlatformRoutes 汇总平台工作空间的各业务路由。
type PlatformRoutes struct {
	Logs            PlatformLogRoutes
	Dictionaries    PlatformDictionaryRoutes
	Employees       PlatformEmployeeRoutes
	Roles           PlatformRoleRoutes
	Menus           PlatformMenuRoutes
	Departments     PlatformDepartmentRoutes
	Tenants         PlatformTenantRoutes
	MiniappSettings PlatformMiniappSettingsRoutes
	BasicSettings   PlatformBasicSettingsRoutes
	Images          PlatformImageRoutes
	Users           PlatformUserRoutes
}

// PlatformLogRoutes 描述平台系统日志和操作日志接口。
type PlatformLogRoutes struct {
	SystemView          gin.HandlerFunc
	SystemList          gin.HandlerFunc
	SystemFilterOptions gin.HandlerFunc
	AuditView           gin.HandlerFunc
	AuditList           gin.HandlerFunc
	AuditFilterOptions  gin.HandlerFunc
	LoginView           gin.HandlerFunc
	LoginList           gin.HandlerFunc
	LoginFilterOptions  gin.HandlerFunc
}

// PlatformDictionaryRoutes 描述平台字典管理接口。
type PlatformDictionaryRoutes struct {
	View       gin.HandlerFunc
	List       gin.HandlerFunc
	Create     gin.HandlerFunc
	Update     gin.HandlerFunc
	Delete     gin.HandlerFunc
	ItemCreate gin.HandlerFunc
	ItemUpdate gin.HandlerFunc
	ItemDelete gin.HandlerFunc
}

// PlatformEmployeeRoutes 描述平台员工查询、维护和对应权限中间件。
type PlatformEmployeeRoutes struct {
	View               gin.HandlerFunc
	List               gin.HandlerFunc
	Options            gin.HandlerFunc
	CreatePermission   gin.HandlerFunc
	Create             gin.HandlerFunc
	EditPermission     gin.HandlerFunc
	Edit               gin.HandlerFunc
	AssignPermission   gin.HandlerFunc
	Assign             gin.HandlerFunc
	PasswordPermission gin.HandlerFunc
	Password           gin.HandlerFunc
	StatusPermission   gin.HandlerFunc
	Status             gin.HandlerFunc
	DeletePermission   gin.HandlerFunc
	Delete             gin.HandlerFunc
}

// PlatformRoleRoutes 描述平台角色查询、维护和对应权限中间件。
type PlatformRoleRoutes struct {
	View              gin.HandlerFunc
	EmployeesView     gin.HandlerFunc
	List              gin.HandlerFunc
	Detail            gin.HandlerFunc
	PermissionOptions gin.HandlerFunc
	Employees         gin.HandlerFunc
	CreatePermission  gin.HandlerFunc
	Create            gin.HandlerFunc
	EditPermission    gin.HandlerFunc
	Edit              gin.HandlerFunc
	PermissionCheck   gin.HandlerFunc
	Permission        gin.HandlerFunc
	StatusPermission  gin.HandlerFunc
	Status            gin.HandlerFunc
	DeletePermission  gin.HandlerFunc
	Delete            gin.HandlerFunc
}

// PlatformMenuRoutes 描述平台统一维护的平台与租户菜单接口。
type PlatformMenuRoutes struct {
	View             gin.HandlerFunc
	List             gin.HandlerFunc
	CreatePermission gin.HandlerFunc
	Create           gin.HandlerFunc
	EditPermission   gin.HandlerFunc
	Edit             gin.HandlerFunc
	StatusPermission gin.HandlerFunc
	Status           gin.HandlerFunc
	DeletePermission gin.HandlerFunc
	Delete           gin.HandlerFunc
}

// PlatformDepartmentRoutes 描述平台部门查询、维护和对应权限中间件。
type PlatformDepartmentRoutes struct {
	View             gin.HandlerFunc
	List             gin.HandlerFunc
	CreatePermission gin.HandlerFunc
	Create           gin.HandlerFunc
	EditPermission   gin.HandlerFunc
	Edit             gin.HandlerFunc
	DeletePermission gin.HandlerFunc
	Delete           gin.HandlerFunc
}

// PlatformTenantRoutes 描述平台租户生命周期管理接口。
type PlatformTenantRoutes struct {
	View               gin.HandlerFunc
	List               gin.HandlerFunc
	CreatePermission   gin.HandlerFunc
	Create             gin.HandlerFunc
	EditPermission     gin.HandlerFunc
	Edit               gin.HandlerFunc
	PasswordPermission gin.HandlerFunc
	Password           gin.HandlerFunc
	StatusPermission   gin.HandlerFunc
	Status             gin.HandlerFunc
	CodePermission     gin.HandlerFunc
	Code               gin.HandlerFunc
	EnterPermission    gin.HandlerFunc
	Enter              gin.HandlerFunc
	DeletePermission   gin.HandlerFunc
	Delete             gin.HandlerFunc
}

// PlatformMiniappSettingsRoutes 描述平台微信小程序配置接口。
type PlatformMiniappSettingsRoutes struct {
	View           gin.HandlerFunc
	Get            gin.HandlerFunc
	EditPermission gin.HandlerFunc
	Update         gin.HandlerFunc
}

// PlatformBasicSettingsRoutes 描述平台基础设置接口。
type PlatformBasicSettingsRoutes struct {
	View           gin.HandlerFunc
	Get            gin.HandlerFunc
	EditPermission gin.HandlerFunc
	Update         gin.HandlerFunc
}

// PlatformImageRoutes 描述平台图片和图片分类管理接口。
type PlatformImageRoutes struct {
	View             gin.HandlerFunc
	UploadPermission gin.HandlerFunc
	EditPermission   gin.HandlerFunc
	DeletePermission gin.HandlerFunc
	List             gin.HandlerFunc
	Upload           gin.HandlerFunc
	Update           gin.HandlerFunc
	Delete           gin.HandlerFunc
	TenantOptions    gin.HandlerFunc
	Categories       gin.HandlerFunc
	CategoryCreate   gin.HandlerFunc
	CategoryUpdate   gin.HandlerFunc
	CategoryDelete   gin.HandlerFunc
}

// PlatformUserRoutes 描述平台小程序用户查询和状态管理接口。
type PlatformUserRoutes struct {
	View             gin.HandlerFunc
	List             gin.HandlerFunc
	Tenants          gin.HandlerFunc
	TenantOptions    gin.HandlerFunc
	StatusPermission gin.HandlerFunc
	Status           gin.HandlerFunc
}

// registerPlatformRoutes 按日志、RBAC、租户、设置、图片和用户顺序挂载平台接口。
func registerPlatformRoutes(platform *gin.RouterGroup, routes PlatformRoutes) {
	registerPlatformLogRoutes(platform, routes.Logs)
	registerPlatformDictionaryRoutes(platform, routes.Dictionaries)
	registerPlatformEmployeeRoutes(platform, routes.Employees)
	registerPlatformRoleRoutes(platform, routes.Roles)
	registerPlatformMenuRoutes(platform, routes.Menus)
	registerPlatformDepartmentRoutes(platform, routes.Departments)
	registerPlatformTenantRoutes(platform, routes.Tenants)
	registerPlatformSettingsRoutes(platform, routes.MiniappSettings, routes.BasicSettings)
	registerPlatformImageRoutes(platform, routes.Images)
	registerPlatformUserRoutes(platform, routes.Users)
}

// registerPlatformLogRoutes 挂载平台系统日志和操作日志接口。
func registerPlatformLogRoutes(platform *gin.RouterGroup, routes PlatformLogRoutes) {
	logs := platform.Group("/logs")
	logs.GET("/system", routes.SystemView, routes.SystemList)
	logs.GET("/system/filter-options", routes.SystemView, routes.SystemFilterOptions)
	logs.GET("/operations", routes.AuditView, routes.AuditList)
	logs.GET("/operations/filter-options", routes.AuditView, routes.AuditFilterOptions)
	logs.GET("/login", routes.LoginView, routes.LoginList)
	logs.GET("/login/filter-options", routes.LoginView, routes.LoginFilterOptions)
}

// registerPlatformDictionaryRoutes 挂载使用统一字典管理权限的平台接口。
func registerPlatformDictionaryRoutes(platform *gin.RouterGroup, routes PlatformDictionaryRoutes) {
	dictionaries := platform.Group("/dictionaries")
	dictionaries.Use(routes.View)
	dictionaries.GET("", routes.List)
	dictionaries.POST("", routes.Create)
	dictionaries.PATCH("/:dictionaryId", routes.Update)
	dictionaries.DELETE("/:dictionaryId", routes.Delete)
	dictionaries.POST("/:dictionaryId/items", routes.ItemCreate)
	dictionaries.PATCH("/:dictionaryId/items/:itemId", routes.ItemUpdate)
	dictionaries.DELETE("/:dictionaryId/items/:itemId", routes.ItemDelete)
}

// registerPlatformEmployeeRoutes 挂载平台员工接口，并为写操作追加各自的细粒度权限校验。
func registerPlatformEmployeeRoutes(platform *gin.RouterGroup, routes PlatformEmployeeRoutes) {
	employees := platform.Group("/employees")
	employees.Use(routes.View)
	employees.GET("/options", routes.Options)
	employees.GET("", routes.List)
	employees.POST("", routes.CreatePermission, routes.Create)
	employees.PATCH("/:employeeId", routes.EditPermission, routes.Edit)
	employees.PUT("/:employeeId/roles", routes.AssignPermission, routes.Assign)
	employees.PUT("/:employeeId/password", routes.PasswordPermission, routes.Password)
	employees.PATCH("/:employeeId/status", routes.StatusPermission, routes.Status)
	employees.DELETE("/:employeeId", routes.DeletePermission, routes.Delete)
}

// registerPlatformRoleRoutes 挂载平台角色接口，并保持角色员工的独立查看权限。
func registerPlatformRoleRoutes(platform *gin.RouterGroup, routes PlatformRoleRoutes) {
	roles := platform.Group("/roles")
	roles.Use(routes.View)
	roles.GET("", routes.List)
	roles.GET("/permission-options", routes.PermissionOptions)
	roles.GET("/:roleId", routes.Detail)
	roles.GET("/:roleId/employees", routes.EmployeesView, routes.Employees)
	roles.POST("", routes.CreatePermission, routes.Create)
	roles.PATCH("/:roleId", routes.EditPermission, routes.Edit)
	roles.PUT("/:roleId/permissions", routes.PermissionCheck, routes.Permission)
	roles.PATCH("/:roleId/status", routes.StatusPermission, routes.Status)
	roles.DELETE("/:roleId", routes.DeletePermission, routes.Delete)
}

// registerPlatformMenuRoutes 挂载平台菜单接口，并为写操作追加各自的细粒度权限校验。
func registerPlatformMenuRoutes(platform *gin.RouterGroup, routes PlatformMenuRoutes) {
	menus := platform.Group("/menus")
	menus.Use(routes.View)
	menus.GET("", routes.List)
	menus.POST("", routes.CreatePermission, routes.Create)
	menus.PATCH("/:menuId", routes.EditPermission, routes.Edit)
	menus.PATCH("/:menuId/status", routes.StatusPermission, routes.Status)
	menus.DELETE("/:menuId", routes.DeletePermission, routes.Delete)
}

// registerPlatformDepartmentRoutes 挂载平台部门接口。
func registerPlatformDepartmentRoutes(platform *gin.RouterGroup, routes PlatformDepartmentRoutes) {
	departments := platform.Group("/departments")
	departments.Use(routes.View)
	departments.GET("", routes.List)
	departments.POST("", routes.CreatePermission, routes.Create)
	departments.PATCH("/:departmentId", routes.EditPermission, routes.Edit)
	departments.DELETE("/:departmentId", routes.DeletePermission, routes.Delete)
}

// registerPlatformTenantRoutes 挂载平台租户维护、进入租户和小程序码接口。
func registerPlatformTenantRoutes(platform *gin.RouterGroup, routes PlatformTenantRoutes) {
	tenants := platform.Group("/tenants")
	tenants.Use(routes.View)
	tenants.GET("", routes.List)
	tenants.POST("", routes.CreatePermission, routes.Create)
	tenants.PATCH("/:tenantId", routes.EditPermission, routes.Edit)
	tenants.PUT("/:tenantId/owner-password", routes.PasswordPermission, routes.Password)
	tenants.PATCH("/:tenantId/status", routes.StatusPermission, routes.Status)
	tenants.GET("/:tenantId/miniapp-code", routes.CodePermission, routes.Code)
	tenants.POST("/:tenantId/miniapp-code", routes.CodePermission, routes.Code)
	tenants.POST("/:tenantId/enter", routes.EnterPermission, routes.Enter)
	tenants.DELETE("/:tenantId", routes.DeletePermission, routes.Delete)
}

// registerPlatformSettingsRoutes 挂载平台微信小程序配置和基础设置接口。
func registerPlatformSettingsRoutes(platform *gin.RouterGroup, miniapp PlatformMiniappSettingsRoutes, basic PlatformBasicSettingsRoutes) {
	miniappSettings := platform.Group("/settings/miniapp")
	miniappSettings.Use(miniapp.View)
	miniappSettings.GET("", miniapp.Get)
	miniappSettings.PUT("", miniapp.EditPermission, miniapp.Update)

	basicSettings := platform.Group("/settings/basic")
	basicSettings.Use(basic.View)
	basicSettings.GET("", basic.Get)
	basicSettings.PUT("", basic.EditPermission, basic.Update)
}

// registerPlatformImageRoutes 挂载平台图片、租户筛选项和图片分类接口。
func registerPlatformImageRoutes(platform *gin.RouterGroup, routes PlatformImageRoutes) {
	images := platform.Group("/images")
	images.Use(routes.View)
	images.GET("/tenant-options", routes.TenantOptions)
	images.GET("", routes.List)
	images.POST("", routes.UploadPermission, routes.Upload)
	images.PATCH("/:imageId", routes.EditPermission, routes.Update)
	images.DELETE("/:imageId", routes.DeletePermission, routes.Delete)

	categories := platform.Group("/image-categories")
	categories.Use(routes.View)
	categories.GET("", routes.Categories)
	categories.POST("", routes.EditPermission, routes.CategoryCreate)
	categories.PATCH("/:categoryId", routes.EditPermission, routes.CategoryUpdate)
	categories.DELETE("/:categoryId", routes.EditPermission, routes.CategoryDelete)
}

// registerPlatformUserRoutes 挂载平台小程序用户查询、关联租户、租户选项和状态接口。
func registerPlatformUserRoutes(platform *gin.RouterGroup, routes PlatformUserRoutes) {
	users := platform.Group("/users")
	users.Use(routes.View)
	users.GET("/tenant-options", routes.TenantOptions)
	users.GET("", routes.List)
	users.GET("/:userId/tenants", routes.Tenants)
	users.PATCH("/:userId/status", routes.StatusPermission, routes.Status)
}
