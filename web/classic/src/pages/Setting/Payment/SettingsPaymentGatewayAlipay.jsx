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

export default function SettingsPaymentGatewayAlipay(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle ? undefined : t('支付宝直连设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    AlipayEnabled: false,
    AlipaySandbox: false,
    AlipayAppId: '',
    AlipayPrivateKey: '',
    AlipayPublicKey: '',
    AlipayUnitPrice: 7.3,
    AlipayMinTopUp: 1,
    AlipayReturnUrl: '',
    AlipayNotifyUrl: '',
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (!props.options || !formApiRef.current) {
      return;
    }
    const currentInputs = {
      AlipayEnabled: props.options.AlipayEnabled || false,
      AlipaySandbox: props.options.AlipaySandbox || false,
      AlipayAppId: props.options.AlipayAppId || '',
      AlipayPrivateKey: '',
      AlipayPublicKey: props.options.AlipayPublicKey || '',
      AlipayUnitPrice:
        props.options.AlipayUnitPrice !== undefined
          ? Number(props.options.AlipayUnitPrice)
          : 7.3,
      AlipayMinTopUp:
        props.options.AlipayMinTopUp !== undefined
          ? Number(props.options.AlipayMinTopUp)
          : 1,
      AlipayReturnUrl: props.options.AlipayReturnUrl || '',
      AlipayNotifyUrl: props.options.AlipayNotifyUrl || '',
    };
    setInputs(currentInputs);
    formApiRef.current.setValues(currentInputs);
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitAlipaySetting = async () => {
    const unitPrice = Number(inputs.AlipayUnitPrice);
    const minTopUp = Number(inputs.AlipayMinTopUp);
    if (!Number.isFinite(unitPrice) || unitPrice <= 0) {
      showError(t('支付宝单位价格必须大于 0'));
      return;
    }
    if (!Number.isFinite(minTopUp) || minTopUp < 0) {
      showError(t('支付宝最低充值金额不能小于 0'));
      return;
    }

    setLoading(true);
    try {
      const options = [
        {
          key: 'AlipayEnabled',
          value: inputs.AlipayEnabled ? 'true' : 'false',
        },
        {
          key: 'AlipaySandbox',
          value: inputs.AlipaySandbox ? 'true' : 'false',
        },
        { key: 'AlipayAppId', value: inputs.AlipayAppId || '' },
        { key: 'AlipayUnitPrice', value: String(unitPrice) },
        { key: 'AlipayMinTopUp', value: String(Math.floor(minTopUp)) },
        {
          key: 'AlipayReturnUrl',
          value: removeTrailingSlash(inputs.AlipayReturnUrl || ''),
        },
        {
          key: 'AlipayNotifyUrl',
          value: removeTrailingSlash(inputs.AlipayNotifyUrl || ''),
        },
      ];

      if ((inputs.AlipayPrivateKey || '').trim()) {
        options.push({
          key: 'AlipayPrivateKey',
          value: inputs.AlipayPrivateKey.trim(),
        });
      }
      if ((inputs.AlipayPublicKey || '').trim()) {
        options.push({
          key: 'AlipayPublicKey',
          value: inputs.AlipayPublicKey.trim(),
        });
      }

      const results = await Promise.all(
        options.map((opt) => API.put('/api/option/', opt)),
      );
      const failed = results.filter((res) => !res.data.success);
      if (failed.length > 0) {
        failed.forEach((res) => showError(res.data.message));
        return;
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
              '支付宝直连独立于易支付配置，支付方式类型为 alipay_direct。',
            )}
            style={{ marginBottom: 16 }}
          />

          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Switch field='AlipayEnabled' label={t('启用支付宝直连')} />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Switch field='AlipaySandbox' label={t('沙箱模式')} />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='AlipayAppId'
                label={t('支付宝应用 ID')}
                placeholder='2021000000000000'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='AlipayUnitPrice'
                precision={2}
                min={0.01}
                label={t('支付宝单位价格（本币 / USD）')}
                placeholder='7.3'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='AlipayMinTopUp'
                min={0}
                label={t('支付宝最低充值美元数量')}
                placeholder='1'
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.TextArea
                field='AlipayPrivateKey'
                label={t('支付宝商户私钥')}
                placeholder={t('留空表示不更新已保存密钥')}
                autosize={{ minRows: 4, maxRows: 8 }}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.TextArea
                field='AlipayPublicKey'
                label={t('支付宝公钥')}
                placeholder={t('支持 PEM 或纯 base64 格式')}
                autosize={{ minRows: 4, maxRows: 8 }}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='AlipayReturnUrl'
                label={t('支付返回地址')}
                placeholder='https://example.com/console/topup'
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='AlipayNotifyUrl'
                label={t('支付宝异步通知地址')}
                placeholder='https://example.com/api/alipay/notify'
              />
            </Col>
          </Row>

          <Button onClick={submitAlipaySetting} style={{ marginTop: 16 }}>
            {t('更新支付宝直连设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
