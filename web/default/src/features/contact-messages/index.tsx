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
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
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
import { SectionPageLayout } from '@/components/layout'
import {
  deleteContactMessage,
  getContactMessages,
  updateContactMessage,
} from './api'
import type { ContactMessage, ContactMessageStatus } from './types'

const CONTACT_MESSAGES_PAGE_SIZE = 20
const CONTACT_MESSAGE_STATUSES: ContactMessageStatus[] = [
  'pending',
  'contacted',
  'unreachable',
]

function formatTime(timestamp?: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function getContactMessageStatusLabel(t: TFunction, status?: string) {
  const map: Record<string, string> = {
    pending: 'Pending contact',
    contacted: 'Contacted',
    unreachable: 'Unreachable',
  }
  return t(map[status || ''] || 'Unknown')
}

function PaginationBar(props: {
  page: number
  pageSize: number
  total: number
  loading?: boolean
  onPageChange: (page: number) => void
  t: TFunction
}) {
  const pageCount = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <div className='flex flex-col gap-2 border-t p-3 text-sm sm:flex-row sm:items-center sm:justify-between'>
      <div className='text-muted-foreground'>
        {props.t('Total {{count}} records', { count: props.total })}
      </div>
      <div className='flex items-center gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={props.loading || props.page <= 1}
          onClick={() => props.onPageChange(props.page - 1)}
        >
          {props.t('Previous')}
        </Button>
        <span className='text-muted-foreground min-w-20 text-center'>
          {props.page} / {pageCount}
        </span>
        <Button
          variant='outline'
          size='sm'
          disabled={props.loading || props.page >= pageCount}
          onClick={() => props.onPageChange(props.page + 1)}
        >
          {props.t('Next')}
        </Button>
      </div>
    </div>
  )
}

