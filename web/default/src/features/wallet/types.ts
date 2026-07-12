/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// ============================================================================
// Wallet Type Definitions
// ============================================================================

/**
 * Generic API response
 */
export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

/**
 * Standard API response types
 */
export type TopupInfoResponse = ApiResponse<TopupInfo>
export type RedemptionResponse = ApiResponse<number>
export type AmountResponse = ApiResponse<string>
export type PaymentResponse = ApiResponse<Record<string, unknown>> & {
  url?: string
}
export interface SignedFormPaymentData {
  url: string
  params: Record<string, unknown>
  order_id?: string
}
export interface QrPaymentData {
  trade_no?: string
  order_id?: string
  code_url: string
  expires_at?: number | string
}
export type QrPaymentStatus = 'pending' | 'success' | 'failed' | 'expired'
export interface QrPaymentResult extends QrPaymentData {
  type: 'qr'
  payment_method: 'wechat_direct'
  purpose: 'topup' | 'vvip_activation'
  amount?: number
}
export type StripePaymentResponse = ApiResponse<{ pay_link: string }>
export type AlipayPaymentResponse = ApiResponse<SignedFormPaymentData>
export type WechatPayPaymentResponse = ApiResponse<QrPaymentData>
export type AffiliateCodeResponse = ApiResponse<string>
export type AffiliateTransferResponse = ApiResponse
export type CreemPaymentResponse = ApiResponse<{ checkout_url: string }>
export type WaffoPaymentResponse = ApiResponse<
  { payment_url?: string } | string
>
export type WaffoPancakePaymentResponse = ApiResponse<
  | {
      checkout_url?: string
      session_id?: string
      expires_at?: number | string
      order_id?: string
    }
  | string
>
export type VipActivationInfoResponse = ApiResponse<VipActivationInfo>
export type VipActivationEpayResponse = PaymentResponse
export type VipActivationAlipayResponse = ApiResponse<SignedFormPaymentData>
export type VipActivationWechatPayResponse = ApiResponse<QrPaymentData>
export type VipActivationStripeResponse = ApiResponse<{
  pay_link: string
  order_id: string
}>
export type VipActivationCreemResponse = ApiResponse<{
  checkout_url: string
  order_id: string
}>
export type VipActivationWaffoResponse = ApiResponse<{
  payment_url?: string
  order_id?: string
}>
export type VipActivationRecordsResponse = ApiResponse<VipActivationRecordsPage>

/**
 * Creem product configuration
 */
export interface CreemProduct {
  /** Product display name */
  name: string
  /** Creem product ID */
  productId: string
  /** Product price */
  price: number
  /** Quota amount to credit */
  quota: number
  /** Currency (USD or EUR) */
  currency: 'USD' | 'EUR'
}

/**
 * Creem payment request
 */
export interface CreemPaymentRequest {
  /** Creem product ID */
  product_id: string
  /** Payment method identifier */
  payment_method: 'creem'
}

/**
 * Payment method configuration
 */
export interface PaymentMethod {
  /** Display name of payment method */
  name: string
  /** Payment method type identifier */
  type: string
  /** Optional color for UI display */
  color?: string
  /** Minimum topup amount for this payment method */
  min_topup?: number
  /** Optional icon URL provided by backend (preferred over built-in icons) */
  icon?: string
}

/**
 * Waffo payment method configuration
 */
export interface WaffoPayMethod {
  /** Display name of payment method */
  name: string
  /** Optional icon path */
  icon?: string
  /** Waffo pay method type */
  payMethodType?: string
  /** Waffo pay method name */
  payMethodName?: string
}

/**
 * Topup configuration information
 */
