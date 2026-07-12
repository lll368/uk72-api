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
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptions } from '../hooks/use-update-option'
import {
  buildEpaySettingsUpdates,
  epaySettingsSchema,
  type EpaySettingsValues,
} from './payment-settings-core'

type EpaySettingsSectionProps = {
  defaultValues: EpaySettingsValues
}

export function EpaySettingsSection({
  defaultValues,
}: EpaySettingsSectionProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )

  const form = useForm({
    resolver: zodResolver(epaySettingsSchema),
    mode: 'onChange',
    defaultValues,
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(defaultsSignature) as EpaySettingsValues
    initialRef.current = parsedDefaults
    form.reset(parsedDefaults)
  }, [defaultsSignature, form])

  const saveSettings = async (values: EpaySettingsValues) => {
    const updates = buildEpaySettingsUpdates(initialRef.current, values)
    if (updates.length > 0) {
      await updateOptions.mutateAsync({ options: updates })
    }
  }

  return (
    <SettingsSection
      title={t('Epay Gateway')}
      description={t('Configuration for Epay payment integration')}
    >
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(saveSettings)}
          className='space-y-6'
          data-no-autosubmit='true'
        >
          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='PayAddress'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Epay endpoint')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://pay.example.com')}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Base address provided by your Epay service')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='CustomCallbackAddress'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Callback address')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://gateway.example.com')}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional callback override. Leave blank to use server address'
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
              name='EpayId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Epay merchant ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='10001'
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
              name='EpayKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Epay secret key')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('Enter new key to update')}
                      autoComplete='new-password'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave blank unless rotating the secret')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Button type='submit' disabled={updateOptions.isPending}>
            {updateOptions.isPending
              ? t('Saving...')
              : t('Save Epay settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
