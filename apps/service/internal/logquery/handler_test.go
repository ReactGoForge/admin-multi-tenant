package logquery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type fakeLogQueryStore struct {
	err              error
	auditTenantID    *uint64
	loginTenantID    *uint64
	operatorTenantID *uint64
	codeTenantID     *uint64
	systemRows       []systemLogRow
	auditRows        []auditLogRow
	loginRows        []systemLogRow
	tenants          []tenantOptionRow
	operators        []operatorOptionRow
	modules          []auditModuleOptionRow
	actions          []auditActionOptionRow
}

// ListSystemLogs 返回测试预置的系统日志行。
func (store *fakeLogQueryStore) ListSystemLogs(context.Context, listQuery) ([]systemLogRow, int64, error) {
	if store.err != nil {
		return nil, 0, store.err
	}
	return store.systemRows, int64(len(store.systemRows)), nil
}

// ListAuditLogs 返回测试预置的操作审计行并记录强制租户范围。
func (store *fakeLogQueryStore) ListAuditLogs(_ context.Context, _ listQuery, tenantID *uint64) ([]auditLogRow, int64, error) {
	store.auditTenantID = tenantID
	if store.err != nil {
		return nil, 0, store.err
	}
	return store.auditRows, int64(len(store.auditRows)), nil
}

// ListLoginLogs 返回测试预置的登录日志行并记录强制租户范围。
func (store *fakeLogQueryStore) ListLoginLogs(_ context.Context, _ listQuery, tenantID *uint64) ([]systemLogRow, int64, error) {
	store.loginTenantID = tenantID
	if store.err != nil {
		return nil, 0, store.err
	}
	return store.loginRows, int64(len(store.loginRows)), nil
}

// ListTenants 返回测试预置的租户选项行。
func (store *fakeLogQueryStore) ListTenants(context.Context) ([]tenantOptionRow, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.tenants, nil
}

// ListLatestOperators 返回测试预置的操作者选项行并记录强制租户范围。
func (store *fakeLogQueryStore) ListLatestOperators(_ context.Context, _ string, tenantID *uint64) ([]operatorOptionRow, error) {
	store.operatorTenantID = tenantID
	if store.err != nil {
		return nil, store.err
	}
	return store.operators, nil
}

// ListAuditCodeOptions 返回测试预置的审计模块和动作选项行并记录强制租户范围。
func (store *fakeLogQueryStore) ListAuditCodeOptions(_ context.Context, tenantID *uint64) ([]auditModuleOptionRow, []auditActionOptionRow, error) {
	store.codeTenantID = tenantID
	if store.err != nil {
		return nil, nil, store.err
	}
	return store.modules, store.actions, nil
}

// newQueryContext 创建只用于解析查询参数的 Gin 测试上下文。
func newQueryContext(target string) *gin.Context {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", target, nil)
	return context
}

// newDryRunDB 创建不连接真实数据库的 GORM SQL 生成器。
func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("创建 DryRun 数据库失败: %v", err)
	}
	return database
}

// TestParseQueryAcceptsExactActor 验证精确操作者参数可以解析且非法类型会被拒绝。
func TestParseQueryAcceptsExactActor(t *testing.T) {
	handler := &Handler{now: func() time.Time { return time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC) }}
	query, valid := handler.parseQuery(newQueryContext("/?actorType=employee&actorId=12&logType=request"), true)
	if !valid || query.ActorType != "employee" || query.ActorID == nil || *query.ActorID != 12 || query.LogType != "request" {
		t.Fatalf("精确操作者参数解析错误: valid=%v query=%+v", valid, query)
	}
	if _, valid = handler.parseQuery(newQueryContext("/?actorType=unknown&actorId=12"), true); valid {
		t.Fatal("非法操作者类型未被拒绝")
	}
	if _, valid = handler.parseQuery(newQueryContext("/?logType=unknown"), true); valid {
		t.Fatal("非法系统日志类型未被拒绝")
	}
}

