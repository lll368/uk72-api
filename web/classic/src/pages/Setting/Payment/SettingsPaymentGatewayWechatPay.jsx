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

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { Info } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';

const getByteLength = (value) => new TextEncoder().encode(value || '').length;

export default function SettingsPaymentGatewayWechatPay(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle
    ? undefined
    : t('微信支付直连设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    WechatPayEnabled: false,
    WechatPaySandbox: false,
    WechatPayAppId: '',
    WechatPayMchId: '',
    WechatPayMerchantSerialNo: '',
    WechatPayMerchantPrivateKey: '',
    WechatPayAPIv3Key: '',
    WechatPayPlatformSerialNo: '',
    WechatPayPlatformPublicKey: '',
    WechatPayUnitPrice: 7.3,
    WechatPayMinTopUp: 1,
    WechatPayNotifyUrl: '',
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (!props.options || !formApiRef.current) {
      return;
    }
    const currentInputs = {
      WechatPayEnabled: props.options.WechatPayEnabled || false,
      WechatPaySandbox: props.options.WechatPaySandbox || false,
      WechatPayAppId: props.options.WechatPayAppId || '',
      WechatPayMchId: props.options.WechatPayMchId || '',
      WechatPayMerchantSerialNo: props.options.WechatPayMerchantSerialNo || '',
      WechatPayMerchantPrivateKey: '',
      WechatPayAPIv3Key: '',
      WechatPayPlatformSerialNo: props.options.WechatPayPlatformSerialNo || '',
      WechatPayPlatformPublicKey:
        props.options.WechatPayPlatformPublicKey || '',
      WechatPayUnitPrice:
        props.options.WechatPayUnitPrice !== undefined
          ? Number(props.options.WechatPayUnitPrice)
          : 7.3,
      WechatPayMinTopUp:
        props.options.WechatPayMinTopUp !== undefined
          ? Number(props.options.WechatPayMinTopUp)
          : 1,
      WechatPayNotifyUrl: props.options.WechatPayNotifyUrl || '',
    };
    setInputs(currentInputs);
    formApiRef.current.setValues(currentInputs);
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitWechatPaySetting = async () => {
    const unitPrice = Number(inputs.WechatPayUnitPrice);
    const minTopUp = Number(inputs.WechatPayMinTopUp);
    const apiV3Key = (inputs.WechatPayAPIv3Key || '').trim();
    if (!Number.isFinite(unitPrice) || unitPrice <= 0) {
      showError(t('微信支付单位价格必须大于 0'));
      return;
    }
    if (!Number.isFinite(minTopUp) || minTopUp < 0) {
      showError(t('微信支付最低充值金额不能小于 0'));
      return;
    }
    if (apiV3Key && getByteLength(apiV3Key) !== 32) {
      showError(t('微信支付 API v3 密钥必须为 32 字节'));
      return;
    }

    setLoading(true);
    try {
      const options = [
        { key: 'WechatPayAppId', value: inputs.WechatPayAppId || '' },
        { key: 'WechatPayMchId', value: inputs.WechatPayMchId || '' },
        {
          key: 'WechatPayMerchantSerialNo',
          value: inputs.WechatPayMerchantSerialNo || '',
        },
        {
          key: 'WechatPayPlatformSerialNo',
          value: inputs.WechatPayPlatformSerialNo || '',
        },
        { key: 'WechatPayUnitPrice', value: String(unitPrice) },
        { key: 'WechatPayMinTopUp', value: String(Math.floor(minTopUp)) },
        {
          key: 'WechatPayNotifyUrl',
          value: removeTrailingSlash(inputs.WechatPayNotifyUrl || ''),
        },
      ];

      if ((inputs.WechatPayMerchantPrivateKey || '').trim()) {
        options.push({
          key: 'WechatPayMerchantPrivateKey',
          value: inputs.WechatPayMerchantPrivateKey.trim(),
        });
      }
      if (apiV3Key) {
        options.push({
          key: 'WechatPayAPIv3Key',
          value: apiV3Key,
        });
      }
      if ((inputs.WechatPayPlatformPublicKey || '').trim()) {
        options.push({
          key: 'WechatPayPlatformPublicKey',
          value: inputs.WechatPayPlatformPublicKey.trim(),
        });
      }

      options.push({
        key: 'WechatPayEnabled',
        value: inputs.WechatPayEnabled ? 'true' : 'false',
      });

      for (const option of options) {
        const res = await API.put('/api/option/', option);
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
      }

      showSuccess(t('更新成功'));
      props.refresh && props.refresh();
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Banner
            type='info'
            icon={<Info size={16} />}
            description={t(
              '微信支付直连独立于易支付配置，支付方式类型为 wechat_direct。',
            )}
            style={{ marginBottom: 16 }}
          />

          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Switch
                field='WechatPayEnabled'
                label={t('启用微信支付直连')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WechatPayAppId'
                label={t('微信支付应用 ID')}
                placeholder='wx0000000000000000'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WechatPayMchId'
                label={t('微信支付商户号')}
                placeholder='1900000001'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WechatPayMerchantSerialNo'
                label={t('商户证书序列号')}
                placeholder='merchant certificate serial'
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='WechatPayUnitPrice'
                precision={2}
                min={0.01}
                label={t('微信支付单位价格（本币 / USD）')}
                placeholder='7.3'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='WechatPayMinTopUp'
                min={0}
                label={t('微信支付最低充值美元数量')}
                placeholder='1'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WechatPayPlatformSerialNo'
                label={t('微信支付平台序列号')}
                placeholder='platform certificate serial'
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.TextArea
                field='WechatPayMerchantPrivateKey'
                label={t('商户 API 证书私钥')}
                placeholder={t('留空表示不更新已保存密钥')}
                autosize={{ minRows: 4, maxRows: 8 }}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WechatPayAPIv3Key'
                label={t('微信支付 API v3 密钥')}
                placeholder={t('32 字节 API v3 密钥，留空不更新')}
                type='password'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.TextArea
                field='WechatPayPlatformPublicKey'
                label={t('微信支付平台公钥')}
                placeholder={t('支持 PEM 或纯 base64 格式')}
                autosize={{ minRows: 4, maxRows: 8 }}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={24} lg={24} xl={24}>
              <Form.Input
                field='WechatPayNotifyUrl'
                label={t('微信支付异步通知地址')}
                placeholder='https://example.com/api/wechat/notify'
              />
            </Col>
          </Row>

          <Button onClick={submitWechatPaySetting} style={{ marginTop: 16 }}>
            {t('更新微信支付直连设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
