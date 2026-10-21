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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { Loader2, RefreshCw } from 'lucide-react'
import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { MultiSelect } from '@/components/multi-select'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import {
  fetchClineProbeSnapshot,
  fetchClineSubscriptionModels,
  probeClineChannel,
  saveClineUpstreamPins,
  type ClineProbeModelSnapshot,
  type ClineProbeResult,
  type ClineProbeSnapshot,
  type ClineProbeUpstreamSnapshot,
  type ClineUpstreamPins,
  type ClineUpstreamResult,
  type ClineUpstreamStatus,
} from '../../lib/cline-probe'
import { resolveModelMappingTargets } from '../../lib/model-mapping-validation'

interface ClineProbePanelProps {
  channelId?: number
  /** Models currently configured on the channel, used as the probe target. */
  models: string[]
  modelMapping?: string
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

/** A pinned upstream plus a badge that states plainly whether it took effect. */
function PinnedUpstreamChip(props: {
  upstream: string
  held: boolean
  onRemove: () => void
  disabled?: boolean
}) {
  const { t } = useTranslation()

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-xs',
        props.held
          ? 'border-primary/40 bg-primary/10'
          : 'border-dashed border-muted-foreground/40 bg-muted/40'
      )}
    >
      <HugeiconsIcon
        icon={CheckmarkCircle02Icon}
        className={cn(
          'size-3.5',
          props.held ? 'text-primary' : 'text-muted-foreground'
        )}
        aria-hidden='true'
      />
      {props.upstream}
      <Badge
        variant={props.held ? 'default' : 'outline'}
        className='h-4 px-1 text-[10px]'
      >
        {props.held ? t('Confirmed') : t('Pending verification')}
      </Badge>
      <Button
        type='button'
        variant='ghost'
        size='icon-xs'
        onClick={props.onRemove}
        disabled={props.disabled}
        className='text-muted-foreground hover:text-foreground disabled:opacity-50'
        aria-label={t('Unpin {{upstream}}', {
          upstream: props.upstream,
        })}
      >
        ×
      </Button>
    </span>
  )
}

/**
 * Candidate upstream button: one click pins it, another click unpins it.
 *
 * `held` means a real probe saw the gateway honour a pin to this upstream, which
 * is the only state that earns the "Confirmed" styling.
 */
function CandidateUpstreamButton(props: {
  upstream: string
  status: ClineUpstreamStatus
  pinned: boolean
  held: boolean
  onToggle: () => void
  disabled?: boolean
}) {
  const { t } = useTranslation()

  return (
    <Button
      type='button'
      variant='outline'
      size='sm'
      onClick={props.onToggle}
      disabled={props.disabled}
      aria-pressed={props.pinned}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-xs transition-colors disabled:opacity-50',
        props.pinned
          ? 'border-primary bg-primary/10'
          : 'border-border hover:bg-accent'
      )}
    >
      <span>{props.upstream}</span>
      {props.pinned && (
        <Badge
          variant={props.held ? 'default' : 'outline'}
          className='h-4 px-1 text-[10px]'
        >
          {props.held ? t('Confirmed') : t('Pin ignored by gateway')}
        </Badge>
      )}
      {!props.pinned && (
        <Badge
          variant={UPSTREAM_STATUS_TONE[props.status]}
          className='h-4 px-1 text-[10px]'
        >
          {t(upstreamStatusLabelKey(props.status))}
        </Badge>
      )}
      {props.pinned ? (
        <span className='text-primary text-[10px]'>{t('Pinned')}</span>
      ) : (
        <span className='text-muted-foreground text-[10px]'>{t('Pin')}</span>
      )}
    </Button>
  )
}

