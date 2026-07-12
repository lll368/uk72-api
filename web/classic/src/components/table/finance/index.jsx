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
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Select,
  Space,
  Tabs,
  Tag,
  Toast,
  Typography,
} from '@douyinfe/semi-ui';
import { Plus, RefreshCw, RotateCcw } from 'lucide-react';
import CardTable from '../../common/ui/CardTable';
import {
  ADMIN_FINANCE_PAGE_SIZE,
  createFinanceFilterChangeHandler,
  financeApi,
  formatFinanceMoney,
  formatFinanceTimestamp,
  getClassicWithdrawActions,
  getFinancePaymentMethodLabel,
  getFinanceStatusKey,
  getPaymentDiffKey,
  parseProviderOrders,
} from '../../../helpers/finance';

const { Text } = Typography;

const statusColorMap = {
  active: 'green',
  approved: 'blue',
  disabled: 'grey',
  failed: 'red',
  ignored: 'grey',
  paid: 'green',
  pending: 'orange',
  rejected: 'red',
  reversed: 'purple',
  settled: 'green',
  success: 'green',
};

function StatusTag({ t, status }) {
  return (
    <Tag color={statusColorMap[status] || 'grey'}>
      {t(getFinanceStatusKey(status))}
    </Tag>
  );
}

function PageCard({ title, description, actions, filters, children }) {
  return (
    <Card className='!rounded-lg shadow-sm border-0'>
      <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between mb-4'>
        <div>
          <Typography.Title heading={5} className='!mb-1'>
            {title}
          </Typography.Title>
          {description && (
            <Text type='tertiary' size='small'>
              {description}
            </Text>
          )}
        </div>
        <Space wrap>{actions}</Space>
      </div>
      {filters && <div className='mb-4'>{filters}</div>}
      {children}
    </Card>
  );
}

function FilterBar({ children }) {
  return <div className='grid grid-cols-1 md:grid-cols-4 gap-2'>{children}</div>;
}

function PaginationConfig(page, total, setPage) {
  return {
    currentPage: page,
    pageSize: ADMIN_FINANCE_PAGE_SIZE,
    total,
    onPageChange: setPage,
  };
}

function usePagedLoader(loadFn, deps) {
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await loadFn(page);
      if (res?.success) {
        setItems(res.data?.items || []);
        setTotal(res.data?.total || 0);
      } else {
        Toast.error({ content: res?.message || '加载失败' });
      }
    } finally {
      setLoading(false);
    }
  }, [loadFn, page]);

  useEffect(() => {
    refresh();
  }, [refresh, ...deps]);

  return { items, total, page, setPage, loading, refresh };
}

