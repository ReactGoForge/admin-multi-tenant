package logging

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"
)

// serviceStore 定义 Logging Service 需要的最小数据访问能力。
type serviceStore interface {
	CreateEvent(context.Context, string, string, map[string]any) error
	CreateLogin(context.Context, LoginLog) error
	CreateAudit(context.Context, AuditLog) error
	FindAuditSnapshot(context.Context, auditSnapshotQuery) (map[string]any, error)
	CleanupSystemLogs(context.Context, time.Time) (int64, error)
}

// auditSnapshotQuery 描述 Store 可以执行的受控审计快照查询。
type auditSnapshotQuery struct {
	Table    string
	Columns  string
	ID       uint64
	TenantID *uint64
}

// auditSnapshotDefinition 描述一个审计模块允许读取的表、名称列和非敏感字段。
type auditSnapshotDefinition struct {
	Table      string
	NameColumn string
	Columns    string
	TenantSafe bool
}

// Service 编排运行事件、登录日志、操作审计、审计快照和日志保留清理。
type Service struct {
	store serviceStore
}

// NewService 使用日志数据访问能力创建 Logging Service。
func NewService(store serviceStore) *Service {
	return &Service{store: store}
}

// RecordEvent 尽力将运行事件写入数据库，并始终输出结构化标准日志。
func (service *Service) RecordEvent(ctx context.Context, level, message string, metadata map[string]any) {
	if err := service.store.CreateEvent(ctx, level, message, metadata); err != nil {
		log.Printf(`{"type":"event","level":"error","message":"运行事件写入数据库失败"}`)
	}
	WriteEventOutput(level, message, "")
}

// RecordLogin 写入一条不受普通运行事件开关影响的后台登录日志。
func (service *Service) RecordLogin(ctx context.Context, entry LoginLog) error {
	return service.store.CreateLogin(ctx, entry)
}

// RecordAudit 写入一条成功后台写操作的审计日志。
func (service *Service) RecordAudit(ctx context.Context, entry AuditLog) error {
	return service.store.CreateAudit(ctx, entry)
}

// CaptureAuditSnapshot 按已注册后台路由读取目标的非敏感操作前快照。
func (service *Service) CaptureAuditSnapshot(ctx context.Context, route string, params map[string]string, actor Actor) (AuditSnapshot, error) {
	module, _ := auditModule(route)
	id := firstParam(params, "itemId", "dictionaryId", "imageId", "categoryId", "userId", "tenantId", "departmentId", "roleId", "menuId", "employeeId")
	snapshot := AuditSnapshot{TargetID: id, Values: map[string]any{}}
	if id == "" {
		return snapshot, nil
	}
	parsedID, err := strconv.ParseUint(id, 10, 64)
	if err != nil || parsedID == 0 {
		return snapshot, nil
	}
	definition, valid := auditSnapshotFor(module, params)
	if !valid {
		return snapshot, nil
	}
	query := auditSnapshotQuery{
		Table:   definition.Table,
		Columns: definition.Columns,
		ID:      parsedID,
	}
	// 业务约束：租户工作空间读取操作前快照时必须追加可信租户范围，不能跨租户读取目标信息。
	if actor.Workspace == "tenant" && actor.TenantID != nil && definition.TenantSafe {
		query.TenantID = actor.TenantID
	}
	values, err := service.store.FindAuditSnapshot(ctx, query)
	if err != nil {
		return snapshot, err
	}
	if values == nil {
		return snapshot, nil
	}
	snapshot.Values = values
	if value, exists := values[definition.NameColumn]; exists {
		snapshot.TargetName = strings.TrimSpace(stringValue(value))
	}
	return snapshot, nil
}

// CleanupSystemLogs 分批删除截止时间以前的系统日志。
func (service *Service) CleanupSystemLogs(ctx context.Context, cutoff time.Time) (int64, error) {
	return service.store.CleanupSystemLogs(ctx, cutoff)
}

// auditSnapshotFor 返回指定模块允许使用的审计快照白名单。
func auditSnapshotFor(module string, params map[string]string) (auditSnapshotDefinition, bool) {
	switch module {
	case "dictionary":
		if params["itemId"] != "" {
			return auditSnapshotDefinition{Table: "dictionary_items", NameColumn: "label", Columns: "id, label, stable_value, sort, status"}, true
		}
		return auditSnapshotDefinition{Table: "dictionary_types", NameColumn: "name", Columns: "id, name, code, status"}, true
	case "employee":
		return auditSnapshotDefinition{Table: "employees", NameColumn: "name", Columns: "id, name, login_account, department_id, phone, status", TenantSafe: true}, true
	case "role":
		return auditSnapshotDefinition{Table: "roles", NameColumn: "name", Columns: "id, name, description, status", TenantSafe: true}, true
	case "menu":
		return auditSnapshotDefinition{Table: "menus", NameColumn: "name", Columns: "id, name, parent_id, icon, sort, visible, status"}, true
	case "department":
		return auditSnapshotDefinition{Table: "departments", NameColumn: "name", Columns: "id, name, parent_id, leader_employee_id, sort, status", TenantSafe: true}, true
	case "tenant":
		return auditSnapshotDefinition{Table: "tenants", NameColumn: "name", Columns: "id, name, remark, status", TenantSafe: true}, true
	case "image":
		return auditSnapshotDefinition{Table: "image_assets", NameColumn: "name", Columns: "id, name, category_id", TenantSafe: true}, true
	case "image_category":
		return auditSnapshotDefinition{Table: "image_categories", NameColumn: "name", Columns: "id, name", TenantSafe: true}, true
	case "user":
		return auditSnapshotDefinition{Table: "users", NameColumn: "nickname", Columns: "id, nickname, phone, status"}, true
	default:
		return auditSnapshotDefinition{}, false
	}
}

// firstParam 按优先级返回第一个非空路由参数。
func firstParam(params map[string]string, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(params[name]); value != "" {
			return value
		}
	}
	return ""
}

// stringValue 把数据库扫描出的常见字符串表示统一转换为文本。
func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}
