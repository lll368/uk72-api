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
import * as z from 'zod'
import {
  getJsonError,
  normalizeJsonForComparison,
  removeTrailingSlash,
} from './utils'

export const CURRENT_COMPLIANCE_TERMS_VERSION = 'v1'
export const VIP_ACTIVATION_PRICE_KEY = 'payment_setting.vip_activation_price'
export const VIP_ACTIVATION_LEVEL1_AMOUNT_KEY =
  'payment_setting.vip_activation_commission_level1_amount'
export const VIP_ACTIVATION_LEVEL2_AMOUNT_KEY =
  'payment_setting.vip_activation_commission_level2_amount'
export const COMMISSION_MIN_WITHDRAW_AMOUNT_KEY =
  'payment_setting.commission_min_withdraw_amount'

const VIP_ACTIVATION_MONEY_PRECISION_MESSAGE =
  'VVIP activation money fields support at most 2 decimal places'

export type OptionUpdate = {
  key: string
  value: string | number | boolean
}

export type PaymentComplianceDefaults = {
  confirmed: boolean
  termsVersion: string
  confirmedAt: number
  confirmedBy: number
}

function isOptionalHttpUrl(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return true
  return /^https?:\/\//.test(trimmed)
}

export function hasAtMostTwoDecimalPlaces(value: number) {
  if (!Number.isFinite(value)) {
    return false
  }
  const normalized = value.toString().toLowerCase()
  if (normalized.includes('e')) {
    return Math.abs(Math.round(value * 100) - value * 100) < 1e-9
  }
  const decimalPart = normalized.split('.')[1]
  return !decimalPart || decimalPart.length <= 2
}

export const generalPaymentSettingsSchema = z
  .object({
    Price: z.coerce.number().min(0),
    MinTopUp: z.coerce.number().min(0),
    CommissionMinWithdrawAmount: z.coerce.number().min(0),
    DefaultUserTopupDiscount: z.coerce.number().gt(0).lte(1),
    DefaultVvipTopupDiscount: z.coerce.number().gt(0).lte(1),
    VipActivationPrice: z.coerce
      .number()
      .gt(0)
      .refine(
        hasAtMostTwoDecimalPlaces,
        VIP_ACTIVATION_MONEY_PRECISION_MESSAGE
      ),
    VvipActivationCommissionLevel1Amount: z.coerce
      .number()
      .min(0)
      .refine(
        hasAtMostTwoDecimalPlaces,
        VIP_ACTIVATION_MONEY_PRECISION_MESSAGE
      ),
    VvipActivationCommissionLevel2Amount: z.coerce
      .number()
      .min(0)
      .refine(
        hasAtMostTwoDecimalPlaces,
        VIP_ACTIVATION_MONEY_PRECISION_MESSAGE
      ),
    PayMethods: z.string().superRefine((value, ctx) => {
      const error = getJsonError(value)
      if (error) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: error,
        })
      }
    }),
    AmountOptions: z.string().superRefine((value, ctx) => {
      const error = getJsonError(value, (parsed) => Array.isArray(parsed))
      if (error) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: error,
        })
      }
    }),
    AmountDiscount: z.string().superRefine((value, ctx) => {
      const error = getJsonError(
        value,
        (parsed) =>
          !!parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      )
      if (error) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: error,
        })
      }
    }),
  })
  .superRefine((values, ctx) => {
    if (
      values.VvipActivationCommissionLevel1Amount +
        values.VvipActivationCommissionLevel2Amount >
      values.VipActivationPrice
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['VvipActivationCommissionLevel2Amount'],
        message:
          'VVIP activation commission amounts cannot exceed activation price in total',
      })
    }
  })

export const epaySettingsSchema = z.object({
  PayAddress: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid callback URL starting with http:// or https://'
    ),
  EpayId: z.string(),
  EpayKey: z.string(),
  CustomCallbackAddress: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid URL starting with http:// or https://'
    ),
})

