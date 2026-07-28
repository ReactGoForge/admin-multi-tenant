package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	wechatLoginEndpoint       = "https://api.weixin.qq.com/sns/jscode2session"
	wechatAccessTokenEndpoint = "https://api.weixin.qq.com/cgi-bin/token"
	wechatMiniappCodeEndpoint = "https://api.weixin.qq.com/wxa/getwxacodeunlimit"
	wechatMiniappCodePage     = "pages/index/index"
	wechatPhoneEndpoint       = "https://api.weixin.qq.com/wxa/business/getuserphonenumber"
	wechatRequestTimeout      = 8 * time.Second
	wechatMaximumBodyBytes    = 2 * 1024 * 1024
	wechatMaximumPhoneBytes   = 16 * 1024
)

var (
	wechatPNGSignature   = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	wechatJPEGSignature  = []byte{0xff, 0xd8, 0xff}
	errWechatUnavailable = errors.New("wechat service unavailable")
	errWechatCodeInvalid = errors.New("wechat login code invalid")
)

// WechatIdentity 描述微信登录成功后后端可信的最小用户身份。
type WechatIdentity struct {
	OpenID  string
	UnionID *string
}

// WechatExchanger 定义小程序临时登录凭证换取微信身份的最小能力。
type WechatExchanger interface {
	Exchange(context.Context, string, string) (WechatIdentity, error)
	ExchangePhone(context.Context, string, string) (string, error)
}

// MiniappCodeGenerator 定义按租户场景生成小程序码的最小能力。
type MiniappCodeGenerator interface {
	GenerateUnlimitedCode(context.Context, string, string) ([]byte, error)
}

// WechatClient 使用指定 AppID 与服务器密钥调用微信登录和小程序码接口。
type WechatClient struct {
	secret                string
	miniappCodeEnvVersion string
	client                *http.Client
	loginURL              string
	tokenURL              string
	codeURL               string
	phoneURL              string
	// Go 学习提示：Mutex 保护下面三个 Token 缓存字段，防止多个 goroutine 并发刷新时产生数据竞争。
	mutex    sync.Mutex
	token    string
	tokenApp string
	tokenExp time.Time
}

// NewWechatClient 创建带固定超时且不会暴露密钥的微信客户端。
func NewWechatClient(config WechatConfig) *WechatClient {
	return &WechatClient{
		secret:                config.AppSecret,
		miniappCodeEnvVersion: config.MiniappCodeEnvVersion,
		client:                &http.Client{Timeout: wechatRequestTimeout},
		loginURL:              wechatLoginEndpoint,
		tokenURL:              wechatAccessTokenEndpoint,
		codeURL:               wechatMiniappCodeEndpoint,
		phoneURL:              wechatPhoneEndpoint,
	}
}

// ConfigureHTTPTransport 为微信客户端配置出站 HTTP Transport；仅用于进程入口注入开发日志包装。
func (client *WechatClient) ConfigureHTTPTransport(transport http.RoundTripper) {
	if transport != nil {
		client.client.Transport = transport
	}
}

// wechatLoginResponse 映射微信 code2Session 接口返回的用户身份。
type wechatLoginResponse struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	ErrorCode  int    `json:"errcode"`
	ErrorValue string `json:"errmsg"`
}

// wechatTokenResponse 映射微信 access_token 接口返回的数据。
type wechatTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrorCode   int    `json:"errcode"`
}

// wechatPhoneResponse 映射微信手机号接口返回的可信号码。
type wechatPhoneResponse struct {
	ErrorCode  int    `json:"errcode"`
	ErrorValue string `json:"errmsg"`
	PhoneInfo  struct {
		PhoneNumber     string `json:"phoneNumber"`
		PurePhoneNumber string `json:"purePhoneNumber"`
	} `json:"phone_info"`
}

// Exchange 将一次性 code 交给微信服务验证，不保存或返回 session_key。
func (client *WechatClient) Exchange(ctx context.Context, appID, code string) (WechatIdentity, error) {
	if !client.available(appID) {
		return WechatIdentity{}, errWechatUnavailable
	}
	query := url.Values{
		"appid":      {strings.TrimSpace(appID)},
		"secret":     {client.secret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.loginURL+"?"+query.Encode(), nil)
	if err != nil {
		return WechatIdentity{}, fmt.Errorf("创建微信登录请求失败: %w", err)
	}
	// 安全边界：code 是微信签发的一次性短期凭证，只有微信服务返回的 OpenID 才能作为可信身份。
	// AppSecret 只在服务端参与请求，不写入响应、日志或小程序 Token。
	response, err := client.client.Do(request)
	if err != nil {
		return WechatIdentity{}, errWechatUnavailable
	}
	// Go 学习提示：defer 会在当前函数返回前执行，确保无论后面从哪个分支 return，
	// HTTP 响应体都会关闭并归还底层连接资源。
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return WechatIdentity{}, errWechatUnavailable
	}
	var payload wechatLoginResponse
	// 安全边界：即使响应来自外部服务，也要限制可读取大小，避免异常响应耗尽内存。
	if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024)).Decode(&payload); err != nil {
		return WechatIdentity{}, errWechatUnavailable
	}
	if payload.ErrorCode != 0 {
		if payload.ErrorCode == 40029 || payload.ErrorCode == 40163 {
			return WechatIdentity{}, errWechatCodeInvalid
		}
		return WechatIdentity{}, errWechatUnavailable
	}
	payload.OpenID = strings.TrimSpace(payload.OpenID)
	if payload.OpenID == "" || len(payload.OpenID) > 64 {
		return WechatIdentity{}, errWechatUnavailable
	}
	identity := WechatIdentity{OpenID: payload.OpenID}
	if unionID := strings.TrimSpace(payload.UnionID); unionID != "" {
		if len(unionID) > 64 {
			return WechatIdentity{}, errWechatUnavailable
		}
		identity.UnionID = &unionID
	}
	return identity, nil
}

