package common

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"slices"
	"strings"
	"time"
)

type smtpFromIdentity struct {
	address     string
	displayName string
}

func resolveSMTPFromIdentity() (smtpFromIdentity, error) {
	rawAddress := strings.TrimSpace(SMTPFrom)
	if rawAddress == "" {
		rawAddress = strings.TrimSpace(SMTPAccount)
	}
	parsedAddress, err := mail.ParseAddress(rawAddress)
	if err != nil || parsedAddress == nil {
		return smtpFromIdentity{}, fmt.Errorf("SMTP 发件地址无效")
	}
	address := strings.TrimSpace(parsedAddress.Address)
	if Validate.Var(address, "required,email") != nil {
		return smtpFromIdentity{}, fmt.Errorf("SMTP 发件地址无效")
	}

	displayName := strings.TrimSpace(SMTPFromName)
	if displayName == "" {
		displayName = strings.TrimSpace(parsedAddress.Name)
	}
	if displayName == "" {
		displayName = SystemName
	}
	return smtpFromIdentity{
		address:     address,
		displayName: displayName,
	}, nil
}

func generateMessageID() (string, error) {
	from, err := resolveSMTPFromIdentity()
	if err != nil {
		return "", err
	}
	domain := strings.SplitN(from.address, "@", 2)[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	if shouldUseSMTPLoginAuth() {
		return LoginAuth(SMTPAccount, SMTPToken)
	}
	return smtp.PlainAuth("", SMTPAccount, SMTPToken, SMTPServer)
}

func formatSMTPAddressHeader(displayName string, address string) string {
	return (&mail.Address{
		Name:    strings.TrimSpace(displayName),
		Address: strings.TrimSpace(address),
	}).String()
}

func splitSMTPReceivers(receiver string) []string {
	parts := strings.Split(receiver, ";")
	receivers := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			receivers = append(receivers, trimmed)
		}
	}
	return receivers
}

func dialSMTP(addr string, useImplicitTLS bool) (net.Conn, error) {
	dialer := &net.Dialer{}
	if SMTPTimeout > 0 {
		dialer.Timeout = SMTPTimeout
	}
	if useImplicitTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         SMTPServer,
		}
		return tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	}
	return dialer.Dial("tcp", addr)
}

func sendSMTPMail(addr string, receiver string, from string, message []byte) error {
	useImplicitTLS := SMTPPort == 465 || SMTPSSLEnabled
	conn, err := dialSMTP(addr, useImplicitTLS)
	if err != nil {
		return err
	}
	if SMTPTimeout > 0 {
		// SMTPTimeout 同时约束连接、认证和写入过程，避免验证码接口长时间阻塞。
		if err = conn.SetDeadline(time.Now().Add(SMTPTimeout)); err != nil {
			_ = conn.Close()
			return err
		}
	}

	client, err := smtp.NewClient(conn, SMTPServer)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	if !useImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         SMTPServer,
			}
			if err = client.StartTLS(tlsConfig); err != nil {
				return err
			}
		}
	}
	if err = client.Auth(getSMTPAuth()); err != nil {
		return err
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	for _, receiverEmail := range splitSMTPReceivers(receiver) {
		if err = client.Rcpt(receiverEmail); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(message); err != nil {
		_ = w.Close()
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func SendEmail(subject string, receiver string, content string) error {
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	if SMTPServer == "" || SMTPAccount == "" {
		return fmt.Errorf("SMTP 服务器或账户未配置")
	}
	from, err := resolveSMTPFromIdentity()
	if err != nil {
		return err
	}
	id, err2 := generateMessageID()
	if err2 != nil {
		return err2
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	receivers := splitSMTPReceivers(receiver)
	if len(receivers) == 0 {
		return fmt.Errorf("邮件接收人未配置")
	}
	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+ // 添加 Message-ID 头
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		strings.Join(receivers, ", "), formatSMTPAddressHeader(from.displayName, from.address), encodedSubject, time.Now().Format(time.RFC1123Z), id, content))
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	err = sendSMTPMail(addr, receiver, from.address, message)
	if err != nil {
		SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}