function VipRecordsTable({ t }) {
  const [disableTarget, setDisableTarget] = useState(null);
  const [disableReason, setDisableReason] = useState('');

  const loadFn = useCallback(
    (page) => financeApi.getVipActivationRecords(page, ADMIN_FINANCE_PAGE_SIZE),
    [],
  );
  const data = usePagedLoader(loadFn, []);

  const handleDisable = async () => {
    if (!disableTarget) return;
    const res = await financeApi.disableVipActivation(
      disableTarget.user_id,
      disableReason || 'Disabled by administrator',
    );
    if (res?.success) {
      Toast.success({ content: t('操作成功') });
      setDisableTarget(null);
      setDisableReason('');
      data.refresh();
    } else {
      Toast.error({ content: res?.message || t('操作失败') });
    }
  };

  const columns = [
    { title: t('用户ID'), dataIndex: 'user_id' },
    {
      title: t('订单号'),
      dataIndex: 'trade_no',
      render: (value) => <Text copyable>{value || '-'}</Text>,
    },
    {
      title: t('支付金额'),
      dataIndex: 'paid_amount',
      render: formatFinanceMoney,
    },
    { title: t('支付渠道'), dataIndex: 'payment_provider' },
    {
      title: t('支付方式'),
      dataIndex: 'payment_method',
      render: (value) => t(getFinancePaymentMethodLabel(value)),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => <StatusTag t={t} status={value} />,
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      render: formatFinanceTimestamp,
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) =>
        record.status === 'success' ? (
          <Button
            size='small'
            type='danger'
            theme='outline'
            onClick={() => setDisableTarget(record)}
          >
            {t('禁用')}
          </Button>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <>
      <PageCard
        title={t('VVIP开通记录')}
        description={t('查看VVIP开通支付记录并禁用已开通用户')}
        actions={
          <Button icon={<RefreshCw size={14} />} onClick={data.refresh} loading={data.loading}>
            {t('刷新')}
          </Button>
        }
      >
        <CardTable
          columns={columns}
          dataSource={data.items}
          rowKey='id'
          loading={data.loading}
          pagination={PaginationConfig(data.page, data.total, data.setPage)}
          empty={<Empty description={t('暂无VVIP记录')} />}
        />
      </PageCard>
      <Modal
        title={t('禁用VVIP')}
        visible={Boolean(disableTarget)}
        onCancel={() => setDisableTarget(null)}
        onOk={handleDisable}
        centered
      >
        <Input
          value={disableReason}
          onChange={setDisableReason}
          placeholder={t('禁用原因')}
        />
      </Modal>
    </>
  );
}

function RelationsTable({ t }) {
  const [parentUserId, setParentUserId] = useState('');
  const [childUserId, setChildUserId] = useState('');
  const [status, setStatus] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState({
    parent_user_id: '',
    child_user_id: '',
    source_trade_no: '',
    remark: '',
  });
  const [disableTarget, setDisableTarget] = useState(null);
  const [disableReason, setDisableReason] = useState('');

  const loadFn = useCallback(
    (page) =>
      financeApi.getUserRelations({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        parentUserId,
        childUserId,
        status,
      }),
    [parentUserId, childUserId, status],
  );
  const data = usePagedLoader(loadFn, [parentUserId, childUserId, status]);

  const handleCreate = async () => {
    const parentId = Number(createForm.parent_user_id);
    const childId = Number(createForm.child_user_id);
    if (!parentId || !childId) {
      Toast.error({ content: t('请输入有效用户ID') });
      return;
    }
    const res = await financeApi.createUserRelation({
      parent_user_id: parentId,
      child_user_id: childId,
      source_trade_no: createForm.source_trade_no,
      remark: createForm.remark,
    });
    if (res?.success) {
      Toast.success({ content: t('操作成功') });
      setCreateOpen(false);
      setCreateForm({
        parent_user_id: '',
        child_user_id: '',
        source_trade_no: '',
        remark: '',
      });
      data.refresh();
    } else {
      Toast.error({ content: res?.message || t('操作失败') });
    }
  };

  const handleDisable = async () => {
    if (!disableTarget) return;
    const res = await financeApi.disableUserRelation(
      disableTarget.id,
      disableReason || 'Disabled by administrator',
    );
    if (res?.success) {
      Toast.success({ content: t('操作成功') });
      setDisableTarget(null);
      setDisableReason('');
      data.refresh();
    } else {
      Toast.error({ content: res?.message || t('操作失败') });
    }
  };

  const columns = [
    { title: t('上级用户'), dataIndex: 'parent_user_id' },
    { title: t('下级用户'), dataIndex: 'child_user_id' },
    { title: t('来源'), dataIndex: 'source' },
    {
      title: t('来源订单'),
      dataIndex: 'source_trade_no',
      render: (value) => <Text copyable>{value || '-'}</Text>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => <StatusTag t={t} status={value} />,
    },
    {
      title: t('绑定时间'),
      dataIndex: 'bind_time',
      render: formatFinanceTimestamp,
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) =>
        record.status === 'active' ? (
          <Button size='small' type='danger' theme='outline' onClick={() => setDisableTarget(record)}>
            {t('禁用')}
          </Button>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <>
      <PageCard
        title={t('邀请关系')}
        description={t('查看和调整VVIP邀请关系')}
        actions={
          <>
            <Button icon={<Plus size={14} />} onClick={() => setCreateOpen(true)}>
              {t('绑定关系')}
            </Button>
            <Button icon={<RefreshCw size={14} />} onClick={data.refresh} loading={data.loading}>
              {t('刷新')}
            </Button>
          </>
        }
        filters={
          <FilterBar>
            <Input
              placeholder={t('上级用户ID')}
              value={parentUserId}
              onChange={createFinanceFilterChangeHandler(data.setPage, setParentUserId)}
            />
            <Input
              placeholder={t('下级用户ID')}
              value={childUserId}
              onChange={createFinanceFilterChangeHandler(data.setPage, setChildUserId)}
            />
            <Select
              value={status}
              onChange={createFinanceFilterChangeHandler(data.setPage, setStatus)}
              placeholder={t('状态')}
              showClear
            >
              <Select.Option value='active'>{t('正常')}</Select.Option>
              <Select.Option value='disabled'>{t('已禁用')}</Select.Option>
            </Select>
          </FilterBar>
        }
      >
        <CardTable
          columns={columns}
          dataSource={data.items}
          rowKey='id'
          loading={data.loading}
          pagination={PaginationConfig(data.page, data.total, data.setPage)}
          empty={<Empty description={t('暂无邀请关系')} />}
        />
      </PageCard>

      <Modal title={t('绑定关系')} visible={createOpen} onCancel={() => setCreateOpen(false)} onOk={handleCreate} centered>
        <div className='space-y-3'>
          <Input placeholder={t('上级用户ID')} value={createForm.parent_user_id} onChange={(value) => setCreateForm((form) => ({ ...form, parent_user_id: value }))} />
          <Input placeholder={t('下级用户ID')} value={createForm.child_user_id} onChange={(value) => setCreateForm((form) => ({ ...form, child_user_id: value }))} />
          <Input placeholder={t('来源订单号')} value={createForm.source_trade_no} onChange={(value) => setCreateForm((form) => ({ ...form, source_trade_no: value }))} />
          <Input placeholder={t('备注')} value={createForm.remark} onChange={(value) => setCreateForm((form) => ({ ...form, remark: value }))} />
        </div>
      </Modal>
      <Modal title={t('禁用关系')} visible={Boolean(disableTarget)} onCancel={() => setDisableTarget(null)} onOk={handleDisable} centered>
        <Input value={disableReason} onChange={setDisableReason} placeholder={t('禁用原因')} />
      </Modal>
    </>
  );
}

function CommissionRecordsTable({ t }) {
  const [userId, setUserId] = useState('');
  const [status, setStatus] = useState('');
  const loadFn = useCallback(
    (page) =>
      financeApi.getCommissions({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        userId,
        status,
      }),
    [userId, status],
  );
  const data = usePagedLoader(loadFn, [userId, status]);
  const columns = [
    { title: t('受益用户'), dataIndex: 'beneficiary_user_id' },
    { title: t('来源用户'), dataIndex: 'source_user_id' },
    { title: t('层级'), dataIndex: 'level' },
    { title: t('基础金额'), dataIndex: 'base_amount', render: formatFinanceMoney },
    { title: t('佣金金额'), dataIndex: 'amount', render: formatFinanceMoney },
    { title: t('资格状态'), dataIndex: 'qualification_status' },
    { title: t('状态'), dataIndex: 'status', render: (value) => <StatusTag t={t} status={value} /> },
    { title: t('创建时间'), dataIndex: 'created_at', render: formatFinanceTimestamp },
  ];
  return (
    <PageCard
      title={t('佣金记录')}
      description={t('查看充值和VVIP佣金结算状态')}
      actions={<Button icon={<RefreshCw size={14} />} onClick={data.refresh} loading={data.loading}>{t('刷新')}</Button>}
      filters={
        <FilterBar>
          <Input
            placeholder={t('用户ID')}
            value={userId}
            onChange={createFinanceFilterChangeHandler(data.setPage, setUserId)}
          />
          <Select
            value={status}
            onChange={createFinanceFilterChangeHandler(data.setPage, setStatus)}
            placeholder={t('状态')}
            showClear
          >
            <Select.Option value='pending'>{t('待处理')}</Select.Option>
            <Select.Option value='settled'>{t('已结算')}</Select.Option>
            <Select.Option value='reversed'>{t('已冲正')}</Select.Option>
          </Select>
        </FilterBar>
      }
    >
      <CardTable
        columns={columns}
        dataSource={data.items}
        rowKey='id'
        loading={data.loading}
        pagination={PaginationConfig(data.page, data.total, data.setPage)}
        empty={<Empty description={t('暂无佣金记录')} />}
      />
    </PageCard>
  );
}

function WithdrawOrdersTable({ t }) {
  const [userId, setUserId] = useState('');
  const [status, setStatus] = useState('');
  const [target, setTarget] = useState(null);
  const [action, setAction] = useState('');
  const [actionText, setActionText] = useState('');
  const loadFn = useCallback(
    (page) =>
      financeApi.getWithdraws({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        userId,
        status,
      }),
    [userId, status],
  );
  const data = usePagedLoader(loadFn, [userId, status]);

  const openAction = (record, nextAction) => {
    setTarget(record);
    setAction(nextAction);
    setActionText('');
  };

  const handleAction = async () => {
    if (!target || !action) return;
    let res;
    if (action === 'approve') res = await financeApi.approveWithdraw(target.id);
    if (action === 'reject') res = await financeApi.rejectWithdraw(target.id, actionText || 'rejected');
    if (action === 'pay') res = await financeApi.payWithdraw(target.id, actionText);
    if (action === 'fail') res = await financeApi.failWithdraw(target.id, actionText || 'payment failed');
    if (res?.success) {
      Toast.success({ content: t('操作成功') });
      setTarget(null);
      setAction('');
      data.refresh();
    } else {
      Toast.error({ content: res?.message || t('操作失败') });
    }
  };

  const columns = [
    { title: t('提现单号'), dataIndex: 'withdraw_no', render: (value) => <Text copyable>{value || '-'}</Text> },
    { title: t('用户ID'), dataIndex: 'user_id' },
    { title: t('金额'), dataIndex: 'amount', render: formatFinanceMoney },
    { title: t('实际金额'), dataIndex: 'actual_amount', render: formatFinanceMoney },
    { title: t('收款账号'), dataIndex: 'receive_account' },
    { title: t('状态'), dataIndex: 'status', render: (value) => <StatusTag t={t} status={value} /> },
    { title: t('创建时间'), dataIndex: 'created_at', render: formatFinanceTimestamp },
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) => {
        const actions = getClassicWithdrawActions(record);
        if (record.provider === 'piggy_labor_v3') {
          return <Text type='tertiary'>{t('请使用新版后台处理')}</Text>;
        }
        return (
          <Space wrap>
            {actions.includes('approve') && <Button size='small' onClick={() => openAction(record, 'approve')}>{t('审批')}</Button>}
            {actions.includes('pay') && <Button size='small' onClick={() => openAction(record, 'pay')}>{t('标记打款')}</Button>}
            {actions.includes('fail') && <Button size='small' type='warning' onClick={() => openAction(record, 'fail')}>{t('标记失败')}</Button>}
            {actions.includes('reject') && <Button size='small' type='danger' theme='outline' onClick={() => openAction(record, 'reject')}>{t('驳回')}</Button>}
          </Space>
        );
      },
    },
  ];

  return (
    <>
      <PageCard
        title={t('提现订单')}
        description={t('审核、驳回和标记佣金提现订单')}
        actions={<Button icon={<RefreshCw size={14} />} onClick={data.refresh} loading={data.loading}>{t('刷新')}</Button>}
        filters={
          <FilterBar>
            <Input
              placeholder={t('用户ID')}
              value={userId}
              onChange={createFinanceFilterChangeHandler(data.setPage, setUserId)}
            />
            <Select
              value={status}
              onChange={createFinanceFilterChangeHandler(data.setPage, setStatus)}
              placeholder={t('状态')}
              showClear
            >
              {['pending', 'approved', 'paid', 'rejected', 'failed'].map((item) => (
                <Select.Option key={item} value={item}>{t(getFinanceStatusKey(item))}</Select.Option>
              ))}
            </Select>
          </FilterBar>
        }
      >
        <CardTable
          columns={columns}
          dataSource={data.items}
          rowKey='id'
          loading={data.loading}
          pagination={PaginationConfig(data.page, data.total, data.setPage)}
          empty={<Empty description={t('暂无提现订单')} />}
        />
      </PageCard>
      <Modal
        title={t(action === 'approve' ? '审批提现' : action === 'pay' ? '标记打款' : action === 'fail' ? '标记失败' : '驳回提现')}
        visible={Boolean(target)}
        onCancel={() => setTarget(null)}
        onOk={handleAction}
        centered
      >
        {action !== 'approve' && (
          <Input
            value={actionText}
            onChange={setActionText}
            placeholder={action === 'pay' ? t('打款凭证') : t('原因')}
          />
        )}
      </Modal>
    </>
  );
}

