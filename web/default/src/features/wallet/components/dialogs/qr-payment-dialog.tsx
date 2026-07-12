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
import { CheckCircle2, Loader2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { CopyButton } from '@/components/copy-button'
import type { QrPaymentResult, QrPaymentStatus } from '../../types'

interface QrPaymentDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  payment: QrPaymentResult | null
  status?: QrPaymentStatus
  refreshing?: boolean
  onRefresh?: () => void
  onOpenBilling?: () => void
  onSuccessClose?: () => Promise<void> | void
  successClosing?: boolean
}

export function getQrPaymentDialogTitleKey(
  payment: Pick<QrPaymentResult, 'purpose'> | null
): string {
  return payment?.purpose === 'vvip_activation'
    ? 'WeChat Pay VVIP activation'
    : 'WeChat Pay top-up'
}

export function getQrPaymentDialogHelpTextKey(
  payment: Pick<QrPaymentResult, 'purpose'> | null
): string {
  return payment?.purpose === 'vvip_activation'
    ? 'After completing payment in WeChat, refresh compute partners to check the activation result.'
    : 'After completing payment in WeChat, refresh wallet or open order history to check the result.'
}

export function getQrPaymentStatusLabelKey(
  status: QrPaymentStatus,
  payment: Pick<QrPaymentResult, 'purpose'> | null
): string {
  if (status === 'success') {
    return payment?.purpose === 'vvip_activation'
      ? 'Activation successful'
      : 'Top-up successful'
  }
  if (status === 'failed') {
    return 'Payment failed'
  }
  if (status === 'expired') {
    return 'Payment expired'
  }
  return 'Payment pending'
}

export function getQrPaymentStatusHelpTextKey(
  status: QrPaymentStatus,
  payment: Pick<QrPaymentResult, 'purpose'> | null
): string {
  if (status === 'success') {
    return payment?.purpose === 'vvip_activation'
      ? 'Activation successful. Refresh compute partners to view the result.'
      : 'Top-up successful. Balance has been refreshed.'
  }
  if (status === 'failed') {
    return 'Payment failed. Open order history to view the result.'
  }
  if (status === 'expired') {
    return 'Payment expired. Please create a new order if needed.'
  }
  return getQrPaymentDialogHelpTextKey(payment)
}

export function shouldShowQrPaymentCode(status: QrPaymentStatus): boolean {
  return status !== 'success'
}

export function shouldShowQrPaymentRefresh(status: QrPaymentStatus): boolean {
  return status !== 'success'
}

export function getQrPaymentSuccessCloseLabelKey(): string {
  return 'Close dialog'
}

function getQrPaymentStatusClassName(status: QrPaymentStatus): string {
  if (status === 'success') {
    return 'text-success'
  }
  if (status === 'failed' || status === 'expired') {
    return 'text-destructive'
  }
  return ''
}

export function formatQrPaymentExpiresAt(
  expiresAt: QrPaymentResult['expires_at']
): string {
  if (expiresAt === undefined || expiresAt === null || expiresAt === '') {
    return ''
  }

  const numeric =
    typeof expiresAt === 'number' ? expiresAt : Number.parseInt(expiresAt, 10)
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return ''
  }

  const millis = numeric > 10_000_000_000 ? numeric : numeric * 1000
  return new Date(millis).toLocaleString()
}

