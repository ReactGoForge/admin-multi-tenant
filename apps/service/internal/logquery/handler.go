package logquery

import (
	"net/http"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
)

// Handler 提供平台系统日志、平台操作日志和租户操作日志的只读查询接口。
type Handler struct {
	service *Service
	now     nowFunc
}

// NewHandler 使用日志查询服务创建日志查询处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, now: defaultNow}
}

// ListPlatformLoginLogs 返回具有权限的平台员工可见的全平台后台登录日志。
func (handler *Handler) ListPlatformLoginLogs(context *gin.Context) {
	query, valid := handler.parseQuery(context, true)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	response, err := handler.service.ListPlatformLoginLogs(context.Request.Context(), query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListTenantLoginLogs 返回认证上下文中当前租户的后台登录日志。
func (handler *Handler) ListTenantLoginLogs(context *gin.Context) {
	tenantID, valid := currentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	query, valid := handler.parseQuery(context, false)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	response, err := handler.service.ListTenantLoginLogs(context.Request.Context(), tenantID, query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListPlatformLoginFilterOptions 返回平台登录日志可筛选的租户选项。
func (handler *Handler) ListPlatformLoginFilterOptions(context *gin.Context) {
	response, err := handler.service.ListPlatformLoginFilterOptions(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListTenantLoginFilterOptions 返回租户登录日志的空全局选项集合。
func (handler *Handler) ListTenantLoginFilterOptions(context *gin.Context) {
	httpapi.WriteSuccess(context, http.StatusOK, handler.service.ListTenantLoginFilterOptions())
}

// ListPlatformSystemLogs 返回仅平台超级管理员可见的系统日志。
func (handler *Handler) ListPlatformSystemLogs(context *gin.Context) {
	query, valid := handler.parseQuery(context, true)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	response, err := handler.service.ListPlatformSystemLogs(context.Request.Context(), query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListPlatformSystemFilterOptions 返回平台系统日志可使用的租户和历史操作者选项。
func (handler *Handler) ListPlatformSystemFilterOptions(context *gin.Context) {
	response, err := handler.service.ListPlatformSystemFilterOptions(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListPlatformAuditFilterOptions 返回平台操作日志可使用的租户和历史操作者选项。
func (handler *Handler) ListPlatformAuditFilterOptions(context *gin.Context) {
	response, err := handler.service.ListPlatformAuditFilterOptions(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListTenantAuditFilterOptions 返回当前租户操作日志可使用的历史操作者选项。
func (handler *Handler) ListTenantAuditFilterOptions(context *gin.Context) {
	tenantID, valid := currentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	response, err := handler.service.ListTenantAuditFilterOptions(context.Request.Context(), tenantID)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListPlatformAuditLogs 返回平台有权员工可见的全平台操作审计日志。
func (handler *Handler) ListPlatformAuditLogs(context *gin.Context) {
	query, valid := handler.parseQuery(context, true)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	response, err := handler.service.ListPlatformAuditLogs(context.Request.Context(), query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// ListTenantAuditLogs 返回当前认证租户范围内的操作审计日志。
func (handler *Handler) ListTenantAuditLogs(context *gin.Context) {
	tenantID, valid := currentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	query, valid := handler.parseQuery(context, false)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	response, err := handler.service.ListTenantAuditLogs(context.Request.Context(), tenantID, query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// parseQuery 解析并限制日志分页、时间范围和筛选参数。
func (handler *Handler) parseQuery(context *gin.Context, allowTenant bool) (listQuery, bool) {
	return parseQuery(context, handler.now(), allowTenant)
}

// currentTenantID 从认证日志上下文中读取可信租户 ID。
func currentTenantID(context *gin.Context) (uint64, bool) {
	actor, valid := logging.CurrentActor(context)
	if !valid || actor.TenantID == nil {
		return 0, false
	}
	return *actor.TenantID, true
}
