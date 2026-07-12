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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useStatus } from '@/hooks/use-status'
import { getPricing } from '../api'
import {
  sanitizeSupplierBrandText,
  sanitizeSupplierBrandUrl,
} from '../lib/qiniu-market'

export function usePricingData(options?: {
  enabled?: boolean
  silent?: boolean
}) {
  const { status } = useStatus()
  const enabled = options?.enabled ?? true

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['pricing'],
    queryFn: () => getPricing({ silent: options?.silent }),
    enabled,
    staleTime: 5 * 60 * 1000,
  })

  // Ensure rates never reach zero to prevent division errors
  const priceRate = useMemo(
    () => Math.max((status?.price as number) ?? 1, 0.001),
    [status?.price]
  )
  const usdExchangeRate = useMemo(
    () => Math.max((status?.usd_exchange_rate as number) ?? priceRate, 0.001),
    [status?.usd_exchange_rate, priceRate]
  )

  const models = useMemo(() => {
    if (!data?.data || !data?.vendors) return []

    const vendorMap = new Map(
      data.vendors.map((v) => [
        v.id,
        {
          ...v,
          name: sanitizeSupplierBrandText(v.name),
          icon: sanitizeSupplierBrandUrl(v.icon),
          description: sanitizeSupplierBrandText(v.description),
        },
      ])
    )

    return data.data.map((model) => {
      const vendor = model.vendor_id
        ? vendorMap.get(model.vendor_id)
        : undefined
      return {
        ...model,
        description: sanitizeSupplierBrandText(model.description),
        icon: sanitizeSupplierBrandUrl(model.icon),
        tags: sanitizeSupplierBrandText(model.tags),
        key: model.model_name,
        vendor_name: vendor?.name,
        vendor_icon: vendor?.icon,
        vendor_description: vendor?.description,
        group_ratio: data.group_ratio,
      }
    })
  }, [data])

  return {
    models,
    vendors:
      data?.vendors.map((vendor) => ({
        ...vendor,
        name: sanitizeSupplierBrandText(vendor.name),
        icon: sanitizeSupplierBrandUrl(vendor.icon),
        description: sanitizeSupplierBrandText(vendor.description),
      })) ?? [],
    groupRatio: data?.group_ratio ?? {},
    usableGroup: data?.usable_group ?? {},
    endpointMap: data?.supported_endpoint ?? {},
    autoGroups: data?.auto_groups ?? [],
    isLoading,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
  }
}
