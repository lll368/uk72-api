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
import {
  PAYMENT_TYPES,
  DEFAULT_PRESET_MULTIPLIERS,
  DEFAULT_PAYMENT_TYPE,
  DEFAULT_MIN_TOPUP,
} from '../constants'
import type { PaymentMethod, PresetAmount, TopupInfo } from '../types'

// ============================================================================
// Payment Processing Functions
// ============================================================================

export type WalletPaymentDispatch =
  | 'stripe'
  | 'alipay_direct'
  | 'wechat_direct'
  | 'waffo_pancake'
  | 'standard_form'

/**
 * Check if browser is Safari
 */
function isSafariBrowser(): boolean {
  return (
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1
  )
}

/**
 * Submit payment form (for non-Stripe payments)
 */
export function submitPaymentForm(
  url: string,
  params: Record<string, unknown>,
  options: { target?: '_blank' | '_self' } = {}
): void {
  const form = document.createElement('form')
  form.action = url
  form.method = 'POST'

  const target =
    options.target ?? (!isSafariBrowser() ? ('_blank' as const) : undefined)
  if (target) {
    form.target = target
  }

  // Add form parameters
  Object.entries(params).forEach(([key, value]) => {
    const input = document.createElement('input')
    input.type = 'hidden'
    input.name = key
    input.value = String(value)
    form.appendChild(input)
  })

  document.body.appendChild(form)
  form.submit()
  document.body.removeChild(form)
}

/**
 * Check if payment method is Stripe
 */
export function isStripePayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.STRIPE
}

/**
 * Check if payment method is official direct Alipay.
 *
 * This must stay separate from epay's `alipay` method because both can be
 * displayed at the same time and they use different backend routes.
 */
export function isAlipayDirectPayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.ALIPAY_DIRECT
}

/**
 * Check if payment method is official direct WeChat Pay.
 *
 * This is independent from epay's `wxpay` method and uses a WeChat Native QR
 * order created by the backend.
 */
export function isWechatDirectPayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.WECHAT_DIRECT
}

export function isHiddenEpayPayment(paymentType: string | undefined): boolean {
  return (
    paymentType === PAYMENT_TYPES.ALIPAY || paymentType === PAYMENT_TYPES.WECHAT
  )
}

/**
 * Check if payment method is Creem
 */
export function isCreemPayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.CREEM
}

/**
 * Check if payment method is Waffo
 */
export function isWaffoPayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.WAFFO
}

/**
 * Check if payment method is Waffo Pancake
 *
 * Pancake is a metered-style payment that goes through a dedicated checkout
 * URL flow rather than the generic epay form submission, so it must be
 * special-cased in payment dispatch logic.
 */
export function isWaffoPancakePayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.WAFFO_PANCAKE
}

export function filterVisiblePaymentMethods(
  methods: PaymentMethod[] = []
): PaymentMethod[] {
  return methods.filter((method) => !isHiddenEpayPayment(method?.type))
}

export function isSubscriptionEpayPaymentMethod(
  method: Pick<PaymentMethod, 'type'>
): boolean {
  const paymentType = method.type
  return (
    !!paymentType &&
    !isHiddenEpayPayment(paymentType) &&
    !isStripePayment(paymentType) &&
    !isCreemPayment(paymentType) &&
    !isAlipayDirectPayment(paymentType) &&
    !isWechatDirectPayment(paymentType) &&
    !isWaffoPayment(paymentType) &&
    !isWaffoPancakePayment(paymentType)
  )
}

export function getWalletPaymentDispatch(
  paymentType: string
): WalletPaymentDispatch {
  if (isStripePayment(paymentType)) {
    return 'stripe'
  }
  if (isAlipayDirectPayment(paymentType)) {
    return 'alipay_direct'
  }
  if (isWechatDirectPayment(paymentType)) {
    return 'wechat_direct'
  }
  if (isWaffoPancakePayment(paymentType)) {
    return 'waffo_pancake'
  }
  return 'standard_form'
}

export function getPaymentMethodDisplayName(
  method: Pick<PaymentMethod, 'name' | 'type'>,
  t?: (key: string) => string
): string {
  if (isAlipayDirectPayment(method.type)) {
    return t ? t('Alipay') : 'Alipay'
  }
  if (isWechatDirectPayment(method.type)) {
    return t ? t('WeChat') : 'WeChat'
  }
  return method.name || method.type
}

/**
 * Get default payment type from topup info
 */
export function getDefaultPaymentType(topupInfo: TopupInfo | null): string {
  if (!topupInfo) {
    return DEFAULT_PAYMENT_TYPE
  }

  // Return first available payment method or default
  if (topupInfo.pay_methods?.length > 0) {
    return topupInfo.pay_methods[0].type
  }

  if (topupInfo.enable_stripe_topup) {
    return PAYMENT_TYPES.STRIPE
  }

  if (topupInfo.enable_waffo_topup) {
    return PAYMENT_TYPES.WAFFO
  }

  if (topupInfo.enable_waffo_pancake_topup) {
    return PAYMENT_TYPES.WAFFO_PANCAKE
  }

  return DEFAULT_PAYMENT_TYPE
}

/**
 * Get minimum topup amount from topup info
 */
export function getMinTopupAmount(topupInfo: TopupInfo | null): number {
  if (!topupInfo) {
    return DEFAULT_MIN_TOPUP
  }

  const methodMinTopups = (topupInfo.pay_methods ?? [])
    .map((method) => Number(method.min_topup))
    .filter((minTopup) => Number.isFinite(minTopup) && minTopup > 0)
  const standardMinimums = [
    ...(topupInfo.enable_online_topup ? [topupInfo.min_topup] : []),
    ...methodMinTopups,
  ].filter((minTopup) => Number.isFinite(minTopup) && minTopup > 0)
  if (standardMinimums.length > 0) {
    return Math.min(...standardMinimums)
  }

  if (topupInfo.enable_stripe_topup) {
    return topupInfo.stripe_min_topup
  }

  if (topupInfo.enable_waffo_topup) {
    return topupInfo.waffo_min_topup || DEFAULT_MIN_TOPUP
  }

  if (topupInfo.enable_waffo_pancake_topup) {
    return topupInfo.waffo_pancake_min_topup || DEFAULT_MIN_TOPUP
  }

  return DEFAULT_MIN_TOPUP
}

/**
 * Generate preset amounts based on minimum topup
 */
export function generatePresetAmounts(minAmount: number): PresetAmount[] {
  return DEFAULT_PRESET_MULTIPLIERS.map((multiplier) => ({
    value: minAmount * multiplier,
  }))
}

/**
 * Merge custom preset amounts with discounts
 */
export function mergePresetAmounts(
  amountOptions: number[],
  discounts: Record<number, number>
): PresetAmount[] {
  if (!amountOptions || amountOptions.length === 0) {
    return []
  }

  return amountOptions.map((amount) => ({
    value: amount,
    discount: discounts[amount] || 1.0,
  }))
}

export function getEffectiveTopupDiscount(
  amount: number,
  discounts: Record<number, number> = {},
  userTopupDiscount?: number
): number {
  if (userTopupDiscount && userTopupDiscount > 0 && userTopupDiscount < 1) {
    return userTopupDiscount
  }
  return discounts[amount] || 1.0
}
