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
import { Eye, Plus, Power, PowerOff, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  createQiniuChildAccount,
  disableQiniuChildAccount,
  enableQiniuChildAccount,
  getQiniuChildAccountDetail,
  getQiniuChildAccounts,
  retryQiniuChildAccountTask,
} from '../api'
import type {
  QiniuChildAccount,
  QiniuChildAccountDetail,
  QiniuChildAccountTask,
} from '../types'
import {
  ADMIN_FINANCE_PAGE_SIZE,
  FilterInput,
  PaginationBar,
  StatusBadge,
  StatusFilter,
  TableShell,
  formatTime,
} from './shared'

const accountStatusOptions = ['creating', 'enabled', 'disabled', 'failed']

function getChildAccountStatusLabel(t: TFunction, status?: string) {
  const map: Record<string, string> = {
    creating: 'Creating',
    enabled: 'Enabled',
    disabled: 'Disabled',
    failed: 'Failed',
    pending: 'Pending',
    running: 'Running',
    success: 'Success',
    skipped: 'Skipped',
  }
  return t(map[status || ''] || status || 'Unknown')
}

function getTokenStatusLabel(t: TFunction, status?: number | string) {
  const map: Record<string, string> = {
    '1': 'Enabled',
    '2': 'Disabled',
    '3': 'Expired',
    '4': 'Exhausted',
  }
  return t(map[String(status ?? '')] || 'Unknown')
}

function canDisable(account: QiniuChildAccount) {
  return account.status === 'enabled'
}

function canEnable(account: QiniuChildAccount) {
  return account.status === 'disabled' || account.status === 'failed'
}

function canRetry(task?: QiniuChildAccountTask | null) {
  return task?.status === 'failed'
}

export function getQiniuChildAccountImpactSummary(
  account: QiniuChildAccount,
  t: TFunction
) {
  return `${t('Users')}: ${account.user_count} · ${t('Tokens')}: ${
    account.impact?.associated_token_count ?? 0
  } · ${t('Enabled Tokens')}: ${account.impact?.enabled_token_count ?? 0}`
}

