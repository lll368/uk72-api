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
  findTopupStatusByTradeNo,
  getQrPaymentStatusFromVipActivationOrderStatus,
  walletPiggyContractPreviewPath,
  walletPiggyTaxTrialPath,
} from './api'
import type { TopupRecord } from './types'

function record(value: Partial<TopupRecord>): TopupRecord {
  return {
    id: 1,
    user_id: 1,
    amount: 1,
    money: 1,
    trade_no: 'ORDER-1',
    payment_method: 'wechat_direct',
    create_time: 0,
    complete_time: 0,
    status: 'pending',
    ...value,
  }
}

describe('findTopupStatusByTradeNo', () => {
  test('returns the exact matched order status', () => {
    expect(
      findTopupStatusByTradeNo(
        [
          record({ trade_no: 'ORDER-10', status: 'pending' }),
          record({ trade_no: 'ORDER-1', status: 'success' }),
        ],
        'ORDER-1'
      )
    ).toBe('success')
  })

  test('returns null when the order is missing', () => {
    expect(
      findTopupStatusByTradeNo([record({ trade_no: 'ORDER-10' })], 'ORDER-1')
    ).toBeNull()
  })
})

describe('getQrPaymentStatusFromVipActivationOrderStatus', () => {
  test('maps pending and failed VVIP states to QR payment states', () => {
    expect(getQrPaymentStatusFromVipActivationOrderStatus('pending')).toBe(
      'pending'
    )
    expect(getQrPaymentStatusFromVipActivationOrderStatus('failed')).toBe(
      'failed'
    )
    expect(getQrPaymentStatusFromVipActivationOrderStatus('success')).toBe(
      'success'
    )
  })

  test('does not treat disabled VVIP state as the current QR order status', () => {
    expect(getQrPaymentStatusFromVipActivationOrderStatus('disabled')).toBeNull()
  })
})

describe('wallet Piggy contract preview API', () => {
  test('uses the backend preview endpoint instead of the Piggy raw viewContract URL', () => {
    expect(walletPiggyContractPreviewPath).toBe(
      '/api/wallet/withdraw/piggy/contract-preview'
    )
    expect(
      walletPiggyContractPreviewPath.includes('/contract/sign/viewContract')
    ).toBe(false)
  })
})

describe('wallet Piggy tax trial API', () => {
  test('uses the backend tax trial endpoint instead of the Piggy raw endpoint', () => {
    expect(walletPiggyTaxTrialPath).toBe(
      '/api/wallet/withdraw/piggy/tax-trial'
    )
    expect(walletPiggyTaxTrialPath.includes('/open/payment')).toBe(false)
  })
})
