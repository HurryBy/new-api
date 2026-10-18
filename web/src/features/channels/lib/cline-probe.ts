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
import { api, type ApiRequestConfig } from '@/lib/api'

/**
 * Result of pinning one concrete upstream during a probe.
 *
 * `status` mirrors the backend classification so the UI can explain *why* an
 * upstream is unusable instead of only painting it red.
 */
export type ClineUpstreamStatus = 'ok' | 'limited' | 'bad' | 'auth' | 'unknown'

export interface ClineUpstreamResult {
  upstream: string
  status: ClineUpstreamStatus
  /** The provider the gateway reported using for this pinned attempt. */
  finalProvider?: string
  note?: string
}

/**
 * Probe result for one model.
 *
 * The two booleans are deliberately separate:
 * - `pinnable`: the pipeline nominally accepts a pin
 * - `pinnedHeld`: the gateway *actually* obeyed the pin
 *
 * A Cline Pass subscription model reports `pinnable: true, pinnedHeld: false`
 * because the gateway plans execution with system credentials and silently
 * ignores client-side pinning. The UI must not claim success in that case.
 */
export interface ClineModelProbe {
  model: string
  reachable: boolean
  pipeline?: 'gateway' | 'direct'
  pinnable: boolean
  pinnedHeld: boolean
  requestedPin?: string
  finalProvider?: string
  canonicalSlug?: string
  available?: string[]
  upstreams?: ClineUpstreamResult[]
  error?: string
  detail?: string
  elapsedMs: number
}

export interface ClineProbeResult {
  sampledAt: number
  models: ClineModelProbe[]
}

interface ClineProbeResponse {
  success: boolean
  message?: string
  data?: ClineProbeResult
}

const CLINE_PROBE_TIMEOUT_MS = 300_000

function probeConfig(): ApiRequestConfig {
  return {
    // A probe hits the real gateway once per model plus once per pinned
    // upstream, so it is far slower than an ordinary admin request.
    timeout: CLINE_PROBE_TIMEOUT_MS,
  }
}

/**
 * Fetch the known Cline Pass subscription model IDs.
 *
 * `/v1/models` on api.cline.bot returns the generic OpenRouter catalogue and does
 * NOT contain `cline-pass/*` subscription models, so the selectable list comes
 * from the backend. It is a suggestion list: the probe result is the authority on
 * whether a model actually works.
 */
export async function fetchClineSubscriptionModels(): Promise<string[]> {
  const res = await api.get<{
    success: boolean
    message?: string
    data?: string[]
  }>('/api/channel/cline/models')
  const body = res.data
  if (!body.success) {
    throw new Error(body.message || 'Failed to load Cline models')
  }
  return body.data ?? []
}

/**
 * Probe a Cline Pass channel: report which upstream is really serving each
 * model and whether the configured pin actually holds.
 *
 * `models` is optional; when omitted the backend probes every model configured
 * on the channel.
 */
export async function probeClineChannel(
  id: number,
  models?: string[]
): Promise<ClineProbeResult> {
  const res = await api.post<ClineProbeResponse>(
    `/api/channel/${id}/cline/probe`,
    models && models.length > 0 ? { models } : {},
    probeConfig()
  )
  const body = res.data
  if (!body.success || !body.data) {
    throw new Error(body.message || 'Probe failed')
  }
  return body.data
}
