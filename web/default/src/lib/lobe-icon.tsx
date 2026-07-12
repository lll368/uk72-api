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
/**
 * LobeHub Icon Loader
 * Dynamically load and render icons from @lobehub/icons.
 *
 * Supports:
 * - Basic:               "OpenAI", "OpenAI.Color"
 * - Chained properties:  "OpenAI.Avatar.type={'platform'}"
 * - Size parameter:      getLobeIcon("OpenAI", 20)
 *
 * ─────────────────────────────────────────────────────────────────────────────
 *  Performance note (IMPORTANT — do NOT revert to `import * as`):
 *
 *  `@lobehub/icons` is ~4.6 MB and uses a namespace API. A static
 *  `import * as LobeIcons from '@lobehub/icons'` would pull the entire
 *  package into whichever chunk imports this file — and because this file
 *  is referenced by many features (model-card, pricing, channels, …) that
 *  are needed early, webpack ended up shipping the whole 4.6 MB on the
 *  first paint.
 *
 *  We now load the package lazily via `import('@lobehub/icons')`, which
 *  splits it into its own async chunk that is fetched only the first time
 *  any `<LobeIcon />` mounts. While the chunk is in flight we render the
 *  existing first-letter / "?" fallback, then re-render once the module is
 *  ready. The exported `getLobeIcon()` function keeps its synchronous
 *  signature (returns a React node) so existing call sites do not need to
 *  change.
 * ─────────────────────────────────────────────────────────────────────────────
 */
import { useEffect, useState } from 'react'

type LobeIconsModule = typeof import('@lobehub/icons')

let cachedModule: LobeIconsModule | null = null
let pending: Promise<LobeIconsModule> | null = null
const subscribers = new Set<() => void>()

function loadLobeIcons(): Promise<LobeIconsModule> {
  if (cachedModule) return Promise.resolve(cachedModule)
  if (pending) return pending
  pending = import('@lobehub/icons').then((mod) => {
    cachedModule = mod
    pending = null
    // Notify every mounted <LobeIcon /> so they can re-render with the real icon.
    subscribers.forEach((cb) => cb())
    return mod
  })
  return pending
}

/**
 * Hook that subscribes to lazy module readiness.
 * Returns the loaded module, or null while still loading.
 */
function useLobeIconsModule(): LobeIconsModule | null {
  // Trigger an initial render when the module finishes loading.
  const [, force] = useState(0)

  useEffect(() => {
    if (cachedModule) return
    const cb = () => force((n) => n + 1)
    subscribers.add(cb)
    void loadLobeIcons()
    return () => {
      subscribers.delete(cb)
    }
  }, [])

  return cachedModule
}

/**
 * Parse a property value from string to appropriate type.
 */
function parseValue(raw: string | undefined | null): string | number | boolean {
  if (raw == null) return true

  let v = String(raw).trim()

  // Remove curly braces
  if (v.startsWith('{') && v.endsWith('}')) {
    v = v.slice(1, -1).trim()
  }

  // Remove quotes
  if (
    (v.startsWith('"') && v.endsWith('"')) ||
    (v.startsWith("'") && v.endsWith("'"))
  ) {
    return v.slice(1, -1)
  }

  // Boolean
  if (v === 'true') return true
  if (v === 'false') return false

  // Number
  if (/^-?\d+(?:\.\d+)?$/.test(v)) return Number(v)

  return v
}

interface FallbackProps {
  letter: string
  size: number
}

function IconFallback({ letter, size }: FallbackProps) {
  return (
    <div
      className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
      style={{ width: size, height: size }}
    >
      {letter}
    </div>
  )
}

interface LobeIconProps {
  name?: string | null
  size?: number
}

/**
 * React component variant. Use this directly if you already have JSX context.
 * Internally lazy-loads `@lobehub/icons` on first mount.
 */
export function LobeIcon({ name, size = 20 }: LobeIconProps): React.ReactNode {
  const mod = useLobeIconsModule()

  if (!name || typeof name !== 'string') {
    return <IconFallback letter='?' size={size} />
  }
  const trimmedName = name.trim()
  if (!trimmedName) {
    return <IconFallback letter='?' size={size} />
  }

  // While the lazy module is still loading, render the same first-letter
  // fallback we'd use on a missing icon. Once the chunk arrives, the
  // subscriber wakes us up and we re-render with the real icon component.
  if (!mod) {
    return <IconFallback letter={trimmedName.charAt(0).toUpperCase()} size={size} />
  }

  const segments = trimmedName.split('.')
  const baseKey = segments[0]
  const BaseIcon = (mod as Record<string, unknown>)[baseKey] as
    | Record<string, unknown>
    | undefined

  let IconComponent: React.ComponentType<Record<string, unknown>> | undefined
  let propStartIndex: number

  if (BaseIcon && segments.length > 1 && BaseIcon[segments[1]]) {
    IconComponent = BaseIcon[segments[1]] as React.ComponentType<
      Record<string, unknown>
    >
    propStartIndex = 2
  } else {
    IconComponent = (mod as Record<string, unknown>)[baseKey] as
      | React.ComponentType<Record<string, unknown>>
      | undefined
    propStartIndex = segments.length > 1 && /^[A-Z]/.test(segments[1]) ? 2 : 1
  }

  // Fallback if icon not found in the loaded module
  if (
    !IconComponent ||
    (typeof IconComponent !== 'function' && typeof IconComponent !== 'object')
  ) {
    return (
      <IconFallback letter={trimmedName.charAt(0).toUpperCase()} size={size} />
    )
  }

  // Parse chained properties (e.g., "type={'platform'}", "shape='square'")
  const props: Record<string, string | number | boolean> = {}

  for (let i = propStartIndex; i < segments.length; i++) {
    const seg = segments[i]
    if (!seg) continue

    const eqIdx = seg.indexOf('=')
    if (eqIdx === -1) {
      props[seg.trim()] = true
      continue
    }

    const key = seg.slice(0, eqIdx).trim()
    const valRaw = seg.slice(eqIdx + 1).trim()
    props[key] = parseValue(valRaw)
  }

  // Set size if not explicitly specified in the string
  if (props.size == null && size != null) {
    props.size = size
  }

  return <IconComponent {...props} />
}

/**
 * Synchronous helper preserved for backwards compatibility with existing
 * call sites that use this as a plain function (e.g. when assigning to a
 * Select option's `icon` prop). Returns a JSX node that internally
 * lazy-loads the icon module on first mount.
 *
 * @example
 *   getLobeIcon("OpenAI", 24)
 *   getLobeIcon("Claude.Avatar.type={'platform'}", 32)
 */
export function getLobeIcon(
  iconName: string | undefined | null,
  size: number = 20
): React.ReactNode {
  return <LobeIcon name={iconName} size={size} />
}
