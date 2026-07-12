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
import { api } from '@/lib/api'
import type {
  AdminTopUpRecord,
  AdminTopUpRecordFilters,
  AdminVipActivationRecordFilters,
  ApiResponse,
  CommissionRecord,
  CreateReconciliationTaskRequest,
  CreateReconciliationTaskResponse,
  CreateUserRelationRequest,
  PageResponse,
  PaymentCallbackLog,
  PaymentReconciliationTask,
  PiggyWithdrawCallbackLog,
  QiniuBillingBucket,
  QiniuBillingBucketApplication,
  QiniuBillingBucketItem,
  QiniuBillingBucketOperationResult,
  QiniuBillingSummary,
  QiniuChildAccount,
  QiniuChildAccountDetail,
  QiniuChildAccountTask,
  QiniuCostDetailRecord,
  QiniuKeyDisableResult,
  QiniuKeyListItem,
  UserRelation,
  VipActivationRecord,
  WithdrawApprovalResult,
  WithdrawOrder,
} from './types'

type QueryValue = string | number | boolean | undefined | null

function buildQuery(params: Record<string, QueryValue>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    query.set(key, String(value))
  }
  const text = query.toString()
  return text ? `?${text}` : ''
}

export async function getAdminVipActivationRecords(
  page: number,
  pageSize: number,
  filters?: AdminVipActivationRecordFilters
): Promise<ApiResponse<PageResponse<VipActivationRecord>>> {
  const res = await api.get(
    buildAdminVipActivationRecordsPath(page, pageSize, filters)
  )
  return res.data
}

export function buildAdminTopUpRecordsPath(
  page: number,
  pageSize: number,
  filters: AdminTopUpRecordFilters = {}
) {
  return `/api/user/topup${buildQuery({
    p: page,
    page_size: pageSize,
    user_id: filters.userId,
    email: filters.email,
    phone_number: filters.phoneNumber,
    trade_no: filters.tradeNo,
    status: filters.status,
    payment_provider: filters.paymentProvider,
    payment_method: filters.paymentMethod,
    created_from: filters.createdFrom,
    created_to: filters.createdTo,
    completed_from: filters.completedFrom,
    completed_to: filters.completedTo,
  })}`
}

export function buildAdminVipActivationRecordsPath(
  page: number,
  pageSize: number,
  filters: AdminVipActivationRecordFilters = {}
) {
  return `/api/vip/admin/records${buildQuery({
    p: page,
    page_size: pageSize,
    user_id: filters.userId,
    email: filters.email,
    phone_number: filters.phoneNumber,
    trade_no: filters.tradeNo,
    status: filters.status,
    payment_provider: filters.paymentProvider,
    payment_method: filters.paymentMethod,
    created_from: filters.createdFrom,
    created_to: filters.createdTo,
    activated_from: filters.activatedFrom,
    activated_to: filters.activatedTo,
  })}`
}

export async function getAdminTopUpRecords(
  page: number,
  pageSize: number,
  filters?: AdminTopUpRecordFilters
): Promise<ApiResponse<PageResponse<AdminTopUpRecord>>> {
  const res = await api.get(buildAdminTopUpRecordsPath(page, pageSize, filters))
  return res.data
}

export async function disableAdminVipActivation(
  userId: number,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/vip/admin/users/${userId}/disable`, {
    reason,
  })
  return res.data
}

export async function getAdminUserRelations(params: {
  page: number
  pageSize: number
  parentUserId?: string
  childUserId?: string
  status?: string
}): Promise<ApiResponse<PageResponse<UserRelation>>> {
  const res = await api.get(
    `/api/vip/admin/relations${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      parent_user_id: params.parentUserId,
      child_user_id: params.childUserId,
      status: params.status,
    })}`
  )
  return res.data
}

export async function createAdminUserRelation(
  request: CreateUserRelationRequest
): Promise<ApiResponse<UserRelation>> {
  const res = await api.post('/api/vip/admin/relations', request)
  return res.data
}

export async function disableAdminUserRelation(
  id: number,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/vip/admin/relations/${id}/disable`, {
    reason,
  })
  return res.data
}