function CallbackLogsTable({ t }) {
  const [provider, setProvider] = useState('');
  const [tradeNo, setTradeNo] = useState('');
  const [processStatus, setProcessStatus] = useState('');
  const loadFn = useCallback(
    (page) =>
      financeApi.getCallbackLogs({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        provider,
        tradeNo,
        processStatus,
      }),
    [provider, tradeNo, processStatus],
  );
  const data = usePagedLoader(loadFn, [provider, tradeNo, processStatus]);
  const columns = [
    { title: t('支付渠道'), dataIndex: 'provider' },
    { title: t('事件类型'), dataIndex: 'event_type' },
    { title: t('订单号'), dataIndex: 'trade_no', render: (value) => <Text copyable>{value || '-'}</Text> },
    { title: t('业务类型'), dataIndex: 'biz_type' },
    { title: t('验签'), dataIndex: 'verify_status', render: (value) => (value ? t('通过') : t('失败')) },
    { title: t('处理状态'), dataIndex: 'process_status', render: (value) => <StatusTag t={t} status={value} /> },
    { title: t('Payload摘要'), dataIndex: 'payload_digest', render: (value) => <Text copyable>{value || '-'}</Text> },
    { title: t('错误信息'), dataIndex: 'error_message', render: (value) => value || '-' },
    { title: t('创建时间'), dataIndex: 'created_at', render: formatFinanceTimestamp },
  ];
  return (
    <PageCard
      title={t('支付回调日志')}
      description={t('查看支付渠道回调验签和处理结果')}
      actions={<Button icon={<RefreshCw size={14} />} onClick={data.refresh} loading={data.loading}>{t('刷新')}</Button>}
      filters={
        <FilterBar>
          <Input
            placeholder={t('支付渠道')}
            value={provider}
            onChange={createFinanceFilterChangeHandler(data.setPage, setProvider)}
          />
          <Input
            placeholder={t('订单号')}
            value={tradeNo}
            onChange={createFinanceFilterChangeHandler(data.setPage, setTradeNo)}
          />
          <Select
            value={processStatus}
            onChange={createFinanceFilterChangeHandler(data.setPage, setProcessStatus)}
            placeholder={t('处理状态')}
            showClear
          >
            {['success', 'failed', 'ignored'].map((item) => (
              <Select.Option key={item} value={item}>{t(getFinanceStatusKey(item))}</Select.Option>
            ))}
          </Select>
        </FilterBar>
      }
    >
      <CardTable
        columns={columns}
        dataSource={data.items}
        rowKey='id'
        loading={data.loading}
        pagination={PaginationConfig(data.page, data.total, data.setPage)}
        empty={<Empty description={t('暂无回调日志')} />}
      />
    </PageCard>
  );
}

