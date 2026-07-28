package rbac

import (
	stdcontext "context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const (
	tenantOwnerSystemKey       = "tenant_owner"
	tenantOwnerDisplayName     = "企业管理员"
	tenantMenuPermissionPrefix = "tenant:menu:"
)

// delegationAuthority 描述当前操作者可以继续向下授予的平台与租户权限编码集合。
type delegationAuthority struct {
	Unrestricted        bool
	PlatformPermissions map[string]struct{}
	TenantPermissions   map[string]struct{}
}

// delegationAuthorityStore 定义只读接口计算角色可配置性与可分配性所需的查询能力。
type delegationAuthorityStore interface {
	LoadDelegationAuthority(stdcontext.Context, auth.Employee, auth.TokenIdentity, managementScope, bool) (delegationAuthority, error)
	RolesWithinDelegationAuthority(stdcontext.Context, managementScope, []uint64, delegationAuthority) (map[uint64]bool, error)
}

// newDelegationAuthority 创建默认不允许越权授予的空权限集合。
func newDelegationAuthority(unrestricted bool) delegationAuthority {
	return delegationAuthority{
		Unrestricted:        unrestricted,
		PlatformPermissions: map[string]struct{}{},
		TenantPermissions:   map[string]struct{}{},
	}
}

// permissionsForScope 返回指定工作空间对应的可授予权限集合。
func (authority delegationAuthority) permissionsForScope(scope string) map[string]struct{} {
	if scope == "platform" {
		return authority.PlatformPermissions
	}
	return authority.TenantPermissions
}

// LoadDelegationAuthority 实时读取当前员工启用角色能够向下授予的权限边界。
func (store *GormStore) LoadDelegationAuthority(ctx stdcontext.Context, employee auth.Employee, identity auth.TokenIdentity, scope managementScope, unrestricted bool) (delegationAuthority, error) {
	authority := newDelegationAuthority(unrestricted)
	if identity.EmployeeID != employee.ID ||
		(identity.Mode == "normal" && employee.Scope != scope.Name) ||
		(identity.Mode == "managed" && (employee.Scope != "platform" || scope.Name != "tenant")) {
		return authority, errManagementForbidden
	}
	if unrestricted {
		return authority, nil
	}

	roleIDs := make([]uint64, 0)
	roleQuery := store.db.WithContext(ctx).
		Table("employee_roles AS er").
		Joins("JOIN roles AS r ON r.id = er.role_id AND r.scope = er.scope AND r.status = ?", 1).
		Where("er.employee_id = ? AND er.scope = ?", employee.ID, employee.Scope)
	if employee.TenantID == nil {
		roleQuery = roleQuery.Where("er.tenant_id IS NULL AND r.tenant_id IS NULL")
	} else {
		roleQuery = roleQuery.Where("er.tenant_id = ? AND r.tenant_id = ?", *employee.TenantID, *employee.TenantID)
	}
	if err := roleQuery.Distinct("r.id").Pluck("r.id", &roleIDs).Error; err != nil {
		return authority, err
	}
	if len(roleIDs) == 0 {
		return authority, nil
	}

	if scope.Name == "platform" {
		permissions, err := store.listDirectPermissionCodes(ctx, managementScope{Name: "platform"}, roleIDs)
		if err != nil {
			return authority, err
		}
		authority.PlatformPermissions = permissionCodeSet(permissions)
		managedPermissions, err := store.listManagedPermissionCodes(ctx, roleIDs)
		if err != nil {
			return authority, err
		}
		authority.TenantPermissions = permissionCodeSet(managedPermissions)
		return authority, nil
	}

	var permissions []string
	var err error
	if identity.Mode == "managed" {
		permissions, err = store.listManagedPermissionCodes(ctx, roleIDs)
	} else {
		permissions, err = store.listDirectPermissionCodes(ctx, scope, roleIDs)
	}
	if err != nil {
		return authority, err
	}
	authority.TenantPermissions = permissionCodeSet(permissions)
	return authority, nil
}

