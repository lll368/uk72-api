package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	WalletTypeBalance    = "balance"
	WalletTypeCommission = "commission"
)

const (
	WalletFlowDirectionIn  = "in"
	WalletFlowDirectionOut = "out"
)

const (
	WalletFlowTypeRechargeBalance     = "recharge_balance"
	WalletFlowTypeVipActivation       = "vip_activation"
	WalletFlowTypeCommissionIncome    = "commission_income"
	WalletFlowTypeCommissionToBalance = "commission_to_balance"
	WalletFlowTypeWithdrawFreeze      = "withdraw_freeze"
	WalletFlowTypeWithdrawSuccess     = "withdraw_success"
	WalletFlowTypeWithdrawReject      = "withdraw_reject"
	WalletFlowTypeRefundReverse       = "refund_reverse"
	WalletFlowTypeBalanceConsume      = "balance_consume"
	WalletFlowTypeBalanceRefund       = "balance_refund"
	WalletFlowTypeLegacyBalanceInit   = "legacy_balance_init"
)

// WalletAccount 对应 wallet_accounts 表，记录用户消费余额和佣金余额的分账户汇总。
type WalletAccount struct {
	Id                     int     `json:"id" gorm:"comment:主键ID"`
	UserId                 int     `json:"user_id" gorm:"uniqueIndex;comment:用户ID"`
	BalanceAmount          float64 `json:"balance_amount" gorm:"type:decimal(18,6);not null;default:0;comment:消费余额"`
	CommissionAmount       float64 `json:"commission_amount" gorm:"type:decimal(18,6);not null;default:0;comment:可提现佣金余额"`
	FrozenCommissionAmount float64 `json:"frozen_commission_amount" gorm:"type:decimal(18,6);not null;default:0;comment:冻结佣金余额"`
	TotalCommissionAmount  float64 `json:"total_commission_amount" gorm:"type:decimal(18,6);not null;default:0;comment:累计佣金金额"`
	TotalWithdrawAmount    float64 `json:"total_withdraw_amount" gorm:"type:decimal(18,6);not null;default:0;comment:累计提现金额"`
	CreatedAt              int64   `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt              int64   `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// WalletFlow 对应 wallet_flows 表，记录余额和佣金的每次资金变动。
type WalletFlow struct {
	Id                    int     `json:"id" gorm:"comment:主键ID"`
	UserId                int     `json:"user_id" gorm:"index;comment:用户ID"`
	BizNo                 string  `json:"biz_no" gorm:"type:varchar(255);index;comment:业务单号"`
	IdempotencyKey        *string `json:"idempotency_key,omitempty" gorm:"type:varchar(255);uniqueIndex;comment:幂等键"`
	FlowType              string  `json:"flow_type" gorm:"type:varchar(64);index;comment:流水类型"`
	WalletType            string  `json:"wallet_type" gorm:"type:varchar(32);index;comment:钱包类型"`
	Direction             string  `json:"direction" gorm:"type:varchar(16);index;comment:资金方向"`
	Amount                float64 `json:"amount" gorm:"type:decimal(18,6);not null;default:0;comment:变动金额"`
	BalanceAfter          float64 `json:"balance_after" gorm:"type:decimal(18,6);not null;default:0;comment:变动后消费余额"`
	CommissionAfter       float64 `json:"commission_after" gorm:"type:decimal(18,6);not null;default:0;comment:变动后可提现佣金余额"`
	FrozenCommissionAfter float64 `json:"frozen_commission_after" gorm:"type:decimal(18,6);not null;default:0;comment:变动后冻结佣金余额"`
	Remark                string  `json:"remark" gorm:"type:text;comment:备注"`
	CreatedAt             int64   `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
}

// BeforeCreate 初始化钱包账户的创建和更新时间戳。
func (a *WalletAccount) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	return nil
}

// BeforeUpdate 在钱包账户更新时刷新更新时间戳。
func (a *WalletAccount) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = common.GetTimestamp()
	return nil
}

// BeforeCreate 初始化钱包流水的创建时间戳。
func (f *WalletFlow) BeforeCreate(tx *gorm.DB) error {
	if f.CreatedAt == 0 {
		f.CreatedAt = common.GetTimestamp()
	}
	return nil
}

// GetWalletAccountByUserId 根据用户 ID 查询钱包账户。
func GetWalletAccountByUserId(userId int) (*WalletAccount, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var account WalletAccount
	if err := DB.Where("user_id = ?", userId).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
