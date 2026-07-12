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
import { useState } from 'react'
import { type Row } from '@tanstack/react-table'
import {
  MoreHorizontal,
  Pencil,
  Trash2,
  Power,
  PowerOff,
  ArrowUp,
  ArrowDown,
  KeyRound,
  ShieldAlert,
  Link2,
  CreditCard,
  Crown,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Textarea } from '@/components/ui/textarea'
import { UserSubscriptionsDialog } from '@/features/subscriptions/components/dialogs/user-subscriptions-dialog'
import {
  disableUserVvip,
  activateUserVvip,
  manageUser,
  resetUserPasskey,
  resetUserTwoFA,
} from '../api'
import {
  USER_STATUS,
  USER_ROLE,
  ERROR_MESSAGES,
  isUserDeleted,
} from '../constants'
import { getUserActionMessage } from '../lib'
import { type User, type ManageUserAction } from '../types'
import { UserBindingDialog } from './dialogs/user-binding-dialog'
import { useUsers } from './users-provider'

interface DataTableRowActionsProps {
  row: Row<User>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const user = row.original
  const { setOpen, setCurrentRow, triggerRefresh } = useUsers()
  const [resetPasskeyOpen, setResetPasskeyOpen] = useState(false)
  const [resetTwoFAOpen, setResetTwoFAOpen] = useState(false)
  const [bindingDialogOpen, setBindingDialogOpen] = useState(false)
  const [subscriptionsDialogOpen, setSubscriptionsDialogOpen] = useState(false)
  const [disableVvipOpen, setDisableVvipOpen] = useState(false)
  const [activateVvipOpen, setActivateVvipOpen] = useState(false)
  const [activateVvipRemark, setActivateVvipRemark] = useState('')
  const [activateVvipLoading, setActivateVvipLoading] = useState(false)

  const handleEdit = () => {
    setCurrentRow(user)
    setOpen('update')
  }

  const handleDelete = () => {
    setCurrentRow(user)
    setOpen('delete')
  }

