package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type testEmployeeStore struct {
	loginEmployee        *Employee
	idEmployee           *Employee
	tenant               *Tenant
	roles                []Role
	permissions          []string
	managedPermissions   []string
	platformBrand        *PlatformBrand
	navigationMenus      []NavigationMenu
	permissionCalls      int
	managedSuperAdmin    bool
	loginError           error
	idError              error
	tenantError          error
	rolesError           error
	permissionsError     error
	platformBrandError   error
	navigationMenusError error
	activateError        error
	activatedSessionID   string
	updateProfileError   error
	updatedEmployeeID    uint64
	updatedPhone         *string
	changePasswordError  error
	changedPasswordHash  string
}

// UpdateBasicProfile 记录测试中的当前员工资料更新参数。
func (store *testEmployeeStore) UpdateBasicProfile(_ context.Context, employeeID uint64, phone *string) error {
	if store.updateProfileError != nil {
		return store.updateProfileError
	}
	store.updatedEmployeeID = employeeID
	store.updatedPhone = phone
	if store.idEmployee != nil {
		store.idEmployee.Phone = phone
	}
	return nil
}

// ChangePassword 记录测试中的新密码哈希并清空预设员工会话。
func (store *testEmployeeStore) ChangePassword(_ context.Context, _ uint64, passwordHash string) error {
	if store.changePasswordError != nil {
		return store.changePasswordError
	}
	store.changedPasswordHash = passwordHash
	if store.idEmployee != nil {
		store.idEmployee.ActiveSessionID = nil
	}
	return nil
}

// FindByLogin 返回测试预设的登录员工。
func (store *testEmployeeStore) FindByLogin(_ context.Context, _ string) (*Employee, error) {
	return store.loginEmployee, store.loginError
}

// FindByID 返回测试预设的认证员工。
func (store *testEmployeeStore) FindByID(_ context.Context, _ uint64) (*Employee, error) {
	return store.idEmployee, store.idError
}

// ActivateSession 记录测试登录激活的会话标识，并同步到预设员工。
func (store *testEmployeeStore) ActivateSession(_ context.Context, _ uint64, sessionID string) error {
	if store.activateError != nil {
		return store.activateError
	}
	store.activatedSessionID = sessionID
	if store.loginEmployee != nil {
		store.loginEmployee.ActiveSessionID = &store.activatedSessionID
	}
	if store.idEmployee != nil {
		store.idEmployee.ActiveSessionID = &store.activatedSessionID
	}
	return nil
}

// FindTenantByID 返回测试预设的租户。
func (store *testEmployeeStore) FindTenantByID(_ context.Context, _ uint64) (*Tenant, error) {
	return store.tenant, store.tenantError
}

// ListRoles 返回测试预设的员工角色。
func (store *testEmployeeStore) ListRoles(_ context.Context, _ Employee) ([]Role, error) {
	return store.roles, store.rolesError
}

// ListPermissions 返回测试预设的实时权限并记录调用次数。
func (store *testEmployeeStore) ListPermissions(_ context.Context, _ Employee, _ []uint64) ([]string, error) {
	store.permissionCalls++
	return store.permissions, store.permissionsError
}

// ListManagedPermissions 返回测试预设的平台代管租户权限。
func (store *testEmployeeStore) ListManagedPermissions(_ context.Context, _ []uint64, superAdmin bool) ([]string, error) {
	store.permissionCalls++
	store.managedSuperAdmin = superAdmin
	return store.managedPermissions, store.permissionsError
}

// FindPlatformBrand 返回测试预设的平台品牌信息。
func (store *testEmployeeStore) FindPlatformBrand(_ context.Context) (*PlatformBrand, error) {
	return store.platformBrand, store.platformBrandError
}

// ListNavigationMenus 返回测试预设的当前工作空间菜单。
func (store *testEmployeeStore) ListNavigationMenus(_ context.Context, _ string) ([]NavigationMenu, error) {
	return store.navigationMenus, store.navigationMenusError
}

// newTestHandler 创建关闭验证码的测试认证处理器。
func newTestHandler(store *testEmployeeStore) *Handler {
	captcha := &CaptchaManager{enabled: false}
	tokens := NewTokenManager(strings.Repeat("s", minimumJWTBytes))
	tokens.now = func() time.Time { return tokenTestNow }
	return NewHandlerWithServices(NewService(captcha, tokens, store), NewAuthorizationService(tokens, store))
}

// newTestRouter 只注册认证测试实际使用的路由，避免测试依赖完整业务路由表。
func newTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/admin")
	admin.POST("/auth/login", handler.Login)

	protected := admin.Group("")
	protected.Use(handler.Authenticate)
	protected.GET("/me", handler.CurrentUser)
	protected.PUT("/profile/basic", handler.UpdateBasicProfile)
	protected.PUT("/profile/password", handler.ChangePassword)

	platform := protected.Group("/platform")
	platform.GET("/logs/system/filter-options", handler.RequirePermission("platform", "platform:system-log:view"), noContentHandler)
	platform.GET("/logs/operations/filter-options", handler.RequirePermission("platform", "platform:audit-log:view"), noContentHandler)
	platform.GET("/logs/login/filter-options", handler.RequirePermission("platform", "platform:login-log:view"), noContentHandler)
	platform.GET("/dictionaries", handler.RequirePermission("platform", "platform:field:view"), noContentHandler)
	platform.GET("/employees", handler.RequirePermission("platform", "platform:employee:view"), noContentHandler)
	platform.GET("/roles/:roleId/employees", handler.RequirePermission("platform", "platform:role:view"), handler.RequirePermission("platform", "platform:role:employees"), noContentHandler)
	platform.GET("/users/tenant-options", handler.RequirePermission("platform", "platform:user:view"), noContentHandler)
	platform.GET("/menus", handler.RequirePermission("platform", "platform:menu:view"), noContentHandler)
	platform.POST("/menus", handler.RequirePermission("platform", "platform:menu:view"), handler.RequirePermission("platform", "platform:menu:create"), noContentHandler)
	platform.GET("/departments", handler.RequirePermission("platform", "platform:department:view"), noContentHandler)
	platform.GET("/settings/miniapp", handler.RequirePermission("platform", "platform:miniapp:view"), noContentHandler)
	platform.PUT("/settings/miniapp", handler.RequirePermission("platform", "platform:miniapp:view"), handler.RequirePermission("platform", "platform:miniapp:edit"), noContentHandler)
	platform.GET("/settings/basic", handler.RequirePermission("platform", "platform:basic:view"), noContentHandler)
	platform.PUT("/settings/basic", handler.RequirePermission("platform", "platform:basic:view"), handler.RequirePermission("platform", "platform:basic:edit"), noContentHandler)
	tenant := protected.Group("/tenant")
	tenant.GET("/logs/login/filter-options", handler.RequirePermission("tenant", "tenant:login-log:view"), noContentHandler)
	return router
}

