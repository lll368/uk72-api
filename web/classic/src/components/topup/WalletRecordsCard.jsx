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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, Empty, Tabs, Toast, Typography } from '@douyinfe/semi-ui';
import { RefreshCw } from 'lucide-react';
import CardTable from '../common/ui/CardTable';
import {
  WALLET_RECORD_PAGE_SIZE,
  getWalletCommissions,
  getWalletFlows,
  getWalletFlowLabelKey,
  getWalletRecordLoadError,
  getWalletStatusKey,
  getWalletWithdraws,
} from '../../helpers/wallet';

const getTime = (timestamp) =>
  timestamp ? new Date(timestamp * 1000).toLocaleString() : '-';

const WalletRecordsCard = ({ t, refreshKey, renderAmount }) => {
  const [activeKey, setActiveKey] = useState('flows');
  const [loading, setLoading] = useState(false);
  const [pages, setPages] = useState({
    flows: 1,
    commissions: 1,
    withdraws: 1,
  });
  const [totals, setTotals] = useState({
    flows: 0,
    commissions: 0,
    withdraws: 0,
  });
  const [records, setRecords] = useState({
    flows: [],
    commissions: [],
    withdraws: [],
  });

  const loadRecords = useCallback(async () => {
    setLoading(true);
    try {
      const [flowResp, commissionResp, withdrawResp] = await Promise.all([
        getWalletFlows(pages.flows, WALLET_RECORD_PAGE_SIZE),
        getWalletCommissions(pages.commissions, WALLET_RECORD_PAGE_SIZE),
        getWalletWithdraws(pages.withdraws, WALLET_RECORD_PAGE_SIZE),
      ]);
      const errorMessage = getWalletRecordLoadError({
        flows: flowResp,
        commissions: commissionResp,
        withdraws: withdrawResp,
      });
      if (errorMessage) {
        Toast.error({ content: t(errorMessage) });
        return;
      }
      setRecords({
        flows: flowResp?.data?.items || [],
        commissions: commissionResp?.data?.items || [],
        withdraws: withdrawResp?.data?.items || [],
      });
      setTotals({
        flows: flowResp?.data?.total || 0,
        commissions: commissionResp?.data?.total || 0,
        withdraws: withdrawResp?.data?.total || 0,
      });
    } catch (error) {
      Toast.error({ content: t('钱包记录加载失败') });
    } finally {
      setLoading(false);
    }
  }, [pages, t]);

  useEffect(() => {
    loadRecords();
  }, [loadRecords, refreshKey]);

  const pagination = useMemo(() => {
    const total = totals[activeKey] || 0;
    return {
      currentPage: pages[activeKey],
      pageSize: WALLET_RECORD_PAGE_SIZE,
      total,
      onPageChange: (page) =>
        setPages((value) => ({ ...value, [activeKey]: page })),
    };
  }, [activeKey, pages, totals]);

  const flowColumns = [
    {
      title: t('类型'),
      dataIndex: 'flow_type',
      render: (value) => t(getWalletFlowLabelKey(value)),
    },
    {
      title: t('金额'),
      dataIndex: 'amount',
      render: (value, record) =>
        `${record.direction === 'out' ? '-' : '+'}${renderAmount(value)}`,
    },
    {
      title: t('变动后余额'),
      dataIndex: 'balance_after',
      render: renderAmount,
    },
    {
      title: t('变动后佣金'),
      dataIndex: 'commission_after',
      render: renderAmount,
    },
    {
      title: t('变动后冻结佣金'),
      dataIndex: 'frozen_commission_after',
      render: renderAmount,
    },
    {
      title: t('业务单号'),
      dataIndex: 'biz_no',
      render: (value) => <Typography.Text copyable>{value || '-'}</Typography.Text>,
    },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: getTime,
    },
  ];

  const commissionColumns = [
    {
      title: t('来源用户'),
      dataIndex: 'source_user_id',
    },
    {
      title: t('层级'),
      dataIndex: 'level',
    },
    {
      title: t('佣金金额'),
      dataIndex: 'amount',
      render: renderAmount,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => t(getWalletStatusKey(value)),
    },
  ];

  const withdrawColumns = [
    {
      title: t('提现单号'),
      dataIndex: 'withdraw_no',
      render: (value) => <Typography.Text copyable>{value || '-'}</Typography.Text>,
    },
    {
      title: t('金额'),
      dataIndex: 'amount',
      render: renderAmount,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => t(getWalletStatusKey(value)),
    },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: getTime,
    },
  ];

  const renderTable = (key, columns) => (
    <CardTable
      columns={columns}
      dataSource={records[key]}
      rowKey='id'
      loading={loading}
      pagination={pagination}
      empty={<Empty description={t('暂无钱包记录')} style={{ padding: 30 }} />}
      className='rounded-xl overflow-hidden'
    />
  );

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-4'>
        <Typography.Text strong className='text-lg'>
          {t('钱包记录')}
        </Typography.Text>
        <Button
          theme='outline'
          type='tertiary'
          icon={<RefreshCw size={15} />}
          loading={loading}
          onClick={loadRecords}
        >
          {t('刷新')}
        </Button>
      </div>
      <Tabs activeKey={activeKey} onChange={setActiveKey}>
        <Tabs.TabPane itemKey='flows' tab={t('流水')}>
          {renderTable('flows', flowColumns)}
        </Tabs.TabPane>
        <Tabs.TabPane itemKey='commissions' tab={t('佣金')}>
          {renderTable('commissions', commissionColumns)}
        </Tabs.TabPane>
        <Tabs.TabPane itemKey='withdraws' tab={t('提现')}>
          {renderTable('withdraws', withdrawColumns)}
        </Tabs.TabPane>
      </Tabs>
    </Card>
  );
};

export default WalletRecordsCard;
