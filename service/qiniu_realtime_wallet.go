package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	qiniuRealtimeWalletRepairTickInterval = 5 * time.Minute
	qiniuRealtimeWalletRepairBatchSize    = 50
)

var (
	qiniuRealtimeWalletRepairOnce    sync.Once
	qiniuRealtimeWalletRepairRunning atomic.Bool
)

type QiniuRealtimeWalletFlowInput struct {
	UserId            int
	TokenId           int
	RequestId         string
	BatchId           string
	ConsumeLogIds     []int
	Quota             int
	PreConsumedQuota  int
	SettlementApplied bool
	UsageStatsApplied bool
	UsageLogApplied   bool
	UsageApplied      bool
	ConsumeLogParams  *model.RecordConsumeLogParams
	LogUsername       string
	UpstreamRequestId string
	Remark            string
}

type QiniuRealtimeWalletRepairResult struct {
	ProcessedCount int
	AppliedCount   int
	FailedCount    int
	SkippedCount   int
	Errors         []string
}

// RecordConsumeLogWithQiniuRealtimeWalletFlow 记录消费日志，并为七牛实时市场价请求补齐最终钱包消费流水。
func RecordConsumeLogWithQiniuRealtimeWalletFlow(c *gin.Context, relayInfo *relaycommon.RelayInfo, params model.RecordConsumeLogParams, settlementApplied bool) *model.Log {
	return recordConsumeLogWithQiniuRealtimeWalletFlow(c, relayInfo, params, settlementApplied)
}

func recordConsumeLogWithQiniuRealtimeWalletFlow(c *gin.Context, relayInfo *relaycommon.RelayInfo, params model.RecordConsumeLogParams, settlementApplied bool) *model.Log {
	return recordConsumeLogWithQiniuRealtimeWalletFlowWithError(c, relayInfo, params, settlementApplied, nil)
}

func recordConsumeLogWithQiniuRealtimeWalletFlowWithError(c *gin.Context, relayInfo *relaycommon.RelayInfo, params model.RecordConsumeLogParams, settlementApplied bool, settlementErr error) *model.Log {
	if relayInfo == nil {
		log, _ := model.RecordConsumeLog(c, 0, params)
		return log
	}
	useQiniuRealtime := shouldUseQiniuMarketRealtimeFunding(relayInfo)
	if !useQiniuRealtime || params.Quota <= 0 {
		log, _ := model.RecordConsumeLog(c, relayInfo.UserId, params)
		return log
	}
	input := qiniuRealtimeWalletFlowInput(c, relayInfo, params, nil)
	if qiniuRealtimeWalletIdempotencyKey(input) == "" {
		// 没有 request/batch/log 幂等键时只能保留旧的本地观测语义，不能创建七牛实时钱包最终流水。
		if err := applyQiniuRealtimeUsageStatsDirect(relayInfo.UserId, params.ChannelId, params.Quota); err != nil {
			logger.LogError(c, "error applying qiniu realtime usage stats: "+err.Error())
			return nil
		}
		log, _ := model.RecordConsumeLog(c, relayInfo.UserId, params)
		return log
	}
	if useQiniuRealtime && params.Quota > 0 && !settlementApplied && !qiniuRealtimeBillingSessionSettled(relayInfo) {
		cause := settlementErr
		if cause == nil {
			cause = errors.New("七牛实时钱包静默结算未完成")
		}
		if err := recordQiniuRealtimeWalletFailure(input, cause); err != nil {
			logger.LogError(c, "error recording qiniu realtime wallet failure: "+err.Error())
		}
		return nil
	}
	input.SettlementApplied = true
	log, err := applyQiniuRealtimeWalletFlowWithUsageFacts(c, input)
	if err != nil {
		logger.LogError(c, "error applying qiniu realtime wallet flow: "+err.Error())
	}
	return log
}

func qiniuRealtimeWalletFlowInput(c *gin.Context, relayInfo *relaycommon.RelayInfo, params model.RecordConsumeLogParams, log *model.Log) QiniuRealtimeWalletFlowInput {
	requestId := ""
	if relayInfo != nil {
		requestId = strings.TrimSpace(relayInfo.RequestId)
	}
	if requestId == "" && c != nil {
		requestId = strings.TrimSpace(c.GetString(common.RequestIdKey))
	}
	if requestId == "" && log != nil {
		requestId = log.RequestId
	}
	consumeLogIds := make([]int, 0, 1)
	if log != nil && log.Id > 0 {
		consumeLogIds = append(consumeLogIds, log.Id)
	}
	input := QiniuRealtimeWalletFlowInput{
		RequestId:        requestId,
		ConsumeLogIds:    consumeLogIds,
		Quota:            params.Quota,
		ConsumeLogParams: &params,
	}
	if c != nil {
		input.LogUsername = c.GetString("username")
		input.UpstreamRequestId = c.GetString(common.UpstreamRequestIdKey)
	}
	if relayInfo != nil {
		input.UserId = relayInfo.UserId
		input.TokenId = relayInfo.TokenId
		if relayInfo.Billing != nil {
			input.PreConsumedQuota = relayInfo.Billing.GetPreConsumedQuota()
			input.SettlementApplied = qiniuRealtimeBillingSessionSettled(relayInfo)
		} else {
			input.SettlementApplied = true
		}
	}
	input.UsageLogApplied = (log != nil && log.Id > 0) || !common.LogConsumeEnabled
	input.UsageApplied = input.UsageStatsApplied && input.UsageLogApplied
	input.Remark = qiniuRealtimeWalletConsumptionRemark(relayInfo, requestId, params.ModelName)
	return input
}

