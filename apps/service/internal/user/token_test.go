package user

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var userTokenTestNow = time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)

// TestMiniappTokenIssueAndParse 验证小程序 Token 能还原用户和租户身份。
func TestMiniappTokenIssueAndParse(t *testing.T) {
	manager := NewTokenManager(strings.Repeat("s", 32))
	manager.now = func() time.Time { return userTokenTestNow }
	rawToken, expiresAt, err := manager.Issue(9007199254740993, 9007199254740995)
	if err != nil {
		t.Fatalf("Issue() 返回错误: %v", err)
	}
	if !expiresAt.Equal(userTokenTestNow.Add(7 * 24 * time.Hour)) {
		t.Fatalf("有效期 = %s", expiresAt)
	}
	identity, err := manager.Parse(rawToken)
	if err != nil || identity.UserID != 9007199254740993 || identity.TenantID != 9007199254740995 {
		t.Fatalf("Parse() = %#v, %v", identity, err)
	}
}

// TestMiniappTokenRejectsAdminAudience 验证后台受众 Token 不能访问小程序接口。
func TestMiniappTokenRejectsAdminAudience(t *testing.T) {
	secret := strings.Repeat("s", 32)
	manager := NewTokenManager(secret)
	manager.now = func() time.Time { return userTokenTestNow }
	claims := jwt.RegisteredClaims{
		Issuer: miniappTokenIssuer, Subject: "1", Audience: jwt.ClaimStrings{"admin-multi-tenant-admin"},
		ExpiresAt: jwt.NewNumericDate(userTokenTestNow.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(userTokenTestNow),
	}
	rawToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() 返回错误: %v", err)
	}
	if _, err := manager.Parse(rawToken); err == nil {
		t.Fatal("小程序 Token 管理器接受了后台受众")
	}
}
