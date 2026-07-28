package logging

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestRecorder 定义纯 HTTP 请求日志中间件需要的最小写入能力。
type RequestRecorder interface {
	CreateRequest(context.Context, RequestLog) error
}

// AuditRecorder 定义后台操作审计中间件需要的业务日志能力。
type AuditRecorder interface {
	CaptureAuditSnapshot(context.Context, string, map[string]string, Actor) (AuditSnapshot, error)
	RecordAudit(context.Context, AuditLog) error
}

// Middleware 为 API 请求生成请求 ID，并按配置尽力写入系统请求日志。
func Middleware(recorder RequestRecorder, mode RequestLogMode) gin.HandlerFunc {
	return func(context *gin.Context) {
		// 中间件在 Next 前记录开始时间，在 Next 返回后读取最终路由、状态码和业务码。
		startedAt := time.Now()
		requestID := newRequestID()
		context.Set(requestIDContextKey, requestID)
		context.Header("X-Request-ID", requestID)
		context.Next()

		route := context.FullPath()
		persist := shouldRecordRequest(mode, context.Request.Method, context.Request.URL.Path, route, context.Writer.Status())
		output := persist || (mode == RequestLogModeOff && shouldRecordRequest(RequestLogModeMutationAndError, context.Request.Method, context.Request.URL.Path, route, context.Writer.Status()))
		if persist || output {
			entry := newRequestLog(context, requestID, route, startedAt)
			if persist {
				if err := recorder.CreateRequest(context.Request.Context(), entry); err != nil {
					writeFallback("error", "系统请求日志写入失败", requestID)
				}
			}
			if output {
				writeRequestOutput(entry)
			}
		}
	}
}

// AuditMiddleware 在后台认证完成后读取脱敏变更，并记录成功写操作。
func AuditMiddleware(recorder AuditRecorder) gin.HandlerFunc {
	return func(context *gin.Context) {
		// 安全边界：请求体只在进入业务 Handler 前读取一次，并立即恢复 Body，
		// 这样审计可以提取脱敏字段，又不会阻止 ShouldBindJSON 后续读取。
		sanitizedChanges := readSanitizedChanges(context)
		var snapshot AuditSnapshot
		if isMutation(context.Request.Method) {
			params := make(map[string]string, len(context.Params))
			for _, param := range context.Params {
				params[param.Key] = param.Value
			}
			actor, _ := CurrentActor(context)
			captured, err := recorder.CaptureAuditSnapshot(context.Request.Context(), context.FullPath(), params, actor)
			if err != nil {
				writeFallback("warn", "操作前审计快照读取失败", RequestID(context))
			} else {
				snapshot = captured
			}
		}
		context.Next()
		if entry, ok := newAuditLog(context, RequestID(context), mergeAuditChanges(snapshot, sanitizedChanges)); ok {
			if entry.TargetID == nil && snapshot.TargetID != "" {
				entry.TargetID = stringPointer(snapshot.TargetID)
			}
			if entry.TargetName == nil && snapshot.TargetName != "" {
				entry.TargetName = stringPointer(snapshot.TargetName)
			}
			if err := recorder.RecordAudit(context.Request.Context(), entry); err != nil {
				writeFallback("error", "操作审计日志写入失败", RequestID(context))
			}
		}
	}
}

// mergeAuditChanges 把操作前快照与请求中的脱敏新值合并为审计变更。
func mergeAuditChanges(snapshot AuditSnapshot, current any) any {
	currentFields, _ := current.(map[string]any)
	if len(currentFields) == 0 {
		if len(snapshot.Values) == 0 {
			return nil
		}
		return map[string]any{"deleted": map[string]any{"before": snapshot.Values}}
	}
	for key, value := range currentFields {
		field, _ := value.(map[string]any)
		if _, sensitive := field["changed"]; sensitive {
			continue
		}
		if before, exists := snapshot.Values[camelToSnake(key)]; exists {
			field["before"] = before
		}
	}
	return currentFields
}

// camelToSnake 将 JSON 常用的驼峰字段名转换为数据库快照使用的 snake_case。
func camelToSnake(value string) string {
	var builder strings.Builder
	for index, character := range value {
		if character >= 'A' && character <= 'Z' {
			if index > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(character + ('a' - 'A'))
			continue
		}
		builder.WriteRune(character)
	}
	return builder.String()
}

// WriteEventOutput 将运行事件以结构化 JSON 输出到标准输出。
func WriteEventOutput(level, message, requestID string) {
	writeFallback(level, message, requestID)
}

// newRequestID 生成不可预测的请求标识；系统随机源异常时使用时间文本兜底。
func newRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(value)
}

// shouldRecordRequest 判断当前请求是否应按采集模式写入系统日志表。
func shouldRecordRequest(mode RequestLogMode, method, path, route string, status int) bool {
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	if route == "/api/public/images/:imageId" {
		return false
	}
	if mode == RequestLogModeOff {
		return false
	}
	if mode == RequestLogModeAll {
		return true
	}
	if (method == http.MethodGet || method == http.MethodHead) && status >= 200 && status < 400 {
		return false
	}
	return true
}

