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
  getPiggyActualReceivedAmount,
  getPiggyPlatformFeeAmount,
  getPiggyProviderFeeAmount,
  getPiggyTaxBeforeAmount,
  getPiggyTaxDetails,
  hasPiggyPlatformFeeSnapshot,
} from '@/features/wallet/lib/piggy-withdraw-amounts'

describe('WithdrawOrdersTable Piggy amount semantics', () => {
  test('separates local platform fee from provider fee fields', () => {
    const order = {
      provider: 'piggy_labor_v3',
      amount: 100,
      platform_fee_rate: 8,
      platform_fee_amount_cents: 800,
      tax_before_amount_cents: 9200,
      piggy_after_tax_amount_cents: 8744,
      piggy_fee_amount_cents: 35,
      fee_amount: 0.35,
      actual_amount: 87.44,
    }

    expect(hasPiggyPlatformFeeSnapshot(order)).toBe(true)
    expect(getPiggyPlatformFeeAmount(order)).toBe(8)
    expect(getPiggyTaxBeforeAmount(order)).toBe(92)
    expect(getPiggyProviderFeeAmount(order)).toBe(0.35)
    expect(getPiggyActualReceivedAmount(order)).toBe(87.44)
  })

  test('treats zero platform fee fields as valid display values', () => {
    const order = {
      provider: 'piggy_labor_v3',
      amount: 100,
      platform_fee_rate: 0,
      platform_fee_amount_cents: 0,
      tax_before_amount_cents: 10000,
      piggy_after_tax_amount_cents: 0,
      piggy_fee_amount_cents: 0,
      fee_amount: 0,
      actual_amount: 0,
    }

    expect(hasPiggyPlatformFeeSnapshot(order)).toBe(true)
    expect(getPiggyPlatformFeeAmount(order)).toBe(0)
    expect(getPiggyTaxBeforeAmount(order)).toBe(100)
    expect(getPiggyProviderFeeAmount(order)).toBe(0)
    expect(getPiggyActualReceivedAmount(order)).toBe(0)
  })

  test('falls back to legacy provider amounts when Piggy cent fields are empty', () => {
    const order = {
      provider: 'piggy_labor_v3',
      amount: 100,
      platform_fee_rate: 0,
      platform_fee_amount_cents: 0,
      tax_before_amount_cents: 10000,
      piggy_after_tax_amount_cents: 0,
      piggy_fee_amount_cents: 0,
      fee_amount: 0.35,
      actual_amount: 87.44,
    }

    expect(getPiggyPlatformFeeAmount(order)).toBe(0)
    expect(getPiggyProviderFeeAmount(order)).toBe(0.35)
    expect(getPiggyActualReceivedAmount(order)).toBe(87.44)
  })

  test('falls back to callback pretax amount when submission snapshot is missing', () => {
    const order = {
      provider: 'piggy_labor_v3',
      amount: 100,
      tax_before_amount_cents: 0,
      piggy_pay_amount_cents: 0,
      piggy_pretax_amount_cents: 9200,
    }

    expect(getPiggyTaxBeforeAmount(order)).toBe(92)
  })

  test('keeps individual tax and VAT separated for admin display', () => {
    const order = {
      provider: 'piggy_labor_v3',
      piggy_individual_tax_cents: 350,
      piggy_added_tax_cents: 106,
    }

    expect(getPiggyTaxDetails(order)).toEqual({
      individualTaxAmount: 3.5,
      addedTaxAmount: 1.06,
      totalTaxAmount: 4.56,
    })
  })
})
