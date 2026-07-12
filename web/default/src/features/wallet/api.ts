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
  RedemptionRequest,
  PaymentRequest,
  AmountRequest,
  AffiliateTransferRequest,
  ApiResponse,
  TopupInfoResponse,
  RedemptionResponse,
  AmountResponse,
  PaymentResponse,
  StripePaymentResponse,
  AlipayPaymentResponse,
  WechatPayPaymentResponse,
  AffiliateCodeResponse,
  AffiliateTransferResponse,
  BillingHistoryResponse,
  CompleteOrderRequest,
  TopupRecord,
  TopupStatus,
  QrPaymentStatus,
  CreemPaymentRequest,
  CreemPaymentResponse,
  WaffoPaymentRequest,
  WaffoPaymentResponse,
  WaffoPancakePaymentRequest,
  WaffoPancakePaymentResponse,
  VipActivationInfoResponse,
  VipActivationStatus,
  VipActivationEpayRequest,
  VipActivationEpayResponse,
  VipActivationAlipayRequest,
  VipActivationAlipayResponse,
  VipActivationWechatPayRequest,
  VipActivationWechatPayResponse,
  VipActivationStripeResponse,
  VipActivationCreemRequest,
  VipActivationCreemResponse,
  VipActivationWaffoRequest,
  VipActivationWaffoResponse,
  VipActivationRecordsResponse,
  WalletAccountPayload,
  WalletFlow,
  CommissionRecord,
  WithdrawOrder,
  PageResponse,
  CommissionTransferRequest,
  PiggyContractPreviewResponse,
  PiggySignUrlResponse,
  PiggyTaxTrialRequest,
  PiggyTaxTrialResult,
  PiggyWithdrawSubmitRequest,
  WithdrawalEligibility,
  WithdrawalProfile,
  WithdrawalProfileInput,
  WithdrawSubmitRequest,
  UpdateVipSubordinateDiscountRequest,
  VipSubordinatesPage,
} from './types'

// ============================================================================
// Wallet API Functions
// ============================================================================

export const walletPiggyContractPreviewPath =
  '/api/wallet/withdraw/piggy/contract-preview'

/**
 * Check if API response is successful
 */
export function isApiSuccess(response: ApiResponse): boolean {
  return response.success === true || response.message === 'success'
}

/**
 * Get topup configuration info
 */
export async function getTopupInfo(): Promise<TopupInfoResponse> {
  const res = await api.get('/api/user/topup/info')
  return res.data
}

/**
 * Redeem a topup code
 */
export async function redeemTopupCode(
  request: RedemptionRequest
): Promise<RedemptionResponse> {
  const res = await api.post('/api/user/topup', request)
  return res.data
}

/**
 * Calculate payment amount for regular payment
 */
export async function calculateAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Calculate payment amount for Stripe payment
 */
export async function calculateStripeAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/stripe/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Calculate payment amount for direct Alipay payment
 */
export async function calculateAlipayAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/alipay/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Calculate payment amount for direct WeChat Pay payment
 */
export async function calculateWechatPayAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/wechat/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request regular payment
 */
export async function requestPayment(
  request: PaymentRequest
): Promise<PaymentResponse> {
  const res = await api.post('/api/user/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return {
    ...res.data,
    url: res.data.url || (res as unknown as { url?: string }).url,
  }
}

/**
 * Request Stripe payment
 */
