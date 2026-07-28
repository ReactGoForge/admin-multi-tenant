package httpapi

import (
	"os"
	"strings"
	"testing"
)

// TestOpenAPIContainsPilotRoutes 验证契约已登记本阶段新增探针和两端真实试点接口。
func TestOpenAPIContainsPilotRoutes(t *testing.T) {
	content, err := os.ReadFile("../../docs/openapi/openapi.yaml")
	if err != nil {
		t.Fatalf("读取 OpenAPI 契约失败: %v", err)
	}
	contract := string(content)
	for _, path := range []string{
		"/healthz:",
		"/readyz:",
		"/api/admin/platform/users:",
		"/api/miniapp/auth/login:",
		"/api/miniapp/me:",
	} {
		if !strings.Contains(contract, "  "+path) {
			t.Errorf("OpenAPI 缺少试点路由 %s", path)
		}
	}
}
