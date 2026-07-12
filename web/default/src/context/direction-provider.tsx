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
import { createContext, useContext, useEffect, useState } from 'react'
import { DirectionProvider as BaseDirectionProvider } from '@base-ui/react/direction-provider'
import { useTranslation } from 'react-i18next'
import { isRtlInterfaceLanguage } from '@/i18n/languages'
import { getCookie, setCookie, removeCookie } from '@/lib/cookies'

export type Direction = 'ltr' | 'rtl'

const DEFAULT_DIRECTION = 'ltr'
const DIRECTION_COOKIE_NAME = 'dir'
const DIRECTION_COOKIE_MAX_AGE = 60 * 60 * 24 * 365 // 1 year

type DirectionContextType = {
  defaultDir: Direction
  dir: Direction
  setDir: (dir: Direction) => void
  resetDir: () => void
}

const DirectionContext = createContext<DirectionContextType | null>(null)

function getStoredDirection(): Direction | undefined {
  const dir = getCookie(DIRECTION_COOKIE_NAME)
  return dir === 'ltr' || dir === 'rtl' ? dir : undefined
}

function getDefaultDirection(language?: string | null): Direction {
  return isRtlInterfaceLanguage(language) ? 'rtl' : DEFAULT_DIRECTION
}

export function DirectionProvider({ children }: { children: React.ReactNode }) {
  const { i18n } = useTranslation()
  const defaultDir = getDefaultDirection(i18n.language)
  const [manualDir, setManualDir] = useState<Direction | undefined>(
    () => getStoredDirection()
  )
  const [dir, _setDir] = useState<Direction>(
    () => getStoredDirection() || getDefaultDirection(i18n.language)
  )

  useEffect(() => {
    if (!manualDir) {
      _setDir(defaultDir)
    }
  }, [defaultDir, manualDir])

  useEffect(() => {
    const htmlElement = document.documentElement
    htmlElement.setAttribute('dir', dir)
  }, [dir])

  const setDir = (dir: Direction) => {
    setManualDir(dir)
    _setDir(dir)
    setCookie(DIRECTION_COOKIE_NAME, dir, DIRECTION_COOKIE_MAX_AGE)
  }

  const resetDir = () => {
    setManualDir(undefined)
    _setDir(defaultDir)
    removeCookie(DIRECTION_COOKIE_NAME)
  }

  return (
    <DirectionContext
      value={{
        defaultDir,
        dir,
        setDir,
        resetDir,
      }}
    >
      <BaseDirectionProvider direction={dir}>{children}</BaseDirectionProvider>
    </DirectionContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useDirection() {
  const context = useContext(DirectionContext)
  if (!context) {
    throw new Error('useDirection must be used within a DirectionProvider')
  }
  return context
}
