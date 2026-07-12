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
import enLocale from '../i18n/locales/en.json';
import {
  buildWalletPageQuery,
  formatWalletMoney,
  getEffectiveTopupDiscount,
  getVisibleWalletPaymentMethods,
  getWalletPaymentMethodLabel,
  getWalletTopUpAmountPath,
  getWalletTopUpPayPath,
  getWalletRecordLoadError,
  getWalletFlowLabelKey,
  isClassicWalletWithdrawSupported,
  submitWalletWithdraw,
  isSubscriptionEpayPaymentMethod,
  isAlipayDirectPayment,
  isWechatDirectPayment,
  isApiSuccess,
} from './wallet.js';

describe('wallet helper', () => {
  test('builds page query with backend pagination names', () => {
    expect(buildWalletPageQuery(2, 20)).toBe('p=2&page_size=20');
  });

  test('formats wallet money with two decimals', () => {
    expect(formatWalletMoney(12)).toBe('12.00');
    expect(formatWalletMoney('3.456')).toBe('3.46');
  });

  test('maps wallet flow labels to translation keys', () => {
    expect(getWalletFlowLabelKey('commission_to_balance')).toBe('佣金转余额');
    expect(getWalletFlowLabelKey('unknown_type')).toBe('unknown_type');
  });

  test('treats success flag or success message as successful API response', () => {
    expect(isApiSuccess({ success: true })).toBe(true);
    expect(isApiSuccess({ message: 'success' })).toBe(true);
    expect(isApiSuccess({ success: false, message: 'failed' })).toBe(false);
  });

  test('returns wallet record business error instead of silently accepting empty data', () => {
    expect(
      getWalletRecordLoadError({
        flows: { success: true, data: { items: [] } },
        commissions: { success: false, message: 'commission unavailable' },
        withdraws: { message: 'success', data: { items: [] } },
      }),
    ).toBe('commission unavailable');

    expect(
      getWalletRecordLoadError({
        flows: { success: true },
        commissions: { success: true },
        withdraws: { message: 'success' },
      }),
    ).toBe('');
  });

  test('uses amount discount when relation discount is not active', () => {
    expect(getEffectiveTopupDiscount(100, { 100: 0.9 }, 0)).toBe(0.9);
  });

  test('uses relation discount before amount discount when active', () => {
    expect(getEffectiveTopupDiscount(100, { 100: 0.9 }, 0.8)).toBe(0.8);
  });

  test('maps direct payment methods to friendly labels', () => {
    expect(getWalletPaymentMethodLabel('alipay_direct')).toBe('支付宝');
    expect(getWalletPaymentMethodLabel('wechat_direct')).toBe('微信');
  });

  test('hides epay Alipay and WeChat payment methods from user-facing lists', () => {
    const visible = getVisibleWalletPaymentMethods([
      { name: '支付宝', type: 'alipay' },
      { name: '微信', type: 'wxpay' },
      { name: '支付宝直连', type: 'alipay_direct' },
      { name: '微信支付直连', type: 'wechat_direct' },
      { name: 'Custom', type: 'custom' },
    ]);

    expect(visible).toEqual([
      { name: '支付宝', type: 'alipay_direct' },
      { name: '微信', type: 'wechat_direct' },
      { name: 'Custom', type: 'custom' },
    ]);
  });

  test('keeps subscription epay options limited to real epay methods', () => {
    const types = [
      'alipay',
      'wxpay',
      'alipay_direct',
      'wechat_direct',
      'stripe',
      'creem',
      'waffo',
      'waffo_pancake',
      'custom',
    ];

    expect(
      types.filter((type) => isSubscriptionEpayPaymentMethod({ type })),
    ).toEqual(['custom']);
  });

  test('routes direct Alipay separately from epay Alipay', () => {
    expect(isAlipayDirectPayment('alipay_direct')).toBe(true);
    expect(isAlipayDirectPayment('alipay')).toBe(false);
    expect(getWalletTopUpAmountPath('alipay_direct')).toBe(
      '/api/user/alipay/amount',
    );
    expect(getWalletTopUpAmountPath('alipay')).toBe('/api/user/amount');
    expect(getWalletTopUpPayPath('alipay_direct')).toBe('/api/user/alipay/pay');
    expect(getWalletTopUpPayPath('alipay')).toBe('/api/user/pay');
  });

  test('routes direct WeChat Pay separately from epay WeChat Pay', () => {
    expect(isWechatDirectPayment('wechat_direct')).toBe(true);
    expect(isWechatDirectPayment('wxpay')).toBe(false);
    expect(getWalletTopUpAmountPath('wechat_direct')).toBe(
      '/api/user/wechat/amount',
    );
    expect(getWalletTopUpAmountPath('wxpay')).toBe('/api/user/amount');
    expect(getWalletTopUpPayPath('wechat_direct')).toBe('/api/user/wechat/pay');
    expect(getWalletTopUpPayPath('wxpay')).toBe('/api/user/pay');
  });

  test('blocks classic withdrawal submission instead of calling removed legacy endpoint', async () => {
    const response = await submitWalletWithdraw({ amount: 100 });

    expect(response).toEqual({
      success: false,
      message: '当前界面暂不支持小猪银行卡提现，请切换到新版前台完成提现',
    });
  });

  test('marks classic withdrawal entry as unsupported', () => {
    expect(isClassicWalletWithdrawSupported()).toBe(false);
  });

  test('translates classic withdrawal unsupported copy in English locale', () => {
    const translations = enLocale.translation;
    const unsupportedKey =
      '当前界面暂不支持小猪银行卡提现，请切换到新版前台完成提现';

    expect(translations[unsupportedKey]).toBe(
      'Piggy bankcard withdrawals are not supported in the classic UI. Switch to the new frontend to withdraw.',
    );
    expect(translations['请使用新版后台处理']).toBe(
      'Use the new admin console',
    );
  });
});
