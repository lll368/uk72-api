package common

import (
	"errors"
	"net/smtp"
	"strings"
)

type outlookAuth struct {
	username, password string
	step               int
}

func LoginAuth(username, password string) smtp.Auth {
	return &outlookAuth{
		username: username,
		password: password,
	}
}

func (a *outlookAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	a.step = 0
	return "LOGIN", nil, nil
}

func (a *outlookAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	challenge := strings.ToLower(strings.TrimSpace(string(fromServer)))
	switch {
	case strings.Contains(challenge, "user") ||
		strings.Contains(challenge, "account") ||
		strings.Contains(challenge, "email"):
		a.step = 1
		return []byte(a.username), nil
	case strings.Contains(challenge, "pass") ||
		strings.Contains(challenge, "credential") ||
		strings.Contains(challenge, "secret"):
		a.step = 2
		return []byte(a.password), nil
	}

	// 部分 SMTP 服务商返回非标准 AUTH LOGIN 提示，按协议交互顺序兜底，避免认证器误判中断。
	switch a.step {
	case 0:
		a.step = 1
		return []byte(a.username), nil
	case 1:
		a.step = 2
		return []byte(a.password), nil
	default:
		return nil, errors.New("unexpected SMTP AUTH LOGIN challenge")
	}
}

func isOutlookServer(server string) bool {
	// 兼容多地区的outlook邮箱和ofb邮箱
	// 其实应该加一个Option来区分是否用LOGIN的方式登录
	// 先临时兼容一下
	return strings.Contains(server, "outlook") || strings.Contains(server, "onmicrosoft")
}
