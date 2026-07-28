package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWechatClientExchangePhone 验证手机号 code 通过 access token 换取可信手机号。
func TestWechatClientExchangePhone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	})
	mux.HandleFunc("/phone", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "token" {
			t.Fatalf("access_token = %q", request.URL.Query().Get("access_token"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"errcode":0,"phone_info":{"phoneNumber":"13800138000","purePhoneNumber":"13800138000"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewWechatClient(WechatConfig{AppSecret: "secret"})
	client.tokenURL = server.URL + "/token"
	client.phoneURL = server.URL + "/phone"
	phone, err := client.ExchangePhone(context.Background(), "wx-test", "phone-code")
	if err != nil || phone != "13800138000" {
		t.Fatalf("ExchangePhone() = %q, %v", phone, err)
	}
}

// TestWechatClientExchangePhoneRejectsInvalidCode 验证微信明确拒绝手机号 code 时返回凭证无效。
func TestWechatClientExchangePhoneRejectsInvalidCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	})
	mux.HandleFunc("/phone", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewWechatClient(WechatConfig{AppSecret: "secret"})
	client.tokenURL = server.URL + "/token"
	client.phoneURL = server.URL + "/phone"
	_, err := client.ExchangePhone(context.Background(), "wx-test", "phone-code")
	if err != errWechatCodeInvalid {
		t.Fatalf("ExchangePhone() 错误 = %v", err)
	}
}

// TestWechatClientGenerateUnlimitedCodeIncludesEnvironment 验证小程序码请求携带固定页面和目标版本。
func TestWechatClientGenerateUnlimitedCodeIncludesEnvironment(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	})
	mux.HandleFunc("/code", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("小程序码请求方法 = %s，期望 POST", request.Method)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("小程序码 Content-Type = %q", request.Header.Get("Content-Type"))
		}
		if request.URL.Query().Get("access_token") != "token" {
			t.Fatalf("小程序码 access_token 未通过 QueryString 发送")
		}
		for _, field := range []string{"scene", "page", "check_path", "env_version", "width"} {
			if request.URL.Query().Has(field) {
				t.Fatalf("小程序码业务参数 %q 不应出现在 QueryString", field)
			}
		}
		var payload struct {
			Scene      string `json:"scene"`
			Page       string `json:"page"`
			CheckPath  bool   `json:"check_path"`
			EnvVersion string `json:"env_version"`
			Width      int    `json:"width"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析小程序码请求失败: %v", err)
		}
		if payload.Scene != "42" ||
			payload.Page != wechatMiniappCodePage ||
			payload.CheckPath ||
			payload.EnvVersion != "trial" ||
			payload.Width != 430 {
			t.Fatalf("小程序码请求参数不正确: %#v", payload)
		}
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(append(append([]byte{}, wechatPNGSignature...), 'c', 'o', 'd', 'e'))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewWechatClient(WechatConfig{AppSecret: "secret", MiniappCodeEnvVersion: "trial"})
	client.tokenURL = server.URL + "/token"
	client.codeURL = server.URL + "/code"
	image, err := client.GenerateUnlimitedCode(context.Background(), "wx-test", "42")
	if err != nil {
		t.Fatalf("GenerateUnlimitedCode() 返回错误: %v", err)
	}
	expected := append(append([]byte{}, wechatPNGSignature...), 'c', 'o', 'd', 'e')
	if !bytes.Equal(image, expected) {
		t.Fatalf("小程序码响应 = %v，期望 %v", image, expected)
	}
}

// TestWechatClientGenerateUnlimitedCodeAcceptsWechatJPEG 验证微信真实返回的 JPEG 小程序码可以通过校验。
func TestWechatClientGenerateUnlimitedCodeAcceptsWechatJPEG(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	})
	mux.HandleFunc("/code", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write(append(append([]byte{}, wechatJPEGSignature...), 'c', 'o', 'd', 'e'))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewWechatClient(WechatConfig{AppSecret: "secret", MiniappCodeEnvVersion: "develop"})
	client.tokenURL = server.URL + "/token"
	client.codeURL = server.URL + "/code"
	image, err := client.GenerateUnlimitedCode(context.Background(), "wx-test", "42")
	if err != nil || !bytes.HasPrefix(image, wechatJPEGSignature) {
		t.Fatalf("GenerateUnlimitedCode() = %v, %v", image, err)
	}
}

// TestWechatClientGenerateUnlimitedCodeRejectsJSONError 验证微信错误 JSON 不会被当作图片返回。
func TestWechatClientGenerateUnlimitedCodeRejectsJSONError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	})
	mux.HandleFunc("/code", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"errcode":40097,"errmsg":"invalid args"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewWechatClient(WechatConfig{AppSecret: "secret", MiniappCodeEnvVersion: "develop"})
	client.tokenURL = server.URL + "/token"
	client.codeURL = server.URL + "/code"
	_, err := client.GenerateUnlimitedCode(context.Background(), "wx-test", "42")
	if err != errWechatUnavailable {
		t.Fatalf("GenerateUnlimitedCode() 错误 = %v", err)
	}
}

// TestWechatClientConfiguredTransportCoversAllRequests 验证注入的 Transport 覆盖微信客户端全部四类出站请求。
func TestWechatClientConfiguredTransportCoversAllRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"openid":"openid-1"}`))
	})
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	})
	mux.HandleFunc("/phone", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"errcode":0,"phone_info":{"phoneNumber":"13800138000"}}`))
	})
	mux.HandleFunc("/code", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(append(append([]byte{}, wechatPNGSignature...), 'c', 'o', 'd', 'e'))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	recorder := &recordingRoundTripper{base: http.DefaultTransport}
	client := NewWechatClient(WechatConfig{AppSecret: "secret", MiniappCodeEnvVersion: "develop"})
	client.ConfigureHTTPTransport(recorder)
	client.loginURL = server.URL + "/login"
	client.tokenURL = server.URL + "/token"
	client.phoneURL = server.URL + "/phone"
	client.codeURL = server.URL + "/code"

	if _, err := client.Exchange(context.Background(), "wx-test", "login-code"); err != nil {
		t.Fatalf("微信登录请求失败: %v", err)
	}
	if _, err := client.ExchangePhone(context.Background(), "wx-test", "phone-code"); err != nil {
		t.Fatalf("微信手机号请求失败: %v", err)
	}
	if _, err := client.GenerateUnlimitedCode(context.Background(), "wx-test", "42"); err != nil {
		t.Fatalf("微信小程序码请求失败: %v", err)
	}
	for _, path := range []string{"/login", "/token", "/phone", "/code"} {
		if !recorder.paths[path] {
			t.Fatalf("配置的 Transport 未覆盖微信请求 %s: %+v", path, recorder.paths)
		}
	}
}

// TestIsWechatPhone 验证微信手机号只接受数字和首位国际区号加号。
func TestIsWechatPhone(t *testing.T) {
	for _, phone := range []string{"13800138000", "+8613800138000"} {
		if !isWechatPhone(phone) {
			t.Fatalf("合法手机号 %q 被拒绝", phone)
		}
	}
	for _, phone := range []string{"", "+", "138-0013-8000", "86+13800138000"} {
		if isWechatPhone(phone) {
			t.Fatalf("非法手机号 %q 被接受", phone)
		}
	}
}

type recordingRoundTripper struct {
	base  http.RoundTripper
	paths map[string]bool
}

// RoundTrip 记录测试请求路径后交给真实 Transport。
func (transport *recordingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.paths == nil {
		transport.paths = make(map[string]bool)
	}
	transport.paths[request.URL.Path] = true
	return transport.base.RoundTrip(request)
}
