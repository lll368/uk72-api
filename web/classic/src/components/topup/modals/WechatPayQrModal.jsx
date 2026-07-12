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

import React from 'react';
import { Modal, Button, Card, Typography, Space } from '@douyinfe/semi-ui';
import { QRCodeSVG } from 'qrcode.react';

const { Text } = Typography;

const formatExpiresAt = (expiresAt) => {
  if (!expiresAt) return '';
  const numeric =
    typeof expiresAt === 'number' ? expiresAt : Number.parseInt(expiresAt, 10);
  if (!Number.isFinite(numeric) || numeric <= 0) return '';
  const millis = numeric > 10000000000 ? numeric : numeric * 1000;
  return new Date(millis).toLocaleString();
};

const WechatPayQrModal = ({
  t,
  visible,
  payment,
  onCancel,
  onRefresh,
  onOpenHistory,
}) => {
  const expiresAt = formatExpiresAt(payment?.expires_at);
  const orderNo = payment?.trade_no || payment?.order_id || '';

  return (
    <Modal
      title={
        payment?.purpose === 'vvip_activation'
          ? t('微信支付直连开通VVIP')
          : t('微信支付直连充值')
      }
      visible={visible}
      onCancel={onCancel}
      footer={
        <Space>
          <Button onClick={onOpenHistory}>{t('订单记录')}</Button>
          <Button theme='solid' onClick={onRefresh}>
            {t('刷新状态')}
          </Button>
        </Space>
      }
      maskClosable={false}
      size='small'
      centered
    >
      {payment?.code_url ? (
        <div className='flex flex-col items-center gap-4'>
          <div className='rounded-xl border border-gray-200 bg-white p-4'>
            <QRCodeSVG value={payment.code_url} size={220} />
          </div>
          <Card className='w-full !rounded-lg'>
            <div className='flex flex-col gap-2 text-sm'>
              <div className='flex justify-between gap-3'>
                <Text type='secondary'>{t('状态')}</Text>
                <Text strong>{t('待支付')}</Text>
              </div>
              {orderNo && (
                <div className='flex justify-between gap-3'>
                  <Text type='secondary'>{t('订单号')}</Text>
                  <Text ellipsis={{ showTooltip: true }}>{orderNo}</Text>
                </div>
              )}
              {expiresAt && (
                <div className='flex justify-between gap-3'>
                  <Text type='secondary'>{t('过期时间')}</Text>
                  <Text>{expiresAt}</Text>
                </div>
              )}
            </div>
          </Card>
          <Text type='secondary' size='small' className='text-center'>
            {t('微信支付完成后请刷新状态，或在订单记录中查看结果。')}
          </Text>
        </div>
      ) : null}
    </Modal>
  );
};

export default WechatPayQrModal;
