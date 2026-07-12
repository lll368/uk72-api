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
import { Crown, CreditCard, Link2, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { CopyButton } from '@/components/copy-button'
import {
  formatCurrency,
  getPaymentIcon,
  getPaymentMethodDisplayName,
} from '../lib'
import type { PaymentMethod, VipActivationInfo } from '../types'

interface VipActivationCardProps {
  vipInfo: VipActivationInfo | null
  loading?: boolean
  processing?: string | null
  onPay: (method: PaymentMethod) => void
}

export function VipActivationCard({
  vipInfo,
  loading,
  processing,
  onPay,
}: VipActivationCardProps) {
  const { t } = useTranslation()

  if (loading) {
    return (
      <TitledCard
        title={t('VVIP Activation')}
        description={t('One-time paid activation for invitation privileges')}
        icon={<Crown className='size-4' />}
      >
        <div className='flex flex-col gap-3'>
          <Skeleton className='h-8 w-40' />
          <Skeleton className='h-10 w-full' />
        </div>
      </TitledCard>
    )
  }

  const paymentMethods = vipInfo?.payment_methods ?? []
  const isActive = vipInfo?.is_vvip === true
  const inviteLink = vipInfo?.invite_link ?? ''

  return (
    <TitledCard
      title={
        isActive
          ? t('Compute Partners Activated')
          : t('VVIP Activation')
      }
      description={
        isActive
          ? undefined
          : t('One-time paid activation for invitation privileges')
      }
      icon={<Crown className='size-4' />}
      className={
        isActive ? 'border-warning/40 bg-warning/5' : undefined
      }
      iconClassName={
        isActive ? 'bg-warning/15 text-warning' : undefined
      }
      titleClassName={isActive ? 'text-warning' : undefined}
      action={
        isActive ? (
          <Badge variant='secondary'>{t('Active')}</Badge>
        ) : (
          <Badge variant='outline'>
            {formatCurrency(vipInfo?.paid_amount ?? 1680)}
          </Badge>
        )
      }
      contentClassName='flex flex-col gap-4'
      gradient={!isActive}
    >
      {isActive ? (
        <>
          <div className='grid gap-2 sm:grid-cols-3'>
            <div>
              <div className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                {t('Status')}
              </div>
              <div className='mt-1 text-sm font-semibold'>{t('Active')}</div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                {t('Activated At')}
              </div>
              <div className='mt-1 text-sm font-semibold'>
                {vipInfo?.activated_at
                  ? formatTimestampToDate(vipInfo.activated_at)
                  : t('Unknown')}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                {t('Invite Code')}
              </div>
              <div className='mt-1 text-sm font-semibold'>
                {vipInfo?.aff_code || t('Unknown')}
              </div>
            </div>
          </div>
          {inviteLink ? (
            <div className='flex items-center gap-2'>
              <Input
                value={inviteLink}
                readOnly
                className='font-mono text-xs'
              />
              <CopyButton
                value={inviteLink}
                variant='outline'
                className='size-9 shrink-0'
                iconClassName='size-4'
                tooltip={t('Copy invite link')}
                aria-label={t('Copy invite link')}
              />
            </div>
          ) : null}
        </>
      ) : (
        <>
          <div className='flex flex-col gap-1'>
            <div className='text-2xl font-semibold'>
              {formatCurrency(vipInfo?.paid_amount ?? 1680)}
            </div>
            <div className='text-muted-foreground text-sm'>
              {t('Fixed price, no balance or quota deduction')}
            </div>
          </div>

          {paymentMethods.length > 0 ? (
            <div className='flex flex-wrap gap-2'>
              {paymentMethods.map((method) => {
                const icon = getPaymentIcon(method.type)
                const isProcessing = processing === method.type
                const displayName = getPaymentMethodDisplayName(method, t)
                return (
                  <Button
                    key={method.type}
                    variant='outline'
                    onClick={() => onPay(method)}
                    disabled={Boolean(processing)}
                  >
                    {isProcessing ? (
                      <Loader2
                        data-icon='inline-start'
                        className='animate-spin'
                      />
                    ) : icon ? (
                      <span data-icon='inline-start'>{icon}</span>
                    ) : (
                      <CreditCard data-icon='inline-start' />
                    )}
                    {displayName}
                  </Button>
                )
              })}
            </div>
          ) : (
            <Alert>
              <Link2 className='size-4' />
              <AlertDescription>
                {t('No VVIP payment method is currently available')}
              </AlertDescription>
            </Alert>
          )}
        </>
      )}
    </TitledCard>
  )
}
