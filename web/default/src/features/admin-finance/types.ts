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

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

export interface PageResponse<T> {
  page?: number
  page_size?: number
  total: number
  items: T[]
}

export interface AdminRechargeRecordBaseFilters {
  userId?: string
  email?: string
  phoneNumber?: string
  tradeNo?: string
  status?: string
  paymentProvider?: string
  paymentMethod?: string
  createdFrom?: number
  createdTo?: number
}

export interface AdminTopUpRecordFilters extends AdminRechargeRecordBaseFilters {
  completedFrom?: number
  completedTo?: number
}

export interface AdminVipActivationRecordFilters extends AdminRechargeRecordBaseFilters {
  activatedFrom?: number
  activatedTo?: number
}

export interface AdminTopUpRecord {
  id: number
  user_id: number
  username: string
  display_name: string
  email: string
  phone_number: string
  amount: number
  money: number
  recharge_amount: number
  paid_amount: number
  discount: number
  trade_no: string
  payment_provider: string
  payment_method: string
  create_time: number
  complete_time: number
  reversed_at: number
  status: string
}

export interface VipActivationRecord {
  id: number
  user_id: number
  username: string
  display_name: string
  email: string
  phone_number: string
  trade_no: string
  activation_amount: number
  paid_amount: number
  discount: number
  payment_provider: string
  payment_method: string
  status: string
  activated_at: number
  disabled_at?: number
  disabled_by?: number
  disable_reason?: string
  created_at: number
  updated_at: number
}

export interface UserRelation {
  id: number
  parent_user_id: number
  child_user_id: number
  source: string
  source_trade_no: string
  status: string
  bind_time: number
  created_at: number
  updated_at: number
}

export interface CreateUserRelationRequest {
  parent_user_id: number
  child_user_id: number
  source_trade_no?: string
  remark?: string
}

export interface CommissionRecord {
  id: number
  beneficiary_user_id: number
  source_user_id: number
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
  platform_fee_rate: number
  platform_fee_amount_cents: number
  actual_amount: number
  status: string
  provider: string
  piggy_status: string
  receive_type: string
  receive_account: string
  withdrawal_profile_id: number
  account_name: string
  bank_name: string
  tax_before_amount_cents: number
  frozen_amount_cents: number
  piggy_pay_amount_cents: number
  piggy_pretax_amount_cents: number
  piggy_individual_tax_cents: number
  piggy_added_tax_cents: number
  piggy_after_tax_amount_cents: number
  piggy_fee_amount_cents: number
  piggy_pay_amount: string
  external_trade_no: string
  front_log_no: string
  labor_order_no: string
  notify_type: string
  trade_status: string
  trade_fail_code: string
  trade_result: string
  trade_result_describe: string
  tax_fund_id: string
  position_name: string
  position: string
  calc_type: string
  bank_remark: string
  manual_review_reason: string
  manual_handled_by: number
  manual_handled_at: number
  manual_handle_result: string
  compensation_status: string
  submitted_at: number
  confirmed_at: number
  terminal_at: number
  payment_voucher: string
  fail_reason: string
  remark: string
  created_at: number
  reviewed_at: number
  paid_at: number
}

export interface WithdrawApprovalResult {
  submitted: boolean
  recoverable: boolean
  status: string
  message: string
}

export interface PiggyWithdrawCallbackLog {
  id: number
  callback_type: string
  order_no: string
  user_id: number
  notify_type: string
  trade_status: string
  front_log_no: string
  labor_order_no: string
  idempotency_key: string
  payload_digest: string
  decrypted_digest: string
  process_status: string
  error_message: string
  created_at: number
  updated_at: number
}

export interface PaymentCallbackLog {
  id: number
  provider: string
  event_type: string
  trade_no: string
  biz_type: string
  verify_status: boolean
  process_status: string
  payload_digest: string
  error_message: string
  created_at: number
  updated_at: number
}

export interface PaymentReconciliationTask {
  id: number
  provider: string
  date_from: number
  date_to: number
  status: string
  total_count: number
  diff_count: number
  error_message: string
  created_at: number
  updated_at: number
}

export interface PaymentReconciliationDiff {
  trade_no: string
  biz_type: string
  diff_type: string
  local_status: string
  provider_status: string
  local_paid_amount: number
  provider_paid_amount: number
}

export interface ProviderPaymentOrder {
  trade_no: string
  biz_type: string
  paid_amount: number
  status: string
}

export interface CreateReconciliationTaskRequest {
  provider: string
  date_from: number
  date_to: number
  orders: ProviderPaymentOrder[]
}

export interface CreateReconciliationTaskResponse {
  task: PaymentReconciliationTask
  diffs: PaymentReconciliationDiff[]
}

export type ReverseOrderType = 'topup' | 'vip_activation'

export interface QiniuKeyOwner {
  id: number
  username: string
  display_name: string
  email: string
}

