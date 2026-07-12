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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import type { PricingModel } from '../types'
import {
  formatQiniuMarketPriceItems,
  hasQiniuMarketPricing,
  qiniuMarketVisibleText,
  sanitizeSupplierBrandUrl,
} from './qiniu-market'

const qiniuModel: PricingModel = {
  id: 1,
  model_name: 'kimi-k2',
  quota_type: 0,
  model_ratio: 999,
  completion_ratio: 999,
  enable_groups: ['default'],
  price_source_label: '官方市场价',
  market_pricing: {
    id: 'kimi-k2',
    name: 'Kimi K2',
    description: 'Qiniu 七牛 official description',
    hot_tags: ['qiniu_market', '七牛上新'],
    features: ['工具调用', 'Qiniu realtime'],
    pricing_rules_v2: [
      {
        input_range: [0, 99999999],
        output_range: [0, 99999999],
        details_v2: {
          input: {
            unit_name: 'token',
            unit_size: 1000,
            unit_price: 0.004,
            unit_price_usd: 0.00056,
            name: '输入',
          },
          output: {
            unit_name: 'token',
            unit_size: 1000,
            unit_price: 0.016,
            unit_price_usd: 0.00222,
            name: '输出',
          },
        },
      },
    ],
  },
}

describe('qiniu market pricing helpers', () => {
  test('formats qiniu pricing_rules_v2 instead of local ratio fields', () => {
    const items = formatQiniuMarketPriceItems(qiniuModel, { tokenUnit: 'K' })

    assert.equal(hasQiniuMarketPricing(qiniuModel), true)
    assert.deepEqual(
      items.map((item) => item.label),
      ['输入', '输出']
    )
    assert.deepEqual(
      items.map((item) => item.detailKey),
      ['input', 'output']
    )
    assert.equal(items[0].formatted, '¥0.004')
    assert.equal(items[0].unitLabel, '/ 1K token')
    assert.equal(items[1].formatted, '¥0.016')
  })

  test('converts qiniu token prices by display unit only', () => {
    const millionItems = formatQiniuMarketPriceItems(qiniuModel, {
      tokenUnit: 'M',
      priceRate: 4,
      usdExchangeRate: 7,
    })
    assert.equal(millionItems[0].formatted, '¥4')
    assert.equal(millionItems[0].unitLabel, '/ 1M token')

    const rechargeItems = formatQiniuMarketPriceItems(qiniuModel, {
      tokenUnit: 'M',
      showRechargePrice: true,
      priceRate: 4,
      usdExchangeRate: 7,
    })
    assert.equal(rechargeItems[0].formatted, '¥4')
  })

  test('treats qiniu token price without unit_size as per-token price', () => {
    const model: PricingModel = {
      ...qiniuModel,
      market_pricing: {
        ...qiniuModel.market_pricing!,
        pricing_rules_v2: [
          {
            details_v2: {
              input: {
                unit_name: 'token',
                unit_price: 0.000001,
                name: '输入',
              },
            },
          },
        ],
      },
    }

    const items = formatQiniuMarketPriceItems(model, { tokenUnit: 'M' })

    assert.equal(items[0].formatted, '¥1')
    assert.equal(items[0].unitLabel, '/ 1M token')
  })

  test('does not expose supplier brand as visible text', () => {
    const text = qiniuMarketVisibleText(qiniuModel)

    assert.equal(text.includes('七牛'), false)
    assert.equal(text.includes('Qiniu'), false)
    assert.equal(text.includes('qiniu_'), false)
  })

  test('keeps supplier asset URLs because they are not visible copy', () => {
    assert.equal(
      sanitizeSupplierBrandUrl(' https://static.qiniu.com/model.png '),
      'https://static.qiniu.com/model.png'
    )
  })

  test('reports missing pricing rules as unavailable', () => {
    const model = {
      ...qiniuModel,
      market_pricing: {
        ...qiniuModel.market_pricing!,
        pricing_rules_v2: [],
      },
    }

    assert.equal(hasQiniuMarketPricing(model), false)
    assert.deepEqual(formatQiniuMarketPriceItems(model), [])
  })

  test('skips missing unit_price but preserves explicit zero price', () => {
    const model: PricingModel = {
      ...qiniuModel,
      market_pricing: {
        ...qiniuModel.market_pricing!,
        pricing_rules_v2: [
          {
            details_v2: {
              missing: {
                unit_name: 'token',
                unit_size: 1000,
                name: '缺失价格',
              },
              null_price: {
                unit_name: 'token',
                unit_size: 1000,
                unit_price: null,
                name: '空价格',
              },
              nan_price: {
                unit_name: 'token',
                unit_size: 1000,
                unit_price: Number.NaN,
                name: '非法价格',
              },
              free: {
                unit_name: 'token',
                unit_size: 1000,
                unit_price: 0,
                name: '免费额度',
              },
            },
          },
        ],
      },
    }

    const items = formatQiniuMarketPriceItems(model)
    assert.deepEqual(
      items.map((item) => item.label),
      ['免费额度']
    )
    assert.equal(items[0].formatted, '¥0')
  })
})
