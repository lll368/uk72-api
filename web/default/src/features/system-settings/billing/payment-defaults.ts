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
import type { AlipaySettingsValues } from '../integrations/alipay-settings'
import type { CreemSettingsValues } from '../integrations/payment-settings-core'
import type {
  EpaySettingsValues,
  GeneralPaymentSettingsValues,
  PaymentComplianceDefaults,
  StripeSettingsValues,
} from '../integrations/payment-settings-core'
import type { PiggyWithdrawSettingsValues } from '../integrations/piggy-withdraw-settings-section'
import {
  QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY,
  type QiniuChildAccountAssignmentMode,
  type QiniuSettingsValues,
} from '../integrations/qiniu-settings'
import type { WaffoPancakeSettingsValues } from '../integrations/waffo-pancake-settings-section'
import type { WaffoSettingsValues } from '../integrations/waffo-settings-section'
import type { WechatPaySettingsValues } from '../integrations/wechat-pay-settings'
import type { BillingSettings } from '../types'

export function buildGeneralPaymentDefaults(
  settings: BillingSettings
): GeneralPaymentSettingsValues {
  return {
    Price: settings.Price,
    MinTopUp: settings.MinTopUp,
    PayMethods: settings.PayMethods,
    AmountOptions: settings['payment_setting.amount_options'],
    AmountDiscount: settings['payment_setting.amount_discount'],
    DefaultUserTopupDiscount:
      settings['payment_setting.default_user_topup_discount'] ?? 1,
    DefaultVvipTopupDiscount:
      settings['payment_setting.default_vvip_topup_discount'] ?? 1,
    VipActivationPrice:
      settings['payment_setting.vip_activation_price'] ?? 1680,
    VvipActivationCommissionLevel1Amount:
      settings['payment_setting.vip_activation_commission_level1_amount'] ??
      1000,
    VvipActivationCommissionLevel2Amount:
      settings['payment_setting.vip_activation_commission_level2_amount'] ??
      400,
    CommissionMinWithdrawAmount:
      settings['payment_setting.commission_min_withdraw_amount'] ?? 0,
  }
}

export function buildEpayDefaults(
  settings: BillingSettings
): EpaySettingsValues {
  return {
    PayAddress: settings.PayAddress,
    EpayId: settings.EpayId,
    EpayKey: settings.EpayKey,
    CustomCallbackAddress: settings.CustomCallbackAddress,
  }
}

export function buildAlipayDefaults(
  settings: BillingSettings
): AlipaySettingsValues {
  return {
    AlipayEnabled: settings.AlipayEnabled ?? false,
    AlipaySandbox: settings.AlipaySandbox ?? false,
    AlipayAppId: settings.AlipayAppId ?? '',
    AlipayPrivateKey: settings.AlipayPrivateKey ?? '',
    AlipayPublicKey: settings.AlipayPublicKey ?? '',
    AlipayUnitPrice: settings.AlipayUnitPrice ?? 7.3,
    AlipayMinTopUp: settings.AlipayMinTopUp ?? 1,
    AlipayReturnUrl: settings.AlipayReturnUrl ?? '',
    AlipayNotifyUrl: settings.AlipayNotifyUrl ?? '',
  }
}

export function buildWechatPayDefaults(
  settings: BillingSettings
): WechatPaySettingsValues {
  return {
    WechatPayEnabled: settings.WechatPayEnabled ?? false,
    WechatPaySandbox: settings.WechatPaySandbox ?? false,
    WechatPayAppId: settings.WechatPayAppId ?? '',
    WechatPayMchId: settings.WechatPayMchId ?? '',
    WechatPayMerchantSerialNo: settings.WechatPayMerchantSerialNo ?? '',
    WechatPayMerchantPrivateKey: settings.WechatPayMerchantPrivateKey ?? '',
    WechatPayAPIv3Key: settings.WechatPayAPIv3Key ?? '',
    WechatPayPlatformSerialNo: settings.WechatPayPlatformSerialNo ?? '',
    WechatPayPlatformPublicKey: settings.WechatPayPlatformPublicKey ?? '',
    WechatPayUnitPrice: settings.WechatPayUnitPrice ?? 7.3,
    WechatPayMinTopUp: settings.WechatPayMinTopUp ?? 1,
    WechatPayNotifyUrl: settings.WechatPayNotifyUrl ?? '',
  }
}

export function buildStripeDefaults(
  settings: BillingSettings
): StripeSettingsValues {
  return {
    StripeApiSecret: settings.StripeApiSecret,
    StripeWebhookSecret: settings.StripeWebhookSecret,
    StripePriceId: settings.StripePriceId,
    StripeUnitPrice: settings.StripeUnitPrice,
    StripeMinTopUp: settings.StripeMinTopUp,
    StripePromotionCodesEnabled: settings.StripePromotionCodesEnabled,
  }
}

