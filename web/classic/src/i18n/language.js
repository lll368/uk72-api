/*
Copyright (C) 2025 QuantumNous

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

export const languageOptions = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'en', label: 'English' },
  { value: 'fr', label: 'Français' },
  { value: 'ru', label: 'Русский' },
  { value: 'ja', label: '日本語' },
  { value: 'vi', label: 'Tiếng Việt' },
  { value: 'ko', label: '한국어' },
  { value: 'ar', label: 'العربية' },
];

export const supportedLanguages = languageOptions.map(({ value }) => value);

export const normalizeLanguage = (language) => {
  if (!language) {
    return language;
  }

  const normalized = language.trim().replace(/_/g, '-');
  const lower = normalized.toLowerCase();

  if (
    lower === 'zh' ||
    lower === 'zh-cn' ||
    lower === 'zh-sg' ||
    lower.startsWith('zh-hans')
  ) {
    return 'zh-CN';
  }

  if (
    lower === 'zh-tw' ||
    lower === 'zh-hk' ||
    lower === 'zh-mo' ||
    lower.startsWith('zh-hant')
  ) {
    return 'zh-TW';
  }

  if (lower === 'en' || lower.startsWith('en-')) {
    return 'en';
  }

  if (lower === 'fr' || lower.startsWith('fr-')) {
    return 'fr';
  }

  if (lower === 'ru' || lower.startsWith('ru-')) {
    return 'ru';
  }

  if (lower === 'ja' || lower.startsWith('ja-')) {
    return 'ja';
  }

  if (lower === 'vi' || lower.startsWith('vi-')) {
    return 'vi';
  }

  if (lower === 'ko' || lower.startsWith('ko-')) {
    return 'ko';
  }

  if (lower === 'ar' || lower.startsWith('ar-')) {
    return 'ar';
  }

  const matchedLanguage = supportedLanguages.find(
    (supportedLanguage) => supportedLanguage.toLowerCase() === lower,
  );

  return matchedLanguage || normalized;
};
