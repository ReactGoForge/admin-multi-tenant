package logging

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	developmentHTTPBodyLimit     = 8 * 1024 * 1024
	developmentHTTPRetentionDays = 7
	developmentHTTPFilePrefix    = "http-"
	developmentHTTPFileSuffix    = ".jsonl"
	developmentBase64MinimumSize = 4 * 1024
)

type developmentTraceContextKey struct{}

// DevelopmentHTTPLogger 将开发环境的完整 HTTP 交换按天写入 JSONL 文件。
type DevelopmentHTTPLogger struct {
	directory   string
	now         func() time.Time
	mutex       sync.Mutex
	currentDate string
	file        *os.File
}

type developmentHTTPExchange struct {
	Timestamp  time.Time                `json:"timestamp"`
	Direction  string                   `json:"direction"`
	TraceID    string                   `json:"traceId"`
	RequestID  string                   `json:"requestId,omitempty"`
	Request    developmentHTTPRequest   `json:"request"`
	Response   *developmentHTTPResponse `json:"response,omitempty"`
	DurationMS int64                    `json:"durationMs"`
	Error      string                   `json:"error,omitempty"`
}

type developmentHTTPRequest struct {
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	Headers     http.Header `json:"headers"`
	Body        any         `json:"body,omitempty"`
	BodyBytes   int64       `json:"bodyBytes"`
	Truncated   bool        `json:"truncated,omitempty"`
	BinaryBody  bool        `json:"binaryBody,omitempty"`
	ContentType string      `json:"contentType,omitempty"`
}

type developmentHTTPResponse struct {
	Status      int         `json:"status"`
	Headers     http.Header `json:"headers"`
	Body        any         `json:"body,omitempty"`
	BodyBytes   int64       `json:"bodyBytes"`
	Truncated   bool        `json:"truncated,omitempty"`
	BinaryBody  bool        `json:"binaryBody,omitempty"`
	ContentType string      `json:"contentType,omitempty"`
}

type developmentBodyCapture struct {
	buffer      bytes.Buffer
	contentType string
	bodyBytes   int64
	storeBody   bool
	truncated   bool
}

type developmentRequestBody struct {
	reader io.Reader
	closer io.Closer
}

type developmentResponseWriter struct {
	gin.ResponseWriter
	capture *developmentBodyCapture
}

type developmentHTTPTransport struct {
	base   http.RoundTripper
	logger *DevelopmentHTTPLogger
}

type developmentBase64Summary struct {
	Omitted      bool   `json:"omitted"`
	Encoding     string `json:"encoding"`
	MIME         string `json:"mime,omitempty"`
	EncodedBytes int    `json:"encodedBytes"`
	DecodedBytes int    `json:"decodedBytes"`
	SHA256       string `json:"sha256"`
}

// NewDevelopmentHTTPLogger 创建开发 HTTP 文件记录器并立即准备当天日志文件。
func NewDevelopmentHTTPLogger(directory string) (*DevelopmentHTTPLogger, error) {
	return newDevelopmentHTTPLogger(directory, time.Now)
}

// newDevelopmentHTTPLogger 使用指定时钟创建记录器，便于验证跨日轮转和过期清理。
func newDevelopmentHTTPLogger(directory string, now func() time.Time) (*DevelopmentHTTPLogger, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil, fmt.Errorf("开发 HTTP 日志目录不能为空")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("创建开发 HTTP 日志目录失败: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("设置开发 HTTP 日志目录权限失败: %w", err)
	}
	logger := &DevelopmentHTTPLogger{directory: directory, now: now}
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	if err := logger.rotateLocked(now()); err != nil {
		return nil, err
	}
	return logger, nil
}

