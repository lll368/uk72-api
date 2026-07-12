/*
Copyright (C) 2025 QuantumNous

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

import { describe, expect, test } from 'bun:test';
import {
  formatQiniuMarketPriceItems,
  getQiniuMarketPriceDisplayItems,
  getQiniuMarketRatioDisplayItems,
  hasQiniuMarketPricing,
  qiniuMarketVisibleText,
  sanitizePricingModelForDisplay,
} from './qiniuMarket';

const qiniuModel = {
  model_name: 'kimi-k2',
  model_ratio: 999,
  completion_ratio: 999,
  price_source: 'qiniu_market',
  price_source_label: '七牛官方市场价',
  qiniu_market: {
    id: 'kimi-k2',
    name: 'Kimi K2',
    description: 'official description',
    hot_tags: ['上新'],
    features: ['工具调用'],
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
};

const publicMarketModel = {
  model_name: 'kimi-k2',
  model_ratio: 999,
  completion_ratio: 999,
  price_source_label: '官方市场价',
  market_pricing: {
    id: 'kimi-k2',
    name: 'Kimi K2',
    description: 'Qiniu 七牛 official description',
    hot_tags: ['qiniu_market', '七牛上新'],
    features: ['工具调用', 'Qiniu realtime'],
    pricing_rules_v2: qiniuModel.qiniu_market.pricing_rules_v2,
  },
};

describe('classic qiniu market pricing helpers', () => {
  test('formats qiniu pricing rules instead of local ratio fields', () => {
    const items = formatQiniuMarketPriceItems(qiniuModel);

    expect(hasQiniuMarketPricing(qiniuModel)).toBe(true);
    expect(items.map((item) => item.label)).toEqual(['输入', '输出']);
    expect(items[0].formatted).toBe('¥0.004');
    expect(items[0].unitLabel).toBe('/ 1K token');
  });

  test('formats supplier-neutral public market pricing payloads', () => {
    const items = formatQiniuMarketPriceItems(publicMarketModel);

    expect(hasQiniuMarketPricing(publicMarketModel)).toBe(true);
    expect(items.map((item) => item.label)).toEqual(['输入', '输出']);
    expect(items[0].formatted).toBe('¥0.004');
    expect(
      getQiniuMarketRatioDisplayItems(publicMarketModel, (key) => key),
    ).toEqual([
      { key: 'model-ratio', label: '模型倍率', value: '无' },
      { key: 'completion-ratio', label: '补全倍率', value: '无' },
      { key: 'group-ratio', label: '分组倍率', value: '无' },
    ]);
  });

  test('does not expose hidden source label as visible text', () => {
    expect(qiniuMarketVisibleText(qiniuModel)).not.toContain('七牛官方市场价');
  });

  test('does not expose supplier brand as visible text', () => {
    const text = qiniuMarketVisibleText(publicMarketModel);

    expect(text).not.toContain('七牛');
    expect(text).not.toContain('Qiniu');
    expect(text).not.toContain('qiniu_');
  });

  test('sanitizes supplier brand from classic pricing model display fields', () => {
    const model = sanitizePricingModelForDisplay(
      {
        model_name: 'kimi-k2',
        description: 'QiNiu 七牛 catalog model',
        tags: 'qiniu_market,七牛上新,tools',
        icon: 'https://static.qiniu.com/model.png',
      },
      {
        id: 1,
        name: 'Qiniu vendor',
        icon: 'https://assets.qiniu.com/vendor.png',
        description: 'QiNiu 七牛 vendor',
      },
    );

    const visibleText = JSON.stringify({
      description: model.description,
      tags: model.tags,
      vendorName: model.vendor_name,
      vendorDescription: model.vendor_description,
    });

    expect(visibleText).not.toContain('七牛');
    expect(visibleText.toLowerCase()).not.toContain('qiniu');
    expect(model.tags).toContain('market');
    expect(model.icon).toBe('https://static.qiniu.com/model.png');
    expect(model.vendor_icon).toBe('https://assets.qiniu.com/vendor.png');
  });

  test('skips missing unit price and keeps explicit zero', () => {
    const model = {
      ...qiniuModel,
      qiniu_market: {
        ...qiniuModel.qiniu_market,
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
    };

    const items = formatQiniuMarketPriceItems(model);
    expect(items.map((item) => item.label)).toEqual(['免费额度']);
    expect(items[0].formatted).toBe('¥0');
  });

  test('reports missing rules as unavailable', () => {
    const model = {
      ...qiniuModel,
      qiniu_market: {
        ...qiniuModel.qiniu_market,
        pricing_rules_v2: [],
      },
    };

    expect(hasQiniuMarketPricing(model)).toBe(false);
    expect(formatQiniuMarketPriceItems(model)).toEqual([]);
    expect(getQiniuMarketPriceDisplayItems(model, (key) => key)).toEqual([
      {
        key: 'qiniu-market-unavailable',
        label: '价格',
        value: '—',
        suffix: '',
      },
    ]);
  });

  test('hides local ratio fields for qiniu market models', () => {
    const items = getQiniuMarketRatioDisplayItems(qiniuModel, (key) => key);

    expect(items).toEqual([
      { key: 'model-ratio', label: '模型倍率', value: '无' },
      { key: 'completion-ratio', label: '补全倍率', value: '无' },
      { key: 'group-ratio', label: '分组倍率', value: '无' },
    ]);
    expect(JSON.stringify(items)).not.toContain('999');
  });
});
