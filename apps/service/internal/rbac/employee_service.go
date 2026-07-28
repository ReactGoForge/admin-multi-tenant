package rbac

import (
	"context"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"

	"golang.org/x/crypto/bcrypt"
)

// EmployeeDataStore 定义 Employee Service 需要的数据访问能力。
type EmployeeDataStore interface {
	delegationAuthorityStore
	ListEmployees(context.Context, managementScope, PlatformEmployeeQuery) ([]PlatformEmployee, int64, error)
	ListEmployeeOptions(context.Context, managementScope) (PlatformEmployeeOptions, error)
	ValidateDepartment(context.Context, managementScope, *uint64, *uint64) error
	IsProtectedEmployee(context.Context, managementScope, uint64) (bool, error)
	EnsureEmployeeExists(context.Context, managementScope, uint64) error
	UpdateEmployee(context.Context, managementScope, uint64, EmployeeUpdate) (bool, error)
	ResetEmployeePassword(context.Context, managementScope, uint64, string) (bool, error)
	SetEmployeeStatus(context.Context, managementScope, uint64, uint8) (bool, error)
	WithEmployeeTransaction(context.Context, func(EmployeeTransactionStore) error) error
}

// EmployeeTransactionStore 定义 Employee 事务内需要的数据访问能力。
type EmployeeTransactionStore interface {
	delegationAuthorityStore
	ValidateDepartment(context.Context, managementScope, *uint64, *uint64) error
	ValidateEmployeeRoles(context.Context, managementScope, []uint64) error
	CreateEmployee(context.Context, managementScope, EmployeeCreate) (uint64, error)
	ReplaceEmployeeRoles(context.Context, managementScope, uint64, []uint64) error
	IsProtectedEmployee(context.Context, managementScope, uint64) (bool, error)
	ListEmployeeRoleIDs(context.Context, managementScope, uint64) ([]uint64, error)
	EnsureEmployeeExists(context.Context, managementScope, uint64) error
	FindEmployeeForDelete(context.Context, managementScope, uint64) (employeeDeleteRow, error)
	HasAnySystemRole(context.Context, managementScope, uint64) (bool, error)
	IsTenantOwnerEmployee(context.Context, managementScope, uint64) (bool, error)
	HasEmployeeDeleteReference(context.Context, managementScope, uint64) (bool, error)
	DeleteEmployeeRoles(context.Context, managementScope, uint64) error
	DeleteEmployee(context.Context, managementScope, uint64) error
}

// EmployeeCreate 描述 Store 创建员工需要的数据库字段。
type EmployeeCreate struct {
	DepartmentID *uint64
	Name         string
	LoginAccount string
	PasswordHash string
	Phone        *string
	Status       uint8
}

// EmployeeUpdate 描述 Store 更新员工基础资料需要的字段。
type EmployeeUpdate struct {
	DepartmentID *uint64
	Name         string
	LoginAccount string
	Phone        *string
}

// EmployeeActor 描述 Handler 从认证上下文提取出的可信操作者身份。
type EmployeeActor struct {
	Employee           auth.Employee
	Identity           auth.TokenIdentity
	PlatformSuperAdmin bool
}

// EmployeeService 编排员工业务规则和事务边界。
type EmployeeService struct {
	store EmployeeDataStore
}

// NewEmployeeService 使用员工数据访问能力创建员工服务。
func NewEmployeeService(store EmployeeDataStore) *EmployeeService {
	return &EmployeeService{store: store}
}

// ListEmployees 返回指定工作空间员工分页列表，并标记角色可分配性。
func (service *EmployeeService) ListEmployees(ctx context.Context, actor EmployeeActor, scope managementScope, query PlatformEmployeeQuery) ([]PlatformEmployee, int64, error) {
	if scope.Name == "platform" {
		query.VisibleProtectedEmployeeID = platformActorEmployeeID(actor)
	}
	employees, total, err := service.store.ListEmployees(ctx, scope, query)
	if err != nil {
		return nil, 0, err
	}
	authority, err := service.currentAuthority(ctx, actor, scope)
	if err != nil {
		return nil, 0, err
	}
	if err := markEmployeeRoleAssignability(ctx, service.store, scope, employees, authority); err != nil {
		return nil, 0, err
	}
	return employees, total, nil
}

