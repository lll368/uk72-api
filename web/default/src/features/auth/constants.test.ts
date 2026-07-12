import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  forgotPasswordFormSchema,
  registerFormSchema,
  resetPasswordConfirmFormSchema,
} from './constants'

describe('forgotPasswordFormSchema', () => {
  test('rejects malformed email when email reset is used', () => {
    const result = forgotPasswordFormSchema.safeParse({
      email: 'not-an-email',
      phoneNumber: '',
      verificationCode: '',
    })

    assert.equal(result.success, false)
  })

  test('allows empty email so phone reset can use the same form', () => {
    const result = forgotPasswordFormSchema.safeParse({
      email: '',
      phoneNumber: '13800138000',
      verificationCode: '123456',
    })

    assert.equal(result.success, true)
  })

  test('rejects account values that are not email or phone number', () => {
    const result = forgotPasswordFormSchema.safeParse({
      account: 'legacy_user',
      phoneNumber: '',
      verificationCode: '123456',
    })

    assert.equal(result.success, false)
  })
})

describe('registerFormSchema', () => {
  test('accepts email account with strong password', () => {
    const result = registerFormSchema.safeParse({
      account: 'person@example.com',
      verificationCode: '123456',
      password: 'password123',
      confirmPassword: 'password123',
    })

    assert.equal(result.success, true)
  })

  test('accepts mainland phone account with strong password', () => {
    const result = registerFormSchema.safeParse({
      account: '13800138000',
      verificationCode: '123456',
      password: 'password123',
      confirmPassword: 'password123',
    })

    assert.equal(result.success, true)
  })

  test('rejects weak password without numbers', () => {
    const result = registerFormSchema.safeParse({
      account: 'person@example.com',
      verificationCode: '123456',
      password: 'abcdefgh',
      confirmPassword: 'abcdefgh',
    })

    assert.equal(result.success, false)
  })

  test('rejects mismatched password confirmation', () => {
    const result = registerFormSchema.safeParse({
      account: 'person@example.com',
      verificationCode: '123456',
      password: 'password123',
      confirmPassword: 'password124',
    })

    assert.equal(result.success, false)
  })

  test('rejects missing password confirmation', () => {
    const result = registerFormSchema.safeParse({
      account: 'person@example.com',
      verificationCode: '123456',
      password: 'password123',
    })

    assert.equal(result.success, false)
  })
})

describe('resetPasswordConfirmFormSchema', () => {
  test('rejects weak reset link password without letters', () => {
    const result = resetPasswordConfirmFormSchema.safeParse({
      password: '12345678',
      confirmPassword: '12345678',
    })

    assert.equal(result.success, false)
  })

  test('accepts reset link password with letters and numbers', () => {
    const result = resetPasswordConfirmFormSchema.safeParse({
      password: 'newpass123',
      confirmPassword: 'newpass123',
    })

    assert.equal(result.success, true)
  })
})
