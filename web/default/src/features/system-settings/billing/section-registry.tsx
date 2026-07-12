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
import { parseCurrencyDisplayType } from '@/lib/currency'
import { CheckinSettingsSection } from '../general/checkin-settings-section'
import { PricingSection } from '../general/pricing-section'
import { QuotaSettingsSection } from '../general/quota-settings-section'
import { AlipaySettingsSection } from '../integrations/alipay-settings-section'
import { CreemSettingsSection } from '../integrations/creem-settings-section'
import { EpaySettingsSection } from '../integrations/epay-settings-section'
import { PaymentComplianceGate } from '../integrations/payment-compliance-gate'
import { PaymentSettingsSection } from '../integrations/payment-settings-section'
import { PiggyWithdrawSettingsSection } from '../integrations/piggy-withdraw-settings-section'
import { QiniuSettingsSection } from '../integrations/qiniu-settings-section'
import { StripeSettingsSection } from '../integrations/stripe-settings-section'
import { WaffoPancakeSettingsSection } from '../integrations/waffo-pancake-settings-section'
import { WaffoSettingsSection } from '../integrations/waffo-settings-section'
import { WechatPaySettingsSection } from '../integrations/wechat-pay-settings-section'
import { RatioSettingsCard } from '../models/ratio-settings-card'
import type { BillingSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import {
  buildAlipayDefaults,
  buildCreemDefaults,
  buildEpayDefaults,
  buildGeneralPaymentDefaults,
  buildPaymentComplianceDefaults,
  buildPiggyWithdrawDefaults,
  buildQiniuDefaults,
  buildStripeDefaults,
  buildWaffoDefaults,
  buildWaffoPancakeDefaults,
  buildWechatPayDefaults,
} from './payment-defaults'

const getModelDefaults = (settings: BillingSettings) => ({
  ModelPrice: settings.ModelPrice,
  ModelRatio: settings.ModelRatio,
  CacheRatio: settings.CacheRatio,
  CreateCacheRatio: settings.CreateCacheRatio,
  CompletionRatio: settings.CompletionRatio,
  ImageRatio: settings.ImageRatio,
  AudioRatio: settings.AudioRatio,
  AudioCompletionRatio: settings.AudioCompletionRatio,
  ExposeRatioEnabled: settings.ExposeRatioEnabled,
  BillingMode: settings['billing_setting.billing_mode'],
  BillingExpr: settings['billing_setting.billing_expr'],
})

const getGroupDefaults = (settings: BillingSettings) => ({
  TopupGroupRatio: settings.TopupGroupRatio,
  GroupRatio: settings.GroupRatio,
  UserUsableGroups: settings.UserUsableGroups,
  GroupGroupRatio: settings.GroupGroupRatio,
  AutoGroups: settings.AutoGroups,
  DefaultUseAutoGroup: settings.DefaultUseAutoGroup,
  GroupSpecialUsableGroup:
    settings['group_ratio_setting.group_special_usable_group'],
})

const BILLING_SECTIONS = [
  {
    id: 'quota',
    titleKey: 'Quota Settings',
    descriptionKey: 'Configure user quota allocation and rewards',
    build: (settings: BillingSettings) => (
      <QuotaSettingsSection
        defaultValues={{
          QuotaForNewUser: settings.QuotaForNewUser,
          PreConsumedQuota: settings.PreConsumedQuota,
          QuotaForInviter: settings.QuotaForInviter,
          QuotaForInvitee: settings.QuotaForInvitee,
          TopUpLink: settings.TopUpLink,
          general_setting: {
            docs_link: settings['general_setting.docs_link'],
          },
          quota_setting: {
            enable_free_model_pre_consume:
              settings['quota_setting.enable_free_model_pre_consume'],
          },
        }}
        complianceConfirmed={
          (settings['payment_setting.compliance_confirmed'] ?? false) &&
          settings['payment_setting.compliance_terms_version'] === 'v1'
        }
      />
    ),
  },
  {
    id: 'currency',
    titleKey: 'Currency & Display',
    descriptionKey: 'Configure currency conversion and quota display options',
    build: (settings: BillingSettings) => (
      <PricingSection
        defaultValues={{
          QuotaPerUnit: settings.QuotaPerUnit,
          USDExchangeRate: settings.USDExchangeRate,
          DisplayInCurrencyEnabled: settings.DisplayInCurrencyEnabled,
          DisplayTokenStatEnabled: settings.DisplayTokenStatEnabled,
          general_setting: {
            quota_display_type: parseCurrencyDisplayType(
              settings['general_setting.quota_display_type']
            ),
            custom_currency_symbol:
              settings['general_setting.custom_currency_symbol'] ?? '¤',
            custom_currency_exchange_rate:
              settings['general_setting.custom_currency_exchange_rate'] ?? 1,
          },
        }}
      />
    ),
  },
  {
    id: 'model-pricing',
    titleKey: 'Model Pricing',
    descriptionKey: 'Configure model pricing ratios and tool prices',
    build: (settings: BillingSettings) => (
      <RatioSettingsCard
        titleKey='Model Pricing'
        descriptionKey='Configure model pricing ratios and tool prices'
        modelDefaults={getModelDefaults(settings)}
        groupDefaults={getGroupDefaults(settings)}
        toolPricesDefault={settings['tool_price_setting.prices']}
        visibleTabs={['models', 'tool-prices', 'upstream-sync']}
      />
    ),
  },
  {
    id: 'group-pricing',
    titleKey: 'Group Pricing',
    descriptionKey: 'Configure group ratios and group-specific pricing rules',
    build: (settings: BillingSettings) => (
      <RatioSettingsCard
        titleKey='Group Pricing'
        descriptionKey='Configure group ratios and group-specific pricing rules'
        modelDefaults={getModelDefaults(settings)}
        groupDefaults={getGroupDefaults(settings)}
        toolPricesDefault={settings['tool_price_setting.prices']}
        visibleTabs={['groups']}
      />
    ),
  },
  {
    id: 'qiniu',
    titleKey: 'Qiniu Key & Ledger',
    descriptionKey:
      'Configure Qiniu managed keys, official ledger sync, and model catalog',
    build: (settings: BillingSettings) => (
      <QiniuSettingsSection defaultValues={buildQiniuDefaults(settings)} />
    ),
  },
  {
    id: 'payment',
    titleKey: 'Payment General',
    descriptionKey: 'Shared recharge and VVIP payment rules',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <PaymentSettingsSection
          defaultValues={buildGeneralPaymentDefaults(settings)}
        />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'payment-epay',
    titleKey: 'Epay Gateway',
    descriptionKey: 'Configuration for Epay payment integration',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <EpaySettingsSection defaultValues={buildEpayDefaults(settings)} />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'payment-alipay-direct',
    titleKey: 'Alipay Direct Gateway',
    descriptionKey:
      'Configuration for official Alipay direct payment integration',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <AlipaySettingsSection defaultValues={buildAlipayDefaults(settings)} />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'payment-wechat-direct',
    titleKey: 'WeChat Pay Direct Gateway',
    descriptionKey:
      'Configuration for official WeChat Pay Native payment integration',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <WechatPaySettingsSection
          defaultValues={buildWechatPayDefaults(settings)}
        />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'payment-stripe',
    titleKey: 'Stripe Gateway',
    descriptionKey: 'Configuration for Stripe payment integration',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <StripeSettingsSection defaultValues={buildStripeDefaults(settings)} />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'payment-creem',
    titleKey: 'Creem Gateway',
    descriptionKey: 'Configuration for Creem payment integration',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <CreemSettingsSection defaultValues={buildCreemDefaults(settings)} />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'payment-waffo',
    titleKey: 'Waffo Payment Gateway',
    descriptionKey: 'Configure Waffo payment aggregation platform integration',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <WaffoSettingsSection defaultValues={buildWaffoDefaults(settings)} />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'payment-waffo-pancake',
    titleKey: 'Waffo Pancake Payment Gateway',
    descriptionKey:
      'Configure Waffo Pancake hosted checkout integration for USD-priced top-ups',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <WaffoPancakeSettingsSection
          defaultValues={buildWaffoPancakeDefaults(settings)}
        />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'withdraw-piggy',
    titleKey: 'Piggy bank card withdrawal',
    descriptionKey:
      'Configure Piggy continuous labor V3 bank card withdrawal for commissions',
    build: (settings: BillingSettings) => (
      <PaymentComplianceGate
        defaults={buildPaymentComplianceDefaults(settings)}
      >
        <PiggyWithdrawSettingsSection
          defaultValues={buildPiggyWithdrawDefaults(settings)}
        />
      </PaymentComplianceGate>
    ),
  },
  {
    id: 'checkin',
    titleKey: 'Check-in Rewards',
    descriptionKey: 'Configure daily check-in rewards for users',
    build: (settings: BillingSettings) => (
      <CheckinSettingsSection
        defaultValues={{
          enabled: settings['checkin_setting.enabled'],
          minQuota: settings['checkin_setting.min_quota'],
          maxQuota: settings['checkin_setting.max_quota'],
        }}
      />
    ),
  },
] as const

export type BillingSectionId = (typeof BILLING_SECTIONS)[number]['id']

const HIDDEN_BILLING_SECTION_IDS = [
  'model-pricing',
  'group-pricing',
] satisfies readonly BillingSectionId[]

const hiddenBillingSectionIdSet = new Set<BillingSectionId>(
  HIDDEN_BILLING_SECTION_IDS
)

const VISIBLE_BILLING_SECTIONS = BILLING_SECTIONS.filter(
  (section) => !hiddenBillingSectionIdSet.has(section.id)
)

const billingRegistry = createSectionRegistry<
  BillingSectionId,
  BillingSettings
>({
  sections: VISIBLE_BILLING_SECTIONS,
  defaultSection: 'quota',
  basePath: '/system-settings/billing',
  urlStyle: 'path',
})

export const BILLING_SECTION_IDS = billingRegistry.sectionIds
export const BILLING_DEFAULT_SECTION = billingRegistry.defaultSection
export const getBillingSectionNavItems = billingRegistry.getSectionNavItems
export const getBillingSectionContent = billingRegistry.getSectionContent
