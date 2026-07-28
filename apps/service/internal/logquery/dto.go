package logquery

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 10
	maximumPageSize = 100
	maximumRange    = 31 * 24 * time.Hour
)

type nowFunc func() time.Time

// defaultNow 返回日志查询默认使用的当前时间。
func defaultNow() time.Time { return time.Now() }

// listQuery 保存已经完成类型和边界校验的日志筛选条件。
type listQuery struct {
	Page       int
	PageSize   int
	StartAt    time.Time
	EndAt      time.Time
	LogType    string
	Level      string
	Method     string
	Route      string
	StatusCode *int
	Code       *int
	RequestID  string
	TenantID   *uint64
	Operator   string
	ActorType  string
	ActorID    *uint64
	Workspace  string
	Module     string
	Action     string
	TargetType string
	Target     string
	Account    string
	Result     string
	ClientIP   string
}

// listResponse 描述日志列表统一分页响应。
type listResponse struct {
	Items    any   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

// filterOption 描述前端筛选器使用的通用 ID、名称和状态。
type filterOption struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// operatorOption 描述前端筛选器使用的稳定操作者标识。
type operatorOption struct {
	Key       string  `json:"key"`
	ActorType string  `json:"actorType"`
	ActorID   string  `json:"actorId"`
	Name      string  `json:"name"`
	Account   *string `json:"account"`
}

// codeOption 描述模块或动作筛选器使用的稳定值和展示文案。
type codeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// filterOptionsResponse 汇总日志筛选器需要的租户、操作者、模块和动作选项。
type filterOptionsResponse struct {
	Tenants   []filterOption   `json:"tenants"`
	Operators []operatorOption `json:"operators"`
	Modules   []codeOption     `json:"modules"`
	Actions   []codeOption     `json:"actions"`
}

type objectResponse map[string]any

// parseQuery 解析并限制日志分页、时间范围和筛选参数。
func parseQuery(context *gin.Context, now time.Time, allowTenant bool) (listQuery, bool) {
	page, ok := parsePositive(context.DefaultQuery("page", "1"), 1, 1_000_000)
	if !ok {
		return listQuery{}, false
	}
	pageSize, ok := parsePositive(context.DefaultQuery("pageSize", "10"), defaultPageSize, maximumPageSize)
	if !ok {
		return listQuery{}, false
	}
	current := now.UTC()
	startAt := current.Add(-7 * 24 * time.Hour)
	endAt := current
	if value := strings.TrimSpace(context.Query("startAt")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return listQuery{}, false
		}
		startAt = parsed.UTC()
	}
	if value := strings.TrimSpace(context.Query("endAt")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return listQuery{}, false
		}
		endAt = parsed.UTC()
	}
	if endAt.Before(startAt) || endAt.Sub(startAt) > maximumRange {
		return listQuery{}, false
	}
	query := listQuery{Page: page, PageSize: pageSize, StartAt: startAt, EndAt: endAt, LogType: strings.TrimSpace(context.Query("logType")), Level: strings.TrimSpace(context.Query("level")), Method: strings.ToUpper(strings.TrimSpace(context.Query("method"))), Route: strings.TrimSpace(context.Query("route")), RequestID: strings.TrimSpace(context.Query("requestId")), Operator: strings.TrimSpace(context.Query("operator")), ActorType: strings.TrimSpace(context.Query("actorType")), Workspace: strings.TrimSpace(context.Query("workspace")), Module: strings.TrimSpace(context.Query("module")), Action: strings.TrimSpace(context.Query("action")), TargetType: strings.TrimSpace(context.Query("targetType")), Target: strings.TrimSpace(context.Query("target")), Account: strings.TrimSpace(context.Query("account")), Result: strings.TrimSpace(context.Query("result")), ClientIP: strings.TrimSpace(context.Query("clientIp"))}
	if len(query.Route) > 255 || len(query.RequestID) > 64 || len(query.Operator) > 100 || len(query.Target) > 200 || len(query.Account) > 100 || len(query.ClientIP) > 45 {
		return listQuery{}, false
	}
	if query.Result != "" && query.Result != "success" && query.Result != "failed" && query.Result != "limited" {
		return listQuery{}, false
	}
	if query.Level != "" && query.Level != "info" && query.Level != "warn" && query.Level != "error" {
		return listQuery{}, false
	}
	if query.LogType != "" && query.LogType != "request" && query.LogType != "event" {
		return listQuery{}, false
	}
	if query.Workspace != "" && query.Workspace != "platform" && query.Workspace != "tenant" && query.Workspace != "miniapp" {
		return listQuery{}, false
	}
	if query.ActorType != "" && query.ActorType != "employee" && query.ActorType != "miniapp_user" {
		return listQuery{}, false
	}
	// Go 学习提示：这里的 **int 是“指向可选整数指针的指针”，让循环可以给两个目标字段统一赋值。
	for name, target := range map[string]**int{"statusCode": &query.StatusCode, "businessCode": &query.Code} {
		if value := strings.TrimSpace(context.Query(name)); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return listQuery{}, false
			}
			*target = &parsed
		}
	}
	if allowTenant {
		if value := strings.TrimSpace(context.Query("tenantId")); value != "" {
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil || parsed == 0 {
				return listQuery{}, false
			}
			query.TenantID = &parsed
		}
	}
	if value := strings.TrimSpace(context.Query("actorId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			return listQuery{}, false
		}
		query.ActorID = &parsed
	}
	return query, true
}