// recordingLoginStore 保存登录日志测试收到的脱敏事件，并可模拟数据库失败。
type recordingLoginStore struct {
	entries []logging.LoginLog
	err     error
}

// RecordLogin 记录测试登录事件，不接触真实数据库。
func (store *recordingLoginStore) RecordLogin(_ context.Context, entry logging.LoginLog) error {
	store.entries = append(store.entries, entry)
	return store.err
}

// recordingAuditStore 保存个人资料接口测试产生的请求和操作审计日志。
type recordingAuditStore struct {
	requests []logging.RequestLog
	audits   []logging.AuditLog
}

// CreateRequest 记录测试请求日志。
func (store *recordingAuditStore) CreateRequest(_ context.Context, entry logging.RequestLog) error {
	store.requests = append(store.requests, entry)
	return nil
}

// CaptureAuditSnapshot 返回空的操作前快照，供个人资料审计测试使用。
func (store *recordingAuditStore) CaptureAuditSnapshot(_ context.Context, _ string, _ map[string]string, _ logging.Actor) (logging.AuditSnapshot, error) {
	return logging.AuditSnapshot{Values: map[string]any{}}, nil
}

// RecordAudit 记录测试操作审计日志。
func (store *recordingAuditStore) RecordAudit(_ context.Context, entry logging.AuditLog) error {
	store.audits = append(store.audits, entry)
	return nil
}

// noContentHandler 为认证测试中已经通过权限校验的业务接口返回空响应。
func noContentHandler(context *gin.Context) {
	context.Status(http.StatusNoContent)
}

