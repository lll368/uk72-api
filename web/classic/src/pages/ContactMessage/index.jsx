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
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Select,
  Space,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Edit3, RefreshCw, Trash2 } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';
import CardTable from '../../components/common/ui/CardTable';

const { Text } = Typography;

const CONTACT_MESSAGE_PAGE_SIZE = 10;

const CONTACT_MESSAGE_STATUS_OPTIONS = [
  { value: 'pending', labelKey: '待联系', color: 'orange' },
  { value: 'contacted', labelKey: '已联系', color: 'green' },
  { value: 'unreachable', labelKey: '联系不上', color: 'red' },
];

const getStatusOption = (status) =>
  CONTACT_MESSAGE_STATUS_OPTIONS.find((item) => item.value === status) ||
  CONTACT_MESSAGE_STATUS_OPTIONS[0];

const formatTimestamp = (timestamp) => {
  if (!timestamp) return '-';
  const time = Number(timestamp);
  if (!Number.isFinite(time) || time <= 0) return '-';
  return new Date(time * 1000).toLocaleString();
};

const ContactMessage = () => {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');
  const [loading, setLoading] = useState(false);
  const [editRecord, setEditRecord] = useState(null);
  const [editForm, setEditForm] = useState({
    status: 'pending',
    remark: '',
  });
  const [saving, setSaving] = useState(false);

  const loadMessages = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/contact/admin/messages', {
        params: {
          p: page,
          page_size: CONTACT_MESSAGE_PAGE_SIZE,
          status: status || undefined,
        },
      });
      const { success, message, data } = res.data;
      if (success) {
        setItems(data?.items || []);
        setTotal(data?.total || 0);
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(error?.response?.data?.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  }, [page, status, t]);

  useEffect(() => {
    loadMessages();
  }, [loadMessages]);

  const openEditModal = (record) => {
    setEditRecord(record);
    setEditForm({
      status: record.status || 'pending',
      remark: record.remark || '',
    });
  };

  const saveContactMessage = async () => {
    if (!editRecord) return;
    setSaving(true);
    try {
      const res = await API.put(
        `/api/contact/admin/messages/${editRecord.id}`,
        editForm,
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('留言已更新'));
        setEditRecord(null);
        await loadMessages();
      } else {
        showError(message || t('操作失败'));
      }
    } catch (error) {
      showError(error?.response?.data?.message || t('操作失败'));
    } finally {
      setSaving(false);
    }
  };

  const deleteContactMessage = (record) => {
    Modal.confirm({
      title: t('删除留言'),
      content: t('确定删除这条留言吗？此操作不可恢复。'),
      okText: t('确认删除'),
      cancelText: t('取消'),
      okButtonProps: { type: 'danger' },
      onOk: async () => {
        try {
          const res = await API.delete(
            `/api/contact/admin/messages/${record.id}`,
          );
          const { success, message } = res.data;
          if (success) {
            showSuccess(t('留言已删除'));
            await loadMessages();
          } else {
            showError(message || t('操作失败'));
          }
        } catch (error) {
          showError(error?.response?.data?.message || t('操作失败'));
        }
      },
    });
  };

  const columns = useMemo(
    () => [
      {
        title: t('姓名'),
        dataIndex: 'name',
        render: (value) => value || '-',
      },
      {
        title: t('电话'),
        dataIndex: 'phone',
        render: (value) => (value ? <Text copyable>{value}</Text> : '-'),
      },
      {
        title: t('留言'),
        dataIndex: 'message',
        render: (value) => (
          <Text style={{ whiteSpace: 'pre-wrap' }}>{value || '-'}</Text>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (value) => {
          const option = getStatusOption(value);
          return <Tag color={option.color}>{t(option.labelKey)}</Tag>;
        },
      },
      {
        title: t('提交时间'),
        dataIndex: 'created_at',
        render: formatTimestamp,
      },
      {
        title: t('备注'),
        dataIndex: 'remark',
        render: (value) => (
          <Text style={{ whiteSpace: 'pre-wrap' }}>{value || '-'}</Text>
        ),
      },
      {
        title: t('处理时间'),
        dataIndex: 'processed_at',
        render: formatTimestamp,
      },
      {
        title: t('处理人'),
        dataIndex: 'processed_by',
        render: (value) => value || '-',
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        fixed: 'right',
        render: (_, record) => (
          <Space>
            <Button
              size='small'
              icon={<Edit3 size={14} />}
              onClick={() => openEditModal(record)}
            >
              {t('编辑')}
            </Button>
            <Button
              size='small'
              type='danger'
              theme='outline'
              icon={<Trash2 size={14} />}
              onClick={() => deleteContactMessage(record)}
            >
              {t('删除')}
            </Button>
          </Space>
        ),
      },
    ],
    [t],
  );

  return (
    <div className='mt-[60px] px-2'>
      <Card className='!rounded-lg shadow-sm border-0'>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between mb-4'>
          <div>
            <Typography.Title heading={5} className='!mb-1'>
              {t('留言管理')}
            </Typography.Title>
            <Text type='tertiary' size='small'>
              {t('查看并处理首页访客留言')}
            </Text>
          </div>
          <Space wrap>
            <Select
              value={status}
              placeholder={t('全部状态')}
              style={{ width: 160 }}
              onChange={(value) => {
                setStatus(value || '');
                setPage(1);
              }}
              showClear
            >
              {CONTACT_MESSAGE_STATUS_OPTIONS.map((option) => (
                <Select.Option key={option.value} value={option.value}>
                  {t(option.labelKey)}
                </Select.Option>
              ))}
            </Select>
            <Button
              icon={<RefreshCw size={14} />}
              loading={loading}
              onClick={loadMessages}
            >
              {t('刷新')}
            </Button>
          </Space>
        </div>
        <CardTable
          columns={columns}
          dataSource={items}
          rowKey='id'
          loading={loading}
          pagination={{
            currentPage: page,
            pageSize: CONTACT_MESSAGE_PAGE_SIZE,
            total,
            onPageChange: setPage,
          }}
          empty={<Empty description={t('暂无留言记录')} />}
        />
      </Card>

      <Modal
        title={t('修改留言')}
        visible={Boolean(editRecord)}
        okText={t('保存')}
        cancelText={t('取消')}
        confirmLoading={saving}
        onOk={saveContactMessage}
        onCancel={() => setEditRecord(null)}
        centered
      >
        <div className='flex flex-col gap-3'>
          <Text type='tertiary' size='small'>
            {t('更新留言状态和管理员备注')}
          </Text>
          <div>
            <div className='mb-1 text-sm font-medium text-semi-color-text-0'>
              {t('状态')}
            </div>
            <Select
              value={editForm.status}
              style={{ width: '100%' }}
              onChange={(value) =>
                setEditForm((current) => ({ ...current, status: value }))
              }
            >
              {CONTACT_MESSAGE_STATUS_OPTIONS.map((option) => (
                <Select.Option key={option.value} value={option.value}>
                  {t(option.labelKey)}
                </Select.Option>
              ))}
            </Select>
          </div>
          <div>
            <div className='mb-1 text-sm font-medium text-semi-color-text-0'>
              {t('管理员备注')}
            </div>
            <TextArea
              rows={4}
              maxLength={500}
              value={editForm.remark}
              placeholder={t('请输入管理员备注')}
              onChange={(value) =>
                setEditForm((current) => ({ ...current, remark: value }))
              }
            />
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default ContactMessage;