export function QiniuChildAccountsTable() {
  const { t } = useTranslation()
  const [accounts, setAccounts] = useState<QiniuChildAccount[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [detail, setDetail] = useState<QiniuChildAccountDetail | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const [operationTarget, setOperationTarget] =
    useState<QiniuChildAccount | null>(null)
  const [operationType, setOperationType] = useState<
    'enable' | 'disable' | null
  >(null)
  const [operationReason, setOperationReason] = useState('')
  const [operationSubmitting, setOperationSubmitting] = useState(false)
  const [filters, setFilters] = useState({
    id: '',
    email: '',
    uid: '',
    status: '',
  })

  const fetchAccounts = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getQiniuChildAccounts({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        ...filters,
      })
      if (response.success && response.data) {
        setAccounts(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setLoading(false)
    }
  }, [filters, page, t])

  useEffect(() => {
    fetchAccounts()
  }, [fetchAccounts])

  function updateFilter(key: keyof typeof filters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }))
    setPage(1)
  }

  const handleCreate = async () => {
    setCreating(true)
    try {
      const response = await createQiniuChildAccount()
      if (response.success) {
        toast.success(t('Operation successful'))
        await fetchAccounts()
      } else {
        toast.error(response.message || t('Operation failed'))
      }
    } finally {
      setCreating(false)
    }
  }

  const openDetail = async (account: QiniuChildAccount) => {
    const response = await getQiniuChildAccountDetail(account.id)
    if (response.success && response.data) {
      setDetail(response.data)
      setDetailOpen(true)
    } else {
      toast.error(response.message || t('Failed to load records'))
    }
  }

  const openOperation = (
    account: QiniuChildAccount,
    type: 'enable' | 'disable'
  ) => {
    setOperationTarget(account)
    setOperationType(type)
    setOperationReason('')
  }

  const handleOperation = async () => {
    if (!operationTarget || !operationType) return
    setOperationSubmitting(true)
    try {
      const response =
        operationType === 'disable'
          ? await disableQiniuChildAccount(
              operationTarget.id,
              operationReason.trim()
            )
          : await enableQiniuChildAccount(operationTarget.id)
      if (response.success) {
        toast.success(t('Operation successful'))
        setOperationTarget(null)
        setOperationType(null)
        setOperationReason('')
        await fetchAccounts()
      } else {
        toast.error(response.message || t('Operation failed'))
      }
    } finally {
      setOperationSubmitting(false)
    }
  }

  const handleRetry = async (task: QiniuChildAccountTask) => {
    const response = await retryQiniuChildAccountTask(task.id)
    if (response.success) {
      toast.success(t('Operation successful'))
      await fetchAccounts()
      if (detail) {
        await openDetail(detail)
      }
    } else {
      toast.error(response.message || t('Operation failed'))
    }
  }

  return (
    <>
      <TableShell
        title={t('Qiniu Child Accounts')}
        description={t(
          'Manage Qiniu OEM child accounts and their create or lifecycle tasks'
        )}
        loading={loading}
        onRefresh={fetchAccounts}
        refreshLabel={t('Refresh')}
        actions={
          <Button size='sm' onClick={handleCreate} disabled={creating}>
            <Plus data-icon='inline-start' />
            {creating ? t('Creating...') : t('Create')}
          </Button>
        }
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center'>
            <FilterInput
              value={filters.id}
              onChange={(value) => updateFilter('id', value)}
              placeholder={t('Account ID')}
              className='w-full sm:w-36'
            />
            <FilterInput
              value={filters.email}
              onChange={(value) => updateFilter('email', value)}
              placeholder={t('Email')}
            />
            <FilterInput
              value={filters.uid}
              onChange={(value) => updateFilter('uid', value)}
              placeholder={t('UID')}
            />
            <StatusFilter
              value={filters.status}
              onChange={(value) => updateFilter('status', value)}
              options={accountStatusOptions}
              allLabel={t('All Status')}
              getOptionLabel={(value) => getChildAccountStatusLabel(t, value)}
            />
          </div>
        }
      >
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Account')}</TableHead>
                <TableHead>{t('UID')}</TableHead>
                <TableHead>{t('Access Key')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Impact')}</TableHead>
                <TableHead>{t('Latest Task')}</TableHead>
                <TableHead>{t('Updated At')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {accounts.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={8}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No child accounts found')}
                  </TableCell>
                </TableRow>
              ) : (
                accounts.map((account) => (
                  <TableRow key={account.id}>
                    <TableCell className='min-w-[220px]'>
                      <div className='font-medium'>{account.email}</div>
                      <div className='text-muted-foreground text-xs'>
                        #{account.id} · {t('Sequence')} {account.sequence_no}
                      </div>
                    </TableCell>
                    <TableCell className='min-w-[180px] font-mono text-xs'>
                      {account.uid || '-'}
                    </TableCell>
                    <TableCell className='min-w-[160px] font-mono text-xs'>
                      {account.access_key || '-'}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={getChildAccountStatusLabel(t, account.status)}
                      />
                    </TableCell>
                    <TableCell className='min-w-[150px] text-xs'>
                      <div>
                        {t('Users')}: {account.user_count}
                      </div>
                      <div className='text-muted-foreground'>
                        {t('Enabled Tokens')}:{' '}
                        {account.impact?.enabled_token_count ?? 0}
                      </div>
                    </TableCell>
                    <TableCell className='min-w-[180px]'>
                      {account.latest_task ? (
                        <div className='space-y-1'>
                          <StatusBadge
                            label={getChildAccountStatusLabel(
                              t,
                              account.latest_task.status
                            )}
                          />
                          <div className='text-muted-foreground text-xs'>
                            {account.latest_task.task_type}
                          </div>
                          {account.latest_task.last_error ? (
                            <div className='text-destructive max-w-[220px] truncate text-xs'>
                              {account.latest_task.last_error}
                            </div>
                          ) : null}
                        </div>
                      ) : (
                        '-'
                      )}
                    </TableCell>
                    <TableCell className='min-w-[150px] text-xs'>
                      {formatTime(account.updated_time)}
                    </TableCell>
                    <TableCell className='min-w-[220px]'>
                      <div className='flex justify-end gap-2'>
                        <Button
                          variant='outline'
                          size='icon-sm'
                          title={t('View details')}
                          onClick={() => openDetail(account)}
                        >
                          <Eye />
                        </Button>
                        {canRetry(account.latest_task) ? (
                          <Button
                            variant='outline'
                            size='icon-sm'
                            title={t('Retry')}
                            onClick={() => handleRetry(account.latest_task!)}
                          >
                            <RotateCcw />
                          </Button>
                        ) : null}
                        <Button
                          variant='outline'
                          size='icon-sm'
                          title={t('Enable')}
                          disabled={!canEnable(account)}
                          onClick={() => openOperation(account, 'enable')}
                        >
                          <Power />
                        </Button>
                        <Button
                          variant='outline'
                          size='icon-sm'
                          title={t('Disable')}
                          disabled={!canDisable(account)}
                          onClick={() => openOperation(account, 'disable')}
                        >
                          <PowerOff />
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
          page={page}
          pageSize={ADMIN_FINANCE_PAGE_SIZE}
          total={total}
          loading={loading}
          onPageChange={setPage}
          t={t}
        />
      </TableShell>

      <Sheet open={detailOpen} onOpenChange={setDetailOpen}>
        <SheetContent className='w-full overflow-y-auto sm:max-w-2xl'>
          <SheetHeader>
            <SheetTitle>{t('Qiniu Child Account')}</SheetTitle>
            <SheetDescription>{detail?.email || '-'}</SheetDescription>
          </SheetHeader>
          {detail ? <ChildAccountDetail detail={detail} t={t} /> : null}
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={!!operationTarget && !!operationType}
        onOpenChange={(open) => {
          if (open || operationSubmitting) return
          setOperationTarget(null)
          setOperationType(null)
          setOperationReason('')
        }}
        title={
          operationType === 'disable'
            ? t('Disable child account')
            : t('Enable child account')
        }
        desc={
          operationTarget
            ? `${operationTarget.email} · ${operationTarget.uid || '-'}`
            : ''
        }
        confirmText={operationSubmitting ? t('Saving...') : t('Continue')}
        destructive={operationType === 'disable'}
        disabled={operationType === 'disable' && !operationReason.trim()}
        isLoading={operationSubmitting}
        handleConfirm={handleOperation}
      >
        {operationType === 'disable' ? (
          <div className='space-y-3'>
            {operationTarget ? (
              <div className='text-muted-foreground rounded-md border p-3 text-sm'>
                {getQiniuChildAccountImpactSummary(operationTarget, t)}
              </div>
            ) : null}
            <Textarea
              value={operationReason}
              onChange={(event) => setOperationReason(event.target.value)}
              placeholder={t('Reason')}
              className='min-h-24'
            />
          </div>
        ) : null}
      </ConfirmDialog>
    </>
  )
}

export function ChildAccountDetail({
  detail,
  t,
}: {
  detail: QiniuChildAccountDetail
  t: TFunction
}) {
  return (
    <div className='space-y-5 px-4 pb-6'>
      <div className='grid gap-3 sm:grid-cols-2'>
        <DetailItem label={t('Status')} value={detail.status} />
        <DetailItem label={t('Sequence')} value={detail.sequence_no} />
        <DetailItem
          label={t('Users')}
          value={detail.impact.associated_user_count}
        />
        <DetailItem
          label={t('Enabled Tokens')}
          value={detail.impact.enabled_token_count}
        />
        <DetailItem label={t('UID')} value={detail.uid || '-'} />
        <DetailItem label={t('Parent UID')} value={detail.parent_uid || '-'} />
        <DetailItem label={t('Access Key')} value={detail.access_key || '-'} />
        <DetailItem
          label={t('Created At')}
          value={formatTime(detail.created_time)}
        />
      </div>

      <div>
        <div className='mb-2 flex items-center justify-between'>
          <h4 className='text-sm font-medium'>{t('Users')}</h4>
          <Badge variant='outline'>{detail.user_count}</Badge>
        </div>
        <div className='text-muted-foreground rounded-md border p-3 text-sm'>
          {detail.users.length === 0
            ? t('No users are bound')
            : detail.users
                .map((user) => user.email || user.username)
                .join(', ')}
        </div>
      </div>

      <div>
        <div className='mb-2 flex items-center justify-between'>
          <h4 className='text-sm font-medium'>{t('Tokens')}</h4>
          <Badge variant='outline'>
            {detail.impact.associated_token_count}
          </Badge>
        </div>
        <div className='overflow-x-auto rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Token')}</TableHead>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Deleted')}</TableHead>
                <TableHead>{t('Cleanup')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {detail.tokens.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-muted-foreground h-20 text-center'
                  >
                    {t('No tokens found')}
                  </TableCell>
                </TableRow>
              ) : (
                detail.tokens.map((token) => (
                  <TableRow key={token.id}>
                    <TableCell className='min-w-[160px]'>
                      <div className='font-mono text-xs'>#{token.id}</div>
                      <div className='max-w-[180px] truncate text-sm'>
                        {token.name || '-'}
                      </div>
                      <div className='text-muted-foreground font-mono text-xs'>
                        {token.key_fingerprint || '-'}
                      </div>
                    </TableCell>
                    <TableCell className='min-w-[150px]'>
                      <div className='max-w-[180px] truncate text-sm'>
                        {token.display_name || token.username || '-'}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        #{token.user_id}
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={getTokenStatusLabel(t, token.status)}
                      />
                    </TableCell>
                    <TableCell>
                      {token.deleted ? t('Deleted') : t('Active')}
                    </TableCell>
                    <TableCell className='text-xs'>
                      {token.remote_cleanup_result || '-'}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      <div>
        <h4 className='mb-2 text-sm font-medium'>{t('Tasks')}</h4>
        <div className='overflow-x-auto rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Task')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Retry')}</TableHead>
                <TableHead>{t('Updated At')}</TableHead>
                <TableHead>{t('Error')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {detail.tasks.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-muted-foreground h-20 text-center'
                  >
                    {t('No tasks found')}
                  </TableCell>
                </TableRow>
              ) : (
                detail.tasks.map((task) => (
                  <TableRow key={task.id}>
                    <TableCell className='font-mono text-xs'>
                      {task.task_type}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={getChildAccountStatusLabel(t, task.status)}
                      />
                    </TableCell>
                    <TableCell>{task.retry_count}</TableCell>
                    <TableCell>{formatTime(task.updated_time)}</TableCell>
                    <TableCell className='max-w-[260px] truncate text-xs'>
                      {task.last_error || '-'}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>
  )
}

function DetailItem({
  label,
  value,
}: {
  label: string
  value: string | number
}) {
  return (
    <div className='rounded-md border p-3'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-1 text-sm font-medium break-all'>{value}</div>
    </div>
  )
}
