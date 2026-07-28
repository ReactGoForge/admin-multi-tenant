package user

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 10
	maximumPageSize = 100
)

// AdminHandler 组织平台和租户后台用户列表及启停接口。
type AdminHandler struct {
	users UserAdminApplication
}

// NewAdminHandler 创建后台用户管理处理器。
func NewAdminHandler(users UserAdminApplication) *AdminHandler {
	return &AdminHandler{users: users}
}

// ListPlatformUsers 返回平台唯一用户分页列表。
func (handler *AdminHandler) ListPlatformUsers(context *gin.Context) {
	query, valid := parsePlatformUserQuery(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	result, err := handler.users.ListPlatformUsers(context.Request.Context(), query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newPlatformUserListResponse(result))
}

// ListPlatformUserTenants 返回指定平台用户关联的全部租户。
func (handler *AdminHandler) ListPlatformUserTenants(context *gin.Context) {
	userID, valid := parsePathID(context, "userId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	tenants, err := handler.users.ListPlatformUserTenants(context.Request.Context(), userID)
	if err != nil {
		handler.writeAdminError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newPlatformUserTenantResponses(tenants))
}

// ListTenantOptions 返回平台用户筛选所需的全部租户最小信息。
func (handler *AdminHandler) ListTenantOptions(context *gin.Context) {
	options, err := handler.users.ListTenantOptions(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newTenantOptionResponses(options))
}

// SetPlatformUserStatus 更新平台用户的全局状态。
func (handler *AdminHandler) SetPlatformUserStatus(context *gin.Context) {
	userID, valid := parsePathID(context, "userId")
	status, validStatus := parseStatusRequest(context)
	if !valid || !validStatus {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.users.SetPlatformUserStatus(context.Request.Context(), userID, status); err != nil {
		handler.writeAdminError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// ListTenantUsers 返回当前认证租户的用户归属分页列表。
func (handler *AdminHandler) ListTenantUsers(context *gin.Context) {
	tenantID, valid := currentTenantID(context)
	if !valid {
		return
	}
	query, valid := parseTenantUserQuery(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	result, err := handler.users.ListTenantUsers(context.Request.Context(), tenantID, query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newTenantUserListResponse(result))
}

// SetTenantUserStatus 更新用户在当前认证租户内的状态。
func (handler *AdminHandler) SetTenantUserStatus(context *gin.Context) {
	tenantID, valid := currentTenantID(context)
	if !valid {
		return
	}
	userID, validID := parsePathID(context, "userId")
	status, validStatus := parseStatusRequest(context)
	if !validID || !validStatus {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.users.SetTenantUserStatus(context.Request.Context(), tenantID, userID, status); err != nil {
		handler.writeAdminError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// currentTenantID 从后台认证上下文读取租户 ID，拒绝客户端指定数据范围。
func currentTenantID(context *gin.Context) (uint64, bool) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return 0, false
	}
	return tenantID, true
}

// parsePlatformUserQuery 严格解析平台用户分页和筛选参数。
func parsePlatformUserQuery(context *gin.Context) (PlatformUserQuery, bool) {
	page, pageSize, valid := parsePagination(context)
	if !valid {
		return PlatformUserQuery{}, false
	}
	query := PlatformUserQuery{Page: page, PageSize: pageSize, Nickname: strings.TrimSpace(context.Query("nickname")), Phone: strings.TrimSpace(context.Query("phone"))}
	if !validSearchText(query.Nickname, 64) || !validSearchText(query.Phone, 20) {
		return PlatformUserQuery{}, false
	}
	tenantID, valid := parseOptionalID(context, "tenantId")
	if !valid {
		return PlatformUserQuery{}, false
	}
	query.TenantID = tenantID
	status, valid := parseOptionalStatus(context)
	if !valid {
		return PlatformUserQuery{}, false
	}
	query.Status = status
	return query, true
}

// parseTenantUserQuery 严格解析租户用户分页和筛选参数。
func parseTenantUserQuery(context *gin.Context) (TenantUserQuery, bool) {
	page, pageSize, valid := parsePagination(context)
	if !valid {
		return TenantUserQuery{}, false
	}
	query := TenantUserQuery{Page: page, PageSize: pageSize, Nickname: strings.TrimSpace(context.Query("nickname")), Phone: strings.TrimSpace(context.Query("phone"))}
	if !validSearchText(query.Nickname, 64) || !validSearchText(query.Phone, 20) {
		return TenantUserQuery{}, false
	}
	status, valid := parseOptionalStatus(context)
	if !valid {
		return TenantUserQuery{}, false
	}
	query.Status = status
	return query, true
}

// parsePagination 读取统一分页参数并防止偏移量溢出。
func parsePagination(context *gin.Context) (int, int, bool) {
	page, valid := parsePositiveInt(context.Query("page"), 1)
	if !valid {
		return 0, 0, false
	}
	pageSize, valid := parsePositiveInt(context.Query("pageSize"), defaultPageSize)
	if !valid || pageSize > maximumPageSize {
		return 0, 0, false
	}
	maximumInt := int(^uint(0) >> 1)
	if page-1 > maximumInt/pageSize {
		return 0, 0, false
	}
	return page, pageSize, true
}

// parsePositiveInt 读取可缺省的正整数。
func parsePositiveInt(raw string, defaultValue int) (int, bool) {
	if raw == "" {
		return defaultValue, true
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	return value, err == nil && value > 0
}

// parseOptionalID 读取可选的正整数 BIGINT 查询参数。
func parseOptionalID(context *gin.Context, name string) (*uint64, bool) {
	raw, exists := context.GetQuery(name)
	if !exists {
		return nil, true
	}
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || value == 0 {
		return nil, false
	}
	return &value, true
}

// parsePathID 读取路径中的正整数 BIGINT 标识。
func parsePathID(context *gin.Context, name string) (uint64, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(context.Param(name)), 10, 64)
	return value, err == nil && value > 0
}

// parseOptionalStatus 读取可选的启停筛选值。
func parseOptionalStatus(context *gin.Context) (*uint8, bool) {
	raw, exists := context.GetQuery("status")
	if !exists {
		return nil, true
	}
	value, valid := parseStatus(strings.TrimSpace(raw))
	return &value, valid
}

// parseStatusRequest 读取并校验状态写请求。
func parseStatusRequest(context *gin.Context) (uint8, bool) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maximumMiniappRequestBytes)
	var request statusRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		return 0, false
	}
	return parseStatus(strings.TrimSpace(request.Status))
}

// parseStatus 将接口状态枚举转换为数据库值。
func parseStatus(value string) (uint8, bool) {
	switch value {
	case "enabled":
		return 1, true
	case "disabled":
		return 0, true
	default:
		return 0, false
	}
}

// writeAdminError 将后台用户写入错误转换为稳定响应。
func (handler *AdminHandler) writeAdminError(context *gin.Context, err error) {
	if errors.Is(err, errUserNotFound) {
		httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
		return
	}
	httpapi.WriteError(context, httpapi.ErrorCodeInternal)
}
