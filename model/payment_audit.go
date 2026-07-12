package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	PaymentProcessStatusPending = "pending"
	PaymentProcessStatusSuccess = "success"
	PaymentProcessStatusFailed  = "failed"
)

// PaymentCallbackLog 对应 payment_callback_logs 表，记录支付渠道回调验签和处理结果。
type PaymentCallbackLog struct {
	Id            int    `json:"id" gorm:"comment:主键ID"`
	Provider      string `json:"provider" gorm:"type:varchar(50);index;comment:支付提供方"`
	EventType     string `json:"event_type" gorm:"type:varchar(64);index;comment:回调事件类型"`
	TradeNo       string `json:"trade_no" gorm:"type:varchar(255);index;comment:支付订单号"`
	BizType       string `json:"biz_type" gorm:"type:varchar(64);index;comment:业务类型"`
	VerifyStatus  bool   `json:"verify_status" gorm:"default:false;index;comment:验签是否通过"`
	ProcessStatus string `json:"process_status" gorm:"type:varchar(32);default:'pending';index;comment:业务处理状态"`
	PayloadDigest string `json:"payload_digest" gorm:"type:varchar(255);default:'';comment:回调载荷摘要"`
	ErrorMessage  string `json:"error_message" gorm:"type:text;comment:错误信息"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// PaymentReconciliationTask 对应 payment_reconciliation_tasks 表，记录支付对账批次和差异结果。
type PaymentReconciliationTask struct {
	Id           int    `json:"id" gorm:"comment:主键ID"`
	Provider     string `json:"provider" gorm:"type:varchar(50);index;comment:支付提供方"`
	DateFrom     int64  `json:"date_from" gorm:"bigint;index;comment:对账开始时间戳"`
	DateTo       int64  `json:"date_to" gorm:"bigint;index;comment:对账结束时间戳"`
	Status       string `json:"status" gorm:"type:varchar(32);default:'pending';index;comment:对账任务状态"`
	TotalCount   int    `json:"total_count" gorm:"type:int;default:0;comment:对账订单总数"`
	DiffCount    int    `json:"diff_count" gorm:"type:int;default:0;comment:差异订单数量"`
	ErrorMessage string `json:"error_message" gorm:"type:text;comment:错误信息"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// BeforeCreate 初始化支付回调日志的时间戳和默认处理状态。
func (l *PaymentCallbackLog) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if l.CreatedAt == 0 {
		l.CreatedAt = now
	}
	l.UpdatedAt = now
	if l.ProcessStatus == "" {
		l.ProcessStatus = PaymentProcessStatusPending
	}
	return nil
}

// BeforeUpdate 在支付回调日志更新时刷新更新时间戳。
func (l *PaymentCallbackLog) BeforeUpdate(tx *gorm.DB) error {
	l.UpdatedAt = common.GetTimestamp()
	return nil
}

// BeforeCreate 初始化支付对账任务的时间戳和默认状态。
func (t *PaymentReconciliationTask) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = PaymentProcessStatusPending
	}
	return nil
}

// BeforeUpdate 在支付对账任务更新时刷新更新时间戳。
func (t *PaymentReconciliationTask) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = common.GetTimestamp()
	return nil
}

// ListPaymentCallbackLogs 分页查询支付回调审计日志。
func ListPaymentCallbackLogs(provider string, tradeNo string, processStatus string, pageInfo *common.PageInfo) (logs []*PaymentCallbackLog, total int64, err error) {
	query := DB.Model(&PaymentCallbackLog{})
	if provider = strings.TrimSpace(provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if tradeNo = strings.TrimSpace(tradeNo); tradeNo != "" {
		query = query.Where("trade_no = ?", tradeNo)
	}
	if processStatus = strings.TrimSpace(processStatus); processStatus != "" {
		query = query.Where("process_status = ?", processStatus)
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&logs).Error
	return logs, total, err
}

// ListPaymentReconciliationTasks 分页查询支付对账任务。
func ListPaymentReconciliationTasks(provider string, status string, pageInfo *common.PageInfo) (tasks []*PaymentReconciliationTask, total int64, err error) {
	query := DB.Model(&PaymentReconciliationTask{})
	if provider = strings.TrimSpace(provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&tasks).Error
	return tasks, total, err
}
