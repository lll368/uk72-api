package common

import (
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatSMTPAddressHeaderEncodesFromName(t *testing.T) {
	header := formatSMTPAddressHeader("Example Sender", "sender@example.com")

	address, err := mail.ParseAddress(header)

	require.NoError(t, err)
	require.Equal(t, "Example Sender", address.Name)
	require.Equal(t, "sender@example.com", address.Address)
}

func TestShouldUseSMTPLoginAuthFor163Server(t *testing.T) {
	oldServer := SMTPServer
	oldAccount := SMTPAccount
	oldForceAuthLogin := SMTPForceAuthLogin
	t.Cleanup(func() {
		SMTPServer = oldServer
		SMTPAccount = oldAccount
		SMTPForceAuthLogin = oldForceAuthLogin
	})

	SMTPServer = "smtp.163.com"
	SMTPAccount = "sender@example.com"
	SMTPForceAuthLogin = false

	require.True(t, shouldUseSMTPLoginAuth())
}

func TestLoginAuthDoesNotSendEmptyInitialResponse(t *testing.T) {
	auth := LoginAuth("user@example.com", "secret")

	protocol, response, err := auth.Start(nil)

	require.NoError(t, err)
	require.Equal(t, "LOGIN", protocol)
	require.Nil(t, response)
}

func TestLoginAuthAcceptsCaseInsensitiveChallenges(t *testing.T) {
	auth := LoginAuth("user@example.com", "secret")
	_, _, err := auth.Start(nil)
	require.NoError(t, err)

	response, err := auth.Next([]byte("username:"), true)
	require.NoError(t, err)
	require.Equal(t, []byte("user@example.com"), response)

	response, err = auth.Next([]byte("PASSWORD:"), true)
	require.NoError(t, err)
	require.Equal(t, []byte("secret"), response)
}

func TestLoginAuthFallsBackToLoginSequenceForUnknownChallenges(t *testing.T) {
	auth := LoginAuth("user@example.com", "secret")
	_, _, err := auth.Start(nil)
	require.NoError(t, err)

	response, err := auth.Next([]byte("Account:"), true)
	require.NoError(t, err)
	require.Equal(t, []byte("user@example.com"), response)

	response, err = auth.Next([]byte("Credential:"), true)
	require.NoError(t, err)
	require.Equal(t, []byte("secret"), response)
}

func TestSplitSMTPReceiversTrimsEmptyItems(t *testing.T) {
	receivers := splitSMTPReceivers(" first@example.com ; ;second@example.com;")

	require.Equal(t, []string{"first@example.com", "second@example.com"}, receivers)
}

func TestSendEmailReturnsConfigurationErrorBeforeFromAddressValidation(t *testing.T) {
	oldServer := SMTPServer
	oldAccount := SMTPAccount
	oldFrom := SMTPFrom
	t.Cleanup(func() {
		SMTPServer = oldServer
		SMTPAccount = oldAccount
		SMTPFrom = oldFrom
	})

	SMTPServer = ""
	SMTPAccount = ""
	SMTPFrom = ""

	err := SendEmail("subject", "receiver@example.com", "<p>content</p>")

	require.EqualError(t, err, "SMTP 服务器或账户未配置")
}

func TestGenerateMessageIDParsesNamedFromAddress(t *testing.T) {
	oldFrom := SMTPFrom
	t.Cleanup(func() {
		SMTPFrom = oldFrom
	})

	SMTPFrom = "New API <noreply@example.com>"

	id, err := generateMessageID()

	require.NoError(t, err)
	require.True(t, strings.HasSuffix(id, "@example.com>"), id)
	require.NotContains(t, id, ">>")
}

func TestGenerateMessageIDRejectsInvalidFromAddress(t *testing.T) {
	oldFrom := SMTPFrom
	t.Cleanup(func() {
		SMTPFrom = oldFrom
	})

	SMTPFrom = "invalid-sender"

	_, err := generateMessageID()

	require.EqualError(t, err, "SMTP 发件地址无效")
}
