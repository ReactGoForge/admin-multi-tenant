package dictionary

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

type testStore struct {
	types       []Type
	err         error
	enabledOnly bool
	inTx        bool
	typeInput   TypeMutation
	typeUpdate  TypeUpdate
	itemInput   ItemMutation
	itemUpdate  ItemUpdate
	typeID      uint64
	itemID      uint64
	typeRow     Type
	itemRow     Item
	itemCount   int64
}

// ListTypes 返回测试预设的字典字段并记录过滤参数。
func (store *testStore) ListTypes(_ context.Context, enabledOnly bool) ([]Type, error) {
	store.enabledOnly = enabledOnly
	return store.types, store.err
}

// CreateType 记录测试中的字典字段新增内容。
func (store *testStore) CreateType(_ context.Context, mutation TypeMutation) error {
	store.typeInput = mutation
	return store.err
}

// WithTransaction 使用测试 Store 自身模拟事务数据访问能力。
func (store *testStore) WithTransaction(_ context.Context, action func(TransactionStore) error) error {
	store.inTx = true
	return action(store)
}

// FindTypeForUpdate 记录测试中的字典字段加锁读取 ID。
func (store *testStore) FindTypeForUpdate(_ context.Context, typeID uint64) (Type, error) {
	store.typeID = typeID
	if store.err != nil {
		return Type{}, store.err
	}
	return store.typeRow, nil
}

// CountItems 返回测试预设的字典项数量。
func (store *testStore) CountItems(_ context.Context, typeID uint64) (int64, error) {
	store.typeID = typeID
	if store.err != nil {
		return 0, store.err
	}
	return store.itemCount, nil
}

// UpdateType 记录测试中的字典字段更新内容。
func (store *testStore) UpdateType(_ context.Context, typeID uint64, update TypeUpdate) error {
	store.typeID, store.typeUpdate = typeID, update
	return store.err
}

// DeleteType 记录测试中的字典字段删除 ID。
func (store *testStore) DeleteType(_ context.Context, typeID uint64) error {
	store.typeID = typeID
	return store.err
}

// CreateItem 记录测试中的字典项新增内容。
func (store *testStore) CreateItem(_ context.Context, typeID uint64, mutation ItemMutation) error {
	store.typeID, store.itemInput = typeID, mutation
	return store.err
}

// FindItemForUpdate 记录测试中的字典项加锁读取 ID。
func (store *testStore) FindItemForUpdate(_ context.Context, typeID, itemID uint64) (Item, error) {
	store.typeID, store.itemID = typeID, itemID
	if store.err != nil {
		return Item{}, store.err
	}
	return store.itemRow, nil
}

// UpdateItem 记录测试中的字典项更新内容。
func (store *testStore) UpdateItem(_ context.Context, typeID, itemID uint64, update ItemUpdate) error {
	store.typeID, store.itemID, store.itemUpdate = typeID, itemID, update
	return store.err
}

// DeleteItem 记录测试中的字典项删除 ID。
func (store *testStore) DeleteItem(_ context.Context, typeID, itemID uint64) error {
	store.typeID, store.itemID = typeID, itemID
	return store.err
}

// newTestRouter 注册字典 Handler 的测试路由。
func newTestRouter(store *testStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(NewService(store))
	router.GET("/options", handler.ListOptions)
	router.GET("/dictionaries", handler.ListTypes)
	router.POST("/dictionaries", handler.CreateType)
	router.PATCH("/dictionaries/:dictionaryId", handler.UpdateType)
	router.DELETE("/dictionaries/:dictionaryId", handler.DeleteType)
	router.POST("/dictionaries/:dictionaryId/items", handler.CreateItem)
	router.PATCH("/dictionaries/:dictionaryId/items/:itemId", handler.UpdateItem)
	router.DELETE("/dictionaries/:dictionaryId/items/:itemId", handler.DeleteItem)
	return router
}

