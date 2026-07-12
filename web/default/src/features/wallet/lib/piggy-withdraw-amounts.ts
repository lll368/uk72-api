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

export const PIGGY_WITHDRAW_PROVIDER = 'piggy_labor_v3'

export interface PiggyWithdrawAmountLike {
  provider?: string
  amount?: number
  fee_amount?: number
  actual_amount?: number
  platform_fee_rate?: number
  platform_fee_amount_cents?: number
  tax_before_amount_cents?: number
  piggy_pay_amount_cents?: number
  piggy_pretax_amount_cents?: number
  piggy_after_tax_amount_cents?: number
  piggy_individual_tax_cents?: number
  piggy_added_tax_cents?: number
  piggy_fee_amount_cents?: number
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

export function centsToMoneyAmount(value?: number) {
  if (!isFiniteNumber(value) || value <= 0) {
    return undefined
  }
  return value / 100
}

export function nonNegativeCentsToMoneyAmount(value?: number) {
  if (!isFiniteNumber(value) || value < 0) {
    return undefined
  }
  return value / 100
}

function firstPositiveCentsToMoneyAmount(...values: Array<number | undefined>) {
  for (const value of values) {
    const amount = centsToMoneyAmount(value)
    if (amount !== undefined) {
      return amount
    }
  }
  return undefined
}

export function hasPiggyPlatformFeeSnapshot(order: PiggyWithdrawAmountLike) {
  return (
    isFiniteNumber(order.platform_fee_rate) ||
    isFiniteNumber(order.platform_fee_amount_cents)
  )
}

export function getPiggyPlatformFeeAmount(order: PiggyWithdrawAmountLike) {
  if (!hasPiggyPlatformFeeSnapshot(order)) return undefined
  return centsToMoneyAmount(order.platform_fee_amount_cents) ?? 0
}

export function getPiggyTaxBeforeAmount(order: PiggyWithdrawAmountLike) {
  return firstPositiveCentsToMoneyAmount(
    order.tax_before_amount_cents,
    order.piggy_pay_amount_cents,
    order.piggy_pretax_amount_cents
  )
}

export function getPiggyActualReceivedAmount(order: PiggyWithdrawAmountLike) {
  return (
    centsToMoneyAmount(order.piggy_after_tax_amount_cents) ??
    order.actual_amount
  )
}

export function getPiggyProviderFeeAmount(order: PiggyWithdrawAmountLike) {
  return centsToMoneyAmount(order.piggy_fee_amount_cents) ?? order.fee_amount
}

export function getPiggyTaxDetails(order: PiggyWithdrawAmountLike) {
  const individualTaxAmount = nonNegativeCentsToMoneyAmount(
    order.piggy_individual_tax_cents
  )
  const addedTaxAmount = nonNegativeCentsToMoneyAmount(
    order.piggy_added_tax_cents
  )
  const totalTaxCents =
    isFiniteNumber(order.piggy_individual_tax_cents) ||
    isFiniteNumber(order.piggy_added_tax_cents)
      ? Math.max(order.piggy_individual_tax_cents ?? 0, 0) +
        Math.max(order.piggy_added_tax_cents ?? 0, 0)
      : undefined

  return {
    individualTaxAmount,
    addedTaxAmount,
    totalTaxAmount: nonNegativeCentsToMoneyAmount(totalTaxCents),
  }
}
