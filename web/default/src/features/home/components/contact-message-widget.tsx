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
import { type ChangeEvent, useEffect, useState } from 'react'
import { Send } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { submitContactMessage } from '../api'

// Module-level opener so any caller (e.g. the CTA button) can trigger the dialog
// without prop drilling. Only the latest mounted ContactMessageWidget instance
// owns the setter — the widget is mounted once at the page root.
let openSetter: ((open: boolean) => void) | null = null

export function openContactMessageDialog() {
  openSetter?.(true)
}

const CONTACT_PHONE_PATTERN = /^[0-9+\-()\s]{5,32}$/
const CONTACT_NAME_MAX_LENGTH = 64
const CONTACT_MESSAGE_MAX_LENGTH = 1000

type ContactMessageForm = {
  name: string
  phone: string
  message: string
}

const initialForm: ContactMessageForm = {
  name: '',
  phone: '',
  message: '',
}

function isValidPhone(phone: string) {
  const digitCount = Array.from(phone).filter((char) => /\d/.test(char)).length
  return CONTACT_PHONE_PATTERN.test(phone) && digitCount >= 5
}

export function ContactMessageWidget() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<ContactMessageForm>(initialForm)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    openSetter = setOpen
    return () => {
      if (openSetter === setOpen) openSetter = null
    }
  }, [])

  const updateField =
    (field: keyof ContactMessageForm) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setForm((current) => ({ ...current, [field]: event.target.value }))
    }

  const validateForm = () => {
    const name = form.name.trim()
    const phone = form.phone.trim()
    const message = form.message.trim()

    if (!name) {
      toast.error(t('Name is required'))
      return null
    }
    if (name.length > CONTACT_NAME_MAX_LENGTH) {
      toast.error(t('Name cannot exceed {{count}} characters', { count: 64 }))
      return null
    }
    if (!phone) {
      toast.error(t('Phone is required'))
      return null
    }
    if (!isValidPhone(phone)) {
      toast.error(t('Phone format is invalid'))
      return null
    }
    if (message.length > CONTACT_MESSAGE_MAX_LENGTH) {
      toast.error(
        t('Message cannot exceed {{count}} characters', { count: 1000 })
      )
      return null
    }
    return { name, phone, message }
  }

  const handleSubmit = async () => {
    const payload = validateForm()
    if (!payload) return

    setSubmitting(true)
    try {
      const response = await submitContactMessage(payload)
      if (response.success) {
        toast.success(
          t('Submit successful. We will contact you as soon as possible.')
        )
        setForm(initialForm)
        setOpen(false)
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Contact us')}</DialogTitle>
            <DialogDescription>
              {t('Leave your contact information and we will follow up soon.')}
            </DialogDescription>
          </DialogHeader>

          <div className='grid gap-3'>
            <div className='grid gap-1.5'>
              <label className='text-sm font-medium' htmlFor='contact-name'>
                {t('Contact name')}
              </label>
              <Input
                id='contact-name'
                value={form.name}
                onChange={updateField('name')}
                maxLength={CONTACT_NAME_MAX_LENGTH}
                placeholder={t('Your name')}
              />
            </div>
            <div className='grid gap-1.5'>
              <label className='text-sm font-medium' htmlFor='contact-phone'>
                {t('Contact phone')}
              </label>
              <Input
                id='contact-phone'
                value={form.phone}
                onChange={updateField('phone')}
                maxLength={32}
                placeholder={t('Phone number')}
              />
            </div>
            <div className='grid gap-1.5'>
              <label className='text-sm font-medium' htmlFor='contact-message'>
                {t('Contact message')}
              </label>
              <Textarea
                id='contact-message'
                value={form.message}
                onChange={updateField('message')}
                maxLength={CONTACT_MESSAGE_MAX_LENGTH}
                rows={4}
                placeholder={t('Message content')}
              />
            </div>
          </div>

          <DialogFooter>
            <Button onClick={handleSubmit} disabled={submitting}>
              <Send data-icon='inline-start' />
              {submitting ? t('Submitting...') : t('Submit')}
            </Button>
          </DialogFooter>
    </DialogContent>
    </Dialog>
  )
}
