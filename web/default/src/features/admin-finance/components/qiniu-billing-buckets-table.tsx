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
import { useCallback, useEffect, useState } from 'react'
import { Eye, Link2, Play, RotateCcw, SkipForward } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  getQiniuBillingBucketApplications,
  getQiniuBillingBucketItems,
  getQiniuBillingBuckets,
  getQiniuBillingSummary,
  getQiniuCostDetailRecords,
  recalculateQiniuBillingBucket,
  resolveQiniuBillingBucket,
  resolveQiniuCostDetailRecord,
  retryQiniuBillingBucketApplication,
  skipQiniuBillingBucket,
} from '../api'
import type {
  QiniuBillingBucket,
  QiniuBillingBucketApplication,
  QiniuBillingBucketItem,
  QiniuBillingSummary,
  QiniuCostDetailRecord,
} from '../types'
import {
  ADMIN_FINANCE_PAGE_SIZE,
  FilterInput,
  PaginationBar,
  StatusBadge,
  StatusFilter,
  TableShell,
  formatMoney,
  formatTime,
  getStatusLabel,
} from './shared'

type OperationTarget =
  | { type: 'recalculate'; bucket: QiniuBillingBucket }
  | { type: 'skip'; bucket: QiniuBillingBucket }
  | { type: 'retry'; application: QiniuBillingBucketApplication }
  | { type: 'resolve'; record: QiniuCostDetailRecord }
  | { type: 'resolve_bucket'; bucket: QiniuBillingBucket }

const bucketStatusOptions = [
  'pending',
  'needs_review',
  'applied',
  'failed',
  'skipped',
  'reconciled',
]

const ownerStatusOptions = [
  'resolved',
  'unmapped',
  'ambiguous',
  'manual_resolved',
]

function formatQuota(value?: number) {
  return Number(value ?? 0).toLocaleString()
}

function formatSignedQuota(value?: number) {
  const quota = Number(value ?? 0)
  return `${quota > 0 ? '+' : ''}${formatQuota(quota)}`
}

