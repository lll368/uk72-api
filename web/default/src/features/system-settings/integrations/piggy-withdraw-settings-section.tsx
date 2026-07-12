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
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptions } from '../hooks/use-update-option'
import { savePiggyWithdrawSettingOptions } from './piggy-withdraw-settings'
import { removeTrailingSlash } from './utils'

export interface PiggyWithdrawSettingsValues {
  Enabled: boolean
  Domain: string
  AppKey: string
  AppSecret: string
  AESIV: string
  TaxFundId: string
  PositionName: string
  Position: string
  SignJumpPage: string
  SignNotifyUrl: string
  PayNotifyUrl: string
  RequestTimeout: number
  CallbackLockTTL: number
  CooldownMinutes: number
  ForbiddenWithdrawTime: string
  CalcType: string
  PlatformFeeRate: number
  BankRemark: string
}

interface Props {
  defaultValues: PiggyWithdrawSettingsValues
}

const isHttpUrl = (value: string, required = false) => {
  const trimmed = value.trim()
  if (!trimmed) return !required
  return /^https?:\/\//.test(trimmed)
}

export function PiggyWithdrawSettingsSection(props: Props) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const [loading, setLoading] = useState(false)

  const form = useForm<PiggyWithdrawSettingsValues>({
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
  }, [props.defaultValues, form])

  const handleSave = async () => {
    const values = form.getValues()
    const enabled = Boolean(values.Enabled)
    const domain = removeTrailingSlash(values.Domain || '')
    const signJumpPage = removeTrailingSlash(values.SignJumpPage || '')
    const signNotifyUrl = removeTrailingSlash(values.SignNotifyUrl || '')
    const payNotifyUrl = removeTrailingSlash(values.PayNotifyUrl || '')
    const calcType = String(values.CalcType || 'C').toUpperCase()
    const platformFeeRate = Number(values.PlatformFeeRate)

    if (enabled && !isHttpUrl(domain, true)) {
      toast.error(t('Piggy domain must start with http:// or https://'))
      return
    }
    if (enabled && !values.AppKey.trim()) {
      toast.error(t('Piggy app key is required'))
      return
    }
    if (enabled && !values.AESIV.trim()) {
      toast.error(t('Piggy AES IV is required'))
      return
    }
    if (values.AESIV.trim() && values.AESIV.trim().length !== 16) {
      toast.error(t('Piggy AES IV must be 16 bytes'))
      return
    }
    if (enabled && !values.TaxFundId.trim()) {
      toast.error(t('Piggy tax fund ID is required'))
      return
    }
    if (enabled && !values.PositionName.trim()) {
      toast.error(t('Piggy position name is required'))
      return
    }
    if (enabled && !values.Position.trim()) {
      toast.error(t('Piggy position is required'))
      return
    }
    if (enabled && !isHttpUrl(signNotifyUrl, true)) {
      toast.error(t('Piggy contract callback URL is required'))
      return
    }
    if (enabled && !isHttpUrl(payNotifyUrl, true)) {
      toast.error(t('Piggy payment callback URL is required'))
      return
    }
    if (signJumpPage && !isHttpUrl(signJumpPage)) {
      toast.error(
        t('Piggy contract jump page must start with http:// or https://')
      )
      return
    }
    if (calcType !== 'C' && calcType !== 'E') {
      toast.error(t('Piggy calc type only supports C or E'))
      return
    }
    if (
      !Number.isFinite(platformFeeRate) ||
      platformFeeRate < 0 ||
      platformFeeRate >= 100
    ) {
      toast.error(
        t('Piggy platform fee rate must be at least 0 and less than 100')
      )
      return
    }
    if (Math.round(platformFeeRate * 10000) !== platformFeeRate * 10000) {
      toast.error(t('Piggy platform fee rate supports up to 4 decimal places'))
      return
    }

    setLoading(true)
    try {
      const defaults = props.defaultValues
      const options: { key: string; value: string }[] = []
      const appendIfChanged = (
        key: string,
        value: string,
        initialValue: string
      ) => {
        if (value !== initialValue) {
          options.push({ key, value })
        }
      }

      appendIfChanged(
        'piggy_withdraw_setting.domain',
        domain,
        removeTrailingSlash(defaults.Domain || '')
      )
      appendIfChanged(
        'piggy_withdraw_setting.app_key',
        values.AppKey.trim(),
        defaults.AppKey.trim()
      )
      appendIfChanged(
        'piggy_withdraw_setting.aes_iv',
        values.AESIV.trim(),
        defaults.AESIV.trim()
      )
      appendIfChanged(
        'piggy_withdraw_setting.tax_fund_id',
        values.TaxFundId.trim(),
        defaults.TaxFundId.trim()
      )
      appendIfChanged(
        'piggy_withdraw_setting.position_name',
        values.PositionName.trim(),
        defaults.PositionName.trim()
      )
      appendIfChanged(
        'piggy_withdraw_setting.position',
        values.Position.trim(),
        defaults.Position.trim()
      )
      appendIfChanged(
        'piggy_withdraw_setting.sign_jump_page',
        signJumpPage,
        removeTrailingSlash(defaults.SignJumpPage || '')
      )
      appendIfChanged(
        'piggy_withdraw_setting.sign_notify_url',
        signNotifyUrl,
        removeTrailingSlash(defaults.SignNotifyUrl || '')
      )
      appendIfChanged(
        'piggy_withdraw_setting.pay_notify_url',
        payNotifyUrl,
        removeTrailingSlash(defaults.PayNotifyUrl || '')
      )
      appendIfChanged(
        'piggy_withdraw_setting.request_timeout',
        String(Math.max(1, Number(values.RequestTimeout) || 15)),
        String(Math.max(1, Number(defaults.RequestTimeout) || 15))
      )
      appendIfChanged(
        'piggy_withdraw_setting.callback_lock_ttl',
        String(Math.max(1, Number(values.CallbackLockTTL) || 300)),
        String(Math.max(1, Number(defaults.CallbackLockTTL) || 300))
      )
      appendIfChanged(
        'piggy_withdraw_setting.cooldown_minutes',
        String(Math.max(0, Number(values.CooldownMinutes) || 0)),
        String(Math.max(0, Number(defaults.CooldownMinutes) || 0))
      )
      appendIfChanged(
        'piggy_withdraw_setting.forbidden_withdraw_time',
        values.ForbiddenWithdrawTime.trim(),
        defaults.ForbiddenWithdrawTime.trim()
      )
      appendIfChanged(
        'piggy_withdraw_setting.calc_type',
        calcType,
        String(defaults.CalcType || 'C').toUpperCase()
      )
      appendIfChanged(
        'piggy_withdraw_setting.platform_fee_rate',
        String(platformFeeRate),
        String(Number(defaults.PlatformFeeRate ?? 8))
      )
      appendIfChanged(
        'piggy_withdraw_setting.bank_remark',
        values.BankRemark.trim(),
        defaults.BankRemark.trim()
      )

      if (values.AppSecret.trim()) {
        appendIfChanged(
          'piggy_withdraw_setting.app_secret',
          values.AppSecret.trim(),
          defaults.AppSecret.trim()
        )
      }

      appendIfChanged(
        'piggy_withdraw_setting.enabled',
        enabled ? 'true' : 'false',
        defaults.Enabled ? 'true' : 'false'
      )

      await savePiggyWithdrawSettingOptions(options, (nextOptions) =>
        updateOptions.mutateAsync({ options: nextOptions })
      )
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Update failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <SettingsSection
      title={t('Piggy bank card withdrawal')}
      description={t(
        'Configure Piggy continuous labor V3 bank card withdrawal for commissions'
      )}
    >
      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'Contract callback: <ServerAddress>/api/withdraw/piggy/contract/notify. Payment callback: <ServerAddress>/api/withdraw/piggy/payment/notify.'
          )}
        </AlertDescription>
      </Alert>

      <div className='grid gap-4 md:grid-cols-3'>
        <div className='flex items-center gap-2'>
          <Switch
            checked={form.watch('Enabled')}
            onCheckedChange={(value) => form.setValue('Enabled', value)}
          />
          <Label>{t('Enable Piggy withdrawal')}</Label>
        </div>
        <div className='grid gap-1.5 md:col-span-2'>
          <Label>{t('Piggy domain')}</Label>
          <Input
            placeholder='https://saas.xzsz.ltd'
            {...form.register('Domain')}
          />
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-3'>
        <div className='grid gap-1.5'>
          <Label>{t('App Key')}</Label>
          <Input {...form.register('AppKey')} />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('App Secret')}</Label>
          <Input
            type='password'
            placeholder={t('Leave blank unless updating')}
            {...form.register('AppSecret')}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('AES IV')}</Label>
          <Input maxLength={16} {...form.register('AESIV')} />
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-3'>
        <div className='grid gap-1.5'>
          <Label>{t('Tax fund ID')}</Label>
          <Input {...form.register('TaxFundId')} />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Position name')}</Label>
          <Input {...form.register('PositionName')} />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Position')}</Label>
          <Input {...form.register('Position')} />
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-3'>
        <div className='grid gap-1.5'>
          <Label>{t('Contract jump page')}</Label>
          <Input {...form.register('SignJumpPage')} />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Contract callback URL')}</Label>
          <Input {...form.register('SignNotifyUrl')} />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Payment callback URL')}</Label>
          <Input {...form.register('PayNotifyUrl')} />
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-4'>
        <div className='grid gap-1.5'>
          <Label>{t('Request timeout seconds')}</Label>
          <Input
            type='number'
            min={1}
            {...form.register('RequestTimeout', { valueAsNumber: true })}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Callback lock TTL seconds')}</Label>
          <Input
            type='number'
            min={1}
            {...form.register('CallbackLockTTL', { valueAsNumber: true })}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Withdrawal cooldown minutes')}</Label>
          <Input
            type='number'
            min={0}
            {...form.register('CooldownMinutes', { valueAsNumber: true })}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Calc type')}</Label>
          <NativeSelect
            value={form.watch('CalcType') || 'C'}
            onChange={(event) => form.setValue('CalcType', event.target.value)}
          >
            <NativeSelectOption value='C'>
              {t('C - personal bears tax')}
            </NativeSelectOption>
            <NativeSelectOption value='E'>
              {t('E - enterprise bears tax')}
            </NativeSelectOption>
          </NativeSelect>
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-2'>
        <div className='grid gap-1.5'>
          <Label>{t('Forbidden withdrawal time')}</Label>
          <Input
            placeholder='23:30-00:30'
            {...form.register('ForbiddenWithdrawTime')}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Piggy platform fee rate (%)')}</Label>
          <Input
            type='number'
            min={0}
            max={99.9999}
            step={0.0001}
            {...form.register('PlatformFeeRate', { valueAsNumber: true })}
          />
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-2'>
        <div className='grid gap-1.5'>
          <Label>{t('Bank remark')}</Label>
          <Input {...form.register('BankRemark')} />
        </div>
      </div>

      <Button
        onClick={handleSave}
        disabled={loading || updateOptions.isPending}
      >
        {loading || updateOptions.isPending
          ? t('Saving...')
          : t('Save Piggy withdrawal settings')}
      </Button>
    </SettingsSection>
  )
}
