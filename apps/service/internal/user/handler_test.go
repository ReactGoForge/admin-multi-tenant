package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type testWechatExchanger struct {
	identity   WechatIdentity
	phone      string
	err        error
	phoneError error
}

// Exchange 返回测试预设的微信身份或错误。
func (exchanger *testWechatExchanger) Exchange(_ context.Context, _, _ string) (WechatIdentity, error) {
	return exchanger.identity, exchanger.err
}

// ExchangePhone 返回测试预设的可信手机号或错误。
func (exchanger *testWechatExchanger) ExchangePhone(_ context.Context, _, _ string) (string, error) {
	return exchanger.phone, exchanger.phoneError
}

type testStore struct {
	appID               string
	session             *Session
	ensureError         error
	findError           error
	platformUsers       []PlatformUser
	platformUserTenants []PlatformUserTenant
	tenantOptions       []TenantOption
	tenantUsers         []TenantUser
	listError           error
	statusError         error
	lastTenantID        uint64
	lastUserID          uint64
	lastStatus          uint8
	lastPhone           *string
	ensureCallCount     int
}

// GetMiniappAppID 返回测试预设的小程序 AppID。
func (store *testStore) GetMiniappAppID(_ context.Context) (string, error) {
	if store.appID == "" {
		return "wx-test", nil
	}
	return store.appID, nil
}

// WithMiniappTransaction 使用测试 Store 自身模拟事务内数据能力。
func (store *testStore) WithMiniappTransaction(_ context.Context, fn func(MiniappTransactionStore) error) error {
	return fn(store)
}

// FindTenantForLogin 返回测试预设的登录租户。
func (store *testStore) FindTenantForLogin(_ context.Context, tenantID uint64) (Tenant, error) {
	store.ensureCallCount++
	store.lastTenantID = tenantID
	if store.ensureError != nil {
		return Tenant{}, store.ensureError
	}
	if store.session == nil {
		return Tenant{}, errTenantNotFound
	}
	return store.session.Tenant, nil
}

// EnsureWechatUser 返回测试预设的平台用户，并记录可信手机号。
func (store *testStore) EnsureWechatUser(_ context.Context, _ WechatIdentity, phone *string) (User, error) {
	store.lastPhone = phone
	if store.ensureError != nil {
		return User{}, store.ensureError
	}
	if store.session == nil {
		return User{}, errUserNotFound
	}
	user := store.session.User
	if phone != nil {
		user.Phone = phone
	}
	return user, nil
}

// EnsureTenantMembership 返回测试预设的租户归属。
func (store *testStore) EnsureTenantMembership(_ context.Context, tenantID, userID uint64) (TenantMembership, error) {
	store.lastTenantID, store.lastUserID = tenantID, userID
	if store.ensureError != nil {
		return TenantMembership{}, store.ensureError
	}
	if store.session == nil {
		return TenantMembership{}, errUserNotFound
	}
	return TenantMembership{Status: store.session.TenantUserStatus, JoinedAt: store.session.JoinedAt}, nil
}

// FindSession 返回测试预设的实时会话。
func (store *testStore) FindSession(_ context.Context, userID, tenantID uint64) (*Session, error) {
	store.lastUserID, store.lastTenantID = userID, tenantID
	return store.session, store.findError
}

// ListPlatformUsers 返回测试预设的平台用户列表。
func (store *testStore) ListPlatformUsers(_ context.Context, _ PlatformUserQuery) ([]PlatformUser, int64, error) {
	return store.platformUsers, int64(len(store.platformUsers)), store.listError
}

// ListPlatformUserTenants 返回测试预设的用户关联租户。
func (store *testStore) ListPlatformUserTenants(_ context.Context, _ uint64) ([]PlatformUserTenant, error) {
	return store.platformUserTenants, store.listError
}

// ListTenantOptions 返回测试预设的租户筛选选项。
func (store *testStore) ListTenantOptions(_ context.Context) ([]TenantOption, error) {
	return store.tenantOptions, store.listError
}

// SetPlatformUserStatus 记录测试中的平台用户状态更新。
func (store *testStore) SetPlatformUserStatus(_ context.Context, userID uint64, status uint8) error {
	store.lastUserID, store.lastStatus = userID, status
	return store.statusError
}

// ListTenantUsers 返回测试预设的租户用户列表。
func (store *testStore) ListTenantUsers(_ context.Context, tenantID uint64, _ TenantUserQuery) ([]TenantUser, int64, error) {
	store.lastTenantID = tenantID
	return store.tenantUsers, int64(len(store.tenantUsers)), store.listError
}

// SetTenantUserStatus 记录测试中的租户用户状态更新。
func (store *testStore) SetTenantUserStatus(_ context.Context, tenantID, userID uint64, status uint8) error {
	store.lastTenantID, store.lastUserID, store.lastStatus = tenantID, userID, status
	return store.statusError
}