export interface TopupInfo {
  /** Whether online topup is enabled */
  enable_online_topup: boolean
  /** Whether Stripe topup is enabled */
  enable_stripe_topup: boolean
  /** Available payment methods */
  pay_methods: PaymentMethod[]
  /** Minimum topup amount for online topup */
  min_topup: number
  /** Minimum topup amount for Stripe */
  stripe_min_topup: number
  /** Preset amount options */
  amount_options: number[]
  /** Discount rates by amount */
  discount: Record<number, number>
  /** Optional topup link for purchasing codes */
  topup_link?: string
  /** Whether Creem topup is enabled */
  enable_creem_topup?: boolean
  /** Available Creem products */
  creem_products?: CreemProduct[]
  /** Effective VVIP relation topup discount for current user */
  relation_topup_discount?: number
  /** Whether Creem products are hidden because relation discount is active */
  creem_hidden_by_relation_discount?: boolean
  /** Whether Waffo topup is enabled */
  enable_waffo_topup?: boolean
  /** Available Waffo payment methods */
  waffo_pay_methods?: WaffoPayMethod[]
  /** Minimum topup amount for Waffo */
  waffo_min_topup?: number
  /** Whether Waffo Pancake topup is enabled */
  enable_waffo_pancake_topup?: boolean
  /** Minimum topup amount for Waffo Pancake */
  waffo_pancake_min_topup?: number
  /** Whether redemption code usage is enabled */
  enable_redemption?: boolean
  /** Whether compliance confirmation has been completed */
  payment_compliance_confirmed?: boolean
  /** Current compliance terms version */
  payment_compliance_terms_version?: string
}

/**
 * VVIP one-time activation information
 */
export interface VipActivationInfo {
  /** Whether current user has an active paid VVIP activation */
  is_vvip: boolean
  /** Activation status */
  status: 'pending' | 'success' | 'failed' | 'disabled'
  /** Activation timestamp */
  activated_at: number
  /** Fixed activation amount */
  activation_amount: number
  /** Fixed paid amount */
  paid_amount: number
  /** Fixed discount snapshot */
  discount: number
  /** Available payment methods */
  payment_methods: PaymentMethod[]
  /** Eligible Creem products priced exactly for VVIP activation */
  creem_products?: CreemProduct[]
  /** Available Waffo payment methods */
  waffo_pay_methods?: WaffoPayMethod[]
  /** Invite code for active VVIP users */
  aff_code?: string
  /** Invite link for active VVIP users */
  invite_link?: string
}

/**
 * VVIP activation record status
 */
export type VipActivationStatus = 'pending' | 'success' | 'failed' | 'disabled'

/**
 * VVIP activation order and audit record
 */
export interface VipActivationRecord {
  /** Record ID */
  id: number
  /** Activated user ID */
  user_id: number
  /** Payment order number */
  trade_no: string
  /** Fixed activation amount */
  activation_amount: number
  /** Actual paid amount */
  paid_amount: number
  /** Fixed discount snapshot */
  discount: number
  /** Payment provider */
  payment_provider: string
  /** Payment method */
  payment_method: string
  /** Activation status */
  status: VipActivationStatus
  /** Activation timestamp */
  activated_at: number
  /** Disabled timestamp */
  disabled_at?: number
  /** Admin user ID who disabled VVIP */
  disabled_by?: number
  /** Disable reason */
  disable_reason?: string
  /** Creation timestamp */
  created_at: number
  /** Update timestamp */
  updated_at: number
}

/**
 * VVIP activation records page
 */
export interface VipActivationRecordsPage {
  page: number
  page_size: number
  total: number
  items: VipActivationRecord[]
}

/**
 * Preset amount option with optional discount
 */
export interface PresetAmount {
  /** Preset amount value */
  value: number
  /** Optional discount rate (0-1) */
  discount?: number
}

/**
 * Redemption code request
 */
export interface RedemptionRequest {
  /** Redemption code key */
  key: string
}

/**
 * Payment request parameters
 */
export interface PaymentRequest {
  /** Topup amount */
  amount: number
  /** Payment method identifier */
  payment_method: string
}

/**
 * VVIP Epay payment request
 */
export interface VipActivationEpayRequest {
  /** Epay payment method such as alipay or wechat */
  payment_method: string
}

/**
 * VVIP direct Alipay payment request
 */
