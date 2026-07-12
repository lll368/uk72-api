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
import { Code2, Eye } from 'lucide-react'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptions } from '../hooks/use-update-option'
import { CreemProductsVisualEditor } from './creem-products-visual-editor'
import {
  buildCreemSettingsUpdates,
  creemSettingsSchema,
  type CreemSettingsValues,
} from './payment-settings-core'
import { formatJsonForEditor } from './utils'

type CreemSettingsSectionProps = {
  defaultValues: CreemSettingsValues
}

export function CreemSettingsSection({
  defaultValues,
}: CreemSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )
  const [creemProductsVisualMode, setCreemProductsVisualMode] =
    React.useState(true)

  const form = useForm({
    resolver: zodResolver(creemSettingsSchema),
    mode: 'onChange',
    defaultValues: {
      ...defaultValues,
      CreemProducts: formatJsonForEditor(defaultValues.CreemProducts),
    },
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(defaultsSignature) as CreemSettingsValues
    initialRef.current = parsedDefaults
    form.reset({
      ...parsedDefaults,
      CreemProducts: formatJsonForEditor(parsedDefaults.CreemProducts),
    })
  }, [defaultsSignature, form])

  const saveSettings = async (values: CreemSettingsValues) => {
    const updates = buildCreemSettingsUpdates(initialRef.current, values)
    if (updates.length > 0) {
      await updateOptions.mutateAsync({ options: updates })
    }
  }

  return (
    <SettingsSection
      title={t('Creem Gateway')}
      description={t('Configuration for Creem payment integration')}
    >
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(saveSettings)}
          className='space-y-6'
          data-no-autosubmit='true'
        >
          <div className='rounded-md bg-blue-50 p-4 text-sm text-blue-900 dark:bg-blue-950 dark:text-blue-100'>
            <p className='mb-2 font-medium'>{t('Webhook Configuration:')}</p>
            <ul className='list-inside list-disc space-y-1'>
              <li>
                {t('Webhook URL:')}{' '}
                <code className='rounded bg-blue-100 px-1 py-0.5 text-xs dark:bg-blue-900'>
                  {'<ServerAddress>/api/creem/webhook'}
                </code>
              </li>
              <li>{t('Configure in your Creem dashboard')}</li>
            </ul>
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='CreemApiKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('API Key')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('Enter Creem API key')}
                      autoComplete='new-password'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Creem API key (leave blank unless updating)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='CreemWebhookSecret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Webhook Secret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('Enter webhook secret')}
                      autoComplete='new-password'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Webhook signing secret (leave blank unless updating)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='CreemTestMode'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>{t('Test Mode')}</FormLabel>
                  <FormDescription>
                    {t('Enable test mode for Creem payments')}
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
            name='CreemProducts'
            render={({ field }) => (
              <FormItem>
                <div className='mb-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                  <FormLabel>{t('Products')}</FormLabel>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      setCreemProductsVisualMode(!creemProductsVisualMode)
                    }
                    className='w-full sm:w-auto'
                  >
                    {creemProductsVisualMode ? (
                      <>
                        <Code2 className='mr-2 h-3 w-3' />
                        {t('JSON Editor')}
                      </>
                    ) : (
                      <>
                        <Eye className='mr-2 h-3 w-3' />
                        {t('Visual Editor')}
                      </>
                    )}
                  </Button>
                </div>
                <FormControl>
                  {creemProductsVisualMode ? (
                    <CreemProductsVisualEditor
                      value={field.value}
                      onChange={field.onChange}
                    />
                  ) : (
                    <Textarea
                      rows={4}
                      placeholder='[{"name":"Basic","productId":"prod_xxx","price":10,"quota":500000,"currency":"USD"}]'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  )}
                </FormControl>
                <FormDescription>
                  {t('Configure Creem products. Provide a JSON array.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type='submit' disabled={updateOptions.isPending}>
            {updateOptions.isPending
              ? t('Saving...')
              : t('Save Creem settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