// TestPlatformLogFilterOptionsReuseListPermissions 验证平台日志筛选选项复用对应列表权限。
func TestPlatformLogFilterOptionsReuseListPermissions(t *testing.T) {
	employee := &Employee{ID: 8, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{idEmployee: employee, roles: []Role{{ID: 1002, Name: "平台管理员"}}}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	store.permissions = []string{"platform:audit-log:view"}
	auditRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/logs/operations/filter-options", "", rawToken)
	if auditRecorder.Code != http.StatusNoContent {
		t.Fatalf("操作日志筛选选项权限响应 = %d %s", auditRecorder.Code, auditRecorder.Body.String())
	}
	systemRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/logs/system/filter-options", "", rawToken)
	if systemRecorder.Code != http.StatusForbidden {
		t.Fatalf("普通员工系统日志筛选选项响应 = %d %s", systemRecorder.Code, systemRecorder.Body.String())
	}
	store.permissions = []string{"platform:system-log:view"}
	systemRecorder = performJSONRequest(router, http.MethodGet, "/api/admin/platform/logs/system/filter-options", "", rawToken)
	if systemRecorder.Code != http.StatusNoContent {
		t.Fatalf("具有权限的普通员工系统日志筛选选项响应 = %d %s", systemRecorder.Code, systemRecorder.Body.String())
	}
	store.permissions = []string{"platform:login-log:view"}
	loginRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/logs/login/filter-options", "", rawToken)
	if loginRecorder.Code != http.StatusNoContent {
		t.Fatalf("具有权限的普通员工登录日志筛选选项响应 = %d %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	superKey := "platform_super_admin"
	store.roles = []Role{{ID: 1001, Name: "平台超级管理员", SystemKey: &superKey}}
	systemRecorder = performJSONRequest(router, http.MethodGet, "/api/admin/platform/logs/system/filter-options", "", rawToken)
	if systemRecorder.Code != http.StatusNoContent {
		t.Fatalf("超级管理员系统日志筛选选项响应 = %d %s", systemRecorder.Code, systemRecorder.Body.String())
	}
}

// TestTenantLoginLogPermissionRequiresCurrentTenantGrant 验证租户登录日志页面严格要求新增查看权限。
func TestTenantLoginLogPermissionRequiresCurrentTenantGrant(t *testing.T) {
	tenantID := uint64(9)
	employee := &Employee{ID: 3, Scope: "tenant", TenantID: &tenantID, Name: "租户员工", Status: 1}
	store := &testEmployeeStore{
		idEmployee: employee,
		tenant:     &Tenant{ID: tenantID, Name: "测试租户", Status: 1},
		roles:      []Role{{ID: 9, Name: "租户角色"}},
	}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	token := issueTestToken(t, handler, employee)

	forbidden := performJSONRequest(router, http.MethodGet, "/api/admin/tenant/logs/login/filter-options", "", token)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("无登录日志权限响应 = %d %s", forbidden.Code, forbidden.Body.String())
	}
	store.permissions = []string{"tenant:login-log:view"}
	allowed := performJSONRequest(router, http.MethodGet, "/api/admin/tenant/logs/login/filter-options", "", token)
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("有登录日志权限响应 = %d %s", allowed.Code, allowed.Body.String())
	}
}

// performJSONRequest 执行 JSON HTTP 请求并返回记录器。
func performJSONRequest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// issueTestToken 签发测试 Token，并将同一会话标识设为员工当前有效会话。
func issueTestToken(t *testing.T, handler *Handler, employee *Employee) string {
	t.Helper()
	issued, err := handler.service.tokens.Issue(*employee)
	if err != nil {
		t.Fatalf("Issue() 返回错误: %v", err)
	}
	employee.ActiveSessionID = &issued.SessionID
	return issued.AccessToken
}

// TestLoginValidationAndCaptcha 验证无效 JSON、缺失验证码和错误验证码响应。
func TestLoginValidationAndCaptcha(t *testing.T) {
	store := &testEmployeeStore{}
	handler := newTestHandler(store)
	loginLogs := &recordingLoginStore{}
	handler.ConfigureLoginSecurity(nil, loginLogs)
	router := newTestRouter(handler)

	recorder := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", "{", "")
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10001`) {
		t.Fatalf("无效 JSON 响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	memoryStore := &memoryCaptchaStore{values: map[string]string{captchaKeyPrefix + "captcha-id": "1234"}, available: true}
	handler.service.captcha = newMemoryCaptchaManager(memoryStore)
	recorder = performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"secret"}`, "")
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10001`) {
		t.Fatalf("缺少验证码响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"secret","captchaId":"captcha-id","captchaCode":"9999"}`, "")
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10002`) {
		t.Fatalf("错误验证码响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if len(loginLogs.entries) != 1 || !strings.Contains(string(loginLogs.entries[0].Metadata), `"reason":"captcha_invalid"`) {
		t.Fatalf("验证码错误登录日志 = %#v", loginLogs.entries)
	}
}

// TestLoginCredentialsDisabledAndSuccess 验证统一凭证错误、禁用账号与成功登录。
func TestLoginCredentialsDisabledAndSuccess(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() 返回错误: %v", err)
	}
	store := &testEmployeeStore{}
	handler := newTestHandler(store)
	loginLogs := &recordingLoginStore{}
	handler.ConfigureLoginSecurity(nil, loginLogs)
	router := newTestRouter(handler)
	requestBody := `{"username":"admin","password":"secret123"}`

	unknownRecorder := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", requestBody, "")
	if unknownRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("未知账号响应 = %d %s", unknownRecorder.Code, unknownRecorder.Body.String())
	}
	store.loginEmployee = &Employee{ID: 1, Scope: "platform", PasswordHash: string(passwordHash), Status: 1}
	wrongRecorder := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"wrong"}`, "")
	if wrongRecorder.Code != http.StatusUnauthorized || wrongRecorder.Body.String() != unknownRecorder.Body.String() {
		t.Fatalf("凭证错误响应不统一: unknown=%s wrong=%s", unknownRecorder.Body.String(), wrongRecorder.Body.String())
	}

	store.loginEmployee.Status = 0
	disabledRecorder := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", requestBody, "")
	if disabledRecorder.Code != http.StatusForbidden || !strings.Contains(disabledRecorder.Body.String(), `"code":20003`) {
		t.Fatalf("禁用账号响应 = %d %s", disabledRecorder.Code, disabledRecorder.Body.String())
	}

	store.loginEmployee.Status = 1
	successRecorder := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", requestBody, "")
	if successRecorder.Code != http.StatusOK {
		t.Fatalf("登录成功响应 = %d %s", successRecorder.Code, successRecorder.Body.String())
	}
	var response struct {
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    loginResponse `json:"data"`
	}
	if err := json.Unmarshal(successRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析登录响应失败: %v", err)
	}
	if response.Code != 0 || response.Message != "成功" || response.Data.AccessToken == "" || response.Data.ExpiresAt == "" {
		t.Fatalf("登录成功响应字段缺失: %#v", response)
	}
	if !validSessionID(store.activatedSessionID) {
		t.Fatalf("登录未激活有效会话: %q", store.activatedSessionID)
	}
	for _, reason := range []string{"credentials_invalid", "credentials_invalid", "account_disabled", "success"} {
		matched := false
		for _, entry := range loginLogs.entries {
			if strings.Contains(string(entry.Metadata), `"reason":"`+reason+`"`) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("缺少登录结果日志 %s: %#v", reason, loginLogs.entries)
		}
	}
}