/** One model block: verdict, candidate upstream buttons, and pinned summary. */
function ModelPinSection(props: {
  snapshot: ClineProbeModelSnapshot
  pinnedUpstreams: string[]
  onToggleUpstream: (upstream: string) => void
  onChangeUpstreams: (upstreams: string[]) => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const inputId = useId()
  const upstreams = props.snapshot.upstreams ?? []
  const isHeld = (upstream: string) =>
    upstreams.some((item) => item.upstream === upstream && item.held)

  return (
    <div className='space-y-2 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-mono text-sm break-all'>
          {props.snapshot.model}
        </span>
        {typeof props.snapshot.reachable === 'boolean' && (
          <Badge variant={props.snapshot.reachable ? 'default' : 'destructive'}>
            {props.snapshot.reachable ? t('Reachable') : t('Unreachable')}
          </Badge>
        )}
        {props.snapshot.pipeline && (
          <Badge variant='outline'>{props.snapshot.pipeline}</Badge>
        )}
        {props.snapshot.defaultProvider && (
          <span className='text-muted-foreground text-xs'>
            {t('Default upstream')}:{' '}
            <span className='font-mono'>{props.snapshot.defaultProvider}</span>
          </span>
        )}
      </div>

      {props.snapshot.error && (
        <p className='text-destructive text-xs break-all'>
          {props.snapshot.error}
          {props.snapshot.detail ? ` — ${props.snapshot.detail}` : ''}
        </p>
      )}

      <div className='space-y-1.5'>
        <Label htmlFor={inputId} className='text-xs'>
          {t('Pinned upstreams for {{model}}', { model: props.snapshot.model })}
        </Label>
        <MultiSelect
          id={inputId}
          aria-label={t('Pinned upstreams for {{model}}', {
            model: props.snapshot.model,
          })}
          options={upstreams.map((item) => ({
            value: item.upstream,
            label: item.upstream,
          }))}
          selected={props.pinnedUpstreams}
          onChange={props.onChangeUpstreams}
          allowCreate
          placeholder={t('Type an upstream name and press Enter')}
          disabled={props.disabled}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'Custom upstreams can be saved without probing. Only a successful probe confirms that a pin is honored.'
          )}
        </p>
      </div>

      {props.pinnedUpstreams.length > 0 && (
        <div className='flex flex-wrap items-center gap-2'>
          <span className='text-muted-foreground text-xs'>
            {t('Pinned upstreams')}:
          </span>
          {props.pinnedUpstreams.map((upstream) => (
            <PinnedUpstreamChip
              key={upstream}
              upstream={upstream}
              held={isHeld(upstream)}
              onRemove={() => props.onToggleUpstream(upstream)}
              disabled={props.disabled}
            />
          ))}
        </div>
      )}

      {upstreams.length > 0 ? (
        <div className='space-y-1.5'>
          <span className='text-muted-foreground text-xs'>
            {t('Click an upstream to pin it')}
          </span>
          <div className='flex flex-wrap items-center gap-2'>
            {upstreams.map((item: ClineProbeUpstreamSnapshot) => (
              <CandidateUpstreamButton
                key={item.upstream}
                upstream={item.upstream}
                status={item.status}
                pinned={props.pinnedUpstreams.includes(item.upstream)}
                held={item.held === true}
                onToggle={() => props.onToggleUpstream(item.upstream)}
                disabled={props.disabled}
              />
            ))}
          </div>
        </div>
      ) : (
        <p className='text-muted-foreground text-xs'>
          {t('No candidate upstreams found for this model')}
        </p>
      )}
    </div>
  )
}

/**
 * Probe a Cline Pass channel, keep the result, and let the admin pin upstreams
 * with a click.
 *
 * The probe result is persisted on the channel, so reopening this drawer shows
 * the previous candidate list instead of forcing another (paid, slow) probe. The
 * pin configuration is saved through its own endpoint so it survives without
 * having to submit the whole channel form.
 */
