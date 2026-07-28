package user

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
)

const (
	maximumMiniappRequestBytes = 4096
	miniappSessionContextKey   = "authenticated_miniapp_session"
)

// Handler 组织小程序登录、实时认证和当前用户接口。
type Handler struct {
	miniapp MiniappApplication
}

// NewHandler 使用小程序业务服务创建小程序处理器。
func NewHandler(miniapp MiniappApplication) *Handler {
	return &Handler{miniapp: miniapp}
}

// Login 验证微信 code 和租户场景，在事务内建立用户归属并签发小程序 Token。
func (handler *Handler) Login(context *gin.Context) {
	// Go 学习提示：http.MaxBytesReader 会在读取请求体时设置硬上限，避免异常大的 JSON
	// 长时间占用内存；它返回的新 Reader 需要重新放回 Request.Body。
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maximumMiniappRequestBytes)
	var request loginRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Scene = strings.TrimSpace(request.Scene)
	request.PhoneCode = strings.TrimSpace(request.PhoneCode)
	if request.Code == "" || len(request.Code) > 128 || request.Scene == "" || len(request.Scene) > 20 || len(request.PhoneCode) > 128 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	tenantID, err := strconv.ParseUint(request.Scene, 10, 64)
	if err != nil || tenantID == 0 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	// 安全边界：前端传来的 code 不是可信身份。必须先交给微信服务器换取 OpenID，
	// 再在事务中确认租户和用户归属，最后才能把可信身份写入本系统的 JWT。
	result, err := handler.miniapp.Login(context.Request.Context(), MiniappLoginInput{Code: request.Code, PhoneCode: request.PhoneCode, TenantID: tenantID})
	if err != nil {
		handler.writeError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newLoginResponse(result))
}

// Authenticate 校验小程序 Bearer Token，并实时检查平台用户、租户和租户关系状态。
func (handler *Handler) Authenticate(context *gin.Context) {
	parts := strings.Fields(context.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	// 安全边界：JWT 只证明“这个身份曾经登录成功”，不能代表用户和租户现在仍然启用。
	// 因此每次受保护请求都要查询数据库，实时检查三层状态。
	session, err := handler.miniapp.Authenticate(context.Request.Context(), parts[1])
	if err != nil {
		if errors.Is(err, errUserNotFound) {
			httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
			return
		}
		if errors.Is(err, errUserDisabled) || errors.Is(err, errTenantDisabled) || errors.Is(err, errTenantUserDisabled) {
			handler.writeError(context, err)
			return
		}
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	if session.User.Status != 1 {
		httpapi.WriteError(context, httpapi.ErrorCodeUserDisabled)
		return
	}
	if session.Tenant.Status != 1 {
		httpapi.WriteError(context, httpapi.ErrorCodeTenantDisabled)
		return
	}
	if session.TenantUserStatus != 1 {
		httpapi.WriteError(context, httpapi.ErrorCodeTenantUserDisabled)
		return
	}
	// Go 学习提示：Gin Context 是一次 HTTP 请求内共享数据的容器。中间件用 Set 写入，
	// 后续 Handler 用 Get 读取；这些数据不会跨请求共享。
	context.Set(miniappSessionContextKey, *session)
	name := ""
	if session.User.Nickname != nil {
		name = *session.User.Nickname
	}
	tenantID := session.Tenant.ID
	logging.SetActor(context, logging.Actor{Type: "miniapp_user", ID: session.User.ID, Name: name, Workspace: "miniapp", TenantID: &tenantID})
	// Go 学习提示：Next 会继续执行当前请求后面的中间件和 Handler；若认证失败，
	// 上面的 return 会阻止调用 Next，因此业务 Handler 不会运行。
	context.Next()
}

// Current 返回已经通过实时认证的平台用户和当前租户信息。
func (handler *Handler) Current(context *gin.Context) {
	session, valid := CurrentMiniappSession(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, currentResponse{User: newUserResponse(session.User), Tenant: newTenantResponse(session.Tenant)})
}

// CurrentMiniappSession 从认证后的 Gin 上下文读取可信小程序会话。
func CurrentMiniappSession(context *gin.Context) (Session, bool) {
	value, exists := context.Get(miniappSessionContextKey)
	session, valid := value.(Session)
	return session, exists && valid
}

// writeError 将用户模块内部错误映射为稳定的公开业务错误。
func (handler *Handler) writeError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, errWechatCodeInvalid):
		httpapi.WriteError(context, httpapi.ErrorCodeWechatLoginInvalid)
	case errors.Is(err, errWechatUnavailable):
		httpapi.WriteError(context, httpapi.ErrorCodeWechatUnavailable)
	case errors.Is(err, errTenantNotFound):
		httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
	case errors.Is(err, errTenantDisabled):
		httpapi.WriteError(context, httpapi.ErrorCodeTenantDisabled)
	case errors.Is(err, errUserDisabled):
		httpapi.WriteError(context, httpapi.ErrorCodeUserDisabled)
	case errors.Is(err, errTenantUserDisabled):
		httpapi.WriteError(context, httpapi.ErrorCodeTenantUserDisabled)
	case errors.Is(err, errIdentityConflict):
		httpapi.WriteError(context, httpapi.ErrorCodeConflict)
	default:
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
	}
}

// validSearchText 校验后台用户搜索字段长度。
func validSearchText(value string, maximum int) bool {
	return utf8.RuneCountInString(value) <= maximum
}
