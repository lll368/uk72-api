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
import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Copy, ExternalLink, KeyRound } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import {
  AISTUDIO_TARGET_URL,
  fetchUserLaunchKeys,
  type LaunchKeyOption,
} from '@/lib/aistudio-launch'

/**
 * Module-level opener so any caller (CTA buttons across pages) can trigger the
 * dialog without prop drilling. The dialog is mounted exactly once at the
 * application root (`__root.tsx`).
 *
 * Pass {@link OpenAistudioLaunchOptions.presetKey} to short-circuit the key
 * list loading and pin the dialog to a specific key (e.g. when launched from a
 * row action on the API keys table).
 */
export interface OpenAistudioLaunchOptions {
  /** Already-resolved key (with `sk-` prefix). When provided, the dialog
   *  shows just this key and skips the `/api/token` list fetch. */
  presetKey?: string
  /** Optional display label for the preset key. */
  presetName?: string
  /** Override the default AI Studio URL. When provided, the dialog's
   *  "Open" button and the fallback navigation will open this URL
   *  instead of AISTUDIO_TARGET_URL. */
  targetUrl?: string
}

let externalOpen: ((options?: OpenAistudioLaunchOptions) => void) | null = null

export function openAistudioLaunchDialog(
  options?: OpenAistudioLaunchOptions
) {
  if (externalOpen) {
    externalOpen(options)
  } else if (typeof window !== 'undefined') {
    // Fallback: if the dialog has not mounted for some reason, just navigate.
    window.open(
      options?.targetUrl ?? AISTUDIO_TARGET_URL,
      '_blank',
      'noopener,noreferrer'
    )
  }
}

/**
 * Mask an API key for display purposes only.
 * Keeps the alphabetic prefix (e.g. `sk-`) plus the first 4 and last 4 chars
 * of the body, replacing the middle with bullets.
 *
 * Examples:
 *   sk-abcdef1234567890xyz  ->  sk-abcd•••••••0xyz
 *   sk-abcd                 ->  sk-****     (too short to keep both ends)
 *   ''                      ->  ''
 *
 * Note: only used for rendering. Clipboard / copy-again handlers always use
 * the raw key so the user can still paste the complete value.
 */
function maskApiKey(key: string): string {
  if (!key) return ''
  const match = key.match(/^([A-Za-z]+-)(.+)$/)
  const prefix = match ? match[1] : ''
  const body = match ? match[2] : key
  if (body.length <= 8) {
    return `${prefix}${'*'.repeat(body.length)}`
  }
  const head = body.slice(0, 4)
  const tail = body.slice(-4)
  return `${prefix}${head}•••••••${tail}`
}

