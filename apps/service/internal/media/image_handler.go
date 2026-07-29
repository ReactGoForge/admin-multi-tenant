package media

import (
	"net/http"
	"strconv"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"

	"github.com/gin-gonic/gin"
)

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