export interface VipActivationAlipayRequest {
  /** Direct Alipay method identifier */
  payment_method?: 'alipay_direct'
}

/**
 * VVIP direct WeChat Pay payment request
 */
export interface VipActivationWechatPayRequest {
  /** Direct WeChat Pay method identifier */
  payment_method?: 'wechat_direct'
}

/**
 * VVIP Creem payment request
 */
export interface VipActivationCreemRequest {
  /** Creem product ID priced exactly as VVIP activation amount */
  product_id?: string
}

/**
 * VVIP Waffo payment request
 */
export interface VipActivationWaffoRequest {
  /** Optional Waffo payment method index */
  pay_method_index?: number
}

/**
 * Waffo payment request parameters
 */
export interface WaffoPaymentRequest {
  /** Topup amount */
  amount: number
  /** Optional server-side Waffo payment method index */
  pay_method_index?: number
}

/**
 * Waffo Pancake payment request parameters
 */
export interface WaffoPancakePaymentRequest {
  /** Topup amount */
  amount: number
}

/**
 * Amount calculation request
 */
export interface AmountRequest {
  /** Topup amount to calculate */
  amount: number
}

/**
 * Affiliate quota transfer request
 */
export interface AffiliateTransferRequest {
  /** Quota amount to transfer */
  quota: number
}

export interface CommissionTransferRequest {
  amount: number
}

export interface WithdrawSubmitRequest {
  amount: number
  fee_amount?: number
  receive_type: string
  receive_account: string
  remark?: string
}

export interface PiggyWithdrawSubmitRequest {
  amount: number
  remark?: string
}

export interface PiggyTaxTrialRequest {
  amount: number
}

export interface PiggyTaxTrialResult {
  outer_trade_no: string
  calc_month: string
  requested_amount?: string
  requested_amount_cents?: number
  platform_fee_rate?: number
  platform_fee_amount?: string
  platform_fee_amount_cents?: number
  piggy_tax_before_amount?: string
  piggy_tax_before_amount_cents?: number
  pretax_amount: string
  individual_tax_amount: string
  added_tax_amount: string
  after_tax_amount: string
  calc_type: string
}

export interface WithdrawalProfileInput {
  account_type: 'bankcard'
  real_name: string
  id_card_no: string
  mobile: string
  bank_card_no: string
  bank_name: string
}

export interface WithdrawalProfile {
  id: number
  user_id: number
  account_type: 'bankcard'
  real_name: string
  bank_name: string
  masked_id_card_no: string
  masked_mobile: string
  masked_bank_card_no: string
  piggy_sign_status: 'unsigned' | 'signed' | 'failed' | string
  piggy_signed_at: number
  piggy_sign_url_digest?: string
  piggy_contract_url?: string
  piggy_contract_document_id?: string
  piggy_contract_subsidiary_name?: string
  piggy_contract_position?: string
  piggy_contract_position_name?: string
  piggy_contract_tax_fund_id?: string
  created_at: number
  updated_at: number
}

export interface WithdrawalEligibility {
  enabled: boolean
  can_withdraw: boolean
  need_profile: boolean
  need_sign: boolean
  profile?: WithdrawalProfile | null
  withdrawable_commission: number
  frozen_commission: number
  commission_min_withdraw_amount: number
  cooldown_remaining_seconds: number
  disabled_reason: string
  blocking_reasons: string[]
}

export interface PiggySignUrlResponse {
  signed?: boolean
  sign_url?: string
}

export interface PiggyContractPreviewResponse {
  document_id: string
  preview_url: string
}

export interface WalletAccount {
  id: number
  user_id: number
  balance_amount: number
  commission_amount: number
  frozen_commission_amount: number
  total_commission_amount: number
  total_withdraw_amount: number
  created_at: number
  updated_at: number
}

export interface WalletAccountPayload {
  account: WalletAccount
  commission_min_withdraw_amount: number
}

