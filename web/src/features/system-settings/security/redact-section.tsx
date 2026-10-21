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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const redactSchema = z.object({
  redact_setting: z.object({
    enabled: z.boolean(),
    patterns: z.string(),
    mask: z.string(),
    mask_error_messages: z.boolean(),
    mask_model_names: z.boolean(),
    use_channel_upstream_names: z.boolean(),
  }),
})

type RedactFormValues = z.output<typeof redactSchema>
type RedactFormInput = z.input<typeof redactSchema>

type FlatRedactDefaults = {
  'redact_setting.enabled': boolean
  'redact_setting.patterns': string[]
  'redact_setting.mask': string
  'redact_setting.mask_error_messages': boolean
  'redact_setting.mask_model_names': boolean
  'redact_setting.use_channel_upstream_names': boolean
}

type RedactSectionProps = {
  defaultValues: FlatRedactDefaults
}

const splitLines = (value: string) =>
  value
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean)

const joinLines = (patterns: string[]) => patterns.join('\n')

const buildFormDefaults = (
  defaults: RedactSectionProps['defaultValues']
): RedactFormInput => ({
  redact_setting: {
    enabled: defaults['redact_setting.enabled'],
    patterns: joinLines(defaults['redact_setting.patterns'] ?? []),
    mask: defaults['redact_setting.mask'] ?? '***',
    mask_error_messages: defaults['redact_setting.mask_error_messages'],
    mask_model_names: defaults['redact_setting.mask_model_names'],
    use_channel_upstream_names:
      defaults['redact_setting.use_channel_upstream_names'],
  },
})

const normalizeDefaults = (
  defaults: RedactSectionProps['defaultValues']
): FlatRedactDefaults => ({
  'redact_setting.enabled': defaults['redact_setting.enabled'],
  'redact_setting.patterns': defaults['redact_setting.patterns'] ?? [],
  'redact_setting.mask': defaults['redact_setting.mask'] ?? '***',
  'redact_setting.mask_error_messages':
    defaults['redact_setting.mask_error_messages'],
  'redact_setting.mask_model_names':
    defaults['redact_setting.mask_model_names'],
  'redact_setting.use_channel_upstream_names':
    defaults['redact_setting.use_channel_upstream_names'],
})

const normalizeFormValues = (values: RedactFormValues): FlatRedactDefaults => ({
  'redact_setting.enabled': values.redact_setting.enabled,
  'redact_setting.patterns': splitLines(values.redact_setting.patterns ?? ''),
  'redact_setting.mask': (values.redact_setting.mask ?? '').trim() || '***',
  'redact_setting.mask_error_messages':
    values.redact_setting.mask_error_messages,
  'redact_setting.mask_model_names': values.redact_setting.mask_model_names,
  'redact_setting.use_channel_upstream_names':
    values.redact_setting.use_channel_upstream_names,
})

const isEqual = (a: unknown, b: unknown) => {
  if (Array.isArray(a) && Array.isArray(b)) {
    return JSON.stringify(a) === JSON.stringify(b)
  }
  return a === b
}

export function RedactSection({ defaultValues }: RedactSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<FlatRedactDefaults>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<RedactFormInput, unknown, RedactFormValues>({
    resolver: zodResolver(redactSchema),
    defaultValues: formDefaults,
  })

  const onSubmit = async (values: RedactFormValues) => {
    const normalized = normalizeFormValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatRedactDefaults>
    ).filter((key) => !isEqual(normalized[key], baselineRef.current[key]))

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of changedKeys) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value: Array.isArray(value) ? JSON.stringify(value) : value,
      })
    }

    baselineRef.current = normalized
    toast.success(t('Redaction settings saved'))
  }

  const enabled = form.watch('redact_setting.enabled')

  return (
    <SettingsSection title={t('Sensitive keyword redaction')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <FormField
            control={form.control}
            name='redact_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable keyword redaction')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Replace configured keywords with a mask in error messages and model names. Response content is never modified.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='redact_setting.patterns'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Keywords to redact')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    rows={8}
                    disabled={!enabled}
                    placeholder={t(
                      'One keyword per line. Prefix a line with re: to use a regular expression.'
                    )}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Keywords are matched exactly. Use the re: prefix for a regular expression, for example re:upstream-[0-9]+.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='redact_setting.mask'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Replacement text')}</FormLabel>
                <FormControl>
                  <Input {...field} disabled={!enabled} placeholder='***' />
                </FormControl>
                <FormDescription>
                  {t('Each matched keyword is replaced with this text.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='redact_setting.mask_error_messages'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Redact error messages')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Apply the keywords to error messages returned to clients and stored in usage logs.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!enabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='redact_setting.mask_model_names'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Redact model names')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Apply the keywords to model names in responses and in the usage logs of non-admin users.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!enabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='redact_setting.use_channel_upstream_names'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Also redact the channel upstream model name')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Automatically treat the selected channel upstream model name as a keyword, so it never reaches a non-admin client.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!enabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
