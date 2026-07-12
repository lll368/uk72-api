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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptions } from '../hooks/use-update-option'
import {
  buildQiniuSettingsUpdates,
  QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD,
  QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY,
  QINIU_COST_DETAIL_MAX_LOOKBACK_DAYS,
  qiniuSettingsSchema,
  type QiniuSettingsValues,
} from './qiniu-settings'

type QiniuSettingsSectionProps = {
  defaultValues: QiniuSettingsValues
}

export function QiniuSettingsSection({
  defaultValues,
}: QiniuSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptions()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )

  const form = useForm<QiniuSettingsValues>({
    resolver: zodResolver(qiniuSettingsSchema),
    mode: 'onChange',
    defaultValues,
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(defaultsSignature) as QiniuSettingsValues
    initialRef.current = parsedDefaults
    form.reset(parsedDefaults)
  }, [defaultsSignature, form])

  const saveSettings = async (values: QiniuSettingsValues) => {
    const updates = buildQiniuSettingsUpdates(initialRef.current, values)
    if (updates.length > 0) {
      await updateOptions.mutateAsync({ options: updates })
    }
  }

  return (
    <SettingsSection
      title={t('Qiniu Key & Ledger')}
      description={t(
        'Configure Qiniu managed keys, official ledger sync, and model catalog'
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
                'Stored Qiniu AK/SK values are not echoed back for security. Leave AK/SK blank unless updating them.'
              )}
            </AlertDescription>
          </Alert>

          <div className='space-y-4 rounded-lg border p-4'>
            <div>
              <h4 className='text-sm font-medium'>{t('Managed Key')}</h4>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Controls Qiniu key creation, quota sync, and lifecycle retry settings'
                )}
              </p>
            </div>

            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='Enabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Enable Qiniu managed keys')}
                      </FormLabel>
                      <FormDescription>
                        {t('Create and sync user API keys through Qiniu')}
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

            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='BaseURL'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Qiniu API base URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://api.qnaigc.com'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Leave blank to use the default Qiniu API endpoint')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AccessKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Qiniu access key')}</FormLabel>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t('Leave blank to keep the existing key')}
                        autoComplete='new-password'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Qiniu account AccessKey for signed management APIs')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='SecretKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Qiniu secret key')}</FormLabel>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t('Leave blank to keep the existing key')}
                        autoComplete='new-password'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Qiniu account SecretKey for request signatures')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='RequestTimeout'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Request timeout (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 15) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='RetryIntervalSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Lifecycle retry interval (seconds)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 300) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <div className='space-y-4 rounded-lg border p-4'>
            <div>
              <h4 className='text-sm font-medium'>
                {t('Qiniu Child Account')}
              </h4>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Configure OEM child account creation defaults for the admin child account manager'
                )}
              </p>
            </div>

            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='ChildAccountBindingEnabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Enable child account binding')}
                      </FormLabel>
                      <FormDescription>
                        {t('Assign future Qiniu managed keys by account mode')}
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
                name='ChildAccountAssignmentMode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Child account assignment mode')}</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem
                          value={
                            QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY
                          }
                        >
                          {t('Parent account only')}
                        </SelectItem>
                        <SelectItem
                          value={
                            QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD
                          }
                        >
                          {t('One key per child account')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='ChildAccountBindingCutoverTime'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Child binding cutover time')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={(field.value ?? 0) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='ChildAccountBaseURL'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Child account API base URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://api.qiniu.com'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Only child account management requests use this URL; managed key creation, usage, and billing use the Qiniu key base URL'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='ChildAccountEmailDomain'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Child account email domain')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='uk72.cn'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Generated child accounts use prefix plus sequence under this domain'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='ChildAccountEmailPrefix'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Child account email prefix')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='child'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Example: child1@uk72.cn')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='grid gap-6 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='ChildAccountPasswordLength'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Child password length')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={12}
                        max={64}
                        step={1}
                        value={(field.value ?? 18) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='ChildAccountRequestTimeout'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Child request timeout (seconds)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 15) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='ChildAccountRetryIntervalSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Child retry interval (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 300) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <div className='space-y-4 rounded-lg border p-4'>
            <div>
              <h4 className='text-sm font-medium'>{t('Official Ledger')}</h4>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Sync official Qiniu usage and cost details as the billing source of truth'
                )}
              </p>
            </div>

            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='OfficialLedgerEnabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Enable official ledger sync')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Use Qiniu official records for managed key billing'
                        )}
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
                name='CostDetailAutoApplyEnabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Auto-apply cost-detail bucket adjustments')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Default enabled. Turn off only to isolate automatic Qiniu balance adjustments'
                        )}
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
                name='OfficialLedgerCutoverTime'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Ledger cutover timestamp')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={(field.value ?? 0) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Unix timestamp. Records before this time are observed only'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='CostDetailCutoverTime'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Cost-detail cutover timestamp')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={(field.value ?? 0) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Unix timestamp. It is normalized to the platform billing date before settlement'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='OfficialLedgerSyncIntervalSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Ledger sync interval (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 60) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='OfficialLedgerRetryIntervalSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Ledger retry interval (seconds)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 300) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='OfficialLedgerWindowHours'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Hourly sync window (hours)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 6) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='OfficialLedgerWindowDays'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Daily sync window (days)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 2) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='CostDetailLookbackDays'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Cost-detail lookback window (days)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={QINIU_COST_DETAIL_MAX_LOOKBACK_DAYS}
                        step={1}
                        value={(field.value ?? 7) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Days to rescan delayed Qiniu cost-detail bills')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='OfficialLedgerBatchSize'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Ledger batch size')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 100) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='OfficialLedgerRateLimitPerSecond'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Ledger requests per second')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 4) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <div className='space-y-4 rounded-lg border p-4'>
            <div>
              <h4 className='text-sm font-medium'>{t('Model Catalog')}</h4>
              <p className='text-muted-foreground text-xs'>
                {t('Fetch Qiniu model market data for pricing display')}
              </p>
            </div>

            <div className='grid gap-6 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='MarketCatalogEnabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Enable model catalog sync')}
                      </FormLabel>
                      <FormDescription>
                        {t('Use Qiniu market metadata in model pricing views')}
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
                name='MarketCatalogOverseas'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Use overseas model catalog')}
                      </FormLabel>
                      <FormDescription>
                        {t('Request the overseas Qiniu model catalog')}
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
                name='MarketCatalogFallbackEnabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Enable catalog fallback')}
                      </FormLabel>
                      <FormDescription>
                        {t('Keep the last successful catalog when Qiniu fails')}
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

            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='MarketCatalogBaseURL'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Model catalog base URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://openai.qiniu.com'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Leave blank to use the default Qiniu model endpoint')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='MarketCatalogTTLSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Model catalog cache TTL (seconds)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={(field.value ?? 3600) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <Button type='submit' disabled={updateOptions.isPending}>
            {updateOptions.isPending
              ? t('Saving...')
              : t('Save Qiniu settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