// Middleware 记录全部 Gin 入站请求和响应，并把开发追踪 ID 写入标准 Context。
func (logger *DevelopmentHTTPLogger) Middleware() gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		startedAt := logger.now()
		traceID := newDevelopmentTraceID()
		request := ginContext.Request
		requestCapture := newDevelopmentBodyCapture(request.Header.Get("Content-Type"), nil)
		if request.Body != nil {
			request.Body = &developmentRequestBody{
				reader: io.TeeReader(request.Body, requestCapture),
				closer: request.Body,
			}
		}
		requestContext := context.WithValue(request.Context(), developmentTraceContextKey{}, traceID)
		ginContext.Request = request.WithContext(requestContext)

		responseCapture := newDevelopmentBodyCapture("", nil)
		ginContext.Writer = &developmentResponseWriter{
			ResponseWriter: ginContext.Writer,
			capture:        responseCapture,
		}
		ginContext.Next()
		if ginContext.Request.Body != nil {
			_, _ = io.Copy(io.Discard, ginContext.Request.Body)
		}

		exchange := developmentHTTPExchange{
			Timestamp:  startedAt.UTC(),
			Direction:  "inbound",
			TraceID:    traceID,
			RequestID:  RequestID(ginContext),
			Request:    developmentRequestFrom(request, requestCapture, inboundRequestURL(request)),
			Response:   developmentResponseFrom(ginContext.Writer.Status(), ginContext.Writer.Header(), responseCapture),
			DurationMS: logger.now().Sub(startedAt).Milliseconds(),
		}
		if err := logger.write(exchange); err != nil {
			log.Printf("开发 HTTP 入站日志写入失败: %v", err)
		}
	}
}

// WrapTransport 包装出站 HTTP Transport，并保持原请求和响应 Body 可继续读取。
func (logger *DevelopmentHTTPLogger) WrapTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &developmentHTTPTransport{base: base, logger: logger}
}

// Close 关闭当前日志文件；重复调用保持安全。
func (logger *DevelopmentHTTPLogger) Close() error {
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	if logger.file == nil {
		return nil
	}
	err := logger.file.Close()
	logger.file = nil
	return err
}

// Read 读取请求正文并同步保存一份日志副本。
func (body *developmentRequestBody) Read(buffer []byte) (int, error) {
	return body.reader.Read(buffer)
}

// Close 关闭原始请求正文。
func (body *developmentRequestBody) Close() error {
	return body.closer.Close()
}

// Write 将响应正文同时写入客户端和开发日志捕获器。
func (writer *developmentResponseWriter) Write(content []byte) (int, error) {
	writer.capture.ensureContentType(writer.Header().Get("Content-Type"), content)
	_, _ = writer.capture.Write(content)
	return writer.ResponseWriter.Write(content)
}

// WriteString 将字符串响应同时写入客户端和开发日志捕获器。
func (writer *developmentResponseWriter) WriteString(content string) (int, error) {
	writer.capture.ensureContentType(writer.Header().Get("Content-Type"), []byte(content))
	_, _ = writer.capture.Write([]byte(content))
	return writer.ResponseWriter.WriteString(content)
}

// RoundTrip 记录完整出站交换，并在记录后恢复请求和响应正文。
func (transport *developmentHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	startedAt := transport.logger.now()
	traceID := developmentTraceID(request.Context())
	if traceID == "" {
		traceID = newDevelopmentTraceID()
	}
	requestCapture, requestContent, err := readDevelopmentBody(request.Body, request.Header.Get("Content-Type"))
	if err != nil {
		exchange := developmentHTTPExchange{
			Timestamp:  startedAt.UTC(),
			Direction:  "outbound",
			TraceID:    traceID,
			Request:    developmentRequestFrom(request, requestCapture, request.URL.String()),
			DurationMS: transport.logger.now().Sub(startedAt).Milliseconds(),
			Error:      err.Error(),
		}
		_ = transport.logger.write(exchange)
		return nil, err
	}
	if request.Body != nil {
		request.Body = io.NopCloser(bytes.NewReader(requestContent))
	}

	response, roundTripErr := transport.base.RoundTrip(request)
	exchange := developmentHTTPExchange{
		Timestamp:  startedAt.UTC(),
		Direction:  "outbound",
		TraceID:    traceID,
		Request:    developmentRequestFrom(request, requestCapture, request.URL.String()),
		DurationMS: transport.logger.now().Sub(startedAt).Milliseconds(),
	}
	if roundTripErr != nil {
		exchange.Error = roundTripErr.Error()
		if writeErr := transport.logger.write(exchange); writeErr != nil {
			log.Printf("开发 HTTP 出站日志写入失败: %v", writeErr)
		}
		return nil, roundTripErr
	}

	responseCapture, responseContent, readErr := readDevelopmentBody(response.Body, response.Header.Get("Content-Type"))
	if response.Body != nil {
		response.Body = io.NopCloser(bytes.NewReader(responseContent))
	}
	exchange.Response = developmentResponseFrom(response.StatusCode, response.Header, responseCapture)
	exchange.DurationMS = transport.logger.now().Sub(startedAt).Milliseconds()
	if readErr != nil {
		exchange.Error = readErr.Error()
	}
	if writeErr := transport.logger.write(exchange); writeErr != nil {
		log.Printf("开发 HTTP 出站日志写入失败: %v", writeErr)
	}
	if readErr != nil {
		return nil, readErr
	}
	return response, nil
}