export function QiniuBillingBucketsTable() {
  const { t } = useTranslation()
  const [buckets, setBuckets] = useState<QiniuBillingBucket[]>([])
  const [bucketTotal, setBucketTotal] = useState(0)
  const [bucketPage, setBucketPage] = useState(1)
  const [bucketLoading, setBucketLoading] = useState(false)
  const [filters, setFilters] = useState({
    billingDate: '',
    userId: '',
    tokenId: '',
    childAccountId: '',
    status: '',
    ownerStatus: '',
    maskedKey: '',
  })
  const [selectedBucket, setSelectedBucket] =
    useState<QiniuBillingBucket | null>(null)
  const [items, setItems] = useState<QiniuBillingBucketItem[]>([])
  const [applications, setApplications] = useState<
    QiniuBillingBucketApplication[]
  >([])
  const [detailsLoading, setDetailsLoading] = useState(false)

  const [rawRecords, setRawRecords] = useState<QiniuCostDetailRecord[]>([])
  const [rawTotal, setRawTotal] = useState(0)
  const [rawPage, setRawPage] = useState(1)
  const [rawOwnerStatus, setRawOwnerStatus] = useState('ambiguous')
  const [rawChildAccountId, setRawChildAccountId] = useState('')
  const [rawLoading, setRawLoading] = useState(false)
  const [summary, setSummary] = useState<QiniuBillingSummary | null>(null)
  const [summaryLoading, setSummaryLoading] = useState(false)

  const [operation, setOperation] = useState<OperationTarget | null>(null)
  const [reason, setReason] = useState('')
  const [resolveTokenId, setResolveTokenId] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const fetchBuckets = useCallback(async () => {
    setBucketLoading(true)
    try {
      const response = await getQiniuBillingBuckets({
        page: bucketPage,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        ...filters,
      })
      if (response.success && response.data) {
        setBuckets(response.data.items || [])
        setBucketTotal(response.data.total || 0)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setBucketLoading(false)
    }
  }, [bucketPage, filters, t])

  const fetchRawRecords = useCallback(async () => {
    setRawLoading(true)
    try {
      const response = await getQiniuCostDetailRecords({
        page: rawPage,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        ownerStatus: rawOwnerStatus,
        childAccountId: rawChildAccountId,
      })
      if (response.success && response.data) {
        setRawRecords(response.data.items || [])
        setRawTotal(response.data.total || 0)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setRawLoading(false)
    }
  }, [rawChildAccountId, rawOwnerStatus, rawPage, t])

  const fetchSummary = useCallback(async () => {
    setSummaryLoading(true)
    try {
      const response = await getQiniuBillingSummary()
      if (response.success && response.data) {
        setSummary(response.data)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setSummaryLoading(false)
    }
  }, [t])

  const fetchDetails = useCallback(
    async (bucket: QiniuBillingBucket | null) => {
      if (!bucket) {
        setItems([])
        setApplications([])
        return
      }
      setDetailsLoading(true)
      try {
        const [itemResponse, appResponse] = await Promise.all([
          getQiniuBillingBucketItems({
            page: 1,
            pageSize: 100,
            bucketId: bucket.id,
          }),
          getQiniuBillingBucketApplications({
            page: 1,
            pageSize: 100,
            bucketId: bucket.id,
          }),
        ])
        if (itemResponse.success && itemResponse.data) {
          setItems(itemResponse.data.items || [])
        }
        if (appResponse.success && appResponse.data) {
          setApplications(appResponse.data.items || [])
        }
      } finally {
        setDetailsLoading(false)
      }
    },
    []
  )

  useEffect(() => {
    fetchBuckets()
  }, [fetchBuckets])

  useEffect(() => {
    fetchRawRecords()
  }, [fetchRawRecords])

  useEffect(() => {
    fetchSummary()
  }, [fetchSummary])

  useEffect(() => {
    fetchDetails(selectedBucket)
  }, [fetchDetails, selectedBucket])

  function updateFilter(key: keyof typeof filters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }))
    setBucketPage(1)
  }

  function openOperation(next: OperationTarget) {
    setOperation(next)
    setReason('')
    setResolveTokenId('')
  }

  async function submitOperation() {
    if (!operation || submitting) return
    if (operation.type === 'skip' && reason.trim() === '') {
      toast.error(t('Reason is required'))
      return
    }
    if (
      (operation.type === 'resolve' || operation.type === 'resolve_bucket') &&
      Number(resolveTokenId) <= 0
    ) {
      toast.error(t('Token ID is required'))
      return
    }
    setSubmitting(true)
    try {
      const response =
        operation.type === 'recalculate'
          ? await recalculateQiniuBillingBucket(operation.bucket.id, reason)
          : operation.type === 'skip'
            ? await skipQiniuBillingBucket(operation.bucket.id, reason)
            : operation.type === 'retry'
              ? await retryQiniuBillingBucketApplication(
                  operation.application.id,
                  reason
                )
              : operation.type === 'resolve_bucket'
                ? await resolveQiniuBillingBucket(
                    operation.bucket.id,
                    Number(resolveTokenId),
                    reason
                  )
                : await resolveQiniuCostDetailRecord(
                    operation.record.id,
                    Number(resolveTokenId),
                    reason
                  )
      if (response.success) {
        toast.success(response.data?.message || t('Operation successful'))
        setOperation(null)
        await Promise.all([
          fetchSummary(),
          fetchBuckets(),
          fetchRawRecords(),
          fetchDetails(selectedBucket),
        ])
      } else {
        toast.error(response.message || t('Operation failed'))
      }
    } finally {
      setSubmitting(false)
    }
  }

  const operationTitle =
    operation?.type === 'recalculate'
      ? t('Recalculate bucket')
      : operation?.type === 'skip'
        ? t('Skip bucket')
        : operation?.type === 'retry'
          ? t('Retry application')
          : operation?.type === 'resolve'
            ? t('Resolve raw record')
            : operation?.type === 'resolve_bucket'
              ? t('Resolve bucket')
              : ''

  function renderRetryMeta(
    retryCount?: number,
    lastRetryTime?: number,
    nextRetryTime?: number
  ) {
    if (!retryCount && !lastRetryTime && !nextRetryTime) return null
    return (
      <div className='text-muted-foreground mt-1 space-y-0.5 text-xs'>
        <div>
          {t('Retries')}: {retryCount ?? 0}
        </div>
        {lastRetryTime ? (
          <div>
            {t('Last retry')}: {formatTime(lastRetryTime)}
          </div>
        ) : null}
        {nextRetryTime ? (
          <div>
            {t('Next retry')}: {formatTime(nextRetryTime)}
          </div>
        ) : null}
      </div>
    )
  }

  return (
    <div className='space-y-4'>
      <div className='grid gap-3 md:grid-cols-4'>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Unmapped records')}
          </div>
          <div className='mt-1 font-mono text-lg'>
            {summaryLoading && !summary ? '-' : (summary?.unmapped_count ?? 0)}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Ambiguous records')}
          </div>
          <div className='mt-1 font-mono text-lg'>
            {summaryLoading && !summary ? '-' : (summary?.ambiguous_count ?? 0)}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Failed applications')}
          </div>
          <div className='mt-1 font-mono text-lg'>
            {summaryLoading && !summary
              ? '-'
              : (summary?.failed_application_count ?? 0)}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Affected amount')}
          </div>
          <div className='mt-1 font-mono text-lg'>
            {formatMoney(summary?.affected_amount ?? 0)}
          </div>
          <div className='text-muted-foreground mt-1 font-mono text-xs'>
            {t('Quota')}: {formatQuota(summary?.affected_quota)}
          </div>
        </div>
      </div>

      <div className='grid gap-3 lg:grid-cols-3'>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Latest sync')}
          </div>
          <div className='mt-1 text-sm'>
            {formatTime(summary?.latest_successful_sync_time)}
          </div>
        </div>
        <div className='rounded-lg border p-3 lg:col-span-2'>
          <div className='text-muted-foreground text-xs'>
            {t('Latest retry result')}
          </div>
          <div className='mt-1 truncate font-mono text-xs'>
            {summary?.latest_retry_result || '-'}
          </div>
          {summary?.latest_error ? (
            <div className='text-muted-foreground mt-1 truncate text-xs'>
              {summary.latest_error}
            </div>
          ) : null}
        </div>
      </div>

      <TableShell
        title={t('Qiniu Billing Buckets')}
        description={t(
          'Review delayed Qiniu cost-detail bucket settlements and application status'
        )}
        loading={bucketLoading}
        onRefresh={() => {
          fetchSummary()
          fetchBuckets()
        }}
        refreshLabel={t('Refresh')}
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap'>
            <FilterInput
              value={filters.billingDate}
              onChange={(value) => updateFilter('billingDate', value)}
              placeholder={t('Billing date')}
            />
            <FilterInput
              value={filters.userId}
              onChange={(value) => updateFilter('userId', value)}
              placeholder={t('User ID')}
            />
            <FilterInput
              value={filters.tokenId}
              onChange={(value) => updateFilter('tokenId', value)}
              placeholder={t('Token ID')}
            />
            <FilterInput
              value={filters.childAccountId}
              onChange={(value) => updateFilter('childAccountId', value)}
              placeholder={t('Child Account ID')}
            />
            <FilterInput
              value={filters.maskedKey}
              onChange={(value) => updateFilter('maskedKey', value)}
              placeholder={t('Masked Key')}
            />
            <StatusFilter
              value={filters.status}
              onChange={(value) => updateFilter('status', value)}
              options={bucketStatusOptions}
              allLabel={t('All Status')}
              getOptionLabel={(value) => getStatusLabel(t, value)}
            />
            <StatusFilter
              value={filters.ownerStatus}
              onChange={(value) => updateFilter('ownerStatus', value)}
              options={ownerStatusOptions}
              allLabel={t('All Owners')}
              getOptionLabel={(value) => getStatusLabel(t, value)}
            />
          </div>
        }
      >
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Bucket')}</TableHead>
                <TableHead>{t('Owner')}</TableHead>
                <TableHead>{t('Masked Key')}</TableHead>
                <TableHead>{t('Official')}</TableHead>
                <TableHead>{t('Local realtime')}</TableHead>
                <TableHead>{t('Pending delta')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {buckets.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={8}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No Qiniu billing buckets found')}
                  </TableCell>
                </TableRow>
              ) : (
                buckets.map((bucket) => (
                  <TableRow key={bucket.id}>
                    <TableCell>
                      <div className='font-mono text-xs'>#{bucket.id}</div>
                      <div className='text-muted-foreground text-xs'>
                        {bucket.billing_date}
                      </div>
                    </TableCell>
                    <TableCell className='text-xs'>
                      <div>
                        {t('User ID')}: {bucket.user_id}
                      </div>
                      <div>
                        {t('Token ID')}: {bucket.token_id}
                      </div>
                      <div>
                        {t('Account')}:{' '}
                        {bucket.qiniu_child_account_id > 0
                          ? `${t('Child Account')} #${bucket.qiniu_child_account_id}`
                          : t('Parent Account')}
                      </div>
                      <StatusBadge
                        label={getStatusLabel(t, bucket.owner_status)}
                      />
                    </TableCell>
                    <TableCell className='max-w-[180px] truncate font-mono text-xs'>
                      {bucket.qiniu_masked_key || '-'}
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      <div>{formatMoney(bucket.official_amount)}</div>
                      <div className='text-muted-foreground'>
                        {formatQuota(bucket.official_quota)}
                      </div>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      <div>{formatQuota(bucket.local_realtime_quota)}</div>
                      <div className='text-muted-foreground'>
                        {getStatusLabel(t, bucket.local_realtime_status)}
                      </div>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      {formatSignedQuota(bucket.pending_delta_quota)}
                    </TableCell>
                    <TableCell>
                      <StatusBadge label={getStatusLabel(t, bucket.status)} />
                      {bucket.last_error ? (
                        <div className='text-muted-foreground mt-1 max-w-[180px] truncate text-xs'>
                          {bucket.last_error}
                        </div>
                      ) : null}
                      {renderRetryMeta(
                        bucket.retry_count,
                        bucket.last_retry_time,
                        bucket.next_retry_time
                      )}
                    </TableCell>
                    <TableCell>
                      <div className='flex flex-wrap gap-1'>
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() => setSelectedBucket(bucket)}
                        >
                          <Eye data-icon='inline-start' />
                          {t('Details')}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() =>
                            openOperation({ type: 'recalculate', bucket })
                          }
                        >
                          <RotateCcw data-icon='inline-start' />
                          {t('Recalculate')}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={[
                            'applied',
                            'reconciled',
                            'skipped',
                          ].includes(bucket.status)}
                          onClick={() =>
                            openOperation({ type: 'skip', bucket })
                          }
                        >
                          <SkipForward data-icon='inline-start' />
                          {t('Skip')}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={['resolved', 'manual_resolved'].includes(
                            bucket.owner_status
                          )}
                          onClick={() =>
                            openOperation({
                              type: 'resolve_bucket',
                              bucket,
                            })
                          }
                        >
                          <Link2 data-icon='inline-start' />
                          {t('Resolve')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <PaginationBar
          page={bucketPage}
          pageSize={ADMIN_FINANCE_PAGE_SIZE}
          total={bucketTotal}
          loading={bucketLoading}
          onPageChange={setBucketPage}
          t={t}
        />
        {selectedBucket ? (
          <div className='space-y-4 border-t p-4'>
            <div className='flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between'>
              <div>
                <div className='text-sm font-medium'>
                  {t('Bucket Details')} #{selectedBucket.id}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {selectedBucket.billing_date} ·{' '}
                  {selectedBucket.qiniu_masked_key}
                </div>
              </div>
              {detailsLoading ? (
                <span className='text-muted-foreground text-xs'>
                  {t('Loading...')}
                </span>
              ) : null}
            </div>

            <div className='grid gap-4 xl:grid-cols-2'>
              <div className='overflow-x-auto rounded-md border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Model')}</TableHead>
                      <TableHead>{t('Billing item')}</TableHead>
                      <TableHead>{t('Usage')}</TableHead>
                      <TableHead>{t('Fee')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {items.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={4}
                          className='text-muted-foreground h-20 text-center'
                        >
                          {t('No bucket items found')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      items.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell>{item.model_name || '-'}</TableCell>
                          <TableCell>{item.billing_item || '-'}</TableCell>
                          <TableCell className='font-mono'>
                            {item.usage_count.toLocaleString()}
                          </TableCell>
                          <TableCell className='font-mono'>
                            {formatMoney(item.fee_amount)} {item.currency}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>

              <div className='overflow-x-auto rounded-md border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Version')}</TableHead>
                      <TableHead>{t('Delta')}</TableHead>
                      <TableHead>{t('Debt')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {applications.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className='text-muted-foreground h-20 text-center'
                        >
                          {t('No bucket applications found')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      applications.map((application) => (
                        <TableRow key={application.id}>
                          <TableCell className='font-mono'>
                            v{application.apply_version}
                          </TableCell>
                          <TableCell className='font-mono'>
                            {formatSignedQuota(application.delta_quota)}
                          </TableCell>
                          <TableCell className='font-mono'>
                            {formatQuota(application.debt_quota)}
                          </TableCell>
                          <TableCell>
                            <StatusBadge
                              label={getStatusLabel(t, application.status)}
                            />
                            {application.last_error ? (
                              <div className='text-muted-foreground mt-1 max-w-[180px] truncate text-xs'>
                                {application.last_error}
                              </div>
                            ) : null}
                            {renderRetryMeta(
                              application.retry_count,
                              application.last_retry_time,
                              application.next_retry_time
                            )}
                          </TableCell>
                          <TableCell>
                            <Button
                              size='sm'
                              variant='outline'
                              disabled={application.status !== 'failed'}
                              onClick={() =>
                                openOperation({
                                  type: 'retry',
                                  application,
                                })
                              }
                            >
                              <Play data-icon='inline-start' />
                              {t('Retry')}
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </div>
          </div>
        ) : null}
      </TableShell>

      <TableShell
        title={t('Qiniu Raw Records')}
        description={t(
          'Resolve unmapped or ambiguous cost-detail records before settlement'
        )}
        loading={rawLoading}
        onRefresh={() => {
          fetchSummary()
          fetchRawRecords()
        }}
        refreshLabel={t('Refresh')}
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap'>
            <StatusFilter
              value={rawOwnerStatus}
              onChange={(value) => {
                setRawOwnerStatus(value)
                setRawPage(1)
              }}
              options={ownerStatusOptions}
              allLabel={t('All Owners')}
              getOptionLabel={(value) => getStatusLabel(t, value)}
            />
            <FilterInput
              value={rawChildAccountId}
              onChange={(value) => {
                setRawChildAccountId(value)
                setRawPage(1)
              }}
              placeholder={t('Child Account ID')}
            />
          </div>
        }
      >
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Raw Record')}</TableHead>
                <TableHead>{t('Masked Key')}</TableHead>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Fee')}</TableHead>
                <TableHead>{t('Owner')}</TableHead>
                <TableHead>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rawRecords.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No Qiniu raw records found')}
                  </TableCell>
                </TableRow>
              ) : (
                rawRecords.map((record) => (
                  <TableRow key={record.id}>
                    <TableCell>
                      <div className='font-mono text-xs'>#{record.id}</div>
                      <div className='text-muted-foreground text-xs'>
                        {record.billing_date}
                      </div>
                    </TableCell>
                    <TableCell className='max-w-[180px] truncate font-mono text-xs'>
                      {record.qiniu_masked_key || '-'}
                    </TableCell>
                    <TableCell>
                      <div>{record.model_name || '-'}</div>
                      <div className='text-muted-foreground text-xs'>
                        {record.billing_item || '-'}
                      </div>
                    </TableCell>
                    <TableCell className='font-mono'>
                      {formatMoney(record.fee_amount)} {record.currency}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={getStatusLabel(t, record.owner_status)}
                      />
                      {record.token_id > 0 ? (
                        <div className='text-muted-foreground mt-1 text-xs'>
                          {t('Token ID')}: {record.token_id}
                        </div>
                      ) : null}
                      <div className='text-muted-foreground mt-1 text-xs'>
                        {t('Account')}:{' '}
                        {record.qiniu_child_account_id > 0
                          ? `${t('Child Account')} #${record.qiniu_child_account_id}`
                          : t('Parent Account')}
                      </div>
                      {record.last_error ? (
                        <div className='text-muted-foreground mt-1 max-w-[180px] truncate text-xs'>
                          {record.last_error}
                        </div>
                      ) : null}
                      {renderRetryMeta(
                        record.retry_count,
                        record.last_retry_time,
                        record.next_retry_time
                      )}
                    </TableCell>
                    <TableCell>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={['resolved', 'manual_resolved'].includes(
                          record.owner_status
                        )}
                        onClick={() =>
                          openOperation({ type: 'resolve', record })
                        }
                      >
                        <Link2 data-icon='inline-start' />
                        {t('Resolve')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <PaginationBar
          page={rawPage}
          pageSize={ADMIN_FINANCE_PAGE_SIZE}
          total={rawTotal}
          loading={rawLoading}
          onPageChange={setRawPage}
          t={t}
        />
      </TableShell>

      <Dialog
        open={!!operation}
        onOpenChange={(open) => !open && setOperation(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{operationTitle}</DialogTitle>
          </DialogHeader>
          <div className='space-y-3'>
            {operation?.type === 'resolve' ||
            operation?.type === 'resolve_bucket' ? (
              <Input
                type='number'
                min={1}
                value={resolveTokenId}
                onChange={(event) => setResolveTokenId(event.target.value)}
                placeholder={t('Target token ID')}
              />
            ) : null}
            <Textarea
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder={t('Operation reason')}
              rows={3}
            />
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setOperation(null)}
              disabled={submitting}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={submitOperation} disabled={submitting}>
              {submitting ? t('Saving...') : t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
