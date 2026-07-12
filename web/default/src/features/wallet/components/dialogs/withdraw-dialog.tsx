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
import { useEffect, useState } from 'react'
import { Calculator, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { formatCurrency } from '../../lib/format'
import type {
  PiggyTaxTrialResult,
  PiggyWithdrawSubmitRequest,
  WithdrawalEligibility,
} from '../../types'

interface WithdrawDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  availableAmount: number
  minAmount: number
  submitting: boolean
  eligibility?: WithdrawalEligibility | null
  taxTrial?: PiggyTaxTrialResult | null
  taxTrialLoading?: boolean
  taxTrialError?: string
  onTaxTrial?: (amount: number) => Promise<PiggyTaxTrialResult | null>
  onTaxTrialReset?: () => void
  onSubmit: (request: PiggyWithdrawSubmitRequest) => Promise<boolean>
}

interface WithdrawSubmissionFieldsProps {
  availableAmount: number
  minAmount: number
  eligibility?: WithdrawalEligibility | null
  amount: number
  remark: string
  taxTrial?: PiggyTaxTrialResult | null
  taxTrialLoading?: boolean
  taxTrialError?: string
  taxTrialActionDisabled?: boolean
  onTaxTrialClick?: () => void
  onAmountChange: (amount: number) => void
  onRemarkChange: (remark: string) => void
}

interface CanRequestWithdrawTaxTrialInput {
  open: boolean
  hasTaxTrialHandler: boolean
  eligibility?: WithdrawalEligibility | null
  amount: number
}

export function canRequestWithdrawTaxTrial({
  open,
  hasTaxTrialHandler,
  eligibility,
  amount,
}: CanRequestWithdrawTaxTrialInput) {
  return (
    open &&
    hasTaxTrialHandler &&
    !!eligibility?.enabled &&
    !eligibility?.need_profile &&
    !eligibility?.need_sign &&
    amount > 0
  )
}

