package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"

	"golang.org/x/crypto/bcrypt"
)

// LoginAuditMeta 保存登录日志需要的请求上下文，不包含密码、验证码或 Token。
type LoginAuditMeta struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

// LoginRecorder 定义 Auth Service 保存后台登录结果所需的最小日志能力。
type LoginRecorder interface {
	RecordLogin(context.Context, logging.LoginLog) error
}

// ServiceErrorKind 表示 Auth 业务服务内部稳定错误类型。
type ServiceErrorKind int

const (
	serviceErrorInternal ServiceErrorKind = iota + 1
	serviceErrorCaptchaInvalid
	serviceErrorCaptchaUnavailable
	serviceErrorCurrentPasswordInvalid
	serviceErrorUnauthorized
	serviceErrorCredentialsInvalid
	serviceErrorAccountDisabled
	serviceErrorTenantDisabled
	serviceErrorLoginRateLimited
	serviceErrorForbidden
	serviceErrorResourceNotFound
	serviceErrorLoginSecurityUnavailable
)

// ServiceError 表示 Auth 业务服务返回给 Handler 映射的稳定模块错误。
type ServiceError struct {
	Kind       ServiceErrorKind
	RetryAfter time.Duration
}

// Error 返回服务错误的调试文本。
func (err ServiceError) Error() string {
	return strconv.Itoa(int(err.Kind))
}

// Service 组织后台验证码、登录、当前用户和个人资料业务流程。
type Service struct {
	captcha       *CaptchaManager
	tokens        *TokenManager
	employees     EmployeeStore
	loginLimiter  *LoginLimiter
	avatarURLs    AvatarURLProvider
	loginRecorder LoginRecorder
	now           func() time.Time
}