export function ClineProbePanel(props: ClineProbePanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [isProbing, setIsProbing] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [draftPins, setDraftPins] = useState<ClineUpstreamPins | null>(null)
  const modelInputId = useId()
  const mappedModels = useMemo(
    () => resolveModelMappingTargets(props.modelMapping ?? ''),
    [props.modelMapping]
  )
  const selectedModels = [
    ...new Set(selected.map((model) => mappedModels.get(model) ?? model)),
  ]

  // Cline Pass subscription models are absent from the gateway's /v1/models, so
  // the suggestion list comes from the backend.
  const catalogue = useQuery({
    queryKey: ['channel', 'cline', 'subscription-models'],
    queryFn: fetchClineSubscriptionModels,
    staleTime: 5 * 60 * 1000,
    enabled: Boolean(props.channelId),
  })

  // Snapshot of the last probe, restored so pins remain operable across visits.
  const snapshotQuery = useQuery({
    queryKey: ['channel', 'cline', 'probe-snapshot', props.channelId],
    queryFn: () => fetchClineProbeSnapshot(props.channelId as number),
    enabled: Boolean(props.channelId),
    staleTime: 0,
  })

  const modelOptions = useMemo(() => {
    const merged = new Set(
      [...(catalogue.data ?? []), ...props.models].map(
        (model) => mappedModels.get(model) ?? model
      )
    )
    return [...merged]
      .filter(Boolean)
      .sort()
      .map((value) => ({ value, label: value }))
  }, [catalogue.data, props.models, mappedModels])

  const snapshot: ClineProbeSnapshot | null =
    snapshotQuery.data?.snapshot ?? null
  // Start from the channel's saved pins so the panel shows the real current
  // state; a local edit takes over until it is saved or discarded.
  const savedPins: ClineUpstreamPins = snapshotQuery.data?.pins ?? {}
  const pins: ClineUpstreamPins = draftPins ?? savedPins
  const isDirty =
    draftPins !== null &&
    JSON.stringify(draftPins) !== JSON.stringify(savedPins)

  // Preserve the saved keys and every pin option. Mapping affects presentation
  // only; an explicit edit still updates the original entry. Exact target pins
  // take precedence, just as they do in the relay's lookupClinePin.
  const pinKeys = new Map<string, string>()
  for (const key of Object.keys(pins)) {
    const model = mappedModels.get(key) ?? key
    if (!pinKeys.has(model) || key === model) pinKeys.set(model, key)
  }
  const pinnedFor = (model: string): string[] =>
    pins[pinKeys.get(model) ?? model]?.upstreams ?? []

  const changeUpstreams = (model: string, nextUpstreams: string[]) => {
    const pinKey = pinKeys.get(model) ?? model
    setDraftPins((current) => {
      const base: ClineUpstreamPins = { ...(current ?? savedPins) }
      if (
        nextUpstreams.length === 0 &&
        !base[pinKey]?.sort &&
        !base[pinKey]?.exclude?.length
      ) {
        delete base[pinKey]
      } else {
        base[pinKey] = { ...base[pinKey], upstreams: nextUpstreams }
      }
      return base
    })
  }

  const modelSnapshots = new Map<string, ClineProbeModelSnapshot>()
  for (const entry of snapshot?.models ?? []) {
    const model = mappedModels.get(entry.model) ?? entry.model
    if (!modelSnapshots.has(model) || entry.model === model) {
      modelSnapshots.set(model, { ...entry, model })
    }
  }
  for (const source of [
    ...props.models,
    ...selectedModels,
    ...Object.keys(pins),
  ]) {
    const model = mappedModels.get(source) ?? source
    if (model && !modelSnapshots.has(model)) {
      modelSnapshots.set(model, { model })
    }
  }
  const editingDisabled =
    props.disabled || isProbing || isSaving || !snapshotQuery.isSuccess

  const runProbe = async () => {
    if (!props.channelId) return
    setIsProbing(true)
    try {
      const result: ClineProbeResult = await probeClineChannel(
        props.channelId,
        selectedModels.length > 0 || mappedModels.size === 0
          ? selectedModels
          : [
              ...new Set(
                props.models.map((model) => mappedModels.get(model) ?? model)
              ),
            ].filter(Boolean)
      )
      // The backend already persisted the snapshot; refresh from the server so
      // the rendered candidate list is exactly what was stored.
      await queryClient.invalidateQueries({
        queryKey: ['channel', 'cline', 'probe-snapshot', props.channelId],
      })
      toast.success(t('Probe finished'))
      return result
    } catch (error) {
      handleServerError(error, t('Probe failed'))
    } finally {
      setIsProbing(false)
    }
  }

  const savePins = async () => {
    if (!props.channelId) return
    setIsSaving(true)
    try {
      await saveClineUpstreamPins(props.channelId, pins)
      setDraftPins(null)
      await queryClient.invalidateQueries({
        queryKey: ['channel', 'cline', 'probe-snapshot', props.channelId],
      })
      toast.success(t('Pins saved'))
    } catch (error) {
      handleServerError(error, t('Failed to save pins'))
    } finally {
      setIsSaving(false)
    }
  }

  const pinnedCount = Object.values(pins).reduce(
    (total, pin) => total + (pin.upstreams?.length ?? 0),
    0
  )

  return (
    <div className={cn('space-y-3 border-t pt-4', props.className)}>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
        <div className='space-y-1'>
          <h4 className='text-sm font-medium'>{t('Cline upstream pins')}</h4>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Enter your own upstream names or choose probe results. Save pins to keep them for your next visit.'
            )}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={runProbe}
          disabled={props.disabled || isProbing || isSaving || !props.channelId}
        >
          {isProbing ? (
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
          ) : (
            <RefreshCw className='mr-2 h-4 w-4' />
          )}
          {snapshot ? t('Probe again') : t('Probe channel')}
        </Button>
      </div>

      {!props.channelId && (
        <p className='text-muted-foreground text-xs'>
          {t('Save the channel first, then probe it for upstream candidates.')}
        </p>
      )}

      {props.channelId && (
        <div className='space-y-1.5'>
          <Label htmlFor={modelInputId} className='text-xs font-medium'>
            {t('Models to configure or probe')}
          </Label>
          <MultiSelect
            id={modelInputId}
            aria-label={t('Models to configure or probe')}
            options={modelOptions}
            selected={selectedModels}
            onChange={setSelected}
            placeholder={t('Leave empty to probe every configured model')}
            disabled={props.disabled || isProbing || isSaving}
            allowCreate
          />
        </div>
      )}

      {snapshot && (
        <p className='text-muted-foreground text-xs'>
          {t('Probed at {{time}}', {
            time: dayjs(snapshot.sampledAt * 1000).format(
              'YYYY-MM-DD HH:mm:ss'
            ),
          })}
        </p>
      )}

      {props.channelId && modelSnapshots.size > 0 ? (
        <div className='space-y-3'>
          {[...modelSnapshots.values()].map((model) => (
            <ModelPinSection
              key={model.model}
              snapshot={model}
              pinnedUpstreams={pinnedFor(model.model)}
              onToggleUpstream={(upstream) =>
                changeUpstreams(
                  model.model,
                  pinnedFor(model.model).includes(upstream)
                    ? pinnedFor(model.model).filter((name) => name !== upstream)
                    : [...pinnedFor(model.model), upstream]
                )
              }
              onChangeUpstreams={(upstreams) =>
                changeUpstreams(model.model, upstreams)
              }
              disabled={editingDisabled}
            />
          ))}
        </div>
      ) : (
        <p className='text-muted-foreground text-xs'>
          {snapshotQuery.isLoading
            ? t('Loading previous probe result')
            : t('Choose or enter a model above to configure its upstreams.')}
        </p>
      )}

      {isDirty && (
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            size='sm'
            onClick={savePins}
            disabled={editingDisabled}
          >
            {isSaving && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('Save {{count}} pins', { count: pinnedCount })}
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => setDraftPins(null)}
            disabled={props.disabled || isSaving}
          >
            {t('Discard changes')}
          </Button>
        </div>
      )}
    </div>
  )
}

/** One pinned-attempt row, kept for the verbose probe details. */
export function ClineUpstreamAttemptRow(props: {
  attempt: ClineUpstreamResult
}) {
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
