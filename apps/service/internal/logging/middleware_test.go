package logging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type memoryRecorder struct {
	requests     []RequestLog
	audits       []AuditLog
	requestError error
	auditError   error
}

// CreateRequest 保存测试请求日志。
func (recorder *memoryRecorder) CreateRequest(_ context.Context, entry RequestLog) error {
	recorder.requests = append(recorder.requests, entry)
	return recorder.requestError
}

// CaptureAuditSnapshot 返回空的操作前快照，供中间件测试使用。
func (recorder *memoryRecorder) CaptureAuditSnapshot(_ context.Context, _ string, _ map[string]string, _ Actor) (AuditSnapshot, error) {
	return AuditSnapshot{Values: map[string]any{}}, nil
}

// RecordAudit 保存测试操作审计日志。
func (recorder *memoryRecorder) RecordAudit(_ context.Context, entry AuditLog) error {
	recorder.audits = append(recorder.audits, entry)
	return recorder.auditError
}

// TestMiddlewareRecordsSanitizedMetadata 验证请求日志不保存请求体和认证头，并带服务端请求 ID。
func TestMiddlewareRecordsSanitizedMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &memoryRecorder{}
	router := gin.New()
	router.Use(Middleware(recorder, RequestLogModeMutationAndError), func(context *gin.Context) {
		SetActor(context, Actor{Type: "employee", ID: 7, Name: "管理员", Account: "admin", Scope: "platform", Workspace: "platform", AuthMode: "normal"})
		context.Next()
	}, AuditMiddleware(recorder))
	router.POST("/api/admin/platform/employees", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"code": 0, "message": "成功", "data": nil})
	})

	request := httptest.NewRequest(http.MethodPost, "/api/admin/platform/employees?token=query-secret", strings.NewReader(`{"password":"body-secret"}`))
	request.Header.Set("Authorization", "Bearer header-secret")
	request.Header.Set("User-Agent", strings.Repeat("a", 600))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if len(recorder.requests) != 1 || len(recorder.audits) != 1 {
		t.Fatalf("请求日志=%d，审计日志=%d", len(recorder.requests), len(recorder.audits))
	}
	entry := recorder.requests[0]
	if entry.RequestID == "" || response.Header().Get("X-Request-ID") != entry.RequestID {
		t.Fatalf("请求 ID 未正确生成或返回")
	}
	encoded := entry.Path + entry.Route + entry.Message + string(entry.Metadata)
	if strings.Contains(encoded, "secret") || strings.Contains(encoded, "token=") || len([]rune(entry.UserAgent)) != 512 {
		t.Fatalf("请求日志包含敏感内容或未截断 User-Agent: %#v", entry)
	}
}

// TestMiddlewareOnlyAuditsSuccessfulMutations 验证失败写操作和只读请求不会写操作审计。
func TestMiddlewareOnlyAuditsSuccessfulMutations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &memoryRecorder{}
	router := gin.New()
	router.Use(Middleware(recorder, RequestLogModeMutationAndError), func(context *gin.Context) {
		SetActor(context, Actor{Type: "employee", ID: 1, Name: "平台人员", Account: "operator", Scope: "platform", Workspace: "platform", AuthMode: "normal"})
		context.Next()
	}, AuditMiddleware(recorder))
	router.PATCH("/api/admin/platform/users/:userId/status", func(context *gin.Context) { context.Status(http.StatusBadRequest) })
	router.GET("/api/admin/platform/users", func(context *gin.Context) { context.Status(http.StatusOK) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/api/admin/platform/users/9/status", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/admin/platform/users", nil))
	if len(recorder.audits) != 0 {
		t.Fatalf("失败写操作或只读请求产生了审计日志")
	}
}

