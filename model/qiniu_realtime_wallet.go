package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	QiniuRealtimeWalletApplicationStatusPending = "pending"
	QiniuRealtimeWalletApplicationStatusApplied = "applied"
	QiniuRealtimeWalletApplicationStatusFailed  = "failed"
)

// QiniuRealtimeWalletApplication 记录七牛实时使用日志到最终钱包流水的幂等应用结果。
type QiniuRealtimeWalletApplication struct {
	Id                int     `json:"id"`
	UserId            int     `json:"user_id" gorm:"index;comment:用户 ID"`
	TokenId           int     `json:"token_id" gorm:"index;comment:Token ID"`
	RequestId         string  `json:"request_id" gorm:"type:varchar(64);index;comment:请求 ID"`
	BatchId           string  `json:"batch_id" gorm:"type:varchar(128);index;comment:使用日志聚合批次 ID"`
	ConsumeLogId      int     `json:"consume_log_id" gorm:"index;comment:单条或首条使用日志 ID"`
	ConsumeLogIds     string  `json:"consume_log_ids" gorm:"type:text;comment:覆盖的使用日志 ID 列表"`
	CoveredLogCount   int     `json:"covered_log_count" gorm:"default:0;comment:覆盖使用日志数量"`
	WalletFlowId      int     `json:"wallet_flow_id" gorm:"index;comment:钱包流水 ID"`
	IdempotencyKey    string  `json:"idempotency_key" gorm:"type:varchar(255);uniqueIndex;comment:应用幂等键"`
	Quota             int     `json:"quota" gorm:"comment:最终消费 quota"`
	PreConsumedQuota  int     `json:"pre_consumed_quota" gorm:"default:0;comment:实时请求已预扣 quota"`
	Amount            float64 `json:"amount" gorm:"type:decimal(18,6);not null;default:0;comment:最终消费金额"`
	BalanceAfter      float64 `json:"balance_after" gorm:"type:decimal(18,6);not null;default:0;comment:静默结算后钱包余额"`
	SettlementApplied bool    `json:"settlement_applied" gorm:"default:false;comment:静默余额结算是否已完成"`
	UsageStatsApplied bool    `json:"usage_stats_applied" gorm:"default:false;comment:使用统计是否已恢复"`
	UsageLogApplied   bool    `json:"usage_log_applied" gorm:"default:false;comment:使用日志是否已恢复"`
	UsageApplied      bool    `json:"usage_applied" gorm:"default:false;comment:使用日志与统计是否已恢复"`
	ConsumeLogPayload string  `json:"consume_log_payload" gorm:"type:text;comment:使用日志重放参数"`
	LogUsername       string  `json:"log_username" gorm:"type:varchar(191);comment:使用日志用户名快照"`
	UpstreamRequestId string  `json:"upstream_request_id" gorm:"type:varchar(191);comment:上游请求 ID 快照"`
	Status            string  `json:"status" gorm:"type:varchar(32);index;comment:应用状态"`
	RetryCount        int     `json:"retry_count" gorm:"default:0;comment:重试次数"`
	LastError         string  `json:"last_error" gorm:"type:text;comment:最近失败原因"`
	CreatedTime       int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64   `json:"updated_time" gorm:"bigint"`
}

// SetConsumeLogIds 以稳定字符串保存覆盖的使用日志集合，避免跨库 JSON 查询差异。
func (application *QiniuRealtimeWalletApplication) SetConsumeLogIds(ids []int) {
	if application == nil {
		return
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		parts = append(parts, strconv.Itoa(id))
	}
	application.ConsumeLogIds = strings.Join(parts, ",")
	application.CoveredLogCount = len(parts)
	if len(ids) > 0 && ids[0] > 0 {
		application.ConsumeLogId = ids[0]
	}
}

// SetConsumeLogParams 保存可重放的使用日志参数，用于 settlement 失败后的补偿恢复。
func (application *QiniuRealtimeWalletApplication) SetConsumeLogParams(params *RecordConsumeLogParams) error {
	if application == nil || params == nil {
		return nil
	}
	data, err := common.Marshal(params)
	if err != nil {
		return err
	}
	application.ConsumeLogPayload = string(data)
	return nil
}

// ConsumeLogParamsValue 返回 settlement 失败时保存的使用日志参数。
func (application *QiniuRealtimeWalletApplication) ConsumeLogParamsValue() (*RecordConsumeLogParams, error) {
	if application == nil || strings.TrimSpace(application.ConsumeLogPayload) == "" {
		return nil, nil
	}
	var params RecordConsumeLogParams
	if err := common.Unmarshal([]byte(application.ConsumeLogPayload), &params); err != nil {
		return nil, err
	}
	return &params, nil
}

// ConsumeLogIdList 返回该应用覆盖的使用日志 ID。
func (application *QiniuRealtimeWalletApplication) ConsumeLogIdList() []int {
	if application == nil || strings.TrimSpace(application.ConsumeLogIds) == "" {
		if application != nil && application.ConsumeLogId > 0 {
			return []int{application.ConsumeLogId}
		}
		return nil
	}
	rawParts := strings.Split(application.ConsumeLogIds, ",")
	ids := make([]int, 0, len(rawParts))
	for _, part := range rawParts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// BeforeCreate 初始化七牛实时钱包应用记录的时间和默认状态。
func (application *QiniuRealtimeWalletApplication) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if application.CreatedTime == 0 {
		application.CreatedTime = now
	}
	if application.UpdatedTime == 0 {
		application.UpdatedTime = now
	}
	if application.Status == "" {
		application.Status = QiniuRealtimeWalletApplicationStatusPending
	}
	return nil
}

// BeforeUpdate 刷新七牛实时钱包应用记录更新时间。
func (application *QiniuRealtimeWalletApplication) BeforeUpdate(tx *gorm.DB) error {
	application.UpdatedTime = common.GetTimestamp()
	return nil
}
