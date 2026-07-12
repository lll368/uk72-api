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
import { useTranslation } from 'react-i18next'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { TopUpRecordsTable } from './components/top-up-records-table'
import { VipActivationRecordsTable } from './components/vip-activation-records-table'

export function AdminRechargeRecords() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('topups')

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('admin_recharge_records_title')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList>
            <TabsTrigger value='topups'>{t('Ordinary Top-ups')}</TabsTrigger>
            <TabsTrigger value='vip'>
              {t('Compute Partner Activations')}
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Tabs value={activeTab} onValueChange={setActiveTab} className='w-full'>
          <TabsContent value='topups' className='mt-0'>
            <TopUpRecordsTable />
          </TabsContent>
          <TabsContent value='vip' className='mt-0'>
            <VipActivationRecordsTable
              title={t('Compute Partner Activations')}
              description={t('Compute partner activation payments')}
              enableFilters
              showUserContactColumns
              showActions={false}
            />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
