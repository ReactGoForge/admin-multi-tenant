package httpapi

import (
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorCode 表示客户端可稳定识别的数字业务错误码。
type ErrorCode int

const (
	SuccessCode                                 = 0
	ErrorCodeInvalidRequest           ErrorCode = 10001
	ErrorCodeCaptchaInvalid           ErrorCode = 10002
	ErrorCodeCaptchaUnavailable       ErrorCode = 10003
	ErrorCodeMethodNotAllowed         ErrorCode = 10004
	ErrorCodeCurrentPasswordInvalid   ErrorCode = 10005
	ErrorCodeUnauthorized             ErrorCode = 20001
	ErrorCodeCredentialsInvalid       ErrorCode = 20002
	ErrorCodeAccountDisabled          ErrorCode = 20003
	ErrorCodeWechatLoginInvalid       ErrorCode = 20004
	ErrorCodeUserDisabled             ErrorCode = 20005
	ErrorCodeTenantUserDisabled       ErrorCode = 20006
	ErrorCodeTenantDisabled           ErrorCode = 20007
	ErrorCodeLoginRateLimited         ErrorCode = 20008
	ErrorCodeForbidden                ErrorCode = 30001
	ErrorCodeRoleNotFound             ErrorCode = 40001
	ErrorCodeResourceNotFound         ErrorCode = 40002
	ErrorCodeConflict                 ErrorCode = 40003
	ErrorCodeProtectedResource        ErrorCode = 40004
	ErrorCodeEndpointNotFound         ErrorCode = 40005
	ErrorCodeInternal                 ErrorCode = 50000
	ErrorCodeWechatUnavailable        ErrorCode = 50001
	ErrorCodeMediaUnavailable         ErrorCode = 50002
	ErrorCodeLoginSecurityUnavailable ErrorCode = 50003
	ErrorCodeServiceNotReady          ErrorCode = 50004
)

// errorDefinition 保存内部业务错误码对应的 HTTP 状态与安全文案。
type errorDefinition struct {
	status  int
	message string
}

var errorDefinitions = map[ErrorCode]errorDefinition{
	ErrorCodeInvalidRequest:           {status: http.StatusBadRequest, message: "请求参数错误"},
	ErrorCodeCaptchaInvalid:           {status: http.StatusBadRequest, message: "验证码错误或已过期"},
	ErrorCodeCaptchaUnavailable:       {status: http.StatusServiceUnavailable, message: "验证码服务暂时不可用"},
	ErrorCodeMethodNotAllowed:         {status: http.StatusMethodNotAllowed, message: "请求方法不支持"},
	ErrorCodeCurrentPasswordInvalid:   {status: http.StatusBadRequest, message: "原密码错误"},
	ErrorCodeUnauthorized:             {status: http.StatusUnauthorized, message: "登录状态已失效，请重新登录"},
	ErrorCodeCredentialsInvalid:       {status: http.StatusUnauthorized, message: "账号或密码错误"},
	ErrorCodeAccountDisabled:          {status: http.StatusForbidden, message: "当前员工账号已被禁用"},
	ErrorCodeWechatLoginInvalid:       {status: http.StatusUnauthorized, message: "微信登录凭证无效，请重新进入小程序"},
	ErrorCodeUserDisabled:             {status: http.StatusForbidden, message: "当前用户已被平台禁用"},
	ErrorCodeTenantUserDisabled:       {status: http.StatusForbidden, message: "当前用户已被租户禁用"},
	ErrorCodeTenantDisabled:           {status: http.StatusForbidden, message: "当前租户已被禁用"},
	ErrorCodeLoginRateLimited:         {status: http.StatusTooManyRequests, message: "登录尝试过于频繁，请稍后重试"},
	ErrorCodeForbidden:                {status: http.StatusForbidden, message: "无权执行此操作"},
	ErrorCodeRoleNotFound:             {status: http.StatusNotFound, message: "角色不存在"},
	ErrorCodeResourceNotFound:         {status: http.StatusNotFound, message: "数据不存在"},
	ErrorCodeConflict:                 {status: http.StatusConflict, message: "数据冲突，请检查关联关系或唯一字段"},
	ErrorCodeProtectedResource:        {status: http.StatusConflict, message: "内置对象或所有者不允许执行此操作"},
	ErrorCodeEndpointNotFound:         {status: http.StatusNotFound, message: "接口不存在"},
	ErrorCodeInternal:                 {status: http.StatusInternalServerError, message: "服务暂时不可用"},
	ErrorCodeWechatUnavailable:        {status: http.StatusServiceUnavailable, message: "微信登录服务暂时不可用"},
	ErrorCodeMediaUnavailable:         {status: http.StatusServiceUnavailable, message: "图片存储服务暂时不可用"},
	ErrorCodeLoginSecurityUnavailable: {status: http.StatusServiceUnavailable, message: "登录安全服务暂时不可用"},
	ErrorCodeServiceNotReady:          {status: http.StatusServiceUnavailable, message: "服务尚未就绪"},
}

// Response 描述所有应用 HTTP 接口统一返回的 JSON 结构。
type Response struct {
	// Go 学习提示：反引号中的 json 标签决定字段序列化后的名称，
	// 所以 Go 字段使用大写开头，客户端仍收到 code、message、data。
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// WriteSuccess 写入包含业务数据的统一成功响应。
func WriteSuccess(context *gin.Context, status int, data any) {
	logging.SetBusinessCode(context, SuccessCode)
	context.JSON(status, Response{
		Code:    SuccessCode,
		Message: "成功",
		Data:    data,
	})
}

// WriteError 根据集中定义的错误码中止请求链并写入统一失败响应。
func WriteError(context *gin.Context, code ErrorCode) {
	definition, exists := errorDefinitions[code]
	if !exists {
		code = ErrorCodeInternal
		definition = errorDefinitions[code]
	}
	logging.SetBusinessCode(context, int(code))
	// Go 学习提示：Abort 会阻止当前请求继续执行后面的 Gin Handler，
	// WithStatusJSON 同时写入 HTTP 状态和 JSON 响应，避免错误后业务逻辑仍继续运行。
	context.AbortWithStatusJSON(definition.status, Response{
		Code:    int(code),
		Message: definition.message,
		Data:    nil,
	})
}

// Recovery 统一恢复未捕获 panic，将异常细节记录在服务端并返回稳定错误响应。
func Recovery() gin.HandlerFunc {
	return func(context *gin.Context) {
		// Go 学习提示：recover 只有在 defer 调用的函数中才能捕获 panic。
		// 无论后续 Handler 正常返回还是发生 panic，这个匿名函数都会在请求结束前执行。
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			logging.WriteEventOutput("error", "HTTP 请求发生未捕获异常", logging.RequestID(context))
			WriteError(context, ErrorCodeInternal)
		}()
		// Next 把控制权交给当前请求链中的下一个 Handler，返回后再继续执行本中间件。
		context.Next()
	}
}
