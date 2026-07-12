package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// EmailPurpose 表示业务邮件场景，用于集中管理邮件标题和正文。
type EmailPurpose string

const (
	EmailPurposeRegister      EmailPurpose = "register"
	EmailPurposeLogin         EmailPurpose = "login"
	EmailPurposePasswordReset EmailPurpose = "password_reset"
)

// EmailTemplate 是发送邮件前生成的业务模板。
type EmailTemplate struct {
	Subject string
	Content string
}

// BuildVerificationCodeEmail 构建验证码类业务邮件。
func BuildVerificationCodeEmail(purpose EmailPurpose, code string) EmailTemplate {
	return EmailTemplate{
		Subject: buildBusinessEmailSubject(purpose),
		Content: fmt.Sprintf("<p>%s</p>"+
			"<p>您的验证码为: <strong>%s</strong></p>"+
			"<p>验证码 %d 分钟内有效，如果不是本人操作，请忽略。</p>",
			buildVerificationEmailGreeting(purpose), code, common.VerificationValidMinutes),
	}
}

// BuildPasswordResetLinkEmail 构建忘记密码重置链接邮件。
func BuildPasswordResetLinkEmail(link string) EmailTemplate {
	return EmailTemplate{
		Subject: buildBusinessEmailSubject(EmailPurposePasswordReset),
		Content: fmt.Sprintf("<p>您好，你正在进行忘记密码操作。</p>"+
			"<p>点击 <a href='%s'>此处</a> 进行密码重置。</p>"+
			"<p>如果链接无法点击，请尝试点击下面的链接或将其复制到浏览器中打开：<br> %s </p>"+
			"<p>重置链接 %d 分钟内有效，如果不是本人操作，请忽略。</p>",
			link, link, common.VerificationValidMinutes),
	}
}

func buildBusinessEmailSubject(purpose EmailPurpose) string {
	switch purpose {
	case EmailPurposeRegister:
		return "注册验证邮件"
	case EmailPurposeLogin:
		return "登录验证邮件"
	case EmailPurposePasswordReset:
		return "忘记密码验证邮件"
	default:
		return "系统通知邮件"
	}
}

func buildVerificationEmailGreeting(purpose EmailPurpose) string {
	switch purpose {
	case EmailPurposeRegister:
		return "您好，你正在进行邮箱验证。"
	case EmailPurposeLogin:
		return "您好，你正在进行登录验证。"
	case EmailPurposePasswordReset:
		return "您好，你正在进行忘记密码验证。"
	default:
		return "您好，你正在进行邮箱验证。"
	}
}
