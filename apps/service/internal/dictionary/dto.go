package dictionary

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

var dictionaryCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// typeMutationRequest 描述字典类型新增和编辑接口接收的字段。
type typeMutationRequest struct {
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Remark *string `json:"remark"`
	Sort   uint32  `json:"sort"`
	Status string  `json:"status"`
}

// itemMutationRequest 描述字典项新增和编辑接口接收的字段。
type itemMutationRequest struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Sort   uint32 `json:"sort"`
	Status string `json:"status"`
}

// itemResponse 描述返回给前端的字典项字段。
type itemResponse struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Sort   uint32 `json:"sort"`
	Status string `json:"status"`
}

// typeResponse 描述包含字典项列表的字典类型响应。
type typeResponse struct {
	ID       string         `json:"id"`
	Code     string         `json:"code"`
	Name     string         `json:"name"`
	Remark   *string        `json:"remark"`
	Sort     uint32         `json:"sort"`
	Status   string         `json:"status"`
	IsSystem bool           `json:"isSystem"`
	Items    []itemResponse `json:"items"`
}

// optionResponse 描述业务页面读取字典选项时的精简字段。
type optionResponse struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Sort  uint32 `json:"sort"`
}

// parseTypeMutation 清理并校验字典字段请求。
func parseTypeMutation(context *gin.Context) (TypeMutation, bool) {
	var request typeMutationRequest
	if context.ShouldBindJSON(&request) != nil {
		return TypeMutation{}, false
	}
	code := strings.TrimSpace(request.Code)
	name := strings.TrimSpace(request.Name)
	remark := trimOptional(request.Remark)
	status, validStatus := parseStatus(request.Status)
	if !dictionaryCodePattern.MatchString(code) || !validText(name, 50) || !validOptionalText(remark, 200) || !validStatus {
		return TypeMutation{}, false
	}
	return TypeMutation{Code: code, Name: name, Remark: remark, Sort: request.Sort, Status: status}, true
}

// parseItemMutation 清理并校验字典项请求。
func parseItemMutation(context *gin.Context) (ItemMutation, bool) {
	var request itemMutationRequest
	if context.ShouldBindJSON(&request) != nil {
		return ItemMutation{}, false
	}
	label := strings.TrimSpace(request.Label)
	value := strings.TrimSpace(request.Value)
	status, validStatus := parseStatus(request.Status)
	if !validText(label, 50) || !validText(value, 100) || !validStatus {
		return ItemMutation{}, false
	}
	return ItemMutation{Label: label, Value: value, Sort: request.Sort, Status: status}, true
}

// parsePathID 解析正整数路径 ID。
func parsePathID(context *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(context.Param(name), 10, 64)
	return id, err == nil && id > 0
}

// newTypeResponse 将服务层字典字段转换为字符串 ID 的接口响应。
func newTypeResponse(typeRow Type) typeResponse {
	items := make([]itemResponse, 0, len(typeRow.Items))
	for _, item := range typeRow.Items {
		items = append(items, itemResponse{ID: strconv.FormatUint(item.ID, 10), Label: item.Label, Value: item.Value, Sort: item.Sort, Status: statusName(item.Status)})
	}
	return typeResponse{ID: strconv.FormatUint(typeRow.ID, 10), Code: typeRow.Code, Name: typeRow.Name, Remark: typeRow.Remark, Sort: typeRow.Sort, Status: statusName(typeRow.Status), IsSystem: typeRow.IsSystem, Items: items}
}

// newOptionResponses 将服务层字典字段转换为消费接口需要的选项 Map。
func newOptionResponses(types []Type) map[string][]optionResponse {
	options := make(map[string][]optionResponse, len(types))
	for _, typeRow := range types {
		items := make([]optionResponse, 0, len(typeRow.Items))
		for _, item := range typeRow.Items {
			items = append(items, optionResponse{Label: item.Label, Value: item.Value, Sort: item.Sort})
		}
		options[typeRow.Code] = items
	}
	return options
}

// newTypeResponses 将服务层字典字段列表转换为管理接口响应。
func newTypeResponses(types []Type) []typeResponse {
	responses := make([]typeResponse, 0, len(types))
	for _, typeRow := range types {
		responses = append(responses, newTypeResponse(typeRow))
	}
	return responses
}

// parseStatus 将公开状态字符串转换为数据库状态值。
func parseStatus(status string) (uint8, bool) {
	switch strings.TrimSpace(status) {
	case "enabled":
		return 1, true
	case "disabled":
		return 0, true
	default:
		return 0, false
	}
}

// statusName 将数据库状态值转换为公开状态字符串。
func statusName(status uint8) string {
	if status == 1 {
		return "enabled"
	}
	return "disabled"
}

// trimOptional 清理可空文本，并将空白内容统一转换为 null。
func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// validText 校验必填文本的字符长度。
func validText(value string, maximum int) bool {
	length := utf8.RuneCountInString(value)
	return length > 0 && length <= maximum
}

// validOptionalText 校验可空文本的字符长度。
func validOptionalText(value *string, maximum int) bool {
	return value == nil || utf8.RuneCountInString(*value) <= maximum
}
