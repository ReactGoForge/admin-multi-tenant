package user

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	miniappTokenIssuer   = "admin-multi-tenant-service"
	miniappTokenAudience = "admin-multi-tenant-miniapp"
	miniappTokenTTL      = 7 * 24 * time.Hour
	miniappTokenLeeway   = 30 * time.Second
)

// miniappTokenClaims 描述写入小程序 JWT 的用户、租户和标准声明。
type miniappTokenClaims struct {
	TenantID string `json:"tenantId"`
	jwt.RegisteredClaims
}

// TokenIdentity 描述小程序 Token 中的平台用户和当前租户身份。
type TokenIdentity struct {
	UserID   uint64
	TenantID uint64
}

// TokenManager 负责签发和严格解析小程序访问 Token。
type TokenManager struct {
	secret []byte
	now    func() time.Time
	ttl    time.Duration
}

// NewTokenManager 使用现有 JWT 密钥创建独立受众的小程序 Token 管理器。
func NewTokenManager(secret string) *TokenManager {
	// 安全边界：小程序 Token 使用独立 Audience，后台员工 Token 即使使用同一签名密钥，
	// 也无法通过小程序认证入口的受众校验。
	return &TokenManager{secret: []byte(secret), now: time.Now, ttl: miniappTokenTTL}
}

// Issue 为指定平台用户和租户签发七天有效的小程序 Token。
func (manager *TokenManager) Issue(userID, tenantID uint64) (string, time.Time, error) {
	now := manager.now().UTC()
	expiresAt := now.Add(manager.ttl)
	claims := miniappTokenClaims{
		TenantID: strconv.FormatUint(tenantID, 10),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: miniappTokenIssuer, Subject: strconv.FormatUint(userID, 10),
			Audience: jwt.ClaimStrings{miniappTokenAudience}, ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now), IssuedAt: jwt.NewNumericDate(now),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(manager.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("签发小程序 Token 失败: %w", err)
	}
	return token, expiresAt, nil
}

// Parse 严格校验小程序 Token 的算法、签发者、受众和有效期。
func (manager *TokenManager) Parse(rawToken string) (TokenIdentity, error) {
	claims := &miniappTokenClaims{}
	// 安全边界：解析时同时固定签名算法、签发者、受众和有效期，不能只验证签名。
	// 回调中的 token.Method 再检查一次算法，可防止调用方传入其他算法的 Token。
	parsed, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("不允许的小程序 Token 签名算法")
		}
		return manager.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(miniappTokenIssuer),
		jwt.WithAudience(miniappTokenAudience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(),
		jwt.WithLeeway(miniappTokenLeeway), jwt.WithTimeFunc(manager.now))
	if err != nil || !parsed.Valid {
		return TokenIdentity{}, fmt.Errorf("小程序 Token 无效")
	}
	userID, userErr := strconv.ParseUint(claims.Subject, 10, 64)
	tenantID, tenantErr := strconv.ParseUint(claims.TenantID, 10, 64)
	if userErr != nil || tenantErr != nil || userID == 0 || tenantID == 0 {
		return TokenIdentity{}, fmt.Errorf("小程序 Token 身份无效")
	}
	return TokenIdentity{UserID: userID, TenantID: tenantID}, nil
}
