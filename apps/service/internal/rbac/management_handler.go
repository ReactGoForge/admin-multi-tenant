package rbac

import (
	"errors"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

var (
	errManagementNotFound  = errors.New("management resource not found")
	errManagementInvalid   = errors.New("management resource invalid")
	errManagementConflict  = errors.New("management resource conflict")
	errManagementProtected = errors.New("management resource protected")
	errManagementForbidden = errors.New("management operation forbidden")
)

// managementScope 描述一次管理操作被服务端限定的平台或租户数据范围。
type managementScope struct {
	// Go 学习提示：TenantID 为 nil 表示平台范围；非 nil 表示所有查询必须限制到该租户。
	Name     string
	TenantID *uint64
}

// employeeInsertRow 映射员工创建事务写入的数据库字段。
type employeeInsertRow struct {
	ID           uint64  `gorm:"column:id"`
	Scope        string  `gorm:"column:scope"`
	TenantID     *uint64 `gorm:"column:tenant_id"`
	DepartmentID *uint64 `gorm:"column:department_id"`
	Name         string  `gorm:"column:name"`
	LoginAccount string  `gorm:"column:login_account"`
	PasswordHash string  `gorm:"column:password_hash"`
	Phone        *string `gorm:"column:phone"`
	Status       uint8   `gorm:"column:status"`
}

// employeeDeleteRow 映射员工删除前需要锁定和校验的字段。
type employeeDeleteRow struct {
	ID     uint64 `gorm:"column:id"`
	Status uint8  `gorm:"column:status"`
}

// roleInsertRow 映射角色创建事务写入的数据库字段。
type roleInsertRow struct {
	ID          uint64  `gorm:"column:id"`
	Scope       string  `gorm:"column:scope"`
	TenantID    *uint64 `gorm:"column:tenant_id"`
	Name        string  `gorm:"column:name"`
	Description *string `gorm:"column:description"`
	SystemKey   *string `gorm:"column:system_key"`
	Status      uint8   `gorm:"column:status"`
}

// departmentInsertRow 映射部门创建和编辑使用的数据库字段。
type departmentInsertRow struct {
	ID               uint64  `gorm:"column:id"`
	Scope            string  `gorm:"column:scope"`
	TenantID         *uint64 `gorm:"column:tenant_id"`
	ParentID         *uint64 `gorm:"column:parent_id"`
	Name             string  `gorm:"column:name"`
	LeaderEmployeeID *uint64 `gorm:"column:leader_employee_id"`
	Sort             uint32  `gorm:"column:sort"`
	Status           uint8   `gorm:"column:status"`
}

// tenantScopeFromContext 从认证上下文提取租户 ID，拒绝客户端自行选择租户。
func tenantScopeFromContext(context *gin.Context) (managementScope, bool) {
	// 安全边界：租户 ID 只从认证中间件写入的可信上下文读取，绝不接受客户端自行指定。
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return managementScope{}, false
	}
	return managementScope{Name: "tenant", TenantID: &tenantID}, true
}
