package httpapi

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.yaml.in/yaml/v3"
)

var openAPIPathParameterPattern = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9]*)\}`)

// TestOpenAPICoversRegisteredRoutes 验证多文件契约完整覆盖 Gin 注册路由且 operationId 全局唯一。
func TestOpenAPICoversRegisteredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contractPath := filepath.Clean("../../docs/openapi/openapi.yaml")
	root := readYAMLDocument(t, contractPath)
	validateExternalReferences(t, contractPath, root, map[string]bool{})

	paths := mappingValue(t, root, "paths")
	actual := make([]string, 0)
	operationIDs := make(map[string]string)
	for index := 0; index < len(paths.Content); index += 2 {
		openAPIPath := paths.Content[index].Value
		pathReference := paths.Content[index+1]
		reference := mappingValue(t, pathReference, "$ref").Value
		pathItemPath, pathItem := resolveYAMLReference(t, contractPath, reference)
		for itemIndex := 0; itemIndex < len(pathItem.Content); itemIndex += 2 {
			method := strings.ToUpper(pathItem.Content[itemIndex].Value)
			if !isHTTPMethod(method) {
				continue
			}
			operation := pathItem.Content[itemIndex+1]
			operationID := mappingValue(t, operation, "operationId").Value
			if operationID == "" {
				t.Fatalf("%s %s 缺少 operationId", method, openAPIPath)
			}
			if previous, exists := operationIDs[operationID]; exists {
				t.Fatalf("operationId %q 重复用于 %s 和 %s %s", operationID, previous, method, openAPIPath)
			}
			operationIDs[operationID] = method + " " + openAPIPath
			actual = append(actual, method+" "+normalizeOpenAPIPath(openAPIPath))
		}
		if pathItemPath == "" {
			t.Fatalf("无法解析路径项 %s", openAPIPath)
		}
	}

	router := NewRouter(routesWithNoopHandlers())
	expected := make([]string, 0, len(router.Routes()))
	for _, route := range router.Routes() {
		expected = append(expected, route.Method+" "+route.Path)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("OpenAPI 与 Gin 路由表不一致\n契约: %v\n路由: %v", actual, expected)
	}
}

// readYAMLDocument 读取并解析指定 YAML 文档。
func readYAMLDocument(t *testing.T, documentPath string) *yaml.Node {
	t.Helper()
	content, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatalf("读取 YAML 文档 %s 失败: %v", documentPath, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("解析 YAML 文档 %s 失败: %v", documentPath, err)
	}
	if len(document.Content) != 1 {
		t.Fatalf("YAML 文档 %s 缺少根节点", documentPath)
	}
	return document.Content[0]
}

// mappingValue 从 YAML 映射节点中读取指定键对应的值。
func mappingValue(t *testing.T, mapping *yaml.Node, key string) *yaml.Node {
	t.Helper()
	if mapping.Kind != yaml.MappingNode {
		t.Fatalf("读取键 %q 时节点不是映射", key)
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	t.Fatalf("YAML 映射缺少键 %q", key)
	return nil
}

// resolveYAMLReference 解析相对于当前文档的外部 YAML 引用和 JSON Pointer。
func resolveYAMLReference(t *testing.T, currentPath string, reference string) (string, *yaml.Node) {
	t.Helper()
	parts := strings.SplitN(reference, "#", 2)
	referencedPath := currentPath
	if parts[0] != "" {
		referencedPath = filepath.Clean(filepath.Join(filepath.Dir(currentPath), parts[0]))
	}
	node := readYAMLDocument(t, referencedPath)
	if len(parts) == 1 || parts[1] == "" {
		return referencedPath, node
	}
	for _, encodedToken := range strings.Split(strings.TrimPrefix(parts[1], "/"), "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(encodedToken, "~1", "/"), "~0", "~")
		node = mappingValue(t, node, token)
	}
	return referencedPath, node
}

// validateExternalReferences 递归验证契约中的全部外部引用均可读取并定位。
func validateExternalReferences(t *testing.T, currentPath string, node *yaml.Node, visited map[string]bool) {
	t.Helper()
	if node.Kind == yaml.MappingNode {
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			value := node.Content[index+1]
			if key.Value == "$ref" && !strings.HasPrefix(value.Value, "#") {
				referencedPath, referencedNode := resolveYAMLReference(t, currentPath, value.Value)
				referenceKey := referencedPath + "#" + value.Value
				if !visited[referenceKey] {
					visited[referenceKey] = true
					validateExternalReferences(t, referencedPath, referencedNode, visited)
				}
				continue
			}
			validateExternalReferences(t, currentPath, value, visited)
		}
		return
	}
	for _, child := range node.Content {
		validateExternalReferences(t, currentPath, child, visited)
	}
}

// normalizeOpenAPIPath 将 OpenAPI 路径参数格式转换为 Gin 路径参数格式。
func normalizeOpenAPIPath(openAPIPath string) string {
	return openAPIPathParameterPattern.ReplaceAllString(openAPIPath, `:$1`)
}

// isHTTPMethod 判断 OpenAPI Path Item 键是否为需要核对的 HTTP 方法。
func isHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}
