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
import type { TFunction } from 'i18next'
import { Ban } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { disableQiniuKey, getQiniuKeys } from '../api'
import type { QiniuKeyListItem } from '../types'
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

const tokenStatusOptions = ['1', '2', '3', '4']

function getTokenStatusLabel(t: TFunction, status?: number | string) {
  const map: Record<string, string> = {
    '1': 'Enabled',
    '2': 'Disabled',
    '3': 'Expired',
    '4': 'Exhausted',
  }
  return t(map[String(status ?? '')] || 'Unknown')
}

function formatOwner(key: QiniuKeyListItem) {
  if (key.user.display_name) return key.user.display_name
  if (key.user.username) return key.user.username
  if (key.user.email) return key.user.email
  return ''
}

function renderLimitAmount(label: string, value?: number) {
  return (
    <div className='flex items-center justify-between gap-3 font-mono'>
      <span className='text-muted-foreground font-sans'>{label}</span>
      <span>{formatMoney(value)}</span>
    </div>
  )
}

function canDisableQiniuKey(key: QiniuKeyListItem) {
  return key.status === 1 && !key.deleted
}

export function renderQiniuKeyAccountOwnership(
  key: QiniuKeyListItem,
  t: TFunction
) {
  if (key.qiniu_child_account_id > 0) {
    return (
      <div className='space-y-1'>
        <StatusBadge label={t('Child Account')} />
        <div className='font-mono text-xs'>#{key.qiniu_child_account_id}</div>
        <div className='text-muted-foreground max-w-[220px] truncate text-xs'>
          {key.qiniu_child_account?.email ||
            key.qiniu_child_account?.uid ||
            '-'}
        </div>
      </div>
    )
  }

  return <StatusBadge label={t('Parent Account')} />
}

