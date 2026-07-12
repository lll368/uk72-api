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
import { useEffect, useState } from 'react'
import {
  AlertTriangle,
  FileSignature,
  IdCard,
  Loader2,
  Pencil,
  QrCode,
  RefreshCw,
  Save,
  ShieldCheck,
} from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type {
  WithdrawalEligibility,
  WithdrawalProfile,
  WithdrawalProfileInput,
} from '../types'
import {
  createPiggyContractProfileForm,
  discardPiggyContractProfileDraft,
  hasPiggyContractSigningStarted,
  resolvePiggyContractRefreshAction,
  type PiggyContractPanelStep,
} from './piggy-contract-status-panel-logic'

interface PiggyContractStatusPanelProps {
  profile?: WithdrawalProfile | null
  eligibility?: WithdrawalEligibility | null
  loading?: boolean
  eligibilityLoadFailed?: boolean
  profileSaving?: boolean
  signing?: boolean
  contractPreviewing?: boolean
  piggySignUrl?: string
  onRefresh?: () => Promise<boolean | void>
  onRefreshContractStatus?: () => Promise<boolean | void>
  onOpenContract?: () => Promise<boolean | void>
  onSaveProfile: (request: WithdrawalProfileInput) => Promise<boolean>
  onRequestSign: () => Promise<boolean>
}

interface PiggyContractPreviewActionProps {
  signed: boolean
  contractDocumentId?: string
}

export function PiggyContractPreviewAction({
  signed,
  contractDocumentId,
}: PiggyContractPreviewActionProps) {
  const { t } = useTranslation()
  if (!signed || !contractDocumentId) {
    return null
  }
  return (
    <div className='flex flex-wrap items-center gap-2'>
      <span className='text-muted-foreground text-xs'>
        {t('Contract No.')}
        <span className='text-foreground ms-1 font-mono'>
          {contractDocumentId}
        </span>
      </span>
    </div>
  )
}

export function PiggySignQrCode({
  signUrl,
  label,
  title,
}: {
  signUrl: string
  label: string
  title: string
}) {
  return (
    <div className='border-border bg-background flex flex-col items-center gap-2 rounded-md border p-4 text-center'>
      <div className='rounded-md bg-white p-3'>
        <QRCodeSVG value={signUrl} size={192} title={title} />
      </div>
      <p className='text-sm font-medium'>{label}</p>
    </div>
  )
}

