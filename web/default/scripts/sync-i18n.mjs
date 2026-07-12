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
import fs from 'node:fs/promises'
import path from 'node:path'

// This script is executed from the web/ package root (see package.json script).
const LOCALES_DIR = path.resolve('src/i18n/locales')
const SRC_DIR = path.resolve('src')
const BASE_LOCALE = 'en'
const FALLBACK_COMPARE_LOCALE = 'en' // used for "still English" detection only
const ZH_COMPARE_LOCALE = 'zh-CN'
const CJK_ALLOWED_LOCALES = new Set(['zh', 'zh-CN', 'zh-TW', 'ja'])
const SOURCE_IGNORED_DIRS = new Set(['node_modules', '.git', 'locales', '_reports', '_extras'])
const SOURCE_FILE_PATTERN = /\.(tsx?|jsx?)$/
const STATIC_T_CALL_PATTERNS = [
  /\bt\(\s*['"`]([^'"`\n]+?)['"`]\s*[,)]/g,
  /\bt\(\s*['"`]([^'"`]+?)['"`]\s*\)/g,
]
const OBFUSCATED_KEYS = [
  {
    runtime: ['footer', 'new' + 'api', 'projectAttributionSuffix'].join('.'),
    serialized: 'footer.new\\u0061pi.projectAttributionSuffix',
  },
]

const ALLOW_EXACT = new Set([
  'AIGC2D',
  'Anthropic',
  'API2GPT',
  'Azure',
  'Claude',
  'Cloudflare',
  'Codex',
  'Cohere',
  'Coze',
  'DeepSeek',
  'Dify',
  'Discord',
  'DoubaoVideo',
  'Epay',
  'FastGPT',
  'Gemini',
  'GitHub',
  'Grok',
  'Jimeng',
  'Jina',
  'JustSong',
  'Kling',
  'LingYiWanWu',
  'LinuxDO',
  'Midjourney',
  'MidjourneyPlus',
  'MiniMax',
  'Mistral',
  'MokaAI',
  'Moonshot',
  'New API',
  'NewAPI',
  'OhMyGPT',
  'Ollama',
  'One API',
  'OpenAI',
  'OpenAIMax',
  'OpenRouter',
  'PaLM',
  'Passkey',
  'Perplexity',
  'Piggy Labor V3',
  'QuantumNous',
  'Replicate',
  'SiliconFlow',
  'Sora',
  'Stripe',
  'Submodel',
  'SunoAPI',
  'Telegram',
  'Tencent',
  'Uptime Kuma',
  'Vertex AI',
  'Vidu',
  'VolcEngine',
  'WeChat',
  'Xinference',
  'Xunfei',
  'vip',
  'xAI',
  'AI Proxy',
  'AK/SK mode: use AccessKey|SecretAccessKey|Region',
  'Ali',
  'Alipay',
  'Alipay Direct',
  'API URL',
  'API Base URL *',
  'AccessKey / SecretAccessKey',
  'AZURE_OPENAI_ENDPOINT *',
  'AWS Bedrock Claude Compat',
  'Base URL',
  'Baidu',
  'Baidu V2',
  'Client ID',
  'Client Secret',
  'Epay Gateway',
  'Full API Key',
  'Full Base URL (supports',
  'Gemini Image 4K',
  'Huawei Ascend GPU',
  'JSON Editor',
  'Logo URL',
  'ms',
  'Quota:',
  'Password / Access Token',
  'TTFT P50',
  'TTFT P95',
  'TTFT P99',
  'Stripe Gateway',
  'Uptime Kuma URL',
  'User ID',
  'Webhook URL:',
  'Webhook URL',
  'WeChat Pay',
  'WeChat Pay Direct',
  'Well-Known URL',
  'Worker URL',
  'Zhipu',
  'Zhipu V4',
])

const ALLOW_PATTERNS = [
  /^https?:\/\//,
  /^\/(?:status|your)\//,
  /^smtp\./,
  /^socks5:/,
  /^name@/,
  /^noreply@/,
  /&lt;[^&]+@[^&]+&gt;/,
  /^\{\{value\}\}(?:ms|s)$/,
  /^example\.com/,
  /^checkout\./,
  /^footer\.columns\.related\.links\./,
  /^org-/,
  /^price_/,
  /^whsec_/,
  /^edit_this$/,
  /^_copy$/,
  /^\[/,
  /^\{(?!\{)/,
  /^"/,
  /^gpt-/,
  /^claude-/,
  /^o\d/,
  /^[A-Z0-9_ *:/.-]+$/,
  /^[0-9.,%+\-#|(){}\s]+$/,
  /^\d+\s*\/\s*page$/,
  /^\d{1,3}(?:\.\d{1,3}){3}/,
  /^tokens\s*\/\s*mo$/,
  /^@\w/,
]

function isPlainObject(v) {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

async function walkSourceFiles(dir) {
  const files = []
  const entries = await fs.readdir(dir, { withFileTypes: true })

  for (const entry of entries) {
    if (entry.isDirectory()) {
      if (SOURCE_IGNORED_DIRS.has(entry.name)) continue
      files.push(...(await walkSourceFiles(path.join(dir, entry.name))))
      continue
    }

    if (entry.isFile() && SOURCE_FILE_PATTERN.test(entry.name)) {
      files.push(path.join(dir, entry.name))
    }
  }

  return files
}

async function findMissingSourceTranslationKeys(baseTrans) {
  const baseKeys = new Set(Object.keys(isPlainObject(baseTrans) ? baseTrans : {}))
  const missingKeys = {}
  const files = await walkSourceFiles(SRC_DIR)

  for (const file of files) {
    const content = await fs.readFile(file, 'utf8')
    const relativeFile = path.relative(SRC_DIR, file)

    for (const pattern of STATIC_T_CALL_PATTERNS) {
      pattern.lastIndex = 0
      let match
      while ((match = pattern.exec(content)) !== null) {
        const key = match[1]
        if (key.startsWith('{{') || key.includes('${')) continue
        if (baseKeys.has(key)) continue

        missingKeys[key] ??= []
        if (!missingKeys[key].includes(relativeFile)) {
          missingKeys[key].push(relativeFile)
        }
      }
    }
  }

  return Object.fromEntries(
    Object.entries(missingKeys)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, files]) => [key, files.sort((a, b) => a.localeCompare(b))]),
  )
}

function stableStringify(obj) {
  let text = JSON.stringify(obj, null, 2)
  for (const key of OBFUSCATED_KEYS) {
    text = text.replaceAll(`"${key.runtime}":`, `"${key.serialized}":`)
  }
  return text + '\n'
}

function reorderLikeBase(base, target, fill, extras, missing, currentPath = []) {
  // If base is an object, we keep base's key order and recurse.
  if (isPlainObject(base)) {
    const out = {}
    const t = isPlainObject(target) ? target : {}
    const f = isPlainObject(fill) ? fill : {}

    for (const key of Object.keys(base)) {
      const nextPath = [...currentPath, key]
      if (Object.prototype.hasOwnProperty.call(t, key)) {
        out[key] = reorderLikeBase(base[key], t[key], f[key], extras, missing, nextPath)
      } else {
        missing.push(nextPath.join('.'))
        out[key] = reorderLikeBase(base[key], undefined, f[key], extras, missing, nextPath)
      }
    }

    for (const key of Object.keys(t)) {
      if (!Object.prototype.hasOwnProperty.call(base, key)) {
        const nextPath = [...currentPath, key].join('.')
        extras[nextPath] = t[key]
      }
    }

    return out
  }

  // For arrays: prefer target if it's also an array; otherwise use base.
  if (Array.isArray(base)) {
    if (Array.isArray(target)) return target
    if (Array.isArray(fill)) return fill
    return base
  }

  // For primitives: prefer target if defined, else base.
  return target === undefined ? (fill ?? base) : target
}

function isAllowedUntranslated(key, value) {
  const s = String(value ?? '').trim()
  return (
    ALLOW_EXACT.has(key) ||
    ALLOW_EXACT.has(s) ||
    ALLOW_PATTERNS.some((pattern) => pattern.test(key)) ||
    ALLOW_PATTERNS.some((pattern) => pattern.test(s))
  )
}

function hasUnexpectedCjk(locale, value) {
  if (CJK_ALLOWED_LOCALES.has(locale)) return false
  const s = String(value ?? '')
  if (!/[\u4e00-\u9fff]/.test(s)) return false
  return s.replaceAll('验证码', '').match(/[\u4e00-\u9fff]/) !== null
}

function isLikelyUntranslated({ locale, key, baseValue, zhValue, value }) {
  if (typeof value !== 'string' || typeof baseValue !== 'string') return false
  if (isAllowedUntranslated(key, value)) return false

  const s = value.trim()
  if (s.length < 2) return false

  if (value === baseValue) return true
  if (!CJK_ALLOWED_LOCALES.has(locale) && typeof zhValue === 'string' && value === zhValue) return true
  if (hasUnexpectedCjk(locale, value)) return true

  if (locale === 'ru' && /[A-Za-z]{3,}/.test(s) && !/[А-Яа-яЁё]/.test(s)) return true
  if (locale === 'ko' && /[A-Za-z]{3,}/.test(s) && !/[\uac00-\ud7af\u1100-\u11ff\u3130-\u318f]/.test(s)) return true
  if (locale === 'ar' && /[A-Za-z]{3,}/.test(s) && !/[\u0600-\u06ff\u0750-\u077f\u08a0-\u08ff]/.test(s)) return true
  if (locale === 'ko' && /[\u0600-\u06ff\u0750-\u077f\u08a0-\u08ff]/.test(s)) return true
  if (locale === 'ar' && /[\uac00-\ud7af\u1100-\u11ff\u3130-\u318f]/.test(s)) return true

  return false
}

async function main() {
  const entries = await fs.readdir(LOCALES_DIR, { withFileTypes: true })
  const localeFiles = entries
    .filter((e) => e.isFile() && e.name.endsWith('.json'))
    .map((e) => e.name)
    .sort((a, b) => a.localeCompare(b))

  // Keep English as the canonical key/value baseline so equal key counts never make base drift.
  const parsedByLocale = {}
  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const raw = await fs.readFile(path.join(LOCALES_DIR, filename), 'utf8')
    parsedByLocale[locale] = JSON.parse(raw)
  }

  const baseLocale = BASE_LOCALE

  if (!parsedByLocale[baseLocale]) throw new Error(`Base locale ${baseLocale}.json not found.`)

  const baseFile = `${baseLocale}.json`
  const baseJson = parsedByLocale[baseLocale]

  const compareJson = parsedByLocale[FALLBACK_COMPARE_LOCALE] ?? baseJson
  const sourceMissingKeys = await findMissingSourceTranslationKeys(baseJson.translation)
  const sourceMissingCount = Object.keys(sourceMissingKeys).length

  const report = {
    base: baseFile,
    sourceMissingCount,
    sourceMissingKeys,
    locales: {},
  }

  const extrasDir = path.join(LOCALES_DIR, '_extras')
  const reportsDir = path.join(LOCALES_DIR, '_reports')
  await fs.mkdir(extrasDir, { recursive: true })
  await fs.mkdir(reportsDir, { recursive: true })

  const sourceMissingReport = path.join(reportsDir, '_source-missing.json')
  if (sourceMissingCount > 0) {
    await fs.writeFile(sourceMissingReport, stableStringify(sourceMissingKeys), 'utf8')
  } else {
    await fs.rm(sourceMissingReport, { force: true })
  }

  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const full = path.join(LOCALES_DIR, filename)
    const json = parsedByLocale[locale]

    const extras = {}
    const missing = []
    const fixed = reorderLikeBase(baseJson, json, compareJson, extras, missing)

    // Untranslated scan (translation namespace only)
    const untranslated = {}
    const compareTrans = compareJson?.translation ?? {}
    const zhTrans = parsedByLocale[ZH_COMPARE_LOCALE]?.translation ?? {}
    const trans = fixed?.translation ?? {}
    if (
      isPlainObject(compareTrans) &&
      isPlainObject(trans) &&
      locale !== baseLocale
    ) {
      for (const k of Object.keys(compareTrans)) {
        const baseValue = compareTrans[k]
        const zhValue = zhTrans[k]
        const value = trans[k]
        if (isLikelyUntranslated({ locale, key: k, baseValue, zhValue, value })) {
          untranslated[k] = value
        }
      }
    }

    report.locales[locale] = {
      file: filename,
      missingCount: missing.length,
      extrasCount: Object.keys(extras).length,
      untranslatedCount: Object.keys(untranslated).length,
    }

    if (Object.keys(extras).length > 0) {
      await fs.writeFile(path.join(extrasDir, `${locale}.extras.json`), stableStringify(extras), 'utf8')
    }
    if (Object.keys(untranslated).length > 0) {
      await fs.writeFile(
        path.join(reportsDir, `${locale}.untranslated.json`),
        stableStringify(untranslated),
        'utf8',
      )
    } else {
      await fs.rm(path.join(reportsDir, `${locale}.untranslated.json`), { force: true })
    }

    // Rewrite locale file in base order (even for en to normalize formatting)
    await fs.writeFile(full, stableStringify(fixed), 'utf8')
  }

  await fs.writeFile(path.join(reportsDir, '_sync-report.json'), stableStringify(report), 'utf8')

  if (sourceMissingCount > 0) {
    console.error(
      `i18n sync found ${sourceMissingCount} static t() key(s) missing from ${baseFile}. Report: ${sourceMissingReport}`,
    )
    process.exitCode = 1
    return
  }
   
  console.log(`i18n sync done. Report: ${path.join(reportsDir, '_sync-report.json')}`)
}

main().catch((err) => {
   
  console.error(err)
  process.exitCode = 1
})
