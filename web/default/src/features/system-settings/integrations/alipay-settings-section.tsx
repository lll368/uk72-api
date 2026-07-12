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
import * as React from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptions } from '../hooks/use-update-option'
import {
  buildAlipaySettingsUpdates,
  type AlipaySettingsValues,
} from './alipay-settings'
import { alipaySettingsSchema } from './payment-settings-core'

type AlipaySettingsSectionProps = {
  defaultValues: AlipaySettingsValues
}

export function AlipaySettingsSection({
  defaultValues,
}: AlipaySettingsSectionProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )

  const form = useForm({
    resolver: zodResolver(alipaySettingsSchema),
    mode: 'onChange',
    defaultValues,
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(defaultsSignature) as AlipaySettingsValues
    initialRef.current = parsedDefaults
    form.reset(parsedDefaults)
  }, [defaultsSignature, form])

  const saveSettings = async (values: AlipaySettingsValues) => {
    const updates = buildAlipaySettingsUpdates(initialRef.current, values)
    if (updates.length > 0) {
      await updateOptions.mutateAsync({ options: updates })
    }
  }

  return (
    <SettingsSection
      title={t('Alipay Direct Gateway')}
      description={t(
        'Configuration for official Alipay direct payment integration'
      )}
    >
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(saveSettings)}
          className='space-y-6'
          data-no-autosubmit='true'
        >
          <Alert>
            <AlertDescription className='text-xs'>
              {t(
                'Direct Alipay is independent from Epay and keeps Epay alipay unchanged.'
              )}
            </AlertDescription>
          </Alert>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AlipayEnabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Enable direct Alipay')}
                    </FormLabel>
                    <FormDescription>
                      {t('Show alipay_direct as an independent payment method')}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AlipaySandbox'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Sandbox mode')}
                    </FormLabel>
                    <FormDescription>
                      {t('Use the Alipay sandbox gateway')}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='AlipayAppId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Alipay app ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='2021000000000000'
                      autoComplete='off'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AlipayUnitPrice'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Alipay unit price (local currency / USD)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0.01}
                      value={(field.value ?? 7.3) as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Used for direct Alipay top-up pricing')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AlipayMinTopUp'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Minimum top-up (USD)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='1'
                      min={0}
                      value={(field.value ?? 1) as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Minimum recharge amount for direct Alipay')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AlipayPrivateKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Alipay merchant private key')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={4}
                      placeholder={t('Leave blank to keep the existing key')}
                      autoComplete='new-password'
                      className='font-mono text-xs'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'RSA2 private key in PEM or plain base64 format. Stored value is not echoed back for security.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AlipayPublicKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Alipay public key')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={4}
                      placeholder={t('Leave blank to keep the existing key')}
                      autoComplete='new-password'
                      className='font-mono text-xs'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Alipay platform public key in PEM or plain base64 format. Stored value is not echoed back for security.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AlipayReturnUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Payment return URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://example.com/console/topup'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Optional. Defaults to the wallet page when empty')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AlipayNotifyUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Alipay notify URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://example.com/api/alipay/notify'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional. Defaults to <ServerAddress>/api/alipay/notify when empty'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Button type='submit' disabled={updateOptions.isPending}>
            {updateOptions.isPending
              ? t('Saving...')
              : t('Save direct Alipay settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