// newMiniappTestRouter 创建只包含小程序登录和当前用户接口的测试路由。
func newMiniappTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/miniapp/auth/login", handler.Login)
	protected := router.Group("/api/miniapp")
	protected.Use(handler.Authenticate)
	protected.GET("/me", handler.Current)
	return router
}

// newTestMiniappHandler 使用真实 Service 和测试依赖创建小程序 Handler。
func newTestMiniappHandler(exchanger WechatExchanger, tokens *TokenManager, store *testStore) *Handler {
	return NewHandler(NewMiniappService(store, exchanger, tokens))
}

// performUserRequest 执行用户模块 JSON 请求。
func performUserRequest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestMiniappLoginValidationAndSuccess 验证场景校验、事务调用和成功响应不暴露 OpenID。
func TestMiniappLoginValidationAndSuccess(t *testing.T) {
	store := &testStore{session: &Session{User: User{ID: 10, WechatOpenID: "sensitive-openid", Status: 1}, Tenant: Tenant{ID: 20, Name: "测试租户", Status: 1}, TenantUserStatus: 1}}
	exchanger := &testWechatExchanger{identity: WechatIdentity{OpenID: "sensitive-openid"}}
	tokens := NewTokenManager(strings.Repeat("s", 32))
	tokens.now = func() time.Time { return userTokenTestNow }
	router := newMiniappTestRouter(newTestMiniappHandler(exchanger, tokens, store))

	invalid := performUserRequest(router, http.MethodPost, "/api/miniapp/auth/login", `{"code":"code","scene":"invalid"}`, "")
	if invalid.Code != http.StatusBadRequest || store.ensureCallCount != 0 {
		t.Fatalf("非法场景响应 = %d %s", invalid.Code, invalid.Body.String())
	}
	success := performUserRequest(router, http.MethodPost, "/api/miniapp/auth/login", `{"code":"code","scene":"20"}`, "")
	if success.Code != http.StatusOK || store.lastTenantID != 20 || strings.Contains(success.Body.String(), "sensitive-openid") {
		t.Fatalf("登录响应 = %d %s", success.Code, success.Body.String())
	}
	var response struct {
		Data loginResponse `json:"data"`
	}
	if err := json.Unmarshal(success.Body.Bytes(), &response); err != nil || response.Data.AccessToken == "" || response.Data.User.ID != "10" {
		t.Fatalf("登录响应解析失败: %#v, %v", response, err)
	}
}

// TestMiniappPhoneLoginBindsTrustedPhone 验证手机号登录只把微信适配器返回的可信号码传入事务。
func TestMiniappPhoneLoginBindsTrustedPhone(t *testing.T) {
	store := &testStore{session: &Session{User: User{ID: 10, WechatOpenID: "openid", Status: 1}, Tenant: Tenant{ID: 20, Name: "测试租户", Status: 1}, TenantUserStatus: 1}}
	exchanger := &testWechatExchanger{identity: WechatIdentity{OpenID: "openid"}, phone: "13800138000"}
	router := newMiniappTestRouter(newTestMiniappHandler(exchanger, NewTokenManager(strings.Repeat("s", 32)), store))

	recorder := performUserRequest(router, http.MethodPost, "/api/miniapp/auth/login", `{"code":"login-code","phoneCode":"phone-code","scene":"20"}`, "")
	if recorder.Code != http.StatusOK || store.lastPhone == nil || *store.lastPhone != "13800138000" {
		t.Fatalf("手机号登录响应 = %d %s，事务手机号 = %#v", recorder.Code, recorder.Body.String(), store.lastPhone)
	}
	if !strings.Contains(recorder.Body.String(), `"phone":"13800138000"`) || strings.Contains(recorder.Body.String(), "phone-code") {
		t.Fatalf("手机号登录响应字段不正确: %s", recorder.Body.String())
	}
}

