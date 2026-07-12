package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	CommissionSourceTypeTopUp         = "topup"
	CommissionSourceTypeVipActivation = "vip_activation"
)

const (
	CommissionQualificationQualified   = "qualified"
	CommissionQualificationUnqualified = "unqualified"
)

const (
	CommissionStatusPending  = "pending"
	CommissionStatusSettled  = "settled"
	CommissionStatusFailed   = "failed"
	CommissionStatusReversed = "reversed"
)

// CommissionRecord 对应 commission_records 表，记录充值和 VVIP 开通产生的佣金。
type CommissionRecord struct {
	Id                  int     `json:"id" gorm:"comment:主键ID"`
	BeneficiaryUserId   int     `json:"beneficiary_user_id" gorm:"index;uniqueIndex:idx_commission_source_level_beneficiary;comment:佣金受益用户ID"`
	SourceUserId        int     `json:"source_user_id" gorm:"index;comment:佣金来源用户ID"`
	SourceUserLabel     string  `json:"source_user_label" gorm:"type:varchar(255);default:'';comment:佣金来源用户展示快照"`
	SourceOrderNo       string  `json:"source_order_no" gorm:"type:varchar(255);index;uniqueIndex:idx_commission_source_level_beneficiary;comment:来源订单号"`
	SourceType          string  `json:"source_type" gorm:"type:varchar(32);index;uniqueIndex:idx_commission_source_level_beneficiary;comment:佣金来源类型"`
	Level               int     `json:"level" gorm:"type:int;index;uniqueIndex:idx_commission_source_level_beneficiary;comment:佣金层级"`
	BaseAmount          float64 `json:"base_amount" gorm:"type:decimal(18,6);not null;default:0;comment:佣金计算基数"`
	CommissionRate      float64 `json:"commission_rate" gorm:"type:decimal(10,6);not null;default:0;comment:佣金比例快照"`
	Amount              float64 `json:"amount" gorm:"type:decimal(18,6);not null;default:0;comment:佣金金额"`
	QualificationStatus string  `json:"qualification_status" gorm:"type:varchar(32);default:'qualified';index;comment:分佣资格状态"`
	Status              string  `json:"status" gorm:"type:varchar(32);default:'pending';index;comment:佣金状态"`
	ErrorMessage        string  `json:"error_message" gorm:"type:varchar(512);default:'';comment:分佣失败原因"`
	SettledAt           int64   `json:"settled_at" gorm:"bigint;default:0;index;comment:结算时间戳"`
	ReversedAt          int64   `json:"reversed_at" gorm:"bigint;default:0;index;comment:冲正时间戳"`
	ReverseReason       string  `json:"reverse_reason" gorm:"type:varchar(255);default:'';comment:冲正原因"`
	CreatedAt           int64   `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt           int64   `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// BeforeCreate 初始化佣金记录的时间戳、资格状态和默认状态。
func (r *CommissionRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if r.QualificationStatus == "" {
		r.QualificationStatus = CommissionQualificationQualified
	}
	if r.Status == "" {
		r.Status = CommissionStatusPending
	}
	return nil
}

// BeforeUpdate 在佣金记录更新时刷新更新时间戳。
func (r *CommissionRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}