// TestLoginDoesNotReturnTokenWhenSessionActivationFails 验证会话写入失败时登录不会返回 Token。
func TestLoginDoesNotReturnTokenWhenSessionActivationFails(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() 返回错误: %v", err)
	}
	store := &testEmployeeStore{
		loginEmployee: &Employee{ID: 1, Scope: "platform", PasswordHash: string(passwordHash), Status: 1},
		activateError: context.DeadlineExceeded,
	}
	recorder := performJSONRequest(newTestRouter(newTestHandler(store)), http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"secret123"}`, "")
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "accessToken") {
		t.Fatalf("会话写入失败响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestSecondLoginInvalidatesFirstToken 验证同一员工第二次登录后只有最新 Token 有效。
func TestSecondLoginInvalidatesFirstToken(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() 返回错误: %v", err)
	}
	employee := &Employee{ID: 1, Scope: "platform", Name: "平台员工", PasswordHash: string(passwordHash), Status: 1}
	store := &testEmployeeStore{loginEmployee: employee, idEmployee: employee}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	loginBody := `{"username":"admin","password":"secret123"}`

	firstLogin := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", loginBody, "")
	var firstResponse struct {
		Data loginResponse `json:"data"`
	}
	if firstLogin.Code != http.StatusOK || json.Unmarshal(firstLogin.Body.Bytes(), &firstResponse) != nil {
		t.Fatalf("第一次登录响应 = %d %s", firstLogin.Code, firstLogin.Body.String())
	}
	secondLogin := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", loginBody, "")
	var secondResponse struct {
		Data loginResponse `json:"data"`
	}
	if secondLogin.Code != http.StatusOK || json.Unmarshal(secondLogin.Body.Bytes(), &secondResponse) != nil {
		t.Fatalf("第二次登录响应 = %d %s", secondLogin.Code, secondLogin.Body.String())
	}
	if firstResponse.Data.AccessToken == secondResponse.Data.AccessToken {
		t.Fatal("两次登录不应签发相同 Token")
	}

	oldRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/me", "", firstResponse.Data.AccessToken)
	if oldRecorder.Code != http.StatusUnauthorized || !strings.Contains(oldRecorder.Body.String(), `"code":20001`) {
		t.Fatalf("旧 Token 响应 = %d %s", oldRecorder.Code, oldRecorder.Body.String())
	}
	newRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/me", "", secondResponse.Data.AccessToken)
	if newRecorder.Code != http.StatusOK {
		t.Fatalf("新 Token 响应 = %d %s", newRecorder.Code, newRecorder.Body.String())
	}
}

// TestChangePasswordValidatesCurrentPasswordAndInvalidatesSessions 验证原密码错误使用 400，成功后普通和代管 Token 全部失效。
func TestChangePasswordValidatesCurrentPasswordAndInvalidatesSessions(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() 返回错误: %v", err)
	}
	tenantID := uint64(9)
	employee := &Employee{ID: 1, Scope: "platform", Name: "平台员工", LoginAccount: "admin", PasswordHash: string(passwordHash), Status: 1}
	store := &testEmployeeStore{
		idEmployee:  employee,
		tenant:      &Tenant{ID: tenantID, Name: "测试租户", Status: 1},
		roles:       []Role{{ID: 8, Name: "运营角色"}},
		permissions: []string{"platform:tenant:enter"},
	}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	normalToken := issueTestToken(t, handler, employee)
	identity, err := handler.service.tokens.Parse(normalToken)
	if err != nil {
		t.Fatalf("Parse() 返回错误: %v", err)
	}
	managed, err := handler.service.tokens.IssueManaged(employee.ID, tenantID, identity.SessionID, identity.ExpiresAt)
	if err != nil {
		t.Fatalf("IssueManaged() 返回错误: %v", err)
	}

	wrong := performJSONRequest(router, http.MethodPut, "/api/admin/profile/password", `{"currentPassword":"wrong","newPassword":"newpass123"}`, normalToken)
	if wrong.Code != http.StatusBadRequest || !strings.Contains(wrong.Body.String(), `"code":10005`) || store.changedPasswordHash != "" {
		t.Fatalf("原密码错误响应 = %d %s", wrong.Code, wrong.Body.String())
	}

	success := performJSONRequest(router, http.MethodPut, "/api/admin/profile/password", `{"currentPassword":"secret123","newPassword":"新密码1234"}`, normalToken)
	if success.Code != http.StatusOK || store.changedPasswordHash == "" {
		t.Fatalf("密码修改响应 = %d %s", success.Code, success.Body.String())
	}
	if bcrypt.CompareHashAndPassword([]byte(store.changedPasswordHash), []byte("新密码1234")) != nil {
		t.Fatal("保存的新密码哈希无法验证")
	}
	for name, token := range map[string]string{"normal": normalToken, "managed": managed.AccessToken} {
		recorder := performJSONRequest(router, http.MethodGet, "/api/admin/me", "", token)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s Token 修改密码后响应 = %d %s", name, recorder.Code, recorder.Body.String())
		}
	}
}

// TestValidProfilePasswordUsesUnicodeLength 验证新密码按 Unicode 字符数接受六至十八位边界。
func TestValidProfilePasswordUsesUnicodeLength(t *testing.T) {
	for value, expected := range map[string]bool{
		"12345":                 false,
		"123456":                true,
		strings.Repeat("密", 18): true,
		strings.Repeat("密", 19): false,
	} {
		if actual := validProfilePassword(value); actual != expected {
			t.Fatalf("validProfilePassword(%q) = %v，期望 %v", value, actual, expected)
		}
	}
}

// TestUpdateBasicProfileValidatesAndNormalizesFields 验证姓名字段被忽略、手机号清空和指定格式规则。
func TestUpdateBasicProfileValidatesAndNormalizesFields(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCode  int
		wantPhone *string
	}{
		{name: "清空手机号且忽略姓名", body: `{"name":"  新姓名  ","phone":"  "}`, wantCode: http.StatusOK},
		{name: "兼容缺少姓名", body: `{"phone":"+8613812345678"}`, wantCode: http.StatusOK, wantPhone: stringPointerForTest("+8613812345678")},
		{name: "忽略空姓名", body: `{"name":"  ","phone":null}`, wantCode: http.StatusOK},
		{name: "忽略超长姓名", body: `{"name":"` + strings.Repeat("名", 31) + `","phone":null}`, wantCode: http.StatusOK},
		{name: "手机号格式错误", body: `{"name":"平台员工","phone":"138-1234-5678"}`, wantCode: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			employee := &Employee{ID: 27, Scope: "platform", Name: "原姓名", LoginAccount: "admin", Status: 1}
			store := &testEmployeeStore{idEmployee: employee}
			handler := newTestHandler(store)
			token := issueTestToken(t, handler, employee)
			recorder := performJSONRequest(newTestRouter(handler), http.MethodPut, "/api/admin/profile/basic", test.body, token)
			if recorder.Code != test.wantCode {
				t.Fatalf("资料更新响应 = %d %s，期望 %d", recorder.Code, recorder.Body.String(), test.wantCode)
			}
			if test.wantCode != http.StatusOK {
				if store.updatedEmployeeID != 0 {
					t.Fatalf("非法资料仍执行了更新: %#v", store)
				}
				return
			}
			if employee.Name != "原姓名" {
				t.Fatalf("姓名不应被接口修改: %q", employee.Name)
			}
			if store.updatedEmployeeID != employee.ID || !sameOptionalString(store.updatedPhone, test.wantPhone) {
				t.Fatalf("资料更新参数错误: employeeID=%d phone=%v", store.updatedEmployeeID, store.updatedPhone)
			}
		})
	}
}

// TestUpdateBasicProfileUsesRealEmployeeDuringManagedSession 验证代管 Token 仍只更新真实平台员工本人的手机号并允许重复保存。
func TestUpdateBasicProfileUsesRealEmployeeDuringManagedSession(t *testing.T) {
	tenantID := uint64(9)
	employee := &Employee{ID: 42, Scope: "platform", Name: "平台员工", LoginAccount: "admin", Status: 1}
	store := &testEmployeeStore{
		idEmployee: employee,
		tenant:     &Tenant{ID: tenantID, Name: "测试租户", Status: 1},
		roles:      []Role{{ID: 8, Name: "运营角色"}},
		permissions: []string{
			"platform:tenant:enter",
		},
	}
	handler := newTestHandler(store)
	normalToken := issueTestToken(t, handler, employee)
	identity, err := handler.service.tokens.Parse(normalToken)
	if err != nil {
		t.Fatalf("Parse() 返回错误: %v", err)
	}
	managed, err := handler.service.tokens.IssueManaged(employee.ID, tenantID, identity.SessionID, identity.ExpiresAt)
	if err != nil {
		t.Fatalf("IssueManaged() 返回错误: %v", err)
	}
	for range 2 {
		recorder := performJSONRequest(newTestRouter(handler), http.MethodPut, "/api/admin/profile/basic", `{"name":"新姓名","phone":"008613812345678"}`, managed.AccessToken)
		if recorder.Code != http.StatusOK || store.updatedEmployeeID != employee.ID {
			t.Fatalf("代管会话资料更新响应 = %d %s，员工 ID=%d", recorder.Code, recorder.Body.String(), store.updatedEmployeeID)
		}
		if employee.Name != "平台员工" {
			t.Fatalf("代管会话不应修改姓名: %q", employee.Name)
		}
	}

	store.updateProfileError = context.DeadlineExceeded
	recorder := performJSONRequest(newTestRouter(handler), http.MethodPut, "/api/admin/profile/basic", `{"name":"再次修改","phone":null}`, managed.AccessToken)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("资料更新数据库失败响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestUpdateBasicProfileWritesSanitizedAudit 验证个人资料成功修改生成受控的手机号变更审计。
func TestUpdateBasicProfileWritesSanitizedAudit(t *testing.T) {
	phone := "13812345678"
	employee := &Employee{ID: 7, Scope: "platform", Name: "原姓名", LoginAccount: "admin", Phone: &phone, Status: 1}
	store := &testEmployeeStore{idEmployee: employee}
	handler := newTestHandler(store)
	token := issueTestToken(t, handler, employee)
	logs := &recordingAuditStore{}
	router := gin.New()
	router.Use(logging.Middleware(logs, logging.RequestLogModeMutationAndError))
	protected := router.Group("/api/admin")
	protected.Use(handler.Authenticate, logging.AuditMiddleware(logs))
	protected.PUT("/profile/basic", handler.UpdateBasicProfile)

	recorder := performJSONRequest(router, http.MethodPut, "/api/admin/profile/basic", `{"name":"新姓名","phone":"+8613912345678"}`, token)
	if recorder.Code != http.StatusOK || len(logs.audits) != 1 {
		t.Fatalf("资料更新审计响应=%d audits=%d body=%s", recorder.Code, len(logs.audits), recorder.Body.String())
	}
	audit := logs.audits[0]
	if audit.ModuleCode != "profile" || audit.ActionCode != "update" || audit.Summary != "基本资料已修改" || audit.TargetID == nil || *audit.TargetID != "7" || audit.TargetName == nil || *audit.TargetName != "原姓名" {
		t.Fatalf("资料更新审计元数据错误: %#v", audit)
	}
	var changes map[string]map[string]any
	if json.Unmarshal(audit.Changes, &changes) != nil || changes["name"] != nil || changes["phone"]["after"] != "+8613912345678" {
		t.Fatalf("资料更新审计变更错误: %s", audit.Changes)
	}
}

// stringPointerForTest 创建只供表驱动测试比较使用的字符串指针。
func stringPointerForTest(value string) *string {
	return &value
}

// TestLoginLogFailureDoesNotChangeAuthenticationResult 验证登录日志写入失败不覆盖原登录结果。
func TestLoginLogFailureDoesNotChangeAuthenticationResult(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() 返回错误: %v", err)
	}
	employee := &Employee{ID: 1, Scope: "platform", Name: "平台员工", LoginAccount: "admin", PasswordHash: string(passwordHash), Status: 1}
	store := &testEmployeeStore{loginEmployee: employee}
	handler := newTestHandler(store)
	recorderStore := &recordingLoginStore{err: context.DeadlineExceeded}
	handler.ConfigureLoginSecurity(nil, recorderStore)
	recorder := performJSONRequest(newTestRouter(handler), http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"secret123"}`, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("登录日志失败改变了认证结果: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(recorderStore.entries) != 1 || recorderStore.entries[0].ActorID == nil || recorderStore.entries[0].ActorAccount != "admin" {
		t.Fatalf("登录日志内容错误: %#v", recorderStore.entries)
	}
}

