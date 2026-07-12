package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	QiniuBillingOwnerStatusResolved       = "resolved"
	QiniuBillingOwnerStatusUnmapped       = "unmapped"
	QiniuBillingOwnerStatusAmbiguous      = "ambiguous"
	QiniuBillingOwnerStatusManualResolved = "manual_resolved"
)

const (
	QiniuBillingBucketStatusPending     = "pending"
	QiniuBillingBucketStatusNeedsReview = "needs_review"
	QiniuBillingBucketStatusApplied     = "applied"
	QiniuBillingBucketStatusFailed      = "failed"
	QiniuBillingBucketStatusSkipped     = "skipped"
	QiniuBillingBucketStatusReconciled  = "reconciled"
)

const (
	QiniuBillingApplicationStatusSuccess = "success"
	QiniuBillingApplicationStatusFailed  = "failed"
)

const (
	QiniuQuotaGrantStatusPending = "pending"
	QiniuQuotaGrantStatusApplied = "applied"
	QiniuQuotaGrantStatusFailed  = "failed"
)

const (
	QiniuBillingOperationSourceSystem = "system"
	QiniuBillingOperationSourceAdmin  = "admin"
)

const (
	QiniuBillingLocalRealtimeStatusFound   = "found"
	QiniuBillingLocalRealtimeStatusMissing = "missing"
	QiniuBillingLocalRealtimeStatusError   = "error"
)

// QiniuCostDetailRecord 保存七牛 cost-detail 返回的原始账单明细。
type QiniuCostDetailRecord struct {
	Id                  int     `json:"id"`
	QiniuMaskedKey      string  `json:"qiniu_masked_key" gorm:"type:varchar(128);index;comment:七牛脱敏 Key"`
	KeyPrefix           string  `json:"key_prefix" gorm:"type:varchar(32);index:idx_qiniu_cost_detail_key_match,priority:1;comment:脱敏 Key 前缀"`
	KeySuffix           string  `json:"key_suffix" gorm:"type:varchar(32);index:idx_qiniu_cost_detail_key_match,priority:2;comment:脱敏 Key 后缀"`
	BillingDate         string  `json:"billing_date" gorm:"type:varchar(16);index;comment:账单日期"`
	ModelName           string  `json:"model_name" gorm:"type:varchar(191);index;comment:七牛模型名"`
	BillingItem         string  `json:"billing_item" gorm:"type:varchar(191);index;comment:七牛账单项"`
	UsageCount          float64 `json:"usage_count" gorm:"type:decimal(24,6);not null;default:0;comment:官方用量"`
	UsageUnit           string  `json:"usage_unit" gorm:"type:varchar(32);comment:官方用量单位"`
	FeeAmount           float64 `json:"fee_amount" gorm:"type:decimal(18,6);not null;default:0;comment:官方费用金额"`
	Currency            string  `json:"currency" gorm:"type:varchar(16);default:'CNY';comment:币种"`
	RecordHash          string  `json:"record_hash" gorm:"type:varchar(128);uniqueIndex;comment:cost-detail 明细幂等摘要"`
	RawResponse         string  `json:"raw_response" gorm:"type:text;comment:官方原始明细 JSON"`
	OwnerStatus         string  `json:"owner_status" gorm:"type:varchar(32);index;comment:归属状态"`
	UserId              int     `json:"user_id" gorm:"index;comment:本地用户 ID"`
	TokenId             int     `json:"token_id" gorm:"index;comment:本地 Token ID"`
	QiniuChildAccountId int     `json:"qiniu_child_account_id" gorm:"column:qiniu_child_account_id;default:0;index;comment:Token 七牛子账号归属 ID，0 表示父账号"`
	RetryCount          int     `json:"retry_count" gorm:"default:0;comment:自动归属重试次数"`
	LastRetryTime       int64   `json:"last_retry_time" gorm:"bigint;default:0;comment:最近自动归属重试时间"`
	NextRetryTime       int64   `json:"next_retry_time" gorm:"bigint;default:0;comment:下次自动归属重试时间"`
	LastError           string  `json:"last_error" gorm:"type:text;comment:最近归属失败原因"`
	CreatedTime         int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64   `json:"updated_time" gorm:"bigint"`
}

