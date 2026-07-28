package rbac

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// MenuApplication 定义 Menu Handler 依赖的业务服务能力。
type MenuApplication interface {
	ListMenus(context.Context, string) ([]PlatformMenu, error)
	CreateMenu(context.Context, MenuMutation) error
	UpdateMenu(context.Context, uint64, MenuMutation) error
	SetMenuStatus(context.Context, uint64, uint8) error
	DeleteMenu(context.Context, uint64) error
}

// MenuHandler 组织平台和租户菜单接口的请求解析与响应转换。
type MenuHandler struct {
	menus MenuApplication
}

// NewMenuHandler 使用菜单业务服务创建菜单接口处理器。
func NewMenuHandler(menus MenuApplication) *MenuHandler {
	return &MenuHandler{menus: menus}
}

// menuListResponse 包装菜单管理返回的节点列表。
type menuListResponse struct {
	Items []menuResponse `json:"items"`
}

// menuMutationRequest 描述菜单新增和编辑接口接收的可选字段。
type menuMutationRequest struct {
	Scope            string  `json:"scope"`
	ParentID         *string `json:"parentId"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	Path             *string `json:"path"`
	Component        *string `json:"component"`
	Icon             *string `json:"icon"`
	PermissionCode   *string `json:"permissionCode"`
	TenantAssignable bool    `json:"tenantAssignable"`
	Sort             uint32  `json:"sort"`
	Visible          bool    `json:"visible"`
	Status           string  `json:"status"`
}

// ListPlatformMenus 校验菜单范围并返回平台维护的全部启用和禁用节点。
func (handler *MenuHandler) ListPlatformMenus(context *gin.Context) {
	rawScope, exists := context.GetQuery("scope")
	if !exists {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	scope := strings.TrimSpace(rawScope)
	if scope != "platform" && scope != "tenant" {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.listMenus(context, scope)
}

// ListTenantMenus 返回当前租户可使用的只读租户菜单定义。
func (handler *MenuHandler) ListTenantMenus(context *gin.Context) {
	if _, ok := tenantScopeFromContext(context); ok {
		handler.listMenus(context, "tenant")
	}
}

// CreatePlatformMenu 校验请求并创建平台统一维护的菜单节点。
func (handler *MenuHandler) CreatePlatformMenu(context *gin.Context) {
	mutation, valid := parseMenuMutationRequest(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.menus.CreateMenu(context.Request.Context(), mutation); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, nil)
}

// UpdatePlatformMenu 校验请求并更新菜单节点的可编辑属性。
func (handler *MenuHandler) UpdatePlatformMenu(context *gin.Context) {
	menuID, valid := parsePathUint(context, "menuId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	mutation, valid := parseMenuMutationRequest(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.menus.UpdateMenu(context.Request.Context(), menuID, mutation); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// SetPlatformMenuStatus 校验状态并启用或停用菜单节点。
func (handler *MenuHandler) SetPlatformMenuStatus(context *gin.Context) {
	menuID, valid := parsePathUint(context, "menuId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	var request statusRequest
	if context.ShouldBindJSON(&request) != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	status, valid := parseStatus(request.Status)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.menus.SetMenuStatus(context.Request.Context(), menuID, status); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// DeletePlatformMenu 保守删除无子节点且无角色关联的菜单节点。
func (handler *MenuHandler) DeletePlatformMenu(context *gin.Context) {
	menuID, valid := parsePathUint(context, "menuId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.menus.DeleteMenu(context.Request.Context(), menuID); err != nil {
		writeManagementError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// listMenus 返回指定范围的菜单节点。
func (handler *MenuHandler) listMenus(context *gin.Context, scope string) {
	menus, err := handler.menus.ListMenus(context.Request.Context(), scope)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, menuListResponse{Items: newMenuResponses(menus)})
}

// parseMenuMutationRequest 清理菜单请求并校验各节点类型的字段组合。
func parseMenuMutationRequest(context *gin.Context) (MenuMutation, bool) {
	var request menuMutationRequest
	if context.ShouldBindJSON(&request) != nil {
		return MenuMutation{}, false
	}
	request.Scope = strings.TrimSpace(request.Scope)
	request.Name = strings.TrimSpace(request.Name)
	request.Type = strings.TrimSpace(request.Type)
	if request.Scope != "platform" && request.Scope != "tenant" {
		return MenuMutation{}, false
	}
	if request.Type != "directory" && request.Type != "menu" && request.Type != "permission" {
		return MenuMutation{}, false
	}
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 40 {
		return MenuMutation{}, false
	}
	parentID, valid := parseOptionalID(request.ParentID)
	if !valid {
		return MenuMutation{}, false
	}
	path, valid := normalizeOptionalMenuText(request.Path, 255)
	if !valid {
		return MenuMutation{}, false
	}
	component, valid := normalizeOptionalMenuText(request.Component, 255)
	if !valid {
		return MenuMutation{}, false
	}
	icon, valid := normalizeOptionalMenuText(request.Icon, 64)
	if !valid {
		return MenuMutation{}, false
	}
	permissionCode, valid := normalizeOptionalMenuText(request.PermissionCode, 100)
	if !valid {
		return MenuMutation{}, false
	}
	status, valid := parseStatus(request.Status)
	if !valid {
		return MenuMutation{}, false
	}

	mutation := MenuMutation{
		Scope: request.Scope, ParentID: parentID, Name: request.Name, Type: request.Type,
		Path: path, Component: component, Icon: icon, PermissionCode: permissionCode,
		Sort: request.Sort, Status: status,
	}
	if request.Scope == "tenant" && request.TenantAssignable {
		mutation.TenantAssignable = 1
	}
	if request.Visible {
		mutation.Visible = 1
	}
	return mutation, true
}

// normalizeOptionalMenuText 清理可选文本并拒绝空字符串和超长值。
func normalizeOptionalMenuText(value *string, maximum int) (*string, bool) {
	if value == nil {
		return nil, true
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > maximum {
		return nil, false
	}
	return &trimmed, true
}
