package auth

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
)

var profilePhonePattern = regexp.MustCompile(`^(\+86|0086)?1[0-9]{10}$`)

// UpdateBasicProfile 校验并更新当前认证员工本人的手机号。
func (handler *Handler) UpdateBasicProfile(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maximumRequestBytes)
	var request updateBasicProfileRequest
	if err := context.ShouldBindJSON(&request); err != nil || !normalizeBasicProfile(&request) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	employee, valid := CurrentEmployee(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	if err := handler.service.UpdateBasicProfile(context.Request.Context(), employee, request); err != nil {
		writeServiceError(context, err)
		return
	}
	logging.SetAuditDetail(context, logging.AuditDetail{
		TargetID:   idText(employee.ID),
		TargetName: employee.Name,
		Summary:    "基本资料已修改",
		Changes: map[string]any{
			"phone": map[string]any{"before": optionalStringValue(employee.Phone), "after": optionalStringValue(request.Phone)},
		},
	})
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// normalizeBasicProfile 清理个人资料文本并校验可选手机号。
func normalizeBasicProfile(request *updateBasicProfileRequest) bool {
	if request.Phone != nil {
		value := strings.TrimSpace(*request.Phone)
		if value == "" {
			request.Phone = nil
		} else {
			request.Phone = &value
		}
	}
	return request.Phone == nil || profilePhonePattern.MatchString(*request.Phone)
}

// optionalStringValue 把可空文本转换为适合审计 JSON 的字符串或 null。
func optionalStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// ChangePassword 校验当前员工原密码后写入新哈希，并使全部旧后台会话失效。
func (handler *Handler) ChangePassword(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maximumRequestBytes)
	var request changePasswordRequest
	if err := context.ShouldBindJSON(&request); err != nil || !validProfilePassword(request.NewPassword) || request.CurrentPassword == "" || utf8.RuneCountInString(request.CurrentPassword) > 72 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	employee, valid := CurrentEmployee(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	if err := handler.service.ChangePassword(context.Request.Context(), employee, request); err != nil {
		writeServiceError(context, err)
		return
	}
	logging.SetAuditDetail(context, logging.AuditDetail{TargetID: idText(employee.ID), TargetName: employee.Name, Summary: "密码已修改", Changes: map[string]any{"password": map[string]any{"changed": true}}})
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// validProfilePassword 校验个人新密码仍遵循六至十八个 Unicode 字符的规则。
func validProfilePassword(password string) bool {
	length := utf8.RuneCountInString(password)
	return length >= 6 && length <= 18
}

// idText 将员工 BIGINT ID 转为前端安全的十进制字符串。
func idText(value uint64) string {
	return strconv.FormatUint(value, 10)
}