export function QrPaymentDialog({
  open,
  onOpenChange,
  payment,
  status = 'pending',
  refreshing = false,
  onRefresh,
  onOpenBilling,
  onSuccessClose,
  successClosing = false,
}: QrPaymentDialogProps) {
  const { t } = useTranslation()
  const expiresAt = formatQrPaymentExpiresAt(payment?.expires_at)
  const isSuccess = status === 'success'
  const showQrCode = shouldShowQrPaymentCode(status) && !!payment?.code_url
  const showRefresh = shouldShowQrPaymentRefresh(status) && !!onRefresh
  const statusLabelKey =
    refreshing && status === 'pending'
      ? 'Checking payment status...'
      : getQrPaymentStatusLabelKey(status, payment)
  const handleSuccessClose = () => {
    if (successClosing) return
    if (onSuccessClose) {
      void onSuccessClose()
      return
    }
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t(getQrPaymentDialogTitleKey(payment))}</DialogTitle>
          <DialogDescription>
            {t(
              isSuccess
                ? getQrPaymentStatusHelpTextKey(status, payment)
                : 'Scan with WeChat to pay'
            )}
          </DialogDescription>
        </DialogHeader>

        {isSuccess ? (
          <div className='border-success/20 bg-success/5 flex min-h-[300px] flex-col items-center justify-center gap-4 rounded-lg border px-6 py-10 text-center'>
            <div className='bg-success/10 text-success flex size-16 items-center justify-center rounded-full'>
              <CheckCircle2 className='size-9' />
            </div>
            <div className='space-y-2'>
              <h3 className='text-foreground text-2xl font-semibold leading-tight'>
                {t(getQrPaymentStatusLabelKey(status, payment))}
              </h3>
              <p className='text-muted-foreground mx-auto max-w-xs text-sm leading-6'>
                {t(getQrPaymentStatusHelpTextKey(status, payment))}
              </p>
            </div>
          </div>
        ) : showQrCode ? (
          <div className='flex flex-col items-center gap-4'>
            <div
              className='rounded-lg border bg-white p-4'
              data-testid='qr-payment-code'
            >
              <QRCodeSVG value={payment?.code_url ?? ''} size={220} />
            </div>

            <div className='bg-muted/40 w-full space-y-2 rounded-lg border p-3 text-xs'>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-muted-foreground'>{t('Status')}</span>
                <span
                  className={cn(
                    'font-medium',
                    getQrPaymentStatusClassName(status)
                  )}
                >
                  {t(statusLabelKey)}
                </span>
              </div>
              {(payment.trade_no || payment.order_id) && (
                <div className='flex items-center justify-between gap-3'>
                  <span className='text-muted-foreground'>
                    {t('Order number')}
                  </span>
                  <span className='truncate font-mono'>
                    {payment.trade_no || payment.order_id}
                  </span>
                </div>
              )}
              {expiresAt && (
                <div className='flex items-center justify-between gap-3'>
                  <span className='text-muted-foreground'>
                    {t('Expires at')}
                  </span>
                  <span className='truncate'>{expiresAt}</span>
                </div>
              )}
            </div>

            <p className='text-muted-foreground text-center text-xs'>
              {t(getQrPaymentStatusHelpTextKey(status, payment))}
            </p>
          </div>
        ) : null}

        <DialogFooter className='grid grid-cols-1 gap-2 sm:grid-flow-col sm:auto-cols-fr sm:justify-stretch'>
          {isSuccess ? (
            <>
              {onOpenBilling ? (
                <Button
                  type='button'
                  variant='outline'
                  onClick={onOpenBilling}
                  disabled={successClosing}
                >
                  {t('Order History')}
                </Button>
              ) : null}
              <Button
                type='button'
                onClick={handleSuccessClose}
                disabled={successClosing}
              >
                {successClosing ? (
                  <>
                    <Loader2 data-icon='inline-start' className='animate-spin' />
                    {t('Refreshing...')}
                  </>
                ) : (
                  t(getQrPaymentSuccessCloseLabelKey())
                )}
              </Button>
            </>
          ) : (
            <>
              {showQrCode && payment?.code_url ? (
                <CopyButton
                  value={payment.code_url}
                  variant='outline'
                  size='default'
                  className='w-full'
                  tooltip={t('Copy payment link')}
                  aria-label={t('Copy payment link')}
                >
                  {t('Copy link')}
                </CopyButton>
              ) : null}
              {onOpenBilling ? (
                <Button
                  type='button'
                  variant='outline'
                  onClick={onOpenBilling}
                  className='w-full'
                >
                  {t('Order History')}
                </Button>
              ) : null}
              {showRefresh ? (
                <Button
                  type='button'
                  onClick={() => onRefresh?.()}
                  disabled={refreshing}
                  className='w-full'
                >
                  {t(refreshing ? 'Checking payment status...' : 'Refresh')}
                </Button>
              ) : null}
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