// listDirectPermissionCodes 查询普通平台或租户角色当前生效的非空权限编码。
func (store *GormStore) listDirectPermissionCodes(ctx stdcontext.Context, scope managementScope, roleIDs []uint64) ([]string, error) {
	permissions := make([]string, 0)
	query := store.db.WithContext(ctx).
		Table("role_menus AS rm").
		Joins("JOIN menus AS m ON m.id = rm.menu_id AND m.scope = rm.scope").
		Where("rm.role_id IN ? AND rm.scope = ? AND m.status = ? AND m.permission_code IS NOT NULL", roleIDs, scope.Name, 1)
	if scope.TenantID == nil {
		query = query.Where("rm.tenant_id IS NULL")
	} else {
		query = query.Where("rm.tenant_id = ?", *scope.TenantID)
	}
	err := query.Distinct("m.permission_code").Pluck("m.permission_code", &permissions).Error
	return permissions, err
}

// listManagedPermissionCodes 查询平台角色映射到租户工作空间的非空权限编码。
func (store *GormStore) listManagedPermissionCodes(ctx stdcontext.Context, roleIDs []uint64) ([]string, error) {
	permissions := make([]string, 0)
	err := store.db.WithContext(ctx).
		Table("platform_role_tenant_menus AS prtm").
		Joins("JOIN menus AS m ON m.id = prtm.menu_id").
		Where("prtm.role_id IN ? AND m.scope = ? AND m.status = ? AND m.tenant_assignable = ? AND m.permission_code IS NOT NULL", roleIDs, "tenant", 1, 1).
		Distinct("m.permission_code").
		Pluck("m.permission_code", &permissions).Error
	return permissions, err
}

// permissionCodeSet 把权限编码切片转换为便于执行子集判断的集合。
func permissionCodeSet(permissions []string) map[string]struct{} {
	result := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		result[permission] = struct{}{}
	}
	return result
}

// permissionsWithinAuthority 判断全部非空权限编码是否都处于操作者可授予范围。
func permissionsWithinAuthority(permissions []string, authority map[string]struct{}) bool {
	for _, permission := range permissions {
		if _, allowed := authority[permission]; !allowed {
			return false
		}
	}
	return true
}

// validatePermissionIDsWithinAuthority 校验待写入菜单节点不会扩大当前操作者的权限边界。
func validatePermissionIDsWithinAuthority(db *gorm.DB, scope managementScope, permissionIDs []uint64, authority delegationAuthority) error {
	if authority.Unrestricted || len(permissionIDs) == 0 {
		return nil
	}
	permissions := make([]string, 0)
	if err := db.Table("menus").
		Where("id IN ? AND scope = ? AND permission_code IS NOT NULL", permissionIDs, scope.Name).
		Distinct("permission_code").
		Pluck("permission_code", &permissions).Error; err != nil {
		return err
	}
	if !permissionsWithinAuthority(permissions, authority.permissionsForScope(scope.Name)) {
		return errManagementForbidden
	}
	return nil
}

// RolesWithinDelegationAuthority 批量判断角色的平台权限和租户代管权限是否均未超过操作者。
func (store *GormStore) RolesWithinDelegationAuthority(ctx stdcontext.Context, scope managementScope, roleIDs []uint64, authority delegationAuthority) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(roleIDs))
	for _, roleID := range roleIDs {
		result[roleID] = true
	}
	if authority.Unrestricted || len(roleIDs) == 0 {
		return result, nil
	}

	type rolePermissionRow struct {
		RoleID         uint64 `gorm:"column:role_id"`
		PermissionCode string `gorm:"column:permission_code"`
	}
	directRows := make([]rolePermissionRow, 0)
	directQuery := store.db.WithContext(ctx).
		Table("role_menus AS rm").
		Select("rm.role_id", "m.permission_code").
		Joins("JOIN menus AS m ON m.id = rm.menu_id AND m.scope = rm.scope").
		Where("rm.role_id IN ? AND rm.scope = ? AND m.status = ? AND m.permission_code IS NOT NULL", roleIDs, scope.Name, 1)
	if scope.TenantID == nil {
		directQuery = directQuery.Where("rm.tenant_id IS NULL")
	} else {
		directQuery = directQuery.Where("rm.tenant_id = ?", *scope.TenantID)
	}
	if err := directQuery.Scan(&directRows).Error; err != nil {
		return nil, err
	}
	directAuthority := authority.permissionsForScope(scope.Name)
	for _, row := range directRows {
		if _, allowed := directAuthority[row.PermissionCode]; !allowed {
			result[row.RoleID] = false
		}
	}

	if scope.Name == "platform" {
		managedRows := make([]rolePermissionRow, 0)
		if err := store.db.WithContext(ctx).
			Table("platform_role_tenant_menus AS prtm").
			Select("prtm.role_id", "m.permission_code").
			Joins("JOIN menus AS m ON m.id = prtm.menu_id").
			Where("prtm.role_id IN ? AND m.scope = ? AND m.status = ? AND m.tenant_assignable = ? AND m.permission_code IS NOT NULL", roleIDs, "tenant", 1, 1).
			Scan(&managedRows).Error; err != nil {
			return nil, err
		}
		for _, row := range managedRows {
			if _, allowed := authority.TenantPermissions[row.PermissionCode]; !allowed {
				result[row.RoleID] = false
			}
		}
	}
	return result, nil
}

