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
import { renderToStaticMarkup } from 'react-dom/server'
import en from '../../../../i18n/locales/en.json'
import zhCN from '../../../../i18n/locales/zh-CN.json'
import zhTW from '../../../../i18n/locales/zh-TW.json'
import {
  canRequestWithdrawTaxTrial,
  WithdrawSubmissionFields,
} from './withdraw-dialog'

const taxTrialTranslationKeys = [
  'Tax estimate',
  'Estimate tax',
  'Withdrawal amount',
  'Requested amount',
  'Platform fee rate',
  'Platform fee amount',
  'Piggy tax-before amount',
  'Provider fee',
  'Individual tax',
  'VAT',
  'Enter an amount and click Estimate tax to view details',
  'Final tax is based on provider payment result',
  'Tax estimate unavailable',
]

describe('WithdrawDialog', () => {
  test('allows tax estimate above available amount while keeping submit restrictions separate', () => {
    expect(
      canRequestWithdrawTaxTrial({
        open: true,
        hasTaxTrialHandler: true,
        eligibility: {
          enabled: true,
          can_withdraw: false,
          need_profile: false,
          need_sign: false,
          withdrawable_commission: 5,
          frozen_commission: 0,
          commission_min_withdraw_amount: 10,
          cooldown_remaining_seconds: 0,
          disabled_reason: 'Insufficient commission',
          blocking_reasons: ['Insufficient commission'],
        },
        amount: 500,
      })
    ).toBe(true)
  })

  test('stays focused on withdrawal submission instead of contract signing', () => {
    const html = renderToStaticMarkup(
      <WithdrawSubmissionFields
        availableAmount={100}
        minAmount={10}
        eligibility={{
          enabled: true,
          can_withdraw: true,
          need_profile: false,
          need_sign: false,
          withdrawable_commission: 100,
          frozen_commission: 0,
          commission_min_withdraw_amount: 10,
          cooldown_remaining_seconds: 0,
          disabled_reason: '',
          blocking_reasons: [],
        }}
        amount={10}
        remark=''
        onAmountChange={() => {}}
        onRemarkChange={() => {}}
      />
    )

    expect(html).toContain('Amount')
    expect(html).toContain('Remark')
    expect(html.includes('Withdrawal profile')).toBe(false)
    expect(html.includes('Generate signing QR code')).toBe(false)
    expect(html.includes('Open contract')).toBe(false)
  })

  test('renders a manual tax estimate button', () => {
    const html = renderToStaticMarkup(
      <WithdrawSubmissionFields
        availableAmount={100}
        minAmount={10}
        eligibility={{
          enabled: true,
          can_withdraw: true,
          need_profile: false,
          need_sign: false,
          withdrawable_commission: 100,
          frozen_commission: 0,
          commission_min_withdraw_amount: 10,
          cooldown_remaining_seconds: 0,
          disabled_reason: '',
          blocking_reasons: [],
        }}
        amount={10}
        remark=''
        taxTrialActionDisabled={false}
        onTaxTrialClick={() => {}}
        onAmountChange={() => {}}
        onRemarkChange={() => {}}
      />
    )

    expect(html).toContain('Estimate tax')
    expect(html).toContain(
      'Enter an amount and click Estimate tax to view details'
    )
  })

  test('renders Piggy tax trial details before submission', () => {
    const html = renderToStaticMarkup(
      <WithdrawSubmissionFields
        availableAmount={100}
        minAmount={10}
        eligibility={{
          enabled: true,
          can_withdraw: true,
          need_profile: false,
          need_sign: false,
          withdrawable_commission: 100,
          frozen_commission: 0,
          commission_min_withdraw_amount: 10,
          cooldown_remaining_seconds: 0,
          disabled_reason: '',
          blocking_reasons: [],
        }}
        amount={100}
        remark=''
        taxTrial={{
          outer_trade_no: 'PTRIAL1',
          calc_month: '2026-06',
          requested_amount: '100.00',
          requested_amount_cents: 10000,
          platform_fee_rate: 8,
          platform_fee_amount: '8.00',
          platform_fee_amount_cents: 800,
          piggy_tax_before_amount: '92.00',
          piggy_tax_before_amount_cents: 9200,
          pretax_amount: '92.00',
          individual_tax_amount: '3.50',
          added_tax_amount: '1.06',
          after_tax_amount: '87.44',
          calc_type: 'C',
        }}
        taxTrialLoading={false}
        taxTrialError=''
        onAmountChange={() => {}}
        onRemarkChange={() => {}}
      />
    )

    expect(html).toContain('Tax estimate')
    expect(html).toContain('Requested amount')
    expect(html).toContain('Platform fee rate')
    expect(html).toContain('Platform fee amount')
    expect(html).toContain('Piggy tax-before amount')
    expect(html).toContain('Actual received')
    expect(html).toContain('Individual tax')
    expect(html).toContain('VAT')
    expect(html).toContain('¥100')
    expect(html).toContain('8%')
    expect(html).toContain('¥8')
    expect(html).toContain('¥92')
    expect(html).toContain('¥87.44')
    expect(html).toContain('¥3.5')
    expect(html).toContain('¥1.06')
  })

  test('keeps tax trial copy translated in primary locales', () => {
    for (const key of taxTrialTranslationKeys) {
      expect(typeof en.translation[key as keyof typeof en.translation]).toBe(
        'string'
      )
      expect(
        typeof zhCN.translation[key as keyof typeof zhCN.translation]
      ).toBe('string')
      expect(
        zhCN.translation[key as keyof typeof zhCN.translation] !== key
      ).toBe(true)
      expect(
        typeof zhTW.translation[key as keyof typeof zhTW.translation]
      ).toBe('string')
      expect(
        zhTW.translation[key as keyof typeof zhTW.translation] !== key
      ).toBe(true)
    }
  })
})