// newRequestLog 从完成后的 Gin 请求构造不含请求体和凭证的系统日志。
func newRequestLog(context *gin.Context, requestID, route string, startedAt time.Time) RequestLog {
	status := context.Writer.Status()
	level := "info"
	if status >= http.StatusInternalServerError {
		level = "error"
	} else if status >= http.StatusBadRequest {
		level = "warn"
	}
	businessCode, _ := context.Get(businessCodeContextKey)
	code, _ := businessCode.(int)
	entry := RequestLog{
		LogType: "request", Level: level, RequestID: requestID, OccurredAt: startedAt.UTC(),
		Method: context.Request.Method, Route: route, Path: truncate(context.Request.URL.Path, 255),
		StatusCode: status, BusinessCode: code, DurationMS: uint64(time.Since(startedAt).Milliseconds()),
		ClientIP: truncate(context.ClientIP(), 45), UserAgent: truncate(context.Request.UserAgent(), 512),
		Message: http.StatusText(status),
	}
	if actor, ok := CurrentActor(context); ok {
		entry.ActorType = stringPointer(actor.Type)
		entry.ActorID = &actor.ID
		entry.ActorName = stringPointer(truncate(actor.Name, 100))
		entry.ActorAccount = stringPointer(truncate(actor.Account, 100))
		entry.Workspace = stringPointer(actor.Workspace)
		entry.TenantID = actor.TenantID
		entry.AuthMode = stringPointer(actor.AuthMode)
	}
	return entry
}

// newAuditLog 为成功的后台写操作构造审计日志；不符合审计条件时返回 false。
func newAuditLog(context *gin.Context, requestID string, sanitizedChanges any) (AuditLog, bool) {
	if context.Writer.Status() < 200 || context.Writer.Status() >= 300 || !isMutation(context.Request.Method) {
		return AuditLog{}, false
	}
	actor, ok := CurrentActor(context)
	if !ok || actor.Type != "employee" {
		return AuditLog{}, false
	}
	moduleCode, moduleName := auditModule(context.FullPath())
	if moduleCode == "" {
		return AuditLog{}, false
	}
	actionCode, actionName := auditAction(context.Request.Method, context.FullPath())
	targetID := formatTargetID(context)
	targetName := targetNameFromChanges(sanitizedChanges)
	summary := actionName + moduleName
	var changes json.RawMessage
	if sanitizedChanges != nil {
		changes, _ = json.Marshal(sanitizedChanges)
	}
	if value, exists := context.Get("logging_audit_detail"); exists {
		if detail, valid := value.(AuditDetail); valid {
			if detail.TargetID != "" {
				targetID = stringPointer(detail.TargetID)
			}
			targetName = stringPointer(truncate(detail.TargetName, 200))
			if strings.TrimSpace(detail.Summary) != "" {
				summary = truncate(detail.Summary, 500)
			}
			if detail.Changes != nil {
				changes, _ = json.Marshal(detail.Changes)
			}
		}
	}
	// 安全边界：密码接口无论业务层提供什么详情，都只记录“已修改”，绝不保存密码值。
	if strings.Contains(context.FullPath(), "/password") {
		changes = json.RawMessage(`{"password":{"changed":true}}`)
	}
	return AuditLog{
		RequestID: requestID, OccurredAt: time.Now().UTC(), ActorEmployeeID: actor.ID,
		ActorName: truncate(actor.Name, 30), ActorAccount: truncate(actor.Account, 40),
		ActorScope: actor.Scope, Workspace: actor.Workspace, AuthMode: actor.AuthMode,
		TenantID: actor.TenantID, ModuleCode: moduleCode, ActionCode: actionCode,
		ActionName: actionName, TargetType: moduleCode, TargetID: targetID, TargetName: targetName,
		Summary: truncate(summary, 500), Changes: changes, ClientIP: truncate(context.ClientIP(), 45),
		UserAgent: truncate(context.Request.UserAgent(), 512),
	}, true
}

// targetNameFromChanges 从常见展示字段中提取审计目标名称。
func targetNameFromChanges(changes any) *string {
	fields, _ := changes.(map[string]any)
	for _, key := range []string{"name", "label", "loginAccount", "nickname", "appId"} {
		field, valid := fields[key].(map[string]any)
		if !valid {
			continue
		}
		value, valid := field["after"].(string)
		if valid && strings.TrimSpace(value) != "" {
			return stringPointer(truncate(value, 200))
		}
	}
	return nil
}