export async function getAdminCommissions(params: {
  page: number
  pageSize: number
  userId?: string
  status?: string
}): Promise<ApiResponse<PageResponse<CommissionRecord>>> {
  const res = await api.get(
    `/api/wallet/admin/commissions${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      user_id: params.userId,
      status: params.status,
    })}`
  )
  return res.data
}

export async function getAdminWithdraws(params: {
  page: number
  pageSize: number
  userId?: string
  status?: string
}): Promise<ApiResponse<PageResponse<WithdrawOrder>>> {
  const res = await api.get(
    `/api/wallet/admin/withdraws${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      user_id: params.userId,
      status: params.status,
    })}`
  )
  return res.data
}

export async function approveAdminWithdraw(
  id: number
): Promise<ApiResponse<WithdrawApprovalResult>> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/approve`, {})
  return res.data
}

export async function rejectAdminWithdraw(
  id: number,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/reject`, {
    reason,
  })
  return res.data
}

export async function payAdminWithdraw(
  id: number,
  paymentVoucher: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/pay`, {
    payment_voucher: paymentVoucher,
  })
  return res.data
}

export async function failAdminWithdraw(
  id: number,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/fail`, {
    reason,
  })
  return res.data
}

export async function getPiggyWithdrawCallbackLogs(params: {
  page: number
  pageSize: number
  callbackType?: string
  orderNo?: string
  processStatus?: string
}): Promise<ApiResponse<PageResponse<PiggyWithdrawCallbackLog>>> {
  const res = await api.get(
    `/api/wallet/admin/withdraw-callbacks${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      callback_type: params.callbackType,
      order_no: params.orderNo,
      process_status: params.processStatus,
    })}`
  )
  return res.data
}

export async function retryPiggyWithdrawConfirm(
  id: number
): Promise<ApiResponse> {
  const res = await api.post(
    `/api/wallet/admin/withdraws/${id}/piggy/retry-confirm`,
    {}
  )
  return res.data
}

export async function recoverPiggyWithdrawSubmit(
  id: number
): Promise<ApiResponse<WithdrawApprovalResult>> {
  const res = await api.post(
    `/api/wallet/admin/withdraws/${id}/piggy/recover-submit`,
    {}
  )
  return res.data
}