func applyQiniuRealtimeUsageStatsDirect(userId int, channelId int, quota int) error {
	if quota <= 0 {
		return nil
	}
	return runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", userId).Updates(map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", 1),
		}).Error; err != nil {
			return err
		}
		if channelId > 0 {
			if err := tx.Model(&model.Channel{}).Where("id = ?", channelId).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func applyQiniuRealtimeWalletFlowWithUsageFacts(c *gin.Context, input QiniuRealtimeWalletFlowInput) (*model.Log, error) {
	application, alreadyApplied, err := reserveQiniuRealtimeWalletApplication(input)
	if err != nil {
		return nil, err
	}
	if alreadyApplied {
		return getQiniuRealtimeConsumeLogByApplication(application), nil
	}
	if !application.UsageStatsApplied {
		if err := restoreQiniuRealtimeUsageStats(application.Id, input.ConsumeLogParams); err != nil {
			_ = markQiniuRealtimeWalletApplicationFailed(application.Id, err)
			return nil, err
		}
	}

	current, err := loadQiniuRealtimeWalletApplication(application.Id)
	if err != nil {
		return nil, err
	}
	log := getQiniuRealtimeConsumeLogByApplication(current)
	var logErr error
	if !current.UsageLogApplied {
		log, logErr = ensureQiniuRealtimeOnlineConsumeLog(c, current, input.ConsumeLogParams)
		if logErr == nil {
			logID := 0
			if log != nil {
				logID = log.Id
			}
			if err := markQiniuRealtimeUsageLogApplied(current.Id, logID); err != nil {
				_ = markQiniuRealtimeWalletApplicationFailed(current.Id, err)
				return log, err
			}
		}
	}

	current, err = loadQiniuRealtimeWalletApplication(application.Id)
	if err != nil {
		return log, err
	}
	flowInput := qiniuRealtimeWalletFlowInputFromApplication(current, input.Remark, input.ConsumeLogParams)
	if _, err := applyQiniuRealtimeWalletFlow(flowInput); err != nil {
		_ = markQiniuRealtimeWalletApplicationFailed(current.Id, err)
		return log, err
	}
	if logErr != nil {
		_ = markQiniuRealtimeWalletApplicationFailed(current.Id, logErr)
		return log, logErr
	}
	return log, nil
}

func reserveQiniuRealtimeWalletApplication(input QiniuRealtimeWalletFlowInput) (*model.QiniuRealtimeWalletApplication, bool, error) {
	if err := validateQiniuRealtimeWalletFlowInput(input); err != nil {
		return nil, false, err
	}
	idempotencyKey := qiniuRealtimeWalletIdempotencyKey(input)
	var application model.QiniuRealtimeWalletApplication
	alreadyApplied := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		existing, exists, err := lockQiniuRealtimeWalletApplicationByKeyTx(tx, idempotencyKey)
		if err != nil {
			return err
		}
		if exists &&
			existing.Status == model.QiniuRealtimeWalletApplicationStatusApplied &&
			existing.WalletFlowId > 0 &&
			existing.SettlementApplied &&
			existing.UsageApplied {
			application = *existing
			alreadyApplied = true
			return nil
		}

		reserved := model.QiniuRealtimeWalletApplication{
			UserId:            input.UserId,
			TokenId:           input.TokenId,
			RequestId:         strings.TrimSpace(input.RequestId),
			BatchId:           strings.TrimSpace(input.BatchId),
			IdempotencyKey:    idempotencyKey,
			Quota:             input.Quota,
			PreConsumedQuota:  input.PreConsumedQuota,
			Amount:            quotaToWalletAmount(input.Quota),
			SettlementApplied: input.SettlementApplied,
			UsageStatsApplied: input.UsageStatsApplied,
			UsageLogApplied:   input.UsageLogApplied,
			UsageApplied:      input.UsageApplied || (input.UsageStatsApplied && input.UsageLogApplied),
			LogUsername:       strings.TrimSpace(input.LogUsername),
			UpstreamRequestId: strings.TrimSpace(input.UpstreamRequestId),
			Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
			LastError:         "七牛实时钱包应用待完成，等待在线流程或后台修复",
		}
		reserved.SetConsumeLogIds(input.ConsumeLogIds)
		if err := reserved.SetConsumeLogParams(input.ConsumeLogParams); err != nil {
			return err
		}
		if exists {
			if len(reserved.ConsumeLogIdList()) == 0 && len(existing.ConsumeLogIdList()) > 0 {
				reserved.ConsumeLogId = existing.ConsumeLogId
				reserved.ConsumeLogIds = existing.ConsumeLogIds
				reserved.CoveredLogCount = existing.CoveredLogCount
			}
			if reserved.ConsumeLogPayload == "" {
				reserved.ConsumeLogPayload = existing.ConsumeLogPayload
			}
			if reserved.LogUsername == "" {
				reserved.LogUsername = existing.LogUsername
			}
			if reserved.UpstreamRequestId == "" {
				reserved.UpstreamRequestId = existing.UpstreamRequestId
			}
			reserved.WalletFlowId = existing.WalletFlowId
			reserved.BalanceAfter = existing.BalanceAfter
			reserved.UsageStatsApplied = reserved.UsageStatsApplied || existing.UsageStatsApplied || existing.UsageApplied
			reserved.UsageLogApplied = reserved.UsageLogApplied || existing.UsageLogApplied || existing.UsageApplied
			reserved.UsageApplied = reserved.UsageApplied || existing.UsageApplied || (reserved.UsageStatsApplied && reserved.UsageLogApplied)
			reserved.RetryCount = existing.RetryCount + 1
			if err := tx.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
				"user_id":             reserved.UserId,
				"token_id":            reserved.TokenId,
				"request_id":          reserved.RequestId,
				"batch_id":            reserved.BatchId,
				"consume_log_id":      reserved.ConsumeLogId,
				"consume_log_ids":     reserved.ConsumeLogIds,
				"covered_log_count":   reserved.CoveredLogCount,
				"wallet_flow_id":      reserved.WalletFlowId,
				"quota":               reserved.Quota,
				"pre_consumed_quota":  reserved.PreConsumedQuota,
				"amount":              reserved.Amount,
				"balance_after":       reserved.BalanceAfter,
				"settlement_applied":  reserved.SettlementApplied,
				"usage_stats_applied": reserved.UsageStatsApplied,
				"usage_log_applied":   reserved.UsageLogApplied,
				"usage_applied":       reserved.UsageApplied,
				"consume_log_payload": reserved.ConsumeLogPayload,
				"log_username":        reserved.LogUsername,
				"upstream_request_id": reserved.UpstreamRequestId,
				"status":              reserved.Status,
				"last_error":          reserved.LastError,
				"retry_count":         reserved.RetryCount,
				"updated_time":        common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
			return tx.First(&application, "id = ?", existing.Id).Error
		}
		if err := tx.Create(&reserved).Error; err != nil {
			return err
		}
		application = reserved
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &application, alreadyApplied, nil
}

func loadQiniuRealtimeWalletApplication(applicationId int) (*model.QiniuRealtimeWalletApplication, error) {
	var application model.QiniuRealtimeWalletApplication
	if err := model.DB.First(&application, "id = ?", applicationId).Error; err != nil {
		return nil, err
	}
	return &application, nil
}

func getQiniuRealtimeConsumeLogByApplication(application *model.QiniuRealtimeWalletApplication) *model.Log {
	if application == nil {
		return nil
	}
	ids := application.ConsumeLogIdList()
	if len(ids) == 0 {
		return nil
	}
	var log model.Log
	if err := model.LOG_DB.First(&log, "id = ?", ids[0]).Error; err != nil {
		return nil
	}
	return &log
}

func ensureQiniuRealtimeOnlineConsumeLog(c *gin.Context, application *model.QiniuRealtimeWalletApplication, params *model.RecordConsumeLogParams) (*model.Log, error) {
	if application == nil || params == nil || !common.LogConsumeEnabled {
		return nil, nil
	}
	if log := getQiniuRealtimeConsumeLogByApplication(application); log != nil {
		return log, nil
	}
	var existing model.Log
	query := model.LOG_DB.Where("user_id = ? AND token_id = ? AND type = ? AND quota = ?", application.UserId, application.TokenId, model.LogTypeConsume, application.Quota)
	if requestId := strings.TrimSpace(application.RequestId); requestId != "" {
		query = query.Where("request_id = ?", requestId)
	}
	err := query.Order("id asc").First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if c == nil {
		logID, err := ensureQiniuRealtimeReplayConsumeLog(qiniuRealtimeReplayLogDB(nil), application, params)
		if err != nil || logID <= 0 {
			return nil, err
		}
		var log model.Log
		if err := model.LOG_DB.First(&log, "id = ?", logID).Error; err != nil {
			return nil, err
		}
		return &log, nil
	}
	return model.RecordConsumeLog(c, application.UserId, *params)
}

func qiniuRealtimeWalletFlowInputFromApplication(application *model.QiniuRealtimeWalletApplication, remark string, consumeLogParams *model.RecordConsumeLogParams) QiniuRealtimeWalletFlowInput {
	if application == nil {
		return QiniuRealtimeWalletFlowInput{}
	}
	return QiniuRealtimeWalletFlowInput{
		UserId:            application.UserId,
		TokenId:           application.TokenId,
		RequestId:         application.RequestId,
		BatchId:           application.BatchId,
		ConsumeLogIds:     application.ConsumeLogIdList(),
		Quota:             application.Quota,
		PreConsumedQuota:  application.PreConsumedQuota,
		SettlementApplied: application.SettlementApplied,
		UsageStatsApplied: application.UsageStatsApplied,
		UsageLogApplied:   application.UsageLogApplied,
		UsageApplied:      application.UsageApplied,
		ConsumeLogParams:  consumeLogParams,
		LogUsername:       application.LogUsername,
		UpstreamRequestId: application.UpstreamRequestId,
		Remark:            remark,
	}
}

func qiniuRealtimeBillingSessionSettled(relayInfo *relaycommon.RelayInfo) bool {
	if !shouldUseQiniuMarketRealtimeFunding(relayInfo) || relayInfo.Billing == nil {
		return false
	}
	session, ok := relayInfo.Billing.(*BillingSession)
	if !ok {
		return false
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.settled && !session.refunded
}

func ApplyQiniuRealtimeWalletFlow(input QiniuRealtimeWalletFlowInput) (*model.QiniuRealtimeWalletApplication, error) {
	app, err := applyQiniuRealtimeWalletFlow(input)
	if err != nil {
		_ = recordQiniuRealtimeWalletFailure(input, err)
		return nil, err
	}
	return app, nil
}

func RepairQiniuRealtimeWalletApplication(applicationId int) (*model.QiniuRealtimeWalletApplication, error) {
	if applicationId <= 0 {
		return nil, errors.New("七牛实时钱包应用记录无效")
	}
	var existing model.QiniuRealtimeWalletApplication
	if err := model.DB.First(&existing, "id = ?", applicationId).Error; err != nil {
		return nil, err
	}
	if !existing.SettlementApplied {
		if err := retryQiniuRealtimeWalletSettlement(&existing); err != nil {
			_ = markQiniuRealtimeWalletApplicationFailed(existing.Id, err)
			return nil, err
		}
		if err := model.DB.First(&existing, "id = ?", applicationId).Error; err != nil {
			return nil, err
		}
	}
	if !existing.UsageApplied {
		if err := restoreQiniuRealtimeUsageFacts(&existing); err != nil {
			_ = markQiniuRealtimeWalletApplicationFailed(existing.Id, err)
			return nil, err
		}
		if err := model.DB.First(&existing, "id = ?", applicationId).Error; err != nil {
			return nil, err
		}
	}
	input := QiniuRealtimeWalletFlowInput{
		UserId:            existing.UserId,
		TokenId:           existing.TokenId,
		RequestId:         existing.RequestId,
		BatchId:           existing.BatchId,
		ConsumeLogIds:     existing.ConsumeLogIdList(),
		Quota:             existing.Quota,
		PreConsumedQuota:  existing.PreConsumedQuota,
		SettlementApplied: existing.SettlementApplied,
		UsageStatsApplied: existing.UsageStatsApplied,
		UsageLogApplied:   existing.UsageLogApplied,
		UsageApplied:      existing.UsageApplied,
		Remark:            qiniuRealtimeWalletRemark(&existing),
	}
	app, err := applyQiniuRealtimeWalletFlow(input)
	if err != nil {
		_ = markQiniuRealtimeWalletApplicationFailed(existing.Id, err)
		return nil, err
	}
	if app != nil && app.Id > 0 {
		if err := markQiniuRealtimeTaskSettlementApplied(app.Id, app.Quota); err != nil {
			_ = markQiniuRealtimeWalletApplicationFailed(app.Id, err)
			return nil, err
		}
		if err := model.DB.First(app, "id = ?", app.Id).Error; err != nil {
			return nil, err
		}
	}
	return app, nil
}

func retryQiniuRealtimeWalletSettlement(application *model.QiniuRealtimeWalletApplication) error {
	if application == nil {
		return errors.New("七牛实时钱包应用记录无效")
	}
	appliedDelta := 0
	appliedTokenId := 0
	err := runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		var current model.QiniuRealtimeWalletApplication
		query := tx.Where("id = ?", application.Id)
		if !common.UsingSQLite {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&current).Error; err != nil {
			return err
		}
		if current.SettlementApplied {
			return nil
		}
		delta := current.Quota - current.PreConsumedQuota
		if delta > 0 {
			if err := debitBalanceByQuotaSilentlyAllowDebtTx(tx, current.UserId, delta); err != nil {
				return err
			}
		} else if delta < 0 {
			if err := creditBalanceByQuotaSilentlyTx(tx, current.UserId, -delta); err != nil {
				return err
			}
		}
		account, err := getOrCreateWalletAccountTx(tx, current.UserId, true)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", current.Id).Updates(map[string]interface{}{
			"settlement_applied": true,
			"balance_after":      account.BalanceAmount,
			"last_error":         "",
			"retry_count":        current.RetryCount + 1,
			"updated_time":       common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		appliedDelta = delta
		appliedTokenId = current.TokenId
		return nil
	})
	if err != nil {
		return err
	}
	if appliedDelta != 0 && appliedTokenId > 0 {
		if token, err := model.GetTokenById(appliedTokenId); err == nil {
			if appliedDelta > 0 {
				err = model.DecreaseTokenQuota(appliedTokenId, token.Key, appliedDelta)
			} else {
				err = model.IncreaseTokenQuota(appliedTokenId, token.Key, -appliedDelta)
			}
			if err != nil {
				common.SysLog(fmt.Sprintf("error adjusting token quota when repairing qiniu realtime settlement (applicationId=%d, tokenId=%d, delta=%d): %s",
					application.Id, appliedTokenId, appliedDelta, err.Error()))
			}
		} else {
			common.SysLog(fmt.Sprintf("error loading token when repairing qiniu realtime settlement (applicationId=%d, tokenId=%d, delta=%d): %s",
				application.Id, appliedTokenId, appliedDelta, err.Error()))
		}
	}
	return nil
}