// systemResponse 将数据库系统日志行转换为前端字段和字符串 ID。
func systemResponse(row systemLogRow) objectResponse {
	return objectResponse{"id": strconv.FormatUint(row.ID, 10), "logType": row.LogType, "level": row.Level, "requestId": row.RequestID, "occurredAt": row.OccurredAt.Format(time.RFC3339Nano), "method": row.Method, "route": row.Route, "path": row.Path, "statusCode": row.StatusCode, "businessCode": row.BusinessCode, "durationMs": row.DurationMS, "clientIp": row.ClientIP, "userAgent": row.UserAgent, "actorType": row.ActorType, "actorId": formatID(row.ActorID), "actorName": row.ActorName, "actorAccount": row.ActorAccount, "workspace": row.Workspace, "tenantId": formatID(row.TenantID), "tenantName": row.TenantName, "authMode": row.AuthMode, "message": row.Message, "metadata": decodeJSON(row.Metadata)}
}

// auditResponse 将数据库操作审计行转换为前端字段和字符串 ID。
func auditResponse(row auditLogRow) objectResponse {
	return objectResponse{"id": strconv.FormatUint(row.ID, 10), "requestId": row.RequestID, "occurredAt": row.OccurredAt.Format(time.RFC3339Nano), "actorEmployeeId": strconv.FormatUint(row.ActorEmployeeID, 10), "actorName": row.ActorName, "actorAccount": row.ActorAccount, "actorScope": row.ActorScope, "workspace": row.Workspace, "authMode": row.AuthMode, "tenantId": formatID(row.TenantID), "tenantName": row.TenantName, "moduleCode": row.ModuleCode, "actionCode": row.ActionCode, "actionName": row.ActionName, "targetType": row.TargetType, "targetId": row.TargetID, "targetName": row.TargetName, "summary": row.Summary, "changes": decodeJSON(row.Changes), "clientIp": row.ClientIP, "userAgent": row.UserAgent}
}

// loginResponse 将系统日志中的登录事件转换为不暴露内部元数据结构的响应。
func loginResponse(row systemLogRow) objectResponse {
	metadata, _ := decodeJSON(row.Metadata).(map[string]any)
	result, _ := metadata["result"].(string)
	reason, _ := metadata["reason"].(string)
	return objectResponse{"id": strconv.FormatUint(row.ID, 10), "requestId": row.RequestID, "occurredAt": row.OccurredAt.Format(time.RFC3339Nano), "result": result, "reason": reason, "employeeId": formatID(row.ActorID), "employeeName": row.ActorName, "account": row.ActorAccount, "workspace": row.Workspace, "tenantId": formatID(row.TenantID), "tenantName": row.TenantName, "clientIp": row.ClientIP, "userAgent": row.UserAgent}
}

// decodeJSON 把可空 JSON 数据库字段安全还原为响应对象。
func decodeJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return nil
	}
	return decoded
}

// formatID 将可空 BIGINT ID 转换为可空字符串 ID。
func formatID(value *uint64) *string {
	if value == nil {
		return nil
	}
	formatted := strconv.FormatUint(*value, 10)
	return &formatted
}

// parsePositive 解析指定上限内的正整数查询参数。
func parsePositive(value string, fallback, maximum int) (int, bool) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 || parsed > maximum {
		return fallback, false
	}
	return parsed, true
}