export function WithdrawSubmissionFields({
  availableAmount,
  minAmount,
  eligibility,
  amount,
  remark,
  taxTrial,
  taxTrialLoading = false,
  taxTrialError = '',
  taxTrialActionDisabled = true,
  onTaxTrialClick,
  onAmountChange,
  onRemarkChange,
}: WithdrawSubmissionFieldsProps) {
  const { t } = useTranslation()
  const blockingReasons = eligibility?.blocking_reasons || []
  const requestedAmount = taxTrial?.requested_amount || taxTrial?.pretax_amount
  const platformFeeAmount = taxTrial?.platform_fee_amount
  const piggyTaxBeforeAmount =
    taxTrial?.piggy_tax_before_amount || taxTrial?.pretax_amount
  const platformFeeRate =
    typeof taxTrial?.platform_fee_rate === 'number'
      ? `${taxTrial.platform_fee_rate}%`
      : '-'

  return (
    <div className='space-y-4 py-3'>
      <div className='grid grid-cols-2 gap-3'>
        <div>
          <div className='text-muted-foreground text-xs font-medium uppercase'>
            {t('Available')}
          </div>
          <div className='mt-1 font-mono text-lg font-semibold'>
            {formatCurrency(availableAmount)}
          </div>
        </div>
        <div>
          <div className='text-muted-foreground text-xs font-medium uppercase'>
            {t('Minimum')}
          </div>
          <div className='mt-1 font-mono text-lg font-semibold'>
            {formatCurrency(minAmount)}
          </div>
        </div>
      </div>

      {blockingReasons.length > 0 ? (
        <Alert>
          <AlertDescription>
            <ul className='list-disc space-y-1 ps-4'>
              {blockingReasons.map((reason) => (
                <li key={reason}>{t(reason)}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='space-y-2'>
        <Label htmlFor='withdraw-amount'>{t('Amount')}</Label>
        <Input
          id='withdraw-amount'
          type='number'
          value={amount}
          min={minAmount || 0.01}
          max={availableAmount}
          step={1}
          onChange={(event) => onAmountChange(Number(event.target.value))}
        />
      </div>

      <div className='space-y-2'>
        <Label htmlFor='withdraw-remark'>{t('Remark')}</Label>
        <Textarea
          id='withdraw-remark'
          value={remark}
          onChange={(event) => onRemarkChange(event.target.value)}
        />
      </div>

      <div className='rounded-lg border p-3'>
        <div className='flex items-center justify-between gap-2'>
          <div className='text-sm font-medium'>{t('Tax estimate')}</div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={onTaxTrialClick}
            disabled={taxTrialActionDisabled || taxTrialLoading}
          >
            {taxTrialLoading ? (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            ) : (
              <Calculator data-icon='inline-start' />
            )}
            {t('Estimate tax')}
          </Button>
        </div>
        {taxTrial ? (
          <div className='mt-3 grid grid-cols-2 gap-3 text-sm'>
            <div>
              <div className='text-muted-foreground text-xs'>
                {t('Requested amount')}
              </div>
              <div className='font-mono font-medium'>
                {formatCurrency(requestedAmount || 0)}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>
                {t('Platform fee rate')}
              </div>
              <div className='font-mono font-medium'>{platformFeeRate}</div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>
                {t('Platform fee amount')}
              </div>
              <div className='font-mono font-medium'>
                {platformFeeAmount ? formatCurrency(platformFeeAmount) : '-'}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>
                {t('Piggy tax-before amount')}
              </div>
              <div className='font-mono font-medium'>
                {formatCurrency(piggyTaxBeforeAmount || 0)}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>
                {t('Actual received')}
              </div>
              <div className='font-mono font-medium'>
                {formatCurrency(taxTrial.after_tax_amount)}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>
                {t('Individual tax')}
              </div>
              <div className='font-mono font-medium'>
                {formatCurrency(taxTrial.individual_tax_amount)}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>{t('VAT')}</div>
              <div className='font-mono font-medium'>
                {formatCurrency(taxTrial.added_tax_amount)}
              </div>
            </div>
          </div>
        ) : (
          <div className='text-muted-foreground mt-2 text-sm'>
            {taxTrialError ||
              t('Enter an amount and click Estimate tax to view details')}
          </div>
        )}
        {taxTrial ? (
          <div className='text-muted-foreground mt-2 text-xs'>
            {t('Final tax is based on provider payment result')}
          </div>
        ) : null}
      </div>
    </div>
  )
}

export function WithdrawDialog({
  open,
  onOpenChange,
  availableAmount,
  minAmount,
  submitting,
  eligibility,
  taxTrial,
  taxTrialLoading,
  taxTrialError,
  onTaxTrial,
  onTaxTrialReset,
  onSubmit,
}: WithdrawDialogProps) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState(0)
  const [remark, setRemark] = useState('')

  useEffect(() => {
    if (open) {
      // 每次打开提现弹窗时重置提交草稿，避免沿用上一次输入。
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setAmount(Math.max(minAmount, 0) || 1)
      setRemark('')
      onTaxTrialReset?.()
    }
  }, [minAmount, onTaxTrialReset, open])

  const invalid =
    !eligibility?.enabled ||
    !eligibility?.can_withdraw ||
    eligibility?.need_profile ||
    eligibility?.need_sign ||
    amount <= 0 ||
    amount > availableAmount ||
    (minAmount > 0 && amount < minAmount)

  const taxTrialEnabled = canRequestWithdrawTaxTrial({
    open,
    hasTaxTrialHandler: !!onTaxTrial,
    eligibility,
    amount,
  })

  useEffect(() => {
    if (!open) {
      onTaxTrialReset?.()
      return
    }
    onTaxTrialReset?.()
  }, [amount, onTaxTrialReset, open])

  const handleTaxTrial = () => {
    if (!taxTrialEnabled) {
      return
    }
    void onTaxTrial?.(amount)
  }

  const handleSubmit = async () => {
    const success = await onSubmit({
      amount,
      remark: remark.trim(),
    })
    if (success) {
      onOpenChange(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Withdraw Commission')}</DialogTitle>
          <DialogDescription>
            {t(
              'Only withdrawable commission can be withdrawn. Balance cannot be withdrawn.'
            )}
          </DialogDescription>
        </DialogHeader>

        <WithdrawSubmissionFields
          availableAmount={availableAmount}
          minAmount={minAmount}
          eligibility={eligibility}
          amount={amount}
          remark={remark}
          taxTrial={taxTrial}
          taxTrialLoading={taxTrialLoading}
          taxTrialError={taxTrialError}
          taxTrialActionDisabled={!taxTrialEnabled}
          onTaxTrialClick={handleTaxTrial}
          onAmountChange={setAmount}
          onRemarkChange={setRemark}
        />

        <DialogFooter className='grid grid-cols-2 gap-2 sm:flex'>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            {t('Close')}
          </Button>
          <Button onClick={handleSubmit} disabled={submitting || invalid}>
            {submitting && <Loader2 className='mr-2 size-4 animate-spin' />}
            {t('Submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