func restoreQiniuRealtimeUsageFacts(application *model.QiniuRealtimeWalletApplication) error {
	if application == nil {
		return errors.New("七牛实时钱包应用记录无效")
	}
	if application.UsageApplied {
		return nil
	}
	params, err := application.ConsumeLogParamsValue()
	if err != nil {
		return err
	}
	if err := restoreQiniuRealtimeUsageStats(application.Id, params); err != nil {
		return err
	}
	return restoreQiniuRealtimeUsageLog(application.Id, params)
}

func restoreQiniuRealtimeUsageStats(applicationId int, params *model.RecordConsumeLogParams) error {
	return runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		var current model.QiniuRealtimeWalletApplication
		query := tx.Where("id = ?", applicationId)
		if !common.UsingSQLite {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&current).Error; err != nil {
			return err
		}
		if current.UsageApplied {
			return nil
		}
		if current.UsageStatsApplied {
			if current.UsageLogApplied {
				return markQiniuRealtimeUsageAppliedTx(tx, current.Id)
			}
			return nil
		}

		usageLogApplied := current.UsageLogApplied || params == nil || !common.LogConsumeEnabled
		updates := map[string]interface{}{
			"usage_stats_applied": true,
			"last_error":          "",
			"updated_time":        common.GetTimestamp(),
		}
		if usageLogApplied {
			updates["usage_log_applied"] = true
			updates["usage_applied"] = true
		}
		if params != nil && current.Quota > 0 {
			// settlement 已经完成后再恢复统计；使用 usage_stats_applied 分阶段幂等保护，避免后台重试重复累计。
			if err := tx.Model(&model.User{}).Where("id = ?", current.UserId).Updates(map[string]interface{}{
				"used_quota":    gorm.Expr("used_quota + ?", current.Quota),
				"request_count": gorm.Expr("request_count + ?", 1),
			}).Error; err != nil {
				return err
			}
			if params.ChannelId > 0 {
				if err := tx.Model(&model.Channel{}).Where("id = ?", params.ChannelId).Update("used_quota", gorm.Expr("used_quota + ?", current.Quota)).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", current.Id).Updates(updates).Error
	})
}