export function PiggyContractStatusPanel({
  profile,
  eligibility,
  loading = false,
  eligibilityLoadFailed = false,
  profileSaving = false,
  signing = false,
  piggySignUrl,
  onRefresh,
  onRefreshContractStatus,
  onSaveProfile,
  onRequestSign,
}: PiggyContractStatusPanelProps) {
  const { t } = useTranslation()
  const needProfile = eligibility?.need_profile ?? !profile
  const profileComplete = !needProfile
  const needSign = profileComplete
    ? (eligibility?.need_sign ?? profile?.piggy_sign_status !== 'signed')
    : true
  const signed = profileComplete && !needSign
  const contractDocumentId = profile?.piggy_contract_document_id?.trim()
  const blockingReasons = eligibility?.blocking_reasons || []
  const currentStep: PiggyContractPanelStep = needProfile
    ? 'profile'
    : needSign
      ? 'sign'
      : 'signed'
  const signingStarted = hasPiggyContractSigningStarted(profile, piggySignUrl)
  const refreshAction = resolvePiggyContractRefreshAction(
    currentStep,
    onRefresh,
    onRefreshContractStatus,
    signingStarted
  )

  const [profileForm, setProfileForm] = useState<WithdrawalProfileInput>(() =>
    createPiggyContractProfileForm(profile)
  )
  const [editingProfile, setEditingProfile] = useState(needProfile)

  useEffect(() => {
    // 资料保存或签约状态刷新后，后端返回的 profile 是表单草稿的来源。
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setProfileForm(createPiggyContractProfileForm(profile))
    setEditingProfile(needProfile)
  }, [needProfile, profile])

  const canSaveProfile =
    profileForm.real_name.trim() !== '' &&
    profileForm.id_card_no.trim() !== '' &&
    profileForm.mobile.trim() !== '' &&
    profileForm.bank_card_no.trim() !== '' &&
    profileForm.bank_name.trim() !== ''

  const updateProfileField = (
    key: keyof WithdrawalProfileInput,
    value: string
  ) => {
    setProfileForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  const handleSaveProfile = async () => {
    const ok = await onSaveProfile({
      account_type: 'bankcard',
      real_name: profileForm.real_name.trim(),
      id_card_no: profileForm.id_card_no.trim(),
      mobile: profileForm.mobile.trim(),
      bank_card_no: profileForm.bank_card_no.trim(),
      bank_name: profileForm.bank_name.trim(),
    })
    if (ok) {
      setEditingProfile(false)
    }
  }

  if (loading) {
    return (
      <Card className='py-0'>
        <CardHeader className='p-4'>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
            <div>
              <CardTitle className='flex items-center gap-2 text-base'>
                <IdCard className='size-4' />
                {t('Withdrawal profile')}
              </CardTitle>
              <CardDescription>
                {t('Loading withdrawal profile')}
              </CardDescription>
            </div>
            <Badge variant='outline'>{t('Loading')}</Badge>
          </div>
        </CardHeader>
        <CardContent className='space-y-4 p-4 pt-0'>
          <Alert>
            <Loader2 className='size-4 animate-spin' />
            <AlertTitle>{t('Bank card withdrawal')}</AlertTitle>
            <AlertDescription>
              {t('Loading withdrawal profile')}
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  if (eligibilityLoadFailed) {
    return (
      <Card className='py-0'>
        <CardHeader className='p-4'>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
            <div>
              <CardTitle className='flex items-center gap-2 text-base'>
                <IdCard className='size-4' />
                {t('Withdrawal profile')}
              </CardTitle>
              <CardDescription>
                {t('Failed to load withdrawal profile')}
              </CardDescription>
            </div>
            <Badge variant='outline'>{t('Failed')}</Badge>
          </div>
        </CardHeader>
        <CardContent className='space-y-4 p-4 pt-0'>
          <Alert>
            <AlertTriangle className='size-4' />
            <AlertTitle>{t('Bank card withdrawal')}</AlertTitle>
            <AlertDescription className='space-y-2'>
              <span>{t('Failed to load withdrawal profile')}</span>
              {onRefresh ? (
                <div>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={onRefresh}
                    disabled={profileSaving || signing}
                  >
                    <RefreshCw data-icon='inline-start' />
                    {t('Refresh')}
                  </Button>
                </div>
              ) : null}
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className='py-0'>
      <CardHeader className='p-4'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle className='flex items-center gap-2 text-base'>
              <IdCard className='size-4' />
              {t('Withdrawal profile')}
            </CardTitle>
            <CardDescription>
              {currentStep === 'profile'
                ? t('Complete bank card profile before signing.')
                : currentStep === 'sign'
                  ? t(
                      'Profile is complete. Sign the Piggy contract to enable withdrawal.'
                    )
                  : t(
                      'Only withdrawable commission can be withdrawn. Balance cannot be withdrawn.'
                    )}
            </CardDescription>
          </div>
          <Badge variant={signed ? 'default' : 'outline'}>
            {signed ? t('Signed') : t('Unsigned')}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-4 p-4 pt-0'>
        <Alert>
          <ShieldCheck className='size-4' />
          <AlertTitle>{t('Bank card withdrawal')}</AlertTitle>
          <AlertDescription className='space-y-2'>
            {blockingReasons.length > 0 ? (
              <ul className='list-disc space-y-1 ps-4'>
                {blockingReasons.map((reason) => (
                  <li key={reason}>{t(reason)}</li>
                ))}
              </ul>
            ) : (
              <span>{t('You can submit a bank card withdrawal request.')}</span>
            )}
            <div className='flex flex-wrap items-center gap-2'>
              <PiggyContractPreviewAction
                signed={signed}
                contractDocumentId={contractDocumentId}
              />
              {refreshAction ? (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={refreshAction}
                  disabled={profileSaving || signing}
                >
                  {signing && currentStep === 'sign' ? (
                    <Loader2
                      data-icon='inline-start'
                      className='animate-spin'
                    />
                  ) : (
                    <RefreshCw data-icon='inline-start' />
                  )}
                  {t(
                    currentStep === 'sign'
                      ? 'Refresh contract status'
                      : 'Refresh'
                  )}
                </Button>
              ) : null}
            </div>
          </AlertDescription>
        </Alert>

        {editingProfile ? (
          <div className='space-y-3 rounded-lg border p-3'>
            <div className='flex items-center justify-between gap-2'>
              <div>
                <div className='text-sm font-medium'>
                  {t('Withdrawal profile')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {currentStep === 'profile'
                    ? t('Complete bank card profile before signing.')
                    : t(
                        'Editing core profile fields will require re-signing the contract.'
                      )}
                </div>
              </div>
              <Badge variant='outline'>{t('Bank card')}</Badge>
            </div>

            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='space-y-2'>
                <Label htmlFor='piggy-profile-real-name'>
                  {t('Real name')}
                </Label>
                <Input
                  id='piggy-profile-real-name'
                  value={profileForm.real_name}
                  onChange={(event) =>
                    updateProfileField('real_name', event.target.value)
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='piggy-profile-mobile'>{t('Mobile')}</Label>
                <Input
                  id='piggy-profile-mobile'
                  value={profileForm.mobile}
                  placeholder={profile?.masked_mobile || t('Mobile')}
                  onChange={(event) =>
                    updateProfileField('mobile', event.target.value)
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='piggy-profile-id-card'>
                  {t('ID card number')}
                </Label>
                <Input
                  id='piggy-profile-id-card'
                  value={profileForm.id_card_no}
                  placeholder={
                    profile?.masked_id_card_no || t('ID card number')
                  }
                  onChange={(event) =>
                    updateProfileField('id_card_no', event.target.value)
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='piggy-profile-bank-card'>
                  {t('Bank card number')}
                </Label>
                <Input
                  id='piggy-profile-bank-card'
                  value={profileForm.bank_card_no}
                  placeholder={
                    profile?.masked_bank_card_no || t('Bank card number')
                  }
                  onChange={(event) =>
                    updateProfileField('bank_card_no', event.target.value)
                  }
                />
              </div>
              <div className='space-y-2 sm:col-span-2'>
                <Label htmlFor='piggy-profile-bank-name'>
                  {t('Bank name')}
                </Label>
                <Input
                  id='piggy-profile-bank-name'
                  value={profileForm.bank_name}
                  onChange={(event) =>
                    updateProfileField('bank_name', event.target.value)
                  }
                />
              </div>
            </div>

            <div className='flex flex-wrap gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={handleSaveProfile}
                disabled={!canSaveProfile || profileSaving || signing}
              >
                {profileSaving ? (
                  <Loader2 data-icon='inline-start' className='animate-spin' />
                ) : (
                  <Save data-icon='inline-start' />
                )}
                {t('Save withdrawal profile')}
              </Button>
              {currentStep !== 'profile' ? (
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={() => {
                    setProfileForm(
                      discardPiggyContractProfileDraft(profileForm, profile)
                    )
                    setEditingProfile(false)
                  }}
                  disabled={profileSaving}
                >
                  {t('Cancel')}
                </Button>
              ) : null}
            </div>
          </div>
        ) : (
          <div className='space-y-2 rounded-lg border p-3'>
            <div className='flex items-center justify-between gap-2'>
              <div className='text-sm font-medium'>
                {t('Withdrawal profile')}
              </div>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => {
                  setProfileForm(createPiggyContractProfileForm(profile))
                  setEditingProfile(true)
                }}
                disabled={profileSaving || signing}
              >
                <Pencil data-icon='inline-start' />
                {t('Edit')}
              </Button>
            </div>
            <dl className='grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-xs'>
              <dt className='text-muted-foreground'>{t('Real name')}</dt>
              <dd>{profile?.real_name || '-'}</dd>
              <dt className='text-muted-foreground'>{t('Mobile')}</dt>
              <dd className='font-mono'>{profile?.masked_mobile || '-'}</dd>
              <dt className='text-muted-foreground'>{t('ID card number')}</dt>
              <dd className='font-mono'>{profile?.masked_id_card_no || '-'}</dd>
              <dt className='text-muted-foreground'>{t('Bank card number')}</dt>
              <dd className='font-mono'>
                {profile?.masked_bank_card_no || '-'}
              </dd>
              <dt className='text-muted-foreground'>{t('Bank name')}</dt>
              <dd>{profile?.bank_name || '-'}</dd>
            </dl>
            {currentStep !== 'profile' ? (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Editing core profile fields will require re-signing the contract.'
                )}
              </p>
            ) : null}
          </div>
        )}

        {currentStep === 'sign' ? (
          <Alert>
            <FileSignature className='size-4' />
            <AlertTitle>{t('Sign Piggy electronic contract')}</AlertTitle>
            <AlertDescription className='space-y-2'>
              <span>
                {t(
                  'Profile is complete. Sign the Piggy contract to enable withdrawal.'
                )}
              </span>
              <div className='flex flex-wrap gap-2'>
                <Button
                  type='button'
                  size='sm'
                  onClick={onRequestSign}
                  disabled={!profileComplete || profileSaving || signing}
                >
                  {signing ? (
                    <Loader2
                      data-icon='inline-start'
                      className='animate-spin'
                    />
                  ) : (
                    <QrCode data-icon='inline-start' />
                  )}
                  {t('手机扫描签约二维码')}
                </Button>
              </div>
              {piggySignUrl ? (
                <PiggySignQrCode
                  signUrl={piggySignUrl}
                  label={t('Scan with phone to open')}
                  title={t('Piggy signing QR code')}
                />
              ) : null}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
    </Card>
  )
}