// QiniuBillingBucket 按 token_id + billing_date 汇总官方账单和本地实时扣费差额。
type QiniuBillingBucket struct {
	Id                     int     `json:"id"`
	UserId                 int     `json:"user_id" gorm:"index;comment:用户 ID"`
	TokenId                int     `json:"token_id" gorm:"uniqueIndex:idx_qiniu_bucket_token_date,priority:1;index;comment:Token ID"`
	QiniuChildAccountId    int     `json:"qiniu_child_account_id" gorm:"column:qiniu_child_account_id;default:0;index;comment:Token 七牛子账号归属 ID，0 表示父账号"`
	BillingDate            string  `json:"billing_date" gorm:"type:varchar(16);uniqueIndex:idx_qiniu_bucket_token_date,priority:2;index;comment:账单日期"`
	QiniuMaskedKey         string  `json:"qiniu_masked_key" gorm:"type:varchar(128);index;comment:七牛脱敏 Key"`
	KeyFingerprint         string  `json:"key_fingerprint" gorm:"type:varchar(128);index;comment:Key 指纹，仅用于审计"`
	OwnerStatus            string  `json:"owner_status" gorm:"type:varchar(32);index;comment:归属状态"`
	OfficialAmount         float64 `json:"official_amount" gorm:"type:decimal(18,6);not null;default:0;comment:当前官方金额"`
	OfficialQuota          int     `json:"official_quota" gorm:"default:0;comment:当前官方金额换算 quota"`
	PreviousOfficialAmount float64 `json:"previous_official_amount" gorm:"type:decimal(18,6);not null;default:0;comment:上一次官方金额"`
	PreviousOfficialQuota  int     `json:"previous_official_quota" gorm:"default:0;comment:上一次官方 quota"`
	LocalRealtimeQuota     int     `json:"local_realtime_quota" gorm:"default:0;comment:本地实时已扣 quota"`
	LocalRealtimeStatus    string  `json:"local_realtime_status" gorm:"type:varchar(32);index;comment:本地实时日志汇总状态"`
	AppliedDeltaQuota      int     `json:"applied_delta_quota" gorm:"default:0;comment:已通过 bucket 应用的累计差额 quota"`
	PendingDeltaQuota      int     `json:"pending_delta_quota" gorm:"default:0;comment:待应用差额 quota"`
	ApplyVersion           int     `json:"apply_version" gorm:"default:0;comment:应用版本"`
	Status                 string  `json:"status" gorm:"type:varchar(32);index;comment:bucket 状态"`
	LastError              string  `json:"last_error" gorm:"type:text;comment:最近失败原因"`
	RetryCount             int     `json:"retry_count" gorm:"default:0;comment:自动处理重试次数"`
	LastRetryTime          int64   `json:"last_retry_time" gorm:"bigint;default:0;comment:最近自动处理重试时间"`
	NextRetryTime          int64   `json:"next_retry_time" gorm:"bigint;default:0;comment:下次自动处理重试时间"`
	CreatedTime            int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime            int64   `json:"updated_time" gorm:"bigint"`
}

// QiniuBillingBucketItem 保存 bucket 下按模型和账单项汇总的审计明细。
type QiniuBillingBucketItem struct {
	Id           int     `json:"id"`
	BucketId     int     `json:"bucket_id" gorm:"uniqueIndex:idx_qiniu_bucket_item_detail,priority:1;index;comment:bucket ID"`
	ModelName    string  `json:"model_name" gorm:"type:varchar(191);uniqueIndex:idx_qiniu_bucket_item_detail,priority:2;index;comment:七牛模型名"`
	BillingItem  string  `json:"billing_item" gorm:"type:varchar(191);uniqueIndex:idx_qiniu_bucket_item_detail,priority:3;index;comment:七牛账单项"`
	UsageCount   float64 `json:"usage_count" gorm:"type:decimal(24,6);not null;default:0;comment:用量汇总"`
	FeeAmount    float64 `json:"fee_amount" gorm:"type:decimal(18,6);not null;default:0;comment:费用汇总"`
	Currency     string  `json:"currency" gorm:"type:varchar(16);default:'CNY';comment:币种"`
	RawRecordIds string  `json:"raw_record_ids" gorm:"type:text;comment:关联 raw record ID 列表"`
	CreatedTime  int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64   `json:"updated_time" gorm:"bigint"`
}