// currentDelegationAuthority 从可信认证上下文加载当前请求的实时向下授权边界。
func currentDelegationAuthority(context *gin.Context, store delegationAuthorityStore, scope managementScope) (delegationAuthority, error) {
	employee, employeeValid := auth.CurrentEmployee(context)
	identity, identityValid := auth.CurrentTokenIdentity(context)
	if !employeeValid || !identityValid {
		return newDelegationAuthority(false), errManagementForbidden
	}
	return store.LoadDelegationAuthority(
		context.Request.Context(),
		employee,
		identity,
		scope,
		auth.CurrentPlatformSuperAdmin(context),
	)
}

// canConfigureRolePermissions 判断当前操作者是否允许配置指定自定义或受支持的内置角色。
func canConfigureRolePermissions(scope managementScope, role PlatformRole, withinAuthority bool, platformSuperAdmin bool) bool {
	if role.SystemKey == nil {
		return withinAuthority
	}
	if scope.Name == "platform" && *role.SystemKey == "platform_admin" {
		return platformSuperAdmin
	}
	return scope.Name == "tenant" && *role.SystemKey == tenantOwnerSystemKey && platformSuperAdmin
}

// markRolePermissionConfigurability 批量标记角色权限是否允许当前操作者配置。
func markRolePermissionConfigurability(ctx stdcontext.Context, store delegationAuthorityStore, scope managementScope, roles []PlatformRole, authority delegationAuthority, platformSuperAdmin bool) error {
	roleIDs := make([]uint64, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}
	withinAuthority, err := store.RolesWithinDelegationAuthority(ctx, scope, roleIDs, authority)
	if err != nil {
		return err
	}
	for index := range roles {
		roles[index].PermissionConfigurable = canConfigureRolePermissions(scope, roles[index], withinAuthority[roles[index].ID], platformSuperAdmin)
	}
	return nil
}

// markEmployeeRoleAssignability 批量标记员工当前角色是否处于操作者可管理范围。
func markEmployeeRoleAssignability(ctx stdcontext.Context, store delegationAuthorityStore, scope managementScope, employees []PlatformEmployee, authority delegationAuthority) error {
	roleIDs := make([]uint64, 0)
	seen := map[uint64]struct{}{}
	for _, employee := range employees {
		for _, role := range employee.Roles {
			if _, exists := seen[role.ID]; !exists {
				seen[role.ID] = struct{}{}
				roleIDs = append(roleIDs, role.ID)
			}
		}
	}
	assignable, err := store.RolesWithinDelegationAuthority(ctx, scope, roleIDs, authority)
	if err != nil {
		return err
	}
	for employeeIndex := range employees {
		for roleIndex := range employees[employeeIndex].Roles {
			roleID := employees[employeeIndex].Roles[roleIndex].ID
			employees[employeeIndex].Roles[roleIndex].Assignable = assignable[roleID]
		}
	}
	return nil
}

// markEmployeeOptionAssignability 批量标记员工表单中的角色选项是否允许当前操作者分配。
func markEmployeeOptionAssignability(ctx stdcontext.Context, store delegationAuthorityStore, scope managementScope, options *PlatformEmployeeOptions, authority delegationAuthority) error {
	roleIDs := make([]uint64, 0, len(options.Roles))
	for _, role := range options.Roles {
		roleIDs = append(roleIDs, role.ID)
	}
	assignable, err := store.RolesWithinDelegationAuthority(ctx, scope, roleIDs, authority)
	if err != nil {
		return err
	}
	for index := range options.Roles {
		options.Roles[index].Assignable = assignable[options.Roles[index].ID]
	}
	return nil
}

