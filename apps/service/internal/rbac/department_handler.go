package rbac

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// DepartmentApplication 定义 Department Handler 依赖的业务服务能力。
type DepartmentApplication interface {
	ListDepartments(context.Context, managementScope) ([]PlatformDepartment, error)
	CreateDepartment(context.Context, managementScope, DepartmentMutation) error
	UpdateDepartment(context.Context, managementScope, uint64, DepartmentMutation) error
	DeleteDepartment(context.Context, managementScope, uint64) error
}

// DepartmentHandler 组织平台和租户部门接口的请求解析与响应转换。
type DepartmentHandler struct {
	departments DepartmentApplication
}

// NewDepartmentHandler 使用部门业务服务创建部门接口处理器。
func NewDepartmentHandler(departments DepartmentApplication) *DepartmentHandler {
	return &DepartmentHandler{departments: departments}
}

// departmentMutationRequest 描述部门新增和编辑接口接收的字段。
type departmentMutationRequest struct {
	ParentID         *string `json:"parentId"`
	Name             string  `json:"name"`
	LeaderEmployeeID *string `json:"leaderEmployeeId"`
	Sort             uint32  `json:"sort"`
	Status           string  `json:"status"`
}

// departmentLeaderResponse 描述部门负责人在响应中的 ID 和名称。
type departmentLeaderResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// platformDepartmentResponse 描述平台或租户部门的平铺响应节点。
type platformDepartmentResponse struct {
	ID            string                    `json:"id"`
	ParentID      *string                   `json:"parentId"`
	Name          string                    `json:"name"`
	Leader        *departmentLeaderResponse `json:"leader"`
	EmployeeCount int64                     `json:"employeeCount"`
	Sort          uint32                    `json:"sort"`
	Status        string                    `json:"status"`
}

// departmentListResponse 包装部门平铺列表，空结果保持为空数组。
type departmentListResponse struct {
	Items []platformDepartmentResponse `json:"items"`
}

// ListPlatformDepartments 返回平台部门平铺列表，由前端按父节点构建树。
func (handler *DepartmentHandler) ListPlatformDepartments(context *gin.Context) {
	handler.listDepartments(context, managementScope{Name: "platform"})
}

// ListTenantDepartments 返回当前租户部门平铺列表。
func (handler *DepartmentHandler) ListTenantDepartments(context *gin.Context) {
	scope, ok := tenantScopeFromContext(context)
	if ok {
		handler.listDepartments(context, scope)
	}
}

// CreatePlatformDepartment 创建平台部门。
func (handler *DepartmentHandler) CreatePlatformDepartment(context *gin.Context) {
	handler.createDepartment(context, managementScope{Name: "platform"})
}

// CreateTenantDepartment 创建当前租户部门。
func (handler *DepartmentHandler) CreateTenantDepartment(context *gin.Context) {
	scope, ok := tenantScopeFromContext(context)
	if ok {
		handler.createDepartment(context, scope)
	}
}

// UpdatePlatformDepartment 更新平台部门。
func (handler *DepartmentHandler) UpdatePlatformDepartment(context *gin.Context) {
	handler.updateDepartment(context, managementScope{Name: "platform"})
}

// UpdateTenantDepartment 更新当前租户部门。
func (handler *DepartmentHandler) UpdateTenantDepartment(context *gin.Context) {
	scope, ok := tenantScopeFromContext(context)
	if ok {
		handler.updateDepartment(context, scope)
	}
}

// DeletePlatformDepartment 删除空的平台部门。
func (handler *DepartmentHandler) DeletePlatformDepartment(context *gin.Context) {
	handler.deleteDepartment(context, managementScope{Name: "platform"})
}

// DeleteTenantDepartment 删除当前租户空部门。
func (handler *DepartmentHandler) DeleteTenantDepartment(context *gin.Context) {
	scope, ok := tenantScopeFromContext(context)
	if ok {
		handler.deleteDepartment(context, scope)
	}
}

// listDepartments 返回指定工作空间的部门平铺列表。
func (handler *DepartmentHandler) listDepartments(context *gin.Context, scope managementScope) {
	departments, err := handler.departments.ListDepartments(context.Request.Context(), scope)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, departmentListResponse{Items: newDepartmentResponses(departments)})
}

// createDepartment 创建已通过 HTTP 字段校验的部门。
func (handler *DepartmentHandler) createDepartment(context *gin.Context, scope managementScope) {
	mutation, ok := bindDepartmentMutation(context)
	if !ok {
		return
	}
	if err := handler.departments.CreateDepartment(context.Request.Context(), scope, mutation); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, nil)
}

// updateDepartment 更新指定部门基础资料。
func (handler *DepartmentHandler) updateDepartment(context *gin.Context, scope managementScope) {
	departmentID, valid := parsePathUint(context, "departmentId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	mutation, ok := bindDepartmentMutation(context)
	if !ok {
		return
	}
	if err := handler.departments.UpdateDepartment(context.Request.Context(), scope, departmentID, mutation); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// deleteDepartment 删除指定工作空间的空部门。
func (handler *DepartmentHandler) deleteDepartment(context *gin.Context, scope managementScope) {
	departmentID, valid := parsePathUint(context, "departmentId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.departments.DeleteDepartment(context.Request.Context(), scope, departmentID); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// bindDepartmentMutation 绑定并转换部门新增和编辑请求。
func bindDepartmentMutation(context *gin.Context) (DepartmentMutation, bool) {
	var request departmentMutationRequest
	if context.ShouldBindJSON(&request) != nil || !normalizeDepartmentRequest(&request) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return DepartmentMutation{}, false
	}
	parentID, ok := parseOptionalID(request.ParentID)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return DepartmentMutation{}, false
	}
	leaderID, ok := parseOptionalID(request.LeaderEmployeeID)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return DepartmentMutation{}, false
	}
	status, _ := parseStatus(request.Status)
	return DepartmentMutation{ParentID: parentID, Name: request.Name, LeaderEmployeeID: leaderID, Sort: request.Sort, Status: status}, true
}

// normalizeDepartmentRequest 清理并校验部门字段。
func normalizeDepartmentRequest(request *departmentMutationRequest) bool {
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 40 {
		return false
	}
	if request.Status == "" {
		request.Status = "enabled"
	}
	_, ok := parseStatus(request.Status)
	return ok
}

// newDepartmentResponses 转换部门平铺响应。
func newDepartmentResponses(items []PlatformDepartment) []platformDepartmentResponse {
	responses := make([]platformDepartmentResponse, 0, len(items))
	for _, item := range items {
		var parentID *string
		if item.ParentID != nil {
			value := strconv.FormatUint(*item.ParentID, 10)
			parentID = &value
		}
		var leader *departmentLeaderResponse
		if item.LeaderEmployeeID != nil && item.LeaderName != nil {
			leader = &departmentLeaderResponse{ID: strconv.FormatUint(*item.LeaderEmployeeID, 10), Name: *item.LeaderName}
		}
		responses = append(responses, platformDepartmentResponse{ID: strconv.FormatUint(item.ID, 10), ParentID: parentID, Name: item.Name, Leader: leader, EmployeeCount: item.EmployeeCount, Sort: item.Sort, Status: statusName(item.Status)})
	}
	return responses
}
