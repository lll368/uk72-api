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

import React, { useEffect, useState, useRef } from 'react';
import { Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
  verifyJSON,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const VIP_ACTIVATION_PRICE_KEY = 'payment_setting.vip_activation_price';
const VIP_ACTIVATION_LEVEL1_AMOUNT_KEY =
  'payment_setting.vip_activation_commission_level1_amount';
const VIP_ACTIVATION_LEVEL2_AMOUNT_KEY =
  'payment_setting.vip_activation_commission_level2_amount';
const VIP_ACTIVATION_MONEY_PRECISION_MESSAGE =
  'VVIP activation money fields support at most 2 decimal places';

const hasAtMostTwoDecimalPlaces = (value) => {
  if (!Number.isFinite(value)) {
    return false;
  }
  const normalized = value.toString().toLowerCase();
  if (normalized.includes('e')) {
    return Math.abs(Math.round(value * 100) - value * 100) < 1e-9;
  }
  const decimalPart = normalized.split('.')[1];
  return !decimalPart || decimalPart.length <= 2;
};

const orderVipActivationCommissionOptions = (
  options,
  originInputs,
  nextAmounts,
) => {
  const priceOption = options.find(
    (option) => option.key === VIP_ACTIVATION_PRICE_KEY,
  );
  const level1Option = options.find(
    (option) => option.key === VIP_ACTIVATION_LEVEL1_AMOUNT_KEY,
  );
  const level2Option = options.find(
    (option) => option.key === VIP_ACTIVATION_LEVEL2_AMOUNT_KEY,
  );

  if (!priceOption && !level1Option && !level2Option) {
    return options;
  }

  const orderedAmountOptions = [];
  const pushIfPresent = (option) => {
    if (option) {
      orderedAmountOptions.push(option);
    }
  };
  const originLevel1Amount = Number(
    originInputs.VvipActivationCommissionLevel1Amount,
  );
  const originLevel2Amount = Number(
    originInputs.VvipActivationCommissionLevel2Amount,
  );

  // 后端逐项校验金额合计，先降分佣、再改价格、最后升分佣，避免合法批量调整被中间态拦截。
  if (nextAmounts.level1 < originLevel1Amount) {
    pushIfPresent(level1Option);
  }
  if (nextAmounts.level2 < originLevel2Amount) {
    pushIfPresent(level2Option);
  }
  pushIfPresent(priceOption);
  if (nextAmounts.level1 >= originLevel1Amount) {
    pushIfPresent(level1Option);
  }
  if (nextAmounts.level2 >= originLevel2Amount) {
    pushIfPresent(level2Option);
  }

  return [
    ...options.filter(
      (option) =>
        option.key !== VIP_ACTIVATION_PRICE_KEY &&
        option.key !== VIP_ACTIVATION_LEVEL1_AMOUNT_KEY &&
        option.key !== VIP_ACTIVATION_LEVEL2_AMOUNT_KEY,
    ),
    ...orderedAmountOptions,
  ];
};

const parseNumberWithFallback = (value, fallback) => {
  if (value === undefined || value === null) {
    return fallback;
  }
  if (typeof value === 'string' && value.trim() === '') {
    return fallback;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
};

export default function SettingsGeneralPayment(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle ? undefined : t('通用设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    ServerAddress: '',
    CustomCallbackAddress: '',
    TopupGroupRatio: '',
    PayMethods: '',
    AmountOptions: '',
    AmountDiscount: '',
    DefaultUserTopupDiscount: 1,
    DefaultVvipTopupDiscount: 1,
    VipActivationPrice: 1680,
    VvipActivationCommissionLevel1Amount: 1000,
    VvipActivationCommissionLevel2Amount: 400,
  });
  const [originInputs, setOriginInputs] = useState({});
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        ServerAddress: props.options.ServerAddress || '',
        CustomCallbackAddress: props.options.CustomCallbackAddress || '',
        TopupGroupRatio: props.options.TopupGroupRatio || '',
        PayMethods: props.options.PayMethods || '',
        AmountOptions: props.options.AmountOptions || '',
        AmountDiscount: props.options.AmountDiscount || '',
        DefaultUserTopupDiscount: props.options.DefaultUserTopupDiscount || 1,
        DefaultVvipTopupDiscount: props.options.DefaultVvipTopupDiscount || 1,
        VipActivationPrice: parseNumberWithFallback(
          props.options.VipActivationPrice,
          1680,
        ),
        VvipActivationCommissionLevel1Amount: parseNumberWithFallback(
          props.options.VvipActivationCommissionLevel1Amount,
          1000,
        ),
        VvipActivationCommissionLevel2Amount: parseNumberWithFallback(
          props.options.VvipActivationCommissionLevel2Amount,
          400,
        ),
      };
      setInputs(currentInputs);
      setOriginInputs({ ...currentInputs });
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitGeneralSettings = async () => {
    if (
      originInputs.TopupGroupRatio !== inputs.TopupGroupRatio &&
      !verifyJSON(inputs.TopupGroupRatio)
    ) {
      showError(t('充值分组倍率不是合法的 JSON 字符串'));
      return;
    }

    if (
      originInputs.PayMethods !== inputs.PayMethods &&
      !verifyJSON(inputs.PayMethods)
    ) {
      showError(t('充值方式设置不是合法的 JSON 字符串'));
      return;
    }

    if (
      originInputs.AmountOptions !== inputs.AmountOptions &&
      inputs.AmountOptions.trim() !== '' &&
      !verifyJSON(inputs.AmountOptions)
    ) {
      showError(t('自定义充值数量选项不是合法的 JSON 数组'));
      return;
    }

    if (
      originInputs.AmountDiscount !== inputs.AmountDiscount &&
      inputs.AmountDiscount.trim() !== '' &&
      !verifyJSON(inputs.AmountDiscount)
    ) {
      showError(t('充值金额折扣配置不是合法的 JSON 对象'));
      return;
    }

    const defaultUserTopupDiscount = Number(inputs.DefaultUserTopupDiscount);
    const defaultVvipTopupDiscount = Number(inputs.DefaultVvipTopupDiscount);
    const vipActivationPrice = Number(inputs.VipActivationPrice);
    const vvipActivationCommissionLevel1Amount = Number(
      inputs.VvipActivationCommissionLevel1Amount,
    );
    const vvipActivationCommissionLevel2Amount = Number(
      inputs.VvipActivationCommissionLevel2Amount,
    );
    if (
      !Number.isFinite(defaultUserTopupDiscount) ||
      defaultUserTopupDiscount <= 0 ||
      defaultUserTopupDiscount > 1
    ) {
      showError(
        t(
          'Default user top-up discount must be greater than 0 and less than or equal to 1',
        ),
      );
      return;
    }
    if (
      !Number.isFinite(defaultVvipTopupDiscount) ||
      defaultVvipTopupDiscount <= 0 ||
      defaultVvipTopupDiscount > 1
    ) {
      showError(
        t(
          'Default VVIP top-up discount must be greater than 0 and less than or equal to 1',
        ),
      );
      return;
    }
    if (
      !Number.isFinite(vipActivationPrice) ||
      vipActivationPrice <= 0 ||
      !Number.isFinite(vvipActivationCommissionLevel1Amount) ||
      vvipActivationCommissionLevel1Amount < 0 ||
      !Number.isFinite(vvipActivationCommissionLevel2Amount) ||
      vvipActivationCommissionLevel2Amount < 0
    ) {
      showError(
        t(
          'VVIP activation commission amounts cannot exceed activation price in total',
        ),
      );
      return;
    }
    if (
      !hasAtMostTwoDecimalPlaces(vipActivationPrice) ||
      !hasAtMostTwoDecimalPlaces(vvipActivationCommissionLevel1Amount) ||
      !hasAtMostTwoDecimalPlaces(vvipActivationCommissionLevel2Amount)
    ) {
      showError(t(VIP_ACTIVATION_MONEY_PRECISION_MESSAGE));
      return;
    }
    if (
      vvipActivationCommissionLevel1Amount +
        vvipActivationCommissionLevel2Amount >
      vipActivationPrice
    ) {
      showError(
        t(
          'VVIP activation commission amounts cannot exceed activation price in total',
        ),
      );
      return;
    }

    setLoading(true);
    try {
      const options = [
        {
          key: 'ServerAddress',
          value: removeTrailingSlash(inputs.ServerAddress),
        },
      ];

      if (inputs.CustomCallbackAddress !== '') {
        options.push({
          key: 'CustomCallbackAddress',
          value: removeTrailingSlash(inputs.CustomCallbackAddress),
        });
      }
      if (originInputs.TopupGroupRatio !== inputs.TopupGroupRatio) {
        options.push({ key: 'TopupGroupRatio', value: inputs.TopupGroupRatio });
      }
      if (originInputs.PayMethods !== inputs.PayMethods) {
        options.push({ key: 'PayMethods', value: inputs.PayMethods });
      }
      if (originInputs.AmountOptions !== inputs.AmountOptions) {
        options.push({
          key: 'payment_setting.amount_options',
          value: inputs.AmountOptions,
        });
      }
      if (originInputs.AmountDiscount !== inputs.AmountDiscount) {
        options.push({
          key: 'payment_setting.amount_discount',
          value: inputs.AmountDiscount,
        });
      }
      if (
        Number(originInputs.DefaultUserTopupDiscount) !==
        defaultUserTopupDiscount
      ) {
        options.push({
          key: 'payment_setting.default_user_topup_discount',
          value: defaultUserTopupDiscount,
        });
      }
      if (
        Number(originInputs.DefaultVvipTopupDiscount) !==
        defaultVvipTopupDiscount
      ) {
        options.push({
          key: 'payment_setting.default_vvip_topup_discount',
          value: defaultVvipTopupDiscount,
        });
      }
      if (Number(originInputs.VipActivationPrice) !== vipActivationPrice) {
        options.push({
          key: VIP_ACTIVATION_PRICE_KEY,
          value: vipActivationPrice,
        });
      }
      if (
        Number(originInputs.VvipActivationCommissionLevel1Amount) !==
        vvipActivationCommissionLevel1Amount
      ) {
        options.push({
          key: VIP_ACTIVATION_LEVEL1_AMOUNT_KEY,
          value: vvipActivationCommissionLevel1Amount,
        });
      }
      if (
        Number(originInputs.VvipActivationCommissionLevel2Amount) !==
        vvipActivationCommissionLevel2Amount
      ) {
        options.push({
          key: VIP_ACTIVATION_LEVEL2_AMOUNT_KEY,
          value: vvipActivationCommissionLevel2Amount,
        });
      }

      const orderedOptions = orderVipActivationCommissionOptions(
        options,
        originInputs,
        {
          level1: vvipActivationCommissionLevel1Amount,
          level2: vvipActivationCommissionLevel2Amount,
        },
      );
      const results = [];
      for (const option of orderedOptions) {
        const response = await API.put('/api/option/', {
          key: option.key,
          value: option.value,
        });
        results.push(response);
      }

      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length === 0) {
        showSuccess(t('更新成功'));
        setOriginInputs({ ...inputs });
        props.refresh && props.refresh();
      } else {
        errorResults.forEach((res) => {
          showError(res.data.message);
        });
      }
    } catch (error) {
      showError(t('更新失败'));
    }
    setLoading(false);
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Form.Input
            field='ServerAddress'
            label={t('服务器地址')}
            placeholder={'https://yourdomain.com'}
            style={{ width: '100%' }}
            extraText={t(
              '该服务器地址将影响支付回调地址以及默认首页展示的地址，请确保正确配置',
            )}
          />
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='CustomCallbackAddress'
                label={t('回调地址')}
                placeholder={t('例如：https://yourdomain.com')}
                extraText={t(
                  '留空时默认使用服务器地址作为回调地址，填写后将覆盖默认值',
                )}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.TextArea
                field='TopupGroupRatio'
                label={t('充值分组倍率')}
                placeholder={t('为一个 JSON 文本，键为组名称，值为倍率')}
                autosize
              />
            </Col>
          </Row>
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.TextArea
                field='PayMethods'
                label={t('充值方式设置')}
                placeholder={t('为一个 JSON 文本')}
                autosize
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.TextArea
                field='AmountOptions'
                label={t('自定义充值数量选项')}
                placeholder={t(
                  '为一个 JSON 数组，例如：[10, 20, 50, 100, 200, 500]',
                )}
                autosize
                extraText={t(
                  '设置用户可选择的充值数量选项，例如：[10, 20, 50, 100, 200, 500]',
                )}
              />
            </Col>
          </Row>
          <Row style={{ marginTop: 16 }}>
            <Col span={24}>
              <Form.TextArea
                field='AmountDiscount'
                label={t('充值金额折扣配置')}
                placeholder={t(
                  '为一个 JSON 对象，例如：{"100": 0.95, "200": 0.9, "500": 0.85}',
                )}
                autosize
                extraText={t(
                  '设置不同充值金额对应的折扣，键为充值金额，值为折扣率，例如：{"100": 0.95, "200": 0.9, "500": 0.85}',
                )}
              />
            </Col>
          </Row>
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.InputNumber
                field='DefaultUserTopupDiscount'
                label={t('Default user top-up discount')}
                min={0.01}
                max={1}
                step={0.01}
                precision={4}
                style={{ width: '100%' }}
                extraText={t(
                  'Applied to newly registered or created users. Use 1 for no discount.',
                )}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.InputNumber
                field='DefaultVvipTopupDiscount'
                label={t('Default VVIP top-up discount')}
                min={0.01}
                max={1}
                step={0.01}
                precision={4}
                style={{ width: '100%' }}
                extraText={t(
                  'Applied after a user successfully activates VVIP. Use 1 for no discount.',
                )}
              />
            </Col>
          </Row>
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='VipActivationPrice'
                label={t('VVIP activation price')}
                min={0.01}
                step={0.01}
                precision={2}
                style={{ width: '100%' }}
                extraText={t(
                  'One-time VVIP activation amount charged to users.',
                )}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='VvipActivationCommissionLevel1Amount'
                label={t('VVIP activation level 1 commission amount')}
                min={0}
                step={0.01}
                precision={2}
                style={{ width: '100%' }}
                extraText={t(
                  'Fixed amount credited to the direct VVIP parent when a user activates VVIP.',
                )}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='VvipActivationCommissionLevel2Amount'
                label={t('VVIP activation level 2 commission amount')}
                min={0}
                step={0.01}
                precision={2}
                style={{ width: '100%' }}
                extraText={t(
                  'Fixed amount credited to the indirect VVIP parent. The two amounts cannot exceed the activation price in total.',
                )}
              />
            </Col>
          </Row>
          <Button onClick={submitGeneralSettings} style={{ marginTop: 16 }}>
            {t('保存通用设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