// TestMiniappLoginMapsKnownErrors 验证微信、用户和租户状态错误映射为稳定响应。
func TestMiniappLoginMapsKnownErrors(t *testing.T) {
	tests := []struct {
		name          string
		exchangeError error
		phoneError    error
		requestBody   string
		storeError    error
		status        int
		code          string
	}{
		{name: "微信凭证无效", exchangeError: errWechatCodeInvalid, status: http.StatusUnauthorized, code: `"code":20004`},
		{name: "手机号凭证无效", phoneError: errWechatCodeInvalid, requestBody: `{"code":"code","phoneCode":"phone-code","scene":"1"}`, status: http.StatusUnauthorized, code: `"code":20004`},
		{name: "微信服务不可用", exchangeError: errWechatUnavailable, status: http.StatusServiceUnavailable, code: `"code":50001`},
		{name: "手机号冲突", storeError: errIdentityConflict, status: http.StatusConflict, code: `"code":40003`},
		{name: "平台用户禁用", storeError: errUserDisabled, status: http.StatusForbidden, code: `"code":20005`},
		{name: "租户用户禁用", storeError: errTenantUserDisabled, status: http.StatusForbidden, code: `"code":20006`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &testStore{ensureError: test.storeError}
			exchanger := &testWechatExchanger{identity: WechatIdentity{OpenID: "openid"}, phone: "13800138000", err: test.exchangeError, phoneError: test.phoneError}
			router := newMiniappTestRouter(newTestMiniappHandler(exchanger, NewTokenManager(strings.Repeat("s", 32)), store))
			body := test.requestBody
			if body == "" {
				body = `{"code":"code","scene":"1"}`
			}
			recorder := performUserRequest(router, http.MethodPost, "/api/miniapp/auth/login", body, "")
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
				t.Fatalf("响应 = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestMiniappAuthenticationChecksRealtimeStatus 验证 Token 后仍实时检查三层状态。
func TestMiniappAuthenticationChecksRealtimeStatus(t *testing.T) {
	base := Session{User: User{ID: 1, Status: 1}, Tenant: Tenant{ID: 2, Name: "租户", Status: 1}, TenantUserStatus: 1}
	tokens := NewTokenManager(strings.Repeat("s", 32))
	tokens.now = func() time.Time { return userTokenTestNow }
	rawToken, _, err := tokens.Issue(1, 2)
	if err != nil {
		t.Fatalf("Issue() 返回错误: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Session)
		status int
	}{
		{name: "正常", mutate: func(*Session) {}, status: http.StatusOK},
		{name: "平台禁用", mutate: func(value *Session) { value.User.Status = 0 }, status: http.StatusForbidden},
		{name: "租户禁用", mutate: func(value *Session) { value.Tenant.Status = 0 }, status: http.StatusForbidden},
		{name: "关系禁用", mutate: func(value *Session) { value.TenantUserStatus = 0 }, status: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := base
			test.mutate(&session)
			store := &testStore{session: &session}
			router := newMiniappTestRouter(newTestMiniappHandler(&testWechatExchanger{}, tokens, store))
			recorder := performUserRequest(router, http.MethodGet, "/api/miniapp/me", "", rawToken)
			if recorder.Code != test.status {
				t.Fatalf("响应 = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestMiniappAuthenticationReturnsInternalOnStoreFailure 验证数据库异常不会伪装为未登录。
func TestMiniappAuthenticationReturnsInternalOnStoreFailure(t *testing.T) {
	tokens := NewTokenManager(strings.Repeat("s", 32))
	rawToken, _, err := tokens.Issue(1, 2)
	if err != nil {
		t.Fatalf("Issue() 返回错误: %v", err)
	}
	store := &testStore{findError: errors.New("database failed")}
	router := newMiniappTestRouter(newTestMiniappHandler(&testWechatExchanger{}, tokens, store))
	recorder := performUserRequest(router, http.MethodGet, "/api/miniapp/me", "", rawToken)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestListTenantOptions 验证租户选项仅返回最小字段并保留禁用状态。
func TestListTenantOptions(t *testing.T) {
	store := &testStore{tenantOptions: []TenantOption{
		{ID: 1, Name: "启用租户", Status: 1},
		{ID: 2, Name: "禁用租户", Status: 0},
	}}
	handler := NewAdminHandler(NewUserAdminService(store))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/admin/platform/users/tenant-options", handler.ListTenantOptions)
	recorder := performUserRequest(router, http.MethodGet, "/api/admin/platform/users/tenant-options", "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("响应 = %d %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"id":"1"`) ||
		!strings.Contains(recorder.Body.String(), `"status":"enabled"`) ||
		!strings.Contains(recorder.Body.String(), `"status":"disabled"`) ||
		strings.Contains(recorder.Body.String(), "ownerEmployeeId") {
		t.Fatalf("租户选项响应字段不正确: %s", recorder.Body.String())
	}
}

// TestListPlatformUserTenants 验证关联租户接口返回具体租户并拒绝非法用户 ID。
func TestListPlatformUserTenants(t *testing.T) {
	store := &testStore{platformUserTenants: []PlatformUserTenant{{
		TenantID: 2, TenantName: "租户平台2", TenantStatus: 1, UserStatus: 0,
		JoinedAt: time.Date(2026, 7, 27, 10, 5, 12, 0, time.Local),
	}}}
	handler := NewAdminHandler(NewUserAdminService(store))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/admin/platform/users/:userId/tenants", handler.ListPlatformUserTenants)

	invalid := performUserRequest(router, http.MethodGet, "/api/admin/platform/users/invalid/tenants", "", "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("非法 ID 响应 = %d %s", invalid.Code, invalid.Body.String())
	}
	success := performUserRequest(router, http.MethodGet, "/api/admin/platform/users/4/tenants", "", "")
	if success.Code != http.StatusOK ||
		!strings.Contains(success.Body.String(), `"tenantId":"2"`) ||
		!strings.Contains(success.Body.String(), `"tenantName":"租户平台2"`) ||
		!strings.Contains(success.Body.String(), `"userStatus":"disabled"`) {
		t.Fatalf("关联租户响应 = %d %s", success.Code, success.Body.String())
	}
}