function EditContactMessageDialog(props: {
  message: ContactMessage | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [status, setStatus] = useState<ContactMessageStatus>('pending')
  const [remark, setRemark] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!props.message) return
    setStatus(props.message.status)
    setRemark(props.message.remark || '')
  }, [props.message])

  const handleSave = async () => {
    if (!props.message) return
    setSaving(true)
    try {
      const response = await updateContactMessage(props.message.id, {
        status,
        remark: remark.trim(),
      })
      if (response.success) {
        toast.success(t('Contact message updated'))
        props.onOpenChange(false)
        props.onSaved()
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Edit contact message')}</DialogTitle>
          <DialogDescription>
            {t('Update contact status and admin remark')}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-3'>
          <div className='grid gap-1.5'>
            <label className='text-sm font-medium' htmlFor='contact-status'>
              {t('Status')}
            </label>
            <NativeSelect
              id='contact-status'
              value={status}
              onChange={(event) =>
                setStatus(event.target.value as ContactMessageStatus)
              }
            >
              {CONTACT_MESSAGE_STATUSES.map((option) => (
                <NativeSelectOption key={option} value={option}>
                  {getContactMessageStatusLabel(t, option)}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <div className='grid gap-1.5'>
            <label className='text-sm font-medium' htmlFor='contact-remark'>
              {t('Remark')}
            </label>
            <Textarea
              id='contact-remark'
              value={remark}
              maxLength={500}
              rows={4}
              onChange={(event) => setRemark(event.target.value)}
              placeholder={t('Admin remark')}
            />
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function ContactMessages() {
  const { t } = useTranslation()
  const [messages, setMessages] = useState<ContactMessage[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<ContactMessageStatus | ''>('')
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<ContactMessage | null>(null)
  const [deleting, setDeleting] = useState<ContactMessage | null>(null)

  const fetchMessages = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getContactMessages({
        page,
        pageSize: CONTACT_MESSAGES_PAGE_SIZE,
        status,
      })
      if (response.success && response.data) {
        setMessages(response.data.items || [])
        setTotal(response.data.total || 0)
      }
    } finally {
      setLoading(false)
    }
  }, [page, status])

  useEffect(() => {
    fetchMessages()
  }, [fetchMessages])

  const handleStatusFilterChange = (value: string) => {
    setStatus(value as ContactMessageStatus | '')
    setPage(1)
  }

  const handleDelete = async () => {
    if (!deleting) return
    const response = await deleteContactMessage(deleting.id)
    if (response.success) {
      toast.success(t('Contact message deleted'))
      setDeleting(null)
      fetchMessages()
    }
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Contact Messages')}</SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('Manage homepage contact messages')}
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <Card className='py-0 bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50 border-purple-100'>
            <CardHeader className='flex flex-col gap-3 p-4 lg:flex-row lg:items-start lg:justify-between'>
              <div className='min-w-0'>
                <CardTitle className='text-base'>
                  {t('Contact message records')}
                </CardTitle>
                <p className='text-muted-foreground mt-1 text-sm'>
                  {t('Review and process submitted contact messages')}
                </p>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <NativeSelect
                  value={status}
                  onChange={(event) =>
                    handleStatusFilterChange(event.target.value)
                  }
                  className='w-full sm:w-44'
                >
                  <NativeSelectOption value=''>
                    {t('All statuses')}
                  </NativeSelectOption>
                  {CONTACT_MESSAGE_STATUSES.map((option) => (
                    <NativeSelectOption key={option} value={option}>
                      {getContactMessageStatusLabel(t, option)}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={fetchMessages}
                  disabled={loading}
                >
                  <RefreshCw
                    data-icon='inline-start'
                    className={loading ? 'animate-spin' : undefined}
                  />
                  {t('Refresh')}
                </Button>
              </div>
            </CardHeader>
            <CardContent className='p-0'>
              <div className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Contact name')}</TableHead>
                      <TableHead>{t('Contact phone')}</TableHead>
                      <TableHead>{t('Contact message')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Submitted At')}</TableHead>
                      <TableHead>{t('Remark')}</TableHead>
                      <TableHead>{t('Processed At')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {messages.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={8}
                          className='text-muted-foreground h-24 text-center'
                        >
                          {t('No contact messages found')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      messages.map((message) => (
                        <TableRow key={message.id}>
                          <TableCell className='font-medium'>
                            {message.name}
                          </TableCell>
                          <TableCell className='font-mono text-xs'>
                            {message.phone}
                          </TableCell>
                          <TableCell className='max-w-[260px] whitespace-normal'>
                            {message.message || '-'}
                          </TableCell>
                          <TableCell>
                            <Badge variant='outline'>
                              {getContactMessageStatusLabel(t, message.status)}
                            </Badge>
                          </TableCell>
                          <TableCell>{formatTime(message.created_at)}</TableCell>
                          <TableCell className='max-w-[220px] whitespace-normal'>
                            {message.remark || '-'}
                          </TableCell>
                          <TableCell>
                            <div>{formatTime(message.processed_at)}</div>
                            {message.processed_by ? (
                              <div className='text-muted-foreground text-xs'>
                                {t('Processor ID')}: {message.processed_by}
                              </div>
                            ) : null}
                          </TableCell>
                          <TableCell className='text-right'>
                            <div className='flex justify-end gap-2'>
                              <Button
                                size='sm'
                                variant='outline'
                                onClick={() => setEditing(message)}
                              >
                                {t('Edit')}
                              </Button>
                              <Button
                                size='sm'
                                variant='destructive'
                                onClick={() => setDeleting(message)}
                              >
                                {t('Delete')}
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
                pageSize={CONTACT_MESSAGES_PAGE_SIZE}
                total={total}
                loading={loading}
                onPageChange={setPage}
                t={t}
              />
            </CardContent>
          </Card>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <EditContactMessageDialog
        open={Boolean(editing)}
        onOpenChange={(open) => !open && setEditing(null)}
        message={editing}
        onSaved={fetchMessages}
      />
      <ConfirmDialog
        open={Boolean(deleting)}
        onOpenChange={(open) => !open && setDeleting(null)}
        title={t('Delete contact message')}
        desc={t('Delete this contact message? This action cannot be undone.')}
        confirmText={t('Delete')}
        destructive
        handleConfirm={handleDelete}
      />
    </>
  )
}
