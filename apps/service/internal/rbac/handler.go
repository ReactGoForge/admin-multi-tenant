package rbac

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 10
	maximumPageSize = 100
	timeFormat      = "2006-01-02 15:04:05"
)

// EmployeeApplication 定义 Employee Handler 依赖的业务服务能力。
type EmployeeApplication interface {
	ListEmployees(httpContext context.Context, actor EmployeeActor, scope managementScope, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error)
	ListEmployeeOptions(httpContext context.Context, actor EmployeeActor, scope managementScope) (PlatformEmployeeOptions, error)
	CreateEmployee(httpContext context.Context, actor EmployeeActor, scope managementScope, mutation EmployeeMutation) error
	UpdateEmployee(httpContext context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, mutation EmployeeMutation) error
	AssignEmployeeRoles(httpContext context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, roleIDs []uint64) error
	ResetEmployeePassword(httpContext context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, password string) error
	SetEmployeeStatus(httpContext context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, status uint8) error
	DeleteEmployee(httpContext context.Context, actor EmployeeActor, scope managementScope, employeeID uint64) error
}

// Handler 组织平台和租户 Employee 接口的参数校验与响应转换。
type Handler struct {
	service EmployeeApplication
}

// NewHandler 使用 Employee 业务服务创建处理器。
func NewHandler(service EmployeeApplication) *Handler {
	return &Handler{service: service}
}

// employeeRoleResponse 描述员工列表中的角色摘要。
type employeeRoleResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Assignable bool   `json:"assignable"`
}

// departmentResponse 描述员工所属部门摘要。
type departmentResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// employeeResponse 描述员工列表返回给前端的稳定字段。
type employeeResponse struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	LoginAccount string                 `json:"loginAccount"`
	Department   *departmentResponse    `json:"department"`
	Roles        []employeeRoleResponse `json:"roles"`
	Phone        *string                `json:"phone"`
	Status       string                 `json:"status"`
	CreatedAt    string                 `json:"createdAt"`
}

