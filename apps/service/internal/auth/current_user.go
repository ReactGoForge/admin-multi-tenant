package auth

import (
	stdcontext "context"
	"net/http"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const timeFormat = "2006-01-02T15:04:05Z07:00"

// AvatarURLProvider 定义当前用户接口为私有员工头像签发临时访问地址的最小能力。
type AvatarURLProvider interface {
	AvatarURL(stdcontext.Context, uint64) (string, error)
}

// ConfigureAvatarURLs 为当前用户接口接入私有头像临时地址签发能力。
func (handler *Handler) ConfigureAvatarURLs(provider AvatarURLProvider) {
	handler.service.ConfigureAvatarURLs(provider)
}

// CurrentEmployee 从已通过认证的 Gin 上下文读取当前员工。
func CurrentEmployee(context *gin.Context) (Employee, bool) {
	// Go 学习提示：类型断言 value.(Employee) 把 any 还原为 Employee；第二个 bool 避免断言失败时 panic。
	value, exists := context.Get(employeeContextKey)
	employee, valid := value.(Employee)
	return employee, exists && valid
}

// CurrentTokenIdentity 从认证上下文读取当前 Token 的可信身份与代管范围。
func CurrentTokenIdentity(context *gin.Context) (TokenIdentity, bool) {
	value, exists := context.Get(tokenIdentityContextKey)
	identity, valid := value.(TokenIdentity)
	return identity, exists && valid
}

// CurrentTenantID 返回普通租户或平台代管会话的可信租户 ID。
func CurrentTenantID(context *gin.Context) (uint64, bool) {
	employee, employeeValid := CurrentEmployee(context)
	identity, identityValid := CurrentTokenIdentity(context)
	if !employeeValid || !identityValid {
		return 0, false
	}
	if identity.Mode == "managed" && employee.Scope == "platform" && identity.TenantID != nil {
		return *identity.TenantID, true
	}
	if identity.Mode == "normal" && employee.Scope == "tenant" && employee.TenantID != nil {
		return *employee.TenantID, true
	}
	return 0, false
}

// CurrentUser 从数据库读取最新角色与权限，并返回当前后台员工身份。
func (handler *Handler) CurrentUser(context *gin.Context) {
	employee, valid := CurrentEmployee(context)
	if !valid {
		handler.abortUnauthorized(context)
		return
	}
	identity, identityValid := CurrentTokenIdentity(context)
	if !identityValid {
		handler.abortUnauthorized(context)
		return
	}
	response, err := handler.service.CurrentUser(context.Request.Context(), employee, identity)
	if err != nil {
		writeServiceError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// firstCharacter 返回员工姓名的首个 UTF-8 字符作为头像文字。
func firstCharacter(value string) string {
	character, _ := utf8.DecodeRuneInString(value)
	if character == utf8.RuneError || character == 0 {
		return "云"
	}
	return string(character)
}
