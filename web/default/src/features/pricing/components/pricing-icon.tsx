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
import type { CSSProperties } from 'react'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { getPricingIconSource, isRemotePricingIcon } from '../lib/pricing-icon'

export function PricingIcon(props: {
  icon?: string | null
  fallbackIcon?: string | null
  size?: number
  alt?: string
  className?: string
  style?: CSSProperties
}) {
  const source = getPricingIconSource(props.icon, props.fallbackIcon)
  if (!source) return null

  const size = props.size ?? 20
  if (isRemotePricingIcon(source)) {
    return (
      <img
        src={source}
        alt={props.alt ?? ''}
        className={cn('object-contain', props.className)}
        style={{ width: size, height: size, ...props.style }}
        loading='lazy'
        referrerPolicy='no-referrer'
      />
    )
  }

  return getLobeIcon(source, size)
}
