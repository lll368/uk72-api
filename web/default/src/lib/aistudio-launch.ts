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
 * AI Studio launch helpers
 *
 * The launch dialog reuses the keys that the user has already created on the
 * `/keys` page (see {@link import('@/features/keys/api')}). We fetch the
 * paginated list, then resolve the unmasked keys via the batch endpoint so the
 * user can pick which key to send into AI Studio. The first option is selected
 * (and copied to the clipboard) by default.
 */
import { fetchTokenKeysBatch, getApiKeys } from '@/features/keys/api'
import { normalizeFullApiKey } from '@/features/keys/lib/api-key-format'

export const AISTUDIO_TARGET_URL = 'https://aistudio.qnlinking.com/apps'

export interface LaunchKeyOption {
  /** Token id from `/api/token/`. */
  id: number
  /** Human readable name configured by the user. */
  name: string
  /** Full key already prefixed with `sk-`, ready to paste into AI Studio. */
  key: string
}

/**
 * Load the current user's enabled API keys (with their unmasked values) so the
 * launch dialog can offer them as copy targets.
 *
 * - Only keys with `status === 1` (enabled) are returned.
 * - Keys whose unmasked value cannot be resolved are dropped silently.
 * - Order follows the server response (most recently created first by
 *   default), so callers can simply pick `result[0]` as the default option.
 */
export async function fetchUserLaunchKeys(): Promise<LaunchKeyOption[]> {
  const listRes = await getApiKeys({ p: 1, size: 50 })
  const items = listRes.success ? (listRes.data?.items ?? []) : []
  if (items.length === 0) return []

  const enabled = items.filter((item) => item.status === 1)
  if (enabled.length === 0) return []

  const ids = enabled.map((item) => item.id)
  const keysRes = await fetchTokenKeysBatch(ids)
  if (!keysRes.success || !keysRes.data?.keys) return []

  const realKeys = keysRes.data.keys
  return enabled
    .map((item) => {
      const real = realKeys[item.id]
      if (!real) return null
      return {
        id: item.id,
        name: item.name,
        key: normalizeFullApiKey(real),
      } satisfies LaunchKeyOption
    })
    .filter((option): option is LaunchKeyOption => option !== null)
}