// TestAuthenticationRejectsMissingAndInvalidBearer 验证受保护接口拒绝缺少或非法 Token。
func TestAuthenticationRejectsMissingAndInvalidBearer(t *testing.T) {
	handler := newTestHandler(&testEmployeeStore{})
	router := newTestRouter(handler)

	for _, token := range []string{"", "invalid-token"} {
		recorder := performJSONRequest(router, http.MethodGet, "/api/admin/me", "", token)
		if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), `"code":20001`) {
			t.Fatalf("Token %q 响应 = %d %s", token, recorder.Code, recorder.Body.String())
		}
	}
}

// TestCurrentUserSuperAdminAndOrdinaryPermissions 验证超级管理员放行与普通员工实时权限响应。
func TestCurrentUserSuperAdminAndOrdinaryPermissions(t *testing.T) {
	superKey := "platform_super_admin"
	adminKey := "platform_admin"
	employee := &Employee{ID: 1, Scope: "platform", Name: "平台所有者", Status: 1}
	store := &testEmployeeStore{idEmployee: employee}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	store.roles = []Role{{ID: 1001, Name: "平台超级管理员", SystemKey: &superKey}}
	superRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/me", "", rawToken)
	if superRecorder.Code != http.StatusOK || !strings.Contains(superRecorder.Body.String(), `"isSuperAdmin":true`) || !strings.Contains(superRecorder.Body.String(), `"permissions":[]`) {
		t.Fatalf("超级管理员响应 = %d %s", superRecorder.Code, superRecorder.Body.String())
	}
	if store.permissionCalls != 0 {
		t.Fatalf("超级管理员不应查询权限，调用次数 = %d", store.permissionCalls)
	}

	store.roles = []Role{{ID: 1002, Name: "平台管理员", SystemKey: &adminKey}}
	store.permissions = []string{"platform:tenant:view", "platform:menu:view", "platform:field:view", "platform:miniapp:view", "platform:system-log:view"}
	ordinaryRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/me", "", rawToken)
	if ordinaryRecorder.Code != http.StatusOK || !strings.Contains(ordinaryRecorder.Body.String(), `"isSuperAdmin":false`) || !strings.Contains(ordinaryRecorder.Body.String(), "platform:tenant:view") || !strings.Contains(ordinaryRecorder.Body.String(), "platform:menu:view") || !strings.Contains(ordinaryRecorder.Body.String(), "platform:field:view") || !strings.Contains(ordinaryRecorder.Body.String(), "platform:miniapp:view") || !strings.Contains(ordinaryRecorder.Body.String(), "platform:system-log:view") {
		t.Fatalf("普通员工响应 = %d %s", ordinaryRecorder.Code, ordinaryRecorder.Body.String())
	}
	if store.permissionCalls != 1 {
		t.Fatalf("普通员工权限查询次数 = %d，期望 1", store.permissionCalls)
	}
}

