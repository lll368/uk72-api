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

export const INTERFACE_LANGUAGE_OPTIONS = [
  { code: 'zh-CN', label: '简体中文' },
  { code: 'zh-TW', label: '繁體中文' },
  { code: 'en', label: 'English' },
  { code: 'fr', label: 'Français' },
  { code: 'ru', label: 'Русский' },
  { code: 'ja', label: '日本語' },
  { code: 'vi', label: 'Tiếng Việt' },
  { code: 'ko', label: '한국어' },
  { code: 'ar', label: 'العربية' },
  { code: 'pt-BR', label: 'Português (Brasil)' },
  { code: 'es-ES', label: 'Español' },
] as const

export type InterfaceLanguageCode =
  (typeof INTERFACE_LANGUAGE_OPTIONS)[number]['code']

export function normalizeInterfaceLanguage(value?: string | null): string {
  if (!value) return 'en'

  const normalized = value.trim().replace(/_/g, '-').toLowerCase()
  if (
    normalized === 'zh-tw' ||
    normalized === 'zh-hk' ||
    normalized === 'zh-mo' ||
    normalized.startsWith('zh-hant')
  ) {
    return 'zh-TW'
  }
  if (
    normalized === 'zh' ||
    normalized === 'zh-cn' ||
    normalized === 'zh-sg' ||
    normalized.startsWith('zh-hans')
  ) {
    return 'zh-CN'
  }
  if (normalized === 'en' || normalized.startsWith('en-')) return 'en'
  if (normalized === 'fr' || normalized.startsWith('fr-')) return 'fr'
  if (normalized === 'ru' || normalized.startsWith('ru-')) return 'ru'
  if (normalized === 'ja' || normalized.startsWith('ja-')) return 'ja'
  if (normalized === 'vi' || normalized.startsWith('vi-')) return 'vi'
  if (normalized === 'ko' || normalized.startsWith('ko-')) return 'ko'
  if (normalized === 'ar' || normalized.startsWith('ar-')) return 'ar'
  if (normalized === 'pt' || normalized.startsWith('pt-')) return 'pt-BR'
  if (normalized === 'es' || normalized.startsWith('es-')) return 'es-ES'

  return INTERFACE_LANGUAGE_OPTIONS.some((lang) => lang.code === normalized)
    ? normalized
    : 'en'
}

export function isRtlInterfaceLanguage(value?: string | null): boolean {
  return normalizeInterfaceLanguage(value) === 'ar'
}
