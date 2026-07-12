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
import type { PricingModel } from '@/features/pricing/types'
import type { UsageLog } from '../data/schema'
import {
  findQiniuMarketPricingModel,
  formatModelName,
  isQiniuCostDetailBucketLog,
  getQiniuMarketRealtimePriceItems,
  getQiniuMarketRealtimeBillingModeLabel,
  getQiniuOfficialLedgerPriceItems,
  getUsageLogDisplayContent,
  isQiniuLocalOfficialLedgerObservation,
  isQiniuMarketRealtimeLog,
  isQiniuOfficialLedgerLog,
  sanitizeQiniuOfficialLedgerContent,
} from './format'

const qiniuModel: PricingModel = {
  id: 1,
  model_name: 'local-display-name',
  key: 'local-display-name',
  quota_type: 0,
  model_ratio: 37.5,
  completion_ratio: 1,
  enable_groups: ['default'],
  market_pricing: {
    id: 'qiniu-upstream-model',
    name: 'Qiniu Upstream Model',
    pricing_rules_v2: [
      {
        details_v2: {
          input: {
            unit_name: 'token',
            unit_size: 1000,
            unit_price: 0.004,
            name: '输入',
          },
          output: {
            unit_name: 'token',
            unit_size: 1000,
            unit_price: 0.016,
            name: '输出',
          },
        },
      },
    ],
  },
}

const baseLog: UsageLog = {
  id: 1,
  user_id: 2,
  created_at: 1710000000,
  type: 2,
  content: '账单待同步',
  username: 'alice',
  token_name: '',
  model_name: 'fallback-model',
  quota: 0,
  prompt_tokens: 100,
  completion_tokens: 200,
  use_time: 0,
  is_stream: false,
  channel: 0,
  channel_name: '',
  token_id: 0,
  group: '',
  ip: '',
  other: '',
  request_id: '',
  upstream_request_id: '',
}