export const alipaySettingsSchema = z.object({
  AlipayEnabled: z.boolean(),
  AlipaySandbox: z.boolean(),
  AlipayAppId: z.string(),
  AlipayPrivateKey: z.string(),
  AlipayPublicKey: z.string(),
  AlipayUnitPrice: z.coerce.number().gt(0),
  AlipayMinTopUp: z.coerce.number().min(0),
  AlipayReturnUrl: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid URL starting with http:// or https://'
    ),
  AlipayNotifyUrl: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid callback URL starting with http:// or https://'
    ),
})

export const wechatPaySettingsSchema = z.object({
  WechatPayEnabled: z.boolean(),
  WechatPaySandbox: z.boolean(),
  WechatPayAppId: z.string(),
  WechatPayMchId: z.string(),
  WechatPayMerchantSerialNo: z.string(),
  WechatPayMerchantPrivateKey: z.string(),
  WechatPayAPIv3Key: z.string().refine((value) => {
    const trimmed = value.trim()
    return !trimmed || new TextEncoder().encode(trimmed).length === 32
  }, 'WeChat Pay API v3 key must be 32 bytes'),
  WechatPayPlatformSerialNo: z.string(),
  WechatPayPlatformPublicKey: z.string(),
  WechatPayUnitPrice: z.coerce.number().gt(0),
  WechatPayMinTopUp: z.coerce.number().min(0),
  WechatPayNotifyUrl: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid callback URL starting with http:// or https://'
    ),
})

export const stripeSettingsSchema = z.object({
  StripeApiSecret: z.string(),
  StripeWebhookSecret: z.string(),
  StripePriceId: z.string(),
  StripeUnitPrice: z.coerce.number().min(0),
  StripeMinTopUp: z.coerce.number().min(0),
  StripePromotionCodesEnabled: z.boolean(),
})

export const creemSettingsSchema = z.object({
  CreemApiKey: z.string(),
  CreemWebhookSecret: z.string(),
  CreemTestMode: z.boolean(),
  CreemProducts: z.string().superRefine((value, ctx) => {
    const error = getJsonError(value, (parsed) => Array.isArray(parsed))
    if (error) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: error,
      })
    }
  }),
})

export type GeneralPaymentSettingsValues = z.infer<
  typeof generalPaymentSettingsSchema
>
export type EpaySettingsValues = z.infer<typeof epaySettingsSchema>
export type AlipaySettingsFormValues = z.infer<typeof alipaySettingsSchema>
export type WechatPaySettingsFormValues = z.infer<
  typeof wechatPaySettingsSchema
>
export type StripeSettingsValues = z.infer<typeof stripeSettingsSchema>
export type CreemSettingsValues = z.infer<typeof creemSettingsSchema>

type VipActivationCommissionAmountValues = Pick<
  GeneralPaymentSettingsValues,
  | 'VipActivationPrice'
  | 'VvipActivationCommissionLevel1Amount'
  | 'VvipActivationCommissionLevel2Amount'
>