func restoreQiniuRealtimeUsageLog(applicationId int, params *model.RecordConsumeLogParams) error {
	if params == nil || !common.LogConsumeEnabled {
		return markQiniuRealtimeUsageLogApplied(applicationId, 0)
	}
	var current model.QiniuRealtimeWalletApplication
	if err := model.DB.First(&current, "id = ?", applicationId).Error; err != nil {
		return err
	}
	if current.UsageApplied || current.UsageLogApplied {
		if current.UsageStatsApplied && !current.UsageApplied {
			return markQiniuRealtimeUsageLogApplied(applicationId, 0)
		}
		return nil
	}
	if !current.UsageStatsApplied {
		return errors.New("七牛实时使用统计尚未恢复")
	}
	logID, err := ensureQiniuRealtimeReplayConsumeLog(qiniuRealtimeReplayLogDB(nil), &current, params)
	if err != nil {
		return err
	}
	return markQiniuRealtimeUsageLogApplied(applicationId, logID)
}

func markQiniuRealtimeUsageLogApplied(applicationId int, logID int) error {
	return runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		var current model.QiniuRealtimeWalletApplication
		query := tx.Where("id = ?", applicationId)
		if !common.UsingSQLite {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&current).Error; err != nil {
			return err
		}
		if current.UsageApplied {
			return nil
		}
		if !current.UsageStatsApplied {
			return errors.New("七牛实时使用统计尚未恢复")
		}
		if current.UsageLogApplied {
			return markQiniuRealtimeUsageAppliedTx(tx, current.Id)
		}
		updates := map[string]interface{}{
			"usage_log_applied": true,
			"usage_applied":     true,
			"last_error":        "",
			"updated_time":      common.GetTimestamp(),
		}
		if logID > 0 && len(current.ConsumeLogIdList()) == 0 {
			current.SetConsumeLogIds([]int{logID})
			updates["consume_log_id"] = current.ConsumeLogId
			updates["consume_log_ids"] = current.ConsumeLogIds
			updates["covered_log_count"] = current.CoveredLogCount
		}
		return tx.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", current.Id).Updates(updates).Error
	})
}

