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

import React, {
  useCallback,
  useEffect,
  useState,
  useContext,
  useRef,
} from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  renderQuota,
  renderQuotaWithAmount,
  copy,
  getWalletAccount,
  getEffectiveTopupDiscount,
  getWalletTopUpAmountPath,
  getWalletTopUpPayPath,
  getVisibleWalletPaymentMethods,
  getVipActivationInfo,
  isClassicWalletWithdrawSupported,
  isApiSuccess,
  isAlipayDirectPayment,
  isWechatDirectPayment,
  openVipActivationPayment,
  requestVipActivationPayment,
  submitPaymentForm,
  submitWalletWithdraw,
  transferWalletCommission,
} from '../../helpers';
import { Modal, Toast } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';

import RechargeCard from './RechargeCard';
import InvitationCard from './InvitationCard';
import TransferModal from './modals/TransferModal';
import PaymentConfirmModal from './modals/PaymentConfirmModal';
import TopupHistoryModal from './modals/TopupHistoryModal';
import WithdrawModal from './modals/WithdrawModal';
import WechatPayQrModal from './modals/WechatPayQrModal';
import VipActivationCard from './VipActivationCard';
import WalletRecordsCard from './WalletRecordsCard';
import WalletStatsCard from './WalletStatsCard';

const TopUp = () => {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);

  const [redemptionCode, setRedemptionCode] = useState('');
  const [amount, setAmount] = useState(0.0);
  const [minTopUp, setMinTopUp] = useState(statusState?.status?.min_topup || 1);
  const [topUpCount, setTopUpCount] = useState(
    statusState?.status?.min_topup || 1,
  );
  const [topUpLink, setTopUpLink] = useState('');
  const [enableOnlineTopUp, setEnableOnlineTopUp] = useState(
    statusState?.status?.enable_online_topup || false,
  );
  const [priceRatio, setPriceRatio] = useState(statusState?.status?.price || 1);

  const [enableStripeTopUp, setEnableStripeTopUp] = useState(
    statusState?.status?.enable_stripe_topup || false,
  );
  const [statusLoading, setStatusLoading] = useState(true);

  // Creem 相关状态
  const [creemProducts, setCreemProducts] = useState([]);
  const [enableCreemTopUp, setEnableCreemTopUp] = useState(false);
  const [creemOpen, setCreemOpen] = useState(false);
  const [selectedCreemProduct, setSelectedCreemProduct] = useState(null);

  // Waffo 相关状态
  const [enableWaffoTopUp, setEnableWaffoTopUp] = useState(false);
  const [waffoPayMethods, setWaffoPayMethods] = useState([]);
  const [waffoMinTopUp, setWaffoMinTopUp] = useState(1);
  const [enableWaffoPancakeTopUp, setEnableWaffoPancakeTopUp] = useState(false);
  const [waffoPancakeMinTopUp, setWaffoPancakeMinTopUp] = useState(1);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [open, setOpen] = useState(false);
  const [payWay, setPayWay] = useState('');
  const [amountLoading, setAmountLoading] = useState(false);
  const [paymentLoading, setPaymentLoading] = useState(false);
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [payMethods, setPayMethods] = useState([]);

  const affFetchedRef = useRef(false);

  // 邀请相关状态
  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(0);
  const [openWithdraw, setOpenWithdraw] = useState(false);
  const [walletAccount, setWalletAccount] = useState(null);
  const [walletLoading, setWalletLoading] = useState(true);
  const [walletSubmitting, setWalletSubmitting] = useState(false);
  const [minWithdrawAmount, setMinWithdrawAmount] = useState(0);
  const [walletRecordRefreshKey, setWalletRecordRefreshKey] = useState(0);
  const [vipInfo, setVipInfo] = useState(null);
  const [vipLoading, setVipLoading] = useState(true);
  const [vipProcessing, setVipProcessing] = useState(null);
  const [wechatQrVisible, setWechatQrVisible] = useState(false);
  const [wechatQrPayment, setWechatQrPayment] = useState(null);

  // 账单Modal状态
  const [openHistory, setOpenHistory] = useState(false);

  // 订阅相关
  const [subscriptionPlans, setSubscriptionPlans] = useState([]);
  const [subscriptionLoading, setSubscriptionLoading] = useState(true);
  const [billingPreference, setBillingPreference] =
    useState('subscription_first');
  const [activeSubscriptions, setActiveSubscriptions] = useState([]);
  const [allSubscriptions, setAllSubscriptions] = useState([]);

  // 预设充值额度选项
  const [presetAmounts, setPresetAmounts] = useState([]);
  const [selectedPreset, setSelectedPreset] = useState(null);

  // 充值配置信息
  const [topupInfo, setTopupInfo] = useState({
    amount_options: [],
    discount: {},
    enable_redemption: true,
    payment_compliance_confirmed: true,
  });

  const confirmPayMethods = [
    ...payMethods,
    ...waffoPayMethods.map((method, index) => ({
      ...method,
      type: `waffo:${index}`,
      min_topup: waffoMinTopUp,
      color: method.color || 'rgba(var(--semi-primary-5), 1)',
    })),
  ];
  const subscriptionPayMethods = payMethods;
  const hasAnyRechargeMethod = confirmPayMethods.length > 0 || enableCreemTopUp;

  const getPayMethodConfig = (payment) =>
    confirmPayMethods.find((method) => method.type === payment);

  const getPaymentMinTopUp = (payment) => {
    const configuredMinTopUp = Number(getPayMethodConfig(payment)?.min_topup);
    return Number.isFinite(configuredMinTopUp) && configuredMinTopUp > 0
      ? configuredMinTopUp
      : minTopUp;
  };

  const requestAmountByPayment = async (payment, value) => {
    if (payment === 'stripe') {
      return getStripeAmount(value);
    }
    if (isAlipayDirectPayment(payment)) {
      return getAlipayAmount(value);
    }
    if (isWechatDirectPayment(payment)) {
      return getWechatPayAmount(value);
    }
    if (payment === 'waffo_pancake') {
      return getWaffoPancakeAmount(value);
    }
    if (typeof payment === 'string' && payment.startsWith('waffo:')) {
      return getWaffoAmount(value);
    }
    return getAmount(value);
  };

  const topUp = async () => {
    if (redemptionCode === '') {
      showInfo(t('请输入兑换码！'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post('/api/user/topup', {
        key: redemptionCode,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('兑换成功！'));
        Modal.success({
          title: t('兑换成功！'),
          content: t('成功兑换额度：') + renderQuota(data),
          centered: true,
        });
        if (userState.user) {
          const updatedUser = {
            ...userState.user,
            quota: userState.user.quota + data,
          };
          userDispatch({ type: 'login', payload: updatedUser });
        }
        setRedemptionCode('');
        loadWalletAccount().then();
        setWalletRecordRefreshKey((value) => value + 1);
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('请求失败'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const openTopUpLink = () => {
    if (!topUpLink) {
      showError(t('超级管理员未设置充值链接！'));
      return;
    }
    window.open(topUpLink, '_blank');
  };

  const preTopUp = async (payment) => {
    if (payment === 'stripe') {
      if (!enableStripeTopUp) {
        showError(t('管理员未开启Stripe充值！'));
        return;
      }
    } else if (payment === 'waffo_pancake') {
      if (!enableWaffoPancakeTopUp) {
        showError(t('管理员未开启 Waffo Pancake 充值！'));
        return;
      }
    } else if (payment.startsWith('waffo:')) {
      if (!enableWaffoTopUp) {
        showError(t('管理员未开启 Waffo 充值！'));
        return;
      }
    } else if (isAlipayDirectPayment(payment)) {
      if (!getPayMethodConfig(payment)) {
        showError(t('管理员未开启支付宝直连充值！'));
        return;
      }
    } else if (isWechatDirectPayment(payment)) {
      if (!getPayMethodConfig(payment)) {
        showError(t('管理员未开启微信支付直连充值！'));
        return;
      }
    } else {
      if (!enableOnlineTopUp) {
        showError(t('管理员未开启在线充值！'));
        return;
      }
    }

    setPayWay(payment);
    setPaymentLoading(true);
    try {
      const selectedMinTopUp = getPaymentMinTopUp(payment);
      await requestAmountByPayment(payment);

      if (topUpCount < selectedMinTopUp) {
        showError(t('充值数量不能小于') + selectedMinTopUp);
        return;
      }
      setOpen(true);
    } catch (error) {
      showError(t('获取金额失败'));
    } finally {
      setPaymentLoading(false);
    }
  };

  const onlineTopUp = async () => {
    if (payWay === 'waffo_pancake') {
      setConfirmLoading(true);
      try {
        await waffoPancakeTopUp();
      } finally {
        setOpen(false);
        setConfirmLoading(false);
      }
      return;
    }

    if (payWay.startsWith('waffo:')) {
      const payMethodIndex = Number(payWay.split(':')[1]);
      setConfirmLoading(true);
      try {
        await waffoTopUp(Number.isFinite(payMethodIndex) ? payMethodIndex : 0);
      } finally {
        setOpen(false);
        setConfirmLoading(false);
      }
      return;
    }

    if (payWay === 'stripe') {
      // Stripe 支付处理
      if (amount === 0) {
        await getStripeAmount();
      }
    } else if (isAlipayDirectPayment(payWay)) {
      if (amount === 0) {
        await getAlipayAmount();
      }
    } else if (isWechatDirectPayment(payWay)) {
      if (amount === 0) {
        await getWechatPayAmount();
      }
    } else {
      // 普通支付处理
      if (amount === 0) {
        await getAmount();
      }
    }

    const selectedMinTopUp = getPaymentMinTopUp(payWay);
    if (topUpCount < selectedMinTopUp) {
      showError('充值数量不能小于' + selectedMinTopUp);
      return;
    }
    setConfirmLoading(true);
    try {
      let res;
      const payPath = getWalletTopUpPayPath(payWay);
      res = await API.post(payPath, {
        amount: parseInt(topUpCount),
        payment_method: payWay === 'stripe' ? 'stripe' : payWay,
      });

      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          if (payWay === 'stripe') {
            // Stripe 支付回调处理
            window.open(data.pay_link, '_blank');
          } else if (isAlipayDirectPayment(payWay)) {
            submitPaymentForm(data.url, data.params);
          } else if (isWechatDirectPayment(payWay)) {
            setWechatQrPayment({ ...data, purpose: 'topup' });
            setWechatQrVisible(true);
            showSuccess(t('微信支付订单已创建'));
          } else {
            // 普通支付表单提交
            let params = data;
            let url = res.data.url;
            let form = document.createElement('form');
            form.action = url;
            form.method = 'POST';
            let isSafari =
              navigator.userAgent.indexOf('Safari') > -1 &&
              navigator.userAgent.indexOf('Chrome') < 1;
            if (!isSafari) {
              form.target = '_blank';
            }
            for (let key in params) {
              let input = document.createElement('input');
              input.type = 'hidden';
              input.name = key;
              input.value = params[key];
              form.appendChild(input);
            }
            document.body.appendChild(form);
            form.submit();
            document.body.removeChild(form);
          }
        } else {
          const errorMsg =
            typeof data === 'string' ? data : message || t('支付失败');
          showError(errorMsg);
        }
      } else {
        showError(res);
      }
    } catch (err) {
      showError(t('支付请求失败'));
    } finally {
      setOpen(false);
      setConfirmLoading(false);
    }
  };

  const creemPreTopUp = async (product) => {
    if (!enableCreemTopUp) {
      showError(t('管理员未开启 Creem 充值！'));
      return;
    }
    setSelectedCreemProduct(product);
    setCreemOpen(true);
  };

  const onlineCreemTopUp = async () => {
    if (!selectedCreemProduct) {
      showError(t('请选择产品'));
      return;
    }
    // Validate product has required fields
    if (!selectedCreemProduct.productId) {
      showError(t('产品配置错误，请联系管理员'));
      return;
    }
    setConfirmLoading(true);
    try {
      const res = await API.post('/api/user/creem/pay', {
        product_id: selectedCreemProduct.productId,
        payment_method: 'creem',
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          processCreemCallback(data);
        } else {
          const errorMsg =
            typeof data === 'string' ? data : message || t('支付失败');
          showError(errorMsg);
        }
      } else {
        showError(res);
      }
    } catch (err) {
      showError(t('支付请求失败'));
    } finally {
      setCreemOpen(false);
      setConfirmLoading(false);
    }
  };

  const waffoTopUp = async (payMethodIndex) => {
    try {
      if (topUpCount < waffoMinTopUp) {
        showError(t('充值数量不能小于') + waffoMinTopUp);
        return;
      }
      setPaymentLoading(true);
      const requestBody = {
        amount: parseInt(topUpCount),
      };
      if (payMethodIndex != null) {
        requestBody.pay_method_index = payMethodIndex;
      }
      const res = await API.post('/api/user/waffo/pay', requestBody);
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success' && data?.payment_url) {
          window.open(data.payment_url, '_blank');
        } else {
          showError(data || t('支付请求失败'));
        }
      } else {
        showError(res);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaymentLoading(false);
    }
  };

  const getWaffoAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/waffo/amount', {
        amount: parseInt(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      // amount fetch failed silently
    } finally {
      setAmountLoading(false);
    }
  };

  const waffoPancakeTopUp = async () => {
    const minTopUpValue = Number(waffoPancakeMinTopUp || 1);
    if (topUpCount < minTopUpValue) {
      showError(t('充值数量不能小于') + minTopUpValue);
      return;
    }

    setPaymentLoading(true);
    try {
      const res = await API.post('/api/user/waffo-pancake/pay', {
        amount: parseInt(topUpCount),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          const checkoutUrl = data?.checkout_url || '';
          if (checkoutUrl) {
            window.open(checkoutUrl, '_blank');
          } else {
            showError(t('支付请求失败'));
          }
        } else {
          const errorMsg =
            typeof data === 'string' ? data : message || t('支付请求失败');
          showError(errorMsg);
        }
      } else {
        showError(res);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaymentLoading(false);
    }
  };

  const getWaffoPancakeAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/waffo-pancake/amount', {
        amount: parseInt(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      // amount fetch failed silently
    } finally {
      setAmountLoading(false);
    }
  };

  const processCreemCallback = (data) => {
    // 与 Stripe 保持一致的实现方式
    window.open(data.checkout_url, '_blank');
  };

  const loadWalletAccount = useCallback(async () => {
    setWalletLoading(true);
    try {
      const res = await getWalletAccount();
      if (isApiSuccess(res) && res.data) {
        setWalletAccount(res.data.account || null);
        setMinWithdrawAmount(res.data.commission_min_withdraw_amount || 0);
        setTransferAmount(
          Math.max(1, Math.min(res.data.account?.commission_amount || 0, 1)),
        );
      }
    } catch (error) {
      showError(t('钱包账户加载失败'));
    } finally {
      setWalletLoading(false);
    }
  }, [t]);

  const loadVipInfo = useCallback(async () => {
    setVipLoading(true);
    try {
      const res = await getVipActivationInfo();
      if (isApiSuccess(res)) {
        setVipInfo(
          res.data
            ? {
                ...res.data,
                payment_methods: getVisibleWalletPaymentMethods(
                  res.data.payment_methods || [],
                ),
              }
            : null,
        );
      }
    } catch (error) {
      showError(t('VVIP信息加载失败'));
    } finally {
      setVipLoading(false);
    }
  }, [t]);

  const refreshWalletData = async () => {
    await Promise.all([getUserQuota(), loadWalletAccount()]);
    setWalletRecordRefreshKey((value) => value + 1);
  };

  const handleVipActivationPay = async (method) => {
    if (!method?.type) return;
    if (vipInfo?.is_vvip) {
      showInfo(t('您已开通VVIP'));
      return;
    }
    setVipProcessing(method.type);
    try {
      const res = await requestVipActivationPayment(method, vipInfo);
      if (isApiSuccess(res) && openVipActivationPayment(res)) {
        showSuccess(t('正在跳转支付页面'));
      } else if (isApiSuccess(res) && res?.data?.code_url) {
        setWechatQrPayment({ ...res.data, purpose: 'vvip_activation' });
        setWechatQrVisible(true);
        showSuccess(t('微信支付订单已创建'));
      } else {
        showError(res?.message || res?.data || t('支付请求失败'));
      }
    } catch (error) {
      showError(t('支付请求失败'));
    } finally {
      setVipProcessing(null);
    }
  };

  const getUserQuota = async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  const getSubscriptionPlans = async () => {
    setSubscriptionLoading(true);
    try {
      const res = await API.get('/api/subscription/plans');
      if (res.data?.success) {
        setSubscriptionPlans(res.data.data || []);
      }
    } catch (e) {
      setSubscriptionPlans([]);
    } finally {
      setSubscriptionLoading(false);
    }
  };

  const getSubscriptionSelf = async () => {
    try {
      const res = await API.get('/api/subscription/self');
      if (res.data?.success) {
        setBillingPreference(
          res.data.data?.billing_preference || 'subscription_first',
        );
        // Active subscriptions
        const activeSubs = res.data.data?.subscriptions || [];
        setActiveSubscriptions(activeSubs);
        // All subscriptions (including expired)
        const allSubs = res.data.data?.all_subscriptions || [];
        setAllSubscriptions(allSubs);
      }
    } catch (e) {
      // ignore
    }
  };

  const updateBillingPreference = async (pref) => {
    const previousPref = billingPreference;
    setBillingPreference(pref);
    try {
      const res = await API.put('/api/subscription/self/preference', {
        billing_preference: pref,
      });
      if (res.data?.success) {
        showSuccess(t('更新成功'));
        const normalizedPref =
          res.data?.data?.billing_preference || pref || previousPref;
        setBillingPreference(normalizedPref);
      } else {
        showError(res.data?.message || t('更新失败'));
        setBillingPreference(previousPref);
      }
    } catch (e) {
      showError(t('请求失败'));
      setBillingPreference(previousPref);
    }
  };

  // 获取充值配置信息
  const getTopupInfo = async () => {
    try {
      const res = await API.get('/api/user/topup/info');
      const { message, data, success } = res.data;
      if (success) {
        setTopupInfo({
          amount_options: data.amount_options || [],
          discount: data.discount || {},
          relation_topup_discount: data.relation_topup_discount || 0,
        });

        // 处理支付方式
        let payMethods = data.pay_methods || [];
        try {
          if (typeof payMethods === 'string') {
            payMethods = JSON.parse(payMethods);
          }
          if (payMethods && payMethods.length > 0) {
            // 检查name和type是否为空
            payMethods = payMethods.filter((method) => {
              return method.name && method.type;
            });
            payMethods = getVisibleWalletPaymentMethods(payMethods);
            // 如果没有color，则设置默认颜色
            payMethods = payMethods.map((method) => {
              // 规范化最小充值数
              const normalizedMinTopup = Number(method.min_topup);
              method.min_topup = Number.isFinite(normalizedMinTopup)
                ? normalizedMinTopup
                : 0;

              // Stripe 的最小充值从后端字段回填
              if (
                method.type === 'stripe' &&
                (!method.min_topup || method.min_topup <= 0)
              ) {
                const stripeMin = Number(data.stripe_min_topup);
                if (Number.isFinite(stripeMin)) {
                  method.min_topup = stripeMin;
                }
              }

              if (!method.color) {
                if (
                  method.type === 'alipay' ||
                  isAlipayDirectPayment(method.type)
                ) {
                  method.color = 'rgba(var(--semi-blue-5), 1)';
                } else if (
                  method.type === 'wxpay' ||
                  isWechatDirectPayment(method.type)
                ) {
                  method.color = 'rgba(var(--semi-green-5), 1)';
                } else if (method.type === 'stripe') {
                  method.color = 'rgba(var(--semi-purple-5), 1)';
                } else {
                  method.color = 'rgba(var(--semi-primary-5), 1)';
                }
              }
              return method;
            });
          } else {
            payMethods = [];
          }

          // 如果启用了 Stripe 支付，添加到支付方法列表
          // 这个逻辑现在由后端处理，如果 Stripe 启用，后端会在 pay_methods 中包含它

          setPayMethods(payMethods);
          const enableStripeTopUp = data.enable_stripe_topup || false;
          const enableOnlineTopUp = data.enable_online_topup || false;
          const enableCreemTopUp = data.enable_creem_topup || false;
          const enableWaffoTopUp = data.enable_waffo_topup || false;
          const enableWaffoPancakeTopUp =
            data.enable_waffo_pancake_topup || false;
          const standardMinTopUps = [
            ...(enableOnlineTopUp ? [Number(data.min_topup)] : []),
            ...payMethods
              .map((method) => Number(method.min_topup))
              .filter((value) => Number.isFinite(value) && value > 0),
          ].filter((value) => Number.isFinite(value) && value > 0);
          const minTopUpValue =
            standardMinTopUps.length > 0
              ? Math.min(...standardMinTopUps)
              : enableStripeTopUp
                ? data.stripe_min_topup
                : enableWaffoTopUp
                  ? data.waffo_min_topup
                  : enableWaffoPancakeTopUp
                    ? data.waffo_pancake_min_topup
                    : 1;
          setEnableOnlineTopUp(enableOnlineTopUp);
          setEnableStripeTopUp(enableStripeTopUp);
          setEnableCreemTopUp(enableCreemTopUp);
          setEnableWaffoTopUp(enableWaffoTopUp);
          setWaffoPayMethods(data.waffo_pay_methods || []);
          setWaffoMinTopUp(data.waffo_min_topup || 1);
          setEnableWaffoPancakeTopUp(enableWaffoPancakeTopUp);
          setWaffoPancakeMinTopUp(data.waffo_pancake_min_topup || 1);
          setMinTopUp(minTopUpValue);
          setTopUpCount(minTopUpValue);
          setTopUpLink(data.topup_link || '');
          setTopupInfo((prev) => ({
            ...prev,
            enable_redemption: data.enable_redemption !== false,
            relation_topup_discount: data.relation_topup_discount || 0,
            payment_compliance_confirmed:
              data.payment_compliance_confirmed !== false,
            payment_compliance_terms_version:
              data.payment_compliance_terms_version || '',
          }));

          // 设置 Creem 产品
          try {
            const products = JSON.parse(data.creem_products || '[]');
            setCreemProducts(products);
          } catch (e) {
            setCreemProducts([]);
          }

          // 如果没有自定义充值数量选项，根据最小充值金额生成预设充值额度选项
          if (topupInfo.amount_options.length === 0) {
            setPresetAmounts(generatePresetAmounts(minTopUpValue));
          }

          // 初始化显示实付金额
          getAmount(minTopUpValue);
        } catch (e) {
          setPayMethods([]);
        }

        // 如果有自定义充值数量选项，使用它们替换默认的预设选项
        if (data.amount_options && data.amount_options.length > 0) {
          const customPresets = data.amount_options.map((amount) => ({
            value: amount,
            discount: getEffectiveTopupDiscount(
              amount,
              data.discount || {},
              data.relation_topup_discount || 0,
            ),
          }));
          setPresetAmounts(customPresets);
        }
      } else {
        showError(data || t('获取充值配置失败'));
      }
    } catch (error) {
      showError(t('获取充值配置异常'));
    }
  };

  // 获取邀请链接
  const getAffLink = async () => {
    const res = await API.get('/api/user/aff');
    const { success, message, data } = res.data;
    if (success) {
      let link = `${window.location.origin}/register?aff=${data}`;
      setAffLink(link);
    } else {
      showError(message);
    }
  };

  // 划转邀请额度
  const transfer = async () => {
    if (transferAmount < 1) {
      showError(t('划转金额最低为') + ' ' + renderQuotaWithAmount(1));
      return;
    }
    if (transferAmount > (walletAccount?.commission_amount || 0)) {
      showError(t('划转金额不能超过可提现佣金'));
      return;
    }
    setWalletSubmitting(true);
    try {
      const res = await transferWalletCommission(transferAmount);
      if (isApiSuccess(res)) {
        showSuccess(t('划转成功'));
        setOpenTransfer(false);
        await refreshWalletData();
      } else {
        showError(res?.message || t('划转失败'));
      }
    } catch (error) {
      showError(t('划转失败'));
    } finally {
      setWalletSubmitting(false);
    }
  };

  const submitWithdraw = async (request) => {
    setWalletSubmitting(true);
    try {
      const res = await submitWalletWithdraw(request);
      if (isApiSuccess(res)) {
        showSuccess(t('提现申请已提交'));
        setOpenWithdraw(false);
        await refreshWalletData();
      } else {
        showError(res?.message || t('提现申请失败'));
      }
    } catch (error) {
      showError(t('提现申请失败'));
    } finally {
      setWalletSubmitting(false);
    }
  };

  // 复制邀请链接
  const handleAffLinkClick = async () => {
    await copy(affLink);
    showSuccess(t('邀请链接已复制到剪切板'));
  };

  // URL 参数自动打开账单弹窗（支付回跳时触发）
  useEffect(() => {
    if (searchParams.get('show_history') === 'true') {
      setOpenHistory(true);
      searchParams.delete('show_history');
      setSearchParams(searchParams, { replace: true });
    }
  }, []);

  useEffect(() => {
    // 始终获取最新用户数据，确保余额等统计信息准确
    getUserQuota().then();
    loadWalletAccount().then();
    loadVipInfo().then();
  }, []);

  useEffect(() => {
    if (affFetchedRef.current) return;
    affFetchedRef.current = true;
    getAffLink().then();
  }, []);

  // 在 statusState 可用时获取充值信息
  useEffect(() => {
    getTopupInfo().then();
    getSubscriptionPlans().then();
    getSubscriptionSelf().then();
  }, []);

  useEffect(() => {
    if (statusState?.status) {
      // const minTopUpValue = statusState.status.min_topup || 1;
      // setMinTopUp(minTopUpValue);
      // setTopUpCount(minTopUpValue);
      setPriceRatio(statusState.status.price || 1);

      setStatusLoading(false);
    }
  }, [statusState?.status]);

  const renderAmount = () => {
    return amount + ' ' + t('元');
  };

  const getAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/amount', {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      // amount fetch failed silently
    }
    setAmountLoading(false);
  };

  const getStripeAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/stripe/amount', {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      // amount fetch failed silently
    } finally {
      setAmountLoading(false);
    }
  };

  const getAlipayAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post(getWalletTopUpAmountPath('alipay_direct'), {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      // amount fetch failed silently
    } finally {
      setAmountLoading(false);
    }
  };

  const getWechatPayAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post(getWalletTopUpAmountPath('wechat_direct'), {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      // amount fetch failed silently
    } finally {
      setAmountLoading(false);
    }
  };

  const handleWechatQrRefresh = async () => {
    await Promise.all([refreshWalletData(), loadVipInfo()]);
  };

  const handleWechatQrOpenHistory = () => {
    setWechatQrVisible(false);
    setOpenHistory(true);
  };

  const handleCancel = () => {
    setOpen(false);
  };

  const handleTransferCancel = () => {
    setOpenTransfer(false);
  };

  const handleOpenHistory = () => {
    setOpenHistory(true);
  };

  const handleHistoryCancel = () => {
    setOpenHistory(false);
  };

  const handleCreemCancel = () => {
    setCreemOpen(false);
    setSelectedCreemProduct(null);
  };

  // 选择预设充值额度
  const selectPresetAmount = (preset) => {
    setTopUpCount(preset.value);
    setSelectedPreset(preset.value);

    // 计算实际支付金额，考虑折扣
    const discount = getEffectiveTopupDiscount(
      preset.value,
      topupInfo.discount,
      topupInfo.relation_topup_discount,
    );
    const discountedAmount = preset.value * priceRatio * discount;
    setAmount(discountedAmount);
  };

  // 格式化大数字显示
  const formatLargeNumber = (num) => {
    return num.toString();
  };

  // 根据最小充值金额生成预设充值额度选项
  const generatePresetAmounts = (minAmount) => {
    const multipliers = [1, 5, 10, 30, 50, 100, 300, 500];
    return multipliers.map((multiplier) => ({
      value: minAmount * multiplier,
    }));
  };

  return (
    <div className='w-full max-w-7xl mx-auto relative min-h-screen lg:min-h-0 mt-[60px] px-2'>
      {/* 划转模态框 */}
      <TransferModal
        t={t}
        openTransfer={openTransfer}
        transfer={transfer}
        handleTransferCancel={handleTransferCancel}
        availableAmount={walletAccount?.commission_amount || 0}
        minTransferAmount={1}
        renderAmount={renderQuotaWithAmount}
        transferAmount={transferAmount}
        setTransferAmount={setTransferAmount}
      />

      <WithdrawModal
        t={t}
        visible={openWithdraw}
        onCancel={() => setOpenWithdraw(false)}
        onSubmit={submitWithdraw}
        availableAmount={walletAccount?.commission_amount || 0}
        minAmount={minWithdrawAmount}
        loading={walletSubmitting}
        renderAmount={renderQuotaWithAmount}
      />

      {/* 充值确认模态框 */}
      <PaymentConfirmModal
        t={t}
        open={open}
        onlineTopUp={onlineTopUp}
        handleCancel={handleCancel}
        confirmLoading={confirmLoading}
        topUpCount={topUpCount}
        renderQuotaWithAmount={renderQuotaWithAmount}
        amountLoading={amountLoading}
        renderAmount={renderAmount}
        payWay={payWay}
        payMethods={confirmPayMethods}
        amountNumber={amount}
        discountRate={getEffectiveTopupDiscount(
          topUpCount,
          topupInfo?.discount,
          topupInfo?.relation_topup_discount,
        )}
      />

      {/* 充值账单模态框 */}
      <TopupHistoryModal
        visible={openHistory}
        onCancel={handleHistoryCancel}
        t={t}
      />

      <WechatPayQrModal
        t={t}
        visible={wechatQrVisible}
        payment={wechatQrPayment}
        onCancel={() => setWechatQrVisible(false)}
        onRefresh={handleWechatQrRefresh}
        onOpenHistory={handleWechatQrOpenHistory}
      />

      {/* Creem 充值确认模态框 */}
      <Modal
        title={t('确定要充值 $')}
        visible={creemOpen}
        onOk={onlineCreemTopUp}
        onCancel={handleCreemCancel}
        maskClosable={false}
        size='small'
        centered
        confirmLoading={confirmLoading}
      >
        {selectedCreemProduct && (
          <>
            <p>
              {t('产品名称')}：{selectedCreemProduct.name}
            </p>
            <p>
              {t('价格')}：{selectedCreemProduct.currency === 'EUR' ? '€' : '$'}
              {selectedCreemProduct.price}
            </p>
            <p>
              {t('充值额度')}：{selectedCreemProduct.quota}
            </p>
            <p>{t('是否确认充值？')}</p>
          </>
        )}
      </Modal>

      {/* 主布局区域 */}
      <div className='flex flex-col gap-6'>
        <WalletStatsCard
          t={t}
          user={userState?.user}
          account={walletAccount}
          loading={walletLoading}
          renderAmount={renderQuotaWithAmount}
          renderQuota={renderQuota}
        />

        <VipActivationCard
          t={t}
          vipInfo={vipInfo}
          loading={vipLoading}
          processing={vipProcessing}
          onPay={handleVipActivationPay}
          onCopyInviteLink={async (value) => {
            if (await copy(value)) {
              showSuccess(t('邀请链接已复制到剪切板'));
            }
          }}
          renderAmount={renderQuotaWithAmount}
        />

        <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
          <RechargeCard
            t={t}
            enableOnlineTopUp={enableOnlineTopUp}
            enableStripeTopUp={enableStripeTopUp}
            enableCreemTopUp={enableCreemTopUp}
            creemProducts={creemProducts}
            creemPreTopUp={creemPreTopUp}
            enableWaffoTopUp={enableWaffoTopUp}
            enableWaffoPancakeTopUp={enableWaffoPancakeTopUp}
            presetAmounts={presetAmounts}
            selectedPreset={selectedPreset}
            selectPresetAmount={selectPresetAmount}
            formatLargeNumber={formatLargeNumber}
            priceRatio={priceRatio}
            topUpCount={topUpCount}
            minTopUp={minTopUp}
            renderQuotaWithAmount={renderQuotaWithAmount}
            getAmount={getAmount}
            setTopUpCount={setTopUpCount}
            setSelectedPreset={setSelectedPreset}
            renderAmount={renderAmount}
            amountLoading={amountLoading}
            payMethods={confirmPayMethods}
            subscriptionPayMethods={subscriptionPayMethods}
            hasAnyRechargeMethod={hasAnyRechargeMethod}
            preTopUp={preTopUp}
            paymentLoading={paymentLoading}
            payWay={payWay}
            redemptionCode={redemptionCode}
            setRedemptionCode={setRedemptionCode}
            topUp={topUp}
            isSubmitting={isSubmitting}
            topUpLink={topUpLink}
            openTopUpLink={openTopUpLink}
            userState={userState}
            renderQuota={renderQuota}
            statusLoading={statusLoading}
            topupInfo={topupInfo}
            onOpenHistory={handleOpenHistory}
            subscriptionLoading={subscriptionLoading}
            subscriptionPlans={subscriptionPlans}
            billingPreference={billingPreference}
            onChangeBillingPreference={updateBillingPreference}
            activeSubscriptions={activeSubscriptions}
            allSubscriptions={allSubscriptions}
            reloadSubscriptionSelf={getSubscriptionSelf}
            enableRedemption={topupInfo.enable_redemption !== false}
          />
          <InvitationCard
            t={t}
            userState={userState}
            renderAmount={renderQuotaWithAmount}
            setOpenTransfer={setOpenTransfer}
            setOpenWithdraw={setOpenWithdraw}
            affLink={affLink}
            handleAffLinkClick={handleAffLinkClick}
            complianceConfirmed={
              topupInfo.payment_compliance_confirmed !== false
            }
            commissionAmount={walletAccount?.commission_amount || 0}
            totalCommissionAmount={walletAccount?.total_commission_amount || 0}
            withdrawSupported={isClassicWalletWithdrawSupported()}
          />
        </div>

        <WalletRecordsCard
          t={t}
          refreshKey={walletRecordRefreshKey}
          renderAmount={renderQuotaWithAmount}
        />
      </div>
    </div>
  );
};

export default TopUp;
