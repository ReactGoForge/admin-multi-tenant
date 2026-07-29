package media

import (
	"net/http"
	"strconv"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/user"

	"github.com/gin-gonic/gin"
)

// UploadCurrentEmployeeAvatar 校验裁剪头像后替换当前认证员工的私有头像。
func (handler *Handler) UploadCurrentEmployeeAvatar(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxAvatarSize+1024*1024)
	if !handler.requireStorage(context) {
		return
	}
	employee, valid := auth.CurrentEmployee(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	file, err := context.FormFile("file")
	if err != nil || file.Size <= 0 || file.Size > maxAvatarSize {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	data, mimeType, extension, err := readAndValidateAvatar(file)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	result, err := handler.service.UploadCurrentEmployeeAvatar(context.Request.Context(), AvatarUpload{EmployeeID: employee.ID, EmployeeName: employee.Name, Scope: employee.Scope, TenantID: employee.TenantID, OriginalName: file.Filename, Data: data, MIMEType: mimeType, Extension: extension})
	if err != nil {
		writeMediaError(context, err)
		return
	}
	logging.SetAuditDetail(context, logging.AuditDetail{TargetID: strconv.FormatUint(result.EmployeeID, 10), TargetName: result.EmployeeName, Summary: "头像已修改", Changes: map[string]any{"avatar": map[string]any{"changed": true}}})
	httpapi.WriteSuccess(context, http.StatusCreated, avatarResponse{AvatarURL: result.AvatarURL})
}

// UploadCurrentMiniappUserAvatar 校验并替换当前认证小程序用户的私有头像。
func (handler *Handler) UploadCurrentMiniappUserAvatar(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxAvatarSize+1024*1024)
	if !handler.requireStorage(context) {
		return
	}
	session, valid := user.CurrentMiniappSession(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	file, err := context.FormFile("file")
	if err != nil || file.Size <= 0 || file.Size > maxAvatarSize {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	data, mimeType, extension, err := readAndValidateAvatar(file)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	avatarURL, err := handler.service.UploadMiniappUserAvatar(context.Request.Context(), MiniappUserAvatarUpload{
		UserID: session.User.ID, OriginalName: file.Filename, Data: data, MIMEType: mimeType, Extension: extension,
	})
	if err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, avatarResponse{AvatarURL: avatarURL})
}