// ListEmployeeOptions 返回指定工作空间员工表单和筛选选项，并标记角色可分配性。
func (service *EmployeeService) ListEmployeeOptions(ctx context.Context, actor EmployeeActor, scope managementScope) (PlatformEmployeeOptions, error) {
	options, err := service.store.ListEmployeeOptions(ctx, scope)
	if err != nil {
		return PlatformEmployeeOptions{}, err
	}
	authority, err := service.currentAuthority(ctx, actor, scope)
	if err != nil {
		return PlatformEmployeeOptions{}, err
	}
	if err := markEmployeeOptionAssignability(ctx, service.store, scope, &options, authority); err != nil {
		return PlatformEmployeeOptions{}, err
	}
	return options, nil
}

// CreateEmployee 创建员工并在同一事务中写入角色关联。
func (service *EmployeeService) CreateEmployee(ctx context.Context, actor EmployeeActor, scope managementScope, mutation EmployeeMutation) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(mutation.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return service.store.WithEmployeeTransaction(ctx, func(tx EmployeeTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		if err := tx.ValidateDepartment(ctx, scope, mutation.DepartmentID, nil); err != nil {
			return err
		}
		if err := tx.ValidateEmployeeRoles(ctx, scope, mutation.RoleIDs); err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, mutation.RoleIDs, authority); err != nil {
			return err
		}
		employeeID, err := tx.CreateEmployee(ctx, scope, EmployeeCreate{
			DepartmentID: mutation.DepartmentID,
			Name:         mutation.Name,
			LoginAccount: mutation.LoginAccount,
			PasswordHash: string(hash),
			Phone:        mutation.Phone,
			Status:       mutation.Status,
		})
		if err != nil {
			return err
		}
		return tx.ReplaceEmployeeRoles(ctx, scope, employeeID, mutation.RoleIDs)
	})
}

// UpdateEmployee 更新员工基础资料，但不隐式修改密码和角色。
func (service *EmployeeService) UpdateEmployee(ctx context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, mutation EmployeeMutation) error {
	isCurrentEmployee := actorIsCurrentScopedEmployee(actor, scope, employeeID)
	if scope.Name == "platform" && !isCurrentEmployee {
		protected, err := service.store.IsProtectedEmployee(ctx, scope, employeeID)
		if err != nil {
			return err
		}
		if protected {
			return errManagementProtected
		}
	}
	if err := service.store.ValidateDepartment(ctx, scope, mutation.DepartmentID, nil); err != nil {
		return err
	}
	changed, err := service.store.UpdateEmployee(ctx, scope, employeeID, EmployeeUpdate{
		DepartmentID: mutation.DepartmentID,
		Name:         mutation.Name,
		LoginAccount: mutation.LoginAccount,
		Phone:        mutation.Phone,
	})
	if err != nil {
		return normalizeWriteError(err)
	}
	if !changed {
		return service.store.EnsureEmployeeExists(ctx, scope, employeeID)
	}
	return nil
}

// AssignEmployeeRoles 替换员工角色并保护所有者和越权角色。
func (service *EmployeeService) AssignEmployeeRoles(ctx context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, roleIDs []uint64) error {
	if actorIsCurrentScopedEmployee(actor, scope, employeeID) {
		return errManagementForbidden
	}
	return service.store.WithEmployeeTransaction(ctx, func(tx EmployeeTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		if protected, err := tx.IsProtectedEmployee(ctx, scope, employeeID); err != nil {
			return err
		} else if protected {
			return errManagementProtected
		}
		if err := tx.ValidateEmployeeRoles(ctx, scope, roleIDs); err != nil {
			return err
		}
		currentRoleIDs, err := tx.ListEmployeeRoleIDs(ctx, scope, employeeID)
		if err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, currentRoleIDs, authority); err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, roleIDs, authority); err != nil {
			return err
		}
		if err := tx.EnsureEmployeeExists(ctx, scope, employeeID); err != nil {
			return err
		}
		return tx.ReplaceEmployeeRoles(ctx, scope, employeeID, roleIDs)
	})
}