// QiniuBillingBucketApplication 记录每次 bucket 差额落账的幂等应用结果。
type QiniuBillingBucketApplication struct {
	Id                 int     `json:"id"`
	BucketId           int     `json:"bucket_id" gorm:"index;uniqueIndex:idx_qiniu_bucket_application_version,priority:1;comment:bucket ID"`
	ApplyVersion       int     `json:"apply_version" gorm:"uniqueIndex:idx_qiniu_bucket_application_version,priority:2;comment:应用版本"`
	DeltaQuota         int     `json:"delta_quota" gorm:"comment:本次差额 quota，负数表示退款"`
	DeltaAmount        float64 `json:"delta_amount" gorm:"type:decimal(18,6);not null;default:0;comment:本次差额金额，负数表示退款"`
	WalletFlowId       int     `json:"wallet_flow_id" gorm:"index;comment:关联钱包流水 ID"`
	ConsumeLogId       int     `json:"consume_log_id" gorm:"index;comment:关联合成消费/退款日志 ID"`
	IdempotencyKey     string  `json:"idempotency_key" gorm:"type:varchar(255);uniqueIndex;comment:账务幂等键"`
	BalanceBeforeQuota int     `json:"balance_before_quota" gorm:"comment:应用前用户余额 quota"`
	BalanceAfterQuota  int     `json:"balance_after_quota" gorm:"comment:应用后用户余额 quota"`
	DebtQuota          int     `json:"debt_quota" gorm:"default:0;comment:应用后形成或剩余的欠费 quota"`
	Status             string  `json:"status" gorm:"type:varchar(32);index;comment:应用状态"`
	LastError          string  `json:"last_error" gorm:"type:text;comment:最近失败原因"`
	RetryCount         int     `json:"retry_count" gorm:"default:0;comment:自动应用重试次数"`
	LastRetryTime      int64   `json:"last_retry_time" gorm:"bigint;default:0;comment:最近自动应用重试时间"`
	NextRetryTime      int64   `json:"next_retry_time" gorm:"bigint;default:0;comment:下次自动应用重试时间"`
	OperationSource    string  `json:"operation_source" gorm:"type:varchar(32);index;comment:操作来源"`
	CreatedTime        int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime        int64   `json:"updated_time" gorm:"bigint"`
}

// QiniuQuotaGrant 记录本地余额增加事件对应的七牛远端额度授权增量。
type QiniuQuotaGrant struct {
	Id                int     `json:"id"`
	UserId            int     `json:"user_id" gorm:"index;comment:用户 ID"`
	TokenId           int     `json:"token_id" gorm:"index;comment:Token ID"`
	BusinessKey       string  `json:"business_key" gorm:"type:varchar(255);uniqueIndex;comment:业务幂等号"`
	GrantAmount       float64 `json:"grant_amount" gorm:"type:decimal(18,6);not null;default:0;comment:远端 total_quota 授权增量"`
	RemoteApplyStatus string  `json:"remote_apply_status" gorm:"type:varchar(32);index;comment:远端应用状态"`
	RemoteApplyTime   int64   `json:"remote_apply_time" gorm:"bigint;index;comment:远端应用时间戳"`
	LastError         string  `json:"last_error" gorm:"type:text;comment:最近失败原因"`
	CreatedTime       int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64   `json:"updated_time" gorm:"bigint"`
}

type QiniuCostDetailRecordQuery struct {
	OwnerStatus         string
	QiniuMaskedKey      string
	UserId              int
	TokenId             int
	QiniuChildAccountId int
	BillingDate         string
	ModelName           string
	BillingItem         string
	CreatedFrom         int64
	CreatedTo           int64
}

type QiniuBillingBucketQuery struct {
	Status              string
	OwnerStatus         string
	UserId              int
	TokenId             int
	QiniuChildAccountId int
	BillingDate         string
	QiniuMaskedKey      string
	CreatedFrom         int64
	CreatedTo           int64
}

type QiniuBillingBucketItemQuery struct {
	BucketId    int
	ModelName   string
	BillingItem string
	CreatedFrom int64
	CreatedTo   int64
}

type QiniuBillingBucketApplicationQuery struct {
	Status      string
	BucketId    int
	CreatedFrom int64
	CreatedTo   int64
}

type QiniuQuotaGrantQuery struct {
	RemoteApplyStatus string
	UserId            int
	TokenId           int
	BusinessKey       string
	CreatedFrom       int64
	CreatedTo         int64
}

func (record *QiniuCostDetailRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if record.CreatedTime == 0 {
		record.CreatedTime = now
	}
	if record.UpdatedTime == 0 {
		record.UpdatedTime = now
	}
	if record.Currency == "" {
		record.Currency = "CNY"
	}
	if record.OwnerStatus == "" {
		record.OwnerStatus = QiniuBillingOwnerStatusUnmapped
	}
	return nil
}

func (record *QiniuCostDetailRecord) BeforeUpdate(tx *gorm.DB) error {
	record.UpdatedTime = common.GetTimestamp()
	return nil
}

