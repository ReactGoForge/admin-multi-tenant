package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var tokenTestNow = time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)

// TestTokenIssueAndParse 验证平台和租户员工 Token 能还原字符串安全的身份字段。
func TestTokenIssueAndParse(t *testing.T) {
	manager := NewTokenManager(strings.Repeat("s", minimumJWTBytes))
	manager.now = func() time.Time { return tokenTestNow }
	tenantID := uint64(9007199254740993)

	tests := []Employee{
		{ID: 1, Scope: "platform"},
		{ID: 9007199254740995, Scope: "tenant", TenantID: &tenantID},
	}
	for _, employee := range tests {
		issued, err := manager.Issue(employee)
		if err != nil {
			t.Fatalf("Issue() 返回错误: %v", err)
		}
		if !issued.ExpiresAt.Equal(tokenTestNow.Add(tokenTTL)) {
			t.Fatalf("expiresAt = %s，期望 %s", issued.ExpiresAt, tokenTestNow.Add(tokenTTL))
		}
		identity, err := manager.Parse(issued.AccessToken)
		if err != nil {
			t.Fatalf("Parse() 返回错误: %v", err)
		}
		if identity.EmployeeID != employee.ID || identity.Scope != employee.Scope {
			t.Fatalf("Parse() 身份 = %#v，员工 = %#v", identity, employee)
		}
		if identity.SessionID != issued.SessionID || !validSessionID(identity.SessionID) {
			t.Fatalf("Parse() 会话标识 = %q，签发标识 = %q", identity.SessionID, issued.SessionID)
		}
		if (identity.TenantID == nil) != (employee.TenantID == nil) || (identity.TenantID != nil && *identity.TenantID != *employee.TenantID) {
			t.Fatalf("Parse() 租户身份 = %#v，员工 = %#v", identity.TenantID, employee.TenantID)
		}
	}
}

// TestManagedTokenKeepsPlatformActorAndParentExpiry 验证代管 Token 保留平台操作者并继承原会话有效期。
func TestManagedTokenKeepsPlatformActorAndParentExpiry(t *testing.T) {
	manager := NewTokenManager(strings.Repeat("s", minimumJWTBytes))
	manager.now = func() time.Time { return tokenTestNow }
	expiresAt := tokenTestNow.Add(20 * time.Minute)
	sessionID := strings.Repeat("a", 64)
	issued, err := manager.IssueManaged(7, 99, sessionID, expiresAt)
	if err != nil || !issued.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("IssueManaged() = %#v, %v", issued, err)
	}
	identity, err := manager.Parse(issued.AccessToken)
	if err != nil || identity.EmployeeID != 7 || identity.Scope != "platform" || identity.Mode != "managed" || identity.SessionID != sessionID || identity.TenantID == nil || *identity.TenantID != 99 || !identity.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("代管身份解析结果 = %#v, %v", identity, err)
	}
}

// TestTokenRejectsExpiredAndWrongSignature 验证过期和错误密钥签名均被拒绝。
func TestTokenRejectsExpiredAndWrongSignature(t *testing.T) {
	manager := NewTokenManager(strings.Repeat("s", minimumJWTBytes))
	manager.now = func() time.Time { return tokenTestNow }
	manager.ttl = -time.Minute
	expiredToken, err := manager.Issue(Employee{ID: 1, Scope: "platform"})
	if err != nil {
		t.Fatalf("Issue() 返回错误: %v", err)
	}
	if _, err := manager.Parse(expiredToken.AccessToken); err == nil {
		t.Fatal("Parse() 接受了过期 Token")
	}

	manager.ttl = tokenTTL
	issued, err := manager.Issue(Employee{ID: 1, Scope: "platform"})
	if err != nil {
		t.Fatalf("Issue() 返回错误: %v", err)
	}
	otherManager := NewTokenManager(strings.Repeat("x", minimumJWTBytes))
	otherManager.now = manager.now
	if _, err := otherManager.Parse(issued.AccessToken); err == nil {
		t.Fatal("Parse() 接受了错误密钥签名")
	}
}

// TestTokenRejectsMissingSessionID 验证旧 Token 或缺少 jti 的 Token 会被拒绝。
func TestTokenRejectsMissingSessionID(t *testing.T) {
	secret := strings.Repeat("s", minimumJWTBytes)
	manager := NewTokenManager(secret)
	manager.now = func() time.Time { return tokenTestNow }
	claims := tokenClaims{
		Scope: "platform",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: tokenIssuer, Subject: "1", Audience: jwt.ClaimStrings{tokenAudience},
			ExpiresAt: jwt.NewNumericDate(tokenTestNow.Add(time.Hour)), NotBefore: jwt.NewNumericDate(tokenTestNow), IssuedAt: jwt.NewNumericDate(tokenTestNow),
		},
	}
	rawToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() 返回错误: %v", err)
	}
	if _, err := manager.Parse(rawToken); err == nil {
		t.Fatal("Parse() 接受了缺少 jti 的 Token")
	}
}

// TestTokenRejectsAlgorithmIssuerAndAudience 验证算法、签发者和受众白名单。
func TestTokenRejectsAlgorithmIssuerAndAudience(t *testing.T) {
	secret := strings.Repeat("s", minimumJWTBytes)
	manager := NewTokenManager(secret)
	manager.now = func() time.Time { return tokenTestNow }

	tests := []struct {
		name     string
		method   jwt.SigningMethod
		issuer   string
		audience string
	}{
		{name: "错误算法", method: jwt.SigningMethodHS384, issuer: tokenIssuer, audience: tokenAudience},
		{name: "错误签发者", method: jwt.SigningMethodHS256, issuer: "other-service", audience: tokenAudience},
		{name: "错误受众", method: jwt.SigningMethodHS256, issuer: tokenIssuer, audience: "other-client"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := tokenClaims{
				Scope: "platform",
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    test.issuer,
					Subject:   "1",
					Audience:  jwt.ClaimStrings{test.audience},
					ExpiresAt: jwt.NewNumericDate(tokenTestNow.Add(time.Hour)),
					NotBefore: jwt.NewNumericDate(tokenTestNow),
					IssuedAt:  jwt.NewNumericDate(tokenTestNow),
				},
			}
			rawToken, err := jwt.NewWithClaims(test.method, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatalf("SignedString() 返回错误: %v", err)
			}
			if _, err := manager.Parse(rawToken); err == nil {
				t.Fatal("Parse() 接受了不符合策略的 Token")
			}
		})
	}
}
