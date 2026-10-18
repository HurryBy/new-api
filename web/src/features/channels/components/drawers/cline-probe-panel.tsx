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
import { CheckmarkCircle02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { Loader2, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import {
  fetchClineSubscriptionModels,
  probeClineChannel,
  type ClineModelProbe,
  type ClineProbeResult,
  type ClineUpstreamResult,
  type ClineUpstreamStatus,
} from '../../lib/cline-probe'

interface ClineProbePanelProps {
  channelId?: number
  /** Models currently configured on the channel, used as the probe target. */
  models: string[]
  disabled?: boolean
  className?: string
}

/** Tone mapping for an upstream attempt verdict. */
const UPSTREAM_STATUS_TONE: Record<
  ClineUpstreamStatus,
  'default' | 'secondary' | 'destructive' | 'outline'
> = {
  ok: 'default',
  limited: 'secondary',
  bad: 'destructive',
  auth: 'destructive',
  unknown: 'outline',
}

function upstreamStatusLabelKey(status: ClineUpstreamStatus): string {
  switch (status) {
    case 'ok':
      return 'Available'
    case 'limited':
      return 'Rate limited'
    case 'bad':
      return 'Not usable'
    case 'auth':
      return 'Key problem'
    default:
      return 'Unknown'
  }
}

/**
 * Verdict badge for one model probe.
 *
 * A nominally pinnable pipeline whose pin was ignored is an explicit third
 * state: reporting it as "not supported" would hide that the gateway accepted
 * and discarded the pin.
 */
function PinVerdictBadge(props: { probe: ClineModelProbe }) {
  const { t } = useTranslation()
  if (props.probe.pinnedHeld) {
    return <Badge>{t('Pin holds')}</Badge>
  }
  if (props.probe.pinnable) {
    return <Badge variant='outline'>{t('Pin ignored by gateway')}</Badge>
  }
  return <Badge variant='outline'>{t('Pinning not supported')}</Badge>
}

/**
 * Probe a Cline Pass channel and report, per model, which upstream the gateway
 * really used and whether the configured pin actually held.
 *
 * This is intentionally an *observation* tool. The Cline gateway pins plain
 * OpenRouter models but silently ignores pins for `cline-pass/*` subscription
 * models, so the panel reports `pinnedHeld` rather than assuming success.
 */
export function ClineProbePanel(props: ClineProbePanelProps) {
  const { t } = useTranslation()
  const [result, setResult] = useState<ClineProbeResult | null>(null)
  const [isProbing, setIsProbing] = useState(false)
  const [selected, setSelected] = useState<string[]>([])

  // Cline Pass subscription models cannot be enumerated from /v1/models, so the
  // selectable catalogue comes from the backend.
  const catalogue = useQuery({
    queryKey: ['channel', 'cline', 'subscription-models'],
    queryFn: fetchClineSubscriptionModels,
    staleTime: 5 * 60 * 1000,
  })

  // Offer the catalogue plus whatever the channel already configures, so a
  // custom model name is selectable too.
  const options = useMemo(() => {
    const names = new Set<string>([...(catalogue.data ?? []), ...props.models])
    return [...names].sort().map((name) => ({ value: name, label: name }))
  }, [catalogue.data, props.models])

  const canProbe = Boolean(props.channelId) && !props.disabled && !isProbing

  async function runProbe() {
    if (!props.channelId) return
    setIsProbing(true)
    try {
      // An empty selection probes every model configured on the channel.
      const probed = await probeClineChannel(props.channelId, selected)
      setResult(probed)
    } catch (error) {
      handleServerError(error, t('Failed to probe Cline upstreams'))
    } finally {
      setIsProbing(false)
    }
  }

  return (
    <div className={cn('space-y-3 border-t pt-4', props.className)}>
      <div className='space-y-1'>
        <div className='text-sm font-medium'>{t('Upstream probe')}</div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Pick the models to check, then ask the gateway which upstream really serves each one and whether pinning it takes effect.'
          )}
        </p>
      </div>

      <div className='space-y-1'>
        <div className='text-muted-foreground text-xs'>
          {t('Models to probe')}
        </div>
        <MultiSelect
          options={options}
          selected={selected}
          onChange={setSelected}
          allowCreate
          disabled={props.disabled || isProbing}
          placeholder={t('Leave empty to probe every configured model')}
          createLabel={t('Add "{{value}}"')}
          emptyText={t('No matching models')}
        />
      </div>

      <Button
        type='button'
        variant='outline'
        size='sm'
        disabled={!canProbe}
        onClick={runProbe}
      >
        {isProbing ? (
          <Loader2 className='mr-2 h-4 w-4 animate-spin' />
        ) : (
          <RefreshCw className='mr-2 h-4 w-4' />
        )}
        {isProbing ? t('Probing…') : t('Probe upstreams')}
      </Button>

      {!props.channelId && (
        <p className='text-muted-foreground text-xs'>
          {t('Save the channel before probing its upstreams.')}
        </p>
      )}

      {result && result.models.length === 0 && (
        <p className='text-muted-foreground text-xs'>
          {t('No models to probe on this channel.')}
        </p>
      )}

      {result?.models.map((probe) => (
        <ClineModelProbeCard key={probe.model} probe={probe} />
      ))}
    </div>
  )
}