// ExchangePhone 使用一次性手机号 code 向微信换取可信手机号。
func (client *WechatClient) ExchangePhone(ctx context.Context, appID, code string) (string, error) {
	if !client.available(appID) || strings.TrimSpace(code) == "" {
		return "", errWechatUnavailable
	}
	token, err := client.accessToken(ctx, strings.TrimSpace(appID))
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]string{"code": strings.TrimSpace(code)})
	if err != nil {
		return "", errWechatUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.phoneURL+"?access_token="+url.QueryEscape(token), bytes.NewReader(body))
	if err != nil {
		return "", errWechatUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return "", errWechatUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errWechatUnavailable
	}
	var payload wechatPhoneResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, wechatMaximumPhoneBytes)).Decode(&payload); err != nil {
		return "", errWechatUnavailable
	}
	if payload.ErrorCode != 0 {
		if payload.ErrorCode == 40029 || payload.ErrorCode == 40163 {
			return "", errWechatCodeInvalid
		}
		return "", errWechatUnavailable
	}
	phone := strings.TrimSpace(payload.PhoneInfo.PhoneNumber)
	if phone == "" {
		phone = strings.TrimSpace(payload.PhoneInfo.PurePhoneNumber)
	}
	if phone == "" || len(phone) > 20 || !isWechatPhone(phone) {
		return "", errWechatUnavailable
	}
	return phone, nil
}

// GenerateUnlimitedCode 生成携带租户 ID 场景值的小程序码原始图片字节。
func (client *WechatClient) GenerateUnlimitedCode(ctx context.Context, appID, scene string) ([]byte, error) {
	if !client.available(appID) {
		return nil, errWechatUnavailable
	}
	token, err := client.accessToken(ctx, strings.TrimSpace(appID))
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]any{
		"scene":       scene,
		"page":        wechatMiniappCodePage,
		"check_path":  false,
		"env_version": client.miniappCodeEnvVersion,
		"width":       430,
	})
	if err != nil {
		return nil, errWechatUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.codeURL+"?access_token="+url.QueryEscape(token), bytes.NewReader(body))
	if err != nil {
		return nil, errWechatUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return nil, errWechatUnavailable
	}
	defer response.Body.Close()
	// 安全边界：多读 1 字节用于判断响应是否超过上限；微信出错时可能返回 JSON，
	// 因此还要检查 Content-Type 和图片魔数，避免把错误正文伪装成图片返回给前端。
	content, err := io.ReadAll(io.LimitReader(response.Body, wechatMaximumBodyBytes+1))
	contentType := response.Header.Get("Content-Type")
	if err != nil || len(content) == 0 || len(content) > wechatMaximumBodyBytes {
		return nil, errWechatUnavailable
	}
	if strings.Contains(contentType, "json") {
		return nil, errWechatUnavailable
	}
	_, _, ok := miniappCodeImageFormat(content)
	if response.StatusCode != http.StatusOK || !ok {
		return nil, errWechatUnavailable
	}
	return content, nil
}

// miniappCodeImageFormat 根据图片魔数识别微信返回的小程序码真实格式。
func miniappCodeImageFormat(content []byte) (contentType, extension string, ok bool) {
	switch {
	case bytes.HasPrefix(content, wechatPNGSignature):
		return "image/png", "png", true
	case bytes.HasPrefix(content, wechatJPEGSignature):
		return "image/jpeg", "jpg", true
	default:
		return "", "", false
	}
}

// available 判断微信客户端调用所需的 AppID 和密钥是否都已配置。
func (client *WechatClient) available(appID string) bool {
	return strings.TrimSpace(client.secret) != "" && strings.TrimSpace(appID) != ""
}

// isWechatPhone 判断微信返回的手机号只包含数字和可选的国际区号前缀。
func isWechatPhone(phone string) bool {
	if phone == "" {
		return false
	}
	digitCount := 0
	for index, character := range phone {
		if character >= '0' && character <= '9' {
			digitCount++
			continue
		}
		if character == '+' && index == 0 {
			continue
		}
		return false
	}
	return digitCount > 0
}

// accessToken 获取微信接口调用凭证，并按 AppID 在当前进程内缓存。
func (client *WechatClient) accessToken(ctx context.Context, appID string) (string, error) {
	// Go 学习提示：Lock 后立即 defer Unlock，可以保证后续任意 return 分支都会释放锁。
	// 当前实现也让同一时刻只有一个请求刷新 Token，其他请求等待并复用刷新结果。
	client.mutex.Lock()
	defer client.mutex.Unlock()
	// 安全边界：提前五分钟认为缓存过期，可降低临界时刻使用已失效 Token 的概率。
	if client.token != "" && client.tokenApp == appID && time.Now().Add(5*time.Minute).Before(client.tokenExp) {
		return client.token, nil
	}
	query := url.Values{"grant_type": {"client_credential"}, "appid": {appID}, "secret": {client.secret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.tokenURL+"?"+query.Encode(), nil)
	if err != nil {
		return "", errWechatUnavailable
	}
	response, err := client.client.Do(request)
	if err != nil {
		return "", errWechatUnavailable
	}
	defer response.Body.Close()
	var payload wechatTokenResponse
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 16*1024)).Decode(&payload) != nil || payload.ErrorCode != 0 || strings.TrimSpace(payload.AccessToken) == "" || payload.ExpiresIn <= 0 {
		return "", errWechatUnavailable
	}
	client.token = payload.AccessToken
	client.tokenApp = appID
	client.tokenExp = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	return client.token, nil
}
