package auth

// captchaResponse 描述验证码接口返回给前端的开关和图片数据。
type captchaResponse struct {
	Enabled   bool   `json:"enabled"`
	CaptchaID string `json:"captchaId,omitempty"`
	Image     string `json:"image,omitempty"`
	ExpiresIn int    `json:"expiresIn,omitempty"`
}

// loginRequest 描述后台登录接口接收的账号、密码和可选验证码字段。
type loginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	CaptchaID   string `json:"captchaId"`
	CaptchaCode string `json:"captchaCode"`
}

// loginResponse 描述登录或进入租户成功后返回的访问 Token。
type loginResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
}

// currentRoleResponse 描述当前用户接口中的角色摘要。
type currentRoleResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	SystemKey *string `json:"systemKey"`
}

// currentUserResponse 汇总后台当前身份、品牌、角色、权限和菜单。
type currentUserResponse struct {
	EmployeeID      string                `json:"employeeId"`
	Name            string                `json:"name"`
	LoginAccount    string                `json:"loginAccount"`
	Phone           *string               `json:"phone"`
	AvatarText      string                `json:"avatarText"`
	AvatarURL       *string               `json:"avatarUrl"`
	Workspace       string                `json:"workspace"`
	TenantID        *string               `json:"tenantId"`
	TenantName      *string               `json:"tenantName"`
	TenantIconURL   *string               `json:"tenantIconUrl"`
	PlatformName    string                `json:"platformName"`
	PlatformIconURL *string               `json:"platformIconUrl"`
	Mode            string                `json:"mode"`
	IsSuperAdmin    bool                  `json:"isSuperAdmin"`
	Roles           []currentRoleResponse `json:"roles"`
	Permissions     []string              `json:"permissions"`
	Menus           []currentMenuResponse `json:"menus"`
}

// currentMenuResponse 描述前端导航和权限树使用的当前菜单节点。
type currentMenuResponse struct {
	ID               string  `json:"id"`
	ParentID         *string `json:"parentId"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	Scope            string  `json:"scope"`
	Path             *string `json:"path"`
	Component        *string `json:"component"`
	Icon             *string `json:"icon"`
	PermissionCode   *string `json:"permissionCode"`
	TenantAssignable bool    `json:"tenantAssignable"`
	Sort             uint32  `json:"sort"`
	Visible          bool    `json:"visible"`
	Status           string  `json:"status"`
}

// updateBasicProfileRequest 描述当前员工修改本人手机号时提交的字段；Name 仅兼容旧请求，不再写入。
type updateBasicProfileRequest struct {
	Name  string  `json:"name"`
	Phone *string `json:"phone"`
}

// changePasswordRequest 描述当前员工修改本人密码时提交的原密码和新密码。
type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}
