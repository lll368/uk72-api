package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestBuildVerificationCodeEmailByPurpose(t *testing.T) {
	oldSystemName := common.SystemName
	common.SystemName = "中国算力聚合网"
	t.Cleanup(func() {
		common.SystemName = oldSystemName
	})

	tests := []struct {
		name           string
		purpose        EmailPurpose
		wantSubject    string
		wantFirstLine  string
		wantCodeLine   string
		wantExpireLine string
	}{
		{
			name:           "register",
			purpose:        EmailPurposeRegister,
			wantSubject:    "注册验证邮件",
			wantFirstLine:  "您好，你正在进行邮箱验证。",
			wantCodeLine:   "您的验证码为: <strong>524759</strong>",
			wantExpireLine: fmt.Sprintf("验证码 %d 分钟内有效，如果不是本人操作，请忽略。", common.VerificationValidMinutes),
		},
		{
			name:           "login",
			purpose:        EmailPurposeLogin,
			wantSubject:    "登录验证邮件",
			wantFirstLine:  "您好，你正在进行登录验证。",
			wantCodeLine:   "您的验证码为: <strong>524759</strong>",
			wantExpireLine: fmt.Sprintf("验证码 %d 分钟内有效，如果不是本人操作，请忽略。", common.VerificationValidMinutes),
		},
		{
			name:           "password reset",
			purpose:        EmailPurposePasswordReset,
			wantSubject:    "忘记密码验证邮件",
			wantFirstLine:  "您好，你正在进行忘记密码验证。",
			wantCodeLine:   "您的验证码为: <strong>524759</strong>",
			wantExpireLine: fmt.Sprintf("验证码 %d 分钟内有效，如果不是本人操作，请忽略。", common.VerificationValidMinutes),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := BuildVerificationCodeEmail(tt.purpose, "524759")

			require.Equal(t, tt.wantSubject, template.Subject)
			require.Contains(t, template.Content, tt.wantFirstLine)
			require.Contains(t, template.Content, tt.wantCodeLine)
			require.Contains(t, template.Content, tt.wantExpireLine)
			require.False(t, strings.Contains(template.Subject, common.SystemName))
			require.False(t, strings.Contains(template.Content, common.SystemName))
			require.False(t, strings.Contains(template.Subject, "中国算力聚合网"))
			require.False(t, strings.Contains(template.Content, "中国算力聚合网"))
		})
	}
}

func TestBuildPasswordResetLinkEmailOmitsSystemName(t *testing.T) {
	oldSystemName := common.SystemName
	common.SystemName = "中国算力聚合网"
	t.Cleanup(func() {
		common.SystemName = oldSystemName
	})

	link := "https://example.com/user/reset?email=user@example.com&token=abc"
	template := BuildPasswordResetLinkEmail(link)

	require.Equal(t, "忘记密码验证邮件", template.Subject)
	require.Contains(t, template.Content, "您好，你正在进行忘记密码操作。")
	require.Contains(t, template.Content, link)
	require.Contains(t, template.Content, fmt.Sprintf("重置链接 %d 分钟内有效，如果不是本人操作，请忽略。", common.VerificationValidMinutes))
	require.False(t, strings.Contains(template.Subject, common.SystemName))
	require.False(t, strings.Contains(template.Content, common.SystemName))
	require.False(t, strings.Contains(template.Subject, "中国算力聚合网"))
	require.False(t, strings.Contains(template.Content, "中国算力聚合网"))
}
