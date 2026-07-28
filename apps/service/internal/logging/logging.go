package logging

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	requestIDContextKey    = "logging_request_id"
	businessCodeContextKey = "logging_business_code"
	actorContextKey        = "logging_actor"
)

// Actor 描述日志需要保存的可信请求身份快照。
type Actor struct {
	Type      string
	ID        uint64
	Name      string
	Account   string
	Scope     string
	Workspace string
	TenantID  *uint64
	AuthMode  string
}

// RequestLog 描述一次 API 请求完成后写入数据库的脱敏元数据。
type RequestLog struct {
	// Go 学习提示：gorm 标签指定结构体字段对应的数据库列；指针字段用 nil 表达 SQL NULL。
	LogType      string          `gorm:"column:log_type"`
	Level        string          `gorm:"column:level"`
	RequestID    string          `gorm:"column:request_id"`
	OccurredAt   time.Time       `gorm:"column:occurred_at"`
	Method       string          `gorm:"column:method"`
	Route        string          `gorm:"column:route"`
	Path         string          `gorm:"column:path"`
	StatusCode   int             `gorm:"column:status_code"`
	BusinessCode int             `gorm:"column:business_code"`
	DurationMS   uint64          `gorm:"column:duration_ms"`
	ClientIP     string          `gorm:"column:client_ip"`
	UserAgent    string          `gorm:"column:user_agent"`
	ActorType    *string         `gorm:"column:actor_type"`
	ActorID      *uint64         `gorm:"column:actor_id"`
	ActorName    *string         `gorm:"column:actor_name"`
	ActorAccount *string         `gorm:"column:actor_account"`
	Workspace    *string         `gorm:"column:workspace"`
	TenantID     *uint64         `gorm:"column:tenant_id"`
	AuthMode     *string         `gorm:"column:auth_mode"`
	Message      string          `gorm:"column:message"`
	Metadata     json.RawMessage `gorm:"column:metadata"`
}

// LoginLog 描述一次后台登录结果的脱敏安全事件。
type LoginLog struct {
	RequestID    string          `gorm:"column:request_id"`
	OccurredAt   time.Time       `gorm:"column:occurred_at"`
	ActorID      *uint64         `gorm:"column:actor_id"`
	ActorName    *string         `gorm:"column:actor_name"`
	ActorAccount string          `gorm:"column:actor_account"`
	Workspace    *string         `gorm:"column:workspace"`
	TenantID     *uint64         `gorm:"column:tenant_id"`
	ClientIP     string          `gorm:"column:client_ip"`
	UserAgent    string          `gorm:"column:user_agent"`
	Level        string          `gorm:"column:level"`
	Message      string          `gorm:"column:message"`
	Metadata     json.RawMessage `gorm:"column:metadata"`
}

// AuditLog 描述一次成功后台写操作的不可变审计记录。
type AuditLog struct {
	RequestID       string          `gorm:"column:request_id"`
	OccurredAt      time.Time       `gorm:"column:occurred_at"`
	ActorEmployeeID uint64          `gorm:"column:actor_employee_id"`
	ActorName       string          `gorm:"column:actor_name"`
	ActorAccount    string          `gorm:"column:actor_account"`
	ActorScope      string          `gorm:"column:actor_scope"`
	Workspace       string          `gorm:"column:workspace"`
	AuthMode        string          `gorm:"column:auth_mode"`
	TenantID        *uint64         `gorm:"column:tenant_id"`
	ModuleCode      string          `gorm:"column:module_code"`
	ActionCode      string          `gorm:"column:action_code"`
	ActionName      string          `gorm:"column:action_name"`
	TargetType      string          `gorm:"column:target_type"`
	TargetID        *string         `gorm:"column:target_id"`
	TargetName      *string         `gorm:"column:target_name"`
	Summary         string          `gorm:"column:summary"`
	Changes         json.RawMessage `gorm:"column:changes"`
	ClientIP        string          `gorm:"column:client_ip"`
	UserAgent       string          `gorm:"column:user_agent"`
}

// AuditDetail 允许业务 Handler 在不暴露请求体的前提下补充目标快照和脱敏变更。
type AuditDetail struct {
	TargetID   string
	TargetName string
	Summary    string
	Changes    any
}

// AuditSnapshot 保存写操作执行前允许进入审计日志的目标名称与非敏感字段。
type AuditSnapshot struct {
	TargetID   string
	TargetName string
	Values     map[string]any
}

// SetActor 将认证完成后的可信身份写入请求上下文。
func SetActor(context *gin.Context, actor Actor) {
	context.Set(actorContextKey, actor)
}

// CurrentActor 从请求上下文读取可信日志身份。
func CurrentActor(context *gin.Context) (Actor, bool) {
	value, exists := context.Get(actorContextKey)
	actor, valid := value.(Actor)
	return actor, exists && valid
}

// SetBusinessCode 保存统一响应业务码，供请求日志在响应完成后读取。
func SetBusinessCode(context *gin.Context, code int) {
	context.Set(businessCodeContextKey, code)
}

// RequestID 返回当前服务端生成的请求标识。
func RequestID(context *gin.Context) string {
	value, _ := context.Get(requestIDContextKey)
	requestID, _ := value.(string)
	return requestID
}

// SetAuditDetail 为当前成功写操作补充经过业务代码确认的脱敏详情。
func SetAuditDetail(context *gin.Context, detail AuditDetail) {
	context.Set("logging_audit_detail", detail)
}

// stringPointer 清理文本并把空字符串转换为 nil，供可空数据库字段使用。
func stringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// formatTargetID 按已注册资源参数名从路由路径提取审计目标 ID。
func formatTargetID(context *gin.Context) *string {
	for _, name := range []string{"itemId", "dictionaryId", "imageId", "categoryId", "userId", "tenantId", "departmentId", "roleId", "menuId", "employeeId"} {
		if value := strings.TrimSpace(context.Param(name)); value != "" {
			return stringPointer(value)
		}
	}
	return nil
}

// truncate 按 Unicode 字符而不是 UTF-8 字节安全截断文本。
func truncate(value string, maximum int) string {
	characters := []rune(value)
	if len(characters) <= maximum {
		return value
	}
	return string(characters[:maximum])
}

// idText 将 BIGINT ID 转为 JSON 安全的十进制字符串。
func idText(value uint64) string {
	return strconv.FormatUint(value, 10)
}
