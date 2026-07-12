package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	QiniuOfficialRecordTypeUsage = "usage"
	QiniuOfficialRecordTypeBill  = "bill"

	QiniuOfficialRecordStatusPending  = "pending"
	QiniuOfficialRecordStatusApplied  = "applied"
	QiniuOfficialRecordStatusSkipped  = "skipped"
	QiniuOfficialRecordStatusFailed   = "failed"
	QiniuOfficialRecordStatusUnmapped = "unmapped"

	QiniuOfficialLedgerStatusSuccess = "success"
	QiniuOfficialLedgerStatusFailed  = "failed"
)

// QiniuOfficialUsageRecord 保存七牛官方用量和账单接口返回的原始明细。
type QiniuOfficialUsageRecord struct {
	Id                  int     `json:"id"`
	RecordKey           string  `json:"record_key" gorm:"type:varchar(255);uniqueIndex;comment:官方明细幂等键"`
	RecordType          string  `json:"record_type" gorm:"type:varchar(32);index;comment:usage 或 bill"`
	SourceAPI           string  `json:"source_api" gorm:"type:varchar(128);index;comment:官方接口路径"`
	RecordHash          string  `json:"record_hash" gorm:"type:varchar(128);index;comment:官方原始明细摘要"`
	QiniuKey            string  `json:"qiniu_key" gorm:"type:varchar(128);index;comment:七牛托管 Key"`
	UserId              int     `json:"user_id" gorm:"index;comment:本地用户 ID"`
	TokenId             int     `json:"token_id" gorm:"index;comment:本地 Token ID"`
	QiniuChildAccountId int     `json:"qiniu_child_account_id" gorm:"column:qiniu_child_account_id;default:0;index;comment:Token 七牛子账号归属 ID，0 表示父账号"`
	PeriodStart         int64   `json:"period_start" gorm:"bigint;index;comment:统计周期开始时间戳"`
	PeriodEnd           int64   `json:"period_end" gorm:"bigint;index;comment:统计周期结束时间戳"`
	Granularity         string  `json:"granularity" gorm:"type:varchar(32);index;comment:官方统计粒度"`
	ModelName           string  `json:"model_name" gorm:"type:varchar(191);index;comment:官方模型名"`
	BillingItem         string  `json:"billing_item" gorm:"type:varchar(191);index;comment:官方账单项"`
	PromptTokens        int64   `json:"prompt_tokens" gorm:"comment:输入 tokens"`
	CompletionTokens    int64   `json:"completion_tokens" gorm:"comment:输出 tokens"`
	TotalTokens         int64   `json:"total_tokens" gorm:"comment:总 tokens"`
	FeeAmount           float64 `json:"fee_amount" gorm:"type:decimal(18,6);not null;default:0;comment:官方费用金额"`
	Currency            string  `json:"currency" gorm:"type:varchar(16);default:'CNY';comment:币种"`
	OfficialQuota       int     `json:"official_quota" gorm:"default:0;comment:官方费用换算后的 quota"`
	AppliedQuota        int     `json:"applied_quota" gorm:"default:0;comment:已应用到本地账务的 quota"`
	ApplyVersion        int     `json:"apply_version" gorm:"default:0;comment:ledger 应用版本"`
	Status              string  `json:"status" gorm:"type:varchar(32);index;comment:同步/应用状态"`
	LastError           string  `json:"last_error" gorm:"type:text;comment:最近失败原因"`
	RawResponse         string  `json:"raw_response" gorm:"type:text;comment:官方原始明细 JSON"`
	FetchedAt           int64   `json:"fetched_at" gorm:"bigint;index;comment:抓取时间戳"`
	CreatedTime         int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64   `json:"updated_time" gorm:"bigint"`
}

// QiniuOfficialLedgerApplication 记录每次把官方 ledger 差额应用到本地账务的结果。
type QiniuOfficialLedgerApplication struct {
	Id             int     `json:"id"`
	UsageRecordId  int     `json:"usage_record_id" gorm:"index;uniqueIndex:idx_qiniu_ledger_record_version,priority:1;comment:官方原始记录 ID"`
	ApplyVersion   int     `json:"apply_version" gorm:"uniqueIndex:idx_qiniu_ledger_record_version,priority:2;comment:应用版本"`
	UserId         int     `json:"user_id" gorm:"index;comment:用户 ID"`
	TokenId        int     `json:"token_id" gorm:"index;comment:Token ID"`
	DeltaQuota     int     `json:"delta_quota" gorm:"comment:本次差额 quota，负数表示退款"`
	DeltaAmount    float64 `json:"delta_amount" gorm:"type:decimal(18,6);not null;default:0;comment:本次差额金额，负数表示退款"`
	WalletFlowId   int     `json:"wallet_flow_id" gorm:"index;comment:关联钱包流水 ID"`
	ConsumeLogId   int     `json:"consume_log_id" gorm:"index;comment:关联合成消费/退款日志 ID"`
	IdempotencyKey string  `json:"idempotency_key" gorm:"type:varchar(255);uniqueIndex;comment:账务幂等键"`
	Status         string  `json:"status" gorm:"type:varchar(32);index;comment:应用状态"`
	LastError      string  `json:"last_error" gorm:"type:text;comment:最近失败原因"`
	CreatedTime    int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime    int64   `json:"updated_time" gorm:"bigint"`
}