export interface QiniuKeyQuotaSummary {
  applied_limit_amount: number
  pending_limit_amount: number
  failed_limit_amount: number
  latest_grant_error: string
}

export interface QiniuKeyTaskSummary {
  id: number
  task_type: string
  status: string
  retry_count: number
  next_retry_time: number
  last_error: string
  updated_time: number
}

export interface QiniuKeyListItem {
  token_id: number
  user_id: number
  name: string
  key: string
  status: number
  group: string
  qiniu_child_account_id: number
  qiniu_child_account?: {
    id: number
    email: string
    uid: string
    status: string
  } | null
  created_time: number
  accessed_time: number
  deleted: boolean
  deleted_time: number
  user: QiniuKeyOwner
  quota: QiniuKeyQuotaSummary
  latest_task?: QiniuKeyTaskSummary | null
}

export interface QiniuKeyDisableResult {
  token_id: number
  user_id: number
  status: number
  key: string
}

export interface QiniuChildAccountTask {
  id: number
  account_id: number
  task_type: string
  status: string
  retry_count: number
  next_retry_time: number
  last_error: string
  created_by: number
  created_time: number
  updated_time: number
  started_time: number
  completed_time: number
}

export interface QiniuChildAccount {
  id: number
  sequence_no: number
  email: string
  remote_user_id: string
  uid: string
  parent_uid: string
  access_key: string
  backup_access_key: string
  key_state: string
  backup_key_state: string
  status: string
  last_error: string
  user_count: number
  latest_task?: QiniuChildAccountTask | null
  created_by: number
  disabled_by: number
  disabled_reason: string
  impact?: QiniuChildAccountImpact
  created_time: number
  updated_time: number
  disabled_time: number
}

export interface QiniuChildAccountImpact {
  associated_user_count: number
  associated_token_count: number
  enabled_token_count: number
}

export interface QiniuChildAccountTokenSummary {
  id: number
  user_id: number
  username: string
  display_name: string
  email: string
  name: string
  status: number
  qiniu_child_account_id: number
  key_fingerprint: string
  remote_cleanup_result: string
  created_time: number
  accessed_time: number
  expired_time: number
  deleted: boolean
  deleted_time: number
}

export interface QiniuChildAccountDetail extends QiniuChildAccount {
  tasks: QiniuChildAccountTask[]
  users: Array<{
    id: number
    username: string
    display_name: string
    email: string
  }>
  tokens: QiniuChildAccountTokenSummary[]
  impact: QiniuChildAccountImpact
}

export interface QiniuBillingBucket {
  id: number
  user_id: number
  token_id: number
  qiniu_child_account_id: number
  billing_date: string
  qiniu_masked_key: string
  key_fingerprint: string
  owner_status: string
  official_amount: number
  official_quota: number
  previous_official_amount: number
  previous_official_quota: number
  local_realtime_quota: number
  local_realtime_status: string
  applied_delta_quota: number
  pending_delta_quota: number
  apply_version: number
  status: string
  last_error: string
  retry_count: number
  last_retry_time: number
  next_retry_time: number
  created_time: number
  updated_time: number
}

export interface QiniuBillingSummary {
  unmapped_count: number
  ambiguous_count: number
  failed_application_count: number
  affected_quota: number
  affected_amount: number
  latest_error: string
  latest_successful_sync_time: number
  latest_retry_result: string
}

export interface QiniuCostDetailRecord {
  id: number
  qiniu_masked_key: string
  key_prefix: string
  key_suffix: string
  billing_date: string
  model_name: string
  billing_item: string
  usage_count: number
  usage_unit: string
  fee_amount: number
  currency: string
  record_hash: string
  raw_response: string
  owner_status: string
  user_id: number
  token_id: number
  qiniu_child_account_id: number
  retry_count: number
  last_retry_time: number
  next_retry_time: number
  last_error: string
  created_time: number
  updated_time: number
}

export interface QiniuBillingBucketItem {
  id: number
  bucket_id: number
  model_name: string
  billing_item: string
  usage_count: number
  fee_amount: number
  currency: string
  raw_record_ids: string
  created_time: number
  updated_time: number
}

export interface QiniuBillingBucketApplication {
  id: number
  bucket_id: number
  apply_version: number
  delta_quota: number
  delta_amount: number
  wallet_flow_id: number
  consume_log_id: number
  idempotency_key: string
  balance_before_quota: number
  balance_after_quota: number
  debt_quota: number
  status: string
  last_error: string
  retry_count: number
  last_retry_time: number
  next_retry_time: number
  operation_source: string
  created_time: number
  updated_time: number
}

export interface QiniuBillingBucketOperationResult {
  action: string
  bucket_id: number
  raw_record_id?: number
  application_id?: number
  token_id?: number
  billing_date?: string
  status?: string
  applied: boolean
  skipped: boolean
  message: string
}