export function orderVipActivationCommissionUpdates(
  updates: OptionUpdate[],
  initial: VipActivationCommissionAmountValues,
  sanitized: VipActivationCommissionAmountValues
) {
  const priceUpdate = updates.find(
    (update) => update.key === VIP_ACTIVATION_PRICE_KEY
  )
  const level1Update = updates.find(
    (update) => update.key === VIP_ACTIVATION_LEVEL1_AMOUNT_KEY
  )
  const level2Update = updates.find(
    (update) => update.key === VIP_ACTIVATION_LEVEL2_AMOUNT_KEY
  )

  if (!priceUpdate && !level1Update && !level2Update) {
    return updates
  }

  const orderedAmountUpdates: OptionUpdate[] = []
  const pushIfPresent = (update?: OptionUpdate) => {
    if (update) {
      orderedAmountUpdates.push(update)
    }
  }

  // 后端逐项校验金额合计，先降分佣、再改价格、最后升分佣，避免合法批量调整被中间态拦截。
  if (
    sanitized.VvipActivationCommissionLevel1Amount <
    initial.VvipActivationCommissionLevel1Amount
  ) {
    pushIfPresent(level1Update)
  }
  if (
    sanitized.VvipActivationCommissionLevel2Amount <
    initial.VvipActivationCommissionLevel2Amount
  ) {
    pushIfPresent(level2Update)
  }
  pushIfPresent(priceUpdate)
  if (
    sanitized.VvipActivationCommissionLevel1Amount >=
    initial.VvipActivationCommissionLevel1Amount
  ) {
    pushIfPresent(level1Update)
  }
  if (
    sanitized.VvipActivationCommissionLevel2Amount >=
    initial.VvipActivationCommissionLevel2Amount
  ) {
    pushIfPresent(level2Update)
  }

  return [
    ...updates.filter(
      (update) =>
        update.key !== VIP_ACTIVATION_PRICE_KEY &&
        update.key !== VIP_ACTIVATION_LEVEL1_AMOUNT_KEY &&
        update.key !== VIP_ACTIVATION_LEVEL2_AMOUNT_KEY
    ),
    ...orderedAmountUpdates,
  ]
}

export function buildGeneralPaymentSettingsUpdates(
  initialValues: GeneralPaymentSettingsValues,
  nextValues: GeneralPaymentSettingsValues
): OptionUpdate[] {
  const sanitized: GeneralPaymentSettingsValues = {
    Price: Number(nextValues.Price),
    MinTopUp: Number(nextValues.MinTopUp),
    CommissionMinWithdrawAmount: Number(nextValues.CommissionMinWithdrawAmount),
    DefaultUserTopupDiscount: Number(nextValues.DefaultUserTopupDiscount),
    DefaultVvipTopupDiscount: Number(nextValues.DefaultVvipTopupDiscount),
    VipActivationPrice: Number(nextValues.VipActivationPrice),
    VvipActivationCommissionLevel1Amount: Number(
      nextValues.VvipActivationCommissionLevel1Amount
    ),
    VvipActivationCommissionLevel2Amount: Number(
      nextValues.VvipActivationCommissionLevel2Amount
    ),
    PayMethods: nextValues.PayMethods.trim(),
    AmountOptions: nextValues.AmountOptions.trim(),
    AmountDiscount: nextValues.AmountDiscount.trim(),
  }

  const initial: GeneralPaymentSettingsValues = {
    Price: initialValues.Price,
    MinTopUp: initialValues.MinTopUp,
    CommissionMinWithdrawAmount: initialValues.CommissionMinWithdrawAmount,
    DefaultUserTopupDiscount: initialValues.DefaultUserTopupDiscount,
    DefaultVvipTopupDiscount: initialValues.DefaultVvipTopupDiscount,
    VipActivationPrice: initialValues.VipActivationPrice,
    VvipActivationCommissionLevel1Amount:
      initialValues.VvipActivationCommissionLevel1Amount,
    VvipActivationCommissionLevel2Amount:
      initialValues.VvipActivationCommissionLevel2Amount,
    PayMethods: initialValues.PayMethods.trim(),
    AmountOptions: initialValues.AmountOptions.trim(),
    AmountDiscount: initialValues.AmountDiscount.trim(),
  }

  const updates: OptionUpdate[] = []

  if (sanitized.Price !== initial.Price) {
    updates.push({ key: 'Price', value: sanitized.Price })
  }
  if (sanitized.MinTopUp !== initial.MinTopUp) {
    updates.push({ key: 'MinTopUp', value: sanitized.MinTopUp })
  }
  if (
    sanitized.CommissionMinWithdrawAmount !==
    initial.CommissionMinWithdrawAmount
  ) {
    updates.push({
      key: COMMISSION_MIN_WITHDRAW_AMOUNT_KEY,
      value: sanitized.CommissionMinWithdrawAmount,
    })
  }
  if (sanitized.DefaultUserTopupDiscount !== initial.DefaultUserTopupDiscount) {
    updates.push({
      key: 'payment_setting.default_user_topup_discount',
      value: sanitized.DefaultUserTopupDiscount,
    })
  }
  if (sanitized.DefaultVvipTopupDiscount !== initial.DefaultVvipTopupDiscount) {
    updates.push({
      key: 'payment_setting.default_vvip_topup_discount',
      value: sanitized.DefaultVvipTopupDiscount,
    })
  }
  if (sanitized.VipActivationPrice !== initial.VipActivationPrice) {
    updates.push({
      key: VIP_ACTIVATION_PRICE_KEY,
      value: sanitized.VipActivationPrice,
    })
  }
  if (
    sanitized.VvipActivationCommissionLevel1Amount !==
    initial.VvipActivationCommissionLevel1Amount
  ) {
    updates.push({
      key: VIP_ACTIVATION_LEVEL1_AMOUNT_KEY,
      value: sanitized.VvipActivationCommissionLevel1Amount,
    })
  }
  if (
    sanitized.VvipActivationCommissionLevel2Amount !==
    initial.VvipActivationCommissionLevel2Amount
  ) {
    updates.push({
      key: VIP_ACTIVATION_LEVEL2_AMOUNT_KEY,
      value: sanitized.VvipActivationCommissionLevel2Amount,
    })
  }
  if (
    normalizeJsonForComparison(sanitized.PayMethods) !==
    normalizeJsonForComparison(initial.PayMethods)
  ) {
    updates.push({ key: 'PayMethods', value: sanitized.PayMethods })
  }
  if (
    normalizeJsonForComparison(sanitized.AmountOptions) !==
    normalizeJsonForComparison(initial.AmountOptions)
  ) {
    updates.push({
      key: 'payment_setting.amount_options',
      value: sanitized.AmountOptions,
    })
  }
  if (
    normalizeJsonForComparison(sanitized.AmountDiscount) !==
    normalizeJsonForComparison(initial.AmountDiscount)
  ) {
    updates.push({
      key: 'payment_setting.amount_discount',
      value: sanitized.AmountDiscount,
    })
  }

  return orderVipActivationCommissionUpdates(updates, initial, sanitized)
}

