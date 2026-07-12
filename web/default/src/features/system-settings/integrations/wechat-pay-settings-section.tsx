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
import { wechatPaySettingsSchema } from './payment-settings-core'
import {
  buildWechatPaySettingsUpdates,
  type WechatPaySettingsValues,
} from './wechat-pay-settings'

type WechatPaySettingsSectionProps = {
  defaultValues: WechatPaySettingsValues
}

export function WechatPaySettingsSection({
  defaultValues,
}: WechatPaySettingsSectionProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )

  const form = useForm({
    resolver: zodResolver(wechatPaySettingsSchema),
    mode: 'onChange',
    defaultValues,
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(
      defaultsSignature
    ) as WechatPaySettingsValues
    initialRef.current = parsedDefaults
    form.reset(parsedDefaults)
  }, [defaultsSignature, form])

  const saveSettings = async (values: WechatPaySettingsValues) => {
    const updates = buildWechatPaySettingsUpdates(initialRef.current, values)
    if (updates.length > 0) {
      await updateOptions.mutateAsync({ options: updates })
    }
  }

  return (
    <SettingsSection
      title={t('WeChat Pay Direct Gateway')}
      description={t(
        'Configuration for official WeChat Pay Native payment integration'
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
                'Direct WeChat Pay is independent from Epay and keeps Epay WeChat Pay unchanged.'
              )}
            </AlertDescription>
          </Alert>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='WechatPayEnabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Enable direct WeChat Pay')}
                    </FormLabel>
                    <FormDescription>
                      {t('Show wechat_direct as an independent payment method')}
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
              name='WechatPayAppId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('WeChat Pay app ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='wx0000000000000000'
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
              name='WechatPayMchId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('WeChat Pay merchant ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='1900000001'
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
              name='WechatPayMerchantSerialNo'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('WeChat Pay merchant certificate serial number')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder='merchant certificate serial'
                      autoComplete='off'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='WechatPayUnitPrice'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('WeChat Pay unit price (local currency / USD)')}
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
                    {t('Used for direct WeChat Pay top-up pricing')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='WechatPayMinTopUp'
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
                    {t('Minimum recharge amount for direct WeChat Pay')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='WechatPayPlatformSerialNo'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('WeChat Pay platform serial number')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder='platform certificate serial'
                      autoComplete='off'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Optional when using a platform public key')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='WechatPayMerchantPrivateKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('WeChat Pay merchant private key')}</FormLabel>
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
                      'Merchant API certificate private key in PEM or plain base64 format. Stored value is not echoed back for security.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='WechatPayAPIv3Key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('WeChat Pay API v3 key')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('32-byte API v3 key')}
                      autoComplete='new-password'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave blank to keep the existing API v3 key')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='WechatPayPlatformPublicKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('WeChat Pay platform public key')}</FormLabel>
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
                      'Platform certificate or platform public key in PEM or plain base64 format. Stored value is not echoed back for security.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='WechatPayNotifyUrl'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('WeChat Pay notify URL')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='https://example.com/api/wechat/notify'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Optional. Defaults to <ServerAddress>/api/wechat/notify when empty'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type='submit' disabled={updateOptions.isPending}>
            {updateOptions.isPending
              ? t('Saving...')
              : t('Save direct WeChat Pay settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
