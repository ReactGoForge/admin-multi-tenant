package media

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/schema"
)

// TestImageAssetProjectionFieldsAreReadOnly 验证列表关联字段不会参与图片元数据写入。
func TestImageAssetProjectionFieldsAreReadOnly(t *testing.T) {
	parsed, err := schema.Parse(&ImageAsset{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("schema.Parse() 返回错误: %v", err)
	}
	for _, name := range []string{"TenantName", "CategoryName"} {
		field := parsed.LookUpField(name)
		if field == nil {
			t.Fatalf("未找到字段 %s", name)
		}
		if field.Creatable || field.Updatable || !field.Readable {
			t.Fatalf("字段 %s 权限错误: creatable=%v updatable=%v readable=%v", name, field.Creatable, field.Updatable, field.Readable)
		}
	}
}

// TestImageCategoryIncludesSharedMarker 验证分类模型和接口响应都保留共享分类标识。
func TestImageCategoryIncludesSharedMarker(t *testing.T) {
	parsed, err := schema.Parse(&ImageCategory{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("schema.Parse() 返回错误: %v", err)
	}
	field := parsed.LookUpField("IsShared")
	if field == nil || field.DBName != "is_shared" {
		t.Fatalf("共享字段映射错误: %#v", field)
	}
	encoded, err := json.Marshal(categoryResponse{ID: "1", Name: "共享图片", IsShared: true})
	if err != nil {
		t.Fatalf("json.Marshal() 返回错误: %v", err)
	}
	if !strings.Contains(string(encoded), `"isShared":true`) {
		t.Fatalf("分类响应缺少共享标识: %s", encoded)
	}
}

// TestParsePageDefaultsToTen 验证图片列表默认使用每页十张。
func TestParsePageDefaultsToTen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/images", nil)
	page, pageSize, valid := parsePage(context)
	if !valid || page != 1 || pageSize != 10 {
		t.Fatalf("默认分页 = page:%d pageSize:%d valid:%v", page, pageSize, valid)
	}
}

// TestTenantPlatformSourceUsesSharedOwner 验证租户请求平台图库时不能切换为其他租户所有者。
func TestTenantPlatformSourceUsesSharedOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/images?source=platform&tenantId=99", nil)
	owner, valid := tenantOwnerFromQuery(context)
	if !valid || owner != nil {
		t.Fatalf("租户平台图库所有者 = %#v valid:%v", owner, valid)
	}
}

// TestNormalizeImageName 验证图片名称会去除首尾空格并限制为 1 至 255 个字符。
func TestNormalizeImageName(t *testing.T) {
	if name, valid := normalizeImageName("  品牌图标.png  "); !valid || name != "品牌图标.png" {
		t.Fatalf("名称校验结果 = %q %v", name, valid)
	}
	for _, value := range []string{"   ", strings.Repeat("图", 256)} {
		if _, valid := normalizeImageName(value); valid {
			t.Fatalf("名称 %q 应被拒绝", value)
		}
	}
}

// TestUpdateImageRequestDistinguishesCategoryNull 验证分类显式 null 不会被当成字段缺失。
func TestUpdateImageRequestDistinguishesCategoryNull(t *testing.T) {
	var request updateImageRequest
	if err := json.Unmarshal([]byte(`{"categoryId":null}`), &request); err != nil {
		t.Fatalf("json.Unmarshal() 返回错误: %v", err)
	}
	if len(request.CategoryID) == 0 || request.OriginalName != nil {
		t.Fatalf("分类更新字段识别错误: %#v", request)
	}
	categoryID, valid := parseImageCategoryID(request.CategoryID)
	if !valid || categoryID != nil {
		t.Fatalf("分类 null 解析结果 = %#v %v", categoryID, valid)
	}
	for _, raw := range []string{"0", `"invalid"`} {
		if _, valid := parseImageCategoryID(json.RawMessage(raw)); valid {
			t.Fatalf("分类值 %s 应被拒绝", raw)
		}
	}
}

// TestReadAndValidateImageAcceptsDecodedPNG 验证服务端按真实解码结果接受 PNG，而不是只相信文件名。
func TestReadAndValidateImageAcceptsDecodedPNG(t *testing.T) {
	imageData := bytes.NewBuffer(nil)
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(imageData, picture); err != nil {
		t.Fatalf("png.Encode() 返回错误: %v", err)
	}
	header := multipartHeader(t, "brand.fake", imageData.Bytes())
	data, mimeType, extension, err := readAndValidateImage(header)
	if err != nil {
		t.Fatalf("readAndValidateImage() 返回错误: %v", err)
	}
	if len(data) == 0 || mimeType != "image/png" || extension != ".png" {
		t.Fatalf("校验结果 = %d %s %s", len(data), mimeType, extension)
	}
}

