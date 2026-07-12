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
import { Card, Skeleton, Typography } from '@douyinfe/semi-ui';
import { Activity, BarChart3, HandCoins, Snowflake, WalletCards } from 'lucide-react';

const WalletStatsCard = ({ t, user, account, loading, renderAmount, renderQuota }) => {
  const stats = [
    {
      label: t('可消费余额'),
      value: renderAmount(account?.balance_amount || 0),
      description: t('可用于 API 调用的余额'),
      icon: WalletCards,
    },
    {
      label: t('可提现佣金'),
      value: renderAmount(account?.commission_amount || 0),
      description: t('可提现或划转的佣金'),
      icon: HandCoins,
    },
    {
      label: t('冻结佣金'),
      value: renderAmount(account?.frozen_commission_amount || 0),
      description: t('提现处理中冻结的佣金'),
      icon: Snowflake,
    },
    {
      label: t('剩余额度'),
      value: renderQuota(user?.quota || 0),
      description: t('当前账户可用额度'),
      icon: BarChart3,
    },
    {
      label: t('请求次数'),
      value: (user?.request_count || 0).toLocaleString(),
      description: t('累计 API 请求数'),
      icon: Activity,
    },
  ];

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-3'>
        {stats.map((item) => {
          const Icon = item.icon;
          return (
            <div
              key={item.label}
              className='rounded-xl border border-[var(--semi-color-border)] p-4'
            >
              <div className='flex items-center gap-2 text-xs text-[var(--semi-color-text-2)]'>
                <Icon size={15} />
                <span>{item.label}</span>
              </div>
              <Skeleton loading={loading} active placeholder={<Skeleton.Title style={{ width: 96, height: 24, marginTop: 10 }} />}>
                <Typography.Text strong className='block mt-2 text-lg break-all'>
                  {item.value}
                </Typography.Text>
              </Skeleton>
              <div className='mt-1 text-xs text-[var(--semi-color-text-2)]'>
                {item.description}
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
};

export default WalletStatsCard;
