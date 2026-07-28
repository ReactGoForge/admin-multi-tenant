package rbac

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// RoleHandler 组织平台和租户角色接口的参数校验与响应转换。
type RoleHandler struct {
	service RoleApplication
}

// NewRoleHandler 使用 Role 业务服务创建角色处理器。
func NewRoleHandler(service RoleApplication) *RoleHandler {
	return &RoleHandler{service: service}
}

// roleResponse 描述角色列表中的基础信息和关联统计。
type roleResponse struct {
	ID                     string  `json:"id"`
	Name                   string  `json:"name"`
	Description            *string `json:"description"`
	Type                   string  `json:"type"`
	SystemKey              *string `json:"systemKey"`
	Status                 string  `json:"status"`
	EmployeeCount          int64   `json:"employeeCount"`
	PermissionCount        int64   `json:"permissionCount"`
	PermissionConfigurable bool    `json:"permissionConfigurable"`
	CreatedAt              string  `json:"createdAt"`
}

// roleListResponse 描述角色列表统一分页响应。
type roleListResponse struct {
	Items    []roleResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int64          `json:"total"`
}

// menuResponse 描述角色权限树中的菜单节点。
type menuResponse struct {
	ID               string  `json:"id"`
	ParentID         *string `json:"parentId"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	Scope            string  `json:"scope"`
	Path             *string `json:"path"`
	Component        *string `json:"component"`
	Icon             *string `json:"icon"`
	PermissionCode   *string `json:"permissionCode"`
	TenantAssignable bool    `json:"tenantAssignable"`
	Sort             uint32  `json:"sort"`
	Visible          bool    `json:"visible"`
	Status           string  `json:"status"`
}

// roleDetailResponse 描述租户角色详情、当前权限和权限树。
type roleDetailResponse struct {
	Role          roleResponse   `json:"role"`
	PermissionIDs []string       `json:"permissionIds"`
	Menus         []menuResponse `json:"menus"`
}

// platformRoleDetailResponse 描述平台角色在平台与租户两类权限中的详情。
type platformRoleDetailResponse struct {
	Role                  roleResponse   `json:"role"`
	PlatformPermissionIDs []string       `json:"platformPermissionIds"`
	TenantPermissionIDs   []string       `json:"tenantPermissionIds"`
	PlatformMenus         []menuResponse `json:"platformMenus"`
	TenantMenus           []menuResponse `json:"tenantMenus"`
}

// roleMutationRequest 描述角色新增和编辑接口接收的字段。
type roleMutationRequest struct {
	Name                  string   `json:"name"`
	Description           *string  `json:"description"`
	PermissionIDs         []string `json:"permissionIds"`
	PlatformPermissionIDs []string `json:"platformPermissionIds"`
	TenantPermissionIDs   []string `json:"tenantPermissionIds"`
	Status                string   `json:"status"`
}

// rolePermissionsRequest 描述角色权限替换接口接收的各工作空间权限 ID。
type rolePermissionsRequest struct {
	PermissionIDs         []string `json:"permissionIds"`
	PlatformPermissionIDs []string `json:"platformPermissionIds"`
	TenantPermissionIDs   []string `json:"tenantPermissionIds"`
}

// ListPlatformRoles 校验分页与筛选参数并返回平台角色列表。
func (handler *RoleHandler) ListPlatformRoles(context *gin.Context) {
	query, valid := parsePlatformRoleQuery(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	roles, total, err := handler.service.ListPlatformRoles(context.Request.Context(), actor, query)
	if err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newRoleListResponse(roles, query.Page, query.PageSize, total))
}

// ListTenantRoles 返回当前租户角色分页列表。
func (handler *RoleHandler) ListTenantRoles(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	handler.listTenantRoles(context, scope)
}

// listTenantRoles 校验分页与筛选参数并返回租户角色列表。
func (handler *RoleHandler) listTenantRoles(context *gin.Context, scope managementScope) {
	query, valid := parsePlatformRoleQuery(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	roles, total, err := handler.service.ListTenantRoles(context.Request.Context(), actor, scope, query)
	if err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newRoleListResponse(roles, query.Page, query.PageSize, total))
}

// PlatformRoleDetail 返回平台角色详情、有效权限 ID 与只读权限树节点。
func (handler *RoleHandler) PlatformRoleDetail(context *gin.Context) {
	roleID, valid := parsePathUint(context, "roleId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	detail, err := handler.service.PlatformRoleDetail(context.Request.Context(), actor, roleID)
	if err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, platformRoleDetailResponse{
		Role:                  newRoleResponse(detail.Role),
		PlatformPermissionIDs: uint64Strings(detail.PlatformPermissionIDs),
		TenantPermissionIDs:   uint64Strings(detail.TenantPermissionIDs),
		PlatformMenus:         newMenuResponses(detail.PlatformMenus),
		TenantMenus:           newMenuResponses(detail.TenantMenus),
	})
}

// TenantRoleDetail 返回当前租户角色详情和可分配权限树。
func (handler *RoleHandler) TenantRoleDetail(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	roleID, valid := parsePathUint(context, "roleId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	detail, err := handler.service.TenantRoleDetail(context.Request.Context(), actor, scope, roleID)
	if err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, roleDetailResponse{Role: newRoleResponse(detail.Role), PermissionIDs: uint64Strings(detail.PermissionIDs), Menus: newMenuResponses(detail.Menus)})
}

// PlatformRolePermissionOptions 返回平台角色新增表单使用的两棵可分配权限树。
func (handler *RoleHandler) PlatformRolePermissionOptions(context *gin.Context) {
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	options, err := handler.service.PlatformPermissionOptions(context.Request.Context(), actor)
	if err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, gin.H{
		"platformMenus": newMenuResponses(options.PlatformMenus),
		"tenantMenus":   newMenuResponses(options.TenantMenus),
	})
}

// ListPlatformRoleEmployees 返回指定平台角色当前关联的员工分页结果。
func (handler *RoleHandler) ListPlatformRoleEmployees(context *gin.Context) {
	roleID, validID := parsePathUint(context, "roleId")
	page, pageSize, validPagination := parsePagination(context)
	if !validID || !validPagination {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	employees, total, err := handler.service.ListPlatformRoleEmployees(context.Request.Context(), actor, roleID, PlatformEmployeeQuery{Page: page, PageSize: pageSize})
	if err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newEmployeeListResponse(employees, page, pageSize, total))
}

// ListTenantRoleEmployees 返回当前租户指定角色的员工分页列表。
func (handler *RoleHandler) ListTenantRoleEmployees(context *gin.Context) {
	scope, ok := tenantScope(context)
	if !ok {
		return
	}
	roleID, validID := parsePathUint(context, "roleId")
	page, pageSize, validPagination := parsePagination(context)
	if !validID || !validPagination {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	employees, total, err := handler.service.ListTenantRoleEmployees(context.Request.Context(), actor, scope, roleID, PlatformEmployeeQuery{Page: page, PageSize: pageSize})
	if err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newEmployeeListResponse(employees, page, pageSize, total))
}

// CreatePlatformRole 创建平台自定义角色及权限关联。
func (handler *RoleHandler) CreatePlatformRole(context *gin.Context) {
	handler.createRole(context, managementScope{Name: "platform"})
}

// CreateTenantRole 创建当前租户自定义角色及权限关联。
func (handler *RoleHandler) CreateTenantRole(context *gin.Context) {
	scope, ok := tenantScope(context)
	if ok {
		handler.createRole(context, scope)
	}
}

// createRole 校验请求并交给 Service 创建角色。
func (handler *RoleHandler) createRole(context *gin.Context, scope managementScope) {
	mutation, valid := parseRoleMutation(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.CreateRole(context.Request.Context(), actor, scope, mutation); err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, nil)
}

// UpdatePlatformRole 更新平台自定义角色基本信息。
func (handler *RoleHandler) UpdatePlatformRole(context *gin.Context) {
	handler.updateRole(context, managementScope{Name: "platform"})
}

// UpdateTenantRole 更新当前租户自定义角色基本信息。
func (handler *RoleHandler) UpdateTenantRole(context *gin.Context) {
	scope, ok := tenantScope(context)
	if ok {
		handler.updateRole(context, scope)
	}
}

// updateRole 校验请求并交给 Service 更新角色基础资料。
func (handler *RoleHandler) updateRole(context *gin.Context, scope managementScope) {
	roleID, validID := parsePathUint(context, "roleId")
	mutation, validMutation := parseRoleMutation(context)
	if !validID || !validMutation {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.UpdateRole(context.Request.Context(), actor, scope, roleID, mutation); err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// AssignPlatformRolePermissions 替换平台自定义角色权限。
func (handler *RoleHandler) AssignPlatformRolePermissions(context *gin.Context) {
	handler.assignRolePermissions(context, managementScope{Name: "platform"})
}

// AssignTenantRolePermissions 替换当前租户自定义角色权限。
func (handler *RoleHandler) AssignTenantRolePermissions(context *gin.Context) {
	scope, ok := tenantScope(context)
	if ok {
		handler.assignRolePermissions(context, scope)
	}
}

// assignRolePermissions 校验权限 ID 并交给 Service 替换角色权限。
func (handler *RoleHandler) assignRolePermissions(context *gin.Context, scope managementScope) {
	roleID, validID := parsePathUint(context, "roleId")
	mutation, validMutation := parseRolePermissionMutation(context)
	if !validID || !validMutation {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.AssignRolePermissions(context.Request.Context(), actor, scope, roleID, mutation); err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// SetPlatformRoleStatus 更新平台自定义角色状态。
func (handler *RoleHandler) SetPlatformRoleStatus(context *gin.Context) {
	handler.setRoleStatus(context, managementScope{Name: "platform"})
}

// SetTenantRoleStatus 更新当前租户自定义角色状态。
func (handler *RoleHandler) SetTenantRoleStatus(context *gin.Context) {
	scope, ok := tenantScope(context)
	if ok {
		handler.setRoleStatus(context, scope)
	}
}

// setRoleStatus 校验状态并交给 Service 更新。
func (handler *RoleHandler) setRoleStatus(context *gin.Context, scope managementScope) {
	roleID, validID := parsePathUint(context, "roleId")
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
	if err := handler.service.SetRoleStatus(context.Request.Context(), actor, scope, roleID, status); err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// DeletePlatformRole 删除无员工关联的平台自定义角色。
func (handler *RoleHandler) DeletePlatformRole(context *gin.Context) {
	handler.deleteRole(context, managementScope{Name: "platform"})
}

// DeleteTenantRole 删除无员工关联的当前租户自定义角色。
func (handler *RoleHandler) DeleteTenantRole(context *gin.Context) {
	scope, ok := tenantScope(context)
	if ok {
		handler.deleteRole(context, scope)
	}
}

// deleteRole 校验路径 ID 并交给 Service 删除角色。
func (handler *RoleHandler) deleteRole(context *gin.Context, scope managementScope) {
	roleID, valid := parsePathUint(context, "roleId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	actor, ok := currentEmployeeActor(context)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	if err := handler.service.DeleteRole(context.Request.Context(), actor, scope, roleID); err != nil {
		writeRoleError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// parseRoleMutation 读取并规范化角色新增或编辑请求。
func parseRoleMutation(context *gin.Context) (RoleMutation, bool) {
	var request roleMutationRequest
	if context.ShouldBindJSON(&request) != nil || !normalizeRoleRequest(&request) {
		return RoleMutation{}, false
	}
	permissionIDs, ok := parseIDs(request.PermissionIDs)
	if !ok {
		return RoleMutation{}, false
	}
	platformPermissionIDs, ok := parseIDs(request.PlatformPermissionIDs)
	if !ok {
		return RoleMutation{}, false
	}
	tenantPermissionIDs, ok := parseIDs(request.TenantPermissionIDs)
	if !ok {
		return RoleMutation{}, false
	}
	status, _ := parseStatus(request.Status)
	return RoleMutation{Name: request.Name, Description: request.Description, PermissionIDs: permissionIDs, PlatformPermissionIDs: platformPermissionIDs, TenantPermissionIDs: tenantPermissionIDs, Status: status}, true
}

// parseRolePermissionMutation 读取并校验角色授权请求。
func parseRolePermissionMutation(context *gin.Context) (RolePermissionMutation, bool) {
	var request rolePermissionsRequest
	if context.ShouldBindJSON(&request) != nil {
		return RolePermissionMutation{}, false
	}
	permissionIDs, ok := parseIDs(request.PermissionIDs)
	if !ok {
		return RolePermissionMutation{}, false
	}
	platformPermissionIDs, ok := parseIDs(request.PlatformPermissionIDs)
	if !ok {
		return RolePermissionMutation{}, false
	}
	tenantPermissionIDs, ok := parseIDs(request.TenantPermissionIDs)
	if !ok {
		return RolePermissionMutation{}, false
	}
	return RolePermissionMutation{PermissionIDs: permissionIDs, PlatformPermissionIDs: platformPermissionIDs, TenantPermissionIDs: tenantPermissionIDs}, true
}

// normalizeRoleRequest 清理并校验角色字段。
func normalizeRoleRequest(request *roleMutationRequest) bool {
	request.Name = strings.TrimSpace(request.Name)
	if request.Description != nil {
		value := strings.TrimSpace(*request.Description)
		if value == "" {
			request.Description = nil
		} else {
			request.Description = &value
		}
	}
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 30 || (request.Description != nil && utf8.RuneCountInString(*request.Description) > 200) {
		return false
	}
	if request.Status == "" {
		request.Status = "enabled"
	}
	_, ok := parseStatus(request.Status)
	return ok
}

// newRoleListResponse 将角色查询结果转换为统一分页响应。
func newRoleListResponse(roles []PlatformRole, page, pageSize int, total int64) roleListResponse {
	items := make([]roleResponse, 0, len(roles))
	for _, role := range roles {
		items = append(items, newRoleResponse(role))
	}
	return roleListResponse{Items: items, Page: page, PageSize: pageSize, Total: total}
}

// newRoleResponse 将平台角色查询结果转换为稳定接口结构。
func newRoleResponse(role PlatformRole) roleResponse {
	return roleResponse{
		ID:                     strconv.FormatUint(role.ID, 10),
		Name:                   role.Name,
		Description:            role.Description,
		Type:                   role.Type,
		SystemKey:              role.SystemKey,
		Status:                 statusName(role.Status),
		EmployeeCount:          role.EmployeeCount,
		PermissionCount:        role.PermissionCount,
		PermissionConfigurable: role.PermissionConfigurable,
		CreatedAt:              role.CreatedAt.Format(timeFormat),
	}
}

// newMenuResponses 将菜单查询结果转换为字符串 ID 和布尔状态响应。
func newMenuResponses(menus []PlatformMenu) []menuResponse {
	items := make([]menuResponse, 0, len(menus))
	for _, menu := range menus {
		name := menu.Name
		if menu.PermissionCode != nil && *menu.PermissionCode == "platform:field:view" {
			name = "字典管理"
		}
		var parentID *string
		if menu.ParentID != nil {
			value := strconv.FormatUint(*menu.ParentID, 10)
			parentID = &value
		}
		items = append(items, menuResponse{
			ID:               strconv.FormatUint(menu.ID, 10),
			ParentID:         parentID,
			Name:             name,
			Type:             menu.Type,
			Scope:            menu.Scope,
			Path:             menu.Path,
			Component:        menu.Component,
			Icon:             menu.Icon,
			PermissionCode:   menu.PermissionCode,
			TenantAssignable: menu.TenantAssignable == 1,
			Sort:             menu.Sort,
			Visible:          menu.Visible == 1,
			Status:           statusName(menu.Status),
		})
	}
	return items
}

// uint64Strings 将数据库 BIGINT ID 数组转换为 JSON 安全的字符串数组。
func uint64Strings(values []uint64) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.FormatUint(value, 10))
	}
	return items
}

// writeRoleError 将 Role 业务错误转换为统一 HTTP 响应。
func writeRoleError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, errManagementNotFound):
		httpapi.WriteError(context, httpapi.ErrorCodeRoleNotFound)
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

// parsePlatformRoleQuery 读取并严格校验平台角色分页与筛选参数。
func parsePlatformRoleQuery(context *gin.Context) (PlatformRoleQuery, bool) {
	page, pageSize, valid := parsePagination(context)
	if !valid {
		return PlatformRoleQuery{}, false
	}
	query := PlatformRoleQuery{Page: page, PageSize: pageSize, Name: strings.TrimSpace(context.Query("name"))}
	if utf8.RuneCountInString(query.Name) > 30 {
		return PlatformRoleQuery{}, false
	}
	if rawType, exists := context.GetQuery("type"); exists {
		query.Type = strings.TrimSpace(rawType)
		if query.Type != "system" && query.Type != "custom" {
			return PlatformRoleQuery{}, false
		}
	}
	if rawStatus, exists := context.GetQuery("status"); exists {
		switch strings.TrimSpace(rawStatus) {
		case "enabled":
			status := uint8(1)
			query.Status = &status
		case "disabled":
			status := uint8(0)
			query.Status = &status
		default:
			return PlatformRoleQuery{}, false
		}
	}
	return query, true
}

// parsePagination 读取统一分页参数并拒绝非法值和偏移溢出。
func parsePagination(context *gin.Context) (int, int, bool) {
	page, valid := parsePositiveIntQuery(context, "page", 1)
	if !valid {
		return 0, 0, false
	}
	pageSize, valid := parsePositiveIntQuery(context, "pageSize", defaultPageSize)
	if !valid || pageSize > maximumPageSize {
		return 0, 0, false
	}
	maximumInt := int(^uint(0) >> 1)
	if page-1 > maximumInt/pageSize {
		return 0, 0, false
	}
	return page, pageSize, true
}

// parsePathUint 读取路由路径中的正整数 BIGINT ID。
func parsePathUint(context *gin.Context, name string) (uint64, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(context.Param(name)), 10, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return value, true
}