export function QiniuKeysTable() {
  const { t } = useTranslation()
  const [keys, setKeys] = useState<QiniuKeyListItem[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [disableTarget, setDisableTarget] = useState<QiniuKeyListItem | null>(
    null
  )
  const [disableReason, setDisableReason] = useState('')
  const [disableSubmitting, setDisableSubmitting] = useState(false)
  const [filters, setFilters] = useState({
    userId: '',
    tokenId: '',
    childAccountId: '',
    status: '',
    keyFragment: '',
    includeDeleted: false,
  })

  const fetchKeys = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getQiniuKeys({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        ...filters,
      })
      if (response.success && response.data) {
        setKeys(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setLoading(false)
    }
  }, [filters, page, t])

  useEffect(() => {
    fetchKeys()
  }, [fetchKeys])

  function updateFilter(key: keyof typeof filters, value: string | boolean) {
    setFilters((current) => ({ ...current, [key]: value }))
    setPage(1)
  }

  const handleDisable = async () => {
    if (!disableTarget) return
    setDisableSubmitting(true)
    try {
      const response = await disableQiniuKey(
        disableTarget.token_id,
        disableReason
      )
      if (response.success) {
        toast.success(t('Operation successful'))
        setDisableTarget(null)
        setDisableReason('')
        await fetchKeys()
      }
    } finally {
      setDisableSubmitting(false)
    }
  }

  const handleDisableDialogOpenChange = (open: boolean) => {
    if (open || disableSubmitting) return
    setDisableTarget(null)
    setDisableReason('')
  }

  return (
    <>
      <TableShell
        title={t('Qiniu Keys')}
        description={t(
          'Review Qiniu-managed keys, owners, local limits, and lifecycle task status'
        )}
        loading={loading}
        onRefresh={fetchKeys}
        refreshLabel={t('Refresh')}
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center'>
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
              value={filters.keyFragment}
              onChange={(value) => updateFilter('keyFragment', value)}
              placeholder={t('Key fragment')}
            />
            <StatusFilter
              value={filters.status}
              onChange={(value) => updateFilter('status', value)}
              options={tokenStatusOptions}
              allLabel={t('All Status')}
              getOptionLabel={(value) => getTokenStatusLabel(t, value)}
            />
            <div className='flex h-9 items-center gap-2 px-1'>
              <Checkbox
                id='include-deleted-qiniu-keys'
                checked={filters.includeDeleted}
                onCheckedChange={(checked) =>
                  updateFilter('includeDeleted', checked === true)
                }
              />
              <Label
                htmlFor='include-deleted-qiniu-keys'
                className='text-sm font-normal whitespace-nowrap'
              >
                {t('Include deleted')}
              </Label>
            </div>
          </div>
        }
      >
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Token')}</TableHead>
                <TableHead>{t('Key')}</TableHead>
                <TableHead>{t('Account')}</TableHead>
                <TableHead>{t('Local Status')}</TableHead>
                <TableHead>{t('Deleted')}</TableHead>
                <TableHead>{t('Limit Summary')}</TableHead>
                <TableHead>{t('Latest Task')}</TableHead>
                <TableHead>{t('Created At')}</TableHead>
                <TableHead>{t('Accessed At')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {keys.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={11}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No Qiniu keys found')}
                  </TableCell>
                </TableRow>
              ) : (
                keys.map((key) => {
                  const owner = formatOwner(key)
                  const latestTask = key.latest_task
                  return (
                    <TableRow key={key.token_id}>
                      <TableCell className='min-w-[160px]'>
                        <div className='font-medium'>
                          {owner || t('Unknown')}
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          {t('User ID')}: {key.user_id}
                        </div>
                        {key.user.email ? (
                          <div className='text-muted-foreground max-w-[220px] truncate text-xs'>
                            {key.user.email}
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell className='min-w-[160px]'>
                        <div className='font-mono text-xs'>#{key.token_id}</div>
                        <div className='max-w-[220px] truncate text-sm'>
                          {key.name || '-'}
                        </div>
                        {key.group ? (
                          <div className='text-muted-foreground text-xs'>
                            {key.group}
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell className='max-w-[220px] truncate font-mono text-xs'>
                        {key.key || '-'}
                      </TableCell>
                      <TableCell className='min-w-[180px]'>
                        {renderQiniuKeyAccountOwnership(key, t)}
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          label={getTokenStatusLabel(t, key.status)}
                        />
                      </TableCell>
                      <TableCell className='min-w-[120px]'>
                        <StatusBadge
                          label={key.deleted ? t('Deleted') : t('Active')}
                        />
                        {key.deleted ? (
                          <div className='text-muted-foreground mt-1 text-xs'>
                            {t('Deleted At')}: {formatTime(key.deleted_time)}
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell className='min-w-[190px] text-xs'>
                        {renderLimitAmount(
                          t('Applied Limit'),
                          key.quota.applied_limit_amount
                        )}
                        {renderLimitAmount(
                          t('Pending Limit'),
                          key.quota.pending_limit_amount
                        )}
                        {renderLimitAmount(
                          t('Failed Limit'),
                          key.quota.failed_limit_amount
                        )}
                        {key.quota.latest_grant_error ? (
                          <div className='text-muted-foreground mt-1 max-w-[220px] truncate'>
                            {t('Latest grant error')}:{' '}
                            {key.quota.latest_grant_error}
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell className='min-w-[170px]'>
                        {latestTask ? (
                          <>
                            <StatusBadge
                              label={getStatusLabel(t, latestTask.status)}
                            />
                            <div className='text-muted-foreground mt-1 text-xs'>
                              {latestTask.task_type || '-'} · {t('Retry')}{' '}
                              {latestTask.retry_count}
                            </div>
                            {latestTask.last_error ? (
                              <div className='text-muted-foreground mt-1 max-w-[220px] truncate text-xs'>
                                {latestTask.last_error}
                              </div>
                            ) : null}
                          </>
                        ) : (
                          <span className='text-muted-foreground text-sm'>
                            -
                          </span>
                        )}
                      </TableCell>
                      <TableCell className='min-w-[150px] text-xs'>
                        {formatTime(key.created_time)}
                      </TableCell>
                      <TableCell className='min-w-[150px] text-xs'>
                        {formatTime(key.accessed_time)}
                      </TableCell>
                      <TableCell className='min-w-[120px] text-right'>
                        {canDisableQiniuKey(key) ? (
                          <Button
                            variant='destructive'
                            size='sm'
                            className='gap-1'
                            onClick={() => setDisableTarget(key)}
                          >
                            <Ban className='h-4 w-4' />
                            {t('Disable')}
                          </Button>
                        ) : (
                          <span className='text-muted-foreground text-sm'>
                            -
                          </span>
                        )}
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>
        <PaginationBar
          page={page}
          pageSize={ADMIN_FINANCE_PAGE_SIZE}
          total={total}
          loading={loading}
          onPageChange={setPage}
          t={t}
        />
      </TableShell>

      <ConfirmDialog
        open={Boolean(disableTarget)}
        onOpenChange={handleDisableDialogOpenChange}
        title={t('Disable Qiniu Key')}
        desc={t(
          'Disable this Qiniu-managed key after the remote provider confirms it.'
        )}
        confirmText={t('Disable')}
        destructive
        isLoading={disableSubmitting}
        handleConfirm={handleDisable}
      >
        <Input
          value={disableReason}
          onChange={(event) => setDisableReason(event.target.value)}
          placeholder={t('Disable reason')}
          disabled={disableSubmitting}
        />
      </ConfirmDialog>
    </>
  )
}
