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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { ClineProbePanel } from '../cline-probe-panel'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn() },
}))

const getMock = vi.mocked(api.get)
const postMock = vi.mocked(api.post)

let i18n: ReturnType<typeof createInstance>
let client: QueryClient

beforeEach(async () => {
  getMock.mockReset()
  postMock.mockReset()
  // The panel loads the Cline subscription catalogue on mount.
  getMock.mockResolvedValue({
    data: {
      success: true,
      data: ['cline-pass/deepseek-v4.1-flash', 'cline-pass/glm-5.2'],
    },
  })
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

function probeResponse(pinnable: boolean, pinnedHeld: boolean) {
  return {
    data: {
      success: true,
      message: '',
      data: {
        sampledAt: 1_700_000_000_000,
        models: [
          {
            model: 'cline-pass/deepseek-v4.1-flash',
            reachable: true,
            pipeline: 'gateway',
            pinnable,
            pinnedHeld,
            finalProvider: 'deepseek',
            elapsedMs: 900,
          },
        ],
      },
    },
  }
}

describe('ClineProbePanel', () => {
  it('disables probing until the channel has been saved', () => {
    renderPanel({ channelId: undefined })

    expect(
      screen.getByRole('button', { name: /Probe upstreams/i })
    ).toBeDisabled()
    expect(
      screen.getByText('Save the channel before probing its upstreams.')
    ).toBeInTheDocument()
  })

  it('offers the subscription catalogue so models are picked, not typed', async () => {
    renderPanel({})

    // The catalogue arrives asynchronously from the backend.
    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/api/channel/cline/models')
    })
  })

  it('probes every configured model when nothing is selected', async () => {
    postMock.mockResolvedValueOnce(probeResponse(true, true))
    const user = userEvent.setup()
    renderPanel({ models: ['z-ai/glm-5.2'] })

    await user.click(screen.getByRole('button', { name: /Probe upstreams/i }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith(
        '/api/channel/7/cline/probe',
        // An empty selection must not narrow the probe set.
        {},
        expect.anything()
      )
    })
  })

  it('reports a held pin only when the backend says the pin held', async () => {
    postMock.mockResolvedValueOnce(probeResponse(true, true))
    const user = userEvent.setup()
    renderPanel({})

    await user.click(screen.getByRole('button', { name: /Probe upstreams/i }))

    await waitFor(() => {
      expect(screen.getByText('Pin holds')).toBeInTheDocument()
    })
    expect(screen.queryByText('Pin ignored by gateway')).not.toBeInTheDocument()
  })

  it('warns that a pin was not enforced when the gateway ignored it', async () => {
    postMock.mockResolvedValueOnce(probeResponse(true, false))
    const user = userEvent.setup()
    renderPanel({})

    await user.click(screen.getByRole('button', { name: /Probe upstreams/i }))

    await waitFor(() => {
      expect(screen.getByText('Pin ignored by gateway')).toBeInTheDocument()
    })
    expect(
      screen.getByText(
        'This model stays on the provider chosen by the gateway even when a pin is sent, so the pin is not enforced.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByText('Pin holds')).not.toBeInTheDocument()
  })

  it('distinguishes an unsupported pipeline from an ignored pin', async () => {
    postMock.mockResolvedValueOnce(probeResponse(false, false))
    const user = userEvent.setup()
    renderPanel({})

    await user.click(screen.getByRole('button', { name: /Probe upstreams/i }))

    await waitFor(() => {
      expect(screen.getByText('Pinning not supported')).toBeInTheDocument()
    })
    expect(screen.queryByText('Pin ignored by gateway')).not.toBeInTheDocument()
  })
})
