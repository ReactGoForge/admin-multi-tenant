package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
)

const (
	maximumRequestBytes          = 4096
	dummyPasswordHash            = "$2a$10$4KPEUIOfoZ..Dqa8SxZYxezn3geT.xu0fLIWB8TBdZmVZzTo7YxAS"
	employeeContextKey           = "authenticated_employee"
	tokenIdentityContextKey      = "authenticated_token_identity"
	platformSuperAdminContextKey = "authenticated_platform_super_admin"
)

// Handler 组织后台验证码、登录、认证与当前用户接口的处理流程。
type Handler struct {
	service    *Service
	authorizer *AuthorizationService
}

// NewHandlerWithServices 使用已经完成依赖组装的业务服务创建认证接口处理器。
func NewHandlerWithServices(service *Service, authorizer *AuthorizationService) *Handler {
	return &Handler{
		service:    service,
		authorizer: authorizer,
	}
}

// ConfigureLoginSecurity 为登录流程接入限流器和始终持久化的登录日志记录器。
func (handler *Handler) ConfigureLoginSecurity(limiter *LoginLimiter, recorder LoginRecorder) {
	handler.service.ConfigureLoginSecurity(limiter, recorder)
}

// Captcha 返回验证码开关状态，启用时生成并缓存一张一次性数字验证码。
func (handler *Handler) Captcha(context *gin.Context) {
	context.Header("Cache-Control", "no-store")
	response, err := handler.service.Captcha(context.Request.Context())
	if err != nil {
		writeServiceError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// EnterTenant 为已获授权的平台员工签发指定租户的短期代管 Token。
func (handler *Handler) EnterTenant(context *gin.Context) {
	tenantID, err := strconv.ParseUint(strings.TrimSpace(context.Param("tenantId")), 10, 64)
	if err != nil || tenantID == 0 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	employee, employeeValid := CurrentEmployee(context)
	identity, identityValid := CurrentTokenIdentity(context)
	if !employeeValid || !identityValid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	issued, err := handler.service.EnterTenant(context.Request.Context(), employee, identity, tenantID)
	if err != nil {
		writeServiceError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, loginResponse{AccessToken: issued.AccessToken, ExpiresAt: issued.ExpiresAt.Format(timeFormat)})
}

// Login 校验一次性验证码与员工凭证，并签发八小时有效的访问 Token。
func (handler *Handler) Login(context *gin.Context) {
	// 安全边界：先限制请求体大小，再让 Gin 反序列化 JSON，避免异常大请求占用过多内存。
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maximumRequestBytes)
	var request loginRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.CaptchaID = strings.TrimSpace(request.CaptchaID)
	request.CaptchaCode = strings.TrimSpace(request.CaptchaCode)
	if !validLoginRequest(request, handler.service.captcha.Enabled()) {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	issued, err := handler.service.Login(context.Request.Context(), request, LoginAuditMeta{
		RequestID: logging.RequestID(context),
		ClientIP:  context.ClientIP(),
		UserAgent: context.Request.UserAgent(),
	})
	if err != nil {
		writeServiceError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, loginResponse{
		AccessToken: issued.AccessToken,
		ExpiresAt:   issued.ExpiresAt.Format(timeFormat),
	})
}

// writeServiceError 将 Auth 服务错误转换为统一 HTTP 响应。
func writeServiceError(context *gin.Context, err error) {
	serviceErr, ok := err.(ServiceError)
	if !ok {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	if serviceErr.RetryAfter > 0 {
		context.Header("Retry-After", strconv.Itoa(int(serviceErr.RetryAfter/time.Second)))
	}
	httpapi.WriteError(context, serviceHTTPErrorCode(serviceErr.Kind))
}

// serviceHTTPErrorCode 将 Auth 模块错误映射为现有 HTTP 业务错误码。
func serviceHTTPErrorCode(kind ServiceErrorKind) httpapi.ErrorCode {
	switch kind {
	case serviceErrorCaptchaInvalid:
		return httpapi.ErrorCodeCaptchaInvalid
	case serviceErrorCaptchaUnavailable:
		return httpapi.ErrorCodeCaptchaUnavailable
	case serviceErrorCurrentPasswordInvalid:
		return httpapi.ErrorCodeCurrentPasswordInvalid
	case serviceErrorUnauthorized:
		return httpapi.ErrorCodeUnauthorized
	case serviceErrorCredentialsInvalid:
		return httpapi.ErrorCodeCredentialsInvalid
	case serviceErrorAccountDisabled:
		return httpapi.ErrorCodeAccountDisabled
	case serviceErrorTenantDisabled:
		return httpapi.ErrorCodeTenantDisabled
	case serviceErrorLoginRateLimited:
		return httpapi.ErrorCodeLoginRateLimited
	case serviceErrorForbidden:
		return httpapi.ErrorCodeForbidden
	case serviceErrorResourceNotFound:
		return httpapi.ErrorCodeResourceNotFound
	case serviceErrorLoginSecurityUnavailable:
		return httpapi.ErrorCodeLoginSecurityUnavailable
	default:
		return httpapi.ErrorCodeInternal
	}
}
