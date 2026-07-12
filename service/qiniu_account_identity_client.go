package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type QiniuAccountOperationType string

const (
	QiniuAccountOperationCreate           QiniuAccountOperationType = "create"
	QiniuAccountOperationQuota            QiniuAccountOperationType = "quota"
	QiniuAccountOperationHistoricalRevoke QiniuAccountOperationType = "historical_revoke"
	QiniuAccountOperationOfficialUsage    QiniuAccountOperationType = "official_usage"
	QiniuAccountOperationCostDetail       QiniuAccountOperationType = "cost_detail"
)

type QiniuAccountIdentityClientError struct {
	AccountId int
	Retryable bool
	Reason    string
}

func (err *QiniuAccountIdentityClientError) Error() string {
	reason := strings.TrimSpace(err.Reason)
	if reason == "" {
		reason = "账号身份不可用"
	}
	if err.AccountId <= 0 {
		return fmt.Sprintf("七牛父账号身份不可用：%s", reason)
	}
	return fmt.Sprintf("七牛子账号 %d 身份不可用：%s", err.AccountId, reason)
}

// NewQiniuAccountIdentityClient 根据 Token 归属账号创建七牛请求客户端。
func NewQiniuAccountIdentityClient(accountId int, operation QiniuAccountOperationType) (*qiniuKeyClient, error) {
	if accountId < 0 {
		return nil, &QiniuAccountIdentityClientError{Retryable: false, Reason: "账号 ID 无效"}
	}
	if !isValidQiniuAccountOperationType(operation) {
		return nil, &QiniuAccountIdentityClientError{AccountId: accountId, Retryable: false, Reason: "操作类型无效"}
	}
	if accountId == 0 {
		return newQiniuKeyClient(operation_setting.GetQiniuKeySetting())
	}
	account, err := model.GetQiniuChildAccountById(accountId)
	if err != nil {
		return nil, &QiniuAccountIdentityClientError{AccountId: accountId, Retryable: true, Reason: "子账号不存在或暂不可用"}
	}
	if err := validateQiniuChildAccountStatusForOperation(account, operation); err != nil {
		return nil, err
	}
	accessKey, secretKey, err := qiniuChildAccountSigningCredentials(account)
	if err != nil {
		return nil, &QiniuAccountIdentityClientError{AccountId: accountId, Retryable: true, Reason: err.Error()}
	}
	setting := *operation_setting.GetQiniuKeySetting()
	setting.Enabled = true
	setting.AccessKey = accessKey
	setting.SecretKey = secretKey
	return newQiniuKeyClient(&setting)
}

func validateQiniuChildAccountStatusForOperation(account *model.QiniuChildAccount, operation QiniuAccountOperationType) error {
	if account == nil {
		return &QiniuAccountIdentityClientError{Retryable: true, Reason: "子账号不存在或暂不可用"}
	}
	status := strings.TrimSpace(account.Status)
	if qiniuAccountOperationRequiresEnabled(operation) {
		if status != model.QiniuChildAccountStatusEnabled {
			return &QiniuAccountIdentityClientError{
				AccountId: account.Id,
				Retryable: true,
				Reason:    "子账号未启用，暂不能执行 Key 创建或额度操作",
			}
		}
		return nil
	}
	switch status {
	case model.QiniuChildAccountStatusEnabled, model.QiniuChildAccountStatusDisabled:
		return nil
	default:
		return &QiniuAccountIdentityClientError{
			AccountId: account.Id,
			Retryable: true,
			Reason:    "子账号尚不可用，无法执行历史操作",
		}
	}
}

func qiniuChildAccountSigningCredentials(account *model.QiniuChildAccount) (string, string, error) {
	if account == nil {
		return "", "", errors.New("子账号凭证不可用")
	}
	accessKey := strings.TrimSpace(account.AccessKey)
	protectedSecretKey := strings.TrimSpace(account.SecretKey)
	if accessKey == "" || protectedSecretKey == "" {
		return "", "", errors.New("子账号凭证缺失")
	}
	if strings.Contains(accessKey, "*") || strings.Contains(protectedSecretKey, "*") {
		return "", "", errors.New("子账号凭证不可解密，等待凭证同步后重试")
	}
	secretKey, err := model.RevealQiniuChildAccountSecret(protectedSecretKey)
	if err != nil {
		return "", "", errors.New("子账号凭证不可解密，等待凭证同步后重试")
	}
	return accessKey, secretKey, nil
}

func qiniuAccountOperationRequiresEnabled(operation QiniuAccountOperationType) bool {
	switch operation {
	case QiniuAccountOperationCreate, QiniuAccountOperationQuota:
		return true
	default:
		return false
	}
}

func isValidQiniuAccountOperationType(operation QiniuAccountOperationType) bool {
	switch operation {
	case QiniuAccountOperationCreate,
		QiniuAccountOperationQuota,
		QiniuAccountOperationHistoricalRevoke,
		QiniuAccountOperationOfficialUsage,
		QiniuAccountOperationCostDetail:
		return true
	default:
		return false
	}
}
