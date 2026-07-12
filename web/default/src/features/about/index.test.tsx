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
import { describe, expect, test } from 'bun:test'
import * as bunTest from 'bun:test'
import { renderToStaticMarkup } from 'react-dom/server'
import { EmptyAboutState } from '.'

const mock = (
  bunTest as unknown as {
    mock: { module: (specifier: string, factory: () => unknown) => void }
  }
).mock

mock.module('react-i18next', () => ({
  useTranslation: () => ({
    t: (value: string) => value,
  }),
}))

describe('EmptyAboutState', () => {
  test('links both the uk72-api public source and upstream project', () => {
    const html = renderToStaticMarkup(<EmptyAboutState />)

    expect(html).toContain('href="https://github.com/lll368/uk72-api"')
    expect(html).toContain('href="https://github.com/QuantumNous/new-api"')
    expect(html).toContain('uk72-api')
    expect(html).toContain('QuantumNous/new-api')
    expect(html).toContain('AGPL v3.0 License')
  })
})