export interface WalletFlow {
  id: number
  user_id: number
  biz_no: string
  flow_type: string
  wallet_type: string
  direction: 'in' | 'out'
  amount: number
  balance_after: number
  commission_after: number
  frozen_commission_after: number
  remark: string
  created_at: number
}

export interface CommissionRecord {
  id: number
  beneficiary_user_id: number
  source_user_id: number
  source_user_label?: string
  source_order_no: string
  source_type: string
  level: number
  base_amount: number
  commission_rate: number
  amount: number
  qualification_status: string
  status: string
  error_message: string
  settled_at: number
  reversed_at: number
  reverse_reason: string
  created_at: number
}

export interface WithdrawOrder {
  id: number
  user_id: number
  withdraw_no: string
  amount: number
  fee_amount: number
  platform_fee_rate?: number
  platform_fee_amount_cents?: number
  actual_amount: number
  status: string
  provider?: string
  piggy_status?: string
  receive_type: string
  receive_account: string
  withdrawal_profile_id?: number
  account_name?: string
  bank_name?: string
  tax_before_amount_cents?: number
  frozen_amount_cents?: number
  piggy_pay_amount_cents?: number
  piggy_pretax_amount_cents?: number
  piggy_individual_tax_cents?: number
  piggy_added_tax_cents?: number
  piggy_after_tax_amount_cents?: number
  piggy_fee_amount_cents?: number
  piggy_pay_amount?: string
  external_trade_no?: string
  front_log_no?: string
  labor_order_no?: string
  notify_type?: string
  trade_status?: string
  trade_fail_code?: string
  trade_result?: string
  trade_result_describe?: string
  tax_fund_id?: string
  position_name?: string
  position?: string
  calc_type?: string
  bank_remark?: string
  manual_review_reason?: string
  manual_handled_by?: number
  manual_handled_at?: number
  manual_handle_result?: string
  compensation_status?: string
  submitted_at?: number
  confirmed_at?: number
  terminal_at?: number
  payment_voucher: string
  fail_reason: string
  remark: string
  created_at: number
  reviewed_at: number
  paid_at: number
}

export interface PageResponse<T> {
  items: T[]
  total: number
}

export interface VipSubordinate {
  relation_id: number
  child_user_id: number
  username: string
  display_name: string
  status: number
  group: string
  quota: number
  used_quota: number
  bind_time: number
  topup_discount: number
}

export interface VipSubordinatesPage extends PageResponse<VipSubordinate> {
  page?: number
  page_size?: number
  parent_topup_discount: number
  can_set_subordinate_discount: boolean
  min_subordinate_topup_discount: number
}

export interface UpdateVipSubordinateDiscountRequest {
  topup_discount: number
}

/**
 * User wallet data
 */
export interface UserWalletData {
  /** User ID */
  id: number
  /** Username */
  username: string
  /** Current quota balance */
  quota: number
  /** Total used quota */
  used_quota: number
  /** Total request count */
  request_count: number
  /** Affiliate quota (pending rewards) */
  aff_quota: number
  /** Total affiliate quota earned (historical) */
  aff_history_quota: number
  /** Number of successful affiliate invites */
  aff_count: number
  /** User group */
  group: string
}

/**
 * Topup record status
 */
export type TopupStatus = 'success' | 'pending' | 'failed' | 'expired'

/**
 * Topup billing record
 */
export interface TopupRecord {
  /** Record ID */
  id: number
  /** User ID */
  user_id: number
  /** Topup amount (quota) */
  amount: number
  /** Payment amount (actual money paid) */
  money: number
  /** Trade/order number */
  trade_no: string
  /** Payment method type */
  payment_method: string
  /** Creation timestamp */
  create_time: number
  /** Completion timestamp */
  complete_time?: number
  /** Payment status */
  status: TopupStatus
}

/**
 * Billing history response
 */
export interface BillingHistoryResponse {
  items: TopupRecord[]
  total: number
}

/**
 * Complete order request (admin only)
 */
export interface CompleteOrderRequest {
  trade_no: string
}