// performRequest 执行带可选 JSON 请求体的字典 Handler 测试请求。
func performRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestListOptionsAndTypes 验证消费接口过滤参数、字符串 ID 和空数组响应。
func TestListOptionsAndTypes(t *testing.T) {
	remark := "系统状态"
	store := &testStore{types: []Type{{ID: 1, Code: "entity_status", Name: "通用状态", Remark: &remark, Sort: 10, Status: 1, IsSystem: true, Items: []Item{{ID: 2, Label: "启用", Value: "enabled", Sort: 10, Status: 1}}}}}
	router := newTestRouter(store)

	recorder := performRequest(router, http.MethodGet, "/options", "")
	if recorder.Code != http.StatusOK || !store.enabledOnly || recorder.Body.String() != `{"code":0,"message":"成功","data":{"entity_status":[{"label":"启用","value":"enabled","sort":10}]}}` {
		t.Fatalf("字典选项响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	recorder = performRequest(router, http.MethodGet, "/dictionaries", "")
	if recorder.Code != http.StatusOK || store.enabledOnly || !bytes.Contains(recorder.Body.Bytes(), []byte(`"id":"1"`)) || !bytes.Contains(recorder.Body.Bytes(), []byte(`"items":[{"id":"2"`)) {
		t.Fatalf("字典管理响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	store.types = []Type{}
	recorder = performRequest(router, http.MethodGet, "/dictionaries", "")
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"data":[]`)) {
		t.Fatalf("字典空数组响应 = %s", recorder.Body.String())
	}
}

// TestDictionaryMutationValidation 验证字段和字典项写请求的清理、长度与 ID 校验。
func TestDictionaryMutationValidation(t *testing.T) {
	store := &testStore{}
	router := newTestRouter(store)
	recorder := performRequest(router, http.MethodPost, "/dictionaries", `{"code":"Business_Type","name":"业务类型","status":"enabled"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法编码响应 = %d %s", recorder.Code, recorder.Body.String())
	}

	recorder = performRequest(router, http.MethodPost, "/dictionaries", `{"code":"business_type","name":" 业务类型 ","remark":" 备注 ","sort":10,"status":"enabled"}`)
	if recorder.Code != http.StatusCreated || store.typeInput.Code != "business_type" || store.typeInput.Name != "业务类型" || store.typeInput.Remark == nil || *store.typeInput.Remark != "备注" || store.typeInput.Status != 1 {
		t.Fatalf("字段新增响应或输入错误 = %d %+v", recorder.Code, store.typeInput)
	}

	recorder = performRequest(router, http.MethodPost, "/dictionaries/8/items", `{"label":" 选项甲 ","value":" option_a ","sort":20,"status":"disabled"}`)
	if recorder.Code != http.StatusCreated || store.typeID != 8 || store.itemInput.Label != "选项甲" || store.itemInput.Value != "option_a" || store.itemInput.Status != 0 {
		t.Fatalf("字典项新增响应或输入错误 = %d %+v", recorder.Code, store.itemInput)
	}

	recorder = performRequest(router, http.MethodDelete, "/dictionaries/0", "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法 ID 响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestDictionaryBusinessErrors 验证唯一冲突、系统保护、未找到和未知错误的稳定响应。
func TestDictionaryBusinessErrors(t *testing.T) {
	testCases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "冲突", err: ErrConflict, wantStatus: http.StatusConflict, wantCode: `"code":40003`},
		{name: "系统保护", err: ErrProtected, wantStatus: http.StatusConflict, wantCode: `"code":40004`},
		{name: "未找到", err: ErrNotFound, wantStatus: http.StatusNotFound, wantCode: `"code":40002`},
		{name: "内部错误", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: `"code":50000`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			store := &testStore{err: testCase.err}
			recorder := performRequest(newTestRouter(store), http.MethodDelete, "/dictionaries/9", "")
			if recorder.Code != testCase.wantStatus || !bytes.Contains(recorder.Body.Bytes(), []byte(testCase.wantCode)) {
				t.Fatalf("业务错误响应 = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestNormalizeWriteError 验证数据库唯一约束错误转换为字典冲突。
func TestNormalizeWriteError(t *testing.T) {
	if !errors.Is(normalizeWriteError(&mysql.MySQLError{Number: 1062}), ErrConflict) {
		t.Fatal("唯一约束错误未转换为字典冲突")
	}
}
