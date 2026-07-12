/*
Copyright (C) 2023-2026 QuantumNous

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
import { useState } from 'react'
import { RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { CommissionRecordsTable } from './components/commission-records-table'
import { PaymentCallbackLogsTable } from './components/payment-callback-logs-table'
import { PiggyWithdrawCallbackLogsTable } from './components/piggy-withdraw-callback-logs-table'
import { QiniuBillingBucketsTable } from './components/qiniu-billing-buckets-table'
import { QiniuKeysTable } from './components/qiniu-keys-table'
import { ReconciliationTasksTable } from './components/reconciliation-tasks-table'
import { ReverseOrderDialog } from './components/reverse-order-dialog'
import { UserRelationsTable } from './components/user-relations-table'
import { VipActivationRecordsTable } from './components/vip-activation-records-table'
import { WithdrawOrdersTable } from './components/withdraw-orders-table'

export function AdminFinance() {
  const { t } = useTranslation()
  const [reverseOpen, setReverseOpen] = useState(false)

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Finance Operations')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t(
            'Manage VVIP activations, relations, commissions, withdrawals, and payment audits'
          )}
        </SectionPageLayout.Description>
        <SectionPageLayout.Actions>
          <Button
            size='sm'
            variant='outline'
            onClick={() => setReverseOpen(true)}
          >
            <RotateCcw data-icon='inline-start' />
            {t('Reverse Order')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <Tabs defaultValue='vip' className='w-full'>
            <div className='overflow-x-auto pb-2'>
              <TabsList>
                <TabsTrigger value='vip'>{t('VVIP Activations')}</TabsTrigger>
                <TabsTrigger value='relations'>{t('Relations')}</TabsTrigger>
                <TabsTrigger value='commissions'>
                  {t('Commissions')}
                </TabsTrigger>
                <TabsTrigger value='withdrawals'>
                  {t('Withdrawals')}
                </TabsTrigger>
                <TabsTrigger value='callbacks'>
                  {t('Callback Logs')}
                </TabsTrigger>
                <TabsTrigger value='withdraw-callbacks'>
                  {t('Withdrawal Callbacks')}
                </TabsTrigger>
                <TabsTrigger value='reconciliation'>
                  {t('Reconciliation')}
                </TabsTrigger>
                <TabsTrigger value='qiniu-buckets'>
                  {t('Qiniu Buckets')}
                </TabsTrigger>
                <TabsTrigger value='qiniu-keys'>{t('Qiniu Keys')}</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value='vip' className='mt-3'>
              <VipActivationRecordsTable />
            </TabsContent>
            <TabsContent value='relations' className='mt-3'>
              <UserRelationsTable />
            </TabsContent>
            <TabsContent value='commissions' className='mt-3'>
              <CommissionRecordsTable />
            </TabsContent>
            <TabsContent value='withdrawals' className='mt-3'>
              <WithdrawOrdersTable />
            </TabsContent>
            <TabsContent value='callbacks' className='mt-3'>
              <PaymentCallbackLogsTable />
            </TabsContent>
            <TabsContent value='withdraw-callbacks' className='mt-3'>
              <PiggyWithdrawCallbackLogsTable />
            </TabsContent>
            <TabsContent value='reconciliation' className='mt-3'>
              <ReconciliationTasksTable />
            </TabsContent>
            <TabsContent value='qiniu-buckets' className='mt-3'>
              <QiniuBillingBucketsTable />
            </TabsContent>
            <TabsContent value='qiniu-keys' className='mt-3'>
              <QiniuKeysTable />
            </TabsContent>
          </Tabs>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ReverseOrderDialog open={reverseOpen} onOpenChange={setReverseOpen} />
    </>
  )
}
