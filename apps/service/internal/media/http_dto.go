package media

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

// imageResponse 描述图片管理列表返回的完整图片信息。
type imageResponse struct {
	ID           string  `json:"id"`
	TenantID     *string `json:"tenantId"`
	TenantName   *string `json:"tenantName"`
	CategoryID   *string `json:"categoryId"`
	CategoryName *string `json:"categoryName"`
	OriginalName string  `json:"originalName"`
	MIMEType     string  `json:"mimeType"`
	SizeBytes    uint64  `json:"sizeBytes"`
	PreviewURL   string  `json:"previewUrl"`
	CreatedAt    string  `json:"createdAt"`
}

// imagePageResponse 描述图片列表的统一分页响应。
type imagePageResponse struct {
	Items    []imageResponse `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

// categoryResponse 描述图片分类及其图片数量。
type categoryResponse struct {
	ID       string  `json:"id"`
	TenantID *string `json:"tenantId"`
	Name     string  `json:"name"`
	IsShared bool    `json:"isShared"`
}

// imageSummaryResponse 描述基础设置中引用图片的摘要。
type imageSummaryResponse struct {
	ID           string `json:"id"`
	OriginalName string `json:"originalName"`
	PreviewURL   string `json:"previewUrl"`
}

// basicSettingsResponse 描述平台或租户基础设置响应。
type basicSettingsResponse struct {
	Name string                `json:"name"`
	Icon *imageSummaryResponse `json:"icon"`
}

// basicSettingsRequest 描述基础设置更新接口接收的名称和图片 ID。
type basicSettingsRequest struct {
	Name        string  `json:"name"`
	IconImageID *uint64 `json:"iconImageId"`
}

// avatarResponse 描述个人头像上传后可立即展示的临时访问地址。
type avatarResponse struct {
	AvatarURL string `json:"avatarUrl"`
}

// categoryRequest 描述图片分类新增或重命名请求。
type categoryRequest struct {
	Name     string  `json:"name"`
	TenantID *uint64 `json:"tenantId"`
}

// updateImageRequest 描述图片名称和可空分类的更新请求。
type updateImageRequest struct {
	CategoryID   json.RawMessage `json:"categoryId"`
	OriginalName *string         `json:"originalName"`
}

// normalizeImageName 清理并校验图片展示名称长度。
func normalizeImageName(rawName string) (string, bool) {
	name := strings.TrimSpace(rawName)
	return name, name != "" && len([]rune(name)) <= 255
}

// parseImageCategoryID 解析图片分类更新值，并保留 null 表示未分类。
func parseImageCategoryID(raw json.RawMessage) (*uint64, bool) {
	var categoryID *uint64
	if err := json.Unmarshal(raw, &categoryID); err != nil || (categoryID != nil && *categoryID == 0) {
		return nil, false
	}
	return categoryID, true
}

// deleteImage 删除元数据后尽力清理对象存储，不在日志中记录对象键。

// tenantOwnerFromQuery 将租户工作空间来源转换为当前租户或平台共享所有者。
func tenantOwnerFromQuery(context *gin.Context) (*uint64, bool) {
	if context.DefaultQuery("source", "tenant") == "platform" {
		return nil, true
	}
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return nil, false
	}
	return &tenantID, true
}

// platformOwnerFromQuery 将平台图片来源与可选租户 ID 转换为精确所有者。
func platformOwnerFromQuery(context *gin.Context) (*uint64, bool) {
	if context.DefaultQuery("source", "platform") == "platform" {
		return nil, true
	}
	value, valid := parseID(context.Query("tenantId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return nil, false
	}
	return &value, true
}

// parsePage 校验并限制统一分页参数。
func parsePage(context *gin.Context) (int, int, bool) {
	page, err := strconv.Atoi(context.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 0, 0, false
	}
	pageSize, err := strconv.Atoi(context.DefaultQuery("pageSize", "10"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		return 0, 0, false
	}
	return page, pageSize, true
}

// optionalID 将空字符串转换为 nil，其余值必须为正整数。
func optionalID(raw string) (*uint64, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	value, valid := parseID(raw)
	if !valid {
		return nil, false
	}
	return &value, true
}

// optionalCategoryID 将空分类视为全部分类，并允许使用 0 表示未分类筛选。
func optionalCategoryID(raw string) (*uint64, bool) {
	if strings.TrimSpace(raw) == "0" {
		value := uint64(0)
		return &value, true
	}
	return optionalID(raw)
}

// parseID 将请求中的十进制 ID 转换为正整数。
func parseID(raw string) (uint64, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	return value, err == nil && value > 0
}

// formatOptionalID 将可空数据库 ID 转换为前端安全的字符串 ID。
func formatOptionalID(value *uint64) *string {
	if value == nil {
		return nil
	}
	formatted := strconv.FormatUint(*value, 10)
	return &formatted
}

// toImageResponse 将图片元数据转换为不包含对象键的接口数据。
func toImageResponse(asset ImageAsset, previewURL string) imageResponse {
	return imageResponse{ID: strconv.FormatUint(asset.ID, 10), TenantID: formatOptionalID(asset.TenantID), TenantName: asset.TenantName, CategoryID: formatOptionalID(asset.CategoryID), CategoryName: asset.CategoryName, OriginalName: asset.OriginalName, MIMEType: asset.MIMEType, SizeBytes: asset.SizeBytes, PreviewURL: previewURL, CreatedAt: asset.CreatedAt.Format(time.RFC3339)}
}

// toBasicSettingsResponse 将品牌设置转换为稳定公共代理图标摘要。
func toBasicSettingsResponse(settings *BasicSettings) basicSettingsResponse {
	response := basicSettingsResponse{Name: settings.Name}
	if settings.IconImageID != nil {
		name := "品牌图标"
		if settings.IconOriginalName != nil {
			name = *settings.IconOriginalName
		}
		response.Icon = &imageSummaryResponse{ID: strconv.FormatUint(*settings.IconImageID, 10), OriginalName: name, PreviewURL: publicImageURL(*settings.IconImageID)}
	}
	return response
}

// publicImageURL 生成同源公开品牌图片代理路径。
func publicImageURL(imageID uint64) string {
	return "/api/public/images/" + strconv.FormatUint(imageID, 10)
}
