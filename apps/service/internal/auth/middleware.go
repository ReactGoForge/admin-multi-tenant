package auth

import (
	"strings"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
)

// Authenticate 校验 Bearer Token 与数据库中的最新员工、租户状态，并写入请求上下文。
func (handler *Handler) Authenticate(context *gin.Context) {
	// 认证步骤一：Authorization 必须是“Bearer <token>”两个部分。
	headerParts := strings.Fields(context.GetHeader("Authorization"))
	if len(headerParts) != 2 || !strings.EqualFold(headerParts[0], "Bearer") || headerParts[1] == "" {
		handler.abortUnauthorized(context)
		return
	}

	// 认证步骤二：JWT 只提供签名保护的身份声明，仍需查询数据库验证账号和会话最新状态。
	result, err := handler.authorizer.Authenticate(context.Request.Context(), headerParts[1])
	if err != nil {
		writeServiceError(context, err)
		return
	}

	// Go 学习提示：Gin Context 是单次请求范围的键值容器。
	// 认证中间件写入可信身份，后续权限中间件和 Handler 再通过相同 key 读取。
	context.Set(employeeContextKey, result.Employee)
	context.Set(tokenIdentityContextKey, result.Identity)
	logging.SetActor(context, logging.Actor{
		Type: "employee", ID: result.Employee.ID, Name: result.Employee.Name,
		Account: result.Employee.LoginAccount, Scope: result.Employee.Scope,
		Workspace: result.Workspace, TenantID: result.TenantID, AuthMode: result.Identity.Mode,
	})
	// 只有全部认证检查通过才调用 Next；前面的错误响应会 Abort，不会进入业务 Handler。
	context.Next()
}

// abortUnauthorized 终止请求并返回统一的登录失效响应。
func (handler *Handler) abortUnauthorized(context *gin.Context) {
	httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
}

// identityMatchesEmployee 检查 Token 中的员工范围和租户归属是否仍与数据库一致。
