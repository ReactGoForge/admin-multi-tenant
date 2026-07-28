package logquery

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const loginRoute = "/api/admin/auth/login"

// Store 使用 GORM 只读访问系统日志、登录日志和操作审计日志。
type Store struct {
	db *gorm.DB
}

// systemLogRow 映射系统日志及其关联租户名称的数据库查询结果。
type systemLogRow struct {
	ID           uint64          `gorm:"column:id"`
	LogType      string          `gorm:"column:log_type"`
	Level        string          `gorm:"column:level"`
	RequestID    *string         `gorm:"column:request_id"`
	OccurredAt   time.Time       `gorm:"column:occurred_at"`
	Method       *string         `gorm:"column:method"`
	Route        *string         `gorm:"column:route"`
	Path         *string         `gorm:"column:path"`
	StatusCode   *int            `gorm:"column:status_code"`
	BusinessCode *int            `gorm:"column:business_code"`
	DurationMS   *uint64         `gorm:"column:duration_ms"`
	ClientIP     *string         `gorm:"column:client_ip"`
	UserAgent    *string         `gorm:"column:user_agent"`
	ActorType    *string         `gorm:"column:actor_type"`
	ActorID      *uint64         `gorm:"column:actor_id"`
	ActorName    *string         `gorm:"column:actor_name"`
	ActorAccount *string         `gorm:"column:actor_account"`
	Workspace    *string         `gorm:"column:workspace"`
	TenantID     *uint64         `gorm:"column:tenant_id"`
	TenantName   *string         `gorm:"column:tenant_name"`
	AuthMode     *string         `gorm:"column:auth_mode"`
	Message      *string         `gorm:"column:message"`
	Metadata     json.RawMessage `gorm:"column:metadata"`
}

// auditLogRow 映射操作审计及其关联租户名称的数据库查询结果。
type auditLogRow struct {
	ID              uint64          `gorm:"column:id"`
	RequestID       string          `gorm:"column:request_id"`
	OccurredAt      time.Time       `gorm:"column:occurred_at"`
	ActorEmployeeID uint64          `gorm:"column:actor_employee_id"`
	ActorName       string          `gorm:"column:actor_name"`
	ActorAccount    string          `gorm:"column:actor_account"`
	ActorScope      string          `gorm:"column:actor_scope"`
	Workspace       string          `gorm:"column:workspace"`
	AuthMode        string          `gorm:"column:auth_mode"`
	TenantID        *uint64         `gorm:"column:tenant_id"`
	TenantName      *string         `gorm:"column:tenant_name"`
	ModuleCode      string          `gorm:"column:module_code"`
	ActionCode      string          `gorm:"column:action_code"`
	ActionName      string          `gorm:"column:action_name"`
	TargetType      string          `gorm:"column:target_type"`
	TargetID        *string         `gorm:"column:target_id"`
	TargetName      *string         `gorm:"column:target_name"`
	Summary         string          `gorm:"column:summary"`
	Changes         json.RawMessage `gorm:"column:changes"`
	ClientIP        *string         `gorm:"column:client_ip"`
	UserAgent       *string         `gorm:"column:user_agent"`
}

// tenantOptionRow 映射日志筛选器所需的租户选项。
type tenantOptionRow struct {
	ID     uint64 `gorm:"column:id"`
	Name   string `gorm:"column:name"`
	Status int    `gorm:"column:status"`
}

// operatorOptionRow 映射历史日志中的操作者最新快照。
type operatorOptionRow struct {
	ActorType string  `gorm:"column:actor_type"`
	ActorID   uint64  `gorm:"column:actor_id"`
	Name      *string `gorm:"column:actor_name"`
	Account   *string `gorm:"column:actor_account"`
}

// auditModuleOptionRow 映射历史审计日志中的模块编码。
type auditModuleOptionRow struct {
	Value string `gorm:"column:value"`
}

// auditActionOptionRow 映射历史审计日志中的动作编码和名称。
type auditActionOptionRow struct {
	Value string `gorm:"column:value"`
	Label string `gorm:"column:label"`
}

// NewStore 使用现有数据库连接创建日志查询存储。
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// ListSystemLogs 查询平台系统日志列表并返回分页总数。
func (store *Store) ListSystemLogs(ctx context.Context, query listQuery) ([]systemLogRow, int64, error) {
	databaseQuery := store.db.WithContext(ctx).Table("system_logs AS sl").
		Joins("LEFT JOIN tenants ON tenants.id = sl.tenant_id").
		Where("sl.occurred_at >= ? AND sl.occurred_at <= ?", query.StartAt, query.EndAt)
	databaseQuery = applySystemFilters(databaseQuery, query)
	return scanSystemLogs(databaseQuery, query)
}

