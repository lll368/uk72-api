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
import { BadgePercent, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { getVipSubordinates } from '../api'

interface UpstreamDiscountCardProps {
  enabled: boolean
}

function formatDiscount(value?: number) {
  const discount = Number(value || 1)
  if (discount <= 0 || discount >= 1) return '-'
  const percent = discount * 100
  return `${Number.isInteger(percent) ? percent.toFixed(0) : percent.toFixed(1)}%`
}

export function UpstreamDiscountCard(props: UpstreamDiscountCardProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [parentDiscount, setParentDiscount] = useState(1)
  const [canSetDiscount, setCanSetDiscount] = useState(false)
  const [minDiscount, setMinDiscount] = useState(1)

  const fetchData = useCallback(async () => {
    if (!props.enabled) return
    setLoading(true)
    try {
      const response = await getVipSubordinates(1, 1)
      if (response.success && response.data) {
        const nextParentDiscount = response.data.parent_topup_discount || 1
        setParentDiscount(nextParentDiscount)
        setCanSetDiscount(Boolean(response.data.can_set_subordinate_discount))
        setMinDiscount(
          response.data.min_subordinate_topup_discount || nextParentDiscount
        )
        return
      }
      toast.error(response.message || t('Failed to load upstream discount'))
    } finally {
      setLoading(false)
    }
  }, [props.enabled, t])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  if (!props.enabled) return null

  const showMinAssignable = canSetDiscount

  return (
    <Card className='py-0 bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50 border-purple-100'>
      <CardHeader className='flex flex-col gap-3 p-4 lg:flex-row lg:items-start lg:justify-between'>
        <div className='flex min-w-0 items-start gap-3'>
          <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg border'>
            <BadgePercent className='text-muted-foreground size-4' />
          </div>
          <div className='min-w-0'>
            <CardTitle className='text-base'>
              {t('Upstream Discount')}
            </CardTitle>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Recharge discount granted to you by your upstream')}
            </p>
          </div>
        </div>
        <Button
          size='sm'
          variant='outline'
          onClick={fetchData}
          disabled={loading}
        >
          <RefreshCw data-icon='inline-start' />
          {t('Refresh')}
        </Button>
      </CardHeader>
      <CardContent className='border-t p-4'>
        <div className='grid gap-4 sm:grid-cols-2'>
          <div className='flex flex-col gap-1'>
            <span className='text-muted-foreground text-xs'>
              {t('Current discount')}
            </span>
            {loading ? (
              <Skeleton className='h-7 w-20' />
            ) : (
              <span className='font-mono text-lg font-medium'>
                {formatDiscount(parentDiscount)}
              </span>
            )}
          </div>
          {showMinAssignable ? (
            <div className='flex flex-col gap-1'>
              <span className='text-muted-foreground text-xs'>
                {t('Minimum assignable to subordinates')}
              </span>
              {loading ? (
                <Skeleton className='h-7 w-20' />
              ) : (
                <span className='font-mono text-lg font-medium'>
                  {formatDiscount(minDiscount)}
                </span>
              )}
            </div>
          ) : null}
        </div>
        {!loading && !canSetDiscount ? (
          <p className='text-muted-foreground mt-3 text-xs'>
            {t(
              'Your current discount does not allow assigning subordinate discounts'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
