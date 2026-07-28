package logging

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestDevelopmentHTTPMiddlewareRecordsTextExchange 验证入站正文、响应和追踪信息完整写入且不影响 Handler 读取。
func TestDevelopmentHTTPMiddlewareRecordsTextExchange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.Local)
	logger, err := newDevelopmentHTTPLogger(t.TempDir(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("创建开发 HTTP 记录器失败: %v", err)
	}
	defer logger.Close()

	router := gin.New()
	router.Use(logger.Middleware())
	router.Use(func(ginContext *gin.Context) {
		ginContext.Set(requestIDContextKey, "request-1")
		ginContext.Next()
	})
	router.POST("/echo", func(ginContext *gin.Context) {
		content, readErr := io.ReadAll(ginContext.Request.Body)
		if readErr != nil {
			t.Fatalf("Handler 读取请求体失败: %v", readErr)
		}
		ginContext.Header("X-Test-Response", "response-value")
		ginContext.Data(http.StatusCreated, "application/json", content)
	})

	requestBody := `{"password":"raw-password","scene":"42"}`
	request := httptest.NewRequest(http.MethodPost, "http://service.test/echo?access_token=raw-token", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer raw-jwt")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || response.Body.String() != requestBody {
		t.Fatalf("业务响应被日志中间件改变: status=%d body=%s", response.Code, response.Body.String())
	}
	exchanges := readDevelopmentHTTPExchanges(t, logger.directory, now)
	if len(exchanges) != 1 {
		t.Fatalf("入站日志数量 = %d，期望 1", len(exchanges))
	}
	exchange := exchanges[0]
	if exchange.Direction != "inbound" || exchange.TraceID == "" || exchange.RequestID != "request-1" {
		t.Fatalf("入站追踪字段错误: %+v", exchange)
	}
	requestJSON := requireDevelopmentJSONBody(t, exchange.Request.Body)
	if exchange.Request.URL != "http://service.test/echo?access_token=raw-token" ||
		exchange.Request.Headers.Get("Authorization") != "Bearer raw-jwt" ||
		requestJSON["password"] != "raw-password" || requestJSON["scene"] != "42" {
		t.Fatalf("入站请求原文记录错误: %+v", exchange.Request)
	}
	responseJSON := requireDevelopmentJSONBody(t, exchange.Response.Body)
	if exchange.Response == nil || exchange.Response.Status != http.StatusCreated ||
		exchange.Response.Headers.Get("X-Test-Response") != "response-value" ||
		responseJSON["password"] != "raw-password" || responseJSON["scene"] != "42" {
		t.Fatalf("入站响应原文记录错误: %+v", exchange.Response)
	}
}

// TestDevelopmentHTTPMiddlewareOmitsBinaryBodies 验证 multipart 和图片正文只记录类型与字节数。
func TestDevelopmentHTTPMiddlewareOmitsBinaryBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 28, 11, 0, 0, 0, time.Local)
	logger, err := newDevelopmentHTTPLogger(t.TempDir(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("创建开发 HTTP 记录器失败: %v", err)
	}
	defer logger.Close()

	router := gin.New()
	router.Use(logger.Middleware())
	router.POST("/binary", func(ginContext *gin.Context) {
		_, _ = io.Copy(io.Discard, ginContext.Request.Body)
		ginContext.Data(http.StatusOK, "image/png", []byte{0x89, 'P', 'N', 'G'})
	})
	requestContent := []byte("multipart-binary-content")
	request := httptest.NewRequest(http.MethodPost, "http://service.test/binary", bytes.NewReader(requestContent))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	exchange := readDevelopmentHTTPExchanges(t, logger.directory, now)[0]
	if !exchange.Request.BinaryBody || exchange.Request.Body != nil || exchange.Request.BodyBytes != int64(len(requestContent)) {
		t.Fatalf("multipart 请求正文处理错误: %+v", exchange.Request)
	}
	if exchange.Response == nil || !exchange.Response.BinaryBody || exchange.Response.Body != nil || exchange.Response.BodyBytes != 4 {
		t.Fatalf("图片响应正文处理错误: %+v", exchange.Response)
	}
}