// validateRolesWithinAuthority 拒绝分配或移除任何权限超过当前操作者的角色。
func validateRolesWithinAuthority(ctx stdcontext.Context, store delegationAuthorityStore, scope managementScope, roleIDs []uint64, authority delegationAuthority) error {
	manageable, err := store.RolesWithinDelegationAuthority(ctx, scope, roleIDs, authority)
	if err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if !manageable[roleID] {
			return errManagementForbidden
		}
	}
	return nil
}

// validateRolesWithinAuthority 拒绝分配或移除任何权限超过当前操作者的角色。
func (store *GormStore) validateRolesWithinAuthority(ctx stdcontext.Context, scope managementScope, roleIDs []uint64, authority delegationAuthority) error {
	return validateRolesWithinAuthority(ctx, store, scope, roleIDs, authority)
}

// isCurrentScopedEmployee 判断目标是否为当前普通会话在同一工作空间中的员工本人。
func isCurrentScopedEmployee(context *gin.Context, scope managementScope, employeeID uint64) bool {
	employee, employeeValid := auth.CurrentEmployee(context)
	identity, identityValid := auth.CurrentTokenIdentity(context)
	if !employeeValid || !identityValid || identity.Mode != "normal" || employee.ID != employeeID || employee.Scope != scope.Name {
		return false
	}
	if scope.TenantID == nil {
		return employee.TenantID == nil
	}
	return employee.TenantID != nil && *employee.TenantID == *scope.TenantID
}

// filterDelegableMenus 仅保留操作者可授予节点及其必要祖先，避免权限树展示越权选项。
func filterDelegableMenus(menus []PlatformMenu, permissions map[string]struct{}, unrestricted bool, allowTenantMenuPermissions bool) []PlatformMenu {
	menuByID := make(map[uint64]PlatformMenu, len(menus))
	for _, menu := range menus {
		menuByID[menu.ID] = menu
	}
	allowedMemo := make(map[uint64]bool, len(menus))
	var nodeAllowed func(uint64) bool
	nodeAllowed = func(menuID uint64) bool {
		if allowed, exists := allowedMemo[menuID]; exists {
			return allowed
		}
		menu, exists := menuByID[menuID]
		if !exists {
			return false
		}
		allowed := menu.Status == 1
		if menu.Scope == "tenant" {
			allowed = allowed && menu.TenantAssignable == 1 && (allowTenantMenuPermissions || !isTenantMenuPermission(menu.PermissionCode))
		}
		if menu.PermissionCode != nil && !unrestricted {
			_, permissionAllowed := permissions[*menu.PermissionCode]
			allowed = allowed && permissionAllowed
		}
		if allowed && menu.ParentID != nil {
			allowed = nodeAllowed(*menu.ParentID)
		}
		allowedMemo[menuID] = allowed
		return allowed
	}

	included := make(map[uint64]struct{}, len(menus))
	for _, menu := range menus {
		if menu.PermissionCode == nil || !nodeAllowed(menu.ID) {
			continue
		}
		current := menu
		for {
			included[current.ID] = struct{}{}
			if current.ParentID == nil {
				break
			}
			parent, exists := menuByID[*current.ParentID]
			if !exists {
				break
			}
			current = parent
		}
	}
	result := make([]PlatformMenu, 0, len(included))
	for _, menu := range menus {
		if _, exists := included[menu.ID]; exists {
			result = append(result, menu)
		}
	}
	return result
}

// isTenantMenuPermission 判断权限编码是否属于租户菜单管理分组。
func isTenantMenuPermission(permissionCode *string) bool {
	return permissionCode != nil && strings.HasPrefix(*permissionCode, tenantMenuPermissionPrefix)
}

// isDefaultTenantOwnerPermission 判断租户节点是否应默认授予企业管理员。
func isDefaultTenantOwnerPermission(permissionCode *string) bool {
	return !isTenantMenuPermission(permissionCode)
}

// assignDefaultTenantOwnerPermissions 为单个租户的企业管理员写入当前默认权限快照。
func assignDefaultTenantOwnerPermissions(tx *gorm.DB, tenantID, roleID uint64) error {
	return tx.Exec(`
		INSERT INTO role_menus (scope, tenant_id, role_id, menu_id)
		SELECT 'tenant', ?, ?, m.id
		FROM menus AS m
		WHERE m.scope = 'tenant'
		  AND m.tenant_assignable = 1
		  AND (m.permission_code IS NULL OR m.permission_code NOT LIKE 'tenant:menu:%')`, tenantID, roleID).Error
}