// TestParseLoginQueryAndTenantIsolation 验证登录结果筛选合法且租户接口忽略客户端指定的租户 ID。
func TestParseLoginQueryAndTenantIsolation(t *testing.T) {
	handler := &Handler{now: func() time.Time { return time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC) }}
	query, valid := handler.parseQuery(newQueryContext("/?account=admin&result=limited&clientIp=192.0.2.1&tenantId=9"), false)
	if !valid || query.Account != "admin" || query.Result != "limited" || query.ClientIP != "192.0.2.1" || query.TenantID != nil {
		t.Fatalf("租户登录日志参数解析错误: valid=%v query=%+v", valid, query)
	}
	if _, valid := handler.parseQuery(newQueryContext("/?result=unknown"), true); valid {
		t.Fatal("非法登录结果未被拒绝")
	}
}

// TestApplyLogFiltersKeepsLegacyOperator 验证精确筛选与原姓名账号模糊筛选可以兼容共存。
func TestApplyLogFiltersKeepsLegacyOperator(t *testing.T) {
	actorID := uint64(8)
	database := newDryRunDB(t)
	statement := applySystemFilters(database.Table("system_logs AS sl"), listQuery{
		Operator:  "admin",
		ActorType: "employee",
		ActorID:   &actorID,
		LogType:   "event",
	}).Find(&[]systemLogRow{}).Statement
	sql := statement.SQL.String()
	for _, fragment := range []string{"sl.log_type =", "sl.actor_name LIKE", "sl.actor_account LIKE", "sl.actor_type =", "sl.actor_id ="} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("系统日志筛选 SQL 缺少 %q: %s", fragment, sql)
		}
	}

	auditStatement := applyAuditFilters(database.Table("operation_audit_logs AS al"), listQuery{ActorID: &actorID}).Find(&[]auditLogRow{}).Statement
	if !strings.Contains(auditStatement.SQL.String(), "al.actor_employee_id =") {
		t.Fatalf("操作日志未按员工 ID 精确筛选: %s", auditStatement.SQL.String())
	}
}

// TestLatestOperatorQueryUsesLatestSnapshotAndTenantScope 验证历史操作者按稳定 ID 取最新快照且租户查询带强制范围。
func TestLatestOperatorQueryUsesLatestSnapshotAndTenantScope(t *testing.T) {
	systemSQL, systemArguments := latestOperatorQuery("system", nil)
	if len(systemArguments) != 0 || !strings.Contains(systemSQL, "PARTITION BY actor_type, actor_id") || !strings.Contains(systemSQL, "snapshot_rank = 1") {
		t.Fatalf("系统日志操作者去重 SQL 错误: %s %#v", systemSQL, systemArguments)
	}
	tenantID := uint64(9)
	auditSQL, auditArguments := latestOperatorQuery("audit", &tenantID)
	if !strings.Contains(auditSQL, "WHERE tenant_id = ?") || !strings.Contains(auditSQL, "PARTITION BY actor_employee_id") || len(auditArguments) != 1 || auditArguments[0] != tenantID {
		t.Fatalf("租户操作日志操作者范围 SQL 错误: %s %#v", auditSQL, auditArguments)
	}
}

// TestLogResponsesIncludeTenantName 验证系统日志和操作日志响应保留 ID 并新增租户名称。
func TestLogResponsesIncludeTenantName(t *testing.T) {
	tenantName := "示例租户"
	system := systemResponse(systemLogRow{ID: 1, TenantName: &tenantName})
	audit := auditResponse(auditLogRow{ID: 2, TenantName: &tenantName})
	for name, response := range map[string]objectResponse{"system": system, "audit": audit} {
		value, valid := response["tenantName"].(*string)
		if !valid || value == nil || *value != tenantName {
			t.Fatalf("%s 响应缺少租户名称: %#v", name, response)
		}
	}
}

