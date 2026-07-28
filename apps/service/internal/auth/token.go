package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenIssuer   = "admin-multi-tenant-service"
	tokenAudience = "admin-multi-tenant-admin"
	tokenTTL      = 8 * time.Hour
	tokenLeeway   = 30 * time.Second
)

// tokenClaims 描述写入 JWT 负载的后台身份字段和标准声明。
// Go 学习提示：匿名嵌入 RegisteredClaims 后，其标准字段会成为 tokenClaims 的一部分。
type tokenClaims struct {
	Scope    string  `json:"scope"`
	TenantID *string `json:"tenantId,omitempty"`
	Mode     string  `json:"mode,omitempty"`
	jwt.RegisteredClaims
}

// TokenIdentity 描述从 JWT 中解析出的最小后台员工身份。
type TokenIdentity struct {
	EmployeeID uint64
	Scope      string
	TenantID   *uint64
	Mode       string
	SessionID  string
	ExpiresAt  time.Time
}

// IssuedToken 描述一次成功签发的后台 Token、会话标识与有效期。
type IssuedToken struct {
	AccessToken string
	SessionID   string
	ExpiresAt   time.Time
}

// TokenManager 负责签发和严格解析后台访问 Token。
type TokenManager struct {
	secret []byte
	now    func() time.Time
	ttl    time.Duration
}

// NewTokenManager 使用认证密钥创建固定 HS256 策略的 Token 管理器。
func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
		now:    time.Now,
		ttl:    tokenTTL,
	}
}

// Issue 为当前员工生成随机会话标识并签发八小时有效的后台访问 Token。
func (manager *TokenManager) Issue(employee Employee) (IssuedToken, error) {
	// 安全边界：会话 ID 使用 32 字节安全随机数，既作为 JWT 的 jti，也用于数据库单会话校验。
	sessionBytes := make([]byte, 32)
	if _, err := rand.Read(sessionBytes); err != nil {
		return IssuedToken{}, fmt.Errorf("生成后台会话标识失败: %w", err)
	}
	sessionID := hex.EncodeToString(sessionBytes)
	now := manager.now().UTC()
	expiresAt := now.Add(manager.ttl)
	return manager.issue(employee.ID, employee.Scope, employee.TenantID, "normal", sessionID, now, expiresAt)
}

// IssueManaged 为平台员工签发不超过原会话有效期的租户代管 Token。
func (manager *TokenManager) IssueManaged(employeeID, tenantID uint64, sessionID string, expiresAt time.Time) (IssuedToken, error) {
	now := manager.now().UTC()
	if !expiresAt.After(now) {
		return IssuedToken{}, fmt.Errorf("原访问 Token 已过期")
	}
	return manager.issue(employeeID, "platform", &tenantID, "managed", sessionID, now, expiresAt.UTC())
}

// issue 使用指定身份范围和有效期签发后台访问 Token。
func (manager *TokenManager) issue(employeeID uint64, scope string, tenantID *uint64, mode, sessionID string, now, expiresAt time.Time) (IssuedToken, error) {
	if !validSessionID(sessionID) {
		return IssuedToken{}, fmt.Errorf("后台会话标识无效")
	}
	// Go 学习提示：TenantID 使用指针表达“可选值”；nil 表示该身份没有租户范围，
	// omitempty 会让 JSON/JWT 在 nil 时省略 tenantId 字段。
	claims := tokenClaims{
		Scope: scope,
		Mode:  mode,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			Issuer:    tokenIssuer,
			Subject:   strconv.FormatUint(employeeID, 10),
			Audience:  jwt.ClaimStrings{tokenAudience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	if tenantID != nil {
		formattedTenantID := strconv.FormatUint(*tenantID, 10)
		claims.TenantID = &formattedTenantID
	}

	signedToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(manager.secret)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("签发访问 Token 失败: %w", err)
	}
	return IssuedToken{AccessToken: signedToken, SessionID: sessionID, ExpiresAt: expiresAt}, nil
}

// Parse 严格校验算法、签发者、受众与有效期，并还原后台员工身份。
func (manager *TokenManager) Parse(rawToken string) (TokenIdentity, error) {
	// 安全边界：解析时同时限制算法、签发者、受众、有效期和签发时间，
	// 不能只验证签名后就直接信任 Token。
	claims := &tokenClaims{}
	parsedToken, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("不允许的 Token 签名算法")
			}
			return manager.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(tokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(tokenLeeway),
		jwt.WithTimeFunc(manager.now),
	)
	if err != nil || !parsedToken.Valid || (claims.Scope != "platform" && claims.Scope != "tenant") {
		return TokenIdentity{}, fmt.Errorf("访问 Token 无效")
	}

	employeeID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || employeeID == 0 {
		return TokenIdentity{}, fmt.Errorf("访问 Token 员工标识无效")
	}
	mode := claims.Mode
	if mode == "" {
		mode = "normal"
	}
	if mode != "normal" && mode != "managed" {
		return TokenIdentity{}, fmt.Errorf("访问 Token 模式无效")
	}
	if claims.ExpiresAt == nil {
		return TokenIdentity{}, fmt.Errorf("访问 Token 有效期无效")
	}
	if !validSessionID(claims.ID) {
		return TokenIdentity{}, fmt.Errorf("访问 Token 会话标识无效")
	}
	identity := TokenIdentity{EmployeeID: employeeID, Scope: claims.Scope, Mode: mode, SessionID: claims.ID, ExpiresAt: claims.ExpiresAt.Time}
	if claims.TenantID != nil {
		tenantID, parseErr := strconv.ParseUint(*claims.TenantID, 10, 64)
		if parseErr != nil || tenantID == 0 {
			return TokenIdentity{}, fmt.Errorf("访问 Token 租户标识无效")
		}
		identity.TenantID = &tenantID
	}
	if (mode == "managed" && (claims.Scope != "platform" || identity.TenantID == nil)) || (mode == "normal" && claims.Scope == "platform" && identity.TenantID != nil) {
		return TokenIdentity{}, fmt.Errorf("访问 Token 身份范围无效")
	}
	return identity, nil
}

// validSessionID 校验会话标识是否为 32 字节随机值对应的十六进制字符串。
func validSessionID(sessionID string) bool {
	if len(sessionID) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(sessionID)
	return err == nil && len(decoded) == 32
}
