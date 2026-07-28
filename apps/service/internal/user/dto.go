package user

import (
	"strconv"
)

const (
	responseTimeFormat = "2006-01-02T15:04:05Z07:00"
	listTimeFormat     = "2006-01-02 15:04:05"
)

// loginRequest 描述小程序登录接口接收的微信 code 和目标租户。
type loginRequest struct {
	Code      string `json:"code"`
	Scene     string `json:"scene"`
	PhoneCode string `json:"phoneCode"`
}

// userResponse 描述小程序当前平台用户字段。
type userResponse struct {
	ID        string  `json:"id"`
	Phone     *string `json:"phone"`
	Nickname  *string `json:"nickname"`
	AvatarURL *string `json:"avatarUrl"`
	Status    string  `json:"status"`
}

// tenantResponse 描述小程序当前租户字段。
type tenantResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// loginResponse 描述小程序登录成功后的 Token、用户和租户。
type loginResponse struct {
	AccessToken string         `json:"accessToken"`
	ExpiresAt   string         `json:"expiresAt"`
	User        userResponse   `json:"user"`
	Tenant      tenantResponse `json:"tenant"`
}

// currentResponse 描述小程序恢复会话时返回的当前身份。
type currentResponse struct {
	User   userResponse   `json:"user"`
	Tenant tenantResponse `json:"tenant"`
}

// statusRequest 描述平台或租户用户状态更新请求。
type statusRequest struct {
	Status string `json:"status"`
}

// platformUserResponse 描述平台视角的小程序用户及租户归属。
type platformUserResponse struct {
	ID          string  `json:"id"`
	Nickname    *string `json:"nickname"`
	AvatarURL   *string `json:"avatarUrl"`
	Phone       *string `json:"phone"`
	Status      string  `json:"status"`
	TenantCount int64   `json:"tenantCount"`
	CreatedAt   string  `json:"createdAt"`
}

// tenantUserResponse 描述租户视角的小程序用户信息。
type tenantUserResponse struct {
	ID             string  `json:"id"`
	Nickname       *string `json:"nickname"`
	AvatarURL      *string `json:"avatarUrl"`
	Phone          *string `json:"phone"`
	PlatformStatus string  `json:"platformStatus"`
	TenantStatus   string  `json:"tenantStatus"`
	JoinedAt       string  `json:"joinedAt"`
}

// platformUserListResponse 描述平台用户列表统一分页响应。
type platformUserListResponse struct {
	Items    []platformUserResponse `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Total    int64                  `json:"total"`
}

// tenantUserListResponse 描述租户用户列表统一分页响应。
type tenantUserListResponse struct {
	Items    []tenantUserResponse `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Total    int64                `json:"total"`
}

// tenantOptionResponse 描述平台用户筛选器使用的租户选项。
type tenantOptionResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// platformUserTenantResponse 描述平台用户关联的一条租户归属。
type platformUserTenantResponse struct {
	TenantID     string `json:"tenantId"`
	TenantName   string `json:"tenantName"`
	TenantStatus string `json:"tenantStatus"`
	UserStatus   string `json:"userStatus"`
	JoinedAt     string `json:"joinedAt"`
}

// miniappSettingsRequest 描述平台微信小程序配置更新请求。
type miniappSettingsRequest struct {
	AppID string `json:"appId"`
}

// newLoginResponse 将小程序登录结果转换为稳定 HTTP 响应。
func newLoginResponse(result MiniappLoginResult) loginResponse {
	return loginResponse{
		AccessToken: result.AccessToken,
		ExpiresAt:   result.ExpiresAt.Format(responseTimeFormat),
		User:        newUserResponse(result.Session.User),
		Tenant:      newTenantResponse(result.Session.Tenant),
	}
}

// newUserResponse 将用户转换为不暴露微信身份标识的接口结构。
func newUserResponse(user User) userResponse {
	return userResponse{ID: strconv.FormatUint(user.ID, 10), Phone: user.Phone, Nickname: user.Nickname, AvatarURL: user.AvatarURL, Status: statusName(user.Status)}
}

// newTenantResponse 将租户 BIGINT ID 转换为字符串安全的接口结构。
func newTenantResponse(tenant Tenant) tenantResponse {
	return tenantResponse{ID: strconv.FormatUint(tenant.ID, 10), Name: tenant.Name}
}

// newPlatformUserListResponse 将平台用户分页结果转换为 HTTP 响应。
func newPlatformUserListResponse(result PlatformUserListResult) platformUserListResponse {
	items := make([]platformUserResponse, 0, len(result.Items))
	for _, user := range result.Items {
		items = append(items, platformUserResponse{ID: strconv.FormatUint(user.ID, 10), Nickname: user.Nickname, AvatarURL: user.AvatarURL, Phone: user.Phone, Status: statusName(user.Status), TenantCount: user.TenantCount, CreatedAt: user.CreatedAt.Format(listTimeFormat)})
	}
	return platformUserListResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total}
}

// newTenantUserListResponse 将租户用户分页结果转换为 HTTP 响应。
func newTenantUserListResponse(result TenantUserListResult) tenantUserListResponse {
	items := make([]tenantUserResponse, 0, len(result.Items))
	for _, user := range result.Items {
		items = append(items, tenantUserResponse{ID: strconv.FormatUint(user.ID, 10), Nickname: user.Nickname, AvatarURL: user.AvatarURL, Phone: user.Phone, PlatformStatus: statusName(user.Status), TenantStatus: statusName(user.TenantStatus), JoinedAt: user.JoinedAt.Format(listTimeFormat)})
	}
	return tenantUserListResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total}
}

// newTenantOptionResponses 将租户选项转换为 BIGINT 字符串响应。
func newTenantOptionResponses(options []TenantOption) []tenantOptionResponse {
	responses := make([]tenantOptionResponse, 0, len(options))
	for _, option := range options {
		responses = append(responses, tenantOptionResponse{ID: strconv.FormatUint(option.ID, 10), Name: option.Name, Status: statusName(option.Status)})
	}
	return responses
}

// newPlatformUserTenantResponses 将用户租户归属转换为稳定 HTTP 响应。
func newPlatformUserTenantResponses(tenants []PlatformUserTenant) []platformUserTenantResponse {
	responses := make([]platformUserTenantResponse, 0, len(tenants))
	for _, tenant := range tenants {
		responses = append(responses, platformUserTenantResponse{
			TenantID:     strconv.FormatUint(tenant.TenantID, 10),
			TenantName:   tenant.TenantName,
			TenantStatus: statusName(tenant.TenantStatus),
			UserStatus:   statusName(tenant.UserStatus),
			JoinedAt:     tenant.JoinedAt.Format(listTimeFormat),
		})
	}
	return responses
}

// statusName 将数据库状态转换为稳定的接口枚举值。
func statusName(status uint8) string {
	if status == 1 {
		return "enabled"
	}
	return "disabled"
}