export function buildEpaySettingsUpdates(
  initialValues: EpaySettingsValues,
  nextValues: EpaySettingsValues
): OptionUpdate[] {
  const initial = {
    PayAddress: removeTrailingSlash(initialValues.PayAddress),
    EpayId: initialValues.EpayId.trim(),
    EpayKey: initialValues.EpayKey.trim(),
    CustomCallbackAddress: removeTrailingSlash(
      initialValues.CustomCallbackAddress
    ),
  }
  const next = {
    PayAddress: removeTrailingSlash(nextValues.PayAddress),
    EpayId: nextValues.EpayId.trim(),
    EpayKey: nextValues.EpayKey.trim(),
    CustomCallbackAddress: removeTrailingSlash(
      nextValues.CustomCallbackAddress
    ),
  }
  const updates: OptionUpdate[] = []

  if (next.PayAddress !== initial.PayAddress) {
    updates.push({ key: 'PayAddress', value: next.PayAddress })
  }
  if (next.EpayId !== initial.EpayId) {
    updates.push({ key: 'EpayId', value: next.EpayId })
  }
  if (next.EpayKey && next.EpayKey !== initial.EpayKey) {
    updates.push({ key: 'EpayKey', value: next.EpayKey })
  }
  if (next.CustomCallbackAddress !== initial.CustomCallbackAddress) {
    updates.push({
      key: 'CustomCallbackAddress',
      value: next.CustomCallbackAddress,
    })
  }

  return updates
}

