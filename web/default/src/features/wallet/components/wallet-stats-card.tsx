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
import type { ReactNode } from 'react'
import { Activity, BarChart3, HandCoins, Snowflake, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '../lib/format'
import type { UserWalletData, WalletAccount } from '../types'

interface WalletStatsCardProps {
  user: UserWalletData | null
  account?: WalletAccount | null
  loading?: boolean
  onTransfer?: () => void
  complianceConfirmed?: boolean
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='overflow-hidden rounded-lg border'>
        <div className='divide-border/60 grid grid-cols-3 divide-x'>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className='px-3 py-3 sm:px-5 sm:py-4'>
              <Skeleton className='h-3.5 w-20' />
              <Skeleton className='mt-2 h-7 w-28' />
              <Skeleton className='mt-1.5 h-3.5 w-24' />
            </div>
          ))}
        </div>
      </div>
    )
  }

  const complianceConfirmed = props.complianceConfirmed ?? true
  const commissionAmount = props.account?.commission_amount ?? 0
  const showTransferAction =
    typeof props.onTransfer === 'function' && commissionAmount > 0

  const stats: Array<{
    label: string
    value: string
    description: string
    icon: typeof WalletCards
    action?: ReactNode
  }> = [
    {
      label: t('Consumable Balance'),
      value: formatCurrency(props.account?.balance_amount ?? 0),
      description: t('Wallet balance available for API usage'),
      icon: WalletCards,
    },
    {
      label: t('Withdrawable Commission'),
      value: formatCurrency(commissionAmount),
      description: t('Commission available for withdrawal or transfer'),
      icon: HandCoins,
      action: showTransferAction ? (
        <Button
          onClick={props.onTransfer}
          disabled={!complianceConfirmed}
          className='h-8 px-3'
          size='sm'
        >
          {t('Transfer to Balance')}
        </Button>
      ) : null,
    },
    {
      label: t('Frozen Commission'),
      value: formatCurrency(props.account?.frozen_commission_amount ?? 0),
      description: t('Commission locked by pending withdrawals'),
      icon: Snowflake,
    },
    {
      label: t('Total Usage'),
      value: formatQuota(props.user?.used_quota ?? 0),
      description: t('Total consumed quota'),
      icon: BarChart3,
    },
    {
      label: t('API Requests'),
      value: (props.user?.request_count ?? 0).toLocaleString(),
      description: t('Total requests made'),
      icon: Activity,
    },
  ]

  return (
    <div className='overflow-hidden rounded-lg border bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50 border-purple-100'>
      <div className='divide-border/60 grid grid-cols-2 divide-x divide-y sm:grid-cols-3 xl:grid-cols-5 xl:divide-y-0'>
        {stats.map((item) => (
          <div key={item.label} className='px-3 py-3 sm:px-5 sm:py-4'>
            <div className='flex items-center gap-2'>
              <item.icon className='text-muted-foreground/60 size-3.5 shrink-0' />
              <div className='text-muted-foreground truncate text-xs font-medium tracking-wider uppercase'>
                {item.label}
              </div>
            </div>

            <div className='text-foreground mt-1.5 font-mono text-base font-bold tracking-tight break-all tabular-nums sm:mt-2 sm:text-2xl'>
              {item.value}
            </div>
            <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
              {item.description}
            </div>
            {item.action ? <div className='mt-2'>{item.action}</div> : null}
          </div>
        ))}
      </div>
    </div>
  )
}
