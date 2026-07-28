package main

import (
	"strings"
	"testing"
)

// TestValidateIdentity 验证平台所有者账号和姓名的清理及长度规则。
func TestValidateIdentity(t *testing.T) {
	tests := []struct {
		name        string
		account     string
		employee    string
		wantAccount string
		wantName    string
		wantError   bool
	}{
		{name: "有效输入并清理空格", account: " superadmin ", employee: " 平台所有者 ", wantAccount: "superadmin", wantName: "平台所有者"},
		{name: "账号为空", account: " ", employee: "平台所有者", wantError: true},
		{name: "姓名为空", account: "superadmin", employee: " ", wantError: true},
		{name: "账号过长", account: strings.Repeat("a", 41), employee: "平台所有者", wantError: true},
		{name: "姓名过长", account: "superadmin", employee: strings.Repeat("云", 31), wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account, name, err := validateIdentity(test.account, test.employee)
			if test.wantError {
				if err == nil {
					t.Fatal("validateIdentity() 未返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateIdentity() 返回错误: %v", err)
			}
			if account != test.wantAccount || name != test.wantName {
				t.Fatalf("得到 (%q, %q)，期望 (%q, %q)", account, name, test.wantAccount, test.wantName)
			}
		})
	}
}

// TestValidatePassword 验证平台所有者密码的长度和二次确认规则。
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name         string
		password     string
		confirmation string
		wantError    bool
	}{
		{name: "六个字符的有效密码", password: "abc123", confirmation: "abc123"},
		{name: "中文字符按字符数校验", password: "平台安全登录密码", confirmation: "平台安全登录密码"},
		{name: "十八个字符的有效密码", password: strings.Repeat("a", 18), confirmation: strings.Repeat("a", 18)},
		{name: "密码少于六个字符", password: "12345", confirmation: "12345", wantError: true},
		{name: "密码超过十八个字符", password: strings.Repeat("a", 19), confirmation: strings.Repeat("a", 19), wantError: true},
		{name: "两次密码不一致", password: "abc123", confirmation: "abc456", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePassword([]byte(test.password), []byte(test.confirmation))
			if test.wantError && err == nil {
				t.Fatal("validatePassword() 未返回错误")
			}
			if !test.wantError && err != nil {
				t.Fatalf("validatePassword() 返回错误: %v", err)
			}
		})
	}
}