// Write 保存正文并统计完整字节数，超过上限时只截断日志副本。
func (capture *developmentBodyCapture) Write(content []byte) (int, error) {
	capture.bodyBytes += int64(len(content))
	if !capture.storeBody {
		return len(content), nil
	}
	remaining := developmentHTTPBodyLimit - capture.buffer.Len()
	if remaining <= 0 {
		capture.truncated = true
		return len(content), nil
	}
	if len(content) > remaining {
		_, _ = capture.buffer.Write(content[:remaining])
		capture.truncated = true
		return len(content), nil
	}
	_, _ = capture.buffer.Write(content)
	return len(content), nil
}

// ensureContentType 在响应未显式声明类型时依据首段正文判断是否属于文本。
func (capture *developmentBodyCapture) ensureContentType(contentType string, content []byte) {
	if capture.contentType != "" {
		return
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" && len(content) > 0 {
		contentType = http.DetectContentType(content)
	}
	capture.contentType = contentType
	capture.storeBody = isDevelopmentTextBody(contentType, content)
}

// write 串行写入单行 JSON，并在跨日时切换日志文件。
func (logger *DevelopmentHTTPLogger) write(exchange developmentHTTPExchange) error {
	encoded, err := json.Marshal(exchange)
	if err != nil {
		return fmt.Errorf("序列化开发 HTTP 日志失败: %w", err)
	}
	encoded = append(encoded, '\n')
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	if err := logger.rotateLocked(logger.now()); err != nil {
		return err
	}
	if _, err := logger.file.Write(encoded); err != nil {
		return fmt.Errorf("写入开发 HTTP 日志失败: %w", err)
	}
	return nil
}

// rotateLocked 根据当前日期打开日志文件，并删除保留期外的旧文件。
func (logger *DevelopmentHTTPLogger) rotateLocked(now time.Time) error {
	date := now.Format("2006-01-02")
	if logger.file != nil && logger.currentDate == date {
		return nil
	}
	if logger.file != nil {
		if err := logger.file.Close(); err != nil {
			return fmt.Errorf("关闭旧开发 HTTP 日志失败: %w", err)
		}
		logger.file = nil
	}
	if err := logger.cleanupLocked(now); err != nil {
		return err
	}
	path := filepath.Join(logger.directory, developmentHTTPFilePrefix+date+developmentHTTPFileSuffix)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("打开开发 HTTP 日志文件失败: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("设置开发 HTTP 日志文件权限失败: %w", err)
	}
	logger.file = file
	logger.currentDate = date
	return nil
}

// cleanupLocked 删除早于七天保留窗口的开发 HTTP 日志文件。
func (logger *DevelopmentHTTPLogger) cleanupLocked(now time.Time) error {
	entries, err := os.ReadDir(logger.directory)
	if err != nil {
		return fmt.Errorf("读取开发 HTTP 日志目录失败: %w", err)
	}
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(developmentHTTPRetentionDays - 1))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dateText, ok := developmentHTTPLogDate(entry.Name())
		if !ok {
			continue
		}
		date, parseErr := time.ParseInLocation("2006-01-02", dateText, now.Location())
		if parseErr == nil && date.Before(cutoff) {
			if removeErr := os.Remove(filepath.Join(logger.directory, entry.Name())); removeErr != nil {
				return fmt.Errorf("删除过期开发 HTTP 日志失败: %w", removeErr)
			}
		}
	}
	return nil
}

