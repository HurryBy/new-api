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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { ClineProbePanel } from '../cline-probe-panel'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
}))

const getMock = vi.mocked(api.get)
const postMock = vi.mocked(api.post)
const putMock = vi.mocked(api.put)

let i18n: ReturnType<typeof createInstance>
let client: QueryClient

const MODEL = 'cline-pass/deepseek-v4.1-flash'

const SNAPSHOT_MODEL = {
  model: MODEL,
  reachable: true,
  pipeline: 'gateway',
  defaultProvider: 'deepseek',
  pinnable: true,
  upstreams: [
    {
      upstream: 'deepseek',
      status: 'ok',
      finalProvider: 'deepseek',
      held: true,
    },
    { upstream: 'fireworks', status: 'ok', finalProvider: 'deepseek' },
    { upstream: 'baseten', status: 'limited' },
  ],
}

/** Route GETs by path so the catalogue and the snapshot stay independent. */
function mockGets(options: {
  snapshot?: unknown
  pins?: Record<string, unknown>
}) {
  getMock.mockImplementation(async (url: string) => {
    if (url === '/api/channel/cline/models') {
      return {
        data: {
          success: true,
          data: [MODEL, 'cline-pass/glm-5.2'],
        },
      }
    }
    if (url.endsWith('/cline/probe')) {
      return {
        data: {
          success: true,
          data: {
            snapshot: options.snapshot ?? null,
            pins: options.pins ?? {},
          },
        },
      }
    }
    throw new Error(`unexpected GET ${url}`)
  })
}

beforeEach(async () => {
  getMock.mockReset()
  postMock.mockReset()
  putMock.mockReset()
  mockGets({})
  client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  i18n = createInstance()
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
    returnNull: false,
  })
})

afterEach(() => {
  cleanup()
  client.clear()
})

function renderPanel(props: Partial<Parameters<typeof ClineProbePanel>[0]>) {
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <ClineProbePanel channelId={7} models={[]} {...props} />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

/**
 * The pin candidate for `upstream`.
 *
 * Candidates are the only buttons carrying `aria-pressed`; a pinned chip also
 * renders an "Unpin <upstream>" button, so the pressed state is what identifies
 * the toggle the user actually clicks.
 */
function candidate(upstream: string, pressed: boolean) {
  return screen.getByRole('button', {
    name: new RegExp(upstream, 'i'),
    pressed,
  })
}

describe('ClineProbePanel', () => {
  it('disables probing until the channel has been saved', () => {
    renderPanel({ channelId: undefined })

    expect(
      screen.getByRole('button', { name: /Probe channel/i })
    ).toBeDisabled()
    expect(
      screen.getByText(
        'Save the channel first, then probe it for upstream candidates.'
      )
    ).toBeInTheDocument()
  })

  it('offers the subscription catalogue so models are picked, not typed', async () => {
    renderPanel({})

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/api/channel/cline/models')
    })
  })

  it('restores the stored probe result so pins work without a fresh probe', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
    })
    renderPanel({})

    // The candidate upstreams from the saved probe become clickable again.
    expect(
      await screen.findByRole('button', { pressed: false, name: /fireworks/i })
    ).toBeInTheDocument()
    expect(candidate('baseten', false)).toBeInTheDocument()
  })

  it('pins an upstream with a single click and marks it as pinned', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
    })
    const user = userEvent.setup()
    renderPanel({})

    const unpinned = await screen.findByRole('button', {
      pressed: false,
      name: /fireworks/i,
    })
    await user.click(unpinned)

    expect(candidate('fireworks', true)).toHaveAttribute('aria-pressed', 'true')
  })

  it('pins several upstreams for one model', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
    })
    const user = userEvent.setup()
    renderPanel({})

    await user.click(
      await screen.findByRole('button', { pressed: false, name: /fireworks/i })
    )
    await user.click(
      screen.getByRole('button', { pressed: false, name: /baseten/i })
    )

    expect(
      screen.getByRole('button', { name: /Save 2 pins/i })
    ).toBeInTheDocument()
  })

  it('unpins an upstream when it is clicked again', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
    })
    const user = userEvent.setup()
    renderPanel({})

    const unpinned = await screen.findByRole('button', {
      pressed: false,
      name: /fireworks/i,
    })
    await user.click(unpinned)
    await user.click(candidate('fireworks', true))

    expect(candidate('fireworks', false)).toBeInTheDocument()
    // Nothing left to save, so the save action disappears again.
    expect(screen.queryByRole('button', { name: /Save .* pins/i })).toBeNull()
  })

  it('saves the selected pins through the dedicated endpoint', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
    })
    putMock.mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPanel({})

    await user.click(
      await screen.findByRole('button', { pressed: false, name: /fireworks/i })
    )
    await user.click(screen.getByRole('button', { name: /Save 1 pin/i }))

    await waitFor(() => {
      expect(putMock).toHaveBeenCalledWith('/api/channel/7/cline/pins', {
        pins: { [MODEL]: { upstreams: ['fireworks'] } },
      })
    })
  })

  it('shows saved pins as pinned on load and flags a pin the gateway ignored', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
      pins: { [MODEL]: { upstreams: ['fireworks'] } },
    })
    renderPanel({})

    // The probe saw the gateway stay on its default, so the pin is not confirmed.
    const pinned = await screen.findByRole('button', {
      pressed: true,
      name: /fireworks/i,
    })
    expect(
      within(pinned).getByText('Pin ignored by gateway')
    ).toBeInTheDocument()
  })

  it('confirms a pin that the probe saw the gateway honour', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
      pins: { [MODEL]: { upstreams: ['deepseek'] } },
    })
    renderPanel({})

    const pinned = await screen.findByRole('button', {
      pressed: true,
      name: /deepseek/i,
    })
    expect(within(pinned).getByText('Confirmed')).toBeInTheDocument()
  })

  it('probes every configured model when nothing is selected', async () => {
    postMock.mockResolvedValue({
      data: {
        success: true,
        data: { sampledAt: 1_700_000_000_000, models: [] },
      },
    })
    const user = userEvent.setup()
    renderPanel({ models: ['z-ai/glm-5.2'] })

    await user.click(screen.getByRole('button', { name: /Probe channel/i }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith(
        '/api/channel/7/cline/probe',
        // An empty selection must not narrow the probe set.
        {},
        expect.anything()
      )
    })
  })
})
