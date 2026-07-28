package user

import "context"

// UserAdminApplication 定义后台用户 Handler 依赖的业务服务能力。
type UserAdminApplication interface {
	ListPlatformUsers(context.Context, PlatformUserQuery) (PlatformUserListResult, error)
	ListPlatformUserTenants(context.Context, uint64) ([]PlatformUserTenant, error)
	ListTenantOptions(context.Context) ([]TenantOption, error)
	SetPlatformUserStatus(context.Context, uint64, uint8) error
	ListTenantUsers(context.Context, uint64, TenantUserQuery) (TenantUserListResult, error)
	SetTenantUserStatus(context.Context, uint64, uint64, uint8) error
}

// UserAdminDataStore 定义后台用户 Service 需要的数据能力。
type UserAdminDataStore interface {
	ListPlatformUsers(context.Context, PlatformUserQuery) ([]PlatformUser, int64, error)
	ListPlatformUserTenants(context.Context, uint64) ([]PlatformUserTenant, error)
	ListTenantOptions(context.Context) ([]TenantOption, error)
	SetPlatformUserStatus(context.Context, uint64, uint8) error
	ListTenantUsers(context.Context, uint64, TenantUserQuery) ([]TenantUser, int64, error)
	SetTenantUserStatus(context.Context, uint64, uint64, uint8) error
}

// PlatformUserListResult 描述平台用户分页业务结果。
type PlatformUserListResult struct {
	Items    []PlatformUser
	Page     int
	PageSize int
	Total    int64
}

// TenantUserListResult 描述租户用户分页业务结果。
type TenantUserListResult struct {
	Items    []TenantUser
	Page     int
	PageSize int
	Total    int64
}

// UserAdminService 编排后台平台用户和租户用户管理。
type UserAdminService struct {
	store          UserAdminDataStore
	avatarProvider UserAvatarURLProvider
}

// NewUserAdminService 使用后台用户数据能力创建业务服务。
func NewUserAdminService(store UserAdminDataStore) *UserAdminService {
	return &UserAdminService{store: store}
}

// ConfigureAvatarURLs 配置用户头像私有对象键的临时地址签发能力。
func (service *UserAdminService) ConfigureAvatarURLs(provider UserAvatarURLProvider) {
	service.avatarProvider = provider
}

// ListPlatformUsers 返回平台唯一用户分页列表。
func (service *UserAdminService) ListPlatformUsers(ctx context.Context, query PlatformUserQuery) (PlatformUserListResult, error) {
	users, total, err := service.store.ListPlatformUsers(ctx, query)
	if err != nil {
		return PlatformUserListResult{}, err
	}
	service.resolveUserAvatars(ctx, users)
	return PlatformUserListResult{Items: users, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

// ListPlatformUserTenants 返回指定平台用户关联的全部租户。
func (service *UserAdminService) ListPlatformUserTenants(ctx context.Context, userID uint64) ([]PlatformUserTenant, error) {
	return service.store.ListPlatformUserTenants(ctx, userID)
}

// ListTenantOptions 返回平台用户筛选所需的租户选项。
func (service *UserAdminService) ListTenantOptions(ctx context.Context) ([]TenantOption, error) {
	return service.store.ListTenantOptions(ctx)
}

// SetPlatformUserStatus 更新平台用户全局状态。
func (service *UserAdminService) SetPlatformUserStatus(ctx context.Context, userID uint64, status uint8) error {
	return service.store.SetPlatformUserStatus(ctx, userID, status)
}

// ListTenantUsers 返回指定可信租户范围内的用户分页列表。
func (service *UserAdminService) ListTenantUsers(ctx context.Context, tenantID uint64, query TenantUserQuery) (TenantUserListResult, error) {
	users, total, err := service.store.ListTenantUsers(ctx, tenantID, query)
	if err != nil {
		return TenantUserListResult{}, err
	}
	for index := range users {
		service.resolveAvatarURL(ctx, &users[index].User)
	}
	return TenantUserListResult{Items: users, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

// SetTenantUserStatus 更新用户在指定可信租户内的状态。
func (service *UserAdminService) SetTenantUserStatus(ctx context.Context, tenantID, userID uint64, status uint8) error {
	return service.store.SetTenantUserStatus(ctx, tenantID, userID, status)
}

// resolveUserAvatars 为平台用户列表尽力签发头像地址，单个签名失败不阻断列表。
func (service *UserAdminService) resolveUserAvatars(ctx context.Context, users []PlatformUser) {
	for index := range users {
		service.resolveAvatarURL(ctx, &users[index].User)
	}
}

// resolveAvatarURL 将用户私有头像对象键转换为临时访问地址。
func (service *UserAdminService) resolveAvatarURL(ctx context.Context, currentUser *User) {
	currentUser.AvatarURL = nil
	if service.avatarProvider == nil || currentUser.AvatarObjectKey == nil {
		return
	}
	url, err := service.avatarProvider.MiniappUserAvatarURL(ctx, *currentUser.AvatarObjectKey)
	if err == nil {
		currentUser.AvatarURL = &url
	}
}