// developmentHTTPLogDate 从受管理的日志文件名中提取日期。
func developmentHTTPLogDate(name string) (string, bool) {
	if !strings.HasPrefix(name, developmentHTTPFilePrefix) || !strings.HasSuffix(name, developmentHTTPFileSuffix) {
		return "", false
	}
	date := strings.TrimSuffix(strings.TrimPrefix(name, developmentHTTPFilePrefix), developmentHTTPFileSuffix)
	return date, len(date) == len("2006-01-02")
}

// newDevelopmentBodyCapture 根据 Content-Type 创建只保存文本正文的捕获器。
func newDevelopmentBodyCapture(contentType string, content []byte) *developmentBodyCapture {
	capture := &developmentBodyCapture{contentType: strings.TrimSpace(contentType)}
	capture.storeBody = isDevelopmentTextBody(capture.contentType, content)
	return capture
}

// readDevelopmentBody 读取出站正文，并分别返回日志捕获结果与用于恢复 Body 的完整内容。
func readDevelopmentBody(body io.ReadCloser, contentType string) (*developmentBodyCapture, []byte, error) {
	capture := newDevelopmentBodyCapture(contentType, nil)
	if body == nil {
		return capture, nil, nil
	}
	content, err := io.ReadAll(body)
	closeErr := body.Close()
	_, _ = capture.Write(content)
	if err != nil {
		return capture, content, err
	}
	return capture, content, closeErr
}

// developmentRequestFrom 构建开发日志中的请求快照。
func developmentRequestFrom(request *http.Request, capture *developmentBodyCapture, requestURL string) developmentHTTPRequest {
	if capture == nil {
		capture = newDevelopmentBodyCapture(request.Header.Get("Content-Type"), nil)
	}
	bodyBytes := capture.bodyBytes
	if bodyBytes == 0 && request.ContentLength > 0 {
		bodyBytes = request.ContentLength
	}
	return developmentHTTPRequest{
		Method:      request.Method,
		URL:         requestURL,
		Headers:     request.Header.Clone(),
		Body:        capture.body(),
		BodyBytes:   bodyBytes,
		Truncated:   capture.truncated,
		BinaryBody:  bodyBytes > 0 && !capture.storeBody,
		ContentType: capture.contentType,
	}
}

// developmentResponseFrom 构建开发日志中的响应快照。
func developmentResponseFrom(status int, headers http.Header, capture *developmentBodyCapture) *developmentHTTPResponse {
	if capture == nil {
		capture = newDevelopmentBodyCapture(headers.Get("Content-Type"), nil)
	}
	return &developmentHTTPResponse{
		Status:      status,
		Headers:     headers.Clone(),
		Body:        capture.body(),
		BodyBytes:   capture.bodyBytes,
		Truncated:   capture.truncated,
		BinaryBody:  capture.bodyBytes > 0 && !capture.storeBody,
		ContentType: capture.contentType,
	}
}

// body 返回适合写入 JSONL 的正文；JSON 中的图片 Base64 会递归替换为摘要。
func (capture *developmentBodyCapture) body() any {
	if !capture.storeBody || capture.buffer.Len() == 0 {
		return nil
	}
	content := capture.buffer.Bytes()
	if isDevelopmentJSONBody(capture.contentType, content) {
		var value any
		if json.Unmarshal(content, &value) == nil {
			return summarizeDevelopmentJSONValue(value)
		}
	}
	text := string(content)
	if summary, ok := summarizeDevelopmentBase64(text); ok {
		return summary
	}
	return text
}

