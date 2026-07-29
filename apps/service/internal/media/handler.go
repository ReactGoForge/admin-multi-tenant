package media

import (
	"errors"
	"fmt"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
)

const (
	maxImageSize    = 5 * 1024 * 1024
	maxAvatarSize   = 5 * 1024 * 1024
	maxUploadBody   = maxImageSize + 1024*1024
	presignedExpiry = 15 * time.Minute
	avatarExpiry    = 8 * time.Hour
)

// Handler 提供平台与租户图片库、分类、品牌设置和公开品牌图片接口。
type Handler struct {
	service *Service
}

// NewHandler 创建媒体接口处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// requireStorage 在媒体接口访问对象存储前统一返回 503。
func (handler *Handler) requireStorage(context *gin.Context) bool {
	if handler.service == nil || !handler.service.StorageReady() {
		httpapi.WriteError(context, httpapi.ErrorCodeMediaUnavailable)
		return false
	}
	return true
}

// readAndValidateImage 读取单张文件并以真实解码结果限制为 PNG、JPEG 或 WebP。

// writeMediaError 将媒体业务错误转换为统一 HTTP 响应。
func writeMediaError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrMediaUnavailable):
		httpapi.WriteError(context, httpapi.ErrorCodeMediaUnavailable)
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
	case errors.Is(err, ErrConflict), errors.Is(err, ErrImageReferenced):
		httpapi.WriteError(context, httpapi.ErrorCodeConflict)
	case errors.Is(err, ErrInvalidOwner):
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
	default:
		logging.WriteEventOutput("error", fmt.Sprintf("媒体接口处理失败: %v", err), logging.RequestID(context))
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
	}
}