function ReconciliationTasksTable({ t }) {
  const [provider, setProvider] = useState('');
  const [status, setStatus] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [diffs, setDiffs] = useState(null);
  const [form, setForm] = useState({
    provider: '',
    date_from: '',
    date_to: '',
    orders_json: '[]',
  });
  const loadFn = useCallback(
    (page) =>
      financeApi.getReconciliationTasks({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        provider,
        status,
      }),
    [provider, status],
  );
  const data = usePagedLoader(loadFn, [provider, status]);

  const handleCreate = async () => {
    let orders;
    try {
      orders = parseProviderOrders(form.orders_json);
    } catch (error) {
      Toast.error({ content: t('渠道订单JSON必须是数组') });
      return;
    }
    const res = await financeApi.createReconciliationTask({
      provider: form.provider.trim(),
      date_from: Number(form.date_from),
      date_to: Number(form.date_to),
      orders,
    });
    if (res?.success) {
      Toast.success({ content: t('操作成功') });
      setDiffs(res.data?.diffs || []);
      setCreateOpen(false);
      setForm({ provider: '', date_from: '', date_to: '', orders_json: '[]' });
      data.refresh();
    } else {
      Toast.error({ content: res?.message || t('操作失败') });
    }
  };

  const columns = [
    { title: t('支付渠道'), dataIndex: 'provider' },
    {
      title: t('日期范围'),
      dataIndex: 'date_from',
      render: (_, record) => (
        <div>
          <div>{formatFinanceTimestamp(record.date_from)}</div>
          <Text type='tertiary' size='small'>{formatFinanceTimestamp(record.date_to)}</Text>
        </div>
      ),
    },
    { title: t('总数'), dataIndex: 'total_count' },
    { title: t('差异数'), dataIndex: 'diff_count' },
    { title: t('状态'), dataIndex: 'status', render: (value) => <StatusTag t={t} status={value} /> },
    { title: t('创建时间'), dataIndex: 'created_at', render: formatFinanceTimestamp },
  ];

  const diffColumns = [
    { title: t('订单号'), dataIndex: 'trade_no', render: (value) => <Text copyable>{value || '-'}</Text> },
    { title: t('业务类型'), dataIndex: 'biz_type' },
    { title: t('差异类型'), dataIndex: 'diff_type', render: (value) => t(getPaymentDiffKey(value)) },
    { title: t('本地状态'), dataIndex: 'local_status' },
    { title: t('渠道状态'), dataIndex: 'provider_status' },
    { title: t('本地金额'), dataIndex: 'local_paid_amount', render: formatFinanceMoney },
    { title: t('渠道金额'), dataIndex: 'provider_paid_amount', render: formatFinanceMoney },
  ];

  return (
    <>
      <PageCard
        title={t('支付对账')}
        description={t('创建并查看本地支付对账任务')}
        actions={
          <>
            <Button icon={<Plus size={14} />} onClick={() => setCreateOpen(true)}>{t('创建对账任务')}</Button>
            <Button icon={<RefreshCw size={14} />} onClick={data.refresh} loading={data.loading}>{t('刷新')}</Button>
          </>
        }
        filters={
          <FilterBar>
            <Input
              placeholder={t('支付渠道')}
              value={provider}
              onChange={createFinanceFilterChangeHandler(data.setPage, setProvider)}
            />
            <Select
              value={status}
              onChange={createFinanceFilterChangeHandler(data.setPage, setStatus)}
              placeholder={t('状态')}
              showClear
            >
              {['pending', 'success', 'failed'].map((item) => (
                <Select.Option key={item} value={item}>{t(getFinanceStatusKey(item))}</Select.Option>
              ))}
            </Select>
          </FilterBar>
        }
      >
        <CardTable
          columns={columns}
          dataSource={data.items}
          rowKey='id'
          loading={data.loading}
          pagination={PaginationConfig(data.page, data.total, data.setPage)}
          empty={<Empty description={t('暂无对账任务')} />}
        />
      </PageCard>
      <Modal title={t('创建对账任务')} visible={createOpen} onCancel={() => setCreateOpen(false)} onOk={handleCreate} width={760} centered>
        <div className='space-y-3'>
          <Input placeholder={t('支付渠道')} value={form.provider} onChange={(value) => setForm((current) => ({ ...current, provider: value }))} />
          <Input placeholder={t('开始时间戳')} value={form.date_from} onChange={(value) => setForm((current) => ({ ...current, date_from: value }))} />
          <Input placeholder={t('结束时间戳')} value={form.date_to} onChange={(value) => setForm((current) => ({ ...current, date_to: value }))} />
          <Input.TextArea
            rows={8}
            placeholder={t('渠道订单JSON数组')}
            value={form.orders_json}
            onChange={(value) => setForm((current) => ({ ...current, orders_json: value }))}
          />
        </div>
      </Modal>
      <Modal title={t('对账差异')} visible={Array.isArray(diffs)} onCancel={() => setDiffs(null)} footer={null} width={900} centered>
        <CardTable columns={diffColumns} dataSource={diffs || []} rowKey='trade_no' pagination={false} empty={<Empty description={t('暂无对账差异')} />} />
      </Modal>
    </>
  );
}