describe('qiniu official ledger usage log helpers', () => {
  test('detects local observations and official synced ledger logs separately', () => {
    expect(
      isQiniuLocalOfficialLedgerObservation({
        billing_source: 'qiniu_official_ledger',
        qiniu_official_ledger_pending: true,
        qiniu_official_ledger_log: false,
      })
    ).toBe(true)
    expect(
      isQiniuLocalOfficialLedgerObservation({
        billing_source: 'qiniu_official_ledger',
        qiniu_official_ledger_pending: true,
        qiniu_official_ledger_log: true,
      })
    ).toBe(false)
    expect(
      isQiniuOfficialLedgerLog({
        billing_source: 'qiniu_official_ledger',
        qiniu_official_ledger_log: true,
      })
    ).toBe(true)
  })

  test('matches qiniu market pricing by upstream model before local log model name', () => {
    const matched = findQiniuMarketPricingModel(
      baseLog,
      {
        billing_source: 'qiniu_official_ledger',
        qiniu_official_ledger_pending: true,
        upstream_model_name: 'qiniu-upstream-model',
      },
      [qiniuModel]
    )

    expect(matched).toBe(qiniuModel)
  })

  test('formats qiniu official input and output prices per million tokens', () => {
    const prices = getQiniuOfficialLedgerPriceItems(qiniuModel)

    expect(prices).toEqual([
      { detailKey: 'input', formatted: '¥4' },
      { detailKey: 'output', formatted: '¥16' },
    ])
  })

  test('removes official sync content prefixes without changing local observation content', () => {
    expect(
      sanitizeQiniuOfficialLedgerContent(
        '七牛官方用量同步消费：2026-06-03 kimi-k2 input'
      )
    ).toBe('2026-06-03 kimi-k2 input')
    expect(
      sanitizeQiniuOfficialLedgerContent(
        '七牛官方用量同步退款：2026-06-03 kimi-k2 output'
      )
    ).toBe('2026-06-03 kimi-k2 output')
    expect(
      sanitizeQiniuOfficialLedgerContent(
        '官方用量同步消费：2026-06-03 kimi-k2 input'
      )
    ).toBe('2026-06-03 kimi-k2 input')
    expect(sanitizeQiniuOfficialLedgerContent('账单待同步')).toBe('账单待同步')
  })

  test('detects and formats qiniu market realtime billing logs', () => {
    const other = {
      billing_source: 'qiniu_market_realtime',
      price_source: 'qiniu_market',
      token_provider: 'qiniu',
      qiniu_market_input_unit_name: 'token',
      qiniu_market_input_unit_size: 1000,
      qiniu_market_input_unit_price: 0.004,
      qiniu_market_input_currency: 'CNY',
      qiniu_market_output_unit_name: 'token',
      qiniu_market_output_unit_size: 1000,
      qiniu_market_output_unit_price: 0.016,
      qiniu_market_output_currency: 'CNY',
      qiniu_market_catalog_state: 'fresh',
    }

    expect(isQiniuMarketRealtimeLog(other)).toBe(true)
    expect(isQiniuLocalOfficialLedgerObservation(other)).toBe(false)
    expect(getQiniuMarketRealtimePriceItems(other)).toEqual([
      { detailKey: 'input', formatted: '¥0.004/k' },
      { detailKey: 'output', formatted: '¥0.016/k' },
    ])
    expect(getQiniuMarketRealtimeBillingModeLabel(other)).toBe('Per-token')
    expect(getUsageLogDisplayContent(baseLog, other)).toBe(
      '市场价实时扣费 · 账单待同步'
    )
    expect(getUsageLogDisplayContent(baseLog, other).includes('qiniu')).toBe(
      false
    )

    expect(
      getUsageLogDisplayContent(
        {
          ...baseLog,
          content: 'deepseek/deepseek-v4-flash，市场价实时扣费',
        },
        other
      )
    ).toBe('deepseek/deepseek-v4-flash，市场价实时扣费')
  })

  test('formats qiniu market realtime unit prices without token million suffix', () => {
    const other = {
      billing_source: 'qiniu_market_realtime',
      price_source: 'qiniu_market',
      token_provider: 'qiniu',
      qiniu_market_billing_mode: 'unit',
      qiniu_market_unit_detail_key: 'request',
      qiniu_market_unit_name: 'request',
      qiniu_market_unit_size: 1,
      qiniu_market_unit_price: 0.01,
      qiniu_market_unit_currency: 'CNY',
      qiniu_market_unit_quantity: 1,
      qiniu_market_catalog_state: 'fresh',
    }

    expect(getQiniuMarketRealtimePriceItems(other)).toEqual([
      { detailKey: 'unit', formatted: '¥0.01/request' },
    ])
    expect(getQiniuMarketRealtimeBillingModeLabel(other)).toBe('Per-call')
  })

  test('detects and labels qiniu cost-detail bucket settlement logs', () => {
    const other = {
      billing_source: 'qiniu_cost_detail_bucket',
      bucket_id: 12,
      billing_date: '2026-06-01',
      qiniu_masked_key: 'sk-abcd****wxyz',
      official_quota: 2000,
      local_realtime_quota: 500,
      delta_quota: 1500,
      balance_after_quota: -100,
      debt_quota: 100,
      debt: true,
    }

    expect(isQiniuCostDetailBucketLog(other)).toBe(true)
    expect(getUsageLogDisplayContent(baseLog, other)).toBe(
      '账单延迟对账调整 · 账单待同步'
    )
    expect(getUsageLogDisplayContent(baseLog, other).includes('qiniu')).toBe(
      false
    )
  })

  test('maps internal bucket settlement model names away from user display', () => {
    expect(
      formatModelName({
        ...baseLog,
        model_name: 'qiniu_cost_detail_bucket',
      }).name
    ).toBe('billing-settlement')
  })
})