// TestReadAndValidateImageRejectsInvalidAndOversizedFiles 验证伪图片和超过 5MB 的内容被拒绝。
func TestReadAndValidateImageRejectsInvalidAndOversizedFiles(t *testing.T) {
	for name, data := range map[string][]byte{
		"fake.png":  []byte("not an image"),
		"large.png": bytes.Repeat([]byte{1}, maxImageSize+1),
	} {
		header := multipartHeader(t, name, data)
		if _, _, _, err := readAndValidateImage(header); err == nil {
			t.Fatalf("文件 %s 应被拒绝", name)
		}
	}
}

// TestReadAndValidateAvatarRequiresSquareAndFiveMegabytes 验证头像必须可解码、为正方形且不超过 5MB。
func TestReadAndValidateAvatarRequiresSquareAndFiveMegabytes(t *testing.T) {
	for name, bounds := range map[string]image.Rectangle{
		"square.png":    image.Rect(0, 0, 8, 8),
		"rectangle.png": image.Rect(0, 0, 8, 6),
	} {
		buffer := bytes.NewBuffer(nil)
		if err := png.Encode(buffer, image.NewRGBA(bounds)); err != nil {
			t.Fatalf("png.Encode() 返回错误: %v", err)
		}
		_, mimeType, _, err := readAndValidateAvatar(multipartHeader(t, name, buffer.Bytes()))
		if name == "square.png" {
			if err != nil || mimeType != "image/png" {
				t.Fatalf("正方形头像校验失败: mime=%s err=%v", mimeType, err)
			}
		} else if err == nil {
			t.Fatal("非正方形头像应被拒绝")
		}
	}
	if _, _, _, err := readAndValidateAvatar(multipartHeader(t, "large.webp", bytes.Repeat([]byte{1}, maxAvatarSize+1))); err == nil {
		t.Fatal("超过 5MB 的头像应被拒绝")
	}
	if _, _, _, err := readAndValidateAvatar(multipartHeader(t, "fake.png", []byte("not an image"))); err == nil {
		t.Fatal("伪图片头像应被拒绝")
	}
}

// TestCreateObjectKeySeparatesPlatformAndTenant 验证对象键使用随机值和所有者命名空间且不包含原文件名。
func TestCreateObjectKeySeparatesPlatformAndTenant(t *testing.T) {
	platformKey, err := createObjectKey(nil, ".png")
	if err != nil {
		t.Fatalf("createObjectKey() 返回错误: %v", err)
	}
	tenantID := uint64(42)
	tenantKey, err := createObjectKey(&tenantID, ".webp")
	if err != nil {
		t.Fatalf("createObjectKey() 返回错误: %v", err)
	}
	if !strings.HasPrefix(platformKey, "platform/") || !strings.HasPrefix(tenantKey, "tenants/42/") {
		t.Fatalf("对象键命名空间不正确: %s %s", platformKey, tenantKey)
	}
	if platformKey == tenantKey || strings.Contains(platformKey, "brand") || strings.Contains(tenantKey, "brand") {
		t.Fatalf("对象键不应重复或包含原文件名: %s %s", platformKey, tenantKey)
	}
}

// TestImageResponseDoesNotExposeObjectKey 验证图片接口序列化结果不会包含 MinIO 对象键。
func TestImageResponseDoesNotExposeObjectKey(t *testing.T) {
	response := toImageResponse(ImageAsset{ID: 1, OriginalName: "logo.png", ObjectKey: "platform/private-key.png", MIMEType: "image/png"}, "https://example.test/presigned")
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() 返回错误: %v", err)
	}
	if strings.Contains(string(encoded), "objectKey") || strings.Contains(string(encoded), "private-key") {
		t.Fatalf("响应泄露对象键: %s", encoded)
	}
}

// TestPublicImageReturnsUnavailableWithoutStorage 验证对象存储不可用时公开图片接口稳定返回 503。
func TestPublicImageReturnsUnavailableWithoutStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(nil, UnavailableStorage{}))
	router := gin.New()
	router.GET("/api/public/images/:imageId", handler.PublicImage)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/public/images/1", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":50002`) {
		t.Fatalf("不可用响应 = %d %s", recorder.Code, recorder.Body.String())
	}
}

// multipartHeader 创建带单个文件的 multipart 头，供真实文件读取校验测试复用。
func multipartHeader(t *testing.T, filename string, data []byte) *multipart.FileHeader {
	t.Helper()
	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() 返回错误: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("Write() 返回错误: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() 返回错误: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(int64(len(data) + 1024)); err != nil {
		t.Fatalf("ParseMultipartForm() 返回错误: %v", err)
	}
	return request.MultipartForm.File["file"][0]
}