// readSanitizedChanges 读取有限大小的 JSON 请求体，并把敏感字段替换成布尔修改标记。
func readSanitizedChanges(context *gin.Context) any {
	if !isMutation(context.Request.Method) || !strings.HasPrefix(context.GetHeader("Content-Type"), "application/json") {
		return nil
	}
	const maximumAuditBodyBytes = 64 * 1024
	// 安全边界：LimitReader 多允许一个字节，用来判断请求是否超过上限，而不是无界读取到内存。
	body, err := io.ReadAll(io.LimitReader(context.Request.Body, maximumAuditBodyBytes+1))
	if err != nil {
		return nil
	}
	context.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 || len(body) > maximumAuditBodyBytes {
		return nil
	}
	var value map[string]any
	if json.Unmarshal(body, &value) != nil {
		return nil
	}
	changes := make(map[string]any, len(value))
	for key, fieldValue := range value {
		if isSensitiveField(key) {
			changed := fieldValue != nil
			if text, valid := fieldValue.(string); valid {
				changed = text != ""
			}
			changes[key] = map[string]any{"changed": changed}
			continue
		}
		changes[key] = map[string]any{"after": fieldValue}
	}
	return changes
}

// isSensitiveField 判断字段名是否可能包含密码、密钥、Token 或对象存储内部信息。
func isSensitiveField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	for _, fragment := range []string{"password", "secret", "token", "captcha", "authorization", "sessionkey", "objectkey"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

// isMutation 判断 HTTP 方法是否可能改变服务端数据。
func isMutation(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

// auditModuleDefinition 描述稳定模块编码与路由片段、中文名称的对应关系。
type auditModuleDefinition struct {
	segment string
	code    string
	name    string
}

var auditModuleDefinitions = []auditModuleDefinition{
	{"/dictionaries", "dictionary", "字典"}, {"/employees", "employee", "员工"},
	{"/roles", "role", "角色"}, {"/menus", "menu", "菜单"}, {"/departments", "department", "部门"},
	{"/tenants", "tenant", "租户"}, {"/settings/miniapp", "miniapp_settings", "小程序配置"},
	{"/settings/basic", "basic_settings", "基础设置"}, {"/image-categories", "image_category", "图片分类"},
	{"/images", "image", "图片"}, {"/users", "user", "用户"},
	{"/profile", "profile", "个人信息"},
}

// auditModule 返回写操作对应的稳定模块编码和展示名称，未知模块也不会跳过审计。
func auditModule(route string) (string, string) {
	for _, value := range auditModuleDefinitions {
		if strings.Contains(route, value.segment) {
			return value.code, value.name
		}
	}
	code := inferAuditModuleCode(route)
	if code == "" {
		return "unknown", "未知模块"
	}
	return code, code
}

// AuditModuleLabel 返回稳定模块编码对应的中文名称，未知编码回退为原编码。
func AuditModuleLabel(code string) string {
	for _, value := range auditModuleDefinitions {
		if value.code == code {
			return value.name
		}
	}
	if strings.TrimSpace(code) == "" || code == "unknown" {
		return "未知模块"
	}
	return code
}

// inferAuditModuleCode 从后台平台或租户路由中推导未登记模块的稳定编码。
func inferAuditModuleCode(route string) string {
	trimmed := strings.Trim(strings.TrimPrefix(route, "/api/admin/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || (parts[0] != "platform" && parts[0] != "tenant") {
		return ""
	}
	segments := []string{parts[1]}
	if parts[1] == "settings" && len(parts) > 2 {
		segments = append(segments, parts[2])
	}
	return normalizeAuditCode(strings.Join(segments, "_"))
}

// normalizeAuditCode 将路由片段规范为只包含小写字母、数字和下划线的审计编码。
func normalizeAuditCode(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(value) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9', character == '_':
			builder.WriteRune(character)
		case character == '-':
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

// auditAction 根据 HTTP 方法和特殊路径识别审计动作。
func auditAction(method, route string) (string, string) {
	switch {
	case strings.HasSuffix(route, "/enter"):
		return "enter", "进入"
	case strings.HasSuffix(route, "/profile/password"):
		return "change_password", "修改密码"
	case strings.HasSuffix(route, "/profile/avatar"):
		return "update_avatar", "修改头像"
	case strings.Contains(route, "/password"):
		return "reset_password", "重置密码"
	case strings.HasSuffix(route, "/roles"):
		return "assign_roles", "分配角色"
	case strings.HasSuffix(route, "/permissions"):
		return "assign_permissions", "配置权限"
	case strings.HasSuffix(route, "/status"):
		return "change_status", "修改状态"
	case method == http.MethodPost:
		return "create", "新增"
	case method == http.MethodDelete:
		return "delete", "删除"
	default:
		return "update", "编辑"
	}
}

// writeRequestOutput 将请求日志的最小字段同步输出为结构化标准日志。
func writeRequestOutput(entry RequestLog) {
	payload := map[string]any{"type": "request", "level": entry.Level, "requestId": entry.RequestID, "method": entry.Method, "route": entry.Route, "status": entry.StatusCode, "businessCode": entry.BusinessCode, "durationMs": entry.DurationMS}
	encoded, _ := json.Marshal(payload)
	log.Print(string(encoded))
}

// writeFallback 在数据库日志不可用时仍把运行事件输出到标准输出。
func writeFallback(level, message, requestID string) {
	payload := map[string]any{"type": "event", "level": level, "message": message}
	if requestID != "" {
		payload["requestId"] = requestID
	}
	encoded, _ := json.Marshal(payload)
	log.Print(string(encoded))
}