function ReverseOrderModal({ t, visible, onCancel }) {
  const [type, setType] = useState('topup');
  const [tradeNo, setTradeNo] = useState('');
  const [provider, setProvider] = useState('');
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    if (!tradeNo || !provider || !reason) {
      Toast.error({ content: t('请填写完整冲正信息') });
      return;
    }
    setLoading(true);
    try {
      const res =
        type === 'topup'
          ? await financeApi.reverseTopupOrder(tradeNo, provider, reason)
          : await financeApi.reverseVipActivationOrder(tradeNo, provider, reason);
      if (res?.success) {
        Toast.success({ content: t('操作成功') });
        setTradeNo('');
        setProvider('');
        setReason('');
        onCancel();
      } else {
        Toast.error({ content: res?.message || t('操作失败') });
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={t('支付订单冲正')}
      visible={visible}
      onCancel={onCancel}
      onOk={handleSubmit}
      okButtonProps={{ loading }}
      centered
    >
      <div className='space-y-3'>
        <Select value={type} onChange={setType}>
          <Select.Option value='topup'>{t('充值订单')}</Select.Option>
          <Select.Option value='vip_activation'>{t('VVIP开通订单')}</Select.Option>
        </Select>
        <Input placeholder={t('订单号')} value={tradeNo} onChange={setTradeNo} />
        <Input placeholder={t('支付渠道')} value={provider} onChange={setProvider} />
        <Input placeholder={t('冲正原因')} value={reason} onChange={setReason} />
        <Text type='tertiary' size='small'>
          {t('冲正会影响余额、佣金和VVIP状态，请确认订单和原因后再继续。')}
        </Text>
      </div>
    </Modal>
  );
}

const FinanceTable = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const [reverseOpen, setReverseOpen] = useState(false);

  const activeTab = useMemo(() => {
    const tab = new URLSearchParams(location.search).get('tab');
    return tab || 'vip';
  }, [location.search]);

  const handleTabChange = (tab) => {
    navigate(`/console/finance?tab=${tab}`, { replace: true });
  };

  return (
    <div className='w-full max-w-7xl mx-auto mt-[60px] px-2 pb-8'>
      <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-4'>
        <div>
          <Typography.Title heading={3} className='!mb-1'>
            {t('财务运营')}
          </Typography.Title>
          <Text type='tertiary'>
            {t('管理VVIP开通、邀请关系、佣金、提现和支付审计')}
          </Text>
        </div>
        <Button
          icon={<RotateCcw size={15} />}
          onClick={() => setReverseOpen(true)}
        >
          {t('订单冲正')}
        </Button>
      </div>

      <Tabs type='line' activeKey={activeTab} onChange={handleTabChange}>
        <Tabs.TabPane itemKey='vip' tab={t('VVIP开通')}>
          <VipRecordsTable t={t} />
        </Tabs.TabPane>
        <Tabs.TabPane itemKey='relations' tab={t('邀请关系')}>
          <RelationsTable t={t} />
        </Tabs.TabPane>
        <Tabs.TabPane itemKey='commissions' tab={t('佣金')}>
          <CommissionRecordsTable t={t} />
        </Tabs.TabPane>
        <Tabs.TabPane itemKey='withdraws' tab={t('提现')}>
          <WithdrawOrdersTable t={t} />
        </Tabs.TabPane>
        <Tabs.TabPane itemKey='callbacks' tab={t('回调日志')}>
          <CallbackLogsTable t={t} />
        </Tabs.TabPane>
        <Tabs.TabPane itemKey='reconciliation' tab={t('支付对账')}>
          <ReconciliationTasksTable t={t} />
        </Tabs.TabPane>
      </Tabs>

      <ReverseOrderModal
        t={t}
        visible={reverseOpen}
        onCancel={() => setReverseOpen(false)}
      />
    </div>
  );
};

export default FinanceTable;