// TestDevelopmentHTTPTransportRecordsAndRestoresBodies 验证出站日志记录完整 URL 和正文且不消耗响应。
func TestDevelopmentHTTPTransportRecordsAndRestoresBodies(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.Local)
	logger, err := newDevelopmentHTTPLogger(t.TempDir(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("创建开发 HTTP 记录器失败: %v", err)
	}
	defer logger.Close()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		content, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatalf("测试服务读取请求体失败: %v", readErr)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Upstream", "wechat")
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	client := &http.Client{Transport: logger.WrapTransport(http.DefaultTransport)}
	requestBody := `{"scene":"42","check_path":false}`
	request, err := http.NewRequestWithContext(
		context.WithValue(context.Background(), developmentTraceContextKey{}, "trace-1"),
		http.MethodPost,
		server.URL+"/wxa/getwxacodeunlimit?access_token=raw-token",
		strings.NewReader(requestBody),
	)
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("发送测试请求失败: %v", err)
	}
	responseBody, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || string(responseBody) != requestBody {
		t.Fatalf("出站响应未正确恢复: body=%s err=%v", responseBody, err)
	}

	exchange := readDevelopmentHTTPExchanges(t, logger.directory, now)[0]
	requestJSON := requireDevelopmentJSONBody(t, exchange.Request.Body)
	if exchange.Direction != "outbound" || exchange.TraceID != "trace-1" ||
		!strings.Contains(exchange.Request.URL, "access_token=raw-token") ||
		requestJSON["scene"] != "42" || requestJSON["check_path"] != false {
		t.Fatalf("出站请求日志错误: %+v", exchange)
	}
	responseJSON := requireDevelopmentJSONBody(t, exchange.Response.Body)
	if exchange.Response == nil || responseJSON["scene"] != "42" || responseJSON["check_path"] != false ||
		exchange.Response.Headers.Get("X-Upstream") != "wechat" {
		t.Fatalf("出站响应日志错误: %+v", exchange.Response)
	}
}

// TestDevelopmentHTTPMiddlewareSummarizesBase64 验证日志省略图片和较长 Base64，但真实响应保持完整。
func TestDevelopmentHTTPMiddlewareSummarizesBase64(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 28, 12, 30, 0, 0, time.Local)
	logger, err := newDevelopmentHTTPLogger(t.TempDir(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("创建开发 HTTP 记录器失败: %v", err)
	}
	defer logger.Close()

	imageContent := []byte{0x89, 'P', 'N', 'G', 1, 2, 3, 4}
	imageBase64 := base64.StdEncoding.EncodeToString(imageContent)
	dataURL := "data:image/png;base64," + imageBase64
	largeContent := bytes.Repeat([]byte("generic-base64"), 400)
	largeBase64 := base64.StdEncoding.EncodeToString(largeContent)
	responseContent, err := json.Marshal(map[string]any{
		"image": dataURL,
		"blob":  largeBase64,
		"name":  "租户太阳码",
	})
	if err != nil {
		t.Fatalf("构造 Base64 测试响应失败: %v", err)
	}

	router := gin.New()
	router.Use(logger.Middleware())
	router.GET("/code", func(ginContext *gin.Context) {
		ginContext.Data(http.StatusOK, "application/json", responseContent)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://service.test/code", nil))
	if !strings.Contains(response.Body.String(), imageBase64) || !strings.Contains(response.Body.String(), largeBase64) {
		t.Fatal("真实响应中的 Base64 被日志处理改变")
	}

	exchange := readDevelopmentHTTPExchanges(t, logger.directory, now)[0]
	responseJSON := requireDevelopmentJSONBody(t, exchange.Response.Body)
	imageSummary := requireDevelopmentJSONBody(t, responseJSON["image"])
	imageDigest := sha256.Sum256(imageContent)
	if imageSummary["omitted"] != true || imageSummary["mime"] != "image/png" ||
		imageSummary["decodedBytes"] != float64(len(imageContent)) ||
		imageSummary["sha256"] != hex.EncodeToString(imageDigest[:]) {
		t.Fatalf("图片 Base64 摘要错误: %+v", imageSummary)
	}
	blobSummary := requireDevelopmentJSONBody(t, responseJSON["blob"])
	if blobSummary["omitted"] != true || blobSummary["mime"] != nil ||
		blobSummary["decodedBytes"] != float64(len(largeContent)) {
		t.Fatalf("普通长 Base64 摘要错误: %+v", blobSummary)
	}
	logContent, err := os.ReadFile(filepath.Join(logger.directory, "http-"+now.Format("2006-01-02")+".jsonl"))
	if err != nil {
		t.Fatalf("读取 Base64 日志失败: %v", err)
	}
	if bytes.Contains(logContent, []byte(imageBase64)) || bytes.Contains(logContent, []byte(largeBase64)) {
		t.Fatal("JSONL 仍包含被省略的 Base64 原文")
	}
}

// TestDevelopmentHTTPTransportRecordsErrors 验证出站网络错误也会形成可检索日志。
func TestDevelopmentHTTPTransportRecordsErrors(t *testing.T) {
	now := time.Date(2026, 7, 28, 13, 0, 0, 0, time.Local)
	logger, err := newDevelopmentHTTPLogger(t.TempDir(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("创建开发 HTTP 记录器失败: %v", err)
	}
	defer logger.Close()

	transport := logger.WrapTransport(roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	}))
	request := httptest.NewRequest(http.MethodGet, "http://wechat.test/token?secret=raw-secret", nil)
	_, err = transport.RoundTrip(request)
	if err == nil {
		t.Fatal("出站网络错误未返回")
	}
	exchange := readDevelopmentHTTPExchanges(t, logger.directory, now)[0]
	if exchange.Error == "" || !strings.Contains(exchange.Request.URL, "secret=raw-secret") {
		t.Fatalf("出站错误日志不完整: %+v", exchange)
	}
}

// TestDevelopmentHTTPLoggerRotatesCleansAndSerializes 验证权限、七天清理、跨日轮转和并发 JSONL 写入。
func TestDevelopmentHTTPLoggerRotatesCleansAndSerializes(t *testing.T) {
	directory := t.TempDir()
	oldPath := filepath.Join(directory, "http-2026-07-20.jsonl")
	recentPath := filepath.Join(directory, "http-2026-07-22.jsonl")
	if err := os.WriteFile(oldPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("创建过期日志失败: %v", err)
	}
	if err := os.WriteFile(recentPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("创建保留日志失败: %v", err)
	}
	now := time.Date(2026, 7, 28, 14, 0, 0, 0, time.Local)
	logger, err := newDevelopmentHTTPLogger(directory, func() time.Time { return now })
	if err != nil {
		t.Fatalf("创建开发 HTTP 记录器失败: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("过期日志未删除: %v", err)
	}
	if _, err := os.Stat(recentPath); err != nil {
		t.Fatalf("保留期内日志被删除: %v", err)
	}
	assertFileMode(t, directory, 0o700)
	assertFileMode(t, filepath.Join(directory, "http-2026-07-28.jsonl"), 0o600)

	var waitGroup sync.WaitGroup
	for index := 0; index < 20; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if writeErr := logger.write(developmentHTTPExchange{
				Timestamp: now.UTC(),
				Direction: "inbound",
				TraceID:   newDevelopmentTraceID(),
				Request:   developmentHTTPRequest{Method: http.MethodGet, URL: "http://service.test/ping"},
			}); writeErr != nil {
				t.Errorf("并发写入失败: %v", writeErr)
			}
		}()
	}
	waitGroup.Wait()

	now = now.AddDate(0, 0, 1)
	if err := logger.write(developmentHTTPExchange{
		Timestamp: now.UTC(),
		Direction: "inbound",
		TraceID:   "next-day",
		Request:   developmentHTTPRequest{Method: http.MethodGet, URL: "http://service.test/healthz"},
	}); err != nil {
		t.Fatalf("跨日写入失败: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("关闭开发 HTTP 记录器失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "http-2026-07-29.jsonl")); err != nil {
		t.Fatalf("跨日日志文件未创建: %v", err)
	}
	if count := countJSONLines(t, filepath.Join(directory, "http-2026-07-28.jsonl")); count != 20 {
		t.Fatalf("并发 JSONL 行数 = %d，期望 20", count)
	}
}