func (bucket *QiniuBillingBucket) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if bucket.CreatedTime == 0 {
		bucket.CreatedTime = now
	}
	if bucket.UpdatedTime == 0 {
		bucket.UpdatedTime = now
	}
	if bucket.OwnerStatus == "" {
		bucket.OwnerStatus = QiniuBillingOwnerStatusResolved
	}
	if bucket.Status == "" {
		bucket.Status = QiniuBillingBucketStatusPending
	}
	return nil
}

func (bucket *QiniuBillingBucket) BeforeUpdate(tx *gorm.DB) error {
	bucket.UpdatedTime = common.GetTimestamp()
	return nil
}

func (item *QiniuBillingBucketItem) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if item.CreatedTime == 0 {
		item.CreatedTime = now
	}
	if item.UpdatedTime == 0 {
		item.UpdatedTime = now
	}
	if item.Currency == "" {
		item.Currency = "CNY"
	}
	return nil
}

func (item *QiniuBillingBucketItem) BeforeUpdate(tx *gorm.DB) error {
	item.UpdatedTime = common.GetTimestamp()
	return nil
}

func (application *QiniuBillingBucketApplication) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if application.CreatedTime == 0 {
		application.CreatedTime = now
	}
	if application.UpdatedTime == 0 {
		application.UpdatedTime = now
	}
	if application.Status == "" {
		application.Status = QiniuBillingApplicationStatusSuccess
	}
	if application.OperationSource == "" {
		application.OperationSource = QiniuBillingOperationSourceSystem
	}
	return nil
}

func (application *QiniuBillingBucketApplication) BeforeUpdate(tx *gorm.DB) error {
	application.UpdatedTime = common.GetTimestamp()
	return nil
}

func (grant *QiniuQuotaGrant) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if grant.CreatedTime == 0 {
		grant.CreatedTime = now
	}
	if grant.UpdatedTime == 0 {
		grant.UpdatedTime = now
	}
	if grant.RemoteApplyStatus == "" {
		grant.RemoteApplyStatus = QiniuQuotaGrantStatusPending
	}
	return nil
}

func (grant *QiniuQuotaGrant) BeforeUpdate(tx *gorm.DB) error {
	grant.UpdatedTime = common.GetTimestamp()
	return nil
}

func QiniuBillingBucketIdempotencyKey(bucketId int, applyVersion int) string {
	return fmt.Sprintf("qiniu:billing_bucket:%d:v%d", bucketId, applyVersion)
}

func CreateQiniuBillingBucketApplication(tx *gorm.DB, application *QiniuBillingBucketApplication) error {
	if application == nil {
		return errors.New("七牛账单 bucket 应用记录不能为空")
	}
	if tx == nil {
		tx = DB
	}
	return tx.Create(application).Error
}