// assignMenuToTenantOwners 把新建的可分配租户节点自动授予全部企业管理员。
func assignMenuToTenantOwners(tx *gorm.DB, menuID uint64) error {
	return tx.Exec(`
		INSERT INTO role_menus (scope, tenant_id, role_id, menu_id)
		SELECT 'tenant', r.tenant_id, r.id, ?
		FROM roles AS r
		WHERE r.scope = 'tenant'
		  AND r.system_key = ?`, menuID, tenantOwnerSystemKey).Error
}

// normalizeOptionalText 将可选文本清理为空值或去除首尾空白后的内容。
func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// validPassword 校验后台员工密码长度。
func validPassword(password string) bool {
	length := utf8.RuneCountInString(password)
	return length >= 6 && length <= 18
}

// parseStatus 将接口状态转换为数据库状态值。
func parseStatus(status string) (uint8, bool) {
	switch strings.TrimSpace(status) {
	case "enabled":
		return 1, true
	case "disabled":
		return 0, true
	default:
		return 0, false
	}
}

// parseOptionalID 解析可选字符串 BIGINT ID。
func parseOptionalID(value *string) (*uint64, bool) {
	// Go 学习提示：*string 可以区分 JSON 字段为空和存在具体字符串；返回的 *uint64 同样保留可空语义。
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, true
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(*value), 10, 64)
	if err != nil || parsed == 0 {
		return nil, false
	}
	return &parsed, true
}

// parseIDs 解析去重后的字符串 BIGINT ID 数组。
func parseIDs(values []string) ([]uint64, bool) {
	result := make([]uint64, 0, len(values))
	seen := map[uint64]struct{}{}
	for _, value := range values {
		parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil || parsed == 0 {
			return nil, false
		}
		if _, ok := seen[parsed]; !ok {
			seen[parsed] = struct{}{}
			result = append(result, parsed)
		}
	}
	return result, true
}

// scopedTable 为数据库查询强制添加工作空间与租户隔离条件。
func scopedTable(query *gorm.DB, scope managementScope) *gorm.DB {
	// 安全边界：所有无别名 RBAC 查询统一经过这里追加 scope 与 tenant_id，防止漏写租户条件。
	query = query.Where("scope = ?", scope.Name)
	if scope.TenantID == nil {
		return query.Where("tenant_id IS NULL")
	}
	return query.Where("tenant_id = ?", *scope.TenantID)
}

// scopedAlias 为带别名的数据库查询添加明确的工作空间与租户隔离条件。
func scopedAlias(query *gorm.DB, scope managementScope, alias string) *gorm.DB {
	// Go 学习提示：带表别名的联表查询必须给列名加 alias 前缀，避免 SQL 中字段名歧义。
	query = query.Where(alias+".scope = ?", scope.Name)
	if scope.TenantID == nil {
		return query.Where(alias + ".tenant_id IS NULL")
	}
	return query.Where(alias+".tenant_id = ?", *scope.TenantID)
}

// ensureScopedExists 确认指定数据仍存在于当前工作空间。
func ensureScopedExists(db *gorm.DB, table string, scope managementScope, id uint64) error {
	var count int64
	if err := scopedTable(db.Table(table), scope).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errManagementNotFound
	}
	return nil
}

// normalizeWriteError 将数据库约束错误转换为稳定业务错误。
func normalizeWriteError(err error) error {
	// Go 学习提示：errors.As 可以沿错误包装链提取具体的 MySQL 错误类型，
	// 再把数据库编号转换为稳定业务错误，避免把底层 SQL 细节暴露给客户端。
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && (mysqlError.Number == 1062 || mysqlError.Number == 1451 || mysqlError.Number == 1452) {
		return errManagementConflict
	}
	return err
}

// writeManagementError 将管理业务错误转换为统一 HTTP 错误码。
func writeManagementError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, errManagementNotFound):
		httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
	case errors.Is(err, errManagementInvalid):
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
	case errors.Is(err, errManagementConflict):
		httpapi.WriteError(context, httpapi.ErrorCodeConflict)
	case errors.Is(err, errManagementProtected):
		httpapi.WriteError(context, httpapi.ErrorCodeProtectedResource)
	case errors.Is(err, errManagementForbidden):
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
	default:
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
	}
}
