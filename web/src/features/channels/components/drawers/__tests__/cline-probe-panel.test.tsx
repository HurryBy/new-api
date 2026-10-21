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
  it.each(['{', '[]', '{"alias":42}', '{"alias":"other","other":"alias"}'])(
    'keeps configured models usable for an invalid mapping %s',
    async (modelMapping) => {
      renderPanel({ models: ['alias'], modelMapping })
      const input = await screen.findByRole('combobox', {
        name: 'Pinned upstreams for alias',
      })
      await waitFor(() => expect(input).toBeEnabled())
      expect(putMock).not.toHaveBeenCalled()
    }
  )

  it('uses the target snapshot and pin when aliases collide, preserving all other saved entries on an explicit save', async () => {
    const aliasPin = { upstreams: ['legacy-provider'], sort: 'tps' }
    const targetPin = {
      upstreams: ['target-provider'],
      sort: 'cost',
      exclude: ['excluded'],
    }
    mockGets({
      snapshot: {
        sampledAt: 1_700_000_000,
        models: [
          {
            ...SNAPSHOT_MODEL,
            upstreams: [{ upstream: 'target-candidate', status: 'ok' }],
          },
          {
            model: 'alias',
            upstreams: [{ upstream: 'outdated-candidate', status: 'ok' }],
          },
        ],
      },
      pins: { alias: aliasPin, [MODEL]: targetPin },
    })
    putMock.mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    renderPanel({
      models: ['alias', MODEL],
      modelMapping: JSON.stringify({ alias: MODEL }),
    })
    const input = await screen.findByRole('combobox', {
      name: `Pinned upstreams for ${MODEL}`,
    })
    await waitFor(() => expect(input).toBeEnabled())
    expect(
      screen.getAllByRole('combobox', { name: `Pinned upstreams for ${MODEL}` })
    ).toHaveLength(1)
    expect(
      screen.queryByRole('button', { name: /outdated-candidate/i })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Unpin target-provider' })
    ).toBeInTheDocument()
    expect(putMock).not.toHaveBeenCalled()
    await user.click(
      screen.getByRole('button', { name: /target-candidate/i, pressed: false })
    )
    await user.click(screen.getByRole('button', { name: /Save .* pins/i }))
    await waitFor(() =>
      expect(putMock).toHaveBeenCalledWith('/api/channel/7/cline/pins', {
        pins: {
          alias: aliasPin,
          [MODEL]: {
            ...targetPin,
            upstreams: ['target-provider', 'target-candidate'],
          },
        },
      })
    )
  })

  it('resolves a manually entered alias to its target before probing', async () => {
    postMock.mockResolvedValue({
      data: { success: true, data: { models: [] } },
    })
    const user = userEvent.setup()
    renderPanel({
      models: ['alias'],
      modelMapping: JSON.stringify({ alias: MODEL }),
    })
    const input = await screen.findByRole('combobox', {
      name: 'Models to configure or probe',
    })
    await user.type(input, 'alias')
    await user.keyboard('{Enter}{Escape}')
    await user.click(screen.getByRole('button', { name: 'Probe channel' }))
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        '/api/channel/7/cline/probe',
        { models: [MODEL] },
        expect.anything()
      )
    )
    expect(putMock).not.toHaveBeenCalled()
  })

  it('shows one mapped target for configured aliases and probes only that target without rewriting pins', async () => {
    const target = 'cline-pass/deepseek-v4.1-flash-1017'
    const savedPins = { alias: { upstreams: ['custom-provider'], sort: 'tps' } }
    mockGets({
      snapshot: {
        sampledAt: 1_700_000_000,
        models: [{ model: 'alias' }, { model: target }],
      },
      pins: savedPins,
    })
    postMock.mockResolvedValue({
      data: { success: true, data: { models: [] } },
    })
    const user = userEvent.setup()
    renderPanel({
      models: ['alias', 'other-alias', target],
      modelMapping: JSON.stringify({
        alias: 'other-alias',
        'other-alias': target,
      }),
    })

    const upstreamInput = await screen.findByRole('combobox', {
      name: `Pinned upstreams for ${target}`,
    })
    await waitFor(() => expect(upstreamInput).toBeEnabled())
    expect(
      screen.getAllByRole('combobox', {
        name: `Pinned upstreams for ${target}`,
      })
    ).toHaveLength(1)
    expect(
      screen.queryByRole('combobox', { name: 'Pinned upstreams for alias' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('combobox', {
        name: 'Pinned upstreams for other-alias',
      })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Unpin custom-provider' })
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Save .* pins/i })).toBeNull()

    await user.click(
      screen.getByRole('combobox', { name: 'Models to configure or probe' })
    )
    expect(
      screen.queryByRole('option', { name: 'alias' })
    ).not.toBeInTheDocument()
    expect(screen.getAllByRole('option', { name: target })).toHaveLength(1)
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('button', { name: /Probe again/i }))
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        '/api/channel/7/cline/probe',
        { models: [target] },
        expect.anything()
      )
    )
    expect(putMock).not.toHaveBeenCalled()
    expect(savedPins).toEqual({
      alias: { upstreams: ['custom-provider'], sort: 'tps' },
    })
  })
  it('keeps custom pins editable after a failed save and allows retrying', async () => {
    mockGets({})
    putMock.mockResolvedValueOnce({
      data: { success: false, message: 'Save rejected' },
    })
    putMock.mockResolvedValueOnce({ data: { success: true } })
    const user = userEvent.setup()
    renderPanel({ models: [MODEL] })
    const input = await screen.findByRole('combobox', {
      name: `Pinned upstreams for ${MODEL}`,
    })
    await waitFor(() => expect(input).toBeEnabled())
    await user.click(input)
    await user.paste(' custom-provider/eu,custom-provider/eu, ')
    const save = screen.getByRole('button', { name: /Save 1 pin/i })
    await user.click(save)
    await waitFor(() => expect(save).toBeEnabled())
    expect(
      screen.getByRole('button', { name: 'Unpin custom-provider/eu' })
    ).toBeInTheDocument()
    await user.click(save)
    await waitFor(() => expect(putMock).toHaveBeenCalledTimes(2))
    expect(putMock).toHaveBeenLastCalledWith('/api/channel/7/cline/pins', {
      pins: { [MODEL]: { upstreams: ['custom-provider/eu'] } },
    })
  })

  it('accepts a custom upstream without probing and preserves other saved pins', async () => {
    const otherPin = {
      upstreams: ['alibaba'],
      pin_mode: 'preferred',
      sort: 'cost',
    }
    mockGets({ pins: { 'cline-pass/glm-5.2': otherPin } })
    putMock.mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    renderPanel({ models: [MODEL] })

    const input = await screen.findByRole('combobox', {
      name: `Pinned upstreams for ${MODEL}`,
    })
    await waitFor(() => expect(input).toBeEnabled())
    await user.type(input, 'custom-provider/eu')
    expect(input).toHaveValue('custom-provider/eu')
    await user.keyboard('{Enter}')
    expect(
      screen.getByRole('button', { name: 'Unpin custom-provider/eu' })
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Save 2 pins/i }))

    await waitFor(() =>
      expect(putMock).toHaveBeenCalledWith('/api/channel/7/cline/pins', {
        pins: {
          'cline-pass/glm-5.2': otherPin,
          [MODEL]: { upstreams: ['custom-provider/eu'] },
        },
      })
    )
    expect(postMock).not.toHaveBeenCalled()
  })

  it('removes an existing pin without clearing the other models', async () => {
    mockGets({
      snapshot: { sampledAt: 1_700_000_000, models: [SNAPSHOT_MODEL] },
      pins: {
        [MODEL]: { upstreams: ['fireworks'] },
        'cline-pass/glm-5.2': { upstreams: ['alibaba'] },
      },
    })
    putMock.mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    renderPanel({})
    await user.click(
      await screen.findByRole('button', { pressed: true, name: /fireworks/i })
    )
    await user.click(screen.getByRole('button', { name: /Save 1 pin/i }))

    await waitFor(() =>
      expect(putMock).toHaveBeenCalledWith('/api/channel/7/cline/pins', {
        pins: { 'cline-pass/glm-5.2': { upstreams: ['alibaba'] } },
      })
    )
  })

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
