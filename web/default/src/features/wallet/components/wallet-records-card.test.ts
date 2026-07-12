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
import zhCN from '../../../i18n/locales/zh-CN.json'
import {
  getPiggyPlatformFeeAmount,
  getPiggyTaxBeforeAmount,
} from '../lib/piggy-withdraw-amounts'
import type { WalletFlow } from '../types'
import {
  getWalletFlowLabel,
  walletFlowTableHeaders,
} from './wallet-records-card'

const t = (key: string) => `t:${key}`

function flow(value: Partial<WalletFlow>): WalletFlow {
  return {
    id: 1,
    user_id: 1,
    biz_no: '',
    flow_type: 'balance_consume',
    wallet_type: 'balance',
    direction: 'out',
    amount: 0,
    balance_after: 0,
    commission_after: 0,
    frozen_commission_after: 0,
    remark: '',
    created_at: 0,
    ...value,
  }
}

describe('getWalletFlowLabel', () => {
  test('keeps wallet flow table focused on balance records', () => {
    const headers: readonly string[] = walletFlowTableHeaders

    expect(walletFlowTableHeaders).toEqual([
      'Type',
      'Amount',
      'Balance after',
      'Remark',
      'Time',
    ])
    expect(headers.includes('Business No.')).toBe(false)
    expect(headers.includes('Commission after')).toBe(false)
    expect(headers.includes('Frozen commission after')).toBe(false)
  })

  test('labels realtime platform usage as token/model consumption', () => {
    expect(
      getWalletFlowLabel(t, flow({ biz_no: 'qiniu:realtime:request:req-1' }))
    ).toBe('t:Token/model consumption')
  })

  test('labels official sync consumption separately from realtime usage', () => {
    expect(
      getWalletFlowLabel(t, flow({ biz_no: 'qiniu:official_ledger:record-1' }))
    ).toBe('t:Official synchronized consumption')
  })

  test('labels legacy official usage apply flows from backend idempotency keys', () => {
    expect(
      getWalletFlowLabel(
        t,
        flow({
          biz_no: 'qiniu:usage_apply:42:v1',
          remark: '官方用量同步消费：kimi-k2/input',
        })
      )
    ).toBe('t:Official synchronized consumption')
  })

  test('labels direct-sync bucket flows as official synchronized consumption', () => {
    expect(
      getWalletFlowLabel(
        t,
        flow({
          biz_no: 'qiniu:billing_bucket:42:1',
          remark: '官方同步账单延迟对账补扣 date=2026-06-05 delta=1000',
        })
      )
    ).toBe('t:Official synchronized consumption')
  })

  test('keeps delayed bucket settlement distinct', () => {
    expect(
      getWalletFlowLabel(t, flow({ biz_no: 'qiniu:billing_bucket:42:1' }))
    ).toBe('t:Delayed billing settlement')
  })

  test('keeps wallet labels supplier-neutral in the active translation namespace', () => {
    expect(zhCN.translation['Token/model consumption']).toBe('token/model 消费')
    expect(zhCN.translation['Official synchronized consumption']).toBe(
      '官方同步消费'
    )
  })

  test('uses Piggy platform fee snapshots and displays zero for legacy platform fees', () => {
    expect(
      getPiggyPlatformFeeAmount({
        provider: 'piggy_labor_v3',
        amount: 100,
        tax_before_amount_cents: 9200,
        platform_fee_rate: 8,
        platform_fee_amount_cents: 800,
      })
    ).toBe(8)
    expect(
      getPiggyTaxBeforeAmount({
        provider: 'piggy_labor_v3',
        tax_before_amount_cents: 9200,
      })
    ).toBe(92)
    expect(
      getPiggyPlatformFeeAmount({
        provider: 'piggy_labor_v3',
        amount: 100,
        tax_before_amount_cents: 10000,
        platform_fee_rate: 0,
        platform_fee_amount_cents: 0,
      })
    ).toBe(0)
  })
})
