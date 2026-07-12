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
import { toast } from 'sonner'
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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { reverseTopupOrder, reverseVipActivationOrder } from '../api'
import type { ReverseOrderType } from '../types'

interface ReverseOrderDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess?: () => void
}

export function ReverseOrderDialog({
  open,
  onOpenChange,
  onSuccess,
}: ReverseOrderDialogProps) {
  const { t } = useTranslation()
  const [type, setType] = useState<ReverseOrderType>('topup')
  const [tradeNo, setTradeNo] = useState('')
  const [provider, setProvider] = useState('')
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const reset = () => {
    setType('topup')
    setTradeNo('')
    setProvider('')
    setReason('')
  }

  const handleSubmit = async () => {
    if (!tradeNo.trim()) {
      toast.error(t('Please enter trade no.'))
      return
    }
    if (!reason.trim()) {
      toast.error(t('Please enter reverse reason'))
      return
    }
    setSubmitting(true)
    try {
      const response =
        type === 'topup'
          ? await reverseTopupOrder(
              tradeNo.trim(),
              provider.trim(),
              reason.trim()
            )
          : await reverseVipActivationOrder(
              tradeNo.trim(),
              provider.trim(),
              reason.trim()
            )
      if (response.success) {
        toast.success(t('Operation successful'))
        reset()
        onOpenChange(false)
        onSuccess?.()
      } else {
        toast.error(response.message || t('Operation failed'))
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        onOpenChange(nextOpen)
        if (!nextOpen) reset()
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Reverse Payment Order')}</DialogTitle>
          <DialogDescription>
            {t(
              'Reversal affects balance, commission, and VVIP state. Confirm the order and reason before continuing.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-3'>
          <NativeSelect
            value={type}
            onChange={(event) =>
              setType(event.target.value as ReverseOrderType)
            }
            className='w-full'
          >
            <NativeSelectOption value='topup'>
              {t('Top-up Order')}
            </NativeSelectOption>
            <NativeSelectOption value='vip_activation'>
              {t('VVIP Activation Order')}
            </NativeSelectOption>
          </NativeSelect>
          <Input
            value={tradeNo}
            onChange={(event) => setTradeNo(event.target.value)}
            placeholder={t('Trade No.')}
          />
          <Input
            value={provider}
            onChange={(event) => setProvider(event.target.value)}
            placeholder={t('Provider')}
          />
          <Input
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder={t('Reverse reason')}
          />
        </div>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={submitting}
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            variant='destructive'
            disabled={submitting}
            onClick={handleSubmit}
          >
            {t('Reverse')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