type QiniuOfficialUsageRecordQuery struct {
	RecordType          string
	Status              string
	QiniuKey            string
	UserId              int
	TokenId             int
	QiniuChildAccountId int
	PeriodStart         int64
	PeriodEnd           int64
	ModelName           string
	BillingItem         string
	CreatedFrom         int64
	CreatedTo           int64
}

type QiniuOfficialLedgerApplicationQuery struct {
	Status        string
	UserId        int
	TokenId       int
	UsageRecordId int
	CreatedFrom   int64
	CreatedTo     int64
}

func (record *QiniuOfficialUsageRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if record.CreatedTime == 0 {
		record.CreatedTime = now
	}
	if record.UpdatedTime == 0 {
		record.UpdatedTime = now
	}
	if record.FetchedAt == 0 {
		record.FetchedAt = now
	}
	if record.Status == "" {
		record.Status = QiniuOfficialRecordStatusPending
	}
	if record.Currency == "" {
		record.Currency = "CNY"
	}
	return nil
}

func (record *QiniuOfficialUsageRecord) BeforeUpdate(tx *gorm.DB) error {
	record.UpdatedTime = common.GetTimestamp()
	return nil
}

func (application *QiniuOfficialLedgerApplication) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if application.CreatedTime == 0 {
		application.CreatedTime = now
	}
	if application.UpdatedTime == 0 {
		application.UpdatedTime = now
	}
	if application.Status == "" {
		application.Status = QiniuOfficialLedgerStatusSuccess
	}
	return nil
}

func (application *QiniuOfficialLedgerApplication) BeforeUpdate(tx *gorm.DB) error {
	application.UpdatedTime = common.GetTimestamp()
	return nil
}

func QiniuOfficialLedgerIdempotencyKey(usageRecordId int, applyVersion int) string {
	return fmt.Sprintf("qiniu:usage_apply:%d:v%d", usageRecordId, applyVersion)
}

func CreateQiniuOfficialLedgerApplication(tx *gorm.DB, application *QiniuOfficialLedgerApplication) error {
	if application == nil {
		return errors.New("七牛官方 ledger 应用记录不能为空")
	}
	if tx == nil {
		tx = DB
	}
	return tx.Create(application).Error
}

