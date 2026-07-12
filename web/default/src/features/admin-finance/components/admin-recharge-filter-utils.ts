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

export type AdminRechargeFilterDraft = {
  userId: string
  email: string
  phoneNumber: string
  tradeNo: string
  status: string
  paymentProvider: string
  paymentMethod: string
  createdFrom: string
  createdTo: string
  eventFrom: string
  eventTo: string
}

export function createEmptyAdminRechargeFilterDraft(): AdminRechargeFilterDraft {
  return {
    userId: '',
    email: '',
    phoneNumber: '',
    tradeNo: '',
    status: '',
    paymentProvider: '',
    paymentMethod: '',
    createdFrom: '',
    createdTo: '',
    eventFrom: '',
    eventTo: '',
  }
}

export function trimToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed === '' ? undefined : trimmed
}

export function datetimeLocalToUnixSeconds(value: string) {
  if (!value) return undefined
  const time = new Date(value).getTime()
  if (!Number.isFinite(time)) return undefined
  return Math.floor(time / 1000)
}