// TestLoginResponseExposesStructuredResult 验证登录日志只展开允许展示的结构化结果字段。
func TestLoginResponseExposesStructuredResult(t *testing.T) {
	account := "admin"
	metadata := []byte(`{"result":"failed","reason":"credentials_invalid","password":"secret"}`)
	response := loginResponse(systemLogRow{ID: 3, ActorAccount: &account, Metadata: metadata})
	if response["result"] != "failed" || response["reason"] != "credentials_invalid" || response["account"] != &account {
		t.Fatalf("登录日志响应错误: %#v", response)
	}
	if _, exists := response["password"]; exists {
		t.Fatal("登录日志响应不应展开非白名单元数据")
	}
}

// TestServiceTenantAuditUsesForcedScopeAndOptions 验证 Service 向 Store 传递可信租户范围并组装筛选选项。
func TestServiceTenantAuditUsesForcedScopeAndOptions(t *testing.T) {
	name := "租户员工"
	account := "operator"
	store := &fakeLogQueryStore{
		operators: []operatorOptionRow{{ActorType: "employee", ActorID: 7, Name: &name, Account: &account}},
		modules:   []auditModuleOptionRow{{Value: "tenant"}},
		actions:   []auditActionOptionRow{{Value: "update", Label: "更新"}},
	}
	tenantID := uint64(9)
	response, err := NewService(store).ListTenantAuditFilterOptions(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("租户审计筛选选项查询失败: %v", err)
	}
	if store.operatorTenantID == nil || *store.operatorTenantID != tenantID || store.codeTenantID == nil || *store.codeTenantID != tenantID {
		t.Fatalf("Service 未传递可信租户范围: operator=%v code=%v", store.operatorTenantID, store.codeTenantID)
	}
	if len(response.Tenants) != 0 || len(response.Operators) != 1 || response.Operators[0].Key != "employee:7" || len(response.Modules) != 1 || len(response.Actions) != 1 {
		t.Fatalf("租户审计筛选选项响应错误: %#v", response)
	}
}

// TestHandlerTenantAuditRequiresTrustedTenant 验证租户日志接口缺少可信租户身份时返回 403。
func TestHandlerTenantAuditRequiresTrustedTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/api/admin/tenant/logs/operations", nil)
	handler := NewHandler(NewService(&fakeLogQueryStore{}))

	handler.ListTenantAuditLogs(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("缺少可信租户身份时状态码错误: %d", recorder.Code)
	}
}

// TestHandlerTenantAuditUsesActorTenant 验证 Handler 从认证上下文读取租户 ID，而不是信任客户端 tenantId。
func TestHandlerTenantAuditUsesActorTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/api/admin/tenant/logs/operations?tenantId=99", nil)
	tenantID := uint64(12)
	logging.SetActor(context, logging.Actor{Type: "employee", ID: 1, TenantID: &tenantID})
	store := &fakeLogQueryStore{}
	handler := NewHandler(NewService(store))

	handler.ListTenantAuditLogs(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("租户审计列表状态码错误: %d", recorder.Code)
	}
	if store.auditTenantID == nil || *store.auditTenantID != tenantID {
		t.Fatalf("Handler 未使用认证上下文租户 ID: %v", store.auditTenantID)
	}
	var envelope struct {
		Code int          `json:"code"`
		Data listResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || envelope.Code != 0 || envelope.Data.Page != 1 {
		t.Fatalf("租户审计列表响应错误: err=%v body=%s", err, recorder.Body.String())
	}
}

// TestHandlerPlatformSystemMapsStoreError 验证 Store 查询失败时 Handler 保持 500 统一错误响应。
func TestHandlerPlatformSystemMapsStoreError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/api/admin/platform/logs/system", nil)
	handler := NewHandler(NewService(&fakeLogQueryStore{err: errors.New("database failed")}))

	handler.ListPlatformSystemLogs(context)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Store 失败时状态码错误: %d", recorder.Code)
	}
}
