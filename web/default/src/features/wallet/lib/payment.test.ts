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
import { getPaymentMethodName } from './billing'
import {
  filterVisiblePaymentMethods,
  getEffectiveTopupDiscount,
  getMinTopupAmount,
  getPaymentMethodDisplayName,
  getWalletPaymentDispatch,
  isSubscriptionEpayPaymentMethod,
  isAlipayDirectPayment,
  isWechatDirectPayment,
  submitPaymentForm,
} from './payment'

type FakeInputElement = {
  type: string
  name: string
  value: string
}

type FakeFormElement = {
  action: string
  method: string
  target?: string
  children: FakeInputElement[]
  appendChild: (input: FakeInputElement) => void
  submit: () => void
}

function withFakePaymentDocument(
  userAgent: string,
  callback: () => void
): FakeFormElement | null {
  const originalDocument = globalThis.document
  const originalNavigator = globalThis.navigator
  let submittedForm: FakeFormElement | null = null

  const fakeDocument = {
    createElement: (tag: string) => {
      if (tag === 'form') {
        const form: FakeFormElement = {
          action: '',
          method: '',
          children: [],
          appendChild(input: FakeInputElement) {
            this.children.push(input)
          },
          submit() {
            submittedForm = this
          },
        }
        return form
      }
      return {
        type: '',
        name: '',
        value: '',
      }
    },
    body: {
      appendChild: () => undefined,
      removeChild: () => undefined,
    },
  }

  Object.defineProperty(globalThis, 'document', {
    value: fakeDocument,
    configurable: true,
  })
  Object.defineProperty(globalThis, 'navigator', {
    value: { userAgent },
    configurable: true,
  })

  try {
    callback()
    return submittedForm
  } finally {
    Object.defineProperty(globalThis, 'document', {
      value: originalDocument,
      configurable: true,
    })
    Object.defineProperty(globalThis, 'navigator', {
      value: originalNavigator,
      configurable: true,
    })
  }
}

describe('getEffectiveTopupDiscount', () => {
  test('uses configured amount discount when no user discount exists', () => {
    expect(getEffectiveTopupDiscount(100, { 100: 0.9 }, 0)).toBe(0.9)
  })

  test('user topup discount overrides configured amount discount', () => {
    expect(getEffectiveTopupDiscount(100, { 100: 0.9 }, 0.8)).toBe(0.8)
  })

  test('user topup discount of one falls back to amount discount', () => {
    expect(getEffectiveTopupDiscount(100, { 100: 0.9 }, 1)).toBe(0.9)
  })

  test('dispatches direct Alipay separately from epay Alipay', () => {
    expect(isAlipayDirectPayment('alipay_direct')).toBe(true)
    expect(isAlipayDirectPayment('alipay')).toBe(false)
    expect(getWalletPaymentDispatch('alipay_direct')).toBe('alipay_direct')
    expect(getWalletPaymentDispatch('alipay')).toBe('standard_form')
  })

  test('honors explicit current-tab payment form submission', () => {
    const submittedForm = withFakePaymentDocument('Chrome', () => {
      submitPaymentForm(
        'https://openapi.alipay.com/gateway.do?charset=utf-8',
        { out_trade_no: 'USR320NOcTcmDx1782545079' },
        { target: '_self' }
      )
    })

    expect(submittedForm?.target).toBe('_self')
    expect(submittedForm?.children).toEqual([
      {
        type: 'hidden',
        name: 'out_trade_no',
        value: 'USR320NOcTcmDx1782545079',
      },
    ])
  })

  test('localizes direct Alipay label as Alipay without changing epay Alipay name', () => {
    const t = (key: string) => `translated:${key}`

    expect(
      getPaymentMethodDisplayName(
        { name: '支付宝直连', type: 'alipay_direct' },
        t
      )
    ).toBe('translated:Alipay')
    expect(
      getPaymentMethodDisplayName({ name: '支付宝', type: 'alipay' }, t)
    ).toBe('支付宝')
  })

  test('dispatches direct WeChat Pay separately from epay WeChat Pay', () => {
    expect(isWechatDirectPayment('wechat_direct')).toBe(true)
    expect(isWechatDirectPayment('wxpay')).toBe(false)
    expect(getWalletPaymentDispatch('wechat_direct')).toBe('wechat_direct')
    expect(getWalletPaymentDispatch('wxpay')).toBe('standard_form')
  })

  test('localizes direct WeChat Pay label as WeChat without changing epay WeChat Pay name', () => {
    const t = (key: string) => `translated:${key}`

    expect(
      getPaymentMethodDisplayName(
        { name: '微信支付直连', type: 'wechat_direct' },
        t
      )
    ).toBe('translated:WeChat')
    expect(
      getPaymentMethodDisplayName({ name: '微信支付', type: 'wxpay' }, t)
    ).toBe('微信支付')
  })

  test('hides epay Alipay and WeChat payment methods from user-facing lists', () => {
    const visible = filterVisiblePaymentMethods([
      { name: '支付宝', type: 'alipay' },
      { name: '微信', type: 'wxpay' },
      { name: '支付宝直连', type: 'alipay_direct' },
      { name: '微信支付直连', type: 'wechat_direct' },
      { name: 'Custom', type: 'custom' },
    ])

    expect(visible.map((method) => method.type)).toEqual([
      'alipay_direct',
      'wechat_direct',
      'custom',
    ])
  })

  test('keeps subscription epay options limited to real epay methods', () => {
    const types = [
      'alipay',
      'wxpay',
      'alipay_direct',
      'wechat_direct',
      'stripe',
      'creem',
      'waffo',
      'waffo_pancake',
      'custom',
    ]

    expect(
      types.filter((type) => isSubscriptionEpayPaymentMethod({ type }))
    ).toEqual(['custom'])
  })

  test('localizes billing payment method names through the unified wallet labels', () => {
    const t = (key: string) => `translated:${key}`

    expect(getPaymentMethodName('alipay', t)).toBe('translated:Alipay')
    expect(getPaymentMethodName('alipay_direct', t)).toBe('translated:Alipay')
    expect(getPaymentMethodName('wechat_direct', t)).toBe('translated:WeChat')
  })

  test('falls back to default minimum when no real payment methods are available', () => {
    const topupInfo = {
      enable_online_topup: false,
      enable_stripe_topup: false,
      enable_waffo_topup: false,
      enable_waffo_pancake_topup: false,
      min_topup: 10,
    }

    expect(getMinTopupAmount(topupInfo as never)).toBe(1)
  })
})
