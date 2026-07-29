package rbac

import "context"

// PlatformDepartment 描述平台部门树所需的只读字段和统计信息。
type PlatformDepartment struct {
	ID               uint64  `gorm:"column:id"`
	ParentID         *uint64 `gorm:"column:parent_id"`
	Name             string  `gorm:"column:name"`
	LeaderEmployeeID *uint64 `gorm:"column:leader_employee_id"`
	LeaderName       *string `gorm:"column:leader_name"`
	EmployeeCount    int64   `gorm:"column:employee_count"`
	Sort             uint32  `gorm:"column:sort"`
	Status           uint8   `gorm:"column:status"`
}

// DepartmentDataStore 定义 Department Service 需要的数据访问能力。
type DepartmentDataStore interface {
	ListDepartments(context.Context, managementScope) ([]PlatformDepartment, error)
	ValidateDepartment(context.Context, managementScope, *uint64, *uint64) error
	ValidateDepartmentLeader(context.Context, managementScope, *uint64) error
	CreateDepartment(context.Context, managementScope, DepartmentMutation) error
	UpdateDepartment(context.Context, managementScope, uint64, DepartmentMutation) (bool, error)
	EnsureDepartmentExists(context.Context, managementScope, uint64) error
	DepartmentChildCount(context.Context, managementScope, uint64) (int64, error)
	DepartmentEmployeeCount(context.Context, managementScope, uint64) (int64, error)
	DeleteDepartment(context.Context, managementScope, uint64) (bool, error)
}

// DepartmentMutation 描述部门新增和编辑接口经过校验后的字段。
type DepartmentMutation struct {
	ParentID         *uint64
	Name             string
	LeaderEmployeeID *uint64
	Sort             uint32
	Status           uint8
}

// DepartmentService 编排部门业务规则和持久化错误归一化。
type DepartmentService struct {
	store DepartmentDataStore
}

// NewDepartmentService 使用部门数据访问能力创建部门服务。
func NewDepartmentService(store DepartmentDataStore) *DepartmentService {
	return &DepartmentService{store: store}
}

// ListDepartments 返回指定工作空间的部门平铺列表。
func (service *DepartmentService) ListDepartments(ctx context.Context, scope managementScope) ([]PlatformDepartment, error) {
	return service.store.ListDepartments(ctx, scope)
}

// CreateDepartment 创建已校验父部门和负责人的部门。
func (service *DepartmentService) CreateDepartment(ctx context.Context, scope managementScope, mutation DepartmentMutation) error {
	if err := service.store.ValidateDepartment(ctx, scope, mutation.ParentID, nil); err != nil {
		return err
	}
	if err := service.store.ValidateDepartmentLeader(ctx, scope, mutation.LeaderEmployeeID); err != nil {
		return err
	}
	return normalizeWriteError(service.store.CreateDepartment(ctx, scope, mutation))
}

// UpdateDepartment 更新部门并拒绝形成循环层级。
func (service *DepartmentService) UpdateDepartment(ctx context.Context, scope managementScope, departmentID uint64, mutation DepartmentMutation) error {
	if err := service.store.ValidateDepartment(ctx, scope, mutation.ParentID, &departmentID); err != nil {
		return err
	}
	if err := service.store.ValidateDepartmentLeader(ctx, scope, mutation.LeaderEmployeeID); err != nil {
		return err
	}
	changed, err := service.store.UpdateDepartment(ctx, scope, departmentID, mutation)
	if err != nil {
		return normalizeWriteError(err)
	}
	if !changed {
		return service.store.EnsureDepartmentExists(ctx, scope, departmentID)
	}
	return nil
}

// DeleteDepartment 删除无子部门且无员工的部门。
func (service *DepartmentService) DeleteDepartment(ctx context.Context, scope managementScope, departmentID uint64) error {
	childCount, err := service.store.DepartmentChildCount(ctx, scope, departmentID)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return errManagementConflict
	}
	employeeCount, err := service.store.DepartmentEmployeeCount(ctx, scope, departmentID)
	if err != nil {
		return err
	}
	if employeeCount > 0 {
		return errManagementConflict
	}
	changed, err := service.store.DeleteDepartment(ctx, scope, departmentID)
	if err != nil {
		return normalizeWriteError(err)
	}
	if !changed {
		return errManagementNotFound
	}
	return nil
}
