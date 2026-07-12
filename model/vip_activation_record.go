package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	VipActivationStatusPending  = "pending"
	VipActivationStatusSuccess  = "success"
	VipActivationStatusFailed   = "failed"
	VipActivationStatusDisabled = "disabled"
)

const (
	DefaultVipActivationAmount   = 1680.0
	DefaultVipActivationPaid     = 1680.0
	DefaultVipActivationDiscount = 1.0
)

var (
	ErrVipActivationOrderNotFound      = errors.New("vip activation order not found")
	ErrVipActivationOrderStatusInvalid = errors.New("vip activation order status invalid")
	ErrVipActivationAlreadyActive           = errors.New("用户已开通 VVIP，不能重复发起支付")
	ErrVipActivationUserAlreadyVvip         = errors.New("用户已是算力伙伴或曾是算力伙伴")
	ErrVipActivationManualRemarkRequired    = errors.New("备注信息不能为空")
)

// VipActivationRecord 对应 vip_activation_records 表，记录 VVIP 一次性付费开通订单快照。
type VipActivationRecord struct {
	Id               int     `json:"id" gorm:"comment:主键ID"`
	UserId           int     `json:"user_id" gorm:"index;comment:开通用户ID"`
	TradeNo          string  `json:"trade_no" gorm:"type:varchar(255);uniqueIndex;comment:支付订单号"`
	ActivationAmount float64 `json:"activation_amount" gorm:"type:decimal(18,6);not null;default:1680;comment:VVIP开通金额"`
	PaidAmount       float64 `json:"paid_amount" gorm:"type:decimal(18,6);not null;default:1680;comment:实际支付金额"`
	Discount         float64 `json:"discount" gorm:"type:decimal(10,6);not null;default:1;comment:订单折扣"`
	PaymentProvider  string  `json:"payment_provider" gorm:"type:varchar(50);default:'';comment:支付提供方"`
	PaymentMethod    string  `json:"payment_method" gorm:"type:varchar(50);default:'';comment:支付方式"`
	Status           string  `json:"status" gorm:"type:varchar(32);default:'pending';index;comment:开通状态"`
	ProviderPayload  string  `json:"provider_payload" gorm:"type:text;comment:支付渠道回调摘要"`
	ActivatedAt      int64   `json:"activated_at" gorm:"bigint;index;comment:开通时间戳"`
	DisabledAt       int64   `json:"disabled_at" gorm:"bigint;default:0;index;comment:禁用时间戳"`
	DisabledBy       int     `json:"disabled_by" gorm:"type:int;default:0;index;comment:禁用管理员ID"`
	DisableReason    string  `json:"disable_reason" gorm:"type:text;comment:禁用原因"`
	ActivatedBy      int     `json:"activated_by" gorm:"type:int;default:0;index;comment:手动激活管理员ID,0=支付激活"`
	ActivationRemark string  `json:"activation_remark" gorm:"type:text;comment:手动激活备注"`
	CreatedAt        int64   `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt        int64   `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// BeforeCreate 初始化 VVIP 开通记录的时间戳、金额快照和默认状态。
func (r *VipActivationRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if r.ActivationAmount == 0 {
		r.ActivationAmount = DefaultVipActivationAmount
	}
	if r.PaidAmount == 0 {
		r.PaidAmount = DefaultVipActivationPaid
	}
	if r.Discount == 0 {
		r.Discount = DefaultVipActivationDiscount
	}
	if r.Status == "" {
		r.Status = VipActivationStatusPending
	}
	return nil
}

// BeforeUpdate 在 VVIP 开通记录更新时刷新更新时间戳。
func (r *VipActivationRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

// GetVipActivationByUserId 查询用户当前有效的 VVIP 开通记录。
func GetVipActivationByUserId(userId int) (*VipActivationRecord, error) {
	return GetActiveVipActivationByUserId(userId)
}

// GetActiveVipActivationByUserId 查询用户已成功激活的 VVIP 开通记录。
func GetActiveVipActivationByUserId(userId int) (*VipActivationRecord, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var record VipActivationRecord
	if err := DB.Where("user_id = ? AND status = ? AND activated_at > ?", userId, VipActivationStatusSuccess, 0).
		Order("activated_at desc, id desc").
		First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// GetLatestVipActivationByUserId 查询用户最新一条 VVIP 开通记录，主要用于审计和排错。
func GetLatestVipActivationByUserId(userId int) (*VipActivationRecord, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var record VipActivationRecord
	if err := DB.Where("user_id = ?", userId).Order("id desc").First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// GetVipActivationRecordByTradeNo 根据支付订单号查询 VVIP 开通订单。
func GetVipActivationRecordByTradeNo(tradeNo string) (*VipActivationRecord, error) {
	if tradeNo == "" {
		return nil, ErrVipActivationOrderNotFound
	}
	var record VipActivationRecord
	if err := DB.Where("trade_no = ?", tradeNo).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVipActivationOrderNotFound
		}
		return nil, err
	}
	return &record, nil
}

// IsUserActiveVvip 判断用户是否存在已成功且未禁用的 VVIP 开通记录。
func IsUserActiveVvip(userId int) (bool, error) {
	return IsUserActiveVvipTx(DB, userId)
}

// IsUserActiveVvipTx 在指定事务内判断用户是否为有效 VVIP。
func IsUserActiveVvipTx(tx *gorm.DB, userId int) (bool, error) {
	if tx == nil {
		tx = DB
	}
	if userId <= 0 {
		return false, nil
	}
	var count int64
	if err := tx.Model(&VipActivationRecord{}).
		Where("user_id = ? AND status = ? AND activated_at > ?", userId, VipActivationStatusSuccess, 0).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListVipActivationRecords 分页查询 VVIP 开通记录，供管理员审计使用。
func ListVipActivationRecords(pageInfo *common.PageInfo) (records []*VipActivationRecord, total int64, err error) {
	query := DB.Model(&VipActivationRecord{})
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error
	return records, total, err
}
