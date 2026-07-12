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

import React, { useEffect, useState } from 'react';
import { Input, InputNumber, Modal, Typography } from '@douyinfe/semi-ui';
import { HandCoins } from 'lucide-react';

const WithdrawModal = ({
  t,
  visible,
  onCancel,
  onSubmit,
  availableAmount,
  minAmount,
  loading,
  renderAmount,
}) => {
  const [amount, setAmount] = useState(0);
  const [receiveType, setReceiveType] = useState('bank');
  const [receiveAccount, setReceiveAccount] = useState('');
  const [remark, setRemark] = useState('');

  useEffect(() => {
    if (!visible) return;
    setAmount(Math.max(minAmount || 0, 1));
    setReceiveType('bank');
    setReceiveAccount('');
    setRemark('');
  }, [minAmount, visible]);

  const invalid =
    amount <= 0 ||
    amount > availableAmount ||
    (minAmount > 0 && amount < minAmount) ||
    receiveAccount.trim() === '';

  return (
    <Modal
      title={
        <div className='flex items-center'>
          <HandCoins className='mr-2' size={18} />
          {t('申请提现')}
        </div>
      }
      visible={visible}
      onCancel={onCancel}
      onOk={() =>
        onSubmit({
          amount,
          receive_type: receiveType.trim(),
          receive_account: receiveAccount.trim(),
          remark: remark.trim(),
        })
      }
      okButtonProps={{ loading, disabled: invalid }}
      maskClosable={false}
      centered
    >
      <div className='space-y-4'>
        <div className='grid grid-cols-2 gap-3'>
          <div>
            <Typography.Text type='tertiary'>{t('可提现')}</Typography.Text>
            <div className='font-medium'>{renderAmount(availableAmount)}</div>
          </div>
          <div>
            <Typography.Text type='tertiary'>{t('最低提现')}</Typography.Text>
            <div className='font-medium'>{renderAmount(minAmount)}</div>
          </div>
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('提现金额')}
          </Typography.Text>
          <InputNumber
            className='w-full'
            min={minAmount || 0.01}
            max={availableAmount}
            value={amount}
            onChange={setAmount}
          />
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('收款类型')}
          </Typography.Text>
          <Input value={receiveType} onChange={setReceiveType} />
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('收款账号')}
          </Typography.Text>
          <Input value={receiveAccount} onChange={setReceiveAccount} />
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('备注')}
          </Typography.Text>
          <Input.TextArea value={remark} onChange={setRemark} autosize />
        </div>
      </div>
    </Modal>
  );
};

export default WithdrawModal;