// ListAuditLogs 查询平台或认证租户范围内的操作审计日志列表并返回分页总数。
func (store *Store) ListAuditLogs(ctx context.Context, query listQuery, forcedTenantID *uint64) ([]auditLogRow, int64, error) {
	databaseQuery := store.db.WithContext(ctx).Table("operation_audit_logs AS al").
		Joins("LEFT JOIN tenants ON tenants.id = al.tenant_id").
		Where("al.occurred_at >= ? AND al.occurred_at <= ?", query.StartAt, query.EndAt)
	if forcedTenantID != nil {
		databaseQuery = databaseQuery.Where("al.tenant_id = ?", *forcedTenantID)
	} else if query.TenantID != nil {
		databaseQuery = databaseQuery.Where("al.tenant_id = ?", *query.TenantID)
	}
	databaseQuery = applyAuditFilters(databaseQuery, query)
	return scanAuditLogs(databaseQuery, query)
}

// ListLoginLogs 查询平台或认证租户范围内的后台登录日志列表并返回分页总数。
func (store *Store) ListLoginLogs(ctx context.Context, query listQuery, forcedTenantID *uint64) ([]systemLogRow, int64, error) {
	databaseQuery := store.db.WithContext(ctx).Table("system_logs AS sl").
		Joins("LEFT JOIN tenants ON tenants.id = sl.tenant_id").
		Where("sl.log_type = 'event' AND sl.route = ?", loginRoute).
		Where("sl.occurred_at >= ? AND sl.occurred_at <= ?", query.StartAt, query.EndAt)
	if forcedTenantID != nil {
		databaseQuery = databaseQuery.Where("sl.tenant_id = ?", *forcedTenantID)
	} else if query.TenantID != nil {
		databaseQuery = databaseQuery.Where("sl.tenant_id = ?", *query.TenantID)
	}
	if query.Account != "" {
		databaseQuery = databaseQuery.Where("sl.actor_account LIKE ?", "%"+query.Account+"%")
	}
	if query.ClientIP != "" {
		databaseQuery = databaseQuery.Where("sl.client_ip = ?", query.ClientIP)
	}
	if query.Result != "" {
		databaseQuery = databaseQuery.Where("JSON_UNQUOTE(JSON_EXTRACT(sl.metadata, '$.result')) = ?", query.Result)
	}
	return scanSystemLogs(databaseQuery, query)
}

