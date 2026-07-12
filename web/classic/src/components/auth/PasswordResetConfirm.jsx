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

import React, { useEffect, useState } from 'react';
import {
  getLogo,
  getSystemName,
  showError,
  showInfo,
  showSuccess,
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  hasLetterAndNumber,
  resetPasswordByEmail,
} from '../../helpers';
import { useSearchParams, Link, useNavigate } from 'react-router-dom';
import { Button, Card, Form, Typography, Banner } from '@douyinfe/semi-ui';
import { IconMail, IconLock } from '@douyinfe/semi-icons';
import Turnstile from 'react-turnstile';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

const PasswordResetConfirm = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [inputs, setInputs] = useState({
    email: '',
    token: '',
    password: '',
    password2: '',
  });
  const [loading, setLoading] = useState(false);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');

  const { email, token, password, password2 } = inputs;
  const isValidResetLink = Boolean(email && token);
  const logo = getLogo();
  const systemName = getSystemName();

  useEffect(() => {
    setInputs((current) => ({
      ...current,
      token: searchParams.get('token') || '',
      email: searchParams.get('email') || '',
    }));
  }, [searchParams]);

  useEffect(() => {
    let status = localStorage.getItem('status');
    if (status) {
      status = JSON.parse(status);
      if (status.turnstile_check) {
        setTurnstileEnabled(true);
        setTurnstileSiteKey(status.turnstile_site_key);
      }
    }
  }, []);

  function handleChange(name, value) {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  }

  function ensureTurnstileReady() {
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('Please wait a moment, Turnstile is checking your environment'));
      return false;
    }
    return true;
  }

  async function handleSubmit() {
    if (!isValidResetLink) {
      showError(t('Invalid reset link, please request a new password reset'));
      return;
    }
    if (
      password.length < PASSWORD_MIN_LENGTH ||
      password.length > PASSWORD_MAX_LENGTH
    ) {
      showError(t('Password must be 8-20 characters long'));
      return;
    }
    if (!hasLetterAndNumber(password)) {
      showError(t('Password must contain letters and numbers'));
      return;
    }
    if (password !== password2) {
      showError(t("Passwords don't match."));
      return;
    }
    if (!ensureTurnstileReady()) return;

    setLoading(true);
    try {
      const res = await resetPasswordByEmail({
        email,
        token,
        password,
        turnstile: turnstileToken,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('Password updated successfully'));
        navigate('/login');
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('Password reset failed'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className='relative overflow-hidden min-h-screen flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8 bg-[linear-gradient(110deg,#f4f9ff_0%,#eaf3ff_38%,#cfe4ff_68%,#a8d4f5_100%)] text-slate-900'>
      {/* 柔和白色雾化层，提升表单可读性 */}
      <div
        className='pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.55)_0%,rgba(255,255,255,0.18)_55%,rgba(214,234,255,0.32)_100%)]'
      />
      {/* 右上方发光球体（对齐设计稿） */}
      <div
        className='pointer-events-none absolute rounded-full blur-2xl'
        style={{
          top: '-12%',
          right: '-12%',
          width: '760px',
          height: '760px',
          background:
            'radial-gradient(circle, rgba(120,220,245,0.65) 0%, rgba(150,205,255,0.45) 28%, rgba(180,220,255,0.18) 55%, transparent 75%)',
        }}
      />
      {/* 左上柔光高光 */}
      <div
        className='pointer-events-none absolute rounded-full blur-3xl'
        style={{
          top: '-15%',
          left: '8%',
          width: '420px',
          height: '420px',
          background:
            'radial-gradient(circle, rgba(255,255,255,0.6) 0%, rgba(235,246,255,0.25) 45%, transparent 70%)',
        }}
      />
      <div className='w-full max-w-sm mt-[60px]'>
        <div className='flex flex-col items-center'>
          <div className='w-full max-w-md'>
            <div className='flex items-center justify-center mb-6 gap-2'>
              <img src={logo} alt='Logo' className='h-10 rounded-full' />
              <Title heading={3} className='!text-gray-800'>
                {systemName}
              </Title>
            </div>

            <Card className='border-0 !rounded-2xl overflow-hidden'>
              <div className='flex justify-center pt-6 pb-2'>
                <Title heading={3} className='text-gray-800 dark:text-gray-200'>
                  {t('Enter new password')}
                </Title>
              </div>
              <div className='px-2 py-8'>
                {!isValidResetLink && (
                  <Banner
                    type='danger'
                    description={t('Invalid reset link, please request a new password reset.')}
                    className='mb-4 !rounded-lg'
                    closeIcon={null}
                  />
                )}
                <Form className='space-y-4'>
                  <Form.Input
                    field='email'
                    label={t('Email')}
                    name='email'
                    value={email}
                    disabled={true}
                    prefix={<IconMail />}
                    placeholder={t('Waiting for email...')}
                  />
                  <Form.Input
                    field='password'
                    label={t('New password')}
                    name='password'
                    mode='password'
                    value={password}
                    onChange={(value) => handleChange('password', value)}
                    prefix={<IconLock />}
                    placeholder={t('Password must be 8-20 characters and contain letters and numbers')}
                  />
                  <Form.Input
                    field='password2'
                    label={t('Confirm password')}
                    name='password2'
                    mode='password'
                    value={password2}
                    onChange={(value) => handleChange('password2', value)}
                    prefix={<IconLock />}
                    placeholder={t('Confirm password')}
                  />
                  <Text size='small' type='tertiary'>
                    {t('Password must be 8-20 characters and contain letters and numbers')}
                  </Text>

                  <div className='grid grid-cols-2 gap-3 pt-2'>
                    <Button
                      className='w-full !rounded-full'
                      type='tertiary'
                      onClick={() => navigate('/login')}
                      disabled={loading}
                    >
                      {t('Back')}
                    </Button>
                    <Button
                      theme='solid'
                      className='w-full !rounded-full'
                      type='primary'
                      htmlType='submit'
                      onClick={handleSubmit}
                      loading={loading}
                      disabled={!isValidResetLink}
                    >
                      {t('Change password')}
                    </Button>
                  </div>
                </Form>

                <div className='mt-6 text-center text-sm'>
                  <Text>
                    <Link
                      to='/login'
                      className='text-blue-600 hover:text-blue-800 font-medium'
                    >
                      {t('Back to login')}
                    </Link>
                  </Text>
                </div>
              </div>
            </Card>

            {turnstileEnabled && (
              <div className='flex justify-center mt-6'>
                <Turnstile
                  sitekey={turnstileSiteKey}
                  onVerify={(token) => {
                    setTurnstileToken(token);
                  }}
                />
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default PasswordResetConfirm;
