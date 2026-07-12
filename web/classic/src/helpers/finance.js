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

export const ADMIN_FINANCE_PAGE_SIZE = 20;
export const PIGGY_WITHDRAW_PROVIDER = 'piggy_labor_v3';

export function buildFinanceQuery(params) {
  const query = new URLSearchParams();
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value));
    }
  });
  const text = query.toString();
  return text ? `?${text}` : '';
}

export function formatFinanceTimestamp(timestamp) {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
}

export function formatFinanceMoney(value) {
  const amount = Number(value || 0);
  return amount.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

export function getFinanceStatusKey(status) {
  const map = {
    active: '正常',
    approved: '已审批',
    disabled: '已禁用',
    failed: '失败',
    ignored: '已忽略',
    paid: '已打款',
    pending: '待处理',
    rejected: '已驳回',
    reversed: '已冲正',
    settled: '已结算',
    success: '成功',
  };
  return map[status] || status || '未知';
}

export function getPaymentDiffKey(diffType) {
  const map = {
    amount_mismatch: '金额不一致',
    duplicate_callback: '重复回调',
    local_missing: '本地缺失',
    provider_missing: '渠道缺失',
    status_mismatch: '状态不一致',
  };
  return map[diffType] || diffType || '未知';
}

export function parseProviderOrders(text) {
  const orders = JSON.parse(text || '[]');
  if (!Array.isArray(orders)) {
    throw new Error('orders must be an array');
  }
  return orders;
}

export function getFinancePaymentMethodLabel(value) {
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

export function getClassicWithdrawActions(record) {
  if (record?.provider === PIGGY_WITHDRAW_PROVIDER) {
    return [];
  }
  const status = record?.status;
  const actions = [];
  if (status === 'pending') {
    actions.push('approve');
  }
  if (['approved', 'failed'].includes(status)) {
    actions.push('pay');
  }
  if (status === 'approved') {
    actions.push('fail');
  }
  if (['pending', 'approved', 'failed'].includes(status)) {
    actions.push('reject');
  }
  return actions;
}

export function createFinanceFilterChangeHandler(setPage, setValue) {
  return (value) => {
    setPage(1);
    setValue(value);
  };
}

async function getAPI() {
  const { API } = await import('./api.js');
  return API;
}

async function apiGet(path, params) {
  const API = await getAPI();
  const res = await API.get(`${path}${buildFinanceQuery(params)}`);
  return res.data;
}

async function apiPost(path, payload = {}) {
  const API = await getAPI();
  const res = await API.post(path, payload);
  return res.data;
}

export const financeApi = {
  getVipActivationRecords: (page, pageSize) =>
    apiGet('/api/vip/admin/records', { p: page, page_size: pageSize }),
  disableVipActivation: (userId, reason) =>
    apiPost(`/api/vip/admin/users/${userId}/disable`, { reason }),
  getUserRelations: ({ page, pageSize, parentUserId, childUserId, status }) =>
    apiGet('/api/vip/admin/relations', {
      p: page,
      page_size: pageSize,
      parent_user_id: parentUserId,
      child_user_id: childUserId,
      status,
    }),
  createUserRelation: (request) => apiPost('/api/vip/admin/relations', request),
  disableUserRelation: (id, reason) =>
    apiPost(`/api/vip/admin/relations/${id}/disable`, { reason }),
  getCommissions: ({ page, pageSize, userId, status }) =>
    apiGet('/api/wallet/admin/commissions', {
      p: page,
      page_size: pageSize,
      user_id: userId,
      status,
    }),
  getWithdraws: ({ page, pageSize, userId, status }) =>
    apiGet('/api/wallet/admin/withdraws', {
      p: page,
      page_size: pageSize,
      user_id: userId,
      status,
    }),
  approveWithdraw: (id) => apiPost(`/api/wallet/admin/withdraws/${id}/approve`),
  rejectWithdraw: (id, reason) =>
    apiPost(`/api/wallet/admin/withdraws/${id}/reject`, { reason }),
  payWithdraw: (id, paymentVoucher) =>
    apiPost(`/api/wallet/admin/withdraws/${id}/pay`, {
      payment_voucher: paymentVoucher,
    }),
  failWithdraw: (id, reason) =>
    apiPost(`/api/wallet/admin/withdraws/${id}/fail`, { reason }),
  getCallbackLogs: ({ page, pageSize, provider, tradeNo, processStatus }) =>
    apiGet('/api/payment/admin/callback-logs', {
      p: page,
      page_size: pageSize,
      provider,
      trade_no: tradeNo,
      process_status: processStatus,
    }),
  getReconciliationTasks: ({ page, pageSize, provider, status }) =>
    apiGet('/api/payment/admin/reconciliation/tasks', {
      p: page,
      page_size: pageSize,
      provider,
      status,
    }),
  createReconciliationTask: (request) =>
    apiPost('/api/payment/admin/reconciliation/tasks', request),
  reverseTopupOrder: (tradeNo, provider, reason) =>
    apiPost(
      `/api/payment/admin/topups/${encodeURIComponent(tradeNo)}/reverse`,
      {
        provider,
        reason,
      },
    ),
  reverseVipActivationOrder: (tradeNo, provider, reason) =>
    apiPost(
      `/api/payment/admin/vip-activations/${encodeURIComponent(tradeNo)}/reverse`,
      {
        provider,
        reason,
      },
    ),
};