// ListTenants 返回日志筛选器需要的全部租户基础选项。
func (store *Store) ListTenants(ctx context.Context) ([]tenantOptionRow, error) {
	var tenants []tenantOptionRow
	if err := store.db.WithContext(ctx).Table("tenants").Select("id", "name", "status").Order("name ASC, id ASC").Scan(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

// ListLatestOperators 按稳定操作者标识去重，并返回每个操作者的最新日志快照。
func (store *Store) ListLatestOperators(ctx context.Context, kind string, forcedTenantID *uint64) ([]operatorOptionRow, error) {
	var rows []operatorOptionRow
	query, arguments := latestOperatorQuery(kind, forcedTenantID)
	if err := store.db.WithContext(ctx).Raw(query, arguments...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAuditCodeOptions 按日志可见范围返回历史模块和动作的去重筛选选项。
func (store *Store) ListAuditCodeOptions(ctx context.Context, forcedTenantID *uint64) ([]auditModuleOptionRow, []auditActionOptionRow, error) {
	moduleQuery := store.db.WithContext(ctx).Table("operation_audit_logs")
	actionQuery := store.db.WithContext(ctx).Table("operation_audit_logs")
	if forcedTenantID != nil {
		moduleQuery = moduleQuery.Where("tenant_id = ?", *forcedTenantID)
		actionQuery = actionQuery.Where("tenant_id = ?", *forcedTenantID)
	}

	var moduleRows []auditModuleOptionRow
	if err := moduleQuery.Select("DISTINCT module_code AS value").Order("module_code ASC").Scan(&moduleRows).Error; err != nil {
		return nil, nil, err
	}
	var actionRows []auditActionOptionRow
	if err := actionQuery.Select("action_code AS value, MAX(action_name) AS label").Group("action_code").Order("action_code ASC").Scan(&actionRows).Error; err != nil {
		return nil, nil, err
	}
	return moduleRows, actionRows, nil
}

// scanSystemLogs 统计并读取系统日志分页行。
func scanSystemLogs(databaseQuery *gorm.DB, query listQuery) ([]systemLogRow, int64, error) {
	var total int64
	if err := databaseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []systemLogRow
	if err := databaseQuery.Select("sl.*, tenants.name AS tenant_name").Order("sl.occurred_at DESC, sl.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// scanAuditLogs 统计并读取操作审计日志分页行。
func scanAuditLogs(databaseQuery *gorm.DB, query listQuery) ([]auditLogRow, int64, error) {
	var total int64
	if err := databaseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []auditLogRow
	if err := databaseQuery.Select("al.*, tenants.name AS tenant_name").Order("al.occurred_at DESC, al.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// applySystemFilters 把已校验的系统日志筛选条件追加到 GORM 查询链。
func applySystemFilters(databaseQuery *gorm.DB, query listQuery) *gorm.DB {
	if query.LogType != "" {
		databaseQuery = databaseQuery.Where("sl.log_type = ?", query.LogType)
	}
	if query.Level != "" {
		databaseQuery = databaseQuery.Where("sl.level = ?", query.Level)
	}
	if query.Method != "" {
		databaseQuery = databaseQuery.Where("sl.method = ?", query.Method)
	}
	if query.Route != "" {
		databaseQuery = databaseQuery.Where("sl.route LIKE ?", "%"+query.Route+"%")
	}
	if query.StatusCode != nil {
		databaseQuery = databaseQuery.Where("sl.status_code = ?", *query.StatusCode)
	}
	if query.Code != nil {
		databaseQuery = databaseQuery.Where("sl.business_code = ?", *query.Code)
	}
	if query.RequestID != "" {
		databaseQuery = databaseQuery.Where("sl.request_id = ?", query.RequestID)
	}
	if query.TenantID != nil {
		databaseQuery = databaseQuery.Where("sl.tenant_id = ?", *query.TenantID)
	}
	if query.Workspace != "" {
		databaseQuery = databaseQuery.Where("sl.workspace = ?", query.Workspace)
	}
	if query.Operator != "" {
		databaseQuery = databaseQuery.Where("sl.actor_name LIKE ? OR sl.actor_account LIKE ?", "%"+query.Operator+"%", "%"+query.Operator+"%")
	}
	if query.ActorType != "" {
		databaseQuery = databaseQuery.Where("sl.actor_type = ?", query.ActorType)
	}
	if query.ActorID != nil {
		databaseQuery = databaseQuery.Where("sl.actor_id = ?", *query.ActorID)
	}
	return databaseQuery
}

// applyAuditFilters 把已校验的操作审计筛选条件追加到 GORM 查询链。
func applyAuditFilters(databaseQuery *gorm.DB, query listQuery) *gorm.DB {
	if query.Operator != "" {
		databaseQuery = databaseQuery.Where("al.actor_name LIKE ? OR al.actor_account LIKE ?", "%"+query.Operator+"%", "%"+query.Operator+"%")
	}
	if query.ActorID != nil {
		databaseQuery = databaseQuery.Where("al.actor_employee_id = ?", *query.ActorID)
	}
	if query.Workspace != "" {
		databaseQuery = databaseQuery.Where("al.actor_scope = ?", query.Workspace)
	}
	if query.Module != "" {
		databaseQuery = databaseQuery.Where("al.module_code = ?", query.Module)
	}
	if query.Action != "" {
		databaseQuery = databaseQuery.Where("al.action_code = ?", query.Action)
	}
	if query.TargetType != "" {
		databaseQuery = databaseQuery.Where("al.target_type = ?", query.TargetType)
	}
	if query.Target != "" {
		databaseQuery = databaseQuery.Where("al.target_id LIKE ? OR al.target_name LIKE ?", "%"+query.Target+"%", "%"+query.Target+"%")
	}
	return databaseQuery
}

// latestOperatorQuery 生成按稳定操作者标识获取最新历史快照的查询语句。
func latestOperatorQuery(kind string, forcedTenantID *uint64) (string, []any) {
	if kind == "system" {
		return `
			SELECT ranked.actor_type, ranked.actor_id, ranked.actor_name, ranked.actor_account
			FROM (
				SELECT actor_type, actor_id, actor_name, actor_account,
					ROW_NUMBER() OVER (PARTITION BY actor_type, actor_id ORDER BY occurred_at DESC, id DESC) AS snapshot_rank
				FROM system_logs
				WHERE actor_type IS NOT NULL AND actor_id IS NOT NULL
			) AS ranked
			WHERE ranked.snapshot_rank = 1
			ORDER BY ranked.actor_name ASC, ranked.actor_account ASC, ranked.actor_id ASC`, nil
	}
	whereClause := ""
	arguments := []any{}
	if forcedTenantID != nil {
		whereClause = "WHERE tenant_id = ?"
		arguments = append(arguments, *forcedTenantID)
	}
	return `
			SELECT 'employee' AS actor_type, ranked.actor_employee_id AS actor_id, ranked.actor_name, ranked.actor_account
			FROM (
				SELECT actor_employee_id, actor_name, actor_account,
					ROW_NUMBER() OVER (PARTITION BY actor_employee_id ORDER BY occurred_at DESC, id DESC) AS snapshot_rank
				FROM operation_audit_logs ` + whereClause + `
			) AS ranked
			WHERE ranked.snapshot_rank = 1
			ORDER BY ranked.actor_name ASC, ranked.actor_account ASC, ranked.actor_employee_id ASC`, arguments
}