func markQiniuRealtimeUsageAppliedTx(tx *gorm.DB, applicationId int) error {
	return tx.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", applicationId).Updates(map[string]interface{}{
		"usage_applied": true,
		"last_error":    "",
		"updated_time":  common.GetTimestamp(),
	}).Error
}

func qiniuRealtimeReplayLogDB(tx *gorm.DB) *gorm.DB {
	if model.LOG_DB != nil && model.LOG_DB != model.DB {
		return model.LOG_DB
	}
	if tx != nil {
		return tx
	}
	return model.DB
}

func ensureQiniuRealtimeReplayConsumeLog(logDB *gorm.DB, application *model.QiniuRealtimeWalletApplication, params *model.RecordConsumeLogParams) (int, error) {
	if application == nil || params == nil {
		return 0, nil
	}
	if ids := application.ConsumeLogIdList(); len(ids) > 0 {
		return ids[0], nil
	}
	if logDB == nil {
		logDB = model.DB
	}
	var existing model.Log
	query := logDB.Where("user_id = ? AND token_id = ? AND type = ? AND quota = ?", application.UserId, application.TokenId, model.LogTypeConsume, application.Quota)
	if requestId := strings.TrimSpace(application.RequestId); requestId != "" {
		query = query.Where("request_id = ?", requestId)
	}
	err := query.Order("id asc").First(&existing).Error
	if err == nil {
		return existing.Id, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	createdAt := application.CreatedTime
	if createdAt == 0 {
		createdAt = common.GetTimestamp()
	}
	log := &model.Log{
		UserId:            application.UserId,
		Username:          strings.TrimSpace(application.LogUsername),
		CreatedAt:         createdAt,
		Type:              model.LogTypeConsume,
		Content:           params.Content,
		PromptTokens:      params.PromptTokens,
		CompletionTokens:  params.CompletionTokens,
		TokenName:         params.TokenName,
		ModelName:         params.ModelName,
		Quota:             application.Quota,
		ChannelId:         params.ChannelId,
		TokenId:           application.TokenId,
		UseTime:           params.UseTimeSeconds,
		IsStream:          params.IsStream,
		Group:             params.Group,
		RequestId:         strings.TrimSpace(application.RequestId),
		UpstreamRequestId: strings.TrimSpace(application.UpstreamRequestId),
		Other:             common.MapToJsonStr(params.Other),
	}
	if err := logDB.Create(log).Error; err != nil {
		return 0, err
	}
	if common.DataExportEnabled {
		username := log.Username
		modelName := log.ModelName
		quota := log.Quota
		tokenCount := log.PromptTokens + log.CompletionTokens
		userId := log.UserId
		gopool.Go(func() {
			model.LogQuotaData(userId, username, modelName, quota, createdAt, tokenCount)
		})
	}
	return log.Id, nil
}

func StartQiniuRealtimeWalletRepairTask() {
	qiniuRealtimeWalletRepairOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog(fmt.Sprintf("qiniu realtime wallet repair task started: tick=%s", qiniuRealtimeWalletRepairTickInterval))
			ticker := time.NewTicker(qiniuRealtimeWalletRepairTickInterval)
			defer ticker.Stop()
			runQiniuRealtimeWalletRepairOnce()
			for range ticker.C {
				runQiniuRealtimeWalletRepairOnce()
			}
		})
	})
}

func runQiniuRealtimeWalletRepairOnce() {
	if !qiniuRealtimeWalletRepairRunning.CompareAndSwap(false, true) {
		return
	}
	defer qiniuRealtimeWalletRepairRunning.Store(false)
	result, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), qiniuRealtimeWalletRepairBatchSize)
	if err != nil {
		common.SysLog("qiniu realtime wallet repair scan failed: " + sanitizeQiniuTaskError(err))
		return
	}
	if result.ProcessedCount > 0 || len(result.Errors) > 0 {
		common.SysLog(fmt.Sprintf(
			"qiniu realtime wallet repair scan finished processed=%d applied=%d skipped=%d failed=%d errors=%d",
			result.ProcessedCount,
			result.AppliedCount,
			result.SkippedCount,
			result.FailedCount,
			len(result.Errors),
		))
	}
}

