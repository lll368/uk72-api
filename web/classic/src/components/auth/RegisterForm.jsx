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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  API,
  getLogo,
  showError,
  showInfo,
  showSuccess,
  updateAPI,
  getSystemName,
  getOAuthProviderIcon,
  setUserData,
  onDiscordOAuthClicked,
  onCustomOAuthClicked,
  PHONE_VERIFICATION_COUNTDOWN,
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  buildEmailRegisterPayload,
  getPhoneCodeButtonText,
  hasLetterAndNumber,
  normalizeAuthAccount,
  registerByPhone,
  sendAccountVerificationCode,
} from '../../helpers';
import Turnstile from 'react-turnstile';
import {
  Button,
  Card,
  Checkbox,
  Divider,
  Form,
  Icon,
  Modal,
} from '@douyinfe/semi-ui';
import Title from '@douyinfe/semi-ui/lib/es/typography/title';
import Text from '@douyinfe/semi-ui/lib/es/typography/text';
import {
  IconGithubLogo,
  IconMail,
  IconLock,
  IconKey,
} from '@douyinfe/semi-icons';
import {
  onGitHubOAuthClicked,
  onLinuxDOOAuthClicked,
  onOIDCClicked,
} from '../../helpers';
import OIDCIcon from '../common/logo/OIDCIcon';
import LinuxDoIcon from '../common/logo/LinuxDoIcon';
import WeChatIcon from '../common/logo/WeChatIcon';
import TelegramLoginButton from 'react-telegram-login/src';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import { useTranslation } from 'react-i18next';
import { SiDiscord } from 'react-icons/si';
import { Smartphone } from 'lucide-react';

