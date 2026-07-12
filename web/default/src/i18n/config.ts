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
import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'
import { normalizeInterfaceLanguage } from './languages'

if (typeof window !== 'undefined') {
  const storedLanguage = window.localStorage.getItem('i18nextLng')
  const normalizedLanguage = normalizeInterfaceLanguage(storedLanguage)
  if (storedLanguage && storedLanguage !== normalizedLanguage) {
    window.localStorage.setItem('i18nextLng', normalizedLanguage)
  }
}

// ─────────────────────────────────────────────────────────────────────────────
//  Lazy locale loading
//
//  Static `import` of all 11 locale JSONs previously bundled ~4 MB into the
//  initial entry chunk. We now register each locale as a dynamic import factory
//  so Rspack emits each language as its own async chunk; only the language
//  actually used (plus the `en` fallback) is fetched at runtime.
//
//  i18next contract: a backend plugin exposes `type: 'backend'` and a `read()`
//  method. i18next calls `read(lng, ns, cb)` for the active language and the
//  fallback language during init, then again whenever `changeLanguage()` is
//  called for a previously unloaded language.
// ─────────────────────────────────────────────────────────────────────────────

type Translations = Record<string, string>
// Each locale JSON is shaped as `{ translation: { key: value } }` to match
// i18next's default namespace layout. The runtime module shape varies by
// bundler (default-export wrapper / namespace-export edge cases), so we keep
// the type loose and unwrap defensively in `dynamicBackend.read`.
type LocaleModule = {
  default?: unknown
  translation?: Translations
}

const localeLoaders: Record<string, () => Promise<LocaleModule>> = {
  en: () => import('./locales/en.json') as unknown as Promise<LocaleModule>,
  'zh-CN': () =>
    import('./locales/zh-CN.json') as unknown as Promise<LocaleModule>,
  'zh-TW': () =>
    import('./locales/zh-TW.json') as unknown as Promise<LocaleModule>,
  fr: () => import('./locales/fr.json') as unknown as Promise<LocaleModule>,
  ru: () => import('./locales/ru.json') as unknown as Promise<LocaleModule>,
  ja: () => import('./locales/ja.json') as unknown as Promise<LocaleModule>,
  vi: () => import('./locales/vi.json') as unknown as Promise<LocaleModule>,
  ko: () => import('./locales/ko.json') as unknown as Promise<LocaleModule>,
  ar: () => import('./locales/ar.json') as unknown as Promise<LocaleModule>,
  'pt-BR': () =>
    import('./locales/pt-BR.json') as unknown as Promise<LocaleModule>,
  'es-ES': () =>
    import('./locales/es-ES.json') as unknown as Promise<LocaleModule>,
}

// Aliases: short codes share the same bundle as their canonical form so users
// can call `changeLanguage('zh')` and still get the zh-CN translations.
const localeAliasMap: Record<string, string> = {
  zh: 'zh-CN',
  pt: 'pt-BR',
  es: 'es-ES',
}

function resolveCanonicalLocale(lng: string): string {
  return localeAliasMap[lng] ?? lng
}

const dynamicBackend = {
  type: 'backend' as const,
  init: () => {},
  read: (
    lng: string,
    _ns: string,
    callback: (err: Error | null, data?: Translations) => void
  ) => {
    const canonical = resolveCanonicalLocale(lng)
    const loader = localeLoaders[canonical]
    if (!loader) {
      // Unknown language → return empty bundle so i18next falls back to `en`.
      callback(null, {})
      return
    }
    loader()
      .then((mod) => {
        // The JSON file may export either:
        //   1. `{ default: { translation: {...} } }` — Rspack JSON loader (most common)
        //   2. `{ default: { ...keys } }` — same module without translation wrapper
        //   3. `{ translation: {...} }` / `{ ...keys }` — non-default exports edge case
        // i18next's read(lng, ns, cb) expects the bundle for `ns` ('translation'),
        // so we unwrap as needed.
        const root =
          (mod as { default?: unknown }).default !== undefined
            ? ((mod as { default: unknown }).default as Record<string, unknown>)
            : (mod as unknown as Record<string, unknown>)

        const data = (
          root && typeof root === 'object' && 'translation' in root
            ? (root as { translation: Translations }).translation
            : (root as Translations)
        ) ?? {}

        callback(null, data)
      })
      .catch((err) => {
        callback(err instanceof Error ? err : new Error(String(err)))
      })
  },
}

i18n
  .use(dynamicBackend)
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    fallbackLng: 'en',
    supportedLngs: [
      'en',
      'zh-CN',
      'zh',
      'zh-TW',
      'fr',
      'ru',
      'ja',
      'vi',
      'ko',
      'ar',
      'pt-BR',
      'pt',
      'es-ES',
      'es',
    ],
    // Only load the active language (plus fallback). Without this, i18next
    // would also fetch e.g. `en-US` when the active language is `en`.
    load: 'currentOnly',
    // Allows i18next to mix bundled + backend-loaded resources without
    // emitting "missingKey" warnings while async chunks are still in flight.
    partialBundledLanguages: true,
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      convertDetectedLanguage: normalizeInterfaceLanguage,
    },
    react: {
      // While the active locale chunk is loading, t() returns the i18n key.
      // Our keys ARE the English source strings, so the user transiently sees
      // English instead of nothing — acceptable and avoids needing Suspense.
      useSuspense: false,
    },
  })

export default i18n
