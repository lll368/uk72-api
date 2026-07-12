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
declare module 'bun:test' {
  export const describe: (name: string, fn: () => void) => void
  export const test: (name: string, fn: () => void | Promise<void>) => void
  export const expect: (actual: unknown) => {
    toBe(expected: unknown): void
    toEqual(expected: unknown): void
    toStrictEqual(expected: unknown): void
    toBeTruthy(): void
    toBeFalsy(): void
    toBeDefined(): void
    toBeUndefined(): void
    toBeNull(): void
    toContain(expected: unknown): void
  }
}