func UpsertQiniuCostDetailRecord(tx *gorm.DB, record *QiniuCostDetailRecord) (*QiniuCostDetailRecord, bool, error) {
	if record == nil {
		return nil, false, errors.New("七牛 cost-detail 原始记录不能为空")
	}
	if record.RecordHash == "" {
		return nil, false, errors.New("七牛 cost-detail 原始记录缺少 record_hash")
	}
	if tx == nil {
		tx = DB
	}
	var existing QiniuCostDetailRecord
	err := tx.Where("record_hash = ?", record.RecordHash).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(record).Error; err != nil {
			return nil, false, err
		}
		return record, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	ownerStatus := record.OwnerStatus
	userId := record.UserId
	tokenId := record.TokenId
	qiniuChildAccountId := record.QiniuChildAccountId
	if existing.OwnerStatus == QiniuBillingOwnerStatusManualResolved {
		// 已人工归属的历史明细不能被重复同步覆盖，避免延迟账单回扫破坏审计结论。
		ownerStatus = existing.OwnerStatus
		userId = existing.UserId
		tokenId = existing.TokenId
		qiniuChildAccountId = existing.QiniuChildAccountId
	}
	updates := map[string]interface{}{
		"qiniu_masked_key":       record.QiniuMaskedKey,
		"key_prefix":             record.KeyPrefix,
		"key_suffix":             record.KeySuffix,
		"billing_date":           record.BillingDate,
		"model_name":             record.ModelName,
		"billing_item":           record.BillingItem,
		"usage_count":            record.UsageCount,
		"usage_unit":             record.UsageUnit,
		"fee_amount":             record.FeeAmount,
		"currency":               record.Currency,
		"raw_response":           record.RawResponse,
		"owner_status":           ownerStatus,
		"user_id":                userId,
		"token_id":               tokenId,
		"qiniu_child_account_id": qiniuChildAccountId,
		"updated_time":           common.GetTimestamp(),
	}
	if err := tx.Model(&QiniuCostDetailRecord{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		return nil, false, err
	}
	if err := tx.Where("id = ?", existing.Id).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return &existing, false, nil
}

func ListQiniuCostDetailRecords(filter QiniuCostDetailRecordQuery, pageInfo *common.PageInfo) (records []QiniuCostDetailRecord, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuCostDetailRecord{})
	query = applyQiniuCostDetailRecordQuery(query, filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("billing_date desc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error
	return records, total, err
}

func ListQiniuBillingBuckets(filter QiniuBillingBucketQuery, pageInfo *common.PageInfo) (buckets []QiniuBillingBucket, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuBillingBucket{})
	query = applyQiniuBillingBucketQuery(query, filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("billing_date desc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&buckets).Error
	return buckets, total, err
}

func ListQiniuBillingBucketItems(filter QiniuBillingBucketItemQuery, pageInfo *common.PageInfo) (items []QiniuBillingBucketItem, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuBillingBucketItem{})
	query = applyQiniuBillingBucketItemQuery(query, filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id asc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&items).Error
	return items, total, err
}

func ListQiniuBillingBucketApplications(filter QiniuBillingBucketApplicationQuery, pageInfo *common.PageInfo) (apps []QiniuBillingBucketApplication, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuBillingBucketApplication{})
	query = applyQiniuBillingBucketApplicationQuery(query, filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&apps).Error
	return apps, total, err
}

func applyQiniuCostDetailRecordQuery(query *gorm.DB, filter QiniuCostDetailRecordQuery) *gorm.DB {
	if filter.OwnerStatus != "" {
		query = query.Where("owner_status = ?", filter.OwnerStatus)
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
	if filter.BillingDate != "" {
		query = query.Where("billing_date = ?", strings.TrimSpace(filter.BillingDate))
	}
	if filter.QiniuMaskedKey != "" {
		query = query.Where("qiniu_masked_key = ?", strings.TrimSpace(filter.QiniuMaskedKey))
	}
	if filter.ModelName != "" {
		query = query.Where("model_name = ?", strings.TrimSpace(filter.ModelName))
	}
	if filter.BillingItem != "" {
		query = query.Where("billing_item = ?", strings.TrimSpace(filter.BillingItem))
	}
	if filter.CreatedFrom > 0 {
		query = query.Where("created_time >= ?", filter.CreatedFrom)
	}
	if filter.CreatedTo > 0 {
		query = query.Where("created_time <= ?", filter.CreatedTo)
	}
	return query
}

func applyQiniuBillingBucketQuery(query *gorm.DB, filter QiniuBillingBucketQuery) *gorm.DB {
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.OwnerStatus != "" {
		query = query.Where("owner_status = ?", filter.OwnerStatus)
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
	if filter.BillingDate != "" {
		query = query.Where("billing_date = ?", strings.TrimSpace(filter.BillingDate))
	}
	if filter.QiniuMaskedKey != "" {
		query = query.Where("qiniu_masked_key = ?", strings.TrimSpace(filter.QiniuMaskedKey))
	}
	if filter.CreatedFrom > 0 {
		query = query.Where("created_time >= ?", filter.CreatedFrom)
	}
	if filter.CreatedTo > 0 {
		query = query.Where("created_time <= ?", filter.CreatedTo)
	}
	return query
}

func applyQiniuBillingBucketItemQuery(query *gorm.DB, filter QiniuBillingBucketItemQuery) *gorm.DB {
	if filter.BucketId > 0 {
		query = query.Where("bucket_id = ?", filter.BucketId)
	}
	if filter.ModelName != "" {
		query = query.Where("model_name = ?", strings.TrimSpace(filter.ModelName))
	}
	if filter.BillingItem != "" {
		query = query.Where("billing_item = ?", strings.TrimSpace(filter.BillingItem))
	}
	if filter.CreatedFrom > 0 {
		query = query.Where("created_time >= ?", filter.CreatedFrom)
	}
	if filter.CreatedTo > 0 {
		query = query.Where("created_time <= ?", filter.CreatedTo)
	}
	return query
}

func applyQiniuBillingBucketApplicationQuery(query *gorm.DB, filter QiniuBillingBucketApplicationQuery) *gorm.DB {
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.BucketId > 0 {
		query = query.Where("bucket_id = ?", filter.BucketId)
	}
	if filter.CreatedFrom > 0 {
		query = query.Where("created_time >= ?", filter.CreatedFrom)
	}
	if filter.CreatedTo > 0 {
		query = query.Where("created_time <= ?", filter.CreatedTo)
	}
	return query
}