export async function cancelPiggyWithdraw(
  id: number,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/piggy/cancel`, {
    reason,
  })
  return res.data
}

export async function recordPiggyWithdrawManualResult(
  id: number,
  result: string,
  compensationStatus: string
): Promise<ApiResponse> {
  const res = await api.post(
    `/api/wallet/admin/withdraws/${id}/piggy/manual-result`,
    {
      result,
      compensation_status: compensationStatus,
    }
  )
  return res.data
}

export async function scanPiggyWithdrawCompensations(
  limit?: number
): Promise<ApiResponse<{ processed: number }>> {
  const res = await api.post(
    `/api/wallet/admin/piggy/withdraws/scan${buildQuery({ limit })}`,
    {}
  )
  return res.data
}

export async function getPaymentCallbackLogs(params: {
  page: number
  pageSize: number
  provider?: string
  tradeNo?: string
  processStatus?: string
}): Promise<ApiResponse<PageResponse<PaymentCallbackLog>>> {
  const res = await api.get(
    `/api/payment/admin/callback-logs${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      provider: params.provider,
      trade_no: params.tradeNo,
      process_status: params.processStatus,
    })}`
  )
  return res.data
}

export async function getPaymentReconciliationTasks(params: {
  page: number
  pageSize: number
  provider?: string
  status?: string
}): Promise<ApiResponse<PageResponse<PaymentReconciliationTask>>> {
  const res = await api.get(
    `/api/payment/admin/reconciliation/tasks${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      provider: params.provider,
      status: params.status,
    })}`
  )
  return res.data
}

export async function createPaymentReconciliationTask(
  request: CreateReconciliationTaskRequest
): Promise<ApiResponse<CreateReconciliationTaskResponse>> {
  const res = await api.post('/api/payment/admin/reconciliation/tasks', request)
  return res.data
}

export async function reverseTopupOrder(
  tradeNo: string,
  provider: string,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(
    `/api/payment/admin/topups/${encodeURIComponent(tradeNo)}/reverse`,
    {
      provider,
      reason,
    }
  )
  return res.data
}

export async function reverseVipActivationOrder(
  tradeNo: string,
  provider: string,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(
    `/api/payment/admin/vip-activations/${encodeURIComponent(tradeNo)}/reverse`,
    { provider, reason }
  )
  return res.data
}

export async function getQiniuKeys(params: {
  page: number
  pageSize: number
  userId?: string
  tokenId?: string
  childAccountId?: string
  status?: string
  keyFragment?: string
  includeDeleted?: boolean
}): Promise<ApiResponse<PageResponse<QiniuKeyListItem>>> {
  const res = await api.get(buildQiniuKeysPath(params))
  return res.data
}

export function buildQiniuKeysPath(params: {
  page: number
  pageSize: number
  userId?: string
  tokenId?: string
  childAccountId?: string
  status?: string
  keyFragment?: string
  includeDeleted?: boolean
}) {
  return `/api/payment/admin/qiniu-keys${buildQuery({
    p: params.page,
    page_size: params.pageSize,
    user_id: params.userId,
    token_id: params.tokenId,
    qiniu_child_account_id: params.childAccountId,
    status: params.status,
    qiniu_key: params.keyFragment,
    include_deleted: params.includeDeleted ? true : undefined,
  })}`
}

export async function disableQiniuKey(
  tokenId: number,
  reason?: string
): Promise<ApiResponse<QiniuKeyDisableResult>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-keys/${encodeURIComponent(String(tokenId))}/disable`,
    { reason: reason?.trim() || undefined }
  )
  return res.data
}

export async function getQiniuChildAccounts(params: {
  page: number
  pageSize: number
  id?: string
  email?: string
  uid?: string
  status?: string
}): Promise<ApiResponse<PageResponse<QiniuChildAccount>>> {
  const res = await api.get(
    `/api/payment/admin/qiniu-child-accounts${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      id: params.id,
      email: params.email,
      uid: params.uid,
      status: params.status,
    })}`
  )
  return res.data
}

export async function createQiniuChildAccount(): Promise<
  ApiResponse<QiniuChildAccount>
> {
  const res = await api.post('/api/payment/admin/qiniu-child-accounts', {})
  return res.data
}

export async function getQiniuChildAccountDetail(
  accountId: number
): Promise<ApiResponse<QiniuChildAccountDetail>> {
  const res = await api.get(
    `/api/payment/admin/qiniu-child-accounts/${encodeURIComponent(String(accountId))}`
  )
  return res.data
}

export async function disableQiniuChildAccount(
  accountId: number,
  reason: string
): Promise<ApiResponse<QiniuChildAccount>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-child-accounts/${encodeURIComponent(String(accountId))}/disable`,
    { reason }
  )
  return res.data
}

export async function enableQiniuChildAccount(
  accountId: number
): Promise<ApiResponse<QiniuChildAccount>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-child-accounts/${encodeURIComponent(String(accountId))}/enable`,
    {}
  )
  return res.data
}

export async function getQiniuChildAccountTasks(params: {
  page: number
  pageSize: number
  accountId?: string
  taskType?: string
  status?: string
}): Promise<ApiResponse<PageResponse<QiniuChildAccountTask>>> {
  const res = await api.get(
    `/api/payment/admin/qiniu-child-account-tasks${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      account_id: params.accountId,
      task_type: params.taskType,
      status: params.status,
    })}`
  )
  return res.data
}

