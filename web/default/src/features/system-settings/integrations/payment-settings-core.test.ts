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
import { describe, expect, test } from 'bun:test'
import {
  buildCreemSettingsUpdates,
  buildEpaySettingsUpdates,
  buildGeneralPaymentSettingsUpdates,
  buildStripeSettingsUpdates,
  orderVipActivationCommissionUpdates,
  VIP_ACTIVATION_LEVEL1_AMOUNT_KEY,
  VIP_ACTIVATION_LEVEL2_AMOUNT_KEY,
  VIP_ACTIVATION_PRICE_KEY,
  type CreemSettingsValues,
  type EpaySettingsValues,
  type GeneralPaymentSettingsValues,
  type StripeSettingsValues,
} from './payment-settings-core'

const generalSettings: GeneralPaymentSettingsValues = {
  Price: 7.3,
  MinTopUp: 1,
  CommissionMinWithdrawAmount: 0,
  DefaultUserTopupDiscount: 1,
  DefaultVvipTopupDiscount: 1,
  VipActivationPrice: 1680,
  VvipActivationCommissionLevel1Amount: 1000,
  VvipActivationCommissionLevel2Amount: 400,
  PayMethods: '[{"name":"Alipay","type":"alipay","color":"#1677FF"}]',
  AmountOptions: '[10,20,50]',
  AmountDiscount: '{"100":0.95}',
}

describe('buildGeneralPaymentSettingsUpdates', () => {
  test('builds shared payment updates without gateway secrets', () => {
    const updates = buildGeneralPaymentSettingsUpdates(generalSettings, {
      ...generalSettings,
      Price: 8,
      PayMethods: JSON.stringify(
        [{ name: 'Alipay', type: 'alipay', color: '#1677FF' }],
        null,
        2
      ),
      AmountOptions: '[10,20,50,100]',
    })

    expect(updates).toEqual([
      { key: 'Price', value: 8 },
      { key: 'payment_setting.amount_options', value: '[10,20,50,100]' },
    ])
    expect(
      updates.some((update) => /Key|Secret|Private/i.test(update.key))
    ).toBe(false)
  })

  test('builds minimum withdraw amount update', () => {
    const updates = buildGeneralPaymentSettingsUpdates(generalSettings, {
      ...generalSettings,
      CommissionMinWithdrawAmount: 50,
    })

    expect(updates).toEqual([
      { key: 'payment_setting.commission_min_withdraw_amount', value: 50 },
    ])
  })

  test('orders VVIP amount updates to avoid invalid intermediate states', () => {
    const updates = buildGeneralPaymentSettingsUpdates(generalSettings, {
      ...generalSettings,
      VipActivationPrice: 1200,
      VvipActivationCommissionLevel1Amount: 700,
      VvipActivationCommissionLevel2Amount: 300,
    })

    expect(updates.map((update) => update.key)).toEqual([
      VIP_ACTIVATION_LEVEL1_AMOUNT_KEY,
      VIP_ACTIVATION_LEVEL2_AMOUNT_KEY,
      VIP_ACTIVATION_PRICE_KEY,
    ])
  })
})

describe('buildEpaySettingsUpdates', () => {
  const epaySettings: EpaySettingsValues = {
    PayAddress: 'https://pay.example.com',
    EpayId: '10001',
    EpayKey: 'stored-epay-key',
    CustomCallbackAddress: '',
  }

  test('does not clear the Epay key when the field is blank', () => {
    const updates = buildEpaySettingsUpdates(epaySettings, {
      ...epaySettings,
      EpayKey: '',
      CustomCallbackAddress: 'https://callback.example.com/',
    })

    expect(updates).toEqual([
      {
        key: 'CustomCallbackAddress',
        value: 'https://callback.example.com',
      },
    ])
  })
})

describe('buildStripeSettingsUpdates', () => {
  const stripeSettings: StripeSettingsValues = {
    StripeApiSecret: 'stored-api-secret',
    StripeWebhookSecret: 'stored-webhook-secret',
    StripePriceId: 'price_old',
    StripeUnitPrice: 8,
    StripeMinTopUp: 1,
    StripePromotionCodesEnabled: false,
  }

  test('does not clear Stripe secrets when the fields are blank', () => {
    const updates = buildStripeSettingsUpdates(stripeSettings, {
      ...stripeSettings,
      StripeApiSecret: '',
      StripeWebhookSecret: '',
      StripePriceId: 'price_new',
    })

    expect(updates).toEqual([{ key: 'StripePriceId', value: 'price_new' }])
  })
})

describe('buildCreemSettingsUpdates', () => {
  const creemSettings: CreemSettingsValues = {
    CreemApiKey: 'stored-creem-api-key',
    CreemWebhookSecret: 'stored-creem-webhook-secret',
    CreemTestMode: false,
    CreemProducts: '[{"name":"Basic","productId":"prod_old"}]',
  }

  test('does not clear Creem secrets when the fields are blank', () => {
    const updates = buildCreemSettingsUpdates(creemSettings, {
      ...creemSettings,
      CreemApiKey: '',
      CreemWebhookSecret: '',
      CreemTestMode: true,
    })

    expect(updates).toEqual([{ key: 'CreemTestMode', value: true }])
  })
})

describe('orderVipActivationCommissionUpdates', () => {
  test('keeps unrelated updates before ordered VVIP amount updates', () => {
    const updates = orderVipActivationCommissionUpdates(
      [
        { key: 'Price', value: 8 },
        { key: VIP_ACTIVATION_PRICE_KEY, value: 1200 },
        { key: VIP_ACTIVATION_LEVEL1_AMOUNT_KEY, value: 700 },
      ],
      {
        VipActivationPrice: 1680,
        VvipActivationCommissionLevel1Amount: 1000,
        VvipActivationCommissionLevel2Amount: 400,
      },
      {
        VipActivationPrice: 1200,
        VvipActivationCommissionLevel1Amount: 700,
        VvipActivationCommissionLevel2Amount: 400,
      }
    )

    expect(updates.map((update) => update.key)).toEqual([
      'Price',
      VIP_ACTIVATION_LEVEL1_AMOUNT_KEY,
      VIP_ACTIVATION_PRICE_KEY,
    ])
  })
})