// employeeListResponse 描述员工列表统一分页响应。
type employeeListResponse struct {
	Items    []employeeResponse `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
}

// employeeOptionResponse 描述员工筛选器使用的角色或部门选项。
type employeeOptionResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Assignable bool   `json:"assignable"`
}

// employeeOptionsResponse 汇总员工页面需要的角色和部门选项。
type employeeOptionsResponse struct {
	Roles       []employeeOptionResponse `json:"roles"`
	Departments []employeeOptionResponse `json:"departments"`
}

// EmployeeMutation 描述员工新增和编辑接口经过校验后的字段。
type EmployeeMutation struct {
	Name         string
	LoginAccount string
	Password     string
	Phone        *string
	DepartmentID *uint64
	RoleIDs      []uint64
	Status       uint8
}

// employeeMutationRequest 描述员工新增和编辑接口接收的字段。
type employeeMutationRequest struct {
	Name         string   `json:"name"`
	LoginAccount string   `json:"loginAccount"`
	Password     string   `json:"password"`
	Phone        *string  `json:"phone"`
	DepartmentID *string  `json:"departmentId"`
	RoleIDs      []string `json:"roleIds"`
	Status       string   `json:"status"`
}

// employeeRolesRequest 描述员工角色替换接口接收的角色 ID。
type employeeRolesRequest struct {
	RoleIDs []string `json:"roleIds"`
}

// passwordRequest 描述密码重置接口接收的新密码。
type passwordRequest struct {
	Password string `json:"password"`
}

// statusRequest 描述启用和禁用接口接收的稳定状态值。
type statusRequest struct {
	Status string `json:"status"`
}

// ListPlatformEmployees 校验查询参数并返回平台员工的统一分页响应。
func (handler *Handler) ListPlatformEmployees(context *gin.Context) {
	handler.listEmployees(context, managementScope{Name: "platform"})
}

// ListTenantEmployees 返回当前租户员工分页列表。
func (handler *Handler) ListTenantEmployees(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.listEmployees(context, scope)
}

// listEmployees 校验查询参数并返回员工分页响应。
func (handler *Handler) listEmployees(context *gin.Context, scope managementScope) {
	query, valid := parsePlatformEmployeeQuery(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	employees, total, err := handler.service.ListEmployees(context.Request.Context(), actor, scope, query)
	if err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newEmployeeListResponse(employees, query.Page, query.PageSize, total))
}

// PlatformEmployeeOptions 返回平台员工筛选所需的角色与部门选项。
func (handler *Handler) PlatformEmployeeOptions(context *gin.Context) {
	handler.employeeOptions(context, managementScope{Name: "platform"})
}

// TenantEmployeeOptions 返回当前租户员工表单和筛选所需选项。
func (handler *Handler) TenantEmployeeOptions(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.employeeOptions(context, scope)
}

// employeeOptions 返回指定工作空间员工筛选所需的角色与部门选项。
func (handler *Handler) employeeOptions(context *gin.Context, scope managementScope) {
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	options, err := handler.service.ListEmployeeOptions(context.Request.Context(), actor, scope)
	if err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newEmployeeOptionsResponse(options))
}

// CreatePlatformEmployee 创建平台员工并关联角色。
func (handler *Handler) CreatePlatformEmployee(context *gin.Context) {
	handler.createEmployee(context, managementScope{Name: "platform"})
}

// CreateTenantEmployee 创建当前租户员工并关联租户角色。
func (handler *Handler) CreateTenantEmployee(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.createEmployee(context, scope)
}

// createEmployee 校验请求并交给 Service 创建员工。
func (handler *Handler) createEmployee(context *gin.Context, scope managementScope) {
	mutation, valid := parseEmployeeMutation(context, true)
	if !valid || len(mutation.RoleIDs) == 0 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.CreateEmployee(context.Request.Context(), actor, scope, mutation); err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, nil)
}

// UpdatePlatformEmployee 更新平台员工基本资料。
func (handler *Handler) UpdatePlatformEmployee(context *gin.Context) {
	handler.updateEmployee(context, managementScope{Name: "platform"})
}

// UpdateTenantEmployee 更新当前租户员工基本资料。
func (handler *Handler) UpdateTenantEmployee(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.updateEmployee(context, scope)
}

// updateEmployee 校验请求并交给 Service 更新员工基础资料。
func (handler *Handler) updateEmployee(context *gin.Context, scope managementScope) {
	employeeID, validID := parsePathUint(context, "employeeId")
	mutation, validMutation := parseEmployeeMutation(context, false)
	if !validID || !validMutation {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.UpdateEmployee(context.Request.Context(), actor, scope, employeeID, mutation); err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// AssignPlatformEmployeeRoles 替换平台员工角色关联。
func (handler *Handler) AssignPlatformEmployeeRoles(context *gin.Context) {
	handler.assignEmployeeRoles(context, managementScope{Name: "platform"})
}

// AssignTenantEmployeeRoles 替换当前租户员工角色关联。
func (handler *Handler) AssignTenantEmployeeRoles(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.assignEmployeeRoles(context, scope)
}

// assignEmployeeRoles 校验角色 ID 并交给 Service 替换关联。
func (handler *Handler) assignEmployeeRoles(context *gin.Context, scope managementScope) {
	employeeID, validID := parsePathUint(context, "employeeId")
	var request employeeRolesRequest
	if !validID || context.ShouldBindJSON(&request) != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	roleIDs, validRoles := parseIDs(request.RoleIDs)
	if !validRoles || len(roleIDs) == 0 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.AssignEmployeeRoles(context.Request.Context(), actor, scope, employeeID, roleIDs); err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// ResetPlatformEmployeePassword 重置平台员工密码。
func (handler *Handler) ResetPlatformEmployeePassword(context *gin.Context) {
	handler.resetEmployeePassword(context, managementScope{Name: "platform"})
}

// ResetTenantEmployeePassword 重置当前租户员工密码。
func (handler *Handler) ResetTenantEmployeePassword(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.resetEmployeePassword(context, scope)
}

// resetEmployeePassword 校验新密码并交给 Service 更新密码哈希。
func (handler *Handler) resetEmployeePassword(context *gin.Context, scope managementScope) {
	employeeID, validID := parsePathUint(context, "employeeId")
	var request passwordRequest
	if !validID || context.ShouldBindJSON(&request) != nil || !validPassword(request.Password) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.ResetEmployeePassword(context.Request.Context(), actor, scope, employeeID, request.Password); err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// SetPlatformEmployeeStatus 更新平台员工启用状态。
func (handler *Handler) SetPlatformEmployeeStatus(context *gin.Context) {
	handler.setEmployeeStatus(context, managementScope{Name: "platform"})
}

// SetTenantEmployeeStatus 更新当前租户员工启用状态。
func (handler *Handler) SetTenantEmployeeStatus(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.setEmployeeStatus(context, scope)
}

// setEmployeeStatus 校验状态并交给 Service 更新。
func (handler *Handler) setEmployeeStatus(context *gin.Context, scope managementScope) {
	employeeID, validID := parsePathUint(context, "employeeId")
	var request statusRequest
	if !validID || context.ShouldBindJSON(&request) != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	status, ok := parseStatus(request.Status)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.SetEmployeeStatus(context.Request.Context(), actor, scope, employeeID, status); err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// DeletePlatformEmployee 删除无业务引用且已停用的平台普通员工。
func (handler *Handler) DeletePlatformEmployee(context *gin.Context) {
	handler.deleteEmployee(context, managementScope{Name: "platform"})
}

// DeleteTenantEmployee 删除当前租户无业务引用且已停用的普通员工。
func (handler *Handler) DeleteTenantEmployee(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.deleteEmployee(context, scope)
}

// deleteEmployee 校验路径 ID 并交给 Service 删除员工。
func (handler *Handler) deleteEmployee(context *gin.Context, scope managementScope) {
	employeeID, valid := parsePathUint(context, "employeeId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.DeleteEmployee(context.Request.Context(), actor, scope, employeeID); err != nil {
		writeEmployeeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// tenantScope 从认证上下文提取租户 ID，拒绝客户端自行选择租户。
func tenantScope(context *gin.Context) (managementScope, bool) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return managementScope{}, false
	}
	return managementScope{Name: "tenant", TenantID: &tenantID}, true
}

// currentEmployeeActor 从 Gin 认证上下文提取 Service 需要的可信身份。
func currentEmployeeActor(context *gin.Context) (EmployeeActor, bool) {
	employee, employeeValid := auth.CurrentEmployee(context)
	identity, identityValid := auth.CurrentTokenIdentity(context)
	if !employeeValid || !identityValid {
		return EmployeeActor{}, false
	}
	return EmployeeActor{Employee: employee, Identity: identity, PlatformSuperAdmin: auth.CurrentPlatformSuperAdmin(context)}, true
}

// currentPlatformEmployeeID 返回当前普通平台会话中的可信员工 ID，用于仅向本人放行受保护记录。
func currentPlatformEmployeeID(context *gin.Context) *uint64 {
	employee, valid := auth.CurrentEmployee(context)
	if !valid || employee.Scope != "platform" {
		return nil
	}
	employeeID := employee.ID
	return &employeeID
}

// newEmployeeListResponse 将平台员工查询结果转换为统一分页响应。
func newEmployeeListResponse(employees []PlatformEmployee, page, pageSize int, total int64) employeeListResponse {
	items := make([]employeeResponse, 0, len(employees))
	for _, employee := range employees {
		roles := make([]employeeRoleResponse, 0, len(employee.Roles))
		for _, role := range employee.Roles {
			roles = append(roles, employeeRoleResponse{ID: strconv.FormatUint(role.ID, 10), Name: role.Name, Assignable: role.Assignable})
		}
		var department *departmentResponse
		if employee.DepartmentID != nil && employee.DepartmentName != nil {
			department = &departmentResponse{
				ID:   strconv.FormatUint(*employee.DepartmentID, 10),
				Name: *employee.DepartmentName,
			}
		}
		items = append(items, employeeResponse{
			ID:           strconv.FormatUint(employee.ID, 10),
			Name:         employee.Name,
			LoginAccount: employee.LoginAccount,
			Department:   department,
			Roles:        roles,
			Phone:        employee.Phone,
			Status:       statusName(employee.Status),
			CreatedAt:    employee.CreatedAt.Format(timeFormat),
		})
	}
	return employeeListResponse{Items: items, Page: page, PageSize: pageSize, Total: total}
}

// newEmployeeOptionsResponse 转换员工选项响应。
func newEmployeeOptionsResponse(options PlatformEmployeeOptions) employeeOptionsResponse {
	roles := make([]employeeOptionResponse, 0, len(options.Roles))
	for _, item := range options.Roles {
		roles = append(roles, employeeOptionResponse{ID: strconv.FormatUint(item.ID, 10), Name: item.Name, Status: statusName(item.Status), Assignable: item.Assignable})
	}
	departments := make([]employeeOptionResponse, 0, len(options.Departments))
	for _, item := range options.Departments {
		departments = append(departments, employeeOptionResponse{ID: strconv.FormatUint(item.ID, 10), Name: item.Name, Status: statusName(item.Status)})
	}
	return employeeOptionsResponse{Roles: roles, Departments: departments}
}

// parseEmployeeMutation 读取并规范化员工新增或编辑请求。
func parseEmployeeMutation(context *gin.Context, create bool) (EmployeeMutation, bool) {
	var request employeeMutationRequest
	if context.ShouldBindJSON(&request) != nil || !normalizeEmployeeRequest(&request, create) {
		return EmployeeMutation{}, false
	}
	departmentID, ok := parseOptionalID(request.DepartmentID)
	if !ok {
		return EmployeeMutation{}, false
	}
	roleIDs, ok := parseIDs(request.RoleIDs)
	if !ok {
		return EmployeeMutation{}, false
	}
	status, _ := parseStatus(request.Status)
	return EmployeeMutation{
		Name:         request.Name,
		LoginAccount: request.LoginAccount,
		Password:     request.Password,
		Phone:        request.Phone,
		DepartmentID: departmentID,
		RoleIDs:      roleIDs,
		Status:       status,
	}, true
}

// normalizeEmployeeRequest 清理并校验员工表单字段。
func normalizeEmployeeRequest(request *employeeMutationRequest, create bool) bool {
	request.Name = strings.TrimSpace(request.Name)
	request.LoginAccount = strings.TrimSpace(request.LoginAccount)
	if request.Phone != nil {
		value := strings.TrimSpace(*request.Phone)
		if value == "" {
			request.Phone = nil
		} else {
			request.Phone = &value
		}
	}
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 30 || request.LoginAccount == "" || utf8.RuneCountInString(request.LoginAccount) > 40 || (request.Phone != nil && utf8.RuneCountInString(*request.Phone) > 20) {
		return false
	}
	if create && !validPassword(request.Password) {
		return false
	}
	if request.Status == "" {
		request.Status = "enabled"
	}
	_, ok := parseStatus(request.Status)
	return ok
}

// parsePlatformEmployeeQuery 读取并严格校验平台员工分页及筛选参数。
func parsePlatformEmployeeQuery(context *gin.Context) (PlatformEmployeeQuery, bool) {
	page, valid := parsePositiveIntQuery(context, "page", 1)
	if !valid {
		return PlatformEmployeeQuery{}, false
	}
	pageSize, valid := parsePositiveIntQuery(context, "pageSize", defaultPageSize)
	if !valid || pageSize > maximumPageSize {
		return PlatformEmployeeQuery{}, false
	}
	maximumInt := int(^uint(0) >> 1)
	if page-1 > maximumInt/pageSize {
		return PlatformEmployeeQuery{}, false
	}

	query := PlatformEmployeeQuery{
		Page:         page,
		PageSize:     pageSize,
		Name:         strings.TrimSpace(context.Query("name")),
		LoginAccount: strings.TrimSpace(context.Query("loginAccount")),
	}
	if utf8.RuneCountInString(query.Name) > 30 || utf8.RuneCountInString(query.LoginAccount) > 40 {
		return PlatformEmployeeQuery{}, false
	}

	departmentID, valid := parseOptionalUintQuery(context, "departmentId")
	if !valid {
		return PlatformEmployeeQuery{}, false
	}
	query.DepartmentID = departmentID
	roleID, valid := parseOptionalUintQuery(context, "roleId")
	if !valid {
		return PlatformEmployeeQuery{}, false
	}
	query.RoleID = roleID

	if rawStatus, exists := context.GetQuery("status"); exists {
		switch strings.TrimSpace(rawStatus) {
		case "enabled":
			status := uint8(1)
			query.Status = &status
		case "disabled":
			status := uint8(0)
			query.Status = &status
		default:
			return PlatformEmployeeQuery{}, false
		}
	}
	return query, true
}

// parsePositiveIntQuery 读取正整数查询参数，缺省时返回指定默认值。
func parsePositiveIntQuery(context *gin.Context, name string, defaultValue int) (int, bool) {
	rawValue, exists := context.GetQuery(name)
	if !exists {
		return defaultValue, true
	}
	value, err := strconv.Atoi(strings.TrimSpace(rawValue))
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

// parseOptionalUintQuery 读取可选的正整数 BIGINT 查询参数。
func parseOptionalUintQuery(context *gin.Context, name string) (*uint64, bool) {
	rawValue, exists := context.GetQuery(name)
	if !exists {
		return nil, true
	}
	value, err := strconv.ParseUint(strings.TrimSpace(rawValue), 10, 64)
	if err != nil || value == 0 {
		return nil, false
	}
	return &value, true
}

// statusName 将数据库启用标记转换为稳定的接口状态值。
func statusName(status uint8) string {
	if status == 1 {
		return "enabled"
	}
	return "disabled"
}

// writeEmployeeError 将 Employee 业务错误转换为统一 HTTP 响应。
func writeEmployeeError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, errManagementNotFound):
		httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
	case errors.Is(err, errManagementConflict):
		httpapi.WriteError(context, httpapi.ErrorCodeConflict)
	case errors.Is(err, errManagementProtected):
		httpapi.WriteError(context, httpapi.ErrorCodeProtectedResource)
	case errors.Is(err, errManagementForbidden):
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
	default:
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
	}
}
