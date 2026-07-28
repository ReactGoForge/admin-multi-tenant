package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
)

// TestWriteSuccess 验证对象、数组和空数据都使用固定成功响应外壳。
func TestWriteSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		data any
		want string
	}{
		{name: "对象数据", data: gin.H{"message": "pong"}, want: `"data":{"message":"pong"}`},
		{name: "数组数据", data: []string{"a", "b"}, want: `"data":["a","b"]`},
		{name: "空数据", data: nil, want: `"data":null`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)

			WriteSuccess(context, http.StatusOK, test.data)

			if recorder.Code != http.StatusOK {
				t.Fatalf("状态码 = %d，期望 %d", recorder.Code, http.StatusOK)
			}
			body := recorder.Body.String()
			if !strings.Contains(body, `"code":0`) || !strings.Contains(body, `"message":"成功"`) || !strings.Contains(body, test.want) {
				t.Fatalf("成功响应不符合统一结构: %s", body)
			}
		})
	}
}

// TestWriteError 验证所有公开错误码和未知错误码都映射为稳定数字响应并中止请求链。
func TestWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name         string
		requestCode  ErrorCode
		responseCode int
		status       int
		message      string
	}{
		{name: "请求参数错误", requestCode: ErrorCodeInvalidRequest, responseCode: 10001, status: http.StatusBadRequest, message: "请求参数错误"},
		{name: "验证码错误", requestCode: ErrorCodeCaptchaInvalid, responseCode: 10002, status: http.StatusBadRequest, message: "验证码错误或已过期"},
		{name: "验证码服务不可用", requestCode: ErrorCodeCaptchaUnavailable, responseCode: 10003, status: http.StatusServiceUnavailable, message: "验证码服务暂时不可用"},
		{name: "方法不支持", requestCode: ErrorCodeMethodNotAllowed, responseCode: 10004, status: http.StatusMethodNotAllowed, message: "请求方法不支持"},
		{name: "原密码错误", requestCode: ErrorCodeCurrentPasswordInvalid, responseCode: 10005, status: http.StatusBadRequest, message: "原密码错误"},
		{name: "登录失效", requestCode: ErrorCodeUnauthorized, responseCode: 20001, status: http.StatusUnauthorized, message: "登录状态已失效，请重新登录"},
		{name: "凭证错误", requestCode: ErrorCodeCredentialsInvalid, responseCode: 20002, status: http.StatusUnauthorized, message: "账号或密码错误"},
		{name: "账号禁用", requestCode: ErrorCodeAccountDisabled, responseCode: 20003, status: http.StatusForbidden, message: "当前员工账号已被禁用"},
		{name: "登录限流", requestCode: ErrorCodeLoginRateLimited, responseCode: 20008, status: http.StatusTooManyRequests, message: "登录尝试过于频繁，请稍后重试"},
		{name: "权限不足", requestCode: ErrorCodeForbidden, responseCode: 30001, status: http.StatusForbidden, message: "无权执行此操作"},
		{name: "角色不存在", requestCode: ErrorCodeRoleNotFound, responseCode: 40001, status: http.StatusNotFound, message: "角色不存在"},
		{name: "资源不存在", requestCode: ErrorCodeResourceNotFound, responseCode: 40002, status: http.StatusNotFound, message: "数据不存在"},
		{name: "数据冲突", requestCode: ErrorCodeConflict, responseCode: 40003, status: http.StatusConflict, message: "数据冲突，请检查关联关系或唯一字段"},
		{name: "受保护资源", requestCode: ErrorCodeProtectedResource, responseCode: 40004, status: http.StatusConflict, message: "内置对象或所有者不允许执行此操作"},
		{name: "接口不存在", requestCode: ErrorCodeEndpointNotFound, responseCode: 40005, status: http.StatusNotFound, message: "接口不存在"},
		{name: "内部错误", requestCode: ErrorCodeInternal, responseCode: 50000, status: http.StatusInternalServerError, message: "服务暂时不可用"},
		{name: "登录安全服务不可用", requestCode: ErrorCodeLoginSecurityUnavailable, responseCode: 50003, status: http.StatusServiceUnavailable, message: "登录安全服务暂时不可用"},
		{name: "服务尚未就绪", requestCode: ErrorCodeServiceNotReady, responseCode: 50004, status: http.StatusServiceUnavailable, message: "服务尚未就绪"},
		{name: "未知错误安全降级", requestCode: ErrorCode(99999), responseCode: 50000, status: http.StatusInternalServerError, message: "服务暂时不可用"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)

			WriteError(context, test.requestCode)

			if recorder.Code != test.status {
				t.Fatalf("状态码 = %d，期望 %d", recorder.Code, test.status)
			}
			if !context.IsAborted() {
				t.Fatal("WriteError() 未中止 Gin 请求链")
			}
			var response Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("解析错误响应失败: %v", err)
			}
			if response.Code != test.responseCode || response.Message != test.message || response.Data != nil {
				t.Fatalf("错误响应 = %+v，期望 code=%d message=%s data=nil", response, test.responseCode, test.message)
			}
		})
	}
}

