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

export const WALLET_RECORD_PAGE_SIZE = 10;
export const WALLET_PAYMENT_ALIPAY = 'alipay';
export const WALLET_PAYMENT_WECHAT = 'wxpay';
export const WALLET_PAYMENT_ALIPAY_DIRECT = 'alipay_direct';
export const WALLET_PAYMENT_WECHAT_DIRECT = 'wechat_direct';
export const CLASSIC_WITHDRAW_UNSUPPORTED_MESSAGE =
  '当前界面暂不支持小猪银行卡提现，请切换到新版前台完成提现';

export function isApiSuccess(response) {
  return response?.success === true || response?.message === 'success';
}

export function isClassicWalletWithdrawSupported() {
  return false;
}

export function getWalletRecordLoadError(responses) {
  const failedResponse = Object.values(responses || {}).find(
    (response) => !isApiSuccess(response),
  );
  if (!failedResponse) return '';
  return failedResponse.message || failedResponse.data || '钱包记录加载失败';
}

export function buildWalletPageQuery(page, pageSize, extra = {}) {
  const params = new URLSearchParams({
    p: String(page),
    page_size: String(pageSize),
  });
  Object.entries(extra).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value));
    }
  });
  return params.toString();
}

export function formatWalletMoney(value) {
  const amount = Number(value || 0);
  return amount.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

export function getEffectiveTopupDiscount(
  amount,
  discounts = {},
  relationTopupDiscount = 0,
) {
  if (relationTopupDiscount > 0) {
    return relationTopupDiscount;
  }
  return discounts?.[amount] || 1.0;
}

export function getWalletFlowLabelKey(value) {
  const map = {
    recharge_balance: '充值余额',
    vip_activation: 'VVIP开通',
    topup: '充值',
    commission_income: '佣金收入',
    commission_to_balance: '佣金转余额',
    withdraw_freeze: '提现冻结',
    withdraw_success: '提现成功',
    withdraw_reject: '提现驳回',
    refund_reverse: '退款冲正',
  };
  return map[value] || value || '未知';
}

export function getWalletStatusKey(value) {
  const map = {
    active: '正常',
    approved: '已审批',
    disabled: '已禁用',
    failed: '失败',
    paid: '已打款',
    pending: '待处理',
    rejected: '已驳回',
    reversed: '已冲正',
    settled: '已结算',
    success: '成功',
  };
  return map[value] || value || '未知';
}

export function isAlipayDirectPayment(type) {
  return type === WALLET_PAYMENT_ALIPAY_DIRECT;
}

export function isWechatDirectPayment(type) {
  return type === WALLET_PAYMENT_WECHAT_DIRECT;
}

export function isHiddenEpayPayment(type) {
  return type === WALLET_PAYMENT_ALIPAY || type === WALLET_PAYMENT_WECHAT;
}

export function getVisibleWalletPaymentMethods(methods = []) {
  return (methods || [])
    .filter((method) => method?.type && !isHiddenEpayPayment(method.type))
    .map((method) => {
      if (
        isAlipayDirectPayment(method.type) ||
        isWechatDirectPayment(method.type)
      ) {
        return {
          ...method,
          name: getWalletPaymentMethodLabel(method.type),
        };
      }
      return method;
    });
}

export function isSubscriptionEpayPaymentMethod(method) {
  const type = method?.type;
  return (
    !!type &&
    !isHiddenEpayPayment(type) &&
    type !== 'stripe' &&
    type !== 'creem' &&
    type !== 'waffo' &&
    type !== 'waffo_pancake' &&
    !isAlipayDirectPayment(type) &&
    !isWechatDirectPayment(type)
  );
}

export function getWalletTopUpAmountPath(paymentType) {
  if (paymentType === 'stripe') {
    return '/api/user/stripe/amount';
  }
  if (paymentType === 'waffo_pancake') {
    return '/api/user/waffo-pancake/amount';
  }
  if (isAlipayDirectPayment(paymentType)) {
    return '/api/user/alipay/amount';
  }
  if (isWechatDirectPayment(paymentType)) {
    return '/api/user/wechat/amount';
  }
  return '/api/user/amount';
}

export function getWalletTopUpPayPath(paymentType) {
  if (paymentType === 'stripe') {
    return '/api/user/stripe/pay';
  }
  if (isAlipayDirectPayment(paymentType)) {
    return '/api/user/alipay/pay';
  }
  if (isWechatDirectPayment(paymentType)) {
    return '/api/user/wechat/pay';
  }
  return '/api/user/pay';
}

export function getWalletPaymentMethodLabel(value) {
  const map = {
    stripe: 'Stripe',
    creem: 'Creem',
    waffo: 'Waffo',
    alipay_direct: '支付宝',
    wechat_direct: '微信',
    alipay: '支付宝',
    wxpay: '微信',
  };
  return map[value] || value || '未知';
}

export function submitPaymentForm(url, params) {
  const form = document.createElement('form');
  form.action = url;
  form.method = 'POST';

  const isSafari =
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1;
  if (!isSafari) {
    form.target = '_blank';
  }

  Object.entries(params || {}).forEach(([key, value]) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = key;
    input.value = String(value);
    form.appendChild(input);
  });

  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
}