// NewService 使用验证码、Token 与员工数据能力创建认证业务服务。
func NewService(captcha *CaptchaManager, tokens *TokenManager, employees EmployeeStore) *Service {
	return &Service{
		captcha:   captcha,
		tokens:    tokens,
		employees: employees,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// ConfigureLoginSecurity 为登录流程接入限流器和登录日志记录器。
func (service *Service) ConfigureLoginSecurity(limiter *LoginLimiter, recorder LoginRecorder) {
	service.loginLimiter = limiter
	service.loginRecorder = recorder
}

// ConfigureAvatarURLs 为当前用户接口接入私有头像临时地址签发能力。
func (service *Service) ConfigureAvatarURLs(provider AvatarURLProvider) {
	service.avatarURLs = provider
}

// Captcha 返回验证码开关状态，启用时生成并缓存一张一次性数字验证码。
func (service *Service) Captcha(ctx context.Context) (captchaResponse, error) {
	if !service.captcha.Enabled() {
		return captchaResponse{Enabled: false}, nil
	}
	result, err := service.captcha.Generate(ctx)
	if err != nil {
		if errors.Is(err, ErrCaptchaUnavailable) {
			return captchaResponse{}, ServiceError{Kind: serviceErrorCaptchaUnavailable}
		}
		return captchaResponse{}, ServiceError{Kind: serviceErrorInternal}
	}
	return captchaResponse{
		Enabled:   true,
		CaptchaID: result.ID,
		Image:     result.Image,
		ExpiresIn: result.ExpiresIn,
	}, nil
}

// Login 校验一次性验证码与员工凭证，并签发八小时有效的访问 Token。
func (service *Service) Login(ctx context.Context, request loginRequest, meta LoginAuditMeta) (IssuedToken, error) {
	if retryAfter, err := service.loginLimiter.Check(ctx, request.Username, meta.ClientIP); err != nil {
		service.recordLogin(ctx, meta, request.Username, nil, "failed", "security_unavailable")
		return IssuedToken{}, ServiceError{Kind: serviceErrorLoginSecurityUnavailable}
	} else if retryAfter > 0 {
		service.recordLogin(ctx, meta, request.Username, nil, "limited", "rate_limited")
		return IssuedToken{}, ServiceError{Kind: serviceErrorLoginRateLimited, RetryAfter: loginLockDuration}
	}

	if service.captcha.Enabled() {
		if err := service.captcha.Verify(ctx, request.CaptchaID, request.CaptchaCode); err != nil {
			if errors.Is(err, ErrCaptchaUnavailable) {
				service.recordLogin(ctx, meta, request.Username, nil, "failed", "captcha_unavailable")
				return IssuedToken{}, ServiceError{Kind: serviceErrorCaptchaUnavailable}
			}
			retryAfter, limitErr := service.loginLimiter.RecordIPFailure(ctx, meta.ClientIP)
			if limitErr != nil {
				service.recordLogin(ctx, meta, request.Username, nil, "failed", "security_unavailable")
				return IssuedToken{}, ServiceError{Kind: serviceErrorLoginSecurityUnavailable}
			}
			if retryAfter > 0 {
				service.recordLogin(ctx, meta, request.Username, nil, "limited", "rate_limited")
				return IssuedToken{}, ServiceError{Kind: serviceErrorLoginRateLimited, RetryAfter: loginLockDuration}
			}
			service.recordLogin(ctx, meta, request.Username, nil, "failed", "captcha_invalid")
			return IssuedToken{}, ServiceError{Kind: serviceErrorCaptchaInvalid}
		}
	}

	employee, err := service.employees.FindByLogin(ctx, request.Username)
	if err != nil {
		return IssuedToken{}, ServiceError{Kind: serviceErrorInternal}
	}
	if employee == nil {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(request.Password))
		return service.handleCredentialFailure(ctx, meta, request.Username, nil)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(employee.PasswordHash), []byte(request.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return service.handleCredentialFailure(ctx, meta, request.Username, employee)
		}
		return IssuedToken{}, ServiceError{Kind: serviceErrorInternal}
	}
	if employee.Status != 1 {
		service.recordLogin(ctx, meta, request.Username, employee, "failed", "account_disabled")
		return IssuedToken{}, ServiceError{Kind: serviceErrorAccountDisabled}
	}
	if err := service.loginLimiter.ClearAccount(ctx, request.Username); err != nil {
		service.recordLogin(ctx, meta, request.Username, employee, "failed", "security_unavailable")
		return IssuedToken{}, ServiceError{Kind: serviceErrorLoginSecurityUnavailable}
	}

	issued, err := service.tokens.Issue(*employee)
	if err != nil {
		return IssuedToken{}, ServiceError{Kind: serviceErrorInternal}
	}
	if err := service.employees.ActivateSession(ctx, employee.ID, issued.SessionID); err != nil {
		return IssuedToken{}, ServiceError{Kind: serviceErrorInternal}
	}
	service.recordLogin(ctx, meta, request.Username, employee, "success", "success")
	return issued, nil
}

// EnterTenant 为已获授权的平台员工签发指定租户的短期代管 Token。
func (service *Service) EnterTenant(ctx context.Context, employee Employee, identity TokenIdentity, tenantID uint64) (IssuedToken, error) {
	if employee.Scope != "platform" || identity.Mode != "normal" {
		return IssuedToken{}, ServiceError{Kind: serviceErrorForbidden}
	}
	tenant, err := service.employees.FindTenantByID(ctx, tenantID)
	if err != nil {
		return IssuedToken{}, ServiceError{Kind: serviceErrorInternal}
	}
	if tenant == nil {
		return IssuedToken{}, ServiceError{Kind: serviceErrorResourceNotFound}
	}
	if tenant.Status != 1 {
		return IssuedToken{}, ServiceError{Kind: serviceErrorTenantDisabled}
	}
	issued, err := service.tokens.IssueManaged(employee.ID, tenantID, identity.SessionID, identity.ExpiresAt)
	if err != nil {
		return IssuedToken{}, ServiceError{Kind: serviceErrorUnauthorized}
	}
	return issued, nil
}

// CurrentUser 汇总当前后台员工身份、品牌、角色、权限和菜单。
func (service *Service) CurrentUser(ctx context.Context, employee Employee, identity TokenIdentity) (currentUserResponse, error) {
	workspace := employee.Scope
	mode := identity.Mode
	if mode == "managed" {
		workspace = "tenant"
	}

	roles, err := service.employees.ListRoles(ctx, employee)
	if err != nil {
		return currentUserResponse{}, ServiceError{Kind: serviceErrorInternal}
	}
	roleResponses, roleIDs, isSuperAdmin := buildRoleResponse(roles, employee.Scope)

	permissions := make([]string, 0)
	if mode == "managed" {
		permissions, err = service.employees.ListManagedPermissions(ctx, roleIDs, isSuperAdmin)
	} else if !isSuperAdmin {
		permissions, err = service.employees.ListPermissions(ctx, employee, roleIDs)
	}
	if err != nil {
		return currentUserResponse{}, ServiceError{Kind: serviceErrorInternal}
	}
	if permissions == nil {
		permissions = make([]string, 0)
	}

	tenantID, tenantName, tenantIconURL, tenantErr := service.currentTenantInfo(ctx, employee, identity)
	if tenantErr != nil {
		return currentUserResponse{}, tenantErr
	}
	platformName, platformIconURL, brandErr := service.currentPlatformBrand(ctx)
	if brandErr != nil {
		return currentUserResponse{}, brandErr
	}
	menus, menuErr := service.currentMenus(ctx, workspace)
	if menuErr != nil {
		return currentUserResponse{}, menuErr
	}

	return currentUserResponse{
		EmployeeID:      strconv.FormatUint(employee.ID, 10),
		Name:            employee.Name,
		LoginAccount:    employee.LoginAccount,
		Phone:           employee.Phone,
		AvatarText:      firstCharacter(employee.Name),
		AvatarURL:       service.currentAvatarURL(ctx, employee),
		Workspace:       workspace,
		TenantID:        tenantID,
		TenantName:      tenantName,
		TenantIconURL:   tenantIconURL,
		PlatformName:    platformName,
		PlatformIconURL: platformIconURL,
		Mode:            mode,
		IsSuperAdmin:    isSuperAdmin,
		Roles:           roleResponses,
		Permissions:     permissions,
		Menus:           menus,
	}, nil
}

// UpdateBasicProfile 更新当前认证员工本人的手机号。
func (service *Service) UpdateBasicProfile(ctx context.Context, employee Employee, request updateBasicProfileRequest) error {
	if err := service.employees.UpdateBasicProfile(ctx, employee.ID, request.Phone); err != nil {
		return ServiceError{Kind: serviceErrorInternal}
	}
	return nil
}

// ChangePassword 校验当前员工原密码后写入新哈希，并使全部旧后台会话失效。
func (service *Service) ChangePassword(ctx context.Context, employee Employee, request changePasswordRequest) error {
	if err := bcrypt.CompareHashAndPassword([]byte(employee.PasswordHash), []byte(request.CurrentPassword)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ServiceError{Kind: serviceErrorCurrentPasswordInvalid}
		}
		return ServiceError{Kind: serviceErrorInternal}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return ServiceError{Kind: serviceErrorInternal}
	}
	if err := service.employees.ChangePassword(ctx, employee.ID, string(hash)); err != nil {
		return ServiceError{Kind: serviceErrorInternal}
	}
	return nil
}

