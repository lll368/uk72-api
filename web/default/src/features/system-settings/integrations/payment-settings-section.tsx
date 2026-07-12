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
import { cn } from '@/lib/utils'
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
import { Textarea } from '@/components/ui/textarea'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptions } from '../hooks/use-update-option'
import { AmountDiscountVisualEditor } from './amount-discount-visual-editor'
import { AmountOptionsVisualEditor } from './amount-options-visual-editor'
import { PaymentMethodsVisualEditor } from './payment-methods-visual-editor'
import {
  buildGeneralPaymentSettingsUpdates,
  generalPaymentSettingsSchema,
  type GeneralPaymentSettingsValues,
} from './payment-settings-core'
import { formatJsonForEditor } from './utils'

type PaymentSettingsSectionProps = {
  defaultValues: GeneralPaymentSettingsValues
}

export function PaymentSettingsSection({
  defaultValues,
}: PaymentSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )

  const [payMethodsVisualMode, setPayMethodsVisualMode] = React.useState(true)
  const [amountOptionsVisualMode, setAmountOptionsVisualMode] =
    React.useState(true)
  const [amountDiscountVisualMode, setAmountDiscountVisualMode] =
    React.useState(true)

  const form = useForm({
    resolver: zodResolver(generalPaymentSettingsSchema),
    mode: 'onChange',
    defaultValues: {
      ...defaultValues,
      PayMethods: formatJsonForEditor(defaultValues.PayMethods),
      AmountOptions: formatJsonForEditor(defaultValues.AmountOptions),
      AmountDiscount: formatJsonForEditor(defaultValues.AmountDiscount),
    },
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(
      defaultsSignature
    ) as GeneralPaymentSettingsValues
    initialRef.current = parsedDefaults
    form.reset({
      ...parsedDefaults,
      PayMethods: formatJsonForEditor(parsedDefaults.PayMethods),
      AmountOptions: formatJsonForEditor(parsedDefaults.AmountOptions),
      AmountDiscount: formatJsonForEditor(parsedDefaults.AmountDiscount),
    })
  }, [defaultsSignature, form])

  const saveSettings = async (values: GeneralPaymentSettingsValues) => {
    const updates = buildGeneralPaymentSettingsUpdates(
      initialRef.current,
      values
    )
    if (updates.length > 0) {
      await updateOptions.mutateAsync({ options: updates })
    }
  }

  return (
    <SettingsSection
      title={t('Payment General')}
      description={t('Shared recharge and VVIP payment rules')}
    >
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(saveSettings)}
          className={cn('space-y-8')}
          data-no-autosubmit='true'
        >
          <div className='space-y-4'>
            <div>
              <h3 className='text-lg font-medium'>{t('General Settings')}</h3>
              <p className='text-muted-foreground text-sm'>
                {t('Shared configuration for all payment gateways')}
              </p>
            </div>

            <div className='grid gap-6 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='Price'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Price (local currency / USD)')}</FormLabel>
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
                      {t(
                        'How much to charge for each US dollar of balance (Epay)'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='MinTopUp'
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
                      {t('Smallest USD amount users can recharge (Epay)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='CommissionMinWithdrawAmount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum withdrawal amount')}</FormLabel>
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
                      {t(
                        'Smallest commission amount users can withdraw. Use 0 to disable this limit.'
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
                name='DefaultUserTopupDiscount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default user top-up discount')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        min={0.01}
                        max={1}
                        value={(field.value ?? 1) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Applied to newly registered or created users. Use 1 for no discount.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='DefaultVvipTopupDiscount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default VVIP top-up discount')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        min={0.01}
                        max={1}
                        value={(field.value ?? 1) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Applied after a user successfully activates VVIP. Use 1 for no discount.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='space-y-3'>
              <div className='space-y-1'>
                <h4 className='text-sm font-medium'>
                  {t('VVIP activation settings')}
                </h4>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Configure VVIP activation fee and fixed commission amounts'
                  )}
                </p>
              </div>

              <div className='grid gap-6 md:grid-cols-3'>
                <FormField
                  control={form.control}
                  name='VipActivationPrice'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('VVIP activation price')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          step='0.01'
                          min={0.01}
                          value={(field.value ?? 1680) as number}
                          onChange={(event) =>
                            field.onChange(event.target.valueAsNumber)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('One-time VVIP activation amount charged to users.')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='VvipActivationCommissionLevel1Amount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('VVIP activation level 1 commission amount')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          step='0.01'
                          min={0}
                          value={(field.value ?? 1000) as number}
                          onChange={(event) =>
                            field.onChange(event.target.valueAsNumber)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Fixed amount credited to the direct VVIP parent when a user activates VVIP.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='VvipActivationCommissionLevel2Amount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('VVIP activation level 2 commission amount')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          step='0.01'
                          min={0}
                          value={(field.value ?? 400) as number}
                          onChange={(event) =>
                            field.onChange(event.target.valueAsNumber)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Fixed amount credited to the indirect VVIP parent. The two amounts cannot exceed the activation price in total.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </div>

            <FormField
              control={form.control}
              name='PayMethods'
              render={({ field }) => (
                <FormItem>
                  <div className='mb-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                    <FormLabel>{t('Payment methods')}</FormLabel>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        setPayMethodsVisualMode(!payMethodsVisualMode)
                      }
                      className='w-full sm:w-auto'
                    >
                      {payMethodsVisualMode ? (
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
                    {payMethodsVisualMode ? (
                      <PaymentMethodsVisualEditor
                        value={field.value}
                        onChange={field.onChange}
                      />
                    ) : (
                      <Textarea
                        rows={4}
                        placeholder={t(
                          '[{"name":"支付宝","type":"alipay","color":"#1677FF"}]'
                        )}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    )}
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Configure available payment methods. Provide a JSON array.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid gap-6 md:grid-cols-2 md:items-start'>
              <FormField
                control={form.control}
                name='AmountOptions'
                render={({ field }) => (
                  <FormItem>
                    <div className='mb-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                      <FormLabel>{t('Top-up amount options')}</FormLabel>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() =>
                          setAmountOptionsVisualMode(!amountOptionsVisualMode)
                        }
                        className='w-full sm:w-auto'
                      >
                        {amountOptionsVisualMode ? (
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
                      {amountOptionsVisualMode ? (
                        <AmountOptionsVisualEditor
                          value={field.value}
                          onChange={field.onChange}
                        />
                      ) : (
                        <Textarea
                          rows={4}
                          placeholder='[10, 20, 50, 100]'
                          {...field}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                        />
                      )}
                    </FormControl>
                    <FormDescription>
                      {t('Preset recharge amounts (JSON array)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AmountDiscount'
                render={({ field }) => (
                  <FormItem>
                    <div className='mb-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                      <FormLabel>{t('Amount discount')}</FormLabel>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() =>
                          setAmountDiscountVisualMode(!amountDiscountVisualMode)
                        }
                        className='w-full sm:w-auto'
                      >
                        {amountDiscountVisualMode ? (
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
                      {amountDiscountVisualMode ? (
                        <AmountDiscountVisualEditor
                          value={field.value}
                          onChange={field.onChange}
                        />
                      ) : (
                        <Textarea
                          rows={4}
                          placeholder='{"100":0.95,"200":0.9}'
                          {...field}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                        />
                      )}
                    </FormControl>
                    <FormDescription>
                      {t('Discount map by recharge amount (JSON object)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <Button type='submit' disabled={updateOptions.isPending}>
              {updateOptions.isPending
                ? t('Saving...')
                : t('Save general settings')}
            </Button>
          </div>
        </form>
      </Form>
    </SettingsSection>
  )
}
