package dictionary

import (
	"errors"
	"net/http"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// Handler 组织全局字典读取和平台超级管理员写接口。
type Handler struct {
	service *Service
}

// NewHandler 使用字典服务能力创建字典处理器。
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// ListOptions 返回所有已启用字典字段及字典项，供后台统一展示选项使用。
func (handler *Handler) ListOptions(context *gin.Context) {
	types, err := handler.service.ListOptions(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newOptionResponses(types))
}

// ListTypes 返回包含禁用数据的完整字典管理列表。
func (handler *Handler) ListTypes(context *gin.Context) {
	types, err := handler.service.ListTypes(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, newTypeResponses(types))
}

// CreateType 校验并新增自定义字典字段。
func (handler *Handler) CreateType(context *gin.Context) {
	mutation, valid := parseTypeMutation(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.CreateType(context.Request.Context(), mutation); err != nil {
		writeDictionaryError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, nil)
}

// UpdateType 校验并更新字典字段。
func (handler *Handler) UpdateType(context *gin.Context) {
	typeID, valid := parsePathID(context, "dictionaryId")
	mutation, validMutation := parseTypeMutation(context)
	if !valid || !validMutation {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdateType(context.Request.Context(), typeID, mutation); err != nil {
		writeDictionaryError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// DeleteType 删除无字典项的自定义字典字段。
func (handler *Handler) DeleteType(context *gin.Context) {
	typeID, valid := parsePathID(context, "dictionaryId")
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.DeleteType(context.Request.Context(), typeID); err != nil {
		writeDictionaryError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// CreateItem 校验并为自定义字典字段新增字典项。
func (handler *Handler) CreateItem(context *gin.Context) {
	typeID, valid := parsePathID(context, "dictionaryId")
	mutation, validMutation := parseItemMutation(context)
	if !valid || !validMutation {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.CreateItem(context.Request.Context(), typeID, mutation); err != nil {
		writeDictionaryError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, nil)
}

// UpdateItem 校验并更新指定字典项。
func (handler *Handler) UpdateItem(context *gin.Context) {
	typeID, validType := parsePathID(context, "dictionaryId")
	itemID, validItem := parsePathID(context, "itemId")
	mutation, validMutation := parseItemMutation(context)
	if !validType || !validItem || !validMutation {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdateItem(context.Request.Context(), typeID, itemID, mutation); err != nil {
		writeDictionaryError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// DeleteItem 删除自定义字典字段下的指定字典项。
func (handler *Handler) DeleteItem(context *gin.Context) {
	typeID, validType := parsePathID(context, "dictionaryId")
	itemID, validItem := parsePathID(context, "itemId")
	if !validType || !validItem {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.DeleteItem(context.Request.Context(), typeID, itemID); err != nil {
		writeDictionaryError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// writeDictionaryError 将字典业务错误转换为统一 HTTP 响应。
func writeDictionaryError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
	case errors.Is(err, ErrConflict):
		httpapi.WriteError(context, httpapi.ErrorCodeConflict)
	case errors.Is(err, ErrProtected):
		httpapi.WriteError(context, httpapi.ErrorCodeProtectedResource)
	default:
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
	}
}
