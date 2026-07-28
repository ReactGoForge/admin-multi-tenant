package auth

import (
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// RequirePermission 校验当前员工的工作空间与数据库实时权限，平台超级管理员直接放行。
func (handler *Handler) RequirePermission(workspace, permissionCode string) gin.HandlerFunc {
	// Go 学习提示：这个方法返回另一个函数，返回值正好满足 gin.HandlerFunc。
	// 因而每条路由可以在注册时把工作空间和权限编码“固定”到自己的中间件中。
	return func(context *gin.Context) {
		employeeValue, exists := context.Get(employeeContextKey)
		employee, valid := employeeValue.(Employee)
		if !exists || !valid {
			handler.abortUnauthorized(context)
			return
		}
		identity, identityValid := CurrentTokenIdentity(context)
		if !identityValid {
			handler.abortUnauthorized(context)
			return
		}
		allowed, platformSuperAdmin, err := handler.authorizer.RequirePermission(context.Request.Context(), employee, identity, workspace, permissionCode)
		if err != nil {
			httpapi.WriteError(context, httpapi.ErrorCodeInternal)
			return
		}
		if platformSuperAdmin {
			context.Set(platformSuperAdminContextKey, true)
		}
		if allowed {
			context.Next()
			return
		}

		abortForbidden(context)
	}
}

// CurrentPlatformSuperAdmin 判断当前权限中间件是否已实时确认平台超级管理员身份。
func CurrentPlatformSuperAdmin(context *gin.Context) bool {
	value, exists := context.Get(platformSuperAdminContextKey)
	isSuperAdmin, valid := value.(bool)
	return exists && valid && isSuperAdmin
}

// RequirePlatformSuperAdmin 实时校验当前员工是否为启用的平台超级管理员。
func (handler *Handler) RequirePlatformSuperAdmin(context *gin.Context) {
	employeeValue, exists := context.Get(employeeContextKey)
	employee, valid := employeeValue.(Employee)
	if !exists || !valid {
		handler.abortUnauthorized(context)
		return
	}
	if employee.Scope != "platform" {
		abortForbidden(context)
		return
	}
	identity, valid := CurrentTokenIdentity(context)
	if !valid || identity.Mode != "normal" {
		abortForbidden(context)
		return
	}
	allowed, err := handler.authorizer.RequirePlatformSuperAdmin(context.Request.Context(), employee, identity)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	if allowed {
		context.Set(platformSuperAdminContextKey, true)
		context.Next()
		return
	}
	abortForbidden(context)
}

// abortForbidden 终止请求并返回统一的无权限响应。
func abortForbidden(context *gin.Context) {
	httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
}
