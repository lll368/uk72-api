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
import { SORT_OPTIONS } from '../constants'
import type { PricingModel } from '../types'
import { sortModels } from './filters'

function qiniuModel(
  modelName: string,
  unitPrice: number | null,
  localRatio: number
): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: localRatio,
    completion_ratio: localRatio,
    enable_groups: ['default'],
    market_pricing: {
      id: modelName,
      pricing_rules_v2:
        unitPrice === null
          ? []
          : [
              {
                details_v2: {
                  input: {
                    unit_name: 'token',
                    unit_size: 1000,
                    unit_price: unitPrice,
                    name: '输入',
                  },
                },
              },
            ],
    },
  }
}

describe('pricing filters', () => {
  test('sorts qiniu market models by qiniu price instead of local ratio', () => {
    const expensiveLocalCheapQiniu = qiniuModel('cheap-qiniu', 0.001, 100)
    const cheapLocalExpensiveQiniu = qiniuModel('expensive-qiniu', 0.02, 1)
    const missingQiniuPrice = qiniuModel('missing-qiniu-price', null, 0)

    const lowToHigh = sortModels(
      [missingQiniuPrice, cheapLocalExpensiveQiniu, expensiveLocalCheapQiniu],
      SORT_OPTIONS.PRICE_LOW
    )
    assert.deepEqual(
      lowToHigh.map((model) => model.model_name),
      ['cheap-qiniu', 'expensive-qiniu', 'missing-qiniu-price']
    )

    const highToLow = sortModels(
      [missingQiniuPrice, cheapLocalExpensiveQiniu, expensiveLocalCheapQiniu],
      SORT_OPTIONS.PRICE_HIGH
    )
    assert.deepEqual(
      highToLow.map((model) => model.model_name),
      ['expensive-qiniu', 'cheap-qiniu', 'missing-qiniu-price']
    )
  })
})
