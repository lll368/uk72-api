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
import { Switch } from '@/components/ui/switch'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptions } from '../hooks/use-update-option'
import {
  buildStripeSettingsUpdates,
  stripeSettingsSchema,
  type StripeSettingsValues,
} from './payment-settings-core'

type StripeSettingsSectionProps = {
  defaultValues: StripeSettingsValues
}

export function StripeSettingsSection({
  defaultValues,
}: StripeSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )

  const form = useForm({
    resolver: zodResolver(stripeSettingsSchema),
    mode: 'onChange',
    defaultValues,
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(defaultsSignature) as StripeSettingsValues
    initialRef.current = parsedDefaults
    form.reset(parsedDefaults)
  }, [defaultsSignature, form])

  const saveSettings = async (values: StripeSettingsValues) => {
    const updates = buildStripeSettingsUpdates(initialRef.current, values)
    if (updates.length > 0) {
      await updateOptions.mutateAsync({ options: updates })
    }
  }

  return (
    <SettingsSection
      title={t('Stripe Gateway')}
      description={t('Configuration for Stripe payment integration')}
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
                  {'<ServerAddress>/api/stripe/webhook'}
                </code>
              </li>
              <li>
                {t('Required events:')}{' '}
                <code className='rounded bg-blue-100 px-1 py-0.5 text-xs dark:bg-blue-900'>
                  {t('checkout.session.completed')}
                </code>{' '}
                {t('and')}{' '}
                <code className='rounded bg-blue-100 px-1 py-0.5 text-xs dark:bg-blue-900'>
                  {t('checkout.session.expired')}
                </code>
              </li>
              <li>
                {t('Configure at:')}{' '}
                <a
                  href='https://dashboard.stripe.com/developers'
                  target='_blank'
                  rel='noreferrer'
                  className='underline hover:no-underline'
                >
                  {t('Stripe Dashboard')}
                </a>
              </li>
            </ul>
          </div>

          <div className='grid gap-6 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='StripeApiSecret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('API secret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('sk_xxx or rk_xxx')}
                      autoComplete='new-password'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Stripe API key (leave blank unless updating)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='StripeWebhookSecret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Webhook secret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('whsec_xxx')}
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

            <FormField
              control={form.control}
              name='StripePriceId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Price ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('price_xxx')}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Stripe product price ID')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='StripeUnitPrice'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Unit price (local currency / USD)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0}
                      value={(field.value ?? 0) as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('e.g., 8 means 8 local currency per USD')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='StripeMinTopUp'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Minimum top-up (USD)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0}
                      value={(field.value ?? 0) as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Minimum recharge amount in USD')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='StripePromotionCodesEnabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Promotion codes')}
                    </FormLabel>
                    <FormDescription>
                      {t('Allow users to enter promo codes')}
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

          <Button type='submit' disabled={updateOptions.isPending}>
            {updateOptions.isPending
              ? t('Saving...')
              : t('Save Stripe settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