export async function retryQiniuChildAccountTask(
  taskId: number
): Promise<ApiResponse<QiniuChildAccountTask>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-child-account-tasks/${encodeURIComponent(String(taskId))}/retry`,
    {}
  )
  return res.data
}

export function buildQiniuBillingSummaryPath() {
  return '/api/payment/admin/qiniu-billing-summary'
}

export async function getQiniuBillingSummary(): Promise<
  ApiResponse<QiniuBillingSummary>
> {
  const res = await api.get(buildQiniuBillingSummaryPath())
  return res.data
}

export async function getQiniuBillingBuckets(params: {
  page: number
  pageSize: number
  billingDate?: string
  userId?: string
  tokenId?: string
  childAccountId?: string
  status?: string
  ownerStatus?: string
  maskedKey?: string
}): Promise<ApiResponse<PageResponse<QiniuBillingBucket>>> {
  const res = await api.get(
    `/api/payment/admin/qiniu-billing-buckets${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      billing_date: params.billingDate,
      user_id: params.userId,
      token_id: params.tokenId,
      qiniu_child_account_id: params.childAccountId,
      status: params.status,
      owner_status: params.ownerStatus,
      qiniu_masked_key: params.maskedKey,
    })}`
  )
  return res.data
}

export async function getQiniuCostDetailRecords(params: {
  page: number
  pageSize: number
  billingDate?: string
  userId?: string
  tokenId?: string
  childAccountId?: string
  ownerStatus?: string
  maskedKey?: string
  model?: string
  billingItem?: string
}): Promise<ApiResponse<PageResponse<QiniuCostDetailRecord>>> {
  const res = await api.get(
    `/api/payment/admin/qiniu-cost-detail-records${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      billing_date: params.billingDate,
      user_id: params.userId,
      token_id: params.tokenId,
      qiniu_child_account_id: params.childAccountId,
      owner_status: params.ownerStatus,
      qiniu_masked_key: params.maskedKey,
      model: params.model,
      billing_item: params.billingItem,
    })}`
  )
  return res.data
}

export async function getQiniuBillingBucketItems(params: {
  page: number
  pageSize: number
  bucketId?: number
}): Promise<ApiResponse<PageResponse<QiniuBillingBucketItem>>> {
  const res = await api.get(
    `/api/payment/admin/qiniu-billing-bucket-items${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      bucket_id: params.bucketId,
    })}`
  )
  return res.data
}

export async function getQiniuBillingBucketApplications(params: {
  page: number
  pageSize: number
  bucketId?: number
  status?: string
}): Promise<ApiResponse<PageResponse<QiniuBillingBucketApplication>>> {
  const res = await api.get(
    `/api/payment/admin/qiniu-billing-bucket-applications${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      bucket_id: params.bucketId,
      status: params.status,
    })}`
  )
  return res.data
}

export async function recalculateQiniuBillingBucket(
  bucketId: number,
  reason: string
): Promise<ApiResponse<QiniuBillingBucketOperationResult>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-billing-buckets/${bucketId}/recalculate`,
    { reason }
  )
  return res.data
}

export async function skipQiniuBillingBucket(
  bucketId: number,
  reason: string
): Promise<ApiResponse<QiniuBillingBucketOperationResult>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-billing-buckets/${bucketId}/skip`,
    { reason }
  )
  return res.data
}

export async function resolveQiniuBillingBucket(
  bucketId: number,
  tokenId: number,
  reason: string
): Promise<ApiResponse<QiniuBillingBucketOperationResult>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-billing-buckets/${bucketId}/resolve`,
    {
      token_id: tokenId,
      reason,
    }
  )
  return res.data
}

export async function resolveQiniuCostDetailRecord(
  rawRecordId: number,
  tokenId: number,
  reason: string
): Promise<ApiResponse<QiniuBillingBucketOperationResult>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-cost-detail-records/${rawRecordId}/resolve`,
    {
      token_id: tokenId,
      reason,
    }
  )
  return res.data
}

export async function retryQiniuBillingBucketApplication(
  applicationId: number,
  reason: string
): Promise<ApiResponse<QiniuBillingBucketOperationResult>> {
  const res = await api.post(
    `/api/payment/admin/qiniu-billing-bucket-applications/${applicationId}/retry`,
    { reason }
  )
  return res.data
}
