package rbac

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

// TenantQuery 描述平台租户列表已经校验过的分页和筛选条件。
type TenantQuery struct {
	Page     int
	PageSize int
	Name     string
	Status   *uint8
}

// PlatformTenant 描述平台租户列表返回所需的只读字段。
type PlatformTenant struct {
	ID              uint64
	Name            string
	Remark          *string
	IconURL         *string
	Status          uint8
	OwnerEmployeeID *uint64
	OwnerName       *string
	LoginAccount    *string
}

// TenantCreateInput 描述租户及其所有者初始化所需字段。
type TenantCreateInput struct {
	Name         string
	OwnerName    string
	LoginAccount string
	Password     string
}

// TenantUpdateInput 描述平台编辑租户及所有者账号所需字段。
type TenantUpdateInput struct {
	Name         string
	LoginAccount string
	Remark       *string
}

// TenantApplication 定义 Tenant Handler 依赖的业务服务能力。
type TenantApplication interface {
	ListTenants(context.Context, TenantQuery) ([]PlatformTenant, int64, error)
	CreateTenant(context.Context, TenantCreateInput) error
	UpdateTenant(context.Context, uint64, TenantUpdateInput) error
	ResetTenantOwnerPassword(context.Context, uint64, string) error
	SetTenantStatus(context.Context, uint64, uint8) error
	DeleteTenant(context.Context, uint64) error
}

// TenantDataStore 定义 Tenant Service 需要的数据访问能力。
type TenantDataStore interface {
	ListTenants(context.Context, TenantQuery) ([]PlatformTenant, int64, error)
	UpdateTenantStatus(context.Context, uint64, uint8) (bool, error)
	EnsureTenantExists(context.Context, uint64) error
	WithTenantTransaction(context.Context, func(TenantTransactionStore) error) error
}

// TenantTransactionStore 定义租户生命周期事务内需要的数据访问能力。
type TenantTransactionStore interface {
	CreateTenantRecord(context.Context, string) (uint64, error)
	CreateTenantOwnerRole(context.Context, uint64) (uint64, error)
	AssignDefaultTenantOwnerPermissions(context.Context, uint64, uint64) error
	CreateTenantOwnerEmployee(context.Context, uint64, TenantCreateInput, string) (uint64, error)
	CreateTenantOwnerRelation(context.Context, uint64, uint64, uint64) error
	SetTenantOwner(context.Context, uint64, uint64) error
	FindTenantOwner(context.Context, uint64, bool) (*tenantOwnerRecord, error)
	EnsureTenantOwnerEmployee(context.Context, uint64, uint64) error
	UpdateTenantAndOwner(context.Context, uint64, uint64, TenantUpdateInput) error
	UpdateTenantOwnerPassword(context.Context, uint64, uint64, string) (bool, error)
	ValidateEmptyTenantForDelete(context.Context, uint64, uint64) (uint64, error)
	DeleteTenantBootstrapRecords(context.Context, uint64, uint64, uint64) error
}

// TenantService 编排租户生命周期业务规则、密码哈希和事务边界。
type TenantService struct {
	store TenantDataStore
}

// NewTenantService 使用租户数据访问能力创建租户服务。
func NewTenantService(store TenantDataStore) *TenantService {
	return &TenantService{store: store}
}

// ListTenants 返回平台租户分页列表。
func (service *TenantService) ListTenants(ctx context.Context, query TenantQuery) ([]PlatformTenant, int64, error) {
	return service.store.ListTenants(ctx, query)
}

// CreateTenant 在单个事务内创建租户、企业管理员角色、所有者员工和默认权限快照。
func (service *TenantService) CreateTenant(ctx context.Context, input TenantCreateInput) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return service.store.WithTenantTransaction(ctx, func(tx TenantTransactionStore) error {
		tenantID, err := tx.CreateTenantRecord(ctx, input.Name)
		if err != nil {
			return normalizeWriteError(err)
		}
		roleID, err := tx.CreateTenantOwnerRole(ctx, tenantID)
		if err != nil {
			return normalizeWriteError(err)
		}
		if err := tx.AssignDefaultTenantOwnerPermissions(ctx, tenantID, roleID); err != nil {
			return normalizeWriteError(err)
		}
		ownerID, err := tx.CreateTenantOwnerEmployee(ctx, tenantID, input, string(hash))
		if err != nil {
			return normalizeWriteError(err)
		}
		if err := tx.CreateTenantOwnerRelation(ctx, tenantID, ownerID, roleID); err != nil {
			return normalizeWriteError(err)
		}
		return normalizeWriteError(tx.SetTenantOwner(ctx, tenantID, ownerID))
	})
}

// UpdateTenant 在单个事务内更新租户信息和当前所有者登录账号。
func (service *TenantService) UpdateTenant(ctx context.Context, tenantID uint64, input TenantUpdateInput) error {
	return service.store.WithTenantTransaction(ctx, func(tx TenantTransactionStore) error {
		owner, err := tx.FindTenantOwner(ctx, tenantID, false)
		if err != nil {
			return err
		}
		if owner.OwnerEmployeeID == nil {
			return errManagementConflict
		}
		if err := tx.EnsureTenantOwnerEmployee(ctx, tenantID, *owner.OwnerEmployeeID); err != nil {
			return err
		}
		return normalizeWriteError(tx.UpdateTenantAndOwner(ctx, tenantID, *owner.OwnerEmployeeID, input))
	})
}

// ResetTenantOwnerPassword 重置租户当前所有者员工的登录密码。
func (service *TenantService) ResetTenantOwnerPassword(ctx context.Context, tenantID uint64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return service.store.WithTenantTransaction(ctx, func(tx TenantTransactionStore) error {
		owner, err := tx.FindTenantOwner(ctx, tenantID, false)
		if err != nil {
			return err
		}
		if owner.OwnerEmployeeID == nil {
			return errManagementConflict
		}
		changed, err := tx.UpdateTenantOwnerPassword(ctx, tenantID, *owner.OwnerEmployeeID, string(hash))
		if err != nil {
			return normalizeWriteError(err)
		}
		if !changed {
			return errManagementConflict
		}
		return nil
	})
}

// SetTenantStatus 启用或禁用租户，认证中间件会实时应用该状态。
func (service *TenantService) SetTenantStatus(ctx context.Context, tenantID uint64, status uint8) error {
	changed, err := service.store.UpdateTenantStatus(ctx, tenantID, status)
	if err != nil {
		return normalizeWriteError(err)
	}
	if !changed {
		return service.store.EnsureTenantExists(ctx, tenantID)
	}
	return nil
}

// DeleteTenant 删除已停用且未产生业务数据的空租户。
func (service *TenantService) DeleteTenant(ctx context.Context, tenantID uint64) error {
	return service.store.WithTenantTransaction(ctx, func(tx TenantTransactionStore) error {
		owner, err := tx.FindTenantOwner(ctx, tenantID, true)
		if err != nil {
			return err
		}
		if owner.Status != 0 || owner.OwnerEmployeeID == nil {
			return errManagementConflict
		}
		ownerRoleID, err := tx.ValidateEmptyTenantForDelete(ctx, tenantID, *owner.OwnerEmployeeID)
		if err != nil {
			return err
		}
		return normalizeWriteError(tx.DeleteTenantBootstrapRecords(ctx, tenantID, *owner.OwnerEmployeeID, ownerRoleID))
	})
}
