package media

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

func (handler *Handler) ListPlatformCategories(context *gin.Context) {
	owner, valid := optionalID(context.Query("tenantId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.listCategories(context, owner, false)
}

// CreatePlatformCategory 为平台或指定租户新增分类。
func (handler *Handler) CreatePlatformCategory(context *gin.Context) {
	var request categoryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.createCategory(context, request.TenantID, request.Name)
}

// UpdatePlatformCategory 修改平台或指定租户的分类名称。
func (handler *Handler) UpdatePlatformCategory(context *gin.Context) {
	owner, valid := optionalID(context.Query("tenantId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.updateCategory(context, owner)
}

// DeletePlatformCategory 删除平台或指定租户的分类并将图片转入未分类。
func (handler *Handler) DeletePlatformCategory(context *gin.Context) {
	owner, valid := optionalID(context.Query("tenantId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.deleteCategory(context, owner)
}

// ListTenantCategories 读取当前租户分类或平台共享分类。
func (handler *Handler) ListTenantCategories(context *gin.Context) {
	sharedOnly := context.DefaultQuery("source", "tenant") == "platform"
	owner, valid := tenantOwnerFromQuery(context)
	if !valid {
		return
	}
	handler.listCategories(context, owner, sharedOnly)
}

// CreateTenantCategory 为当前可信租户新增图片分类。
func (handler *Handler) CreateTenantCategory(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	var request categoryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.createCategory(context, &tenantID, request.Name)
}

// UpdateTenantCategory 修改当前可信租户的分类名称。
func (handler *Handler) UpdateTenantCategory(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	handler.updateCategory(context, &tenantID)
}

// DeleteTenantCategory 删除当前可信租户分类并将图片转入未分类。
func (handler *Handler) DeleteTenantCategory(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	handler.deleteCategory(context, &tenantID)
}

// PlatformTenantOptions 返回平台图片管理的租户筛选选项。

func (handler *Handler) listCategories(context *gin.Context, ownerTenantID *uint64, sharedOnly bool) {
	categories, err := handler.service.ListCategories(context.Request.Context(), ownerTenantID, sharedOnly)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	result := make([]categoryResponse, 0, len(categories))
	for _, category := range categories {
		result = append(result, categoryResponse{ID: strconv.FormatUint(category.ID, 10), TenantID: formatOptionalID(category.TenantID), Name: category.Name, IsShared: category.IsShared})
	}
	httpapi.WriteSuccess(context, http.StatusOK, result)
}

// createCategory 校验名称后新增指定所有者的分类。
func (handler *Handler) createCategory(context *gin.Context, ownerTenantID *uint64, rawName string) {
	name := strings.TrimSpace(rawName)
	if name == "" || len([]rune(name)) > 40 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	category, err := handler.service.CreateCategory(context.Request.Context(), ownerTenantID, name)
	if err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, categoryResponse{ID: strconv.FormatUint(category.ID, 10), TenantID: formatOptionalID(category.TenantID), Name: category.Name, IsShared: category.IsShared})
}

// updateCategory 校验分类归属与名称后更新分类。
func (handler *Handler) updateCategory(context *gin.Context, ownerTenantID *uint64) {
	categoryID, valid := parseID(context.Param("categoryId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	var request categoryRequest
	if err := context.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" || len([]rune(strings.TrimSpace(request.Name))) > 40 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdateCategory(context.Request.Context(), categoryID, ownerTenantID, strings.TrimSpace(request.Name)); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// deleteCategory 删除指定所有者的分类。
func (handler *Handler) deleteCategory(context *gin.Context, ownerTenantID *uint64) {
	categoryID, valid := parseID(context.Param("categoryId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.DeleteCategory(context.Request.Context(), categoryID, ownerTenantID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}