export function buildStripeSettingsUpdates(
  initialValues: StripeSettingsValues,
  nextValues: StripeSettingsValues
): OptionUpdate[] {
  const initial = {
    StripeApiSecret: initialValues.StripeApiSecret.trim(),
    StripeWebhookSecret: initialValues.StripeWebhookSecret.trim(),
    StripePriceId: initialValues.StripePriceId.trim(),
    StripeUnitPrice: Number(initialValues.StripeUnitPrice),
    StripeMinTopUp: Number(initialValues.StripeMinTopUp),
    StripePromotionCodesEnabled: !!initialValues.StripePromotionCodesEnabled,
  }
  const next = {
    StripeApiSecret: nextValues.StripeApiSecret.trim(),
    StripeWebhookSecret: nextValues.StripeWebhookSecret.trim(),
    StripePriceId: nextValues.StripePriceId.trim(),
    StripeUnitPrice: Number(nextValues.StripeUnitPrice),
    StripeMinTopUp: Number(nextValues.StripeMinTopUp),
    StripePromotionCodesEnabled: !!nextValues.StripePromotionCodesEnabled,
  }
  const updates: OptionUpdate[] = []

  if (
    next.StripeApiSecret &&
    next.StripeApiSecret !== initial.StripeApiSecret
  ) {
    updates.push({ key: 'StripeApiSecret', value: next.StripeApiSecret })
  }
  if (
    next.StripeWebhookSecret &&
    next.StripeWebhookSecret !== initial.StripeWebhookSecret
  ) {
    updates.push({
      key: 'StripeWebhookSecret',
      value: next.StripeWebhookSecret,
    })
  }
  if (next.StripePriceId !== initial.StripePriceId) {
    updates.push({ key: 'StripePriceId', value: next.StripePriceId })
  }
  if (next.StripeUnitPrice !== initial.StripeUnitPrice) {
    updates.push({ key: 'StripeUnitPrice', value: next.StripeUnitPrice })
  }
  if (next.StripeMinTopUp !== initial.StripeMinTopUp) {
    updates.push({ key: 'StripeMinTopUp', value: next.StripeMinTopUp })
  }
  if (
    next.StripePromotionCodesEnabled !== initial.StripePromotionCodesEnabled
  ) {
    updates.push({
      key: 'StripePromotionCodesEnabled',
      value: next.StripePromotionCodesEnabled,
    })
  }

  return updates
}

export function buildCreemSettingsUpdates(
  initialValues: CreemSettingsValues,
  nextValues: CreemSettingsValues
): OptionUpdate[] {
  const initial = {
    CreemApiKey: initialValues.CreemApiKey.trim(),
    CreemWebhookSecret: initialValues.CreemWebhookSecret.trim(),
    CreemTestMode: !!initialValues.CreemTestMode,
    CreemProducts: initialValues.CreemProducts.trim(),
  }
  const next = {
    CreemApiKey: nextValues.CreemApiKey.trim(),
    CreemWebhookSecret: nextValues.CreemWebhookSecret.trim(),
    CreemTestMode: !!nextValues.CreemTestMode,
    CreemProducts: nextValues.CreemProducts.trim(),
  }
  const updates: OptionUpdate[] = []

  if (next.CreemApiKey && next.CreemApiKey !== initial.CreemApiKey) {
    updates.push({ key: 'CreemApiKey', value: next.CreemApiKey })
  }
  if (
    next.CreemWebhookSecret &&
    next.CreemWebhookSecret !== initial.CreemWebhookSecret
  ) {
    updates.push({
      key: 'CreemWebhookSecret',
      value: next.CreemWebhookSecret,
    })
  }
  if (next.CreemTestMode !== initial.CreemTestMode) {
    updates.push({ key: 'CreemTestMode', value: next.CreemTestMode })
  }
  if (
    normalizeJsonForComparison(next.CreemProducts) !==
    normalizeJsonForComparison(initial.CreemProducts)
  ) {
    updates.push({ key: 'CreemProducts', value: next.CreemProducts })
  }

  return updates
}
