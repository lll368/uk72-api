import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { getVerificationAccountValidationMessage } from './verification-code'

describe('getVerificationAccountValidationMessage', () => {
  test('asks for an account before sending a verification code', () => {
    assert.equal(
      getVerificationAccountValidationMessage(''),
      'Please enter your email or phone number first'
    )
  })

  test('rejects malformed accounts before any verification request', () => {
    assert.equal(
      getVerificationAccountValidationMessage('legacy_user'),
      'Please enter a valid email or phone number'
    )
  })

  test('allows email and phone accounts', () => {
    assert.equal(
      getVerificationAccountValidationMessage('person@example.com'),
      null
    )
    assert.equal(getVerificationAccountValidationMessage('13800138000'), null)
  })
})
