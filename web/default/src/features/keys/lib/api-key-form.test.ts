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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  getApiKeyCreateFormDefaultValues,
  transformFormDataToPayload,
} from './api-key-form'

describe('api key form helpers', () => {
  test('create defaults are fixed to default group and one key', () => {
    const defaults = getApiKeyCreateFormDefaultValues()

    assert.equal(defaults.group, 'default')
    assert.equal(defaults.cross_group_retry, false)
    assert.equal(defaults.tokenCount, 1)
  })

  test('create payload keeps the default group instead of auto group', () => {
    const payload = transformFormDataToPayload({
      ...getApiKeyCreateFormDefaultValues(),
      name: 'api-key',
    })

    assert.equal(payload.group, 'default')
    assert.equal(payload.cross_group_retry, false)
  })
})
