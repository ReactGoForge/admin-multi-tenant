package logging

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const cleanupBatchSize = 1000

// Store 提供系统日志、操作审计和过期系统日志清理的数据访问能力。
type Store struct {
	db             *gorm.DB
	eventDBEnabled bool
}

// CreateEvent 写入一条不依赖 HTTP 请求的脱敏运行事件。
func (store *Store) CreateEvent(ctx context.Context, level, message string, metadata map[string]any) error {
	if !store.eventDBEnabled {
		return nil
	}
	var encoded any
	if len(metadata) > 0 {
		value, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		encoded = value
	}
	return store.db.WithContext(ctx).Table("system_logs").Create(map[string]any{"log_type": "event", "level": level, "occurred_at": time.Now().UTC(), "message": truncate(message, 500), "metadata": encoded}).Error
}

// CreateLogin 写入一条不受普通运行事件开关影响的后台登录日志。
func (store *Store) CreateLogin(ctx context.Context, entry LoginLog) error {
	values := map[string]any{
		"log_type": "event", "level": entry.Level, "request_id": entry.RequestID,
		"occurred_at": entry.OccurredAt, "method": "POST", "route": "/api/admin/auth/login",
		"path": "/api/admin/auth/login", "client_ip": truncate(entry.ClientIP, 45),
		"user_agent": truncate(entry.UserAgent, 512), "actor_account": truncate(entry.ActorAccount, 100),
		"workspace": entry.Workspace, "tenant_id": entry.TenantID,
		"message": truncate(entry.Message, 500), "metadata": entry.Metadata,
	}
	if entry.ActorID != nil {
		values["actor_type"] = "employee"
		values["actor_id"] = entry.ActorID
		values["actor_name"] = entry.ActorName
	}
	return store.db.WithContext(ctx).Table("system_logs").Create(values).Error
}

// FindAuditSnapshot 执行 Logging Service 已按白名单生成的审计快照查询。
func (store *Store) FindAuditSnapshot(ctx context.Context, query auditSnapshotQuery) (map[string]any, error) {
	values := make(map[string]any)
	databaseQuery := store.db.WithContext(ctx).Table(query.Table).Select(query.Columns).Where("id = ?", query.ID)
	if query.TenantID != nil {
		databaseQuery = databaseQuery.Where("tenant_id = ?", *query.TenantID)
	}
	if err := databaseQuery.Take(&values).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return values, nil
}

// NewStore 使用现有 GORM 连接和运行事件开关创建日志数据对象。
func NewStore(db *gorm.DB, eventDBEnabled bool) *Store {
	return &Store{db: db, eventDBEnabled: eventDBEnabled}
}

// CreateRequest 写入一条请求日志。
func (store *Store) CreateRequest(ctx context.Context, entry RequestLog) error {
	return store.db.WithContext(ctx).Table("system_logs").Create(&entry).Error
}

// CreateAudit 写入一条成功操作审计日志。
func (store *Store) CreateAudit(ctx context.Context, entry AuditLog) error {
	return store.db.WithContext(ctx).Table("operation_audit_logs").Create(&entry).Error
}

// CleanupSystemLogs 分批删除截止时间以前的系统日志，避免单次长事务。
func (store *Store) CleanupSystemLogs(ctx context.Context, cutoff time.Time) (int64, error) {
	// Go 学习提示：循环分批查询和删除，避免一次删除大量记录造成长事务和数据库压力。
	var total int64
	for {
		var ids []uint64
		if err := store.db.WithContext(ctx).Table("system_logs").Where("occurred_at < ?", cutoff).Order("id ASC").Limit(cleanupBatchSize).Pluck("id", &ids).Error; err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		result := store.db.WithContext(ctx).Table("system_logs").Where("id IN ?", ids).Delete(nil)
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if len(ids) < cleanupBatchSize {
			return total, nil
		}
	}
}
