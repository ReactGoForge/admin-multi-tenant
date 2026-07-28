package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TestServiceLoginActivatesUniqueSession 验证 Auth Service 登录成功会签发 Token 并激活唯一会话。
func TestServiceLoginActivatesUniqueSession(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() 返回错误: %v", err)
	}
	store := &testEmployeeStore{
		loginEmployee: &Employee{ID: 42, Scope: "platform", LoginAccount: "admin", PasswordHash: string(passwordHash), Status: 1},
	}
	tokens := NewTokenManager(strings.Repeat("s", minimumJWTBytes))
	tokens.now = func() time.Time { return tokenTestNow }
	service := NewService(&CaptchaManager{enabled: false}, tokens, store)
	service.ConfigureLoginSecurity(nil, nil)

	issued, err := service.Login(context.Background(), loginRequest{Username: "admin", Password: "secret123"}, LoginAuditMeta{ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Login() 返回错误: %v", err)
	}
	if issued.AccessToken == "" || !validSessionID(issued.SessionID) || store.activatedSessionID != issued.SessionID {
		t.Fatalf("登录结果或会话激活错误: issued=%#v activated=%q", issued, store.activatedSessionID)
	}
}

// TestAuthorizationServiceRequirePermissionUsesRealtimeStore 验证权限服务从 Store 实时读取角色权限。
func TestAuthorizationServiceRequirePermissionUsesRealtimeStore(t *testing.T) {
	employee := Employee{ID: 7, Scope: "tenant", LoginAccount: "tenant-admin", Status: 1}
	store := &testEmployeeStore{
		roles:       []Role{{ID: 12, Name: "租户角色"}},
		permissions: []string{"tenant:user:view"},
	}
	service := NewAuthorizationService(NewTokenManager(strings.Repeat("s", minimumJWTBytes)), store)

	allowed, platformSuperAdmin, err := service.RequirePermission(context.Background(), employee, TokenIdentity{EmployeeID: employee.ID, Scope: employee.Scope, Mode: "normal"}, "tenant", "tenant:user:view")
	if err != nil {
		t.Fatalf("RequirePermission() 返回错误: %v", err)
	}
	if !allowed || platformSuperAdmin || store.permissionCalls != 1 {
		t.Fatalf("权限判断结果错误: allowed=%v platformSuperAdmin=%v calls=%d", allowed, platformSuperAdmin, store.permissionCalls)
	}
}
