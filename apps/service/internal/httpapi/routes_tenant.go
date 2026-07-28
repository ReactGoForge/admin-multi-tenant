package httpapi

import "github.com/gin-gonic/gin"

// TenantRoutes 汇总当前可信租户工作空间的各业务路由。
type TenantRoutes struct {
	Logs          TenantLogRoutes
	Employees     TenantEmployeeRoutes
	Roles         TenantRoleRoutes
	Menus         TenantMenuRoutes
	Departments   TenantDepartmentRoutes
	Users         TenantUserRoutes
	BasicSettings TenantBasicSettingsRoutes
	Images        TenantImageRoutes
}

// TenantLogRoutes 描述租户操作日志接口。
type TenantLogRoutes struct {
	AuditView          gin.HandlerFunc
	AuditList          gin.HandlerFunc
	AuditFilterOptions gin.HandlerFunc
	LoginView          gin.HandlerFunc
	LoginList          gin.HandlerFunc
	LoginFilterOptions gin.HandlerFunc
}

// TenantEmployeeRoutes 描述租户员工查询、维护和对应权限中间件。
type TenantEmployeeRoutes struct {
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

// TenantRoleRoutes 描述租户角色查询、维护和对应权限中间件。
type TenantRoleRoutes struct {
	View             gin.HandlerFunc
	EmployeesView    gin.HandlerFunc
	List             gin.HandlerFunc
	Detail           gin.HandlerFunc
	Employees        gin.HandlerFunc
	CreatePermission gin.HandlerFunc
	Create           gin.HandlerFunc
	EditPermission   gin.HandlerFunc
	Edit             gin.HandlerFunc
	PermissionCheck  gin.HandlerFunc
	Permission       gin.HandlerFunc
	StatusPermission gin.HandlerFunc
	Status           gin.HandlerFunc
	DeletePermission gin.HandlerFunc
	Delete           gin.HandlerFunc
}

// TenantMenuRoutes 描述租户工作空间只读菜单接口。
type TenantMenuRoutes struct {
	View gin.HandlerFunc
	List gin.HandlerFunc
}

// TenantDepartmentRoutes 描述租户部门查询、维护和对应权限中间件。
type TenantDepartmentRoutes struct {
	View             gin.HandlerFunc
	List             gin.HandlerFunc
	CreatePermission gin.HandlerFunc
	Create           gin.HandlerFunc
	EditPermission   gin.HandlerFunc
	Edit             gin.HandlerFunc
	DeletePermission gin.HandlerFunc
	Delete           gin.HandlerFunc
}

// TenantUserRoutes 描述当前租户小程序用户查询和状态接口。
type TenantUserRoutes struct {
	View             gin.HandlerFunc
	List             gin.HandlerFunc
	StatusPermission gin.HandlerFunc
	Status           gin.HandlerFunc
}

// TenantBasicSettingsRoutes 描述当前租户基础设置接口。
type TenantBasicSettingsRoutes struct {
	View           gin.HandlerFunc
	Get            gin.HandlerFunc
	EditPermission gin.HandlerFunc
	Update         gin.HandlerFunc
}

// TenantImageRoutes 描述当前租户图片和图片分类管理接口。
type TenantImageRoutes struct {
	View             gin.HandlerFunc
	UploadPermission gin.HandlerFunc
	EditPermission   gin.HandlerFunc
	DeletePermission gin.HandlerFunc
	List             gin.HandlerFunc
	Upload           gin.HandlerFunc
	Update           gin.HandlerFunc
	Delete           gin.HandlerFunc
	Categories       gin.HandlerFunc
	CategoryCreate   gin.HandlerFunc
	CategoryUpdate   gin.HandlerFunc
	CategoryDelete   gin.HandlerFunc
}

// registerTenantRoutes 按日志、RBAC、用户、设置和图片顺序挂载租户接口。
func registerTenantRoutes(tenant *gin.RouterGroup, routes TenantRoutes) {
	registerTenantLogRoutes(tenant, routes.Logs)
	registerTenantEmployeeRoutes(tenant, routes.Employees)
	registerTenantRoleRoutes(tenant, routes.Roles)
	registerTenantMenuRoutes(tenant, routes.Menus)
	registerTenantDepartmentRoutes(tenant, routes.Departments)
	registerTenantUserRoutes(tenant, routes.Users)
	registerTenantBasicSettingsRoutes(tenant, routes.BasicSettings)
	registerTenantImageRoutes(tenant, routes.Images)
}

// registerTenantLogRoutes 挂载当前租户的操作日志接口。
func registerTenantLogRoutes(tenant *gin.RouterGroup, routes TenantLogRoutes) {
	tenant.GET("/logs/operations", routes.AuditView, routes.AuditList)
	tenant.GET("/logs/operations/filter-options", routes.AuditView, routes.AuditFilterOptions)
	tenant.GET("/logs/login", routes.LoginView, routes.LoginList)
	tenant.GET("/logs/login/filter-options", routes.LoginView, routes.LoginFilterOptions)
}

// registerTenantEmployeeRoutes 挂载当前租户员工接口，并保持租户范围由认证上下文决定。
func registerTenantEmployeeRoutes(tenant *gin.RouterGroup, routes TenantEmployeeRoutes) {
	employees := tenant.Group("/employees")
	employees.Use(routes.View)
	employees.GET("", routes.List)
	employees.GET("/options", routes.Options)
	employees.POST("", routes.CreatePermission, routes.Create)
	employees.PATCH("/:employeeId", routes.EditPermission, routes.Edit)
	employees.PUT("/:employeeId/roles", routes.AssignPermission, routes.Assign)
	employees.PUT("/:employeeId/password", routes.PasswordPermission, routes.Password)
	employees.PATCH("/:employeeId/status", routes.StatusPermission, routes.Status)
	employees.DELETE("/:employeeId", routes.DeletePermission, routes.Delete)
}

// registerTenantRoleRoutes 挂载当前租户角色接口。
func registerTenantRoleRoutes(tenant *gin.RouterGroup, routes TenantRoleRoutes) {
	roles := tenant.Group("/roles")
	roles.Use(routes.View)
	roles.GET("", routes.List)
	roles.GET("/:roleId", routes.Detail)
	roles.GET("/:roleId/employees", routes.EmployeesView, routes.Employees)
	roles.POST("", routes.CreatePermission, routes.Create)
	roles.PATCH("/:roleId", routes.EditPermission, routes.Edit)
	roles.PUT("/:roleId/permissions", routes.PermissionCheck, routes.Permission)
	roles.PATCH("/:roleId/status", routes.StatusPermission, routes.Status)
	roles.DELETE("/:roleId", routes.DeletePermission, routes.Delete)
}

// registerTenantMenuRoutes 挂载当前租户可使用的只读菜单接口。
func registerTenantMenuRoutes(tenant *gin.RouterGroup, routes TenantMenuRoutes) {
	menus := tenant.Group("/menus")
	menus.Use(routes.View)
	menus.GET("", routes.List)
}

// registerTenantDepartmentRoutes 挂载当前租户部门接口。
func registerTenantDepartmentRoutes(tenant *gin.RouterGroup, routes TenantDepartmentRoutes) {
	departments := tenant.Group("/departments")
	departments.Use(routes.View)
	departments.GET("", routes.List)
	departments.POST("", routes.CreatePermission, routes.Create)
	departments.PATCH("/:departmentId", routes.EditPermission, routes.Edit)
	departments.DELETE("/:departmentId", routes.DeletePermission, routes.Delete)
}

// registerTenantUserRoutes 挂载当前租户小程序用户查询和状态接口。
func registerTenantUserRoutes(tenant *gin.RouterGroup, routes TenantUserRoutes) {
	users := tenant.Group("/users")
	users.Use(routes.View)
	users.GET("", routes.List)
	users.PATCH("/:userId/status", routes.StatusPermission, routes.Status)
}

// registerTenantBasicSettingsRoutes 挂载当前租户基础设置接口。
func registerTenantBasicSettingsRoutes(tenant *gin.RouterGroup, routes TenantBasicSettingsRoutes) {
	basic := tenant.Group("/settings/basic")
	basic.Use(routes.View)
	basic.GET("", routes.Get)
	basic.PUT("", routes.EditPermission, routes.Update)
}

// registerTenantImageRoutes 挂载当前租户图片和图片分类接口。
func registerTenantImageRoutes(tenant *gin.RouterGroup, routes TenantImageRoutes) {
	images := tenant.Group("/images")
	images.Use(routes.View)
	images.GET("", routes.List)
	images.POST("", routes.UploadPermission, routes.Upload)
	images.PATCH("/:imageId", routes.EditPermission, routes.Update)
	images.DELETE("/:imageId", routes.DeletePermission, routes.Delete)

	categories := tenant.Group("/image-categories")
	categories.Use(routes.View)
	categories.GET("", routes.Categories)
	categories.POST("", routes.EditPermission, routes.CategoryCreate)
	categories.PATCH("/:categoryId", routes.EditPermission, routes.CategoryUpdate)
	categories.DELETE("/:categoryId", routes.EditPermission, routes.CategoryDelete)
}
