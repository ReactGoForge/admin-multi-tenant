package user

import (
	"sync"
	"testing"
	"time"

	"gorm.io/gorm/schema"
)

// TestTenantUserJoinedAtUsesAutoCreateTime 验证首次建立租户归属时由 GORM 写入有效加入时间。
func TestTenantUserJoinedAtUsesAutoCreateTime(t *testing.T) {
	modelSchema, err := schema.Parse(&tenantUserRow{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("解析 tenantUserRow 失败: %v", err)
	}
	joinedAtField := modelSchema.LookUpField("JoinedAt")
	if joinedAtField == nil || joinedAtField.AutoCreateTime == 0 {
		t.Fatal("JoinedAt 未启用 GORM 自动创建时间")
	}
}

// TestNewUserFromListRowPreservesIdentityAndTimes 验证显式扫描行不会丢失用户 ID、状态和时间。
func TestNewUserFromListRowPreservesIdentityAndTimes(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 10, 3, 57, 0, time.Local)
	currentUser := newUserFromListRow(userListRow{
		ID: 4, WechatOpenID: "openid", Status: 1, CreatedAt: createdAt, UpdatedAt: createdAt,
	})
	if currentUser.ID != 4 || currentUser.Status != 1 || !currentUser.CreatedAt.Equal(createdAt) {
		t.Fatalf("用户扫描转换结果 = %#v", currentUser)
	}
}