  const handleManage = async (action: Exclude<ManageUserAction, 'delete'>) => {
    try {
      const result = await manageUser(user.id, action)
      if (result.success) {
        toast.success(t(getUserActionMessage(action)))
        triggerRefresh()
      } else {
        toast.error(
          result.message || t('Failed to {{action}} user', { action })
        )
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    }
  }

  const handleResetPasskey = async () => {
    try {
      const result = await resetUserPasskey(user.id)
      if (result.success) {
        toast.success(t('Passkey reset successfully'))
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to reset Passkey'))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setResetPasskeyOpen(false)
    }
  }

  const handleResetTwoFA = async () => {
    try {
      const result = await resetUserTwoFA(user.id)
      if (result.success) {
        toast.success(t('Two-factor authentication reset'))
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to reset 2FA'))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setResetTwoFAOpen(false)
    }
  }

  const handleDisableVvip = async () => {
    try {
      const result = await disableUserVvip(user.id)
      if (result.success) {
        toast.success(t('VVIP disabled successfully'))
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to disable VVIP'))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setDisableVvipOpen(false)
    }
  }

  const handleActivateVvip = async () => {
    if (!activateVvipRemark.trim()) {
      toast.error(t('Remark is required.'))
      return
    }
    setActivateVvipLoading(true)
    try {
      const result = await activateUserVvip(user.id, activateVvipRemark.trim())
      if (result.success) {
        toast.success(t('VVIP activated successfully.'))
        triggerRefresh()
        setActivateVvipOpen(false)
        setActivateVvipRemark('')
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.UNEXPECTED))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setActivateVvipLoading(false)
    }
  }

  const isDisabled = user.status === USER_STATUS.DISABLED
  const isAdmin = user.role >= USER_ROLE.ADMIN
  const isRoot = user.role === USER_ROLE.ROOT
  const isActiveVvip = user.is_vvip === true || user.vvip_status === 'active'

  if (isUserDeleted(user)) {
    return null
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant='ghost'
              className='data-popup-open:bg-muted flex h-8 w-8 p-0'
            />
          }
        >
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>{t('Open menu')}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[180px]'>
          <DropdownMenuItem onClick={handleEdit}>
            {t('Edit')}
            <DropdownMenuShortcut>
              <Pencil size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          {isDisabled ? (
            <DropdownMenuItem onClick={() => handleManage('enable')}>
              {t('Enable')}
              <DropdownMenuShortcut>
                <Power size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem
              onClick={() => handleManage('disable')}
              disabled={isRoot}
            >
              {t('Disable')}
              <DropdownMenuShortcut>
                <PowerOff size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {isAdmin && !isRoot && (
            <DropdownMenuItem onClick={() => handleManage('demote')}>
              {t('Demote')}
              <DropdownMenuShortcut>
                <ArrowDown size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {!isAdmin && (
            <DropdownMenuItem onClick={() => handleManage('promote')}>
              {t('Promote')}
              <DropdownMenuShortcut>
                <ArrowUp size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setBindingDialogOpen(true)
            }}
          >
            {t('Manage Bindings')}
            <DropdownMenuShortcut>
              <Link2 size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setSubscriptionsDialogOpen(true)
            }}
          >
            {t('Manage Subscriptions')}
            <DropdownMenuShortcut>
              <CreditCard size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          {isActiveVvip && (
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault()
                setDisableVvipOpen(true)
              }}
              className='text-destructive focus:text-destructive'
            >
              {t('Disable VVIP')}
              <DropdownMenuShortcut>
                <Crown size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {!isActiveVvip && user.vvip_status !== 'disabled' && (
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault()
                setActivateVvipOpen(true)
              }}
            >
              {t('Manual Activate VVIP')}
              <DropdownMenuShortcut>
                <Crown size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          <DropdownMenuSeparator />

          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setResetPasskeyOpen(true)
            }}
            disabled={isRoot}
          >
            {t('Reset Passkey')}
            <DropdownMenuShortcut>
              <KeyRound size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setResetTwoFAOpen(true)
            }}
            disabled={isRoot}
          >
            {t('Reset 2FA')}
            <DropdownMenuShortcut>
              <ShieldAlert size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem
            onClick={handleDelete}
            className='text-destructive focus:text-destructive'
            disabled={isRoot}
          >
            {t('Delete')}
            <DropdownMenuShortcut>
              <Trash2 size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={resetPasskeyOpen}
        onOpenChange={setResetPasskeyOpen}
        title={t('Reset Passkey')}
        desc={`Reset Passkey for ${user.username}? The user will need to register a new Passkey before using passwordless login.`}
        confirmText='Reset Passkey'
        handleConfirm={handleResetPasskey}
      />

      <ConfirmDialog
        open={resetTwoFAOpen}
        onOpenChange={setResetTwoFAOpen}
        title={t('Reset Two-Factor Authentication')}
        desc={`Reset 2FA for ${user.username}? The user must set up 2FA again to continue using it.`}
        confirmText='Reset 2FA'
        handleConfirm={handleResetTwoFA}
      />

      <ConfirmDialog
        open={disableVvipOpen}
        onOpenChange={setDisableVvipOpen}
        title={t('Disable VVIP')}
        desc={t(
          'Are you sure you want to disable this user VVIP? Historical activation records will be retained.'
        )}
        confirmText={t('Disable VVIP')}
        destructive
        handleConfirm={handleDisableVvip}
      />

      <ConfirmDialog
        open={activateVvipOpen}
        onOpenChange={(open) => {
          setActivateVvipOpen(open)
          if (!open) setActivateVvipRemark('')
        }}
        title={t('Manual Activate VVIP')}
        desc={t(
          'Are you sure you want to manually activate VVIP for this user? This will skip the payment process.'
        )}
        confirmText={t('Confirm')}
        handleConfirm={handleActivateVvip}
        isLoading={activateVvipLoading}
      >
        <div className='py-2'>
          <label className='text-sm font-medium mb-1 block'>
            {t('Please provide a reason for this manual activation.')}
          </label>
          <Textarea
            value={activateVvipRemark}
            onChange={(e) => setActivateVvipRemark(e.target.value)}
            placeholder={t('Enter activation reason...')}
            rows={3}
          />
        </div>
      </ConfirmDialog>

      <UserBindingDialog
        open={bindingDialogOpen}
        onOpenChange={setBindingDialogOpen}
        userId={user.id}
        onUnbindSuccess={triggerRefresh}
      />

      <UserSubscriptionsDialog
        open={subscriptionsDialogOpen}
        onOpenChange={setSubscriptionsDialogOpen}
        user={{ id: user.id, username: user.username }}
        onSuccess={triggerRefresh}
      />
    </>
  )
}