// TestHealthRoutes 验证存活探针不访问依赖，就绪探针按检查结果返回统一响应。
func TestHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		path      string
		readiness ReadinessCheck
		status    int
		code      string
	}{
		{name: "存活", path: "/healthz", status: http.StatusOK, code: `"code":0`},
		{name: "就绪", path: "/readyz", readiness: func(_ context.Context) error { return nil }, status: http.StatusOK, code: `"code":0`},
		{name: "未就绪", path: "/readyz", readiness: func(_ context.Context) error { return errors.New("dependency down") }, status: http.StatusServiceUnavailable, code: `"code":50004`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes := routesWithNoopHandlers()
			routes.Readiness = test.readiness
			recorder := httptest.NewRecorder()
			NewRouter(routes).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
				t.Fatalf("健康检查响应 = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestRecovery 验证未捕获 panic 返回固定三字段结构且不向客户端暴露异常细节。
func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(context *gin.Context) {
		panic("boom")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d，期望 %d", recorder.Code, http.StatusInternalServerError)
	}
	body := recorder.Body.String()
	if body != `{"code":50000,"message":"服务暂时不可用","data":null}` || strings.Contains(body, "boom") || strings.Contains(body, "debug") {
		t.Fatalf("panic 响应不符合安全统一结构: %s", body)
	}
}

// TestRouterErrors 验证未知 API 地址和不支持的方法都经过统一错误响应。
func TestRouterErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(routesWithNoopHandlers())
	tests := []struct {
		name   string
		method string
		path   string
		status int
		code   string
	}{
		{name: "接口不存在", method: http.MethodGet, path: "/api/admin/missing", status: http.StatusNotFound, code: `"code":40005`},
		{name: "方法不支持", method: http.MethodPost, path: "/ping", status: http.StatusMethodNotAllowed, code: `"code":10004`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) || !strings.Contains(recorder.Body.String(), `"data":null`) {
				t.Fatalf("路由错误响应 = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestDevelopmentHTTPLoggerCoversHealthAndUnknownRoutes 验证开发文件日志位于最外层并覆盖健康检查和未知路由。
func TestDevelopmentHTTPLoggerCoversHealthAndUnknownRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	directory := t.TempDir()
	developmentLogger, err := logging.NewDevelopmentHTTPLogger(directory)
	if err != nil {
		t.Fatalf("创建开发 HTTP 日志失败: %v", err)
	}
	routes := routesWithNoopHandlers()
	routes.DevelopmentHTTPLogger = developmentLogger
	router := NewRouter(routes)
	for _, path := range []string{"/healthz", "/api/admin/missing"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	}
	if err := developmentLogger.Close(); err != nil {
		t.Fatalf("关闭开发 HTTP 日志失败: %v", err)
	}
	path := filepath.Join(directory, "http-"+time.Now().Format("2006-01-02")+".jsonl")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取开发 HTTP 日志失败: %v", err)
	}
	text := string(content)
	if lines := strings.Count(text, "\n"); lines != 2 {
		t.Fatalf("健康检查和未知路由日志行数 = %d，期望 2", lines)
	}
	if !strings.Contains(text, "/healthz") || !strings.Contains(text, "/api/admin/missing") {
		t.Fatalf("开发 HTTP 日志未覆盖全部路由: %s", text)
	}
}

// TestRegisteredRoutes 验证模块化整理后全部 HTTP 方法和路径与原路由表完全一致。
func TestRegisteredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(routesWithNoopHandlers())
	actual := make([]string, 0, len(router.Routes()))
	for _, route := range router.Routes() {
		actual = append(actual, route.Method+" "+route.Path)
	}
	expected := []string{
		"GET /ping",
		"GET /healthz",
		"GET /readyz",
		"GET /api/public/platform-brand",
		"GET /api/public/images/:imageId",
		"GET /api/admin/auth/captcha",
		"POST /api/admin/auth/login",
		"POST /api/miniapp/auth/login",
		"GET /api/miniapp/me",
		"POST /api/miniapp/profile/avatar",
		"GET /api/admin/me",
		"PUT /api/admin/profile/basic",
		"PUT /api/admin/profile/password",
		"POST /api/admin/profile/avatar",
		"GET /api/admin/dictionary-options",
		"GET /api/admin/platform/logs/system",
		"GET /api/admin/platform/logs/system/filter-options",
		"GET /api/admin/platform/logs/operations",
		"GET /api/admin/platform/logs/operations/filter-options",
		"GET /api/admin/platform/logs/login",
		"GET /api/admin/platform/logs/login/filter-options",
		"GET /api/admin/platform/dictionaries",
		"POST /api/admin/platform/dictionaries",
		"PATCH /api/admin/platform/dictionaries/:dictionaryId",
		"DELETE /api/admin/platform/dictionaries/:dictionaryId",
		"POST /api/admin/platform/dictionaries/:dictionaryId/items",
		"PATCH /api/admin/platform/dictionaries/:dictionaryId/items/:itemId",
		"DELETE /api/admin/platform/dictionaries/:dictionaryId/items/:itemId",
		"GET /api/admin/platform/employees/options",
		"GET /api/admin/platform/employees",
		"POST /api/admin/platform/employees",
		"PATCH /api/admin/platform/employees/:employeeId",
		"PUT /api/admin/platform/employees/:employeeId/roles",
		"PUT /api/admin/platform/employees/:employeeId/password",
		"PATCH /api/admin/platform/employees/:employeeId/status",
		"DELETE /api/admin/platform/employees/:employeeId",
		"GET /api/admin/platform/roles",
		"GET /api/admin/platform/roles/permission-options",
		"GET /api/admin/platform/roles/:roleId",
		"GET /api/admin/platform/roles/:roleId/employees",
		"POST /api/admin/platform/roles",
		"PATCH /api/admin/platform/roles/:roleId",
		"PUT /api/admin/platform/roles/:roleId/permissions",
		"PATCH /api/admin/platform/roles/:roleId/status",
		"DELETE /api/admin/platform/roles/:roleId",
		"GET /api/admin/platform/menus",
		"POST /api/admin/platform/menus",
		"PATCH /api/admin/platform/menus/:menuId",
		"PATCH /api/admin/platform/menus/:menuId/status",
		"DELETE /api/admin/platform/menus/:menuId",
		"GET /api/admin/platform/departments",
		"POST /api/admin/platform/departments",
		"PATCH /api/admin/platform/departments/:departmentId",
		"DELETE /api/admin/platform/departments/:departmentId",
		"GET /api/admin/platform/tenants",
		"POST /api/admin/platform/tenants",
		"PATCH /api/admin/platform/tenants/:tenantId",
		"PUT /api/admin/platform/tenants/:tenantId/owner-password",
		"PATCH /api/admin/platform/tenants/:tenantId/status",
		"GET /api/admin/platform/tenants/:tenantId/miniapp-code",
		"POST /api/admin/platform/tenants/:tenantId/miniapp-code",
		"POST /api/admin/platform/tenants/:tenantId/enter",
		"DELETE /api/admin/platform/tenants/:tenantId",
		"GET /api/admin/platform/settings/miniapp",
		"PUT /api/admin/platform/settings/miniapp",
		"GET /api/admin/platform/settings/basic",
		"PUT /api/admin/platform/settings/basic",
		"GET /api/admin/platform/images/tenant-options",
		"GET /api/admin/platform/images",
		"POST /api/admin/platform/images",
		"PATCH /api/admin/platform/images/:imageId",
		"DELETE /api/admin/platform/images/:imageId",
		"GET /api/admin/platform/image-categories",
		"POST /api/admin/platform/image-categories",
		"PATCH /api/admin/platform/image-categories/:categoryId",
		"DELETE /api/admin/platform/image-categories/:categoryId",
		"GET /api/admin/platform/users/tenant-options",
		"GET /api/admin/platform/users",
		"GET /api/admin/platform/users/:userId/tenants",
		"PATCH /api/admin/platform/users/:userId/status",
		"GET /api/admin/tenant/logs/operations",
		"GET /api/admin/tenant/logs/operations/filter-options",
		"GET /api/admin/tenant/logs/login",
		"GET /api/admin/tenant/logs/login/filter-options",
		"GET /api/admin/tenant/employees",
		"GET /api/admin/tenant/employees/options",
		"POST /api/admin/tenant/employees",
		"PATCH /api/admin/tenant/employees/:employeeId",
		"PUT /api/admin/tenant/employees/:employeeId/roles",
		"PUT /api/admin/tenant/employees/:employeeId/password",
		"PATCH /api/admin/tenant/employees/:employeeId/status",
		"DELETE /api/admin/tenant/employees/:employeeId",
		"GET /api/admin/tenant/roles",
		"GET /api/admin/tenant/roles/:roleId",
		"GET /api/admin/tenant/roles/:roleId/employees",
		"POST /api/admin/tenant/roles",
		"PATCH /api/admin/tenant/roles/:roleId",
		"PUT /api/admin/tenant/roles/:roleId/permissions",
		"PATCH /api/admin/tenant/roles/:roleId/status",
		"DELETE /api/admin/tenant/roles/:roleId",
		"GET /api/admin/tenant/menus",
		"GET /api/admin/tenant/departments",
		"POST /api/admin/tenant/departments",
		"PATCH /api/admin/tenant/departments/:departmentId",
		"DELETE /api/admin/tenant/departments/:departmentId",
		"GET /api/admin/tenant/users",
		"PATCH /api/admin/tenant/users/:userId/status",
		"GET /api/admin/tenant/settings/basic",
		"PUT /api/admin/tenant/settings/basic",
		"GET /api/admin/tenant/images",
		"POST /api/admin/tenant/images",
		"PATCH /api/admin/tenant/images/:imageId",
		"DELETE /api/admin/tenant/images/:imageId",
		"GET /api/admin/tenant/image-categories",
		"POST /api/admin/tenant/image-categories",
		"PATCH /api/admin/tenant/image-categories/:categoryId",
		"DELETE /api/admin/tenant/image-categories/:categoryId",
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("路由表不一致\n实际: %v\n期望: %v", actual, expected)
	}
}

// TestRouteAuthenticationBoundaries 验证公开、小程序和后台路由进入各自正确的认证边界。
func TestRouteAuthenticationBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		method      string
		path        string
		wantAdmin   int
		wantMiniapp int
	}{
		{name: "公开品牌", method: http.MethodGet, path: "/api/public/platform-brand"},
		{name: "后台登录", method: http.MethodPost, path: "/api/admin/auth/login"},
		{name: "小程序登录", method: http.MethodPost, path: "/api/miniapp/auth/login"},
		{name: "小程序当前用户", method: http.MethodGet, path: "/api/miniapp/me", wantMiniapp: 1},
		{name: "后台当前用户", method: http.MethodGet, path: "/api/admin/me", wantAdmin: 1},
		{name: "平台员工", method: http.MethodGet, path: "/api/admin/platform/employees", wantAdmin: 1},
		{name: "租户员工", method: http.MethodGet, path: "/api/admin/tenant/employees", wantAdmin: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes := routesWithNoopHandlers()
			adminCalls := 0
			miniappCalls := 0
			routes.Admin.Authenticate = func(context *gin.Context) {
				adminCalls++
				context.AbortWithStatus(http.StatusUnauthorized)
			}
			routes.Miniapp.Authenticate = func(context *gin.Context) {
				miniappCalls++
				context.AbortWithStatus(http.StatusUnauthorized)
			}
			router := NewRouter(routes)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if adminCalls != test.wantAdmin || miniappCalls != test.wantMiniapp {
				t.Fatalf("认证调用次数 admin=%d miniapp=%d，期望 admin=%d miniapp=%d", adminCalls, miniappCalls, test.wantAdmin, test.wantMiniapp)
			}
		})
	}
}

// routesWithNoopHandlers 为路由协议测试填充不会执行的占位处理器。
func routesWithNoopHandlers() Routes {
	routes := Routes{}
	noop := gin.HandlerFunc(func(context *gin.Context) {
		WriteSuccess(context, http.StatusOK, nil)
	})
	fillNoopHandlers(reflect.ValueOf(&routes).Elem(), reflect.ValueOf(noop))
	return routes
}

// fillNoopHandlers 递归填充嵌套路由配置中的 Gin 处理器，供路由结构测试使用。
func fillNoopHandlers(value reflect.Value, noop reflect.Value) {
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if noop.Type().AssignableTo(field.Type()) {
			field.Set(noop)
			continue
		}
		if field.Kind() == reflect.Struct {
			fillNoopHandlers(field, noop)
		}
	}
}
