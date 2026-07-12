package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplyEmailEnvOverridesReadsEnvironment(t *testing.T) {
	originalEmailVerificationEnabled := EmailVerificationEnabled
	originalServer := SMTPServer
	originalPort := SMTPPort
	originalSSLEnabled := SMTPSSLEnabled
	originalForceAuthLogin := SMTPForceAuthLogin
	originalAccount := SMTPAccount
	originalFrom := SMTPFrom
	originalFromName := SMTPFromName
	originalToken := SMTPToken
	originalTimeout := SMTPTimeout
	t.Cleanup(func() {
		EmailVerificationEnabled = originalEmailVerificationEnabled
		SMTPServer = originalServer
		SMTPPort = originalPort
		SMTPSSLEnabled = originalSSLEnabled
		SMTPForceAuthLogin = originalForceAuthLogin
		SMTPAccount = originalAccount
		SMTPFrom = originalFrom
		SMTPFromName = originalFromName
		SMTPToken = originalToken
		SMTPTimeout = originalTimeout
	})

	EmailVerificationEnabled = false
	SMTPServer = ""
	SMTPPort = 587
	SMTPSSLEnabled = false
	SMTPForceAuthLogin = false
	SMTPAccount = ""
	SMTPFrom = ""
	SMTPFromName = ""
	SMTPToken = ""
	SMTPTimeout = 0

	t.Setenv("EMAIL_VERIFICATION_ENABLED", "true")
	t.Setenv("SMTP_SERVER", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_SSL_ENABLED", "true")
	t.Setenv("SMTP_FORCE_AUTH_LOGIN", "true")
	t.Setenv("SMTP_ACCOUNT", "mailer@example.com")
	t.Setenv("SMTP_FROM", "noreply@example.com")
	t.Setenv("SMTP_FROM_NAME", "Mail Sender")
	t.Setenv("SMTP_TOKEN", "secret-token")
	t.Setenv("SMTP_TIMEOUT_MS", "5000")
	t.Setenv("SMTP_TIMEOUT_SECONDS", "30")

	ApplyEmailEnvOverrides()

	require.True(t, EmailVerificationEnabled)
	require.Equal(t, "smtp.example.com", SMTPServer)
	require.Equal(t, 465, SMTPPort)
	require.True(t, SMTPSSLEnabled)
	require.True(t, SMTPForceAuthLogin)
	require.Equal(t, "mailer@example.com", SMTPAccount)
	require.Equal(t, "noreply@example.com", SMTPFrom)
	require.Equal(t, "Mail Sender", SMTPFromName)
	require.Equal(t, "secret-token", SMTPToken)
	require.Equal(t, 5*time.Second, SMTPTimeout)
}