// handleCredentialFailure 累计账号和 IP 失败次数，并返回对应登录错误。
func (service *Service) handleCredentialFailure(ctx context.Context, meta LoginAuditMeta, account string, employee *Employee) (IssuedToken, error) {
	retryAfter, err := service.loginLimiter.RecordCredentialFailure(ctx, account, meta.ClientIP)
	if err != nil {
		service.recordLogin(ctx, meta, account, employee, "failed", "security_unavailable")
		return IssuedToken{}, ServiceError{Kind: serviceErrorLoginSecurityUnavailable}
	}
	if retryAfter > 0 {
		service.recordLogin(ctx, meta, account, employee, "limited", "rate_limited")
		return IssuedToken{}, ServiceError{Kind: serviceErrorLoginRateLimited, RetryAfter: loginLockDuration}
	}
	service.recordLogin(ctx, meta, account, employee, "failed", "credentials_invalid")
	return IssuedToken{}, ServiceError{Kind: serviceErrorCredentialsInvalid}
}

// recordLogin 尽力保存不含密码、验证码或 Token 的后台登录结果。
func (service *Service) recordLogin(ctx context.Context, meta LoginAuditMeta, account string, employee *Employee, result, reason string) {
	if service.loginRecorder == nil {
		return
	}
	metadata, err := json.Marshal(map[string]string{"result": result, "reason": reason})
	if err != nil {
		return
	}
	entry := logging.LoginLog{
		RequestID: meta.RequestID, OccurredAt: service.now(), ActorAccount: strings.TrimSpace(account),
		ClientIP: meta.ClientIP, UserAgent: meta.UserAgent, Level: "warn", Message: "后台登录失败", Metadata: metadata,
	}
	if result == "success" {
		entry.Level = "info"
		entry.Message = "后台登录成功"
	} else if result == "limited" {
		entry.Message = "后台登录已限流"
	}
	if employee != nil {
		entry.ActorID = &employee.ID
		entry.ActorName = &employee.Name
		entry.ActorAccount = employee.LoginAccount
		entry.Workspace = &employee.Scope
		entry.TenantID = employee.TenantID
	}
	if err := service.loginRecorder.RecordLogin(ctx, entry); err != nil {
		logging.WriteEventOutput("error", "后台登录日志写入失败", meta.RequestID)
	}
}

