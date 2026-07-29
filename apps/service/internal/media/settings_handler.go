package media

import (
	"net/http"
	"strings"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// GetPlatformBasicSettings 返回平台品牌名称和当前图片库图标。
func (handler *Handler) GetPlatformBasicSettings(context *gin.Context) {
	settings, err := handler.service.GetPlatformBasicSettings(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, toBasicSettingsResponse(settings))
}

// UpdatePlatformBasicSettings 校验图片库图标后保存平台品牌。
func (handler *Handler) UpdatePlatformBasicSettings(context *gin.Context) {
	var request basicSettingsRequest
	if err := context.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" || len([]rune(strings.TrimSpace(request.Name))) > 100 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdatePlatformBasicSettings(context.Request.Context(), strings.TrimSpace(request.Name), request.IconImageID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// GetTenantBasicSettings 返回认证上下文中可信租户的品牌设置。
func (handler *Handler) GetTenantBasicSettings(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	settings, err := handler.service.GetTenantBasicSettings(context.Request.Context(), tenantID)
	if err != nil {
		writeMediaError(context, err)
		return
	}
	response := toBasicSettingsResponse(settings)
	if response.Icon == nil && settings.LegacyIconURL != nil && strings.TrimSpace(*settings.LegacyIconURL) != "" {
		response.Icon = &imageSummaryResponse{OriginalName: "兼容图标", PreviewURL: *settings.LegacyIconURL}
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// UpdateTenantBasicSettings 校验图片来自平台共享图库或当前租户后保存租户品牌。
func (handler *Handler) UpdateTenantBasicSettings(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	var request basicSettingsRequest
	if err := context.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" || len([]rune(strings.TrimSpace(request.Name))) > 100 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdateTenantBasicSettings(context.Request.Context(), tenantID, strings.TrimSpace(request.Name), request.IconImageID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// PublicPlatformBrand 返回无需登录即可使用的平台名称和稳定图标代理地址。
func (handler *Handler) PublicPlatformBrand(context *gin.Context) {
	settings, err := handler.service.PublicPlatformBrand(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	var iconURL *string
	if settings.IconImageID != nil {
		value := publicImageURL(*settings.IconImageID)
		iconURL = &value
	}
	httpapi.WriteSuccess(context, http.StatusOK, gin.H{"name": settings.Name, "iconUrl": iconURL})
}

// PublicImage 仅代理当前正在被平台或租户品牌引用的私有图片。
