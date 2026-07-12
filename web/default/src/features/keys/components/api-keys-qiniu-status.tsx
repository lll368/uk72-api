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
import { AlertCircle, Clock, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useApiKeys } from './api-keys-provider'

export function ApiKeysQiniuStatus() {
  const { t } = useTranslation()
  const { qiniuStatus, retryDefaultQiniuKey } = useApiKeys()

  if (!qiniuStatus?.enabled || qiniuStatus.has_key) {
    return null
  }

  if (qiniuStatus.revoke_blocked) {
    const revokeFailed = qiniuStatus.task_status === 'failed'
    return (
      <Alert variant={revokeFailed ? 'destructive' : 'default'}>
        {revokeFailed ? <AlertCircle /> : <Clock />}
        <AlertTitle>
          {revokeFailed
            ? t('Deleted API Key remote disable failed')
            : t('Deleted API Key is being disabled remotely')}
        </AlertTitle>
        <AlertDescription>
          {revokeFailed
            ? qiniuStatus.last_error || t('Please wait for remote disable retry')
            : t('Please wait before creating a replacement API Key')}
        </AlertDescription>
      </Alert>
    )
  }

  const failed = qiniuStatus.task_status === 'failed'
  const title = failed
    ? t('Default API Key creation failed')
    : t('Default API Key is being created')

  return (
    <Alert variant={failed ? 'destructive' : 'default'}>
      {failed ? <AlertCircle /> : <Clock />}
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        {failed
          ? qiniuStatus.last_error || t('Please retry default API Key creation')
          : t('Please wait while the default API Key is being created')}
      </AlertDescription>
      {failed && qiniuStatus.task_retryable && (
        <AlertAction>
          <Button size='sm' variant='outline' onClick={retryDefaultQiniuKey}>
            <RefreshCw className='h-4 w-4' />
            {t('Retry')}
          </Button>
        </AlertAction>
      )}
    </Alert>
  )
}