export function AistudioLaunchDialog() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [keys, setKeys] = useState<LaunchKeyOption[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [targetUrl, setTargetUrl] = useState<string>(AISTUDIO_TARGET_URL)
  const { copyToClipboard } = useCopyToClipboard({ notify: false })

  useEffect(() => {
    const opener = (options?: OpenAistudioLaunchOptions) => {
      setOpen(true)
      setTargetUrl(options?.targetUrl ?? AISTUDIO_TARGET_URL)
      if (options?.presetKey) {
        void usePresetKey(options.presetKey, options.presetName)
      } else {
        void loadKeys()
      }
    }
    externalOpen = opener
    return () => {
      if (externalOpen === opener) externalOpen = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const selectedKey =
    keys.find((item) => item.id === selectedId)?.key ?? ''
  const noKeys = !loading && keys.length === 0

  /**
   * Direct-key mode: the caller already knows which key to use (e.g. row
   * action on the API keys table). Skip the list fetch and just display +
   * copy the supplied key.
   */
  async function usePresetKey(key: string, name?: string) {
    setLoading(false)
    const option: LaunchKeyOption = {
      id: -1,
      name: name?.trim() ? name : t('Selected API key'),
      key,
    }
    setKeys([option])
    setSelectedId(option.id)
    const ok = await copyToClipboard(key)
    if (ok) {
      toast.success(t('API key copied to clipboard'))
    } else {
      toast.error(t('Auto-copy failed. Please tap "Copy again" below.'))
    }
  }

  async function loadKeys() {
    setLoading(true)
    setKeys([])
    setSelectedId(null)
    try {
      const list = await fetchUserLaunchKeys()
      setKeys(list)
      if (list.length > 0) {
        setSelectedId(list[0].id)
        const ok = await copyToClipboard(list[0].key)
        if (ok) {
          toast.success(t('API key copied to clipboard'))
        } else {
          toast.error(t('Auto-copy failed. Please tap "Copy again" below.'))
        }
      }
    } catch {
      toast.error(t('Failed to load API keys. Please try again.'))
    } finally {
      setLoading(false)
    }
  }

  async function handleSelectChange(value: string | null) {
    if (!value) return
    const id = Number(value)
    if (!Number.isFinite(id)) return
    setSelectedId(id)
    const target = keys.find((item) => item.id === id)
    if (!target) return
    const ok = await copyToClipboard(target.key)
    if (ok) {
      toast.success(t('API key copied to clipboard'))
    } else {
      toast.error(t('Auto-copy failed. Please tap "Copy again" below.'))
    }
  }

  async function handleCopyAgain() {
    if (!selectedKey) return
    const ok = await copyToClipboard(selectedKey)
    if (ok) {
      toast.success(t('API key copied to clipboard'))
    }
  }

  function handleConfirm() {
    setOpen(false)
    if (typeof window !== 'undefined') {
      window.open(targetUrl, '_blank', 'noopener,noreferrer')
    }
  }

  function handleGoCreateKey() {
    setOpen(false)
    void navigate({ to: '/keys' })
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <KeyRound className='size-5 text-blue-600' aria-hidden='true' />
            {noKeys
              ? t('No API key available')
              : t('Your API key is ready')}
          </DialogTitle>
          <DialogDescription className='leading-relaxed'>
            {noKeys
              ? t(
                  'You do not have an API key yet. Create one on the API Keys page first, then come back to launch AI Studio.'
                )
              : t(
                  'We have prepared a personal API key and copied it to your clipboard. After AI Studio opens, click "Profile" at the bottom-left and paste the key in the dialog to start instantly.'
                )}
          </DialogDescription>
        </DialogHeader>

        {!noKeys && (
          <div className='space-y-2'>
            {keys.length > 1 && (
              <Select
                value={selectedId != null ? String(selectedId) : ''}
                onValueChange={handleSelectChange}
                disabled={loading}
              >
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder={t('Select an API key')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  {keys.map((item) => (
                    <SelectItem key={item.id} value={String(item.id)}>
                      {item.name || `#${item.id}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}

            <div className='bg-muted/60 flex items-center gap-2 rounded-md border px-3 py-2 font-mono text-sm'>
              <span
                className='min-w-0 flex-1 truncate select-none'
                title={t('API key hidden for security. Use "Copy again" to copy the full key.')}
              >
                {loading
                  ? t('Loading API keys...')
                  : selectedKey
                    ? maskApiKey(selectedKey)
                    : '—'}
              </span>
              <Button
                variant='ghost'
                size='icon-sm'
                onClick={handleCopyAgain}
                disabled={!selectedKey || loading}
                aria-label={t('Copy again')}
                type='button'
              >
                <Copy className='size-4' />
              </Button>
            </div>
          </div>
        )}

        <DialogFooter>
          <Button
            variant='outline'
            onClick={() => setOpen(false)}
            type='button'
          >
            {t('Maybe later')}
          </Button>
          {noKeys ? (
            <Button onClick={handleGoCreateKey} type='button'>
              {t('Create API key')}
            </Button>
          ) : (
            <Button
              onClick={handleConfirm}
              disabled={loading || !selectedKey}
              type='button'
            >
              <ExternalLink data-icon='inline-start' />
              {t('Open AI Studio')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
