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
import { Ban, Check, ChevronLeft, ChevronRight, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  StatusBadge,
  type StatusBadgeProps,
} from '@/components/status-badge'
import { useVipActivationRecords } from '../../hooks/use-vip-activation-records'
import { formatTimestamp, getPaymentMethodName } from '../../lib/billing'
import type {
  VipActivationRecord,
  VipActivationStatus,
} from '../../types'

interface VipActivationRecordsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type VipStatusConfig = {
  label: string
  variant: StatusBadgeProps['variant']
}

const VIP_STATUS_CONFIG: Record<VipActivationStatus, VipStatusConfig> = {
  pending: { label: 'Pending', variant: 'warning' },
  success: { label: 'Active', variant: 'success' },
  failed: { label: 'Failed', variant: 'danger' },
  disabled: { label: 'Disabled', variant: 'neutral' },
}

function getVipStatusConfig(status: VipActivationStatus): VipStatusConfig {
  return VIP_STATUS_CONFIG[status] || VIP_STATUS_CONFIG.pending
}

function formatOptionalTimestamp(timestamp?: number): string {
  return timestamp && timestamp > 0 ? formatTimestamp(timestamp) : '-'
}

export function VipActivationRecordsDialog({
  open,
  onOpenChange,
}: VipActivationRecordsDialogProps) {
  const { t } = useTranslation()
  const {
    records,
    total,
    page,
    pageSize,
    loading,
    disabling,
    handlePageChange,
    handlePageSizeChange,
    handleDisableVipActivation,
  } = useVipActivationRecords()
  const [disableTarget, setDisableTarget] =
    useState<VipActivationRecord | null>(null)
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const handleConfirmDisable = async () => {
    if (!disableTarget) return
    const success = await handleDisableVipActivation(
      disableTarget.user_id,
      'admin disabled from VVIP records'
    )
    if (success) {
      setDisableTarget(null)
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:h-dvh max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-4xl'>
          <DialogHeader>
            <DialogTitle>{t('VVIP Activation Records')}</DialogTitle>
            <DialogDescription>
              {t('View VVIP activation payments and disable active VVIP users')}
            </DialogDescription>
          </DialogHeader>

          <div className='min-h-0 flex-1 space-y-3 sm:space-y-4'>
            <div className='flex items-center justify-end'>
              <Select
                items={[
                  { value: '10', label: t('10 / page') },
                  { value: '20', label: t('20 / page') },
                  { value: '50', label: t('50 / page') },
                  { value: '100', label: t('100 / page') },
                ]}
                value={pageSize.toString()}
                onValueChange={(value) =>
                  value !== null && handlePageSizeChange(parseInt(value))
                }
              >
                <SelectTrigger className='h-9 w-[92px] sm:w-32'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='10'>{t('10 / page')}</SelectItem>
                    <SelectItem value='20'>{t('20 / page')}</SelectItem>
                    <SelectItem value='50'>{t('50 / page')}</SelectItem>
                    <SelectItem value='100'>{t('100 / page')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>

            <ScrollArea className='h-[calc(100dvh-15rem)] pr-3 sm:h-[500px] sm:pr-4'>
              {loading ? (
                <div className='space-y-3'>
                  {Array.from({ length: 5 }).map((_, i) => (
                    <div key={i} className='rounded-lg border p-3 sm:p-4'>
                      <div className='flex items-start justify-between'>
                        <div className='flex-1 space-y-2'>
                          <Skeleton className='h-4 w-48' />
                          <Skeleton className='h-3 w-32' />
                        </div>
                        <Skeleton className='h-5 w-16' />
                      </div>
                      <div className='mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4'>
                        <Skeleton className='h-3 w-full' />
                        <Skeleton className='h-3 w-full' />
                        <Skeleton className='h-3 w-full' />
                        <Skeleton className='h-3 w-full' />
                      </div>
                    </div>
                  ))}
                </div>
              ) : records.length === 0 ? (
                <div className='text-muted-foreground flex h-[320px] flex-col items-center justify-center text-center sm:h-[400px]'>
                  <p className='text-sm font-medium'>
                    {t('No VVIP records found')}
                  </p>
                </div>
              ) : (
                <div className='space-y-3'>
                  {records.map((record) => {
                    const statusConfig = getVipStatusConfig(record.status)
                    return (
                      <div
                        key={record.id}
                        className='hover:bg-muted/50 rounded-lg border p-3 transition-colors sm:p-4'
                      >
                        <div className='flex items-start justify-between gap-2'>
                          <div className='flex-1 space-y-1'>
                            <div className='flex min-w-0 items-center gap-2'>
                              <code className='text-foreground truncate font-mono text-sm'>
                                {record.trade_no}
                              </code>
                              <Button
                                variant='ghost'
                                size='sm'
                                className='h-5 w-5 p-0'
                                onClick={() => copyToClipboard(record.trade_no)}
                              >
                                {copiedText === record.trade_no ? (
                                  <Check className='h-3 w-3' />
                                ) : (
                                  <Copy className='h-3 w-3' />
                                )}
                              </Button>
                              <StatusBadge
                                label={`${t('User ID')}: ${record.user_id}`}
                                variant='neutral'
                                size='sm'
                                copyText={String(record.user_id)}
                              />
                            </div>
                            <div className='text-muted-foreground text-xs'>
                              {formatTimestamp(record.created_at)}
                            </div>
                          </div>
                          <StatusBadge
                            label={t(statusConfig.label)}
                            variant={statusConfig.variant}
                            showDot
                            copyable={false}
                          />
                        </div>

                        <div className='mt-3 grid grid-cols-2 gap-3 sm:mt-4 sm:grid-cols-4 sm:gap-4'>
                          <div className='space-y-1'>
                            <Label className='text-muted-foreground text-xs'>
                              {t('Provider')}
                            </Label>
                            <div className='text-sm font-medium'>
                              {record.payment_provider || '-'}
                            </div>
                          </div>
                          <div className='space-y-1'>
                            <Label className='text-muted-foreground text-xs'>
                              {t('Payment Method')}
                            </Label>
                            <div className='text-sm font-medium'>
                              {record.payment_method
                                ? t(getPaymentMethodName(record.payment_method))
                                : '-'}
                            </div>
                          </div>
                          <div className='space-y-1'>
                            <Label className='text-muted-foreground text-xs'>
                              {t('Activation Amount')}
                            </Label>
                            <div className='text-sm font-semibold'>
                              {formatCurrencyFromUSD(
                                record.activation_amount,
                                {
                                  digitsLarge: 2,
                                  digitsSmall: 2,
                                  abbreviate: false,
                                }
                              )}
                            </div>
                          </div>
                          <div className='space-y-1'>
                            <Label className='text-muted-foreground text-xs'>
                              {t('Paid Amount')}
                            </Label>
                            <div className='text-sm font-semibold'>
                              {formatCurrencyFromUSD(record.paid_amount, {
                                digitsLarge: 2,
                                digitsSmall: 2,
                                abbreviate: false,
                              })}
                            </div>
                          </div>
                          <div className='space-y-1'>
                            <Label className='text-muted-foreground text-xs'>
                              {t('Activated At')}
                            </Label>
                            <div className='text-sm font-medium'>
                              {formatOptionalTimestamp(record.activated_at)}
                            </div>
                          </div>
                          <div className='space-y-1'>
                            <Label className='text-muted-foreground text-xs'>
                              {t('Disabled At')}
                            </Label>
                            <div className='text-sm font-medium'>
                              {formatOptionalTimestamp(record.disabled_at)}
                            </div>
                          </div>
                        </div>

                        {record.status === 'success' && (
                          <div className='mt-4 flex justify-end'>
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => setDisableTarget(record)}
                              disabled={disabling}
                            >
                              <Ban data-icon='inline-start' />
                              {t('Disable VVIP')}
                            </Button>
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              )}
            </ScrollArea>

            {!loading && records.length > 0 && (
              <div className='flex flex-col items-center gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
                <div className='text-muted-foreground text-xs sm:text-sm'>
                  {t('Showing')} {(page - 1) * pageSize + 1}-
                  {Math.min(page * pageSize, total)} {t('of')} {total}
                </div>
                <div className='flex items-center gap-2'>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => handlePageChange(page - 1)}
                    disabled={page <= 1}
                    className='h-8 w-8 p-0'
                  >
                    <ChevronLeft className='h-4 w-4' />
                  </Button>
                  <div className='text-muted-foreground flex items-center gap-1 text-sm'>
                    <span className='font-medium'>{page}</span>
                    <span>/</span>
                    <span>{totalPages}</span>
                  </div>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => handlePageChange(page + 1)}
                    disabled={page >= totalPages}
                    className='h-8 w-8 p-0'
                  >
                    <ChevronRight className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={!!disableTarget}
        onOpenChange={(nextOpen) => !nextOpen && setDisableTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Disable VVIP')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Are you sure you want to disable this user VVIP? Historical activation records will be retained.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={disabling}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmDisable}
              disabled={disabling}
            >
              {disabling ? t('Processing...') : t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
