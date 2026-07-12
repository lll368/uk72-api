package service

import (
	"context"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

type QiniuOfficialLedgerRetryResult struct {
	Applied       bool   `json:"applied"`
	Skipped       bool   `json:"skipped"`
	RepairedLog   bool   `json:"repaired_log"`
	UsageRecordId int    `json:"usage_record_id"`
	Message       string `json:"message,omitempty"`
}

func SanitizeQiniuOfficialAdminText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	sanitized := qiniuSensitiveKeyPattern.ReplaceAllString(text, "sk-********")
	setting := operation_setting.GetQiniuKeySetting()
	if setting != nil {
		sanitized = replaceQiniuSensitiveValue(sanitized, setting.AccessKey)
		sanitized = replaceQiniuSensitiveValue(sanitized, setting.SecretKey)
	}
	return sanitized
}

func RetryQiniuOfficialUsageRecord(ctx context.Context, recordId int) (*QiniuOfficialLedgerRetryResult, error) {
	if recordId <= 0 {
		return nil, errors.New("官方记录 ID 无效")
	}
	var record model.QiniuOfficialUsageRecord
	if err := model.DB.Select("id", "record_type", "status").Where("id = ?", recordId).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("官方记录不存在")
		}
		return nil, err
	}
	if record.RecordType != model.QiniuOfficialRecordTypeBill {
		return nil, errors.New("只有官方账单记录支持 ledger 重试")
	}
	return &QiniuOfficialLedgerRetryResult{
		Applied:       false,
		Skipped:       true,
		RepairedLog:   false,
		UsageRecordId: record.Id,
		Message:       qiniuOfficialLedgerAutoApplyDisabledMessage,
	}, nil
}

func RetryQiniuOfficialLedgerApplication(ctx context.Context, applicationId int) (*QiniuOfficialLedgerRetryResult, error) {
	if applicationId <= 0 {
		return nil, errors.New("官方 ledger 应用 ID 无效")
	}
	var application model.QiniuOfficialLedgerApplication
	if err := model.DB.Select("id", "usage_record_id", "status", "delta_quota", "consume_log_id").Where("id = ?", applicationId).First(&application).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("官方 ledger 应用记录不存在")
		}
		return nil, err
	}
	return &QiniuOfficialLedgerRetryResult{
		Applied:       false,
		Skipped:       true,
		RepairedLog:   false,
		UsageRecordId: application.UsageRecordId,
		Message:       qiniuOfficialLedgerAutoApplyDisabledMessage,
	}, nil
}

func replaceQiniuSensitiveValue(text string, value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 4 {
		return text
	}
	return strings.ReplaceAll(text, value, model.MaskTokenKey(value))
}