// TestCurrentTenantOwnerUsesStoredPermissions 验证企业管理员不再动态获得全部租户权限。
func TestCurrentTenantOwnerUsesStoredPermissions(t *testing.T) {
	tenantID := uint64(9)
	ownerKey := "tenant_owner"
	employee := &Employee{ID: 3, Scope: "tenant", TenantID: &tenantID, Name: "企业管理员", Status: 1}
	store := &testEmployeeStore{
		idEmployee:  employee,
		tenant:      &Tenant{ID: tenantID, Name: "测试租户", Status: 1},
		roles:       []Role{{ID: 1003, Name: "企业管理员", SystemKey: &ownerKey}},
		permissions: []string{"tenant:employee:view"},
	}
	handler := newTestHandler(store)
	recorder := performJSONRequest(newTestRouter(handler), http.MethodGet, "/api/admin/me", "", issueTestToken(t, handler, employee))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"tenant:employee:view"`) || strings.Contains(recorder.Body.String(), `"tenant:menu:view"`) {
		t.Fatalf("企业管理员实时权限响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if store.permissionCalls != 1 {
		t.Fatalf("企业管理员应查询一次存储权限，调用次数 = %d", store.permissionCalls)
	}
}

// TestRequirePermissionSuperAdminAndOrdinary 验证平台超级管理员和具有实时权限的普通员工可以访问平台接口。
func TestRequirePermissionSuperAdminAndOrdinary(t *testing.T) {
	superKey := "platform_super_admin"
	adminKey := "platform_admin"
	employee := &Employee{ID: 1, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{idEmployee: employee}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	store.roles = []Role{{ID: 1001, Name: "平台超级管理员", SystemKey: &superKey}}
	superRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/employees", "", rawToken)
	if superRecorder.Code != http.StatusNoContent {
		t.Fatalf("超级管理员响应 = %d %s", superRecorder.Code, superRecorder.Body.String())
	}
	if store.permissionCalls != 0 {
		t.Fatalf("超级管理员不应查询权限，调用次数 = %d", store.permissionCalls)
	}

	store.roles = []Role{{ID: 1002, Name: "平台管理员", SystemKey: &adminKey}}
	store.permissions = []string{"platform:employee:view"}
	ordinaryRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/employees", "", rawToken)
	if ordinaryRecorder.Code != http.StatusNoContent {
		t.Fatalf("普通员工响应 = %d %s", ordinaryRecorder.Code, ordinaryRecorder.Body.String())
	}
	if store.permissionCalls != 1 {
		t.Fatalf("普通员工权限查询次数 = %d，期望 1", store.permissionCalls)
	}
}

// TestRequirePermissionRejectsForbiddenAndStoreError 验证权限缺失、工作空间错误和权限查询失败的响应。
func TestRequirePermissionRejectsForbiddenAndStoreError(t *testing.T) {
	employee := &Employee{ID: 2, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{idEmployee: employee, roles: []Role{{ID: 1002, Name: "平台管理员"}}}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	forbiddenRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/employees", "", rawToken)
	if forbiddenRecorder.Code != http.StatusForbidden || !strings.Contains(forbiddenRecorder.Body.String(), `"code":30001`) {
		t.Fatalf("权限缺失响应 = %d %s", forbiddenRecorder.Code, forbiddenRecorder.Body.String())
	}

	store.permissionsError = context.DeadlineExceeded
	errorRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/employees", "", rawToken)
	if errorRecorder.Code != http.StatusInternalServerError || !strings.Contains(errorRecorder.Body.String(), `"code":50000`) {
		t.Fatalf("权限查询失败响应 = %d %s", errorRecorder.Code, errorRecorder.Body.String())
	}

	tenantID := uint64(9)
	tenantEmployee := &Employee{ID: 3, Scope: "tenant", TenantID: &tenantID, Name: "租户员工", Status: 1}
	tenantStore := &testEmployeeStore{
		idEmployee: tenantEmployee,
		tenant:     &Tenant{ID: tenantID, Name: "测试租户", Status: 1},
	}
	tenantHandler := newTestHandler(tenantStore)
	tenantRouter := newTestRouter(tenantHandler)
	tenantToken := issueTestToken(t, tenantHandler, tenantEmployee)
	workspaceRecorder := performJSONRequest(tenantRouter, http.MethodGet, "/api/admin/platform/employees", "", tenantToken)
	if workspaceRecorder.Code != http.StatusForbidden {
		t.Fatalf("工作空间错误响应 = %d %s", workspaceRecorder.Code, workspaceRecorder.Body.String())
	}
}

// TestPlatformRoleEmployeesRequiresBothPermissions 验证角色员工接口同时要求角色查看和查看员工权限。
func TestPlatformRoleEmployeesRequiresBothPermissions(t *testing.T) {
	employee := &Employee{ID: 4, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{idEmployee: employee, roles: []Role{{ID: 1002, Name: "平台管理员"}}}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	store.permissions = []string{"platform:role:view"}
	recorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/roles/1002/employees", "", rawToken)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("缺少查看员工权限响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	store.permissions = []string{"platform:role:view", "platform:role:employees"}
	recorder = performJSONRequest(router, http.MethodGet, "/api/admin/platform/roles/1002/employees", "", rawToken)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("双权限通过响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestPlatformUserTenantOptionsUsesUserViewPermission 验证租户选项只依赖用户查看权限。
func TestPlatformUserTenantOptionsUsesUserViewPermission(t *testing.T) {
	employee := &Employee{ID: 6, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{idEmployee: employee, roles: []Role{{ID: 1002, Name: "平台管理员"}}}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	store.permissions = []string{"platform:tenant:view"}
	recorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/users/tenant-options", "", rawToken)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("只有租户查看权限时响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	store.permissions = []string{"platform:user:view"}
	recorder = performJSONRequest(router, http.MethodGet, "/api/admin/platform/users/tenant-options", "", rawToken)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("用户查看权限响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestManagedSessionUsesMappedTenantPermissions 验证代管会话实时要求进入权限并只返回映射的租户权限。
func TestManagedSessionUsesMappedTenantPermissions(t *testing.T) {
	tenantID := uint64(9)
	employee := &Employee{ID: 2, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{
		idEmployee:         employee,
		tenant:             &Tenant{ID: tenantID, Name: "测试租户", Status: 1},
		roles:              []Role{{ID: 8, Name: "运营角色"}},
		permissions:        []string{"platform:tenant:enter"},
		managedPermissions: []string{"tenant:user:view"},
	}
	handler := newTestHandler(store)
	sessionID := strings.Repeat("b", 64)
	employee.ActiveSessionID = &sessionID
	issued, err := handler.service.tokens.IssueManaged(employee.ID, tenantID, sessionID, tokenTestNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	recorder := performJSONRequest(newTestRouter(handler), http.MethodGet, "/api/admin/me", "", issued.AccessToken)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"workspace":"tenant"`) || !strings.Contains(recorder.Body.String(), `"mode":"managed"`) || !strings.Contains(recorder.Body.String(), `"tenant:user:view"`) || strings.Contains(recorder.Body.String(), `platform:tenant:enter`) {
		t.Fatalf("代管会话响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	store.permissions = nil
	recorder = performJSONRequest(newTestRouter(handler), http.MethodGet, "/api/admin/me", "", issued.AccessToken)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("撤销进入权限后响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestManagedSuperAdminKeepsFullTenantPermissionMode 验证企业管理员配置不会限制平台超级管理员代管能力。
func TestManagedSuperAdminKeepsFullTenantPermissionMode(t *testing.T) {
	tenantID := uint64(9)
	superKey := "platform_super_admin"
	employee := &Employee{ID: 1, Scope: "platform", Name: "平台超级管理员", Status: 1}
	store := &testEmployeeStore{
		idEmployee:         employee,
		tenant:             &Tenant{ID: tenantID, Name: "测试租户", Status: 1},
		roles:              []Role{{ID: 1001, Name: "平台超级管理员", SystemKey: &superKey}},
		managedPermissions: []string{"tenant:role:permission"},
	}
	handler := newTestHandler(store)
	sessionID := strings.Repeat("c", 64)
	employee.ActiveSessionID = &sessionID
	issued, err := handler.service.tokens.IssueManaged(employee.ID, tenantID, sessionID, tokenTestNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	recorder := performJSONRequest(newTestRouter(handler), http.MethodGet, "/api/admin/me", "", issued.AccessToken)
	if recorder.Code != http.StatusOK || !store.managedSuperAdmin || !strings.Contains(recorder.Body.String(), `"tenant:role:permission"`) {
		t.Fatalf("平台超级管理员代管响应 = %d %s, super=%v", recorder.Code, recorder.Body.String(), store.managedSuperAdmin)
	}
}

// TestPlatformAssignablePermissions 验证原保留能力改为使用可分配权限码。
func TestPlatformAssignablePermissions(t *testing.T) {
	employee := &Employee{ID: 5, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{idEmployee: employee, roles: []Role{{ID: 1002, Name: "平台管理员"}}}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	store.permissions = []string{"platform:menu:view"}
	menuRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/menus?scope=platform", "", rawToken)
	if menuRecorder.Code != http.StatusNoContent {
		t.Fatalf("具有查看权限的普通平台管理员菜单响应 = %d %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	menuRecorder = performJSONRequest(router, http.MethodPost, "/api/admin/platform/menus", "{}", rawToken)
	if menuRecorder.Code != http.StatusForbidden {
		t.Fatalf("缺少新增权限的普通平台管理员菜单响应 = %d %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	store.permissions = []string{"platform:menu:view", "platform:menu:create"}
	menuRecorder = performJSONRequest(router, http.MethodPost, "/api/admin/platform/menus", "{}", rawToken)
	if menuRecorder.Code != http.StatusNoContent {
		t.Fatalf("具有新增权限的普通平台管理员菜单响应 = %d %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	for permission, path := range map[string]string{
		"platform:field:view":      "/api/admin/platform/dictionaries",
		"platform:miniapp:view":    "/api/admin/platform/settings/miniapp",
		"platform:system-log:view": "/api/admin/platform/logs/system/filter-options",
	} {
		store.permissions = []string{permission}
		recorder := performJSONRequest(router, http.MethodGet, path, "", rawToken)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("权限 %s 响应 = %d %s", permission, recorder.Code, recorder.Body.String())
		}
	}
	superKey := "platform_super_admin"
	store.roles = []Role{{ID: 1001, Name: "平台超级管理员", SystemKey: &superKey}}
	menuRecorder = performJSONRequest(router, http.MethodGet, "/api/admin/platform/menus?scope=platform", "", rawToken)
	if menuRecorder.Code != http.StatusNoContent {
		t.Fatalf("平台超级管理员菜单响应 = %d %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	store.roles = []Role{{ID: 1002, Name: "平台管理员"}}
	departmentRecorder := performJSONRequest(router, http.MethodGet, "/api/admin/platform/departments", "", rawToken)
	if departmentRecorder.Code != http.StatusForbidden {
		t.Fatalf("缺少部门权限响应 = %d %s", departmentRecorder.Code, departmentRecorder.Body.String())
	}

	store.permissions = []string{"platform:department:view"}
	departmentRecorder = performJSONRequest(router, http.MethodGet, "/api/admin/platform/departments", "", rawToken)
	if departmentRecorder.Code != http.StatusNoContent {
		t.Fatalf("部门权限响应 = %d %s", departmentRecorder.Code, departmentRecorder.Body.String())
	}
}

// TestPlatformSettingsViewAndEditPermissions 验证平台设置读取和保存分别使用查看与编辑权限。
func TestPlatformSettingsViewAndEditPermissions(t *testing.T) {
	employee := &Employee{ID: 5, Scope: "platform", Name: "平台员工", Status: 1}
	store := &testEmployeeStore{idEmployee: employee, roles: []Role{{ID: 1002, Name: "平台管理员"}}}
	handler := newTestHandler(store)
	router := newTestRouter(handler)
	rawToken := issueTestToken(t, handler, employee)

	for _, scenario := range []struct {
		name           string
		viewPermission string
		editPermission string
		path           string
	}{
		{
			name:           "平台基础设置",
			viewPermission: "platform:basic:view",
			editPermission: "platform:basic:edit",
			path:           "/api/admin/platform/settings/basic",
		},
		{
			name:           "微信小程序配置",
			viewPermission: "platform:miniapp:view",
			editPermission: "platform:miniapp:edit",
			path:           "/api/admin/platform/settings/miniapp",
		},
	} {
		store.permissions = []string{scenario.viewPermission}
		recorder := performJSONRequest(router, http.MethodGet, scenario.path, "", rawToken)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s 只有查看权限读取响应 = %d %s", scenario.name, recorder.Code, recorder.Body.String())
		}
		recorder = performJSONRequest(router, http.MethodPut, scenario.path, "{}", rawToken)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s 只有查看权限保存响应 = %d %s", scenario.name, recorder.Code, recorder.Body.String())
		}

		store.permissions = []string{scenario.viewPermission, scenario.editPermission}
		recorder = performJSONRequest(router, http.MethodPut, scenario.path, "{}", rawToken)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s 具有编辑权限保存响应 = %d %s", scenario.name, recorder.Code, recorder.Body.String())
		}

		store.permissions = []string{scenario.editPermission}
		recorder = performJSONRequest(router, http.MethodPut, scenario.path, "{}", rawToken)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s 缺少查看权限保存响应 = %d %s", scenario.name, recorder.Code, recorder.Body.String())
		}
	}
}