// ResetEmployeePassword 使用 bcrypt 替换指定员工密码哈希。
func (service *EmployeeService) ResetEmployeePassword(ctx context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, password string) error {
	if actorIsCurrentScopedEmployee(actor, scope, employeeID) {
		return errManagementForbidden
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if scope.Name == "platform" {
		protected, checkErr := service.store.IsProtectedEmployee(ctx, scope, employeeID)
		if checkErr != nil {
			return checkErr
		}
		if protected {
			return errManagementProtected
		}
	}
	changed, err := service.store.ResetEmployeePassword(ctx, scope, employeeID, string(hash))
	if err != nil {
		return normalizeWriteError(err)
	}
	if !changed {
		return service.store.EnsureEmployeeExists(ctx, scope, employeeID)
	}
	return nil
}

// SetEmployeeStatus 更新员工状态并禁止停用内置所有者。
func (service *EmployeeService) SetEmployeeStatus(ctx context.Context, actor EmployeeActor, scope managementScope, employeeID uint64, status uint8) error {
	if actorIsCurrentScopedEmployee(actor, scope, employeeID) {
		return errManagementForbidden
	}
	if status == 0 {
		protected, err := service.store.IsProtectedEmployee(ctx, scope, employeeID)
		if err != nil {
			return err
		}
		if protected {
			return errManagementProtected
		}
	}
	changed, err := service.store.SetEmployeeStatus(ctx, scope, employeeID, status)
	if err != nil {
		return normalizeWriteError(err)
	}
	if !changed {
		return service.store.EnsureEmployeeExists(ctx, scope, employeeID)
	}
	return nil
}

// DeleteEmployee 在事务内校验并删除已停用、无业务引用的普通员工。
func (service *EmployeeService) DeleteEmployee(ctx context.Context, actor EmployeeActor, scope managementScope, employeeID uint64) error {
	if actorIsCurrentScopedEmployee(actor, scope, employeeID) {
		return errManagementProtected
	}
	return service.store.WithEmployeeTransaction(ctx, func(tx EmployeeTransactionStore) error {
		authority, err := service.currentAuthorityWithStore(ctx, tx, actor, scope)
		if err != nil {
			return err
		}
		employee, err := tx.FindEmployeeForDelete(ctx, scope, employeeID)
		if err != nil {
			return err
		}
		if employee.Status != 0 {
			return errManagementConflict
		}
		if protected, err := tx.HasAnySystemRole(ctx, scope, employeeID); err != nil {
			return err
		} else if protected {
			return errManagementProtected
		}
		if owner, err := tx.IsTenantOwnerEmployee(ctx, scope, employeeID); err != nil {
			return err
		} else if owner {
			return errManagementProtected
		}
		roleIDs, err := tx.ListEmployeeRoleIDs(ctx, scope, employeeID)
		if err != nil {
			return err
		}
		if err := validateRolesWithinAuthority(ctx, tx, scope, roleIDs, authority); err != nil {
			return err
		}
		if referenced, err := tx.HasEmployeeDeleteReference(ctx, scope, employeeID); err != nil {
			return err
		} else if referenced {
			return errManagementConflict
		}
		if err := tx.DeleteEmployeeRoles(ctx, scope, employeeID); err != nil {
			return err
		}
		return tx.DeleteEmployee(ctx, scope, employeeID)
	})
}

// currentAuthority 读取当前请求的实时向下授权边界。
func (service *EmployeeService) currentAuthority(ctx context.Context, actor EmployeeActor, scope managementScope) (delegationAuthority, error) {
	return service.currentAuthorityWithStore(ctx, service.store, actor, scope)
}

// currentAuthorityWithStore 从指定 Store 读取当前请求的实时向下授权边界。
func (service *EmployeeService) currentAuthorityWithStore(ctx context.Context, store delegationAuthorityStore, actor EmployeeActor, scope managementScope) (delegationAuthority, error) {
	return store.LoadDelegationAuthority(
		ctx,
		actor.Employee,
		actor.Identity,
		scope,
		actor.PlatformSuperAdmin,
	)
}

// platformActorEmployeeID 返回平台员工身份的 ID，用于只向本人放行受保护记录。
func platformActorEmployeeID(actor EmployeeActor) *uint64 {
	if actor.Employee.Scope != "platform" {
		return nil
	}
	employeeID := actor.Employee.ID
	return &employeeID
}

// actorIsCurrentScopedEmployee 判断目标是否为当前普通会话在同一工作空间中的员工本人。
func actorIsCurrentScopedEmployee(actor EmployeeActor, scope managementScope, employeeID uint64) bool {
	if actor.Identity.Mode != "normal" || actor.Employee.ID != employeeID || actor.Employee.Scope != scope.Name {
		return false
	}
	if scope.TenantID == nil {
		return actor.Employee.TenantID == nil
	}
	return actor.Employee.TenantID != nil && *actor.Employee.TenantID == *scope.TenantID
}