// TestMiddlewareExcludesPublicImage 验证公开图片二进制请求不会写入系统日志。
func TestMiddlewareExcludesPublicImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &memoryRecorder{}
	router := gin.New()
	router.Use(Middleware(recorder, RequestLogModeMutationAndError))
	router.GET("/api/public/images/:imageId", func(context *gin.Context) { context.Status(http.StatusOK) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/public/images/1", nil))
	if len(recorder.requests) != 0 {
		t.Fatalf("公开图片请求被写入系统日志")
	}
}

// TestMiddlewareSkipsSuccessfulQueries 验证正常查询不入库，但查询失败仍保留排错记录。
func TestMiddlewareSkipsSuccessfulQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &memoryRecorder{}
	router := gin.New()
	router.Use(Middleware(recorder, RequestLogModeMutationAndError))
	router.GET("/api/admin/items", func(context *gin.Context) { context.Status(http.StatusOK) })
	router.GET("/api/admin/items/:itemId", func(context *gin.Context) { context.Status(http.StatusInternalServerError) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/admin/items", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/admin/items/1", nil))
	if len(recorder.requests) != 1 || recorder.requests[0].StatusCode != http.StatusInternalServerError {
		t.Fatalf("正常查询未被排除或失败查询未被保留: %#v", recorder.requests)
	}
}

// TestMiddlewareAllModeIncludesSuccessfulQueries 验证全量模式会保存正常查询且仍排除公开图片。
func TestMiddlewareAllModeIncludesSuccessfulQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &memoryRecorder{}
	router := gin.New()
	router.Use(Middleware(recorder, RequestLogModeAll))
	router.GET("/api/admin/items", func(context *gin.Context) { context.Status(http.StatusOK) })
	router.GET("/api/public/images/:imageId", func(context *gin.Context) { context.Status(http.StatusOK) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/admin/items", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/public/images/1", nil))
	if len(recorder.requests) != 1 || recorder.requests[0].Route != "/api/admin/items" {
		t.Fatalf("全量模式记录结果错误: %#v", recorder.requests)
	}
}

// TestMiddlewareOffModeKeepsRequestID 验证关闭入库后不保存请求日志但仍生成请求 ID。
func TestMiddlewareOffModeKeepsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &memoryRecorder{}
	router := gin.New()
	router.Use(Middleware(recorder, RequestLogModeOff))
	router.POST("/api/admin/items", func(context *gin.Context) { context.Status(http.StatusOK) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/admin/items", nil))
	if len(recorder.requests) != 0 || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("关闭入库后的请求记录或请求 ID 错误: requests=%d requestID=%q", len(recorder.requests), response.Header().Get("X-Request-ID"))
	}
}

// TestAuditMiddlewareRecordsUnknownModules 验证新模块未登记中文名称时仍会生成基础操作审计。
func TestAuditMiddlewareRecordsUnknownModules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &memoryRecorder{}
	router := gin.New()
	router.Use(func(context *gin.Context) {
		SetActor(context, Actor{Type: "employee", ID: 1, Name: "平台人员", Account: "operator", Scope: "platform", Workspace: "platform", AuthMode: "normal"})
		context.Next()
	}, AuditMiddleware(recorder))
	router.POST("/api/admin/platform/orders", func(context *gin.Context) { context.Status(http.StatusOK) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/admin/platform/orders", nil))
	if len(recorder.audits) != 1 || recorder.audits[0].ModuleCode != "orders" || recorder.audits[0].ActionCode != "create" {
		t.Fatalf("未知模块基础审计错误: %#v", recorder.audits)
	}
}

// TestAuditModuleLabelKeepsKnownAndUnknownValues 验证模块筛选文案对已知编码翻译并对未知编码安全回退。
func TestAuditModuleLabelKeepsKnownAndUnknownValues(t *testing.T) {
	if label := AuditModuleLabel("employee"); label != "员工" {
		t.Fatalf("已知模块文案错误: %q", label)
	}
	if label := AuditModuleLabel("orders"); label != "orders" {
		t.Fatalf("未知模块文案未回退原编码: %q", label)
	}
	code, name := auditModule("/api/admin/tenant/settings/notifications")
	if code != "settings_notifications" || name != "settings_notifications" {
		t.Fatalf("设置子模块推导错误: code=%q name=%q", code, name)
	}
}
