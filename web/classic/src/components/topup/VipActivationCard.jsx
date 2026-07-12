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
import { Button, Card, Input, Skeleton, Space, Tag, Typography } from '@douyinfe/semi-ui';
import { Copy, CreditCard, Crown } from 'lucide-react';

const VipActivationCard = ({
  t,
  vipInfo,
  loading,
  processing,
  onPay,
  onCopyInviteLink,
  renderAmount,
}) => {
  const isActive = vipInfo?.is_vvip === true;
  const methods = vipInfo?.payment_methods || [];

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex flex-col gap-4'>
        <div className='flex items-start justify-between gap-3'>
          <div className='flex items-center gap-3'>
            <div className='rounded-full bg-yellow-100 p-2 text-yellow-700'>
              <Crown size={18} />
            </div>
            <div>
              <Typography.Text strong className='text-lg'>
                {t('VVIP开通')}
              </Typography.Text>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('一次性付费开通邀请权限')}
              </div>
            </div>
          </div>
          <Tag color={isActive ? 'green' : 'orange'}>
            {isActive ? t('已开通') : renderAmount(vipInfo?.paid_amount || 1680)}
          </Tag>
        </div>

        <Skeleton loading={loading} active placeholder={<Skeleton.Title style={{ width: '80%', height: 40 }} />}>
          {isActive ? (
            <div className='space-y-3'>
              <div className='grid grid-cols-1 sm:grid-cols-3 gap-3 text-sm'>
                <div>
                  <div className='text-[var(--semi-color-text-2)]'>{t('状态')}</div>
                  <div className='font-medium'>{t('已开通')}</div>
                </div>
                <div>
                  <div className='text-[var(--semi-color-text-2)]'>{t('开通时间')}</div>
                  <div className='font-medium'>
                    {vipInfo?.activated_at
                      ? new Date(vipInfo.activated_at * 1000).toLocaleString()
                      : '-'}
                  </div>
                </div>
                <div>
                  <div className='text-[var(--semi-color-text-2)]'>{t('邀请码')}</div>
                  <div className='font-medium'>{vipInfo?.aff_code || '-'}</div>
                </div>
              </div>
              {vipInfo?.invite_link && (
                <Input
                  value={vipInfo.invite_link}
                  readonly
                  prefix={t('邀请链接')}
                  suffix={
                    <Button
                      icon={<Copy size={14} />}
                      onClick={() => onCopyInviteLink(vipInfo.invite_link)}
                    >
                      {t('复制')}
                    </Button>
                  }
                />
              )}
            </div>
          ) : (
            <div className='space-y-3'>
              <div>
                <Typography.Text strong className='text-2xl'>
                  {renderAmount(vipInfo?.paid_amount || 1680)}
                </Typography.Text>
                <div className='text-sm text-[var(--semi-color-text-2)]'>
                  {t('固定价格，不扣减账户余额或额度')}
                </div>
              </div>
              <Space wrap>
                {methods.length > 0 ? (
                  methods.map((method) => (
                    <Button
                      key={method.type}
                      theme='outline'
                      type='tertiary'
                      icon={<CreditCard size={15} />}
                      loading={processing === method.type}
                      disabled={Boolean(processing)}
                      onClick={() => onPay(method)}
                    >
                      {method.name || method.type}
                    </Button>
                  ))
                ) : (
                  <Typography.Text type='tertiary'>
                    {t('暂无可用的VVIP支付方式')}
                  </Typography.Text>
                )}
              </Space>
            </div>
          )}
        </Skeleton>
      </div>
    </Card>
  );
};

export default VipActivationCard;