func ScanFailedQiniuRealtimeWalletApplications(ctx context.Context, limit int) (*QiniuRealtimeWalletRepairResult, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if limit <= 0 || limit > qiniuRealtimeWalletRepairBatchSize {
		limit = qiniuRealtimeWalletRepairBatchSize
	}
	result := &QiniuRealtimeWalletRepairResult{Errors: make([]string, 0)}
	var applications []model.QiniuRealtimeWalletApplication
	// pending/failed 都是未完成应用；status=applied 但 usage/settlement 未完成属于异常半应用状态，也必须继续修复。
	if err := model.DB.Where(
		"status IN ? OR (status = ? AND (usage_applied = ? OR settlement_applied = ?))",
		[]string{
			model.QiniuRealtimeWalletApplicationStatusFailed,
			model.QiniuRealtimeWalletApplicationStatusPending,
		},
		model.QiniuRealtimeWalletApplicationStatusApplied,
		false,
		false,
	).Order("id asc").Limit(limit).Find(&applications).Error; err != nil {
		return nil, err
	}
	for _, application := range applications {
		if ctx != nil && ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.ProcessedCount++
		repaired, err := RepairQiniuRealtimeWalletApplication(application.Id)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
			continue
		}
		if repaired != nil && repaired.Status == model.QiniuRealtimeWalletApplicationStatusApplied {
			result.AppliedCount++
		} else {
			result.SkippedCount++
		}
	}
	return result, nil
}

func BuildQiniuRealtimeTaskSettlement(relayInfo *relaycommon.RelayInfo, actualQuota int, settlementApplied bool) *model.QiniuRealtimeTaskSettlement {
	if !shouldUseQiniuMarketRealtimeFunding(relayInfo) {
		return nil
	}
	preConsumed := relayInfo.FinalPreConsumedQuota
	if relayInfo.Billing != nil {
		preConsumed = relayInfo.Billing.GetPreConsumedQuota()
	}
	applicationId := 0
	idempotencyKey := qiniuRealtimeWalletIdempotencyKey(QiniuRealtimeWalletFlowInput{RequestId: relayInfo.RequestId})
	if idempotencyKey != "" {
		var application model.QiniuRealtimeWalletApplication
		if err := model.DB.Select("id").Where("idempotency_key = ?", idempotencyKey).First(&application).Error; err == nil {
			applicationId = application.Id
		}
	}
	return &model.QiniuRealtimeTaskSettlement{
		ApplicationId:     applicationId,
		PreConsumedQuota:  preConsumed,
		ActualQuota:       actualQuota,
		SettlementApplied: settlementApplied,
	}
}