export function buildCreemDefaults(
  settings: BillingSettings
): CreemSettingsValues {
  return {
    CreemApiKey: settings.CreemApiKey,
    CreemWebhookSecret: settings.CreemWebhookSecret,
    CreemTestMode: settings.CreemTestMode,
    CreemProducts: settings.CreemProducts,
  }
}

export function buildWaffoDefaults(
  settings: BillingSettings
): WaffoSettingsValues {
  return {
    WaffoEnabled: settings.WaffoEnabled ?? false,
    WaffoApiKey: settings.WaffoApiKey ?? '',
    WaffoPrivateKey: settings.WaffoPrivateKey ?? '',
    WaffoPublicCert: settings.WaffoPublicCert ?? '',
    WaffoSandboxPublicCert: settings.WaffoSandboxPublicCert ?? '',
    WaffoSandboxApiKey: settings.WaffoSandboxApiKey ?? '',
    WaffoSandboxPrivateKey: settings.WaffoSandboxPrivateKey ?? '',
    WaffoSandbox: settings.WaffoSandbox ?? false,
    WaffoMerchantId: settings.WaffoMerchantId ?? '',
    WaffoCurrency: settings.WaffoCurrency ?? 'USD',
    WaffoUnitPrice: settings.WaffoUnitPrice ?? 1,
    WaffoMinTopUp: settings.WaffoMinTopUp ?? 1,
    WaffoNotifyUrl: settings.WaffoNotifyUrl ?? '',
    WaffoReturnUrl: settings.WaffoReturnUrl ?? '',
    WaffoPayMethods: settings.WaffoPayMethods ?? '[]',
  }
}

export function buildWaffoPancakeDefaults(
  settings: BillingSettings
): WaffoPancakeSettingsValues {
  return {
    WaffoPancakeEnabled: settings.WaffoPancakeEnabled ?? false,
    WaffoPancakeSandbox: settings.WaffoPancakeSandbox ?? false,
    WaffoPancakeMerchantID: settings.WaffoPancakeMerchantID ?? '',
    WaffoPancakePrivateKey: settings.WaffoPancakePrivateKey ?? '',
    WaffoPancakeWebhookPublicKey: settings.WaffoPancakeWebhookPublicKey ?? '',
    WaffoPancakeWebhookTestKey: settings.WaffoPancakeWebhookTestKey ?? '',
    WaffoPancakeStoreID: settings.WaffoPancakeStoreID ?? '',
    WaffoPancakeProductID: settings.WaffoPancakeProductID ?? '',
    WaffoPancakeReturnURL: settings.WaffoPancakeReturnURL ?? '',
    WaffoPancakeCurrency: settings.WaffoPancakeCurrency ?? 'USD',
    WaffoPancakeUnitPrice: settings.WaffoPancakeUnitPrice ?? 1,
    WaffoPancakeMinTopUp: settings.WaffoPancakeMinTopUp ?? 1,
  }
}

export function buildPiggyWithdrawDefaults(
  settings: BillingSettings
): PiggyWithdrawSettingsValues {
  return {
    Enabled: settings['piggy_withdraw_setting.enabled'] ?? false,
    Domain:
      settings['piggy_withdraw_setting.domain'] ?? 'https://saas.xzsz.ltd',
    AppKey: settings['piggy_withdraw_setting.app_key'] ?? '',
    AppSecret: settings['piggy_withdraw_setting.app_secret'] ?? '',
    AESIV: settings['piggy_withdraw_setting.aes_iv'] ?? '0000000000000000',
    TaxFundId: settings['piggy_withdraw_setting.tax_fund_id'] ?? '',
    PositionName: settings['piggy_withdraw_setting.position_name'] ?? '',
    Position: settings['piggy_withdraw_setting.position'] ?? '',
    SignJumpPage: settings['piggy_withdraw_setting.sign_jump_page'] ?? '',
    SignNotifyUrl: settings['piggy_withdraw_setting.sign_notify_url'] ?? '',
    PayNotifyUrl: settings['piggy_withdraw_setting.pay_notify_url'] ?? '',
    RequestTimeout: settings['piggy_withdraw_setting.request_timeout'] ?? 15,
    CallbackLockTTL:
      settings['piggy_withdraw_setting.callback_lock_ttl'] ?? 300,
    CooldownMinutes: settings['piggy_withdraw_setting.cooldown_minutes'] ?? 30,
    ForbiddenWithdrawTime:
      settings['piggy_withdraw_setting.forbidden_withdraw_time'] ?? '',
    CalcType: settings['piggy_withdraw_setting.calc_type'] ?? 'C',
    PlatformFeeRate: settings['piggy_withdraw_setting.platform_fee_rate'] ?? 8,
    BankRemark: settings['piggy_withdraw_setting.bank_remark'] ?? '',
  }
}

