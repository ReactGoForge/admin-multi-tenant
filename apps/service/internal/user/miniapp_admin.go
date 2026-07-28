package user

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// MiniappAdminHandler 组织仅平台使用的微信配置和租户小程序码接口。
type MiniappAdminHandler struct {
	miniapp MiniappAdminApplication
}

// NewMiniappAdminHandler 使用微信平台管理服务创建处理器。
func NewMiniappAdminHandler(miniapp MiniappAdminApplication) *MiniappAdminHandler {
	return &MiniappAdminHandler{miniapp: miniapp}
}

// GetSettings 返回 AppID 与服务器密钥配置状态，不读取或返回密钥内容。
func (handler *MiniappAdminHandler) GetSettings(context *gin.Context) {
	settings, err := handler.miniapp.GetSettings(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, gin.H{"appId": settings.AppID, "secretConfigured": settings.SecretConfigured})
}

// UpdateSettings 保存全平台唯一微信小程序 AppID。
func (handler *MiniappAdminHandler) UpdateSettings(context *gin.Context) {
	var request miniappSettingsRequest
	if context.ShouldBindJSON(&request) != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	request.AppID = strings.TrimSpace(request.AppID)
	if request.AppID == "" || utf8.RuneCountInString(request.AppID) > 64 || !isASCII(request.AppID) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.miniapp.UpdateSettings(context.Request.Context(), request.AppID); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// TenantMiniappCode 读取小程序码，POST 请求会强制重新生成并覆盖缓存。
func (handler *MiniappAdminHandler) TenantMiniappCode(context *gin.Context) {
	tenantID, err := strconv.ParseUint(strings.TrimSpace(context.Param("tenantId")), 10, 64)
	if err != nil || tenantID == 0 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	image, extension, err := handler.miniapp.TenantMiniappCode(
		context.Request.Context(),
		tenantID,
		context.Request.Method == http.MethodPost,
	)
	if err != nil {
		switch {
		case errors.Is(err, errTenantNotFound):
			httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
			return
		case errors.Is(err, errWechatUnavailable):
			httpapi.WriteError(context, httpapi.ErrorCodeWechatUnavailable)
			return
		}
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	context.Header("Cache-Control", "no-store")
	httpapi.WriteSuccess(context, http.StatusOK, gin.H{"extension": extension, "image": image})
}

// isASCII 判断 AppID 是否仅包含可安全存入 ascii 列的字符。
func isASCII(value string) bool {
	for _, character := range value {
		if character > 127 {
			return false
		}
	}
	return true
}