func UpsertQiniuOfficialUsageRecord(tx *gorm.DB, record *QiniuOfficialUsageRecord) (*QiniuOfficialUsageRecord, bool, error) {
	if record == nil {
		return nil, false, errors.New("七牛官方原始记录不能为空")
	}
	if record.RecordKey == "" {
		return nil, false, errors.New("七牛官方原始记录缺少 record_key")
	}
	if tx == nil {
		tx = DB
	}
	var existing QiniuOfficialUsageRecord
	err := tx.Where("record_key = ?", record.RecordKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(record).Error; err != nil {
			return nil, false, err
		}
		return record, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	status := record.Status
	lastError := record.LastError
	if existing.Status == QiniuOfficialRecordStatusApplied {
		if qiniuOfficialAppliedBillRecordChanged(&existing, record) {
			status = QiniuOfficialRecordStatusPending
			lastError = ""
		} else {
			status = existing.Status
			lastError = existing.LastError
		}
	}
	updates := map[string]interface{}{
		"record_type":            record.RecordType,
		"source_api":             record.SourceAPI,
		"record_hash":            record.RecordHash,
		"qiniu_key":              record.QiniuKey,
		"user_id":                record.UserId,
		"token_id":               record.TokenId,
		"qiniu_child_account_id": record.QiniuChildAccountId,
		"period_start":           record.PeriodStart,
		"period_end":             record.PeriodEnd,
		"granularity":            record.Granularity,
		"model_name":             record.ModelName,
		"billing_item":           record.BillingItem,
		"prompt_tokens":          record.PromptTokens,
		"completion_tokens":      record.CompletionTokens,
		"total_tokens":           record.TotalTokens,
		"fee_amount":             record.FeeAmount,
		"currency":               record.Currency,
		"official_quota":         record.OfficialQuota,
		"status":                 status,
		"last_error":             lastError,
		"raw_response":           record.RawResponse,
		"fetched_at":             record.FetchedAt,
		"updated_time":           common.GetTimestamp(),
	}
	if err := tx.Model(&QiniuOfficialUsageRecord{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		return nil, false, err
	}
	if err := tx.Where("id = ?", existing.Id).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return &existing, false, nil
}

func qiniuOfficialAppliedBillRecordChanged(existing *QiniuOfficialUsageRecord, incoming *QiniuOfficialUsageRecord) bool {
	if existing == nil || incoming == nil {
		return false
	}
	if existing.RecordType != QiniuOfficialRecordTypeBill || incoming.RecordType != QiniuOfficialRecordTypeBill {
		return false
	}
	return existing.OfficialQuota != incoming.OfficialQuota ||
		existing.FeeAmount != incoming.FeeAmount ||
		existing.RecordHash != incoming.RecordHash
}

func ListPendingQiniuOfficialBillRecords(limit int, failedRetryBefore int64) ([]QiniuOfficialUsageRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var records []QiniuOfficialUsageRecord
	query := DB.Where("record_type = ?", QiniuOfficialRecordTypeBill)
	if failedRetryBefore > 0 {
		query = query.Where("(status = ? OR (status = ? AND (updated_time = 0 OR updated_time <= ?)))",
			QiniuOfficialRecordStatusPending,
			QiniuOfficialRecordStatusFailed,
			failedRetryBefore,
		)
	} else {
		query = query.Where("status IN ?", []string{QiniuOfficialRecordStatusPending, QiniuOfficialRecordStatusFailed})
	}
	err := query.
		Order("period_start asc, id asc").
		Limit(limit).
		Find(&records).Error
	return records, err
}

func ListQiniuOfficialUsageRecords(filter QiniuOfficialUsageRecordQuery, pageInfo *common.PageInfo) (records []QiniuOfficialUsageRecord, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuOfficialUsageRecord{})
	query = applyQiniuOfficialUsageRecordQuery(query, filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("period_start desc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error
	return records, total, err
}

func ListQiniuOfficialLedgerApplications(filter QiniuOfficialLedgerApplicationQuery, pageInfo *common.PageInfo) (apps []QiniuOfficialLedgerApplication, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuOfficialLedgerApplication{})
	query = applyQiniuOfficialLedgerApplicationQuery(query, filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&apps).Error
	return apps, total, err
}

func applyQiniuOfficialUsageRecordQuery(query *gorm.DB, filter QiniuOfficialUsageRecordQuery) *gorm.DB {
	if filter.RecordType != "" {
		query = query.Where("record_type = ?", filter.RecordType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.TokenId > 0 {
		query = query.Where("token_id = ?", filter.TokenId)
	}
	if filter.QiniuChildAccountId > 0 {
		query = query.Where("qiniu_child_account_id = ?", filter.QiniuChildAccountId)
	}
	if filter.QiniuKey != "" {
		qiniuKey := strings.TrimSpace(filter.QiniuKey)
		fullKey := qiniuKey
		if !strings.HasPrefix(fullKey, "sk-") {
			fullKey = "sk-" + fullKey
		}
		query = query.Where("(qiniu_key = ? OR qiniu_key = ?)", qiniuKey, fullKey)
	}
	if filter.PeriodStart > 0 {
		query = query.Where("period_start >= ?", filter.PeriodStart)
	}
	if filter.PeriodEnd > 0 {
		query = query.Where("period_end <= ?", filter.PeriodEnd)
	}
	if filter.ModelName != "" {
		query = query.Where("model_name = ?", filter.ModelName)
	}
	if filter.BillingItem != "" {
		query = query.Where("billing_item = ?", filter.BillingItem)
	}
	if filter.CreatedFrom > 0 {
		query = query.Where("created_time >= ?", filter.CreatedFrom)
	}
	if filter.CreatedTo > 0 {
		query = query.Where("created_time <= ?", filter.CreatedTo)
	}
	return query
}

func applyQiniuOfficialLedgerApplicationQuery(query *gorm.DB, filter QiniuOfficialLedgerApplicationQuery) *gorm.DB {
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.TokenId > 0 {
		query = query.Where("token_id = ?", filter.TokenId)
	}
	if filter.UsageRecordId > 0 {
		query = query.Where("usage_record_id = ?", filter.UsageRecordId)
	}
	if filter.CreatedFrom > 0 {
		query = query.Where("created_time >= ?", filter.CreatedFrom)
	}
	if filter.CreatedTo > 0 {
		query = query.Where("created_time <= ?", filter.CreatedTo)
	}
	return query
}