// buildRoleResponse 转换角色响应，并判断平台超级管理员状态。
func buildRoleResponse(roles []Role, employeeScope string) ([]currentRoleResponse, []uint64, bool) {
	roleResponses := make([]currentRoleResponse, 0, len(roles))
	roleIDs := make([]uint64, 0, len(roles))
	isSuperAdmin := false
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
		roleResponses = append(roleResponses, currentRoleResponse{
			ID:        strconv.FormatUint(role.ID, 10),
			Name:      role.Name,
			SystemKey: role.SystemKey,
		})
		if role.SystemKey != nil && *role.SystemKey == "platform_super_admin" && employeeScope == "platform" {
			isSuperAdmin = true
		}
	}
	return roleResponses, roleIDs, isSuperAdmin
}

// currentAvatarURL 为当前员工头像返回临时访问地址。
func (service *Service) currentAvatarURL(ctx context.Context, employee Employee) *string {
	if employee.AvatarImageID == nil || service.avatarURLs == nil {
		return nil
	}
	signedURL, err := service.avatarURLs.AvatarURL(ctx, *employee.AvatarImageID)
	if err != nil || signedURL == "" {
		return nil
	}
	return &signedURL
}

// currentPlatformBrand 读取全平台品牌名称和图标地址。
func (service *Service) currentPlatformBrand(ctx context.Context) (string, *string, error) {
	platformName := "ReactGoForge Admin"
	var platformIconURL *string
	brand, err := service.employees.FindPlatformBrand(ctx)
	if err != nil {
		return "", nil, ServiceError{Kind: serviceErrorInternal}
	}
	if brand != nil {
		platformName = brand.Name
		if brand.IconImageID != nil {
			value := "/api/public/images/" + strconv.FormatUint(*brand.IconImageID, 10)
			platformIconURL = &value
		}
	}
	return platformName, platformIconURL, nil
}

// currentTenantInfo 读取普通租户或代管租户的展示信息。
func (service *Service) currentTenantInfo(ctx context.Context, employee Employee, identity TokenIdentity) (*string, *string, *string, error) {
	targetTenantID := employee.TenantID
	if identity.Mode == "managed" {
		targetTenantID = identity.TenantID
	}
	if targetTenantID == nil {
		return nil, nil, nil, nil
	}
	formattedTenantID := strconv.FormatUint(*targetTenantID, 10)
	tenant, err := service.employees.FindTenantByID(ctx, *targetTenantID)
	if err != nil {
		return nil, nil, nil, ServiceError{Kind: serviceErrorInternal}
	}
	if tenant == nil || tenant.Status != 1 {
		return nil, nil, nil, ServiceError{Kind: serviceErrorUnauthorized}
	}
	tenantName := tenant.Name
	var tenantIconURL *string
	if tenant.IconImageID != nil {
		value := "/api/public/images/" + strconv.FormatUint(*tenant.IconImageID, 10)
		tenantIconURL = &value
	} else {
		tenantIconURL = tenant.IconURL
	}
	return &formattedTenantID, &tenantName, tenantIconURL, nil
}

// currentMenus 读取当前工作空间全部启用菜单定义。
func (service *Service) currentMenus(ctx context.Context, workspace string) ([]currentMenuResponse, error) {
	navigationMenus, err := service.employees.ListNavigationMenus(ctx, workspace)
	if err != nil {
		return nil, ServiceError{Kind: serviceErrorInternal}
	}
	menus := make([]currentMenuResponse, 0, len(navigationMenus))
	for _, menu := range navigationMenus {
		menuName := menu.Name
		if menu.PermissionCode != nil && *menu.PermissionCode == "platform:field:view" {
			menuName = "字典管理"
		}
		var parentID *string
		if menu.ParentID != nil {
			value := strconv.FormatUint(*menu.ParentID, 10)
			parentID = &value
		}
		menus = append(menus, currentMenuResponse{
			ID: strconv.FormatUint(menu.ID, 10), ParentID: parentID, Name: menuName,
			Type: menu.Type, Scope: menu.Scope, Path: menu.Path, Component: menu.Component,
			Icon: menu.Icon, PermissionCode: menu.PermissionCode, Sort: menu.Sort,
			TenantAssignable: menu.TenantAssignable == 1, Visible: menu.Visible == 1, Status: "enabled",
		})
	}
	return menus, nil
}

// validLoginRequest 校验登录字段长度，并在启用验证码时要求四位数字答案。
func validLoginRequest(request loginRequest, captchaEnabled bool) bool {
	if request.Username == "" || utf8.RuneCountInString(request.Username) > 40 {
		return false
	}
	passwordLength := utf8.RuneCountInString(request.Password)
	if passwordLength == 0 || passwordLength > 72 {
		return false
	}
	if !captchaEnabled {
		return true
	}
	if request.CaptchaID == "" || len(request.CaptchaID) > 128 || len(request.CaptchaCode) != captchaLength {
		return false
	}
	for _, character := range request.CaptchaCode {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
