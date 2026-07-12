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
import { Check, Palette } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { THEME_PRESETS } from '@/lib/theme-customization'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export function ThemePresetSwitcher() {
  const { t } = useTranslation()
  const { customization, setPreset } = useThemeCustomization()

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        render={
          <Button variant='ghost' size='icon' className='h-9 w-9'>
            <Palette className='size-[1.2rem]' />
            <span className='sr-only'>{t('Theme preset')}</span>
          </Button>
        }
      />
      <DropdownMenuContent align='end' className='w-56'>
        <div className='px-2 py-1'>
          <span className='text-xs text-muted-foreground px-2'>{t('Theme Preset')}</span>
        </div>
        {THEME_PRESETS.map((preset) => (
          <DropdownMenuItem
            key={preset.value}
            onClick={() => setPreset(preset.value)}
            className='cursor-pointer'
          >
            <div className='flex items-center gap-3 w-full'>
              <div className='flex gap-1'>
                {preset.swatches.map((swatch, index) => (
                  <div
                    key={index}
                    className='w-4 h-4 rounded-full border border-border'
                    style={{ backgroundColor: swatch }}
                  />
                ))}
              </div>
              <span className='flex-1 text-left'>{preset.name}</span>
              <Check
                size={14}
                className={cn('ms-auto', customization.preset !== preset.value && 'hidden')}
              />
            </div>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => setPreset('default')}>
          {t('Reset to default')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
