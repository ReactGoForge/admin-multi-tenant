package logging

import (
	"context"
	"testing"
)

// TestCreateEventSkipsDatabaseWhenDisabled 验证关闭运行事件入库时不会访问数据库。
func TestCreateEventSkipsDatabaseWhenDisabled(t *testing.T) {
	store := &Store{eventDBEnabled: false}
	if err := store.CreateEvent(context.Background(), "info", "测试事件", nil); err != nil {
		t.Fatalf("关闭运行事件入库后仍返回错误: %v", err)
	}
}
