package media

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/user"

	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
)

const (
	maxImageSize    = 5 * 1024 * 1024
	maxAvatarSize   = 5 * 1024 * 1024
	maxUploadBody   = maxImageSize + 1024*1024
	presignedExpiry = 15 * time.Minute
	avatarExpiry    = 8 * time.Hour
)

// Handler 提供平台与租户图片库、分类、品牌设置和公开品牌图片接口。
type Handler struct {
	service *Service
}

// NewHandler 创建媒体接口处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

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

// UploadCurrentEmployeeAvatar 校验裁剪头像后替换当前认证员工的私有头像。
func (handler *Handler) UploadCurrentEmployeeAvatar(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxAvatarSize+1024*1024)
	if !handler.requireStorage(context) {
		return
	}
	employee, valid := auth.CurrentEmployee(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	file, err := context.FormFile("file")
	if err != nil || file.Size <= 0 || file.Size > maxAvatarSize {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	data, mimeType, extension, err := readAndValidateAvatar(file)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	result, err := handler.service.UploadCurrentEmployeeAvatar(context.Request.Context(), AvatarUpload{EmployeeID: employee.ID, EmployeeName: employee.Name, Scope: employee.Scope, TenantID: employee.TenantID, OriginalName: file.Filename, Data: data, MIMEType: mimeType, Extension: extension})
	if err != nil {
		writeMediaError(context, err)
		return
	}
	logging.SetAuditDetail(context, logging.AuditDetail{TargetID: strconv.FormatUint(result.EmployeeID, 10), TargetName: result.EmployeeName, Summary: "头像已修改", Changes: map[string]any{"avatar": map[string]any{"changed": true}}})
	httpapi.WriteSuccess(context, http.StatusCreated, avatarResponse{AvatarURL: result.AvatarURL})
}

// UploadCurrentMiniappUserAvatar 校验并替换当前认证小程序用户的私有头像。
func (handler *Handler) UploadCurrentMiniappUserAvatar(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxAvatarSize+1024*1024)
	if !handler.requireStorage(context) {
		return
	}
	session, valid := user.CurrentMiniappSession(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	file, err := context.FormFile("file")
	if err != nil || file.Size <= 0 || file.Size > maxAvatarSize {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	data, mimeType, extension, err := readAndValidateAvatar(file)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	avatarURL, err := handler.service.UploadMiniappUserAvatar(context.Request.Context(), MiniappUserAvatarUpload{
		UserID: session.User.ID, OriginalName: file.Filename, Data: data, MIMEType: mimeType, Extension: extension,
	})
	if err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, avatarResponse{AvatarURL: avatarURL})
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

// GetPlatformBasicSettings 返回平台品牌名称和当前图片库图标。
func (handler *Handler) GetPlatformBasicSettings(context *gin.Context) {
	settings, err := handler.service.GetPlatformBasicSettings(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, toBasicSettingsResponse(settings))
}

// UpdatePlatformBasicSettings 校验图片库图标后保存平台品牌。
func (handler *Handler) UpdatePlatformBasicSettings(context *gin.Context) {
	var request basicSettingsRequest
	if err := context.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" || len([]rune(strings.TrimSpace(request.Name))) > 100 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdatePlatformBasicSettings(context.Request.Context(), strings.TrimSpace(request.Name), request.IconImageID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// GetTenantBasicSettings 返回认证上下文中可信租户的品牌设置。
func (handler *Handler) GetTenantBasicSettings(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	settings, err := handler.service.GetTenantBasicSettings(context.Request.Context(), tenantID)
	if err != nil {
		writeMediaError(context, err)
		return
	}
	response := toBasicSettingsResponse(settings)
	if response.Icon == nil && settings.LegacyIconURL != nil && strings.TrimSpace(*settings.LegacyIconURL) != "" {
		response.Icon = &imageSummaryResponse{OriginalName: "兼容图标", PreviewURL: *settings.LegacyIconURL}
	}
	httpapi.WriteSuccess(context, http.StatusOK, response)
}

// UpdateTenantBasicSettings 校验图片来自平台共享图库或当前租户后保存租户品牌。
func (handler *Handler) UpdateTenantBasicSettings(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	var request basicSettingsRequest
	if err := context.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" || len([]rune(strings.TrimSpace(request.Name))) > 100 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdateTenantBasicSettings(context.Request.Context(), tenantID, strings.TrimSpace(request.Name), request.IconImageID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// PublicPlatformBrand 返回无需登录即可使用的平台名称和稳定图标代理地址。
func (handler *Handler) PublicPlatformBrand(context *gin.Context) {
	settings, err := handler.service.PublicPlatformBrand(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	var iconURL *string
	if settings.IconImageID != nil {
		value := publicImageURL(*settings.IconImageID)
		iconURL = &value
	}
	httpapi.WriteSuccess(context, http.StatusOK, gin.H{"name": settings.Name, "iconUrl": iconURL})
}

// PublicImage 仅代理当前正在被平台或租户品牌引用的私有图片。
func (handler *Handler) PublicImage(context *gin.Context) {
	if !handler.requireStorage(context) {
		return
	}
	imageID, valid := parseID(context.Param("imageId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	object, err := handler.service.PublicImage(context.Request.Context(), imageID)
	if err != nil {
		writeMediaError(context, err)
		return
	}
	defer func() { _ = object.Body.Close() }()
	context.Header("Cache-Control", "public, max-age=300")
	context.DataFromReader(http.StatusOK, object.Size, object.ContentType, object.Body, nil)
}

// ListPlatformImages 按平台选择的所有者范围分页读取图片。
func (handler *Handler) ListPlatformImages(context *gin.Context) {
	owner, valid := platformOwnerFromQuery(context)
	if !valid || !handler.requireStorage(context) {
		return
	}
	handler.listImages(context, owner, false)
}

// UploadPlatformImage 向平台图库或指定租户图库上传单张图片。
func (handler *Handler) UploadPlatformImage(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxUploadBody)
	owner, valid := optionalID(context.PostForm("tenantId"))
	if !valid || !handler.requireStorage(context) {
		if !valid {
			httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		}
		return
	}
	handler.uploadImage(context, owner)
}

// UpdatePlatformImage 修改平台或任意租户图片的分类。
func (handler *Handler) UpdatePlatformImage(context *gin.Context) {
	owner, valid := platformOwnerFromQuery(context)
	if !valid {
		return
	}
	handler.updateImage(context, owner)
}

// DeletePlatformImage 删除平台或任意租户未被品牌引用的图片。
func (handler *Handler) DeletePlatformImage(context *gin.Context) {
	owner, valid := platformOwnerFromQuery(context)
	if !valid {
		return
	}
	handler.deleteImage(context, owner)
}

// ListTenantImages 读取当前租户图库或平台共享图库。
func (handler *Handler) ListTenantImages(context *gin.Context) {
	sharedOnly := context.DefaultQuery("source", "tenant") == "platform"
	owner, valid := tenantOwnerFromQuery(context)
	if !valid || !handler.requireStorage(context) {
		return
	}
	handler.listImages(context, owner, sharedOnly)
}

// UploadTenantImage 向认证上下文中的可信租户图库上传图片。
func (handler *Handler) UploadTenantImage(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxUploadBody)
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid || !handler.requireStorage(context) {
		if !valid {
			httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		}
		return
	}
	handler.uploadImage(context, &tenantID)
}

// UpdateTenantImage 修改当前租户自有图片分类，不能修改平台共享图片。
func (handler *Handler) UpdateTenantImage(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	handler.updateImage(context, &tenantID)
}

// DeleteTenantImage 删除当前租户自有且未被品牌引用的图片。
func (handler *Handler) DeleteTenantImage(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	handler.deleteImage(context, &tenantID)
}

// ListPlatformCategories 读取平台或指定租户的图片分类。
func (handler *Handler) ListPlatformCategories(context *gin.Context) {
	owner, valid := optionalID(context.Query("tenantId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.listCategories(context, owner, false)
}

// CreatePlatformCategory 为平台或指定租户新增分类。
func (handler *Handler) CreatePlatformCategory(context *gin.Context) {
	var request categoryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.createCategory(context, request.TenantID, request.Name)
}

// UpdatePlatformCategory 修改平台或指定租户的分类名称。
func (handler *Handler) UpdatePlatformCategory(context *gin.Context) {
	owner, valid := optionalID(context.Query("tenantId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.updateCategory(context, owner)
}

// DeletePlatformCategory 删除平台或指定租户的分类并将图片转入未分类。
func (handler *Handler) DeletePlatformCategory(context *gin.Context) {
	owner, valid := optionalID(context.Query("tenantId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.deleteCategory(context, owner)
}

// ListTenantCategories 读取当前租户分类或平台共享分类。
func (handler *Handler) ListTenantCategories(context *gin.Context) {
	sharedOnly := context.DefaultQuery("source", "tenant") == "platform"
	owner, valid := tenantOwnerFromQuery(context)
	if !valid {
		return
	}
	handler.listCategories(context, owner, sharedOnly)
}

// CreateTenantCategory 为当前可信租户新增图片分类。
func (handler *Handler) CreateTenantCategory(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	var request categoryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	handler.createCategory(context, &tenantID, request.Name)
}

// UpdateTenantCategory 修改当前可信租户的分类名称。
func (handler *Handler) UpdateTenantCategory(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	handler.updateCategory(context, &tenantID)
}

// DeleteTenantCategory 删除当前可信租户分类并将图片转入未分类。
func (handler *Handler) DeleteTenantCategory(context *gin.Context) {
	tenantID, valid := auth.CurrentTenantID(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
		return
	}
	handler.deleteCategory(context, &tenantID)
}

// PlatformTenantOptions 返回平台图片管理的租户筛选选项。
func (handler *Handler) PlatformTenantOptions(context *gin.Context) {
	options, err := handler.service.ListTenantOptions(context.Request.Context())
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	result := make([]gin.H, 0, len(options))
	for _, option := range options {
		result = append(result, gin.H{"id": strconv.FormatUint(option.ID, 10), "name": option.Name})
	}
	httpapi.WriteSuccess(context, http.StatusOK, result)
}

// listImages 完成公共的分页参数校验、数据库查询和临时 URL 签发。
func (handler *Handler) listImages(context *gin.Context, ownerTenantID *uint64, sharedOnly bool) {
	page, pageSize, valid := parsePage(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	categoryID, valid := optionalCategoryID(context.Query("categoryId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	pageResult, err := handler.service.ListImages(context.Request.Context(), ownerTenantID, sharedOnly, categoryID, context.Query("name"), page, pageSize)
	if err != nil {
		writeMediaError(context, err)
		return
	}
	items := make([]imageResponse, 0, len(pageResult.Items))
	for _, asset := range pageResult.Items {
		items = append(items, toImageResponse(asset, pageResult.URLs[asset.ID]))
	}
	httpapi.WriteSuccess(context, http.StatusOK, imagePageResponse{Items: items, Page: pageResult.Page, PageSize: pageResult.PageSize, Total: pageResult.Total})
}

// uploadImage 校验图片内容、归属分类和大小后写入对象存储与数据库。
func (handler *Handler) uploadImage(context *gin.Context, ownerTenantID *uint64) {
	// 上传流程一：先解析 multipart 文件和分类，再根据真实解码结果校验内容，不信任文件扩展名。
	file, err := context.FormFile("file")
	if err != nil || file.Size <= 0 || file.Size > maxImageSize {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	categoryID, valid := optionalCategoryID(context.PostForm("categoryId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	data, mimeType, extension, err := readAndValidateImage(file)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	employee, valid := auth.CurrentEmployee(context)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeUnauthorized)
		return
	}
	// 上传流程二：对象先写入 MinIO，再保存数据库元数据；数据库失败时由 Service 尽力删除已经上传的孤立对象。
	result, err := handler.service.UploadImage(context.Request.Context(), ImageUpload{OwnerTenantID: ownerTenantID, CategoryID: categoryID, OriginalName: file.Filename, Data: data, MIMEType: mimeType, Extension: extension, UploadedByEmployeeID: employee.ID})
	if err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, toImageResponse(result.Asset, result.PreviewURL))
}

// updateImage 修改单张图片的分类或展示名称，并严格保持所有者一致。
func (handler *Handler) updateImage(context *gin.Context, ownerTenantID *uint64) {
	imageID, valid := parseID(context.Param("imageId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	var request updateImageRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	// Go 学习提示：RawMessage 保留 JSON 原始值，使代码能够区分“字段未提交”和“明确提交 null”。
	hasCategory := len(request.CategoryID) > 0
	hasName := request.OriginalName != nil
	if hasCategory == hasName {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if hasName {
		name, valid := normalizeImageName(*request.OriginalName)
		if !valid {
			httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
			return
		}
		if err := handler.service.UpdateImageName(context.Request.Context(), imageID, ownerTenantID, name); err != nil {
			writeMediaError(context, err)
			return
		}
		httpapi.WriteSuccess(context, http.StatusOK, nil)
		return
	}
	categoryID, valid := parseImageCategoryID(request.CategoryID)
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdateImageCategory(context.Request.Context(), imageID, ownerTenantID, categoryID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
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
func (handler *Handler) deleteImage(context *gin.Context, ownerTenantID *uint64) {
	imageID, valid := parseID(context.Param("imageId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	// 业务约束：数据库元数据是业务事实来源，先完成受事务保护的删除，再尽力清理 MinIO 对象。
	// 对象清理失败只记录服务端日志，不能把已经成功的数据库操作重新伪装成失败。
	if err := handler.service.DeleteImage(context.Request.Context(), imageID, ownerTenantID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// listCategories 返回指定所有者的分类列表。
func (handler *Handler) listCategories(context *gin.Context, ownerTenantID *uint64, sharedOnly bool) {
	categories, err := handler.service.ListCategories(context.Request.Context(), ownerTenantID, sharedOnly)
	if err != nil {
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
		return
	}
	result := make([]categoryResponse, 0, len(categories))
	for _, category := range categories {
		result = append(result, categoryResponse{ID: strconv.FormatUint(category.ID, 10), TenantID: formatOptionalID(category.TenantID), Name: category.Name, IsShared: category.IsShared})
	}
	httpapi.WriteSuccess(context, http.StatusOK, result)
}

// createCategory 校验名称后新增指定所有者的分类。
func (handler *Handler) createCategory(context *gin.Context, ownerTenantID *uint64, rawName string) {
	name := strings.TrimSpace(rawName)
	if name == "" || len([]rune(name)) > 40 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	category, err := handler.service.CreateCategory(context.Request.Context(), ownerTenantID, name)
	if err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusCreated, categoryResponse{ID: strconv.FormatUint(category.ID, 10), TenantID: formatOptionalID(category.TenantID), Name: category.Name, IsShared: category.IsShared})
}

// updateCategory 校验分类归属与名称后更新分类。
func (handler *Handler) updateCategory(context *gin.Context, ownerTenantID *uint64) {
	categoryID, valid := parseID(context.Param("categoryId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	var request categoryRequest
	if err := context.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" || len([]rune(strings.TrimSpace(request.Name))) > 40 {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.UpdateCategory(context.Request.Context(), categoryID, ownerTenantID, strings.TrimSpace(request.Name)); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// deleteCategory 删除指定所有者的分类。
func (handler *Handler) deleteCategory(context *gin.Context, ownerTenantID *uint64) {
	categoryID, valid := parseID(context.Param("categoryId"))
	if !valid {
		httpapi.WriteError(context, httpapi.ErrorCodeInvalidRequest)
		return
	}
	if err := handler.service.DeleteCategory(context.Request.Context(), categoryID, ownerTenantID); err != nil {
		writeMediaError(context, err)
		return
	}
	httpapi.WriteSuccess(context, http.StatusOK, nil)
}

// requireStorage 在媒体接口访问对象存储前统一返回 503。
func (handler *Handler) requireStorage(context *gin.Context) bool {
	if handler.service == nil || !handler.service.StorageReady() {
		httpapi.WriteError(context, httpapi.ErrorCodeMediaUnavailable)
		return false
	}
	return true
}

// readAndValidateImage 读取单张文件并以真实解码结果限制为 PNG、JPEG 或 WebP。
func readAndValidateImage(file *multipart.FileHeader) ([]byte, string, string, error) {
	source, err := file.Open()
	if err != nil {
		return nil, "", "", err
	}
	defer func() { _ = source.Close() }()
	// 安全边界：最多读取上限加一个字节，既能发现超限文件，也不会无界占用内存。
	data, err := io.ReadAll(io.LimitReader(source, maxImageSize+1))
	if err != nil || len(data) == 0 || len(data) > maxImageSize {
		return nil, "", "", fmt.Errorf("图片大小不合法")
	}
	// DecodeConfig 根据文件内容识别真实格式；随后再与浏览器 MIME 检测结果交叉验证。
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", err
	}
	mimeByFormat := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "webp": "image/webp"}
	extensionByFormat := map[string]string{"png": ".png", "jpeg": ".jpg", "webp": ".webp"}
	mimeType, supported := mimeByFormat[format]
	if !supported || !strings.HasPrefix(http.DetectContentType(data), mimeType) {
		return nil, "", "", fmt.Errorf("图片格式不支持")
	}
	if name := filepath.Base(file.Filename); name == "." || len([]byte(name)) > 255 {
		return nil, "", "", fmt.Errorf("图片名称不合法")
	}
	return data, mimeType, extensionByFormat[format], nil
}

// readAndValidateAvatar 复用真实图片校验，并额外要求裁剪结果不超过 5MB 且宽高相等。
func readAndValidateAvatar(file *multipart.FileHeader) ([]byte, string, string, error) {
	if file.Size <= 0 || file.Size > maxAvatarSize {
		return nil, "", "", fmt.Errorf("头像大小不合法")
	}
	data, mimeType, extension, err := readAndValidateImage(file)
	if err != nil || len(data) > maxAvatarSize {
		return nil, "", "", fmt.Errorf("头像内容不合法")
	}
	configuration, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || configuration.Width <= 0 || configuration.Width != configuration.Height {
		return nil, "", "", fmt.Errorf("头像必须为正方形")
	}
	return data, mimeType, extension, nil
}

// createObjectKey 使用不可预测随机值生成不含原文件名的对象键。
func createObjectKey(tenantID *uint64, extension string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	prefix := "platform"
	if tenantID != nil {
		prefix = "tenants/" + strconv.FormatUint(*tenantID, 10)
	}
	return fmt.Sprintf("%s/%s/%s%s", prefix, time.Now().UTC().Format("2006/01"), hex.EncodeToString(random), extension), nil
}

// createMiniappUserAvatarObjectKey 为小程序用户头像生成独立且不可预测的私有对象键。
func createMiniappUserAvatarObjectKey(extension string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	return filepath.ToSlash(filepath.Join("miniapp-users", strconv.Itoa(now.Year()), fmt.Sprintf("%02d", now.Month()), hex.EncodeToString(random)+extension)), nil
}

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

// writeMediaError 将媒体业务错误转换为统一 HTTP 响应。
func writeMediaError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrMediaUnavailable):
		httpapi.WriteError(context, httpapi.ErrorCodeMediaUnavailable)
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(context, httpapi.ErrorCodeResourceNotFound)
	case errors.Is(err, ErrConflict), errors.Is(err, ErrImageReferenced):
		httpapi.WriteError(context, httpapi.ErrorCodeConflict)
	case errors.Is(err, ErrInvalidOwner):
		httpapi.WriteError(context, httpapi.ErrorCodeForbidden)
	default:
		logging.WriteEventOutput("error", fmt.Sprintf("媒体接口处理失败: %v", err), logging.RequestID(context))
		httpapi.WriteError(context, httpapi.ErrorCodeInternal)
	}
}
