package logging

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

// serviceTestStore 保存 Logging Service 测试收到的数据访问请求。
type serviceTestStore struct {
	eventErr      error
	loginErr      error
	auditErr      error
	snapshotErr   error
	eventCalls    int
	loginEntries  []LoginLog
	auditEntries  []AuditLog
	snapshotQuery auditSnapshotQuery
	snapshotCalls int
	snapshot      map[string]any
	cleanupCutoff time.Time
	cleanupTotal  int64
	cleanupErr    error
}

// CreateEvent 记录测试运行事件。
func (store *serviceTestStore) CreateEvent(_ context.Context, _, _ string, _ map[string]any) error {
	store.eventCalls++
	return store.eventErr
}

// CreateLogin 记录测试登录日志。
func (store *serviceTestStore) CreateLogin(_ context.Context, entry LoginLog) error {
	store.loginEntries = append(store.loginEntries, entry)
	return store.loginErr
}

// CreateAudit 记录测试操作审计日志。
func (store *serviceTestStore) CreateAudit(_ context.Context, entry AuditLog) error {
	store.auditEntries = append(store.auditEntries, entry)
	return store.auditErr
}

// FindAuditSnapshot 记录并返回测试审计快照。
func (store *serviceTestStore) FindAuditSnapshot(_ context.Context, query auditSnapshotQuery) (map[string]any, error) {
	store.snapshotCalls++
	store.snapshotQuery = query
	return store.snapshot, store.snapshotErr
}

// CleanupSystemLogs 记录测试日志清理截止时间。
func (store *serviceTestStore) CleanupSystemLogs(_ context.Context, cutoff time.Time) (int64, error) {
	store.cleanupCutoff = cutoff
	return store.cleanupTotal, store.cleanupErr
}

// TestServiceRecordEventKeepsStructuredOutput 验证运行事件数据库失败时仍输出降级日志和原事件。
func TestServiceRecordEventKeepsStructuredOutput(t *testing.T) {
	store := &serviceTestStore{eventErr: errors.New("database unavailable")}
	service := NewService(store)
	var output bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	service.RecordEvent(context.Background(), "warn", "测试运行事件", nil)

	if store.eventCalls != 1 {
		t.Fatalf("运行事件写入次数错误: %d", store.eventCalls)
	}
	text := output.String()
	if !strings.Contains(text, "运行事件写入数据库失败") || !strings.Contains(text, "测试运行事件") {
		t.Fatalf("运行事件降级输出不完整: %s", text)
	}
}

// TestServiceRecordLoginAndAuditDelegateToStore 验证登录日志和操作审计统一由 Service 委托持久化。
func TestServiceRecordLoginAndAuditDelegateToStore(t *testing.T) {
	store := &serviceTestStore{}
	service := NewService(store)
	loginEntry := LoginLog{ActorAccount: "admin"}
	auditEntry := AuditLog{ModuleCode: "employee"}

	if err := service.RecordLogin(context.Background(), loginEntry); err != nil {
		t.Fatalf("记录登录日志失败: %v", err)
	}
	if err := service.RecordAudit(context.Background(), auditEntry); err != nil {
		t.Fatalf("记录操作审计失败: %v", err)
	}
	if len(store.loginEntries) != 1 || store.loginEntries[0].ActorAccount != "admin" {
		t.Fatalf("登录日志委托错误: %#v", store.loginEntries)
	}
	if len(store.auditEntries) != 1 || store.auditEntries[0].ModuleCode != "employee" {
		t.Fatalf("操作审计委托错误: %#v", store.auditEntries)
	}
}

// TestServiceCaptureAuditSnapshotUsesWhitelistAndTenantScope 验证审计快照只使用模块白名单并附加可信租户范围。
func TestServiceCaptureAuditSnapshotUsesWhitelistAndTenantScope(t *testing.T) {
	tenantID := uint64(88)
	store := &serviceTestStore{snapshot: map[string]any{"id": uint64(7), "name": []byte("租户员工")}}
	service := NewService(store)

	snapshot, err := service.CaptureAuditSnapshot(
		context.Background(),
		"/api/admin/tenant/employees/:employeeId",
		map[string]string{"employeeId": "7"},
		Actor{Workspace: "tenant", TenantID: &tenantID},
	)
	if err != nil {
		t.Fatalf("读取审计快照失败: %v", err)
	}
	if store.snapshotCalls != 1 || store.snapshotQuery.Table != "employees" || store.snapshotQuery.ID != 7 {
		t.Fatalf("审计快照白名单查询错误: %#v", store.snapshotQuery)
	}
	if store.snapshotQuery.TenantID == nil || *store.snapshotQuery.TenantID != tenantID {
		t.Fatalf("审计快照未附加可信租户范围: %#v", store.snapshotQuery)
	}
	if snapshot.TargetID != "7" || snapshot.TargetName != "租户员工" {
		t.Fatalf("审计快照结果错误: %#v", snapshot)
	}
}

// TestServiceCaptureAuditSnapshotSkipsInvalidAndUnknownTargets 验证非法 ID 和未知模块不会触发数据库查询。
func TestServiceCaptureAuditSnapshotSkipsInvalidAndUnknownTargets(t *testing.T) {
	store := &serviceTestStore{}
	service := NewService(store)
	cases := []struct {
		route  string
		params map[string]string
	}{
		{route: "/api/admin/platform/employees/:employeeId", params: map[string]string{"employeeId": "invalid"}},
		{route: "/api/admin/platform/orders/:itemId", params: map[string]string{"itemId": "9"}},
	}
	for _, testCase := range cases {
		snapshot, err := service.CaptureAuditSnapshot(context.Background(), testCase.route, testCase.params, Actor{})
		if err != nil {
			t.Fatalf("跳过无效快照时返回错误: %v", err)
		}
		if len(snapshot.Values) != 0 {
			t.Fatalf("跳过无效快照时返回了字段: %#v", snapshot)
		}
	}
	if store.snapshotCalls != 0 {
		t.Fatalf("非法或未知目标触发了数据库查询: %d", store.snapshotCalls)
	}
}

// TestServiceCleanupSystemLogsDelegatesCutoff 验证日志保留清理由 Service 按原截止时间委托 Store。
func TestServiceCleanupSystemLogsDelegatesCutoff(t *testing.T) {
	cutoff := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	store := &serviceTestStore{cleanupTotal: 12}
	service := NewService(store)

	total, err := service.CleanupSystemLogs(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("清理系统日志失败: %v", err)
	}
	if total != 12 || !store.cleanupCutoff.Equal(cutoff) {
		t.Fatalf("日志清理委托错误: total=%d cutoff=%s", total, store.cleanupCutoff)
	}
}