function ClineModelProbeCard(props: { probe: ClineModelProbe }) {
  const { t } = useTranslation()
  const { probe } = props

  return (
    <div className='bg-muted/40 space-y-2 rounded-md border p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-mono text-xs break-all'>{probe.model}</span>
        {probe.reachable ? (
          <Badge variant='secondary'>
            <HugeiconsIcon
              icon={CheckmarkCircle02Icon}
              className='mr-1 h-3 w-3'
            />
            {t('Reachable')}
          </Badge>
        ) : (
          <Badge variant='destructive'>{t('Unreachable')}</Badge>
        )}
        <PinVerdictBadge probe={probe} />
      </div>

      <dl className='text-muted-foreground grid grid-cols-1 gap-1 text-xs sm:grid-cols-2'>
        {probe.pipeline && (
          <div className='flex gap-1'>
            <dt className='shrink-0'>{t('Pipeline')}:</dt>
            <dd className='font-mono'>{probe.pipeline}</dd>
          </div>
        )}
        {probe.finalProvider && (
          <div className='flex gap-1'>
            <dt className='shrink-0'>{t('Served by')}:</dt>
            <dd className='font-mono break-all'>{probe.finalProvider}</dd>
          </div>
        )}
        {probe.requestedPin && (
          <div className='flex gap-1'>
            <dt className='shrink-0'>{t('Verified pin')}:</dt>
            <dd className='font-mono break-all'>{probe.requestedPin}</dd>
          </div>
        )}
        {probe.elapsedMs > 0 && (
          <div className='flex gap-1'>
            <dt className='shrink-0'>{t('Latency')}:</dt>
            <dd>{probe.elapsedMs} ms</dd>
          </div>
        )}
      </dl>

      {!probe.pinnedHeld && probe.pinnable && (
        <p className='text-xs text-amber-600 dark:text-amber-500'>
          {t(
            'This model stays on the provider chosen by the gateway even when a pin is sent, so the pin is not enforced.'
          )}
        </p>
      )}

      {probe.available && probe.available.length > 0 && (
        <div className='space-y-1'>
          <div className='text-muted-foreground text-xs'>
            {t('Candidate upstreams')}
          </div>
          <div className='flex flex-wrap gap-1'>
            {probe.available.map((name) => (
              <Badge key={name} variant='outline' className='font-mono text-xs'>
                {name}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {probe.upstreams && probe.upstreams.length > 0 && (
        <div className='space-y-1'>
          <div className='text-muted-foreground text-xs'>
            {t('Pin verification attempts')}
          </div>
          <div className='space-y-1'>
            {probe.upstreams.map((attempt) => (
              <ClineUpstreamAttemptRow
                key={attempt.upstream}
                attempt={attempt}
              />
            ))}
          </div>
        </div>
      )}

      {(probe.error || probe.detail) && (
        <p className='text-destructive text-xs break-all'>
          {probe.error}
          {probe.error && probe.detail ? ' — ' : ''}
          {probe.detail}
        </p>
      )}
    </div>
  )
}

function ClineUpstreamAttemptRow(props: { attempt: ClineUpstreamResult }) {
  const { t } = useTranslation()
  const tone = UPSTREAM_STATUS_TONE[props.attempt.status]

  return (
    <div className='flex flex-wrap items-center gap-2 text-xs'>
      <span className='font-mono break-all'>{props.attempt.upstream}</span>
      <Badge variant={tone}>
        {t(upstreamStatusLabelKey(props.attempt.status))}
      </Badge>
      {props.attempt.finalProvider && (
        <span className='text-muted-foreground font-mono'>
          → {props.attempt.finalProvider}
        </span>
      )}
      {props.attempt.note && (
        <span className='text-muted-foreground break-all'>
          {props.attempt.note}
        </span>
      )}
    </div>
  )
}
