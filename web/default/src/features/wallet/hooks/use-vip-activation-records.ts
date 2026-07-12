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
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  disableVipActivation,
  getVipActivationRecords,
  isApiSuccess,
} from '../api'
import type { VipActivationRecord } from '../types'

interface UseVipActivationRecordsOptions {
  initialPage?: number
  initialPageSize?: number
}

export function useVipActivationRecords(
  options: UseVipActivationRecordsOptions = {}
) {
  const { initialPage = 1, initialPageSize = 10 } = options
  const [records, setRecords] = useState<VipActivationRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [loading, setLoading] = useState(false)
  const [disabling, setDisabling] = useState(false)

  const fetchRecords = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getVipActivationRecords(page, pageSize)
      if (isApiSuccess(response) && response.data) {
        setRecords(response.data.items || [])
        setTotal(response.data.total || 0)
        return
      }
      toast.error(
        response.message || i18next.t('Failed to load VVIP records')
      )
      setRecords([])
      setTotal(0)
    } catch (_error) {
      toast.error(i18next.t('Failed to load VVIP records'))
      setRecords([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize])

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize)
    setPage(1)
  }, [])

  const handleDisableVipActivation = useCallback(
    async (userId: number, reason?: string) => {
      setDisabling(true)
      try {
        const response = await disableVipActivation(userId, reason)
        if (isApiSuccess(response)) {
          toast.success(i18next.t('VVIP disabled successfully'))
          await fetchRecords()
          return true
        }
        toast.error(
          response.message || i18next.t('Failed to disable VVIP')
        )
        return false
      } catch (_error) {
        toast.error(i18next.t('Failed to disable VVIP'))
        return false
      } finally {
        setDisabling(false)
      }
    },
    [fetchRecords]
  )

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords])

  return {
    records,
    total,
    page,
    pageSize,
    loading,
    disabling,
    handlePageChange,
    handlePageSizeChange,
    handleDisableVipActivation,
    refresh: fetchRecords,
  }
}
