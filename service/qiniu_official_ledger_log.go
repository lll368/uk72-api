package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ensureQiniuOfficialLedgerLog(usageRecordId int) error {
	if usageRecordId <= 0 {
		return errors.New("官方 ledger 日志缺少 usage_record_id")
	}
	var app model.QiniuOfficialLedgerApplication
	if err := model.DB.Select("id").Where("usage_record_id = ? AND status = ?", usageRecordId, model.QiniuOfficialLedgerStatusSuccess).
		Order("apply_version desc").
		First(&app).Error; err != nil {
		return err
	}
	return ensureQiniuOfficialLedgerApplicationLog(app.Id)
}

func ensureQiniuOfficialLedgerApplicationLog(applicationId int) error {
	if applicationId <= 0 {
		return errors.New("官方 ledger 应用 ID 无效")
	}
	var app model.QiniuOfficialLedgerApplication
	query := model.DB
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Where("id = ? AND status = ?", applicationId, model.QiniuOfficialLedgerStatusSuccess).First(&app).Error; err != nil {
		return err
	}
	if app.ConsumeLogId > 0 || app.DeltaQuota == 0 {
		return nil
	}
	var record model.QiniuOfficialUsageRecord
	if err := model.DB.First(&record, "id = ?", app.UsageRecordId).Error; err != nil {
		return err
	}
	existingLogId, err := findQiniuOfficialLedgerLogId(&record, &app)
	if err != nil {
		return err
	}
	if existingLogId > 0 {
		return linkQiniuOfficialLedgerLogTx(model.DB, app.Id, existingLogId)
	}
	log := buildQiniuOfficialLedgerLog(&record, &app)
	if err := model.LOG_DB.Create(log).Error; err != nil {
		return err
	}
	return linkQiniuOfficialLedgerLogTx(model.DB, app.Id, log.Id)
}

func linkQiniuOfficialLedgerLogTx(tx *gorm.DB, applicationId int, logId int) error {
	return tx.Model(&model.QiniuOfficialLedgerApplication{}).
		Where("id = ? AND consume_log_id = 0", applicationId).
		Update("consume_log_id", logId).Error
}

func findQiniuOfficialLedgerLogId(record *model.QiniuOfficialUsageRecord, app *model.QiniuOfficialLedgerApplication) (int, error) {
	if record == nil || app == nil {
		return 0, nil
	}
	expectedLog := buildQiniuOfficialLedgerLog(record, app)
	var logs []model.Log
	if err := model.LOG_DB.Select("id", "other").
		Where("user_id = ? AND token_id = ? AND type = ? AND model_name = ? AND quota = ? AND other LIKE ?",
			expectedLog.UserId,
			expectedLog.TokenId,
			expectedLog.Type,
			expectedLog.ModelName,
			expectedLog.Quota,
			"%qiniu_official_ledger_id%",
		).
		Order("id asc").
		Limit(100).
		Find(&logs).Error; err != nil {
		return 0, err
	}
	for _, log := range logs {
		other, err := common.StrToMap(log.Other)
		if err != nil {
			continue
		}
		if qiniuOfficialLogIntEquals(other["qiniu_official_ledger_id"], app.Id) &&
			qiniuOfficialLogIntEquals(other["qiniu_official_usage_record_id"], app.UsageRecordId) &&
			qiniuOfficialLogIntEquals(other["apply_version"], app.ApplyVersion) {
			return log.Id, nil
		}
	}
	return 0, nil
}

func qiniuOfficialLogIntEquals(value interface{}, expected int) bool {
	switch typed := value.(type) {
	case int:
		return typed == expected
	case int64:
		return typed == int64(expected)
	case float64:
		return typed == float64(expected)
	case string:
		parsed, err := strconv.Atoi(typed)
		return err == nil && parsed == expected
	default:
		return false
	}
}

func RepairQiniuOfficialLedgerLogs(limit int) (int, error) {
	return 0, nil
}

func repairQiniuOfficialUsageRecordLatestLedgerLogIfMissing(usageRecordId int) (bool, error) {
	return false, nil
}

func repairQiniuOfficialLedgerApplicationLogIfMissing(applicationId int) (bool, error) {
	return false, nil
}

func buildQiniuOfficialLedgerLog(record *model.QiniuOfficialUsageRecord, app *model.QiniuOfficialLedgerApplication) *model.Log {
	logType := model.LogTypeConsume
	quota := app.DeltaQuota
	action := "消费"
	if app.DeltaQuota < 0 {
		logType = model.LogTypeRefund
		quota = -app.DeltaQuota
		action = "退款"
	}
	username, _ := model.GetUsernameById(app.UserId, false)
	tokenName := ""
	if app.TokenId > 0 {
		if token, err := model.GetTokenById(app.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	other := map[string]interface{}{
		"billing_source":                 qiniuOfficialLedgerBillingSource,
		"qiniu_official_ledger_log":      true,
		"qiniu_official_usage_record_id": record.Id,
		"qiniu_official_ledger_id":       app.Id,
		"qiniu_key":                      maskQiniuAPIKey(record.QiniuKey),
		"period_start":                   record.PeriodStart,
		"period_end":                     record.PeriodEnd,
		"billing_item":                   record.BillingItem,
		"fee_amount":                     record.FeeAmount,
		"currency":                       record.Currency,
		"official_quota":                 record.OfficialQuota,
		"applied_quota":                  record.AppliedQuota,
		"delta_quota":                    app.DeltaQuota,
		"apply_version":                  app.ApplyVersion,
		"source_api":                     record.SourceAPI,
	}
	return &model.Log{
		UserId:    app.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   fmt.Sprintf("官方用量同步%s：%s %s %s", action, formatQiniuLedgerPeriod(record.PeriodStart, record.PeriodEnd), record.ModelName, record.BillingItem),
		TokenName: tokenName,
		ModelName: record.ModelName,
		Quota:     quota,
		TokenId:   app.TokenId,
		Group:     "",
		Other:     common.MapToJsonStr(other),
	}
}

func formatQiniuLedgerPeriod(start int64, end int64) string {
	if start <= 0 || end <= 0 {
		return ""
	}
	startTime := time.Unix(start, 0).In(qiniuCSTLocation).Format("2006-01-02 15:04")
	endTime := time.Unix(end, 0).In(qiniuCSTLocation).Format("2006-01-02 15:04")
	return startTime + " - " + endTime
}

func qiniuOfficialLedgerLogExists(recordId int) (bool, error) {
	var app model.QiniuOfficialLedgerApplication
	if err := model.DB.Select("consume_log_id").Where("usage_record_id = ?", recordId).Order("apply_version desc").First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return app.ConsumeLogId > 0, nil
}