// summarizeDevelopmentJSONValue 递归替换 JSON 中的图片 Data URL 和较长 Base64 字符串。
func summarizeDevelopmentJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, field := range typed {
			typed[key] = summarizeDevelopmentJSONValue(field)
		}
		return typed
	case []any:
		for index, item := range typed {
			typed[index] = summarizeDevelopmentJSONValue(item)
		}
		return typed
	case string:
		if summary, ok := summarizeDevelopmentBase64(typed); ok {
			return summary
		}
		return typed
	default:
		return value
	}
}

// summarizeDevelopmentBase64 将图片 Data URL 或较长的有效 Base64 字符串转换为可比较摘要。
func summarizeDevelopmentBase64(value string) (developmentBase64Summary, bool) {
	encoded := value
	mimeType := ""
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		comma := strings.IndexByte(value, ',')
		if comma <= len("data:") {
			return developmentBase64Summary{}, false
		}
		metadata := value[len("data:"):comma]
		parts := strings.Split(metadata, ";")
		if len(parts) == 0 || !strings.HasPrefix(strings.ToLower(parts[0]), "image/") {
			return developmentBase64Summary{}, false
		}
		isBase64 := false
		for _, part := range parts[1:] {
			if strings.EqualFold(strings.TrimSpace(part), "base64") {
				isBase64 = true
				break
			}
		}
		if !isBase64 {
			return developmentBase64Summary{}, false
		}
		mimeType = strings.ToLower(strings.TrimSpace(parts[0]))
		encoded = value[comma+1:]
	} else if len(value) < developmentBase64MinimumSize {
		return developmentBase64Summary{}, false
	}
	decoded, ok := decodeDevelopmentBase64(encoded)
	if !ok {
		return developmentBase64Summary{}, false
	}
	digest := sha256.Sum256(decoded)
	return developmentBase64Summary{
		Omitted:      true,
		Encoding:     "base64",
		MIME:         mimeType,
		EncodedBytes: len(encoded),
		DecodedBytes: len(decoded),
		SHA256:       hex.EncodeToString(digest[:]),
	}, true
}

// decodeDevelopmentBase64 解码常见标准与 URL-safe Base64，同时忽略空白字符。
func decodeDevelopmentBase64(value string) ([]byte, bool) {
	normalized := strings.Map(func(character rune) rune {
		switch character {
		case ' ', '\t', '\r', '\n':
			return -1
		default:
			return character
		}
	}, value)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(normalized)
		if err == nil {
			return decoded, true
		}
	}
	return nil, false
}

// isDevelopmentJSONBody 判断正文是否声明为 JSON 或本身是合法 JSON。
func isDevelopmentJSONBody(contentType string, content []byte) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		mediaType = strings.ToLower(mediaType)
		if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
			return true
		}
	}
	return json.Valid(content)
}

// isDevelopmentTextBody 判断正文是否适合以 UTF-8 文本写入 JSONL。
func isDevelopmentTextBody(contentType string, content []byte) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType != "" {
		mediaType = strings.ToLower(mediaType)
		switch {
		case strings.HasPrefix(mediaType, "multipart/"), strings.HasPrefix(mediaType, "image/"),
			strings.HasPrefix(mediaType, "audio/"), strings.HasPrefix(mediaType, "video/"),
			mediaType == "application/octet-stream", mediaType == "application/pdf":
			return false
		case strings.HasPrefix(mediaType, "text/"),
			mediaType == "application/json", strings.HasSuffix(mediaType, "+json"),
			mediaType == "application/xml", strings.HasSuffix(mediaType, "+xml"),
			mediaType == "application/x-www-form-urlencoded", mediaType == "application/javascript":
			return true
		}
	}
	return len(content) == 0 || utf8.Valid(content)
}

// inboundRequestURL 返回包含协议、主机、路径和 QueryString 的入站完整 URL。
func inboundRequestURL(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + request.Host + request.URL.RequestURI()
}

// newDevelopmentTraceID 生成不可预测的开发 HTTP 追踪 ID。
func newDevelopmentTraceID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(value)
}

// developmentTraceID 从标准 Context 读取当前入站请求的开发追踪 ID。
func developmentTraceID(ctx context.Context) string {
	value, _ := ctx.Value(developmentTraceContextKey{}).(string)
	return value
}