const RegisterForm = () => {
  let navigate = useNavigate();
  const { t } = useTranslation();
  const githubButtonTextKeyByState = {
    idle: '使用 GitHub 继续',
    redirecting: '正在跳转 GitHub...',
    timeout: '请求超时，请刷新页面后重新发起 GitHub 登录',
  };
  const [inputs, setInputs] = useState({
    password: '',
    password2: '',
    email: '',
    verification_code: '',
    phone_number: '',
    phone_verification_code: '',
    wechat_verification_code: '',
  });
  const { password, password2 } = inputs;
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [showWeChatLoginModal, setShowWeChatLoginModal] = useState(false);
  const [showEmailRegister, setShowEmailRegister] = useState(false);
  const [wechatLoading, setWechatLoading] = useState(false);
  const [githubLoading, setGithubLoading] = useState(false);
  const [discordLoading, setDiscordLoading] = useState(false);
  const [oidcLoading, setOidcLoading] = useState(false);
  const [linuxdoLoading, setLinuxdoLoading] = useState(false);
  const [emailRegisterLoading, setEmailRegisterLoading] = useState(false);
  const [registerLoading, setRegisterLoading] = useState(false);
  const [verificationCodeLoading, setVerificationCodeLoading] = useState(false);
  const [otherRegisterOptionsLoading, setOtherRegisterOptionsLoading] =
    useState(false);
  const [wechatCodeSubmitLoading, setWechatCodeSubmitLoading] = useState(false);
  const [customOAuthLoading, setCustomOAuthLoading] = useState({});
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(PHONE_VERIFICATION_COUNTDOWN);
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [hasUserAgreement, setHasUserAgreement] = useState(false);
  const [hasPrivacyPolicy, setHasPrivacyPolicy] = useState(false);
  const [githubButtonState, setGithubButtonState] = useState('idle');
  const [githubButtonDisabled, setGithubButtonDisabled] = useState(false);
  const githubTimeoutRef = useRef(null);
  const githubButtonText = t(githubButtonTextKeyByState[githubButtonState]);
  const [registerMode, setRegisterMode] = useState('email');
  const [phoneCodeLoading, setPhoneCodeLoading] = useState(false);
  const [phoneCountdown, setPhoneCountdown] = useState(0);

  const logo = getLogo();
  const systemName = getSystemName();

  let affCode = new URLSearchParams(window.location.search).get('aff');
  if (affCode) {
    localStorage.setItem('aff', affCode);
  }

  const status = useMemo(() => {
    if (statusState?.status) return statusState.status;
    const savedStatus = localStorage.getItem('status');
    if (!savedStatus) return {};
    try {
      return JSON.parse(savedStatus) || {};
    } catch (err) {
      return {};
    }
  }, [statusState?.status]);
  const hasCustomOAuthProviders =
    (status.custom_oauth_providers || []).length > 0;
  const hasOAuthRegisterOptions = Boolean(
    status.github_oauth ||
      status.discord_oauth ||
      status.oidc_enabled ||
      status.wechat_login ||
      status.linuxdo_oauth ||
      status.telegram_oauth ||
      hasCustomOAuthProviders,
  );

  useEffect(() => {
    if (status?.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(status.turnstile_site_key);
    }

    // 从 status 获取用户协议和隐私政策的启用状态
    setHasUserAgreement(status?.user_agreement_enabled || false);
    setHasPrivacyPolicy(status?.privacy_policy_enabled || false);
  }, [status]);

  useEffect(() => {
    let countdownInterval = null;
    if (disableButton && countdown > 0) {
      countdownInterval = setInterval(() => {
        setCountdown(countdown - 1);
      }, 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(PHONE_VERIFICATION_COUNTDOWN);
    }
    return () => clearInterval(countdownInterval); // Clean up on unmount
  }, [disableButton, countdown]);

  useEffect(() => {
    return () => {
      if (githubTimeoutRef.current) {
        clearTimeout(githubTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (phoneCountdown <= 0) return undefined;
    const timer = setInterval(() => {
      setPhoneCountdown((value) => Math.max(0, value - 1));
    }, 1000);
    return () => clearInterval(timer);
  }, [phoneCountdown]);

  const onWeChatLoginClicked = () => {
    setWechatLoading(true);
    setShowWeChatLoginModal(true);
    setWechatLoading(false);
  };

  const onSubmitWeChatVerificationCode = async () => {
    if (turnstileEnabled && turnstileToken === '') {
      showInfo('请稍后几秒重试，Turnstile 正在检查用户环境！');
      return;
    }
    setWechatCodeSubmitLoading(true);
    try {
      const res = await API.get(
        `/api/oauth/wechat?code=${inputs.wechat_verification_code}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        setUserData(data);
        updateAPI();
        navigate('/');
        showSuccess('登录成功！');
        setShowWeChatLoginModal(false);
      } else {
        showError(message);
      }
    } catch (error) {
      showError('登录失败，请重试');
    } finally {
      setWechatCodeSubmitLoading(false);
    }
  };

  function handleChange(name, value) {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  }

  async function handleSubmit(e) {
    e?.preventDefault?.();
    const account = registerMode === 'phone' ? inputs.phone_number : inputs.email;
    const parsedAccount = normalizeAuthAccount(account);
    if (!parsedAccount || parsedAccount.type !== registerMode) {
      showInfo(t('Please enter a valid email or phone number'));
      return;
    }
    if (
      password.length < PASSWORD_MIN_LENGTH ||
      password.length > PASSWORD_MAX_LENGTH
    ) {
      showInfo(t('Password must be 8-20 characters long'));
      return;
    }
    if (!hasLetterAndNumber(password)) {
      showInfo(t('Password must contain letters and numbers'));
      return;
    }
    if (password !== password2) {
      showInfo(t("Passwords don't match."));
      return;
    }
    if (registerMode === 'phone') {
      if (!inputs.phone_number) {
        showInfo(t('请输入手机号'));
        return;
      }
      if (!inputs.phone_verification_code) {
        showInfo(t('请输入验证码'));
        return;
      }
    } else {
      if (!inputs.email) {
        showInfo(t('请输入邮箱地址'));
        return;
      }
      if (!inputs.verification_code) {
        showInfo(t('请输入验证码'));
        return;
      }
    }
    if (password) {
      if (turnstileEnabled && turnstileToken === '') {
        showInfo('请稍后几秒重试，Turnstile 正在检查用户环境！');
        return;
      }
      setRegisterLoading(true);
      try {
        if (!affCode) {
          affCode = localStorage.getItem('aff');
        }
        const res =
          registerMode === 'phone'
            ? await registerByPhone({
                username: parsedAccount.value,
                password,
                phoneNumber: parsedAccount.value,
                verificationCode: inputs.phone_verification_code,
                affCode,
                turnstile: turnstileToken,
              })
            : await API.post(
                `/api/user/register?turnstile=${turnstileToken}`,
                buildEmailRegisterPayload({
                  email: parsedAccount.value,
                  password,
                  verificationCode: inputs.verification_code,
                  affCode,
                }),
              );
        const { success, message } = res.data;
        if (success) {
          navigate('/login');
          showSuccess('注册成功！');
        } else {
          showError(message);
        }
      } catch (error) {
        showError('注册失败，请重试');
      } finally {
        setRegisterLoading(false);
      }
    }
  }

  const sendVerificationCode = async () => {
    const parsedAccount = normalizeAuthAccount(inputs.email);
    if (!parsedAccount || parsedAccount.type !== 'email') {
      showInfo(t('Please enter a valid email or phone number'));
      return;
    }
    if (inputs.email === '') {
      showInfo(t('请输入邮箱地址'));
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo('请稍后几秒重试，Turnstile 正在检查用户环境！');
      return;
    }
    setVerificationCodeLoading(true);
    try {
      const res = await sendAccountVerificationCode({
        account: parsedAccount.value,
        purpose: 'register',
        turnstile: turnstileToken,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess('验证码发送成功，请检查你的邮箱！');
        setDisableButton(true); // 发送成功后禁用按钮，开始倒计时
      } else {
        showError(message);
      }
    } catch (error) {
      showError('发送验证码失败，请重试');
    } finally {
      setVerificationCodeLoading(false);
    }
  };

  const sendPhoneVerification = async () => {
    const parsedAccount = normalizeAuthAccount(inputs.phone_number);
    if (!parsedAccount || parsedAccount.type !== 'phone') {
      showInfo(t('Please enter a valid email or phone number'));
      return;
    }
    if (!inputs.phone_number) {
      showInfo(t('请输入手机号'));
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setPhoneCodeLoading(true);
    try {
      const res = await sendAccountVerificationCode({
        account: parsedAccount.value,
        purpose: 'register',
        turnstile: turnstileToken,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('验证码发送成功'));
        setPhoneCountdown(PHONE_VERIFICATION_COUNTDOWN);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('验证码发送失败'));
    } finally {
      setPhoneCodeLoading(false);
    }
  };

  const handleGitHubClick = () => {
    if (githubButtonDisabled) {
      return;
    }
    setGithubLoading(true);
    setGithubButtonDisabled(true);
    setGithubButtonState('redirecting');
    if (githubTimeoutRef.current) {
      clearTimeout(githubTimeoutRef.current);
    }
    githubTimeoutRef.current = setTimeout(() => {
      setGithubLoading(false);
      setGithubButtonState('timeout');
      setGithubButtonDisabled(true);
    }, 20000);
    try {
      onGitHubOAuthClicked(status.github_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setGithubLoading(false), 3000);
    }
  };

  const handleDiscordClick = () => {
    setDiscordLoading(true);
    try {
      onDiscordOAuthClicked(status.discord_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setDiscordLoading(false), 3000);
    }
  };

  const handleOIDCClick = () => {
    setOidcLoading(true);
    try {
      onOIDCClicked(
        status.oidc_authorization_endpoint,
        status.oidc_client_id,
        false,
        { shouldLogout: true },
      );
    } finally {
      setTimeout(() => setOidcLoading(false), 3000);
    }
  };

  const handleLinuxDOClick = () => {
    setLinuxdoLoading(true);
    try {
      onLinuxDOOAuthClicked(status.linuxdo_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setLinuxdoLoading(false), 3000);
    }
  };

  const handleCustomOAuthClick = (provider) => {
    setCustomOAuthLoading((prev) => ({ ...prev, [provider.slug]: true }));
    try {
      onCustomOAuthClicked(provider, { shouldLogout: true });
    } finally {
      setTimeout(() => {
        setCustomOAuthLoading((prev) => ({ ...prev, [provider.slug]: false }));
      }, 3000);
    }
  };

  const handleEmailRegisterClick = () => {
    setEmailRegisterLoading(true);
    setShowEmailRegister(true);
    setEmailRegisterLoading(false);
  };

  const handleOtherRegisterOptionsClick = () => {
    setOtherRegisterOptionsLoading(true);
    setShowEmailRegister(false);
    setOtherRegisterOptionsLoading(false);
  };

  const onTelegramLoginClicked = async (response) => {
    const fields = [
      'id',
      'first_name',
      'last_name',
      'username',
      'photo_url',
      'auth_date',
      'hash',
      'lang',
    ];
    const params = {};
    fields.forEach((field) => {
      if (response[field]) {
        params[field] = response[field];
      }
    });
    try {
      const res = await API.get(`/api/oauth/telegram/login`, { params });
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        showSuccess('登录成功！');
        setUserData(data);
        updateAPI();
        navigate('/');
      } else {
        showError(message);
      }
    } catch (error) {
      showError('登录失败，请重试');
    }
  };

  const renderOAuthOptions = () => {
    return (
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
                {t('注 册')}
              </Title>
            </div>
            <div className='px-2 py-8'>
              <div className='space-y-3'>
                {status.wechat_login && (
                  <Button
                    theme='outline'
                    className='w-full h-12 flex items-center justify-center !rounded-full border border-gray-200 hover:bg-gray-50 transition-colors'
                    type='tertiary'
                    icon={
                      <Icon svg={<WeChatIcon />} style={{ color: '#07C160' }} />
                    }
                    onClick={onWeChatLoginClicked}
                    loading={wechatLoading}
                  >
                    <span className='ml-3'>{t('使用 微信 继续')}</span>
                  </Button>
                )}

                {status.github_oauth && (
                  <Button
                    theme='outline'
                    className='w-full h-12 flex items-center justify-center !rounded-full border border-gray-200 hover:bg-gray-50 transition-colors'
                    type='tertiary'
                    icon={<IconGithubLogo size='large' />}
                    onClick={handleGitHubClick}
                    loading={githubLoading}
                    disabled={githubButtonDisabled}
                  >
                    <span className='ml-3'>{githubButtonText}</span>
                  </Button>
                )}

                {status.discord_oauth && (
                  <Button
                    theme='outline'
                    className='w-full h-12 flex items-center justify-center !rounded-full border border-gray-200 hover:bg-gray-50 transition-colors'
                    type='tertiary'
                    icon={
                      <SiDiscord
                        style={{
                          color: '#5865F2',
                          width: '20px',
                          height: '20px',
                        }}
                      />
                    }
                    onClick={handleDiscordClick}
                    loading={discordLoading}
                  >
                    <span className='ml-3'>{t('使用 Discord 继续')}</span>
                  </Button>
                )}

                {status.oidc_enabled && (
                  <Button
                    theme='outline'
                    className='w-full h-12 flex items-center justify-center !rounded-full border border-gray-200 hover:bg-gray-50 transition-colors'
                    type='tertiary'
                    icon={<OIDCIcon style={{ color: '#1877F2' }} />}
                    onClick={handleOIDCClick}
                    loading={oidcLoading}
                  >
                    <span className='ml-3'>{t('使用 OIDC 继续')}</span>
                  </Button>
                )}

                {status.linuxdo_oauth && (
                  <Button
                    theme='outline'
                    className='w-full h-12 flex items-center justify-center !rounded-full border border-gray-200 hover:bg-gray-50 transition-colors'
                    type='tertiary'
                    icon={
                      <LinuxDoIcon
                        style={{
                          color: '#E95420',
                          width: '20px',
                          height: '20px',
                        }}
                      />
                    }
                    onClick={handleLinuxDOClick}
                    loading={linuxdoLoading}
                  >
                    <span className='ml-3'>{t('使用 LinuxDO 继续')}</span>
                  </Button>
                )}

                {status.custom_oauth_providers &&
                  status.custom_oauth_providers.map((provider) => (
                    <Button
                      key={provider.slug}
                      theme='outline'
                      className='w-full h-12 flex items-center justify-center !rounded-full border border-gray-200 hover:bg-gray-50 transition-colors'
                      type='tertiary'
                      icon={getOAuthProviderIcon(provider.icon || '', 20)}
                      onClick={() => handleCustomOAuthClick(provider)}
                      loading={customOAuthLoading[provider.slug]}
                    >
                      <span className='ml-3'>
                        {t('使用 {{name}} 继续', { name: provider.name })}
                      </span>
                    </Button>
                  ))}

                {status.telegram_oauth && (
                  <div className='flex justify-center my-2'>
                    <TelegramLoginButton
                      dataOnauth={onTelegramLoginClicked}
                      botName={status.telegram_bot_name}
                    />
                  </div>
                )}

                <Divider margin='12px' align='center'>
                  {t('或')}
                </Divider>

                <Button
                  theme='solid'
                  type='primary'
                  className='w-full h-12 flex items-center justify-center bg-black text-white !rounded-full hover:bg-gray-800 transition-colors'
                  icon={<IconMail size='large' />}
                  onClick={handleEmailRegisterClick}
                  loading={emailRegisterLoading}
                >
                  <span className='ml-3'>{t('使用 邮箱/手机号 注册')}</span>
                </Button>
              </div>

              <div className='mt-6 text-center text-sm'>
                <Text>
                  {t('已有账户？')}{' '}
                  <Link
                    to='/login'
                    className='text-blue-600 hover:text-blue-800 font-medium'
                  >
                    {t('登录')}
                  </Link>
                </Text>
              </div>
            </div>
          </Card>
        </div>
      </div>
    );
  };

  const renderEmailRegisterForm = () => {
    return (
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
                {t('注 册')}
              </Title>
            </div>
            <div className='px-2 py-8'>
              <div className='grid grid-cols-2 gap-2 mb-4'>
                <Button
                  theme={registerMode === 'email' ? 'solid' : 'outline'}
                  type={registerMode === 'email' ? 'primary' : 'tertiary'}
                  className='!rounded-full'
                  onClick={() => setRegisterMode('email')}
                >
                  {t('邮箱注册')}
                </Button>
                <Button
                  theme={registerMode === 'phone' ? 'solid' : 'outline'}
                  type={registerMode === 'phone' ? 'primary' : 'tertiary'}
                  className='!rounded-full'
                  onClick={() => setRegisterMode('phone')}
                >
                  {t('手机号注册')}
                </Button>
              </div>
              <Form className='space-y-3'>
                <Form.Input
                  field='password'
                  label={t('密码')}
                  placeholder={t('Password must be 8-20 characters and contain letters and numbers')}
                  name='password'
                  mode='password'
                  onChange={(value) => handleChange('password', value)}
                  prefix={<IconLock />}
                />
                <Text size='small' type='tertiary'>
                  {t('Password must be 8-20 characters and contain letters and numbers')}
                </Text>

                <Form.Input
                  field='password2'
                  label={t('确认密码')}
                  placeholder={t('确认密码')}
                  name='password2'
                  mode='password'
                  onChange={(value) => handleChange('password2', value)}
                  prefix={<IconLock />}
                />

                {registerMode === 'phone' && (
                  <>
                    <Form.Input
                      field='phone_number'
                      label={t('手机号')}
                      placeholder={t('请输入手机号')}
                      name='phone_number'
                      value={inputs.phone_number}
                      onChange={(value) => handleChange('phone_number', value)}
                      prefix={<Smartphone size={16} />}
                    />
                    <Form.Input
                      field='phone_verification_code'
                      label={t('验证码')}
                      placeholder={t('输入验证码')}
                      name='phone_verification_code'
                      value={inputs.phone_verification_code}
                      onChange={(value) =>
                        handleChange('phone_verification_code', value)
                      }
                      prefix={<IconKey />}
                      suffix={
                        <Button
                          onClick={sendPhoneVerification}
                          loading={phoneCodeLoading}
                          disabled={phoneCountdown > 0 || phoneCodeLoading}
                        >
                          {getPhoneCodeButtonText(
                            phoneCountdown > 0,
                            phoneCountdown,
                            t,
                          )}
                        </Button>
                      }
                    />
                  </>
                )}

                {registerMode === 'email' && (
                  <>
                    <Form.Input
                      field='email'
                      label={t('邮箱')}
                      placeholder={t('输入邮箱地址')}
                      name='email'
                      type='email'
                      onChange={(value) => handleChange('email', value)}
                      prefix={<IconMail />}
                      suffix={
                        <Button
                          onClick={sendVerificationCode}
                          loading={verificationCodeLoading}
                          disabled={disableButton || verificationCodeLoading}
                        >
                          {disableButton
                            ? `${t('重新发送')} (${countdown})`
                            : t('获取验证码')}
                        </Button>
                      }
                    />
                    <Form.Input
                      field='verification_code'
                      label={t('验证码')}
                      placeholder={t('输入验证码')}
                      name='verification_code'
                      onChange={(value) =>
                        handleChange('verification_code', value)
                      }
                      prefix={<IconKey />}
                    />
                  </>
                )}

                {(hasUserAgreement || hasPrivacyPolicy) && (
                  <div className='pt-4'>
                    <Checkbox
                      checked={agreedToTerms}
                      onChange={(e) => setAgreedToTerms(e.target.checked)}
                    >
                      <Text size='small' className='text-gray-600'>
                        {t('我已阅读并同意')}
                        {hasUserAgreement && (
                          <>
                            <a
                              href='/user-agreement'
                              target='_blank'
                              rel='noopener noreferrer'
                              className='text-blue-600 hover:text-blue-800 mx-1'
                            >
                              {t('用户协议')}
                            </a>
                          </>
                        )}
                        {hasUserAgreement && hasPrivacyPolicy && t('和')}
                        {hasPrivacyPolicy && (
                          <>
                            <a
                              href='/privacy-policy'
                              target='_blank'
                              rel='noopener noreferrer'
                              className='text-blue-600 hover:text-blue-800 mx-1'
                            >
                              {t('隐私政策')}
                            </a>
                          </>
                        )}
                      </Text>
                    </Checkbox>
                  </div>
                )}

                <div className='space-y-2 pt-2'>
                  <Button
                    theme='solid'
                    className='w-full !rounded-full'
                    type='primary'
                    htmlType='submit'
                    onClick={handleSubmit}
                    loading={registerLoading}
                    disabled={
                      (hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms
                    }
                  >
                    {t('注册')}
                  </Button>
                </div>
              </Form>

              {hasOAuthRegisterOptions && (
                <>
                  <Divider margin='12px' align='center'>
                    {t('或')}
                  </Divider>

                  <div className='mt-4 text-center'>
                    <Button
                      theme='outline'
                      type='tertiary'
                      className='w-full !rounded-full'
                      onClick={handleOtherRegisterOptionsClick}
                      loading={otherRegisterOptionsLoading}
                    >
                      {t('其他注册选项')}
                    </Button>
                  </div>
                </>
              )}

              <div className='mt-6 text-center text-sm'>
                <Text>
                  {t('已有账户？')}{' '}
                  <Link
                    to='/login'
                    className='text-blue-600 hover:text-blue-800 font-medium'
                  >
                    {t('登录')}
                  </Link>
                </Text>
              </div>
            </div>
          </Card>
        </div>
      </div>
    );
  };

  const renderWeChatLoginModal = () => {
    return (
      <Modal
        title={t('微信扫码登录')}
        visible={showWeChatLoginModal}
        maskClosable={true}
        onOk={onSubmitWeChatVerificationCode}
        onCancel={() => setShowWeChatLoginModal(false)}
        okText={t('登录')}
        centered={true}
        okButtonProps={{
          loading: wechatCodeSubmitLoading,
        }}
      >
        <div className='flex flex-col items-center'>
          <img src={status.wechat_qrcode} alt='微信二维码' className='mb-4' />
        </div>

        <div className='text-center mb-4'>
          <p>
            {t('微信扫码关注公众号，输入「验证码」获取验证码（三分钟内有效）')}
          </p>
        </div>

        <Form>
          <Form.Input
            field='wechat_verification_code'
            placeholder={t('验证码')}
            label={t('验证码')}
            value={inputs.wechat_verification_code}
            onChange={(value) =>
              handleChange('wechat_verification_code', value)
            }
          />
        </Form>
      </Modal>
    );
  };

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
        {showEmailRegister ||
        !hasOAuthRegisterOptions
          ? renderEmailRegisterForm()
          : renderOAuthOptions()}
        {renderWeChatLoginModal()}

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
  );
};

export default RegisterForm;
