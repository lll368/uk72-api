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
export type AuthAccountType = 'email' | 'phone'

export interface ParsedAuthAccount {
  type: AuthAccountType
  value: string
}

const E164_PHONE_REGEX = /^\+[1-9]\d{7,14}$/
const MAINLAND_PHONE_REGEX = /^1\d{10}$/
const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function normalizeAuthAccount(account: string): ParsedAuthAccount | null {
  const value = account.trim()
  if (!value) return null

  const lowerValue = value.toLowerCase()
  if (EMAIL_REGEX.test(lowerValue)) {
    return { type: 'email', value: lowerValue }
  }
  if (E164_PHONE_REGEX.test(value) || MAINLAND_PHONE_REGEX.test(value)) {
    return { type: 'phone', value }
  }
  return null
}

export function isValidAuthAccount(account: string): boolean {
  return normalizeAuthAccount(account) !== null
}

export function hasLetterAndNumber(password: string): boolean {
  return /[A-Za-z]/.test(password) && /\d/.test(password)
}
