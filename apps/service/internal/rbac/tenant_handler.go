package rbac

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// TenantHandler 组织平台租户生命周期接口的请求解析与响应转换。
type TenantHandler struct {
	tenants TenantApplication
}

// NewTenantHandler 使用租户业务服务创建租户接口处理器。
func NewTenantHandler(tenants TenantApplication) *TenantHandler {
	return &TenantHandler{tenants: tenants}
}

// tenantCreateRequest 描述租户及其所有者初始化接口接收的字段。
type tenantCreateRequest struct {
	Name         string `json:"name"`
	OwnerName    string `json:"ownerName"`
	LoginAccount string `json:"loginAccount"`
	Password     string `json:"password"`
}

// tenantUpdateRequest 描述平台编辑租户及所有者账号时接收的字段。
type tenantUpdateRequest struct {
	Name         string  `json:"name"`
	LoginAccount string  `json:"loginAccount"`
	Remark       *string `json:"remark"`
}

// tenantResponse 描述平台租户列表返回给前端的稳定字段。
type tenantResponse struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Remark          *string `json:"remark"`
	IconURL         *string `json:"iconUrl"`
	Status          string  `json:"status"`
	OwnerEmployeeID *string `json:"ownerEmployeeId"`
	OwnerName       *string `json:"ownerName"`
	LoginAccount    *string `json:"loginAccount"`
}

// tenantListResponse 描述租户列表统一分页响应。
type tenantListResponse struct {
	Items    []tenantResponse `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int64            `json:"total"`
}

// ListTenants 返回平台租户分页列表。
func (handler *TenantHandler) ListTenants(context *gin.Context) {
	query, ok := parseTenantQuery(context)
	if !ok {
		return
	}
	tenants, total, err := handler.tenants.ListTenants(context.Request.Context(), query)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, tenantListResponse{Items: newTenantResponses(tenants), Page: query.Page, PageSize: query.PageSize, Total: total})
}

// CreateTenant 校验请求并创建租户、企业管理员角色和所有者员工。
func (handler *TenantHandler) CreateTenant(context *gin.Context) {
	input, ok := bindTenantCreateInput(context)
	if !ok {
		return
	}
	if err := handler.tenants.CreateTenant(context.Request.Context(), input); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, nil)
}

// UpdateTenant 更新平台租户信息和当前所有者登录账号。
func (handler *TenantHandler) UpdateTenant(context *gin.Context) {
	tenantID, valid := parsePathUint(context, "tenantId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	input, ok := bindTenantUpdateInput(context)
	if !ok {
		return
	}
	if err := handler.tenants.UpdateTenant(context.Request.Context(), tenantID, input); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// ResetTenantOwnerPassword 重置租户当前所有者员工的登录密码。
func (handler *TenantHandler) ResetTenantOwnerPassword(context *gin.Context) {
	tenantID, valid := parsePathUint(context, "tenantId")
	var request passwordRequest
	if !valid || context.ShouldBindJSON(&request) != nil || !validPassword(request.Password) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.tenants.ResetTenantOwnerPassword(context.Request.Context(), tenantID, request.Password); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// SetTenantStatus 启用或禁用租户。
func (handler *TenantHandler) SetTenantStatus(context *gin.Context) {
	tenantID, valid := parsePathUint(context, "tenantId")
	var request statusRequest
	if !valid || context.ShouldBindJSON(&request) != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	status, ok := parseStatus(request.Status)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.tenants.SetTenantStatus(context.Request.Context(), tenantID, status); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// DeleteTenant 删除已停用且未产生业务数据的空租户。
func (handler *TenantHandler) DeleteTenant(context *gin.Context) {
	tenantID, valid := parsePathUint(context, "tenantId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.tenants.DeleteTenant(context.Request.Context(), tenantID); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// parseTenantQuery 解析租户列表分页、名称和状态筛选。
func parseTenantQuery(context *gin.Context) (TenantQuery, bool) {
	page, pageSize, valid := parsePagination(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return TenantQuery{}, false
	}
	query := TenantQuery{Page: page, PageSize: pageSize, Name: strings.TrimSpace(context.Query("name"))}
	if utf8.RuneCountInString(query.Name) > 100 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return TenantQuery{}, false
	}
	if raw, exists := context.GetQuery("status"); exists {
		status, ok := parseStatus(raw)
		if !ok {
			httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
			return TenantQuery{}, false
		}
		query.Status = &status
	}
	return query, true
}

// bindTenantCreateInput 绑定并转换租户创建请求。
func bindTenantCreateInput(context *gin.Context) (TenantCreateInput, bool) {
	var request tenantCreateRequest
	if context.ShouldBindJSON(&request) != nil || !normalizeTenantCreateRequest(&request) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return TenantCreateInput{}, false
	}
	return TenantCreateInput{Name: request.Name, OwnerName: request.OwnerName, LoginAccount: request.LoginAccount, Password: request.Password}, true
}

// bindTenantUpdateInput 绑定并转换租户编辑请求。
func bindTenantUpdateInput(context *gin.Context) (TenantUpdateInput, bool) {
	var request tenantUpdateRequest
	if context.ShouldBindJSON(&request) != nil || !normalizeTenantUpdateRequest(&request) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return TenantUpdateInput{}, false
	}
	return TenantUpdateInput{Name: request.Name, LoginAccount: request.LoginAccount, Remark: request.Remark}, true
}

// normalizeTenantCreateRequest 清理并校验平台租户创建字段。
func normalizeTenantCreateRequest(request *tenantCreateRequest) bool {
	request.Name = strings.TrimSpace(request.Name)
	request.OwnerName = strings.TrimSpace(request.OwnerName)
	request.LoginAccount = strings.TrimSpace(request.LoginAccount)
	return request.Name != "" && utf8.RuneCountInString(request.Name) <= 100 &&
		request.OwnerName != "" && utf8.RuneCountInString(request.OwnerName) <= 30 &&
		request.LoginAccount != "" && utf8.RuneCountInString(request.LoginAccount) <= 40 &&
		validPassword(request.Password)
}

// normalizeTenantUpdateRequest 清理并校验平台租户编辑字段。
func normalizeTenantUpdateRequest(request *tenantUpdateRequest) bool {
	request.Name = strings.TrimSpace(request.Name)
	request.LoginAccount = strings.TrimSpace(request.LoginAccount)
	request.Remark = normalizeOptionalText(request.Remark)
	return request.Name != "" && utf8.RuneCountInString(request.Name) <= 100 && request.LoginAccount != "" && utf8.RuneCountInString(request.LoginAccount) <= 40 && (request.Remark == nil || utf8.RuneCountInString(*request.Remark) <= 500)
}

// newTenantResponses 转换平台租户列表响应。
func newTenantResponses(tenants []PlatformTenant) []tenantResponse {
	responses := make([]tenantResponse, 0, len(tenants))
	for _, tenant := range tenants {
		var ownerID *string
		if tenant.OwnerEmployeeID != nil {
			value := strconv.FormatUint(*tenant.OwnerEmployeeID, 10)
			ownerID = &value
		}
		responses = append(responses, tenantResponse{
			ID: strconv.FormatUint(tenant.ID, 10), Name: tenant.Name, Remark: tenant.Remark,
			IconURL: tenant.IconURL, Status: statusName(tenant.Status), OwnerEmployeeID: ownerID,
			OwnerName: tenant.OwnerName, LoginAccount: tenant.LoginAccount,
		})
	}
	return responses
}
