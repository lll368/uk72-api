package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitOptionMapKeepsEmailEnvironmentOverridesAfterDatabaseLoad(t *testing.T) {
	originalDB := DB
	originalOptionMap := common.OptionMap
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalSMTPServer := common.SMTPServer
	originalSMTPPort := common.SMTPPort
	originalSMTPSSLEnabled := common.SMTPSSLEnabled
	originalSMTPForceAuthLogin := common.SMTPForceAuthLogin
	originalSMTPAccount := common.SMTPAccount
	originalSMTPFrom := common.SMTPFrom
	originalSMTPFromName := common.SMTPFromName
	originalSMTPToken := common.SMTPToken
	originalSMTPTimeout := common.SMTPTimeout
	t.Cleanup(func() {
		DB = originalDB
		common.OptionMap = originalOptionMap
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.SMTPServer = originalSMTPServer
		common.SMTPPort = originalSMTPPort
		common.SMTPSSLEnabled = originalSMTPSSLEnabled
		common.SMTPForceAuthLogin = originalSMTPForceAuthLogin
		common.SMTPAccount = originalSMTPAccount
		common.SMTPFrom = originalSMTPFrom
		common.SMTPFromName = originalSMTPFromName
		common.SMTPToken = originalSMTPToken
		common.SMTPTimeout = originalSMTPTimeout
	})

	t.Setenv("EMAIL_VERIFICATION_ENABLED", "true")
	t.Setenv("SMTP_SERVER", "smtp.env.example")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_SSL_ENABLED", "true")
	t.Setenv("SMTP_FORCE_AUTH_LOGIN", "true")
	t.Setenv("SMTP_ACCOUNT", "env@example.com")
	t.Setenv("SMTP_FROM", "env@example.com")
	t.Setenv("SMTP_FROM_NAME", "Env Sender")
	t.Setenv("SMTP_TOKEN", "env-token")
	t.Setenv("SMTP_TIMEOUT_MS", "5000")

	db, err := gorm.Open(sqlite.Open("file:option_env_priority?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	require.NoError(t, db.Create([]Option{
		{Key: "EmailVerificationEnabled", Value: "false"},
		{Key: "SMTPServer", Value: "smtp.db.example"},
		{Key: "SMTPPort", Value: "2525"},
		{Key: "SMTPSSLEnabled", Value: "false"},
		{Key: "SMTPForceAuthLogin", Value: "false"},
		{Key: "SMTPAccount", Value: "db@example.com"},
		{Key: "SMTPFrom", Value: "db@example.com"},
		{Key: "SMTPFromName", Value: "DB Sender"},
		{Key: "SMTPToken", Value: "db-token"},
		{Key: "SMTPTimeoutMS", Value: "1000"},
	}).Error)

	DB = db
	common.ApplyEmailEnvOverrides()
	InitOptionMap()

	require.True(t, common.EmailVerificationEnabled)
	require.Equal(t, "smtp.env.example", common.SMTPServer)
	require.Equal(t, 465, common.SMTPPort)
	require.True(t, common.SMTPSSLEnabled)
	require.True(t, common.SMTPForceAuthLogin)
	require.Equal(t, "env@example.com", common.SMTPAccount)
	require.Equal(t, "env@example.com", common.SMTPFrom)
	require.Equal(t, "Env Sender", common.SMTPFromName)
	require.Equal(t, "env-token", common.SMTPToken)
	require.Equal(t, 5*time.Second, common.SMTPTimeout)
	require.Equal(t, "smtp.env.example", common.OptionMap["SMTPServer"])
	require.Equal(t, "465", common.OptionMap["SMTPPort"])
	require.Equal(t, "true", common.OptionMap["SMTPSSLEnabled"])
	require.Equal(t, "true", common.OptionMap["SMTPForceAuthLogin"])
	require.Equal(t, "env@example.com", common.OptionMap["SMTPAccount"])
	require.Equal(t, "env@example.com", common.OptionMap["SMTPFrom"])
	require.Equal(t, "Env Sender", common.OptionMap["SMTPFromName"])
	require.Equal(t, "5000", common.OptionMap["SMTPTimeoutMS"])
	require.Equal(t, "true", common.OptionMap["EmailEnvOverrideEnabled"])
}

func TestUpdateOptionKeepsEmailEnvironmentOverrides(t *testing.T) {
	originalDB := DB
	originalOptionMap := common.OptionMap
	originalSMTPServer := common.SMTPServer
	t.Cleanup(func() {
		DB = originalDB
		common.OptionMap = originalOptionMap
		common.SMTPServer = originalSMTPServer
	})

	t.Setenv("SMTP_SERVER", "smtp.env.example")

	db, err := gorm.Open(sqlite.Open("file:option_env_update_priority?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))

	DB = db
	common.OptionMap = make(map[string]string)
	common.ApplyEmailEnvOverrides()
	common.OptionMap["SMTPServer"] = common.SMTPServer

	require.NoError(t, UpdateOption("SMTPServer", "smtp.admin.example"))

	require.Equal(t, "smtp.env.example", common.SMTPServer)
	require.Equal(t, "smtp.env.example", common.OptionMap["SMTPServer"])
	require.Equal(t, "true", common.OptionMap["EmailEnvOverrideEnabled"])
}