export async function requestStripePayment(
  request: PaymentRequest
): Promise<StripePaymentResponse> {
  const res = await api.post('/api/user/stripe/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request direct Alipay payment
 */
export async function requestAlipayPayment(
  request: PaymentRequest
): Promise<AlipayPaymentResponse> {
  const res = await api.post('/api/user/alipay/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request direct WeChat Pay payment
 */
export async function requestWechatPayPayment(
  request: PaymentRequest
): Promise<WechatPayPaymentResponse> {
  const res = await api.post('/api/user/wechat/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request Creem payment
 */
export async function requestCreemPayment(
  request: CreemPaymentRequest
): Promise<CreemPaymentResponse> {
  const res = await api.post('/api/user/creem/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request Waffo payment
 */
export async function requestWaffoPayment(
  request: WaffoPaymentRequest
): Promise<WaffoPaymentResponse> {
  const res = await api.post('/api/user/waffo/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Calculate payment amount for Waffo Pancake payment
 */
export async function calculateWaffoPancakeAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/waffo-pancake/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request Waffo Pancake payment
 */
export async function requestWaffoPancakePayment(
  request: WaffoPancakePaymentRequest
): Promise<WaffoPancakePaymentResponse> {
  const res = await api.post('/api/user/waffo-pancake/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Get current VVIP activation state and payment options
 */
export async function getVipActivationInfo(): Promise<VipActivationInfoResponse> {
  const res = await api.get('/api/vip/info')
  return res.data
}

export function getQrPaymentStatusFromVipActivationOrderStatus(
  status: VipActivationStatus | undefined
): QrPaymentStatus | null {
  if (status === 'pending' || status === 'success' || status === 'failed') {
    return status
  }
  return null
}

export async function getVipActivationQrPaymentStatusByTradeNo(
  tradeNo: string
): Promise<QrPaymentStatus | null> {
  const normalizedTradeNo = tradeNo.trim()
  if (!normalizedTradeNo) {
    return null
  }
  const res = await api.get(
    `/api/vip/orders/${encodeURIComponent(normalizedTradeNo)}/status`
  )
  const response = res.data as ApiResponse<{
    trade_no: string
    status: VipActivationStatus
  }>
  if (!isApiSuccess(response)) {
    return null
  }
  if (response.data?.trade_no !== normalizedTradeNo) {
    return null
  }
  return getQrPaymentStatusFromVipActivationOrderStatus(response.data.status)
}

/**
 * Request VVIP Epay activation payment
 */
export async function requestVipActivationEpayPayment(
  request: VipActivationEpayRequest
): Promise<VipActivationEpayResponse> {
  const res = await api.post('/api/vip/epay/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return {
    ...res.data,
    url: res.data.url || (res as unknown as { url?: string }).url,
  }
}

/**
 * Request VVIP direct Alipay activation payment
 */
export async function requestVipActivationAlipayPayment(
  request: VipActivationAlipayRequest = {}
): Promise<VipActivationAlipayResponse> {
  const res = await api.post('/api/vip/alipay/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request VVIP direct WeChat Pay activation payment
 */
export async function requestVipActivationWechatPayPayment(
  request: VipActivationWechatPayRequest = {}
): Promise<VipActivationWechatPayResponse> {
  const res = await api.post('/api/vip/wechat/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request VVIP Stripe activation payment
 */
export async function requestVipActivationStripePayment(): Promise<VipActivationStripeResponse> {
  const res = await api.post('/api/vip/stripe/pay', {}, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request VVIP Creem activation payment
 */
export async function requestVipActivationCreemPayment(
  request: VipActivationCreemRequest
): Promise<VipActivationCreemResponse> {
  const res = await api.post('/api/vip/creem/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request VVIP Waffo activation payment
 */
export async function requestVipActivationWaffoPayment(
  request: VipActivationWaffoRequest
): Promise<VipActivationWaffoResponse> {
  const res = await api.post('/api/vip/waffo/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Get VVIP activation records (admin only)
 */
export async function getVipActivationRecords(
  page: number,
  pageSize: number
): Promise<VipActivationRecordsResponse> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  const res = await api.get(`/api/vip/admin/records?${params.toString()}`)
  return res.data
}

/**
 * Disable user's active VVIP activation (admin only)
 */
export async function disableVipActivation(
  userId: number,
  reason?: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/vip/admin/users/${userId}/disable`, {
    reason: reason || '',
  })
  return res.data
}

/**
 * Get affiliate code
 */
export async function getAffiliateCode(): Promise<AffiliateCodeResponse> {
  const res = await api.get('/api/user/aff')
  return res.data
}

/**
 * Transfer affiliate quota to balance
 */
export async function transferAffiliateQuota(
  request: AffiliateTransferRequest
): Promise<AffiliateTransferResponse> {
  const res = await api.post('/api/user/aff_transfer', request)
  return res.data
}

export async function getWalletAccount(): Promise<
  ApiResponse<WalletAccountPayload>
> {
  const res = await api.get('/api/wallet/account')
  return res.data
}

export async function getWalletFlows(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageResponse<WalletFlow>>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  const res = await api.get(`/api/wallet/flows?${params.toString()}`)
  return res.data
}

export async function getWalletCommissions(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageResponse<CommissionRecord>>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  const res = await api.get(`/api/wallet/commissions?${params.toString()}`)
  return res.data
}

export async function transferWalletCommission(
  request: CommissionTransferRequest
): Promise<ApiResponse> {
  const res = await api.post('/api/wallet/commission/transfer', request)
  return res.data
}

export async function submitWalletWithdraw(
  request: WithdrawSubmitRequest
): Promise<ApiResponse<WithdrawOrder>> {
  const res = await api.post('/api/wallet/withdraw', request)
  return res.data
}

export async function getWalletWithdrawalProfile(): Promise<
  ApiResponse<WithdrawalProfile | null>
> {
  const res = await api.get('/api/wallet/withdraw/profile')
  return res.data
}

export async function saveWalletWithdrawalProfile(
  request: WithdrawalProfileInput
): Promise<ApiResponse<WithdrawalProfile>> {
  const res = await api.put('/api/wallet/withdraw/profile', request)
  return res.data
}

export async function getWalletWithdrawalEligibility(): Promise<
  ApiResponse<WithdrawalEligibility>
> {
  const res = await api.get('/api/wallet/withdraw/eligibility')
  return res.data
}

export async function getWalletPiggySignUrl(): Promise<
  ApiResponse<PiggySignUrlResponse>
> {
  const res = await api.post('/api/wallet/withdraw/piggy/sign-url', {})
  return res.data
}

export async function refreshWalletPiggyContractStatus(): Promise<
  ApiResponse<WithdrawalProfile>
> {
  const res = await api.post(
    '/api/wallet/withdraw/piggy/sign-status/refresh',
    {}
  )
  return res.data
}

export async function getWalletPiggyContractPreview(): Promise<
  ApiResponse<PiggyContractPreviewResponse>
> {
  const res = await api.post(walletPiggyContractPreviewPath, {})
  return res.data
}

export const walletPiggyTaxTrialPath = '/api/wallet/withdraw/piggy/tax-trial'

export async function trialPiggyWalletWithdrawTax(
  request: PiggyTaxTrialRequest
): Promise<ApiResponse<PiggyTaxTrialResult>> {
  const res = await api.post(walletPiggyTaxTrialPath, request)
  return res.data
}

export async function submitPiggyWalletWithdraw(
  request: PiggyWithdrawSubmitRequest
): Promise<ApiResponse<WithdrawOrder>> {
  const res = await api.post('/api/wallet/withdraw/piggy', request)
  return res.data
}

export async function getWalletWithdraws(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageResponse<WithdrawOrder>>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  const res = await api.get(`/api/wallet/withdraws?${params.toString()}`)
  return res.data
}

export async function getVipSubordinates(
  page: number,
  pageSize: number
): Promise<ApiResponse<VipSubordinatesPage>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  const res = await api.get(`/api/vip/subordinates?${params.toString()}`)
  return res.data
}

export async function updateVipSubordinateDiscount(
  childUserId: number,
  request: UpdateVipSubordinateDiscountRequest
): Promise<ApiResponse> {
  const res = await api.put(
    `/api/vip/subordinates/${childUserId}/discount`,
    request
  )
  return res.data
}

export async function resetVipSubordinateDiscount(
  childUserId: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/vip/subordinates/${childUserId}/discount`)
  return res.data
}

export async function getAdminWalletCommissions(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageResponse<CommissionRecord>>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  const res = await api.get(
    `/api/wallet/admin/commissions?${params.toString()}`
  )
  return res.data
}

export async function getAdminWalletWithdraws(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageResponse<WithdrawOrder>>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  const res = await api.get(`/api/wallet/admin/withdraws?${params.toString()}`)
  return res.data
}

export async function approveAdminWalletWithdraw(
  id: number
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/approve`, {})
  return res.data
}

export async function rejectAdminWalletWithdraw(
  id: number,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/reject`, {
    reason,
  })
  return res.data
}

export async function payAdminWalletWithdraw(
  id: number,
  paymentVoucher: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/pay`, {
    payment_voucher: paymentVoucher,
  })
  return res.data
}

export async function failAdminWalletWithdraw(
  id: number,
  reason: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/wallet/admin/withdraws/${id}/fail`, {
    reason,
  })
  return res.data
}

/**
 * Get billing history for current user
 */
export async function getUserBillingHistory(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<ApiResponse<BillingHistoryResponse>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  if (keyword) {
    params.append('keyword', keyword)
  }
  const res = await api.get(`/api/user/topup/self?${params.toString()}`)
  return res.data
}

export function findTopupStatusByTradeNo(
  records: TopupRecord[] | undefined,
  tradeNo: string
): TopupStatus | null {
  const normalizedTradeNo = tradeNo.trim()
  if (!normalizedTradeNo) {
    return null
  }
  return (
    records?.find((record) => record.trade_no === normalizedTradeNo)?.status ??
    null
  )
}

export async function getUserTopupStatusByTradeNo(
  tradeNo: string
): Promise<TopupStatus | null> {
  const normalizedTradeNo = tradeNo.trim()
  if (!normalizedTradeNo) {
    return null
  }
  const response = await getUserBillingHistory(1, 10, normalizedTradeNo)
  if (!isApiSuccess(response)) {
    return null
  }
  return findTopupStatusByTradeNo(response.data?.items, normalizedTradeNo)
}

/**
 * Get billing history for all users (admin only)
 */
export async function getAllBillingHistory(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<ApiResponse<BillingHistoryResponse>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  if (keyword) {
    params.append('keyword', keyword)
  }
  const res = await api.get(`/api/user/topup?${params.toString()}`)
  return res.data
}

/**
 * Complete a pending order (admin only)
 */
export async function completeOrder(
  request: CompleteOrderRequest
): Promise<ApiResponse> {
  const res = await api.post('/api/user/topup/complete', request)
  return res.data
}
