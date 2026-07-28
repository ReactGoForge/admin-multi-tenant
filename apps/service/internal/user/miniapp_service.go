package user

import (
	"context"
	"strings"
	"time"
)

// UserAvatarURLProvider 定义小程序用户私有头像临时地址签发能力。
type UserAvatarURLProvider interface {
	MiniappUserAvatarURL(context.Context, string) (string, error)
}

// MiniappApplication 定义小程序 Handler 依赖的业务服务能力。
type MiniappApplication interface {
	Login(context.Context, MiniappLoginInput) (MiniappLoginResult, error)
	Authenticate(context.Context, string) (*Session, error)
}

// MiniappDataStore 定义小程序登录 Service 需要的数据能力。
type MiniappDataStore interface {
	GetMiniappAppID(context.Context) (string, error)
	FindSession(context.Context, uint64, uint64) (*Session, error)
	WithMiniappTransaction(context.Context, func(MiniappTransactionStore) error) error
}

// MiniappTransactionStore 定义小程序登录事务内需要的数据能力。
type MiniappTransactionStore interface {
	FindTenantForLogin(context.Context, uint64) (Tenant, error)
	EnsureWechatUser(context.Context, WechatIdentity, *string) (User, error)
	EnsureTenantMembership(context.Context, uint64, uint64) (TenantMembership, error)
}

// MiniappLoginInput 描述小程序登录经过 HTTP 校验后的输入。
type MiniappLoginInput struct {
	Code      string
	PhoneCode string
	TenantID  uint64
}

// MiniappLoginResult 描述小程序登录成功后返回给 Handler 的业务结果。
type MiniappLoginResult struct {
	AccessToken string
	ExpiresAt   time.Time
	Session     Session
}

// TenantMembership 描述用户在租户内的归属状态。
type TenantMembership struct {
	Status   uint8
	JoinedAt time.Time
}

// MiniappService 编排微信身份交换、租户归属事务和小程序 Token。
type MiniappService struct {
	store          MiniappDataStore
	wechat         WechatExchanger
	tokens         *TokenManager
	avatarProvider UserAvatarURLProvider
}

// NewMiniappService 使用数据、微信和 Token 能力创建小程序业务服务。
func NewMiniappService(store MiniappDataStore, wechat WechatExchanger, tokens *TokenManager) *MiniappService {
	return &MiniappService{store: store, wechat: wechat, tokens: tokens}
}

// ConfigureAvatarURLs 配置小程序用户头像私有对象键的临时地址签发能力。
func (service *MiniappService) ConfigureAvatarURLs(provider UserAvatarURLProvider) {
	service.avatarProvider = provider
}

// Login 校验微信 code，建立用户租户归属，并签发小程序 Token。
func (service *MiniappService) Login(ctx context.Context, input MiniappLoginInput) (MiniappLoginResult, error) {
	appID, err := service.availableAppID(ctx)
	if err != nil {
		return MiniappLoginResult{}, err
	}
	identity, err := service.wechat.Exchange(ctx, appID, input.Code)
	if err != nil {
		return MiniappLoginResult{}, err
	}
	var phone *string
	if input.PhoneCode != "" {
		trustedPhone, phoneErr := service.wechat.ExchangePhone(ctx, appID, input.PhoneCode)
		if phoneErr != nil {
			return MiniappLoginResult{}, phoneErr
		}
		phone = &trustedPhone
	}
	var session Session
	err = service.store.WithMiniappTransaction(ctx, func(tx MiniappTransactionStore) error {
		tenant, err := tx.FindTenantForLogin(ctx, input.TenantID)
		if err != nil {
			return err
		}
		if tenant.Status != 1 {
			return errTenantDisabled
		}
		user, err := tx.EnsureWechatUser(ctx, identity, phone)
		if err != nil {
			return err
		}
		if user.Status != 1 {
			return errUserDisabled
		}
		membership, err := tx.EnsureTenantMembership(ctx, tenant.ID, user.ID)
		if err != nil {
			return err
		}
		if membership.Status != 1 {
			return errTenantUserDisabled
		}
		session = Session{User: user, Tenant: tenant, TenantUserStatus: membership.Status, JoinedAt: membership.JoinedAt}
		return nil
	})
	if err != nil {
		return MiniappLoginResult{}, err
	}
	service.resolveAvatarURL(ctx, &session.User)
	accessToken, expiresAt, err := service.tokens.Issue(session.User.ID, session.Tenant.ID)
	if err != nil {
		return MiniappLoginResult{}, err
	}
	return MiniappLoginResult{AccessToken: accessToken, ExpiresAt: expiresAt, Session: session}, nil
}

// Authenticate 解析小程序 Token，并实时校验平台用户、租户和租户归属状态。
func (service *MiniappService) Authenticate(ctx context.Context, rawToken string) (*Session, error) {
	identity, err := service.tokens.Parse(rawToken)
	if err != nil {
		return nil, errUserNotFound
	}
	session, err := service.store.FindSession(ctx, identity.UserID, identity.TenantID)
	if err != nil {
		return nil, err
	}
	if session.User.Status != 1 {
		return nil, errUserDisabled
	}
	if session.Tenant.Status != 1 {
		return nil, errTenantDisabled
	}
	if session.TenantUserStatus != 1 {
		return nil, errTenantUserDisabled
	}
	service.resolveAvatarURL(ctx, &session.User)
	return session, nil
}

// resolveAvatarURL 尽力将私有头像对象键转换为临时地址，签名失败时返回空头像。
func (service *MiniappService) resolveAvatarURL(ctx context.Context, currentUser *User) {
	currentUser.AvatarURL = nil
	if service.avatarProvider == nil || currentUser.AvatarObjectKey == nil {
		return
	}
	url, err := service.avatarProvider.MiniappUserAvatarURL(ctx, *currentUser.AvatarObjectKey)
	if err == nil {
		currentUser.AvatarURL = &url
	}
}

// availableAppID 读取当前 AppID，并确认已经完成微信密钥配置。
func (service *MiniappService) availableAppID(ctx context.Context) (string, error) {
	appID, err := service.store.GetMiniappAppID(ctx)
	if err != nil || strings.TrimSpace(appID) == "" {
		return "", errWechatUnavailable
	}
	return strings.TrimSpace(appID), nil
}
