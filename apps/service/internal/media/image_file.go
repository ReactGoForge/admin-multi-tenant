package media

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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

	_ "golang.org/x/image/webp"
)

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