export function buildQiniuDefaults(
  settings: BillingSettings
): QiniuSettingsValues {
  return {
    Enabled: settings['qiniu_key_setting.enabled'] ?? false,
    BaseURL: settings['qiniu_key_setting.base_url'] ?? 'https://api.qnaigc.com',
    ChildAccountBaseURL:
      settings['qiniu_key_setting.child_account_base_url'] ??
      'https://api.qiniu.com',
    AccessKey: settings['qiniu_key_setting.access_key'] ?? '',
    SecretKey: settings['qiniu_key_setting.secret_key'] ?? '',
    RequestTimeout: settings['qiniu_key_setting.request_timeout'] ?? 15,
    RetryIntervalSeconds:
      settings['qiniu_key_setting.retry_interval_seconds'] ?? 300,
    OfficialLedgerEnabled:
      settings['qiniu_key_setting.official_ledger_enabled'] ?? false,
    OfficialLedgerCutoverTime:
      settings['qiniu_key_setting.official_ledger_cutover_time'] ?? 0,
    OfficialLedgerSyncIntervalSeconds:
      settings['qiniu_key_setting.official_ledger_sync_interval_seconds'] ?? 60,
    OfficialLedgerWindowHours:
      settings['qiniu_key_setting.official_ledger_window_hours'] ?? 6,
    OfficialLedgerWindowDays:
      settings['qiniu_key_setting.official_ledger_window_days'] ?? 2,
    OfficialLedgerBatchSize:
      settings['qiniu_key_setting.official_ledger_batch_size'] ?? 100,
    OfficialLedgerRateLimitPerSecond:
      settings['qiniu_key_setting.official_ledger_rate_limit_per_second'] ?? 4,
    OfficialLedgerRetryIntervalSeconds:
      settings['qiniu_key_setting.official_ledger_retry_interval_seconds'] ??
      300,
    CostDetailCutoverTime:
      settings['qiniu_key_setting.cost_detail_cutover_time'] ?? 0,
    CostDetailLookbackDays:
      settings['qiniu_key_setting.cost_detail_lookback_days'] ?? 7,
    CostDetailAutoApplyEnabled:
      settings['qiniu_key_setting.cost_detail_auto_apply_enabled'] ?? true,
    MarketCatalogEnabled:
      settings['qiniu_key_setting.market_catalog_enabled'] ?? false,
    MarketCatalogBaseURL:
      settings['qiniu_key_setting.market_catalog_base_url'] ??
      'https://openai.qiniu.com',
    MarketCatalogTTLSeconds:
      settings['qiniu_key_setting.market_catalog_ttl_seconds'] ?? 3600,
    MarketCatalogOverseas:
      settings['qiniu_key_setting.market_catalog_overseas'] ?? true,
    MarketCatalogFallbackEnabled:
      settings['qiniu_key_setting.market_catalog_fallback_enabled'] ?? true,
    ChildAccountEmailDomain:
      settings['qiniu_key_setting.child_account_email_domain'] ?? 'uk72.cn',
    ChildAccountEmailPrefix:
      settings['qiniu_key_setting.child_account_email_prefix'] ?? 'child',
    ChildAccountPasswordLength:
      settings['qiniu_key_setting.child_account_password_length'] ?? 18,
    ChildAccountRequestTimeout:
      settings['qiniu_key_setting.child_account_request_timeout'] ?? 15,
    ChildAccountRetryIntervalSeconds:
      settings['qiniu_key_setting.child_account_retry_interval_seconds'] ?? 300,
    ChildAccountBindingEnabled:
      settings['qiniu_key_setting.child_account_binding_enabled'] ?? false,
    ChildAccountAssignmentMode:
      (settings[
        'qiniu_key_setting.child_account_assignment_mode'
      ] as QiniuChildAccountAssignmentMode | undefined) ??
      QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY,
    ChildAccountBindingCutoverTime:
      settings['qiniu_key_setting.child_account_binding_cutover_time'] ?? 0,
  }
}

export function buildPaymentComplianceDefaults(
  settings: BillingSettings
): PaymentComplianceDefaults {
  return {
    confirmed: settings['payment_setting.compliance_confirmed'] ?? false,
    termsVersion: settings['payment_setting.compliance_terms_version'] ?? '',
    confirmedAt: settings['payment_setting.compliance_confirmed_at'] ?? 0,
    confirmedBy: settings['payment_setting.compliance_confirmed_by'] ?? 0,
  }
}
