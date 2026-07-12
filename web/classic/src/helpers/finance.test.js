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
  buildFinanceQuery,
  createFinanceFilterChangeHandler,
  formatFinanceTimestamp,
  getFinancePaymentMethodLabel,
  getFinanceStatusKey,
  getClassicWithdrawActions,
  parseProviderOrders,
} from './finance.js';

describe('finance helper', () => {
  test('builds query and skips empty values', () => {
    expect(
      buildFinanceQuery({
        p: 1,
        page_size: 20,
        user_id: '',
        status: 'pending',
      }),
    ).toBe('?p=1&page_size=20&status=pending');
  });

  test('formats empty timestamp as dash', () => {
    expect(formatFinanceTimestamp(0)).toBe('-');
    expect(formatFinanceTimestamp(undefined)).toBe('-');
  });

  test('maps known statuses to Chinese translation keys', () => {
    expect(getFinanceStatusKey('success')).toBe('成功');
    expect(getFinanceStatusKey('pending')).toBe('待处理');
    expect(getFinanceStatusKey('custom')).toBe('custom');
  });

  test('parses provider orders JSON array', () => {
    expect(
      parseProviderOrders(
        '[{"trade_no":"t1","biz_type":"topup","paid_amount":10,"status":"success"}]',
      ),
    ).toEqual([
      {
        trade_no: 't1',
        biz_type: 'topup',
        paid_amount: 10,
        status: 'success',
      },
    ]);
  });

  test('resets pagination when a finance filter changes', () => {
    const calls = [];
    const handler = createFinanceFilterChangeHandler(
      (page) => calls.push(['page', page]),
      (value) => calls.push(['value', value]),
    );

    handler('pending');

    expect(calls).toEqual([
      ['page', 1],
      ['value', 'pending'],
    ]);
  });

  test('maps payment methods to friendly labels for finance views', () => {
    expect(getFinancePaymentMethodLabel('alipay')).toBe('支付宝');
    expect(getFinancePaymentMethodLabel('alipay_direct')).toBe('支付宝');
    expect(getFinancePaymentMethodLabel('wechat_direct')).toBe('微信');
    expect(getFinancePaymentMethodLabel('custom_method')).toBe('custom_method');
  });

  test('blocks legacy finance actions for Piggy withdrawals in classic frontend', () => {
    expect(
      getClassicWithdrawActions({
        provider: 'piggy_labor_v3',
        status: 'approved',
      }),
    ).toEqual([]);
  });

  test('keeps legacy finance actions for manual withdrawals', () => {
    expect(
      getClassicWithdrawActions({
        provider: 'manual',
        status: 'approved',
      }),
    ).toEqual(['pay', 'fail', 'reject']);
  });
});
