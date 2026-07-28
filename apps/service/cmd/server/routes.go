package main

import (
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/dictionary"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logquery"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/media"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/rbac"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/user"
)

// httpRouteHandlers 汇总已经完成依赖初始化的业务处理器。
//
// 该结构只服务于进程入口的路由组装，不承担业务逻辑，也不创建新的应用分层。
type httpRouteHandlers struct {
	// Go 学习提示：这些字段保存的是已经创建好的对象指针；路由组装只引用其方法，
	// 不负责数据库连接、配置读取或对象生命周期。
	requestRecorder logging.RequestRecorder
	auditRecorder   logging.AuditRecorder
	developmentHTTP *logging.DevelopmentHTTPLogger
	logConfig       logging.Config
	readiness       httpapi.ReadinessCheck
	auth            *auth.Handler
	dictionary      *dictionary.Handler
	employee        *rbac.Handler
	role            *rbac.RoleHandler
	menu            *rbac.MenuHandler
	department      *rbac.DepartmentHandler
	tenant          *rbac.TenantHandler
	miniapp         *user.Handler
	userAdmin       *user.AdminHandler
	miniappAdmin    *user.MiniappAdminHandler
	media           *media.Handler
	logQuery        *logquery.Handler
}

// buildHTTPRoutes 将业务处理器和权限中间件映射为按入口分组的 HTTP 路由配置。
func buildHTTPRoutes(handlers httpRouteHandlers) httpapi.Routes {
	// Go 学习提示：Go 的方法可以像普通函数一样作为值传递。
	// 例如 handlers.auth.Login 的类型满足 gin.HandlerFunc，因此可以交给路由配置稍后注册。
	return httpapi.Routes{
		RequestRecorder:       handlers.requestRecorder,
		AuditRecorder:         handlers.auditRecorder,
		DevelopmentHTTPLogger: handlers.developmentHTTP,
		RequestLogMode:        handlers.logConfig.RequestMode,
		Readiness:             handlers.readiness,
		Public: httpapi.PublicRoutes{
			PlatformBrand: handlers.media.PublicPlatformBrand,
			Image:         handlers.media.PublicImage,
		},
		Miniapp: httpapi.MiniappRoutes{
			Login:        handlers.miniapp.Login,
			Authenticate: handlers.miniapp.Authenticate,
			Current:      handlers.miniapp.Current,
			Avatar:       handlers.media.UploadCurrentMiniappUserAvatar,
		},
		Admin: httpapi.AdminRoutes{
			Captcha:           handlers.auth.Captcha,
			Login:             handlers.auth.Login,
			Authenticate:      handlers.auth.Authenticate,
			CurrentUser:       handlers.auth.CurrentUser,
			ProfileBasic:      handlers.auth.UpdateBasicProfile,
			ProfilePassword:   handlers.auth.ChangePassword,
			ProfileAvatar:     handlers.media.UploadCurrentEmployeeAvatar,
			DictionaryOptions: handlers.dictionary.ListOptions,
		},
		Platform: httpapi.PlatformRoutes{
			Logs: httpapi.PlatformLogRoutes{
				SystemView:          handlers.auth.RequirePermission("platform", "platform:system-log:view"),
				SystemList:          handlers.logQuery.ListPlatformSystemLogs,
				SystemFilterOptions: handlers.logQuery.ListPlatformSystemFilterOptions,
				AuditView:           handlers.auth.RequirePermission("platform", "platform:audit-log:view"),
				AuditList:           handlers.logQuery.ListPlatformAuditLogs,
				AuditFilterOptions:  handlers.logQuery.ListPlatformAuditFilterOptions,
				LoginView:           handlers.auth.RequirePermission("platform", "platform:login-log:view"),
				LoginList:           handlers.logQuery.ListPlatformLoginLogs,
				LoginFilterOptions:  handlers.logQuery.ListPlatformLoginFilterOptions,
			},
			Dictionaries: httpapi.PlatformDictionaryRoutes{
				View:       handlers.auth.RequirePermission("platform", "platform:field:view"),
				List:       handlers.dictionary.ListTypes,
				Create:     handlers.dictionary.CreateType,
				Update:     handlers.dictionary.UpdateType,
				Delete:     handlers.dictionary.DeleteType,
				ItemCreate: handlers.dictionary.CreateItem,
				ItemUpdate: handlers.dictionary.UpdateItem,
				ItemDelete: handlers.dictionary.DeleteItem,
			},
			Employees: httpapi.PlatformEmployeeRoutes{
				View:               handlers.auth.RequirePermission("platform", "platform:employee:view"),
				List:               handlers.employee.ListPlatformEmployees,
				Options:            handlers.employee.PlatformEmployeeOptions,
				CreatePermission:   handlers.auth.RequirePermission("platform", "platform:employee:create"),
				Create:             handlers.employee.CreatePlatformEmployee,
				EditPermission:     handlers.auth.RequirePermission("platform", "platform:employee:edit"),
				Edit:               handlers.employee.UpdatePlatformEmployee,
				AssignPermission:   handlers.auth.RequirePermission("platform", "platform:employee:assign-role"),
				Assign:             handlers.employee.AssignPlatformEmployeeRoles,
				PasswordPermission: handlers.auth.RequirePermission("platform", "platform:employee:reset-password"),
				Password:           handlers.employee.ResetPlatformEmployeePassword,
				StatusPermission:   handlers.auth.RequirePermission("platform", "platform:employee:status"),
				Status:             handlers.employee.SetPlatformEmployeeStatus,
				DeletePermission:   handlers.auth.RequirePermission("platform", "platform:employee:delete"),
				Delete:             handlers.employee.DeletePlatformEmployee,
			},
			Roles: httpapi.PlatformRoleRoutes{
				View:              handlers.auth.RequirePermission("platform", "platform:role:view"),
				EmployeesView:     handlers.auth.RequirePermission("platform", "platform:role:employees"),
				List:              handlers.role.ListPlatformRoles,
				Detail:            handlers.role.PlatformRoleDetail,
				PermissionOptions: handlers.role.PlatformRolePermissionOptions,
				Employees:         handlers.role.ListPlatformRoleEmployees,
				CreatePermission:  handlers.auth.RequirePermission("platform", "platform:role:create"),
				Create:            handlers.role.CreatePlatformRole,
				EditPermission:    handlers.auth.RequirePermission("platform", "platform:role:edit"),
				Edit:              handlers.role.UpdatePlatformRole,
				PermissionCheck:   handlers.auth.RequirePermission("platform", "platform:role:permission"),
				Permission:        handlers.role.AssignPlatformRolePermissions,
				StatusPermission:  handlers.auth.RequirePermission("platform", "platform:role:status"),
				Status:            handlers.role.SetPlatformRoleStatus,
				DeletePermission:  handlers.auth.RequirePermission("platform", "platform:role:delete"),
				Delete:            handlers.role.DeletePlatformRole,
			},
			Menus: httpapi.PlatformMenuRoutes{
				View:             handlers.auth.RequirePermission("platform", "platform:menu:view"),
				List:             handlers.menu.ListPlatformMenus,
				CreatePermission: handlers.auth.RequirePermission("platform", "platform:menu:create"),
				Create:           handlers.menu.CreatePlatformMenu,
				EditPermission:   handlers.auth.RequirePermission("platform", "platform:menu:edit"),
				Edit:             handlers.menu.UpdatePlatformMenu,
				StatusPermission: handlers.auth.RequirePermission("platform", "platform:menu:status"),
				Status:           handlers.menu.SetPlatformMenuStatus,
				DeletePermission: handlers.auth.RequirePermission("platform", "platform:menu:delete"),
				Delete:           handlers.menu.DeletePlatformMenu,
			},
			Departments: httpapi.PlatformDepartmentRoutes{
				View:             handlers.auth.RequirePermission("platform", "platform:department:view"),
				List:             handlers.department.ListPlatformDepartments,
				CreatePermission: handlers.auth.RequirePermission("platform", "platform:department:create"),
				Create:           handlers.department.CreatePlatformDepartment,
				EditPermission:   handlers.auth.RequirePermission("platform", "platform:department:edit"),
				Edit:             handlers.department.UpdatePlatformDepartment,
				DeletePermission: handlers.auth.RequirePermission("platform", "platform:department:delete"),
				Delete:           handlers.department.DeletePlatformDepartment,
			},
			Tenants: httpapi.PlatformTenantRoutes{
				View:               handlers.auth.RequirePermission("platform", "platform:tenant:view"),
				List:               handlers.tenant.ListTenants,
				CreatePermission:   handlers.auth.RequirePermission("platform", "platform:tenant:create"),
				Create:             handlers.tenant.CreateTenant,
				EditPermission:     handlers.auth.RequirePermission("platform", "platform:tenant:edit"),
				Edit:               handlers.tenant.UpdateTenant,
				PasswordPermission: handlers.auth.RequirePermission("platform", "platform:tenant:reset-password"),
				Password:           handlers.tenant.ResetTenantOwnerPassword,
				StatusPermission:   handlers.auth.RequirePermission("platform", "platform:tenant:status"),
				Status:             handlers.tenant.SetTenantStatus,
				CodePermission:     handlers.auth.RequirePermission("platform", "platform:tenant:miniapp-code"),
				Code:               handlers.miniappAdmin.TenantMiniappCode,
				EnterPermission:    handlers.auth.RequirePermission("platform", "platform:tenant:enter"),
				Enter:              handlers.auth.EnterTenant,
				DeletePermission:   handlers.auth.RequirePermission("platform", "platform:tenant:delete"),
				Delete:             handlers.tenant.DeleteTenant,
			},
			MiniappSettings: httpapi.PlatformMiniappSettingsRoutes{
				View:           handlers.auth.RequirePermission("platform", "platform:miniapp:view"),
				Get:            handlers.miniappAdmin.GetSettings,
				EditPermission: handlers.auth.RequirePermission("platform", "platform:miniapp:edit"),
				Update:         handlers.miniappAdmin.UpdateSettings,
			},
			BasicSettings: httpapi.PlatformBasicSettingsRoutes{
				View:           handlers.auth.RequirePermission("platform", "platform:basic:view"),
				Get:            handlers.media.GetPlatformBasicSettings,
				EditPermission: handlers.auth.RequirePermission("platform", "platform:basic:edit"),
				Update:         handlers.media.UpdatePlatformBasicSettings,
			},
			Images: httpapi.PlatformImageRoutes{
				View:             handlers.auth.RequirePermission("platform", "platform:image:view"),
				UploadPermission: handlers.auth.RequirePermission("platform", "platform:image:upload"),
				EditPermission:   handlers.auth.RequirePermission("platform", "platform:image:edit"),
				DeletePermission: handlers.auth.RequirePermission("platform", "platform:image:delete"),
				List:             handlers.media.ListPlatformImages,
				Upload:           handlers.media.UploadPlatformImage,
				Update:           handlers.media.UpdatePlatformImage,
				Delete:           handlers.media.DeletePlatformImage,
				TenantOptions:    handlers.media.PlatformTenantOptions,
				Categories:       handlers.media.ListPlatformCategories,
				CategoryCreate:   handlers.media.CreatePlatformCategory,
				CategoryUpdate:   handlers.media.UpdatePlatformCategory,
				CategoryDelete:   handlers.media.DeletePlatformCategory,
			},
			Users: httpapi.PlatformUserRoutes{
				View:             handlers.auth.RequirePermission("platform", "platform:user:view"),
				List:             handlers.userAdmin.ListPlatformUsers,
				Tenants:          handlers.userAdmin.ListPlatformUserTenants,
				TenantOptions:    handlers.userAdmin.ListTenantOptions,
				StatusPermission: handlers.auth.RequirePermission("platform", "platform:user:status"),
				Status:           handlers.userAdmin.SetPlatformUserStatus,
			},
		},
		Tenant: httpapi.TenantRoutes{
			Logs: httpapi.TenantLogRoutes{
				AuditView:          handlers.auth.RequirePermission("tenant", "tenant:audit-log:view"),
				AuditList:          handlers.logQuery.ListTenantAuditLogs,
				AuditFilterOptions: handlers.logQuery.ListTenantAuditFilterOptions,
				LoginView:          handlers.auth.RequirePermission("tenant", "tenant:login-log:view"),
				LoginList:          handlers.logQuery.ListTenantLoginLogs,
				LoginFilterOptions: handlers.logQuery.ListTenantLoginFilterOptions,
			},
			Employees: httpapi.TenantEmployeeRoutes{
				View:               handlers.auth.RequirePermission("tenant", "tenant:employee:view"),
				List:               handlers.employee.ListTenantEmployees,
				Options:            handlers.employee.TenantEmployeeOptions,
				CreatePermission:   handlers.auth.RequirePermission("tenant", "tenant:employee:create"),
				Create:             handlers.employee.CreateTenantEmployee,
				EditPermission:     handlers.auth.RequirePermission("tenant", "tenant:employee:edit"),
				Edit:               handlers.employee.UpdateTenantEmployee,
				AssignPermission:   handlers.auth.RequirePermission("tenant", "tenant:employee:assign-role"),
				Assign:             handlers.employee.AssignTenantEmployeeRoles,
				PasswordPermission: handlers.auth.RequirePermission("tenant", "tenant:employee:reset-password"),
				Password:           handlers.employee.ResetTenantEmployeePassword,
				StatusPermission:   handlers.auth.RequirePermission("tenant", "tenant:employee:status"),
				Status:             handlers.employee.SetTenantEmployeeStatus,
				DeletePermission:   handlers.auth.RequirePermission("tenant", "tenant:employee:delete"),
				Delete:             handlers.employee.DeleteTenantEmployee,
			},
			Roles: httpapi.TenantRoleRoutes{
				View:             handlers.auth.RequirePermission("tenant", "tenant:role:view"),
				EmployeesView:    handlers.auth.RequirePermission("tenant", "tenant:role:employees"),
				List:             handlers.role.ListTenantRoles,
				Detail:           handlers.role.TenantRoleDetail,
				Employees:        handlers.role.ListTenantRoleEmployees,
				CreatePermission: handlers.auth.RequirePermission("tenant", "tenant:role:create"),
				Create:           handlers.role.CreateTenantRole,
				EditPermission:   handlers.auth.RequirePermission("tenant", "tenant:role:edit"),
				Edit:             handlers.role.UpdateTenantRole,
				PermissionCheck:  handlers.auth.RequirePermission("tenant", "tenant:role:permission"),
				Permission:       handlers.role.AssignTenantRolePermissions,
				StatusPermission: handlers.auth.RequirePermission("tenant", "tenant:role:status"),
				Status:           handlers.role.SetTenantRoleStatus,
				DeletePermission: handlers.auth.RequirePermission("tenant", "tenant:role:delete"),
				Delete:           handlers.role.DeleteTenantRole,
			},
			Menus: httpapi.TenantMenuRoutes{
				View: handlers.auth.RequirePermission("tenant", "tenant:menu:view"),
				List: handlers.menu.ListTenantMenus,
			},
			Departments: httpapi.TenantDepartmentRoutes{
				View:             handlers.auth.RequirePermission("tenant", "tenant:department:view"),
				List:             handlers.department.ListTenantDepartments,
				CreatePermission: handlers.auth.RequirePermission("tenant", "tenant:department:create"),
				Create:           handlers.department.CreateTenantDepartment,
				EditPermission:   handlers.auth.RequirePermission("tenant", "tenant:department:edit"),
				Edit:             handlers.department.UpdateTenantDepartment,
				DeletePermission: handlers.auth.RequirePermission("tenant", "tenant:department:delete"),
				Delete:           handlers.department.DeleteTenantDepartment,
			},
			Users: httpapi.TenantUserRoutes{
				View:             handlers.auth.RequirePermission("tenant", "tenant:user:view"),
				List:             handlers.userAdmin.ListTenantUsers,
				StatusPermission: handlers.auth.RequirePermission("tenant", "tenant:user:status"),
				Status:           handlers.userAdmin.SetTenantUserStatus,
			},
			BasicSettings: httpapi.TenantBasicSettingsRoutes{
				View:           handlers.auth.RequirePermission("tenant", "tenant:basic:view"),
				Get:            handlers.media.GetTenantBasicSettings,
				EditPermission: handlers.auth.RequirePermission("tenant", "tenant:basic:edit"),
				Update:         handlers.media.UpdateTenantBasicSettings,
			},
			Images: httpapi.TenantImageRoutes{
				View:             handlers.auth.RequirePermission("tenant", "tenant:image:view"),
				UploadPermission: handlers.auth.RequirePermission("tenant", "tenant:image:upload"),
				EditPermission:   handlers.auth.RequirePermission("tenant", "tenant:image:edit"),
				DeletePermission: handlers.auth.RequirePermission("tenant", "tenant:image:delete"),
				List:             handlers.media.ListTenantImages,
				Upload:           handlers.media.UploadTenantImage,
				Update:           handlers.media.UpdateTenantImage,
				Delete:           handlers.media.DeleteTenantImage,
				Categories:       handlers.media.ListTenantCategories,
				CategoryCreate:   handlers.media.CreateTenantCategory,
				CategoryUpdate:   handlers.media.UpdateTenantCategory,
				CategoryDelete:   handlers.media.DeleteTenantCategory,
			},
		},
	}
}