async function getAPI() {
  const { API } = await import('./api.js');
  return API;
}

export async function getWalletAccount() {
  const API = await getAPI();
  const res = await API.get('/api/wallet/account');
  return res.data;
}

export async function getWalletFlows(page, pageSize) {
  const API = await getAPI();
  const res = await API.get(
    `/api/wallet/flows?${buildWalletPageQuery(page, pageSize)}`,
  );
  return res.data;
}

export async function getWalletCommissions(page, pageSize) {
  const API = await getAPI();
  const res = await API.get(
    `/api/wallet/commissions?${buildWalletPageQuery(page, pageSize)}`,
  );
  return res.data;
}

export async function getWalletWithdraws(page, pageSize) {
  const API = await getAPI();
  const res = await API.get(
    `/api/wallet/withdraws?${buildWalletPageQuery(page, pageSize)}`,
  );
  return res.data;
}

export async function transferWalletCommission(amount) {
  const API = await getAPI();
  const res = await API.post('/api/wallet/commission/transfer', { amount });
  return res.data;
}

export async function submitWalletWithdraw(request) {
  void request;
  return {
    success: false,
    message: CLASSIC_WITHDRAW_UNSUPPORTED_MESSAGE,
  };
}

export async function getVipActivationInfo() {
  const API = await getAPI();
  const res = await API.get('/api/vip/info');
  return res.data;
}

export async function requestVipActivationPayment(method, vipInfo) {
  const API = await getAPI();
  if (method?.type === 'stripe') {
    const res = await API.post('/api/vip/stripe/pay', {});
    return res.data;
  }
  if (method?.type === 'creem') {
    const productId = vipInfo?.creem_products?.[0]?.productId;
    const res = await API.post('/api/vip/creem/pay', { product_id: productId });
    return res.data;
  }
  if (method?.type === 'waffo') {
    const res = await API.post('/api/vip/waffo/pay', {});
    return res.data;
  }
  if (isAlipayDirectPayment(method?.type)) {
    const res = await API.post('/api/vip/alipay/pay', {});
    return res.data;
  }
  if (isWechatDirectPayment(method?.type)) {
    const res = await API.post('/api/vip/wechat/pay', {
      payment_method: WALLET_PAYMENT_WECHAT_DIRECT,
    });
    return res.data;
  }
  const res = await API.post('/api/vip/epay/pay', {
    payment_method: method?.type,
  });
  return {
    ...res.data,
    url: res.data?.url || res?.url,
  };
}

export function openVipActivationPayment(response) {
  if (response?.data?.mock) {
    return true;
  }
  if (response?.data?.pay_link) {
    window.open(response.data.pay_link, '_blank');
    return true;
  }
  if (response?.data?.checkout_url) {
    window.open(response.data.checkout_url, '_blank');
    return true;
  }
  if (response?.data?.payment_url) {
    window.open(response.data.payment_url, '_blank');
    return true;
  }
  if (response?.data?.url && response?.data?.params) {
    submitPaymentForm(response.data.url, response.data.params);
    return true;
  }
  if (response?.url && response?.data) {
    submitPaymentForm(response.url, response.data);
    return true;
  }
  return false;
}
