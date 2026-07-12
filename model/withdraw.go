package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	WithdrawStatusPending  = "pending"
	WithdrawStatusApproved = "approved"
	WithdrawStatusPaid     = "paid"
	WithdrawStatusRejected = "rejected"
	WithdrawStatusFailed   = "failed"

	WithdrawStatusSubmitted    = "submitted"
	WithdrawStatusAwaitConfirm = "await_confirm"
	WithdrawStatusConfirming   = "confirming"
	WithdrawStatusCancelling   = "cancelling"
	WithdrawStatusConfirmed    = "confirmed"
	WithdrawStatusCancelled    = "cancelled"
	WithdrawStatusManualReview = "manual_review"

	WithdrawProviderManual       = "manual"
	WithdrawProviderPiggyLaborV3 = "piggy_labor_v3"

	WithdrawAccountTypeBankcard = "bankcard"

	PiggySignStatusUnsigned = "unsigned"
	PiggySignStatusSigned   = "signed"
	PiggySignStatusFailed   = "failed"

	PiggyCallbackTypeContract = "contract"
	PiggyCallbackTypePayment  = "payment"
)

// WithdrawOrder 对应 withdraw_orders 表，记录用户佣金提现申请和审核打款状态。
type WithdrawOrder struct {
	Id                       int     `json:"id" gorm:"comment:主键ID"`
	UserId                   int     `json:"user_id" gorm:"index;comment:提现用户ID"`
	WithdrawNo               string  `json:"withdraw_no" gorm:"type:varchar(255);uniqueIndex;comment:提现单号"`
	Amount                   float64 `json:"amount" gorm:"type:decimal(18,6);not null;default:0;comment:提现申请金额"`
	FeeAmount                float64 `json:"fee_amount" gorm:"type:decimal(18,6);not null;default:0;comment:提现手续费"`
	PlatformFeeRate          float64 `json:"platform_fee_rate" gorm:"type:decimal(10,4);not null;default:0;comment:小猪平台服务费率百分比快照"`
	PlatformFeeAmountCents   int64   `json:"platform_fee_amount_cents" gorm:"type:bigint;not null;default:0;comment:小猪平台服务费金额分快照"`
	ActualAmount             float64 `json:"actual_amount" gorm:"type:decimal(18,6);not null;default:0;comment:实际到账金额"`
	Status                   string  `json:"status" gorm:"type:varchar(32);default:'pending';index;comment:提现状态"`
	Provider                 string  `json:"provider" gorm:"type:varchar(32);not null;default:'manual';index;comment:提现通道"`
	PiggyStatus              string  `json:"piggy_status" gorm:"type:varchar(32);not null;default:'';index;comment:小猪状态"`
	ReceiveType              string  `json:"receive_type" gorm:"type:varchar(64);default:'';comment:收款方式"`
	ReceiveAccount           string  `json:"receive_account" gorm:"type:varchar(255);default:'';comment:收款账户"`
	WithdrawalProfileId      int     `json:"withdrawal_profile_id" gorm:"type:int;default:0;index;comment:提现资料ID"`
	AccountName              string  `json:"account_name" gorm:"type:varchar(128);default:'';comment:收款人姓名快照"`
	BankName                 string  `json:"bank_name" gorm:"type:varchar(128);default:'';comment:银行名称快照"`
	PayoutMobile             string  `json:"-" gorm:"type:varchar(32);not null;default:'';comment:打款手机号快照"`
	PayoutIdCardNo           string  `json:"-" gorm:"type:varchar(64);not null;default:'';comment:打款身份证号快照"`
	PayoutBankCardNo         string  `json:"-" gorm:"type:varchar(64);not null;default:'';comment:打款银行卡号快照"`
	TaxBeforeAmountCents     int64   `json:"tax_before_amount_cents" gorm:"type:bigint;not null;default:0;comment:税前申请金额分"`
	FrozenAmountCents        int64   `json:"frozen_amount_cents" gorm:"type:bigint;not null;default:0;comment:冻结佣金金额分"`
	PiggyPayAmountCents      int64   `json:"piggy_pay_amount_cents" gorm:"type:bigint;not null;default:0;comment:提交小猪金额分"`
	PiggyPretaxAmountCents   int64   `json:"piggy_pretax_amount_cents" gorm:"type:bigint;not null;default:0;comment:小猪税前金额分"`
	PiggyIndividualTaxCents  int64   `json:"piggy_individual_tax_cents" gorm:"type:bigint;not null;default:0;comment:小猪个税金额分"`
	PiggyAddedTaxCents       int64   `json:"piggy_added_tax_cents" gorm:"type:bigint;not null;default:0;comment:小猪增值税金额分"`
	PiggyAfterTaxAmountCents int64   `json:"piggy_after_tax_amount_cents" gorm:"type:bigint;not null;default:0;comment:小猪税后到账金额分"`
	PiggyFeeAmountCents      int64   `json:"piggy_fee_amount_cents" gorm:"type:bigint;not null;default:0;comment:小猪手续费金额分"`
	PiggyPayAmount           string  `json:"piggy_pay_amount" gorm:"type:varchar(32);default:'';comment:提交小猪元金额"`
	ExternalTradeNo          string  `json:"external_trade_no" gorm:"type:varchar(255);default:'';index;comment:外部订单号"`
	FrontLogNo               string  `json:"front_log_no" gorm:"type:varchar(255);default:'';index;comment:小猪前置流水号"`
	LaborOrderNo             string  `json:"labor_order_no" gorm:"type:varchar(255);default:'';index;comment:小猪劳务订单号"`
	NotifyType               string  `json:"notify_type" gorm:"type:varchar(64);default:'';index;comment:最近小猪回调类型"`
	TradeStatus              string  `json:"trade_status" gorm:"type:varchar(64);default:'';index;comment:最近小猪交易状态"`
	TradeFailCode            string  `json:"trade_fail_code" gorm:"type:varchar(128);default:'';comment:小猪失败编码"`
	TradeResult              string  `json:"trade_result" gorm:"type:varchar(128);default:'';comment:小猪交易结果"`
	TradeResultDescribe      string  `json:"trade_result_describe" gorm:"type:text;comment:小猪交易结果说明"`
	TaxFundId                string  `json:"tax_fund_id" gorm:"type:varchar(128);default:'';comment:税源地ID"`
	PositionName             string  `json:"position_name" gorm:"type:varchar(128);default:'';comment:岗位名称"`
	Position                 string  `json:"position" gorm:"type:varchar(128);default:'';comment:岗位标识"`
	CalcType                 string  `json:"calc_type" gorm:"type:varchar(16);default:'';comment:税费承担方式"`
	BankRemark               string  `json:"bank_remark" gorm:"type:varchar(255);default:'';comment:银行附言"`
	RequestPayloadDigest     string  `json:"request_payload_digest" gorm:"type:varchar(255);default:'';comment:请求摘要"`
	ResponsePayloadDigest    string  `json:"response_payload_digest" gorm:"type:varchar(255);default:'';comment:响应摘要"`
	ManualReviewReason       string  `json:"manual_review_reason" gorm:"type:text;comment:人工处理原因"`
	ManualHandledBy          int     `json:"manual_handled_by" gorm:"type:int;default:0;index;comment:人工处理人"`
	ManualHandledAt          int64   `json:"manual_handled_at" gorm:"bigint;default:0;comment:人工处理时间"`
	ManualHandleResult       string  `json:"manual_handle_result" gorm:"type:text;comment:人工处理结果"`
	CompensationStatus       string  `json:"compensation_status" gorm:"type:varchar(32);default:'';index;comment:补偿处理状态"`
	SubmittedAt              int64   `json:"submitted_at" gorm:"bigint;default:0;comment:提交小猪时间"`
	ConfirmedAt              int64   `json:"confirmed_at" gorm:"bigint;default:0;comment:确认打款时间"`
	TerminalAt               int64   `json:"terminal_at" gorm:"bigint;default:0;comment:终态时间"`
	ReviewerId               int     `json:"reviewer_id" gorm:"type:int;default:0;index;comment:审核人ID"`
	ReviewedAt               int64   `json:"reviewed_at" gorm:"bigint;default:0;comment:审核时间戳"`
	PaidAt                   int64   `json:"paid_at" gorm:"bigint;default:0;comment:打款时间戳"`
	PaymentVoucher           string  `json:"payment_voucher" gorm:"type:varchar(255);default:'';comment:打款凭证"`
	FailReason               string  `json:"fail_reason" gorm:"type:varchar(255);default:'';comment:失败原因"`
	Remark                   string  `json:"remark" gorm:"type:text;comment:备注"`
	CreatedAt                int64   `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt                int64   `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// WithdrawalProfile 对应 withdrawal_profiles 表，保存用户银行卡提现资料和小猪签约状态。
type WithdrawalProfile struct {
	Id                          int    `json:"id" gorm:"comment:主键ID"`
	UserId                      int    `json:"user_id" gorm:"uniqueIndex;comment:用户ID"`
	AccountType                 string `json:"account_type" gorm:"type:varchar(32);not null;default:'bankcard';index;comment:提现账户类型"`
	RealName                    string `json:"real_name" gorm:"type:varchar(128);not null;default:'';comment:真实姓名"`
	IdCardNo                    string `json:"-" gorm:"type:varchar(64);not null;default:'';comment:身份证号"`
	Mobile                      string `json:"-" gorm:"type:varchar(32);not null;default:'';comment:手机号"`
	BankCardNo                  string `json:"-" gorm:"type:varchar(64);not null;default:'';comment:银行卡号"`
	BankName                    string `json:"bank_name" gorm:"type:varchar(128);not null;default:'';comment:银行名称"`
	MaskedIdCardNo              string `json:"masked_id_card_no" gorm:"-"`
	MaskedMobile                string `json:"masked_mobile" gorm:"-"`
	MaskedBankCardNo            string `json:"masked_bank_card_no" gorm:"-"`
	PiggySignStatus             string `json:"piggy_sign_status" gorm:"type:varchar(32);not null;default:'unsigned';index;comment:小猪签约状态"`
	PiggySignUrlDigest          string `json:"piggy_sign_url_digest" gorm:"type:varchar(255);default:'';comment:最近签约URL摘要"`
	PiggySignedAt               int64  `json:"piggy_signed_at" gorm:"bigint;default:0;comment:小猪签约完成时间"`
	PiggyContractURL            string `json:"piggy_contract_url,omitempty" gorm:"type:varchar(512);default:'';comment:小猪签约合同查看地址"`
	PiggyContractDocumentID     string `json:"piggy_contract_document_id,omitempty" gorm:"type:varchar(128);default:'';index;comment:小猪合同编号"`
	PiggyContractSubsidiaryName string `json:"piggy_contract_subsidiary_name,omitempty" gorm:"type:varchar(255);default:'';comment:小猪签约结算公司名称"`
	PiggyContractPosition       string `json:"piggy_contract_position,omitempty" gorm:"type:varchar(128);default:'';comment:小猪签约服务类型"`
	PiggyContractPositionName   string `json:"piggy_contract_position_name,omitempty" gorm:"type:varchar(128);default:'';comment:小猪签约岗位名称"`
	PiggyContractTaxFundID      string `json:"piggy_contract_tax_fund_id,omitempty" gorm:"type:varchar(128);default:'';comment:小猪签约税源地ID"`
	LastCallbackDigest          string `json:"last_callback_digest" gorm:"type:varchar(255);default:'';comment:最近签约回调摘要"`
	CreatedAt                   int64  `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt                   int64  `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// PiggyWithdrawCallbackLog 对应 piggy_withdraw_callback_logs 表，记录小猪签约和支付回调审计信息。
type PiggyWithdrawCallbackLog struct {
	Id              int    `json:"id" gorm:"comment:主键ID"`
	CallbackType    string `json:"callback_type" gorm:"type:varchar(32);index;comment:回调类型"`
	OrderNo         string `json:"order_no" gorm:"type:varchar(255);index;comment:本地提现单号"`
	UserId          int    `json:"user_id" gorm:"type:int;default:0;index;comment:用户ID"`
	NotifyType      string `json:"notify_type" gorm:"type:varchar(64);index;comment:小猪回调类型"`
	TradeStatus     string `json:"trade_status" gorm:"type:varchar(64);index;comment:小猪交易状态"`
	FrontLogNo      string `json:"front_log_no" gorm:"type:varchar(255);index;comment:小猪前置流水号"`
	LaborOrderNo    string `json:"labor_order_no" gorm:"type:varchar(255);index;comment:小猪劳务订单号"`
	IdempotencyKey  string `json:"idempotency_key" gorm:"type:varchar(512);index;comment:幂等键"`
	PayloadDigest   string `json:"payload_digest" gorm:"type:varchar(255);default:'';comment:原始载荷摘要"`
	DecryptedDigest string `json:"decrypted_digest" gorm:"type:varchar(255);default:'';comment:解密载荷摘要"`
	ProcessStatus   string `json:"process_status" gorm:"type:varchar(32);default:'pending';index;comment:处理状态"`
	ErrorMessage    string `json:"error_message" gorm:"type:text;comment:错误信息"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// BeforeCreate 初始化提现订单的时间戳和默认状态。
func (o *WithdrawOrder) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if o.CreatedAt == 0 {
		o.CreatedAt = now
	}
	o.UpdatedAt = now
	if o.Status == "" {
		o.Status = WithdrawStatusPending
	}
	if strings.TrimSpace(o.Provider) == "" {
		o.Provider = WithdrawProviderManual
	}
	return nil
}

// BeforeUpdate 在提现订单更新时刷新更新时间戳。
func (o *WithdrawOrder) BeforeUpdate(tx *gorm.DB) error {
	o.UpdatedAt = common.GetTimestamp()
	return nil
}

// BeforeCreate 初始化提现资料的时间戳和默认账户/签约状态。
func (p *WithdrawalProfile) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if strings.TrimSpace(p.AccountType) == "" {
		p.AccountType = WithdrawAccountTypeBankcard
	}
	if strings.TrimSpace(p.PiggySignStatus) == "" {
		p.PiggySignStatus = PiggySignStatusUnsigned
	}
	return nil
}

// BeforeUpdate 更新时间戳。
func (p *WithdrawalProfile) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// BeforeCreate 初始化小猪回调审计日志。
func (l *PiggyWithdrawCallbackLog) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if l.CreatedAt == 0 {
		l.CreatedAt = now
	}
	l.UpdatedAt = now
	if strings.TrimSpace(l.ProcessStatus) == "" {
		l.ProcessStatus = PaymentProcessStatusPending
	}
	return nil
}

// BeforeUpdate 更新时间戳。
func (l *PiggyWithdrawCallbackLog) BeforeUpdate(tx *gorm.DB) error {
	l.UpdatedAt = common.GetTimestamp()
	return nil
}
