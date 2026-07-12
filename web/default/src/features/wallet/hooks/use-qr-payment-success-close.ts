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
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { QrPaymentStatus } from '../types'

interface UseQrPaymentSuccessCloseOptions {
  status: QrPaymentStatus
  refresh: () => Promise<boolean> | boolean
  closeSession: () => void
  setOpen: (open: boolean) => void
  failureMessageKey: string
  failureLogMessage: string
}

export function useQrPaymentSuccessClose({
  status,
  refresh,
  closeSession,
  setOpen,
  failureMessageKey,
  failureLogMessage,
}: UseQrPaymentSuccessCloseOptions) {
  const { t } = useTranslation()
  const [successClosing, setSuccessClosing] = useState(false)
  const successClosingRef = useRef(false)

  const notifyRefreshFailure = useCallback(
    (error?: unknown) => {
      if (error) {
        // eslint-disable-next-line no-console
        console.error(failureLogMessage, error)
      }
      toast.error(t(failureMessageKey))
    },
    [failureLogMessage, failureMessageKey, t]
  )

  const closeAfterSuccess = useCallback(async () => {
    if (successClosingRef.current) return

    successClosingRef.current = true
    setSuccessClosing(true)
    try {
      const refreshed = await refresh()
      if (!refreshed) {
        notifyRefreshFailure()
      }
    } catch (error) {
      notifyRefreshFailure(error)
    } finally {
      closeSession()
      setOpen(false)
      successClosingRef.current = false
      setSuccessClosing(false)
    }
  }, [closeSession, notifyRefreshFailure, refresh, setOpen])

  const handleOpenChange = useCallback(
    (open: boolean) => {
      if (!open) {
        if (status === 'success') {
          void closeAfterSuccess()
          return
        }
        closeSession()
      }
      setOpen(open)
    },
    [closeAfterSuccess, closeSession, setOpen, status]
  )

  return {
    successClosing,
    closeAfterSuccess,
    handleOpenChange,
  }
}