func markQiniuRealtimeTaskSettlementApplied(applicationId int, actualQuota int) error {
	if applicationId <= 0 {
		return nil
	}
	var tasks []model.Task
	if err := model.DB.Where("private_data LIKE ?", fmt.Sprintf("%%\"application_id\":%d%%", applicationId)).Find(&tasks).Error; err != nil {
		return err
	}
	for _, task := range tasks {
		settlement := task.PrivateData.QiniuRealtimeSettlement
		if settlement == nil || settlement.ApplicationId != applicationId {
			continue
		}
		settlement.SettlementApplied = true
		if actualQuota > 0 {
			settlement.ActualQuota = actualQuota
			task.Quota = actualQuota
		}
		if err := model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"quota":        task.Quota,
			"private_data": task.PrivateData,
			"updated_at":   common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyQiniuRealtimeWalletFlow(input QiniuRealtimeWalletFlowInput) (*model.QiniuRealtimeWalletApplication, error) {
	if err := validateQiniuRealtimeWalletFlowInput(input); err != nil {
		return nil, err
	}
	idempotencyKey := qiniuRealtimeWalletIdempotencyKey(input)
	var applied model.QiniuRealtimeWalletApplication
	err := runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		existing, exists, err := lockQiniuRealtimeWalletApplicationByKeyTx(tx, idempotencyKey)
		if err != nil {
			return err
		}
		if exists &&
			existing.Status == model.QiniuRealtimeWalletApplicationStatusApplied &&
			existing.WalletFlowId > 0 &&
			existing.SettlementApplied &&
			existing.UsageApplied {
			applied = *existing
			return nil
		}

		usageStatsApplied, usageLogApplied, usageApplied := qiniuRealtimeUsageStateFromInput(input)
		application := model.QiniuRealtimeWalletApplication{
			UserId:            input.UserId,
			TokenId:           input.TokenId,
			RequestId:         strings.TrimSpace(input.RequestId),
			BatchId:           strings.TrimSpace(input.BatchId),
			IdempotencyKey:    idempotencyKey,
			Quota:             input.Quota,
			PreConsumedQuota:  input.PreConsumedQuota,
			Amount:            quotaToWalletAmount(input.Quota),
			SettlementApplied: true,
			UsageStatsApplied: usageStatsApplied,
			UsageLogApplied:   usageLogApplied,
			UsageApplied:      usageApplied,
			LogUsername:       strings.TrimSpace(input.LogUsername),
			UpstreamRequestId: strings.TrimSpace(input.UpstreamRequestId),
			Status:            model.QiniuRealtimeWalletApplicationStatusPending,
		}
		application.SetConsumeLogIds(input.ConsumeLogIds)
		if err := application.SetConsumeLogParams(input.ConsumeLogParams); err != nil {
			return err
		}
		if exists {
			application.Id = existing.Id
			application.RetryCount = existing.RetryCount
			if application.ConsumeLogPayload == "" {
				application.ConsumeLogPayload = existing.ConsumeLogPayload
			}
			if application.LogUsername == "" {
				application.LogUsername = existing.LogUsername
			}
			if application.UpstreamRequestId == "" {
				application.UpstreamRequestId = existing.UpstreamRequestId
			}
			if len(application.ConsumeLogIdList()) == 0 && len(existing.ConsumeLogIdList()) > 0 {
				application.ConsumeLogId = existing.ConsumeLogId
				application.ConsumeLogIds = existing.ConsumeLogIds
				application.CoveredLogCount = existing.CoveredLogCount
			}
			application.UsageStatsApplied = application.UsageStatsApplied || existing.UsageStatsApplied || existing.UsageApplied
			application.UsageLogApplied = application.UsageLogApplied || existing.UsageLogApplied || existing.UsageApplied
			application.UsageApplied = application.UsageApplied || existing.UsageApplied || (application.UsageStatsApplied && application.UsageLogApplied)
			if existing.Status == model.QiniuRealtimeWalletApplicationStatusFailed || existing.Status == model.QiniuRealtimeWalletApplicationStatusPending {
				application.RetryCount++
			}
		}

		account, err := getOrCreateWalletAccountTx(tx, input.UserId, true)
		if err != nil {
			return err
		}
		application.BalanceAfter = account.BalanceAmount

		walletFlowId := 0
		flowExists, err := walletFlowExistsTx(tx, idempotencyKey)
		if err != nil {
			return err
		}
		if flowExists {
			walletFlowId, err = getWalletFlowIdByIdempotencyTx(tx, idempotencyKey)
			if err != nil {
				return err
			}
		} else {
			flow := &model.WalletFlow{
				UserId:                input.UserId,
				BizNo:                 idempotencyKey,
				IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
				FlowType:              model.WalletFlowTypeBalanceConsume,
				WalletType:            model.WalletTypeBalance,
				Direction:             model.WalletFlowDirectionOut,
				Amount:                application.Amount,
				BalanceAfter:          account.BalanceAmount,
				CommissionAfter:       account.CommissionAmount,
				FrozenCommissionAfter: account.FrozenCommissionAmount,
				Remark:                qiniuRealtimeWalletFlowRemark(input),
			}
			if err := createWalletFlowTx(tx, flow); err != nil {
				return err
			}
			walletFlowId, err = getWalletFlowIdByIdempotencyTx(tx, idempotencyKey)
			if err != nil {
				return err
			}
		}

		application.WalletFlowId = walletFlowId
		if application.SettlementApplied && application.UsageApplied {
			application.Status = model.QiniuRealtimeWalletApplicationStatusApplied
			application.LastError = ""
		} else {
			application.Status = model.QiniuRealtimeWalletApplicationStatusFailed
			if application.LastError == "" {
				application.LastError = "七牛实时钱包应用未完整完成，等待后台修复"
			}
		}
		if exists {
			if err := tx.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
				"user_id":             application.UserId,
				"token_id":            application.TokenId,
				"request_id":          application.RequestId,
				"batch_id":            application.BatchId,
				"consume_log_id":      application.ConsumeLogId,
				"consume_log_ids":     application.ConsumeLogIds,
				"covered_log_count":   application.CoveredLogCount,
				"wallet_flow_id":      application.WalletFlowId,
				"quota":               application.Quota,
				"pre_consumed_quota":  application.PreConsumedQuota,
				"amount":              application.Amount,
				"balance_after":       application.BalanceAfter,
				"settlement_applied":  application.SettlementApplied,
				"usage_stats_applied": application.UsageStatsApplied,
				"usage_log_applied":   application.UsageLogApplied,
				"usage_applied":       application.UsageApplied,
				"consume_log_payload": application.ConsumeLogPayload,
				"log_username":        application.LogUsername,
				"upstream_request_id": application.UpstreamRequestId,
				"status":              application.Status,
				"retry_count":         application.RetryCount,
				"last_error":          application.LastError,
				"updated_time":        common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
			if err := tx.First(&applied, "id = ?", existing.Id).Error; err != nil {
				return err
			}
			return nil
		}
		if err := tx.Create(&application).Error; err != nil {
			return err
		}
		applied = application
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &applied, nil
}

func qiniuRealtimeUsageStateFromInput(input QiniuRealtimeWalletFlowInput) (bool, bool, bool) {
	if input.UsageApplied {
		return true, true, true
	}
	usageStatsApplied := input.UsageStatsApplied
	usageLogApplied := input.UsageLogApplied
	if len(input.ConsumeLogIds) > 0 {
		usageStatsApplied = true
		usageLogApplied = true
	}
	if input.ConsumeLogParams == nil || !common.LogConsumeEnabled {
		usageLogApplied = true
		if !input.UsageStatsApplied && !input.UsageLogApplied {
			usageStatsApplied = true
		}
	}
	return usageStatsApplied, usageLogApplied, usageStatsApplied && usageLogApplied
}

func validateQiniuRealtimeWalletFlowInput(input QiniuRealtimeWalletFlowInput) error {
	if input.UserId <= 0 {
		return errors.New("用户不存在")
	}
	if input.TokenId <= 0 {
		return errors.New("Token 不存在")
	}
	if input.Quota <= 0 {
		return errors.New("七牛实时消费 quota 必须大于 0")
	}
	if qiniuRealtimeWalletIdempotencyKey(input) == "" {
		return errors.New("七牛实时钱包流水缺少幂等键")
	}
	return nil
}

func qiniuRealtimeWalletIdempotencyKey(input QiniuRealtimeWalletFlowInput) string {
	if batchId := strings.TrimSpace(input.BatchId); batchId != "" {
		return "qiniu:realtime:batch:" + batchId
	}
	if requestId := strings.TrimSpace(input.RequestId); requestId != "" {
		return "qiniu:realtime:request:" + requestId
	}
	if len(input.ConsumeLogIds) == 1 && input.ConsumeLogIds[0] > 0 {
		return fmt.Sprintf("qiniu:realtime:log:%d", input.ConsumeLogIds[0])
	}
	return ""
}

func lockQiniuRealtimeWalletApplicationByKeyTx(tx *gorm.DB, idempotencyKey string) (*model.QiniuRealtimeWalletApplication, bool, error) {
	var application model.QiniuRealtimeWalletApplication
	query := tx.Where("idempotency_key = ?", idempotencyKey)
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &application, true, nil
}

func qiniuRealtimeWalletFlowRemark(input QiniuRealtimeWalletFlowInput) string {
	if remark := strings.TrimSpace(input.Remark); remark != "" {
		return remark
	}
	parts := []string{"市场价实时 token/model 消费"}
	if requestId := strings.TrimSpace(input.RequestId); requestId != "" {
		parts = append(parts, "request="+requestId)
	}
	if batchId := strings.TrimSpace(input.BatchId); batchId != "" {
		parts = append(parts, "batch="+batchId)
	}
	return strings.Join(parts, " ")
}

func qiniuRealtimeWalletConsumptionRemark(relayInfo *relaycommon.RelayInfo, requestId string, modelName string) string {
	parts := []string{"市场价实时 token/model 消费"}
	if strings.TrimSpace(modelName) != "" {
		parts = append(parts, "model="+strings.TrimSpace(modelName))
	}
	if relayInfo != nil && relayInfo.TokenId > 0 {
		parts = append(parts, fmt.Sprintf("token_id=%d", relayInfo.TokenId))
	}
	if strings.TrimSpace(requestId) != "" {
		parts = append(parts, "request="+strings.TrimSpace(requestId))
	}
	return strings.Join(parts, " ")
}

func qiniuRealtimeWalletRemark(application *model.QiniuRealtimeWalletApplication) string {
	if application == nil {
		return "市场价实时 token/model 消费"
	}
	return qiniuRealtimeWalletFlowRemark(QiniuRealtimeWalletFlowInput{
		RequestId: application.RequestId,
		BatchId:   application.BatchId,
	})
}

func recordQiniuRealtimeWalletFailure(input QiniuRealtimeWalletFlowInput, cause error) error {
	if cause == nil || input.UserId <= 0 || input.TokenId <= 0 || input.Quota <= 0 {
		return nil
	}
	idempotencyKey := qiniuRealtimeWalletIdempotencyKey(input)
	if idempotencyKey == "" {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		existing, exists, err := lockQiniuRealtimeWalletApplicationByKeyTx(tx, idempotencyKey)
		if err != nil {
			return err
		}
		if exists &&
			existing.Status == model.QiniuRealtimeWalletApplicationStatusApplied &&
			existing.SettlementApplied &&
			existing.UsageApplied {
			return nil
		}
		usageStatsApplied, usageLogApplied, usageApplied := qiniuRealtimeUsageStateFromInput(input)
		application := model.QiniuRealtimeWalletApplication{
			UserId:            input.UserId,
			TokenId:           input.TokenId,
			RequestId:         strings.TrimSpace(input.RequestId),
			BatchId:           strings.TrimSpace(input.BatchId),
			IdempotencyKey:    idempotencyKey,
			Quota:             input.Quota,
			PreConsumedQuota:  input.PreConsumedQuota,
			Amount:            quotaToWalletAmount(input.Quota),
			SettlementApplied: input.SettlementApplied,
			UsageStatsApplied: usageStatsApplied,
			UsageLogApplied:   usageLogApplied,
			UsageApplied:      usageApplied,
			LogUsername:       strings.TrimSpace(input.LogUsername),
			UpstreamRequestId: strings.TrimSpace(input.UpstreamRequestId),
			Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
			LastError:         sanitizeQiniuTaskError(cause),
		}
		application.SetConsumeLogIds(input.ConsumeLogIds)
		if err := application.SetConsumeLogParams(input.ConsumeLogParams); err != nil {
			return err
		}
		if exists {
			if len(application.ConsumeLogIdList()) == 0 && len(existing.ConsumeLogIdList()) > 0 {
				application.ConsumeLogId = existing.ConsumeLogId
				application.ConsumeLogIds = existing.ConsumeLogIds
				application.CoveredLogCount = existing.CoveredLogCount
			}
			if application.ConsumeLogPayload == "" {
				application.ConsumeLogPayload = existing.ConsumeLogPayload
			}
			if application.LogUsername == "" {
				application.LogUsername = existing.LogUsername
			}
			if application.UpstreamRequestId == "" {
				application.UpstreamRequestId = existing.UpstreamRequestId
			}
			application.UsageStatsApplied = application.UsageStatsApplied || existing.UsageStatsApplied || existing.UsageApplied
			application.UsageLogApplied = application.UsageLogApplied || existing.UsageLogApplied || existing.UsageApplied
			application.UsageApplied = application.UsageApplied || existing.UsageApplied || (application.UsageStatsApplied && application.UsageLogApplied)
			return tx.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
				"request_id":          application.RequestId,
				"batch_id":            application.BatchId,
				"consume_log_id":      application.ConsumeLogId,
				"consume_log_ids":     application.ConsumeLogIds,
				"covered_log_count":   application.CoveredLogCount,
				"quota":               application.Quota,
				"pre_consumed_quota":  application.PreConsumedQuota,
				"amount":              application.Amount,
				"settlement_applied":  application.SettlementApplied,
				"usage_stats_applied": application.UsageStatsApplied,
				"usage_log_applied":   application.UsageLogApplied,
				"usage_applied":       application.UsageApplied,
				"consume_log_payload": application.ConsumeLogPayload,
				"log_username":        application.LogUsername,
				"upstream_request_id": application.UpstreamRequestId,
				"status":              model.QiniuRealtimeWalletApplicationStatusFailed,
				"last_error":          application.LastError,
				"retry_count":         existing.RetryCount + 1,
				"updated_time":        common.GetTimestamp(),
			}).Error
		}
		return tx.Create(&application).Error
	})
}

func markQiniuRealtimeWalletApplicationFailed(applicationId int, cause error) error {
	if applicationId <= 0 || cause == nil {
		return nil
	}
	return model.DB.Model(&model.QiniuRealtimeWalletApplication{}).Where("id = ?", applicationId).Updates(map[string]interface{}{
		"status":       model.QiniuRealtimeWalletApplicationStatusFailed,
		"last_error":   sanitizeQiniuTaskError(cause),
		"updated_time": common.GetTimestamp(),
	}).Error
}