// TestDevelopmentBodyCaptureTruncatesText 验证文本正文超过八兆后只截断日志副本。
func TestDevelopmentBodyCaptureTruncatesText(t *testing.T) {
	capture := newDevelopmentBodyCapture("application/json", nil)
	content := bytes.Repeat([]byte("a"), developmentHTTPBodyLimit+1)
	written, err := capture.Write(content)
	if err != nil || written != len(content) {
		t.Fatalf("捕获正文失败: written=%d err=%v", written, err)
	}
	if capture.buffer.Len() != developmentHTTPBodyLimit || capture.bodyBytes != int64(len(content)) || !capture.truncated {
		t.Fatalf("正文截断状态错误: bytes=%d buffer=%d truncated=%v", capture.bodyBytes, capture.buffer.Len(), capture.truncated)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip 调用测试函数实现 http.RoundTripper。
func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

// readDevelopmentHTTPExchanges 读取指定日期的全部开发 HTTP JSONL 记录。
func readDevelopmentHTTPExchanges(t *testing.T, directory string, date time.Time) []developmentHTTPExchange {
	t.Helper()
	path := filepath.Join(directory, developmentHTTPFilePrefix+date.Format("2006-01-02")+developmentHTTPFileSuffix)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开开发 HTTP 日志失败: %v", err)
	}
	defer file.Close()
	var exchanges []developmentHTTPExchange
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), developmentHTTPBodyLimit*2)
	for scanner.Scan() {
		var exchange developmentHTTPExchange
		if err := json.Unmarshal(scanner.Bytes(), &exchange); err != nil {
			t.Fatalf("解析开发 HTTP 日志失败: %v", err)
		}
		exchanges = append(exchanges, exchange)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("读取开发 HTTP 日志失败: %v", err)
	}
	return exchanges
}

// assertFileMode 验证开发日志目录或文件权限。
func assertFileMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("读取文件权限失败: %v", err)
	}
	if mode := info.Mode().Perm(); mode != expected {
		t.Fatalf("%s 权限 = %o，期望 %o", path, mode, expected)
	}
}

// countJSONLines 验证文件每一行都是独立 JSON 并返回行数。
func countJSONLines(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开 JSONL 文件失败: %v", err)
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var value map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			t.Fatalf("JSONL 第 %d 行无效: %v", count+1, err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描 JSONL 文件失败: %v", err)
	}
	return count
}

// requireDevelopmentJSONBody 断言日志正文是 JSON 对象并返回其字段。
func requireDevelopmentJSONBody(t *testing.T, body any) map[string]any {
	t.Helper()
	value, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("日志正文不是 JSON 对象: %#v", body)
	}
	return value
}
