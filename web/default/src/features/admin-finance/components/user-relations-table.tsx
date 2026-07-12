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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  createAdminUserRelation,
  disableAdminUserRelation,
  getAdminUserRelations,
} from '../api'
import type { UserRelation } from '../types'
import {
  ADMIN_FINANCE_PAGE_SIZE,
  FilterInput,
  PaginationBar,
  StatusBadge,
  StatusFilter,
  TableShell,
  formatTime,
  getStatusLabel,
} from './shared'

export function UserRelationsTable() {
  const { t } = useTranslation()
  const [items, setItems] = useState<UserRelation[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [parentUserId, setParentUserId] = useState('')
  const [childUserId, setChildUserId] = useState('')
  const [status, setStatus] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState({
    parentUserId: '',
    childUserId: '',
    sourceTradeNo: '',
    remark: '',
  })
  const [disableTarget, setDisableTarget] = useState<UserRelation | null>(null)
  const [disableReason, setDisableReason] = useState('')

  const fetchItems = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getAdminUserRelations({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        parentUserId,
        childUserId,
        status,
      })
      if (response.success && response.data) {
        setItems(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setLoading(false)
    }
  }, [childUserId, page, parentUserId, status, t])

  useEffect(() => {
    fetchItems()
  }, [fetchItems])

  const resetCreateForm = () => {
    setCreateForm({
      parentUserId: '',
      childUserId: '',
      sourceTradeNo: '',
      remark: '',
    })
  }

  const handleCreate = async () => {
    const parentId = Number(createForm.parentUserId)
    const childId = Number(createForm.childUserId)
    if (!Number.isInteger(parentId) || parentId <= 0) {
      toast.error(t('Please enter a valid parent user ID'))
      return
    }
    if (!Number.isInteger(childId) || childId <= 0) {
      toast.error(t('Please enter a valid child user ID'))
      return
    }
    const response = await createAdminUserRelation({
      parent_user_id: parentId,
      child_user_id: childId,
      source_trade_no: createForm.sourceTradeNo.trim(),
      remark: createForm.remark.trim(),
    })
    if (response.success) {
      toast.success(t('Operation successful'))
      setCreateOpen(false)
      resetCreateForm()
      setPage(1)
      fetchItems()
      return
    }
    toast.error(response.message || t('Operation failed'))
  }

  const handleDisable = async () => {
    if (!disableTarget) return
    const response = await disableAdminUserRelation(
      disableTarget.id,
      disableReason.trim() || 'Disabled by administrator'
    )
    if (response.success) {
      toast.success(t('Operation successful'))
      setDisableTarget(null)
      setDisableReason('')
      fetchItems()
      return
    }
    toast.error(response.message || t('Operation failed'))
  }

  return (
    <>
      <TableShell
        title={t('Relations')}
        description={t('Review and adjust VVIP invitation relations')}
        loading={loading}
        onRefresh={fetchItems}
        refreshLabel={t('Refresh')}
        actions={
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            {t('Bind Relation')}
          </Button>
        }
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap'>
            <FilterInput
              value={parentUserId}
              onChange={(value) => {
                setParentUserId(value)
                setPage(1)
              }}
              placeholder={t('Parent user ID')}
            />
            <FilterInput
              value={childUserId}
              onChange={(value) => {
                setChildUserId(value)
                setPage(1)
              }}
              placeholder={t('Child user ID')}
            />
            <StatusFilter
              value={status}
              onChange={(value) => {
                setStatus(value)
                setPage(1)
              }}
              options={['active', 'disabled']}
              allLabel={t('All Status')}
              getOptionLabel={(value) => getStatusLabel(t, value)}
            />
          </div>
        }
      >
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Parent User')}</TableHead>
                <TableHead>{t('Child User')}</TableHead>
                <TableHead>{t('Source')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Bind Time')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No relation records found')}
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.parent_user_id}</TableCell>
                    <TableCell>{item.child_user_id}</TableCell>
                    <TableCell>
                      <div>{item.source || '-'}</div>
                      <div className='text-muted-foreground max-w-[220px] truncate font-mono text-xs'>
                        {item.source_trade_no || '-'}
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge label={getStatusLabel(t, item.status)} />
                    </TableCell>
                    <TableCell>{formatTime(item.bind_time)}</TableCell>
                    <TableCell className='text-right'>
                      {item.status === 'active' ? (
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() => setDisableTarget(item)}
                        >
                          {t('Disable')}
                        </Button>
                      ) : (
                        <span className='text-muted-foreground text-xs'>-</span>
                      )}
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

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Bind Relation')}</DialogTitle>
            <DialogDescription>
              {t('Bind a child user to an active paid VVIP parent.')}
            </DialogDescription>
          </DialogHeader>
          <div className='grid gap-3'>
            <Input
              value={createForm.parentUserId}
              onChange={(event) =>
                setCreateForm((form) => ({
                  ...form,
                  parentUserId: event.target.value,
                }))
              }
              placeholder={t('Parent user ID')}
            />
            <Input
              value={createForm.childUserId}
              onChange={(event) =>
                setCreateForm((form) => ({
                  ...form,
                  childUserId: event.target.value,
                }))
              }
              placeholder={t('Child user ID')}
            />
            <Input
              value={createForm.sourceTradeNo}
              onChange={(event) =>
                setCreateForm((form) => ({
                  ...form,
                  sourceTradeNo: event.target.value,
                }))
              }
              placeholder={t('Source trade no.')}
            />
            <Input
              value={createForm.remark}
              onChange={(event) =>
                setCreateForm((form) => ({
                  ...form,
                  remark: event.target.value,
                }))
              }
              placeholder={t('Remark')}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setCreateOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={handleCreate}>{t('Confirm')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={Boolean(disableTarget)}
        onOpenChange={(open) => !open && setDisableTarget(null)}
        title={t('Disable Relation')}
        desc={t('Disable this invitation relation while keeping its history.')}
        confirmText={t('Disable')}
        handleConfirm={handleDisable}
      >
        <Input
          value={disableReason}
          onChange={(event) => setDisableReason(event.target.value)}
          placeholder={t('Disable reason')}
        />
      </ConfirmDialog>
    </>
  )
}
