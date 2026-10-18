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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { fetchClineSubscriptionModels, probeClineChannel } from '../cline-probe'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn() },
}))

const getMock = vi.mocked(api.get)
const postMock = vi.mocked(api.post)

function probeBody(overrides: Record<string, unknown> = {}) {
  return {
    success: true,
    message: '',
    data: {
      sampledAt: 1_700_000_000_000,
      models: [
        {
          model: 'cline-pass/deepseek-v4.1-flash',
          reachable: true,
          pipeline: 'gateway',
          pinnable: true,
          pinnedHeld: false,
          finalProvider: 'deepseek',
          available: ['deepseek', 'fireworks'],
          elapsedMs: 1200,
          ...overrides,
        },
      ],
    },
  }
}

beforeEach(() => {
  getMock.mockReset()
  postMock.mockReset()
})

describe('fetchClineSubscriptionModels', () => {
  test('returns the subscription catalogue from the backend', async () => {
    getMock.mockResolvedValueOnce({
      data: {
        success: true,
        data: ['cline-pass/deepseek-v4.1-flash', 'cline-pass/glm-5.2'],
      },
    })

    const models = await fetchClineSubscriptionModels()

    expect(getMock).toHaveBeenCalledWith('/api/channel/cline/models')
    expect(models).toEqual([
      'cline-pass/deepseek-v4.1-flash',
      'cline-pass/glm-5.2',
    ])
  })

  test('returns an empty list when the backend sends no data', async () => {
    getMock.mockResolvedValueOnce({ data: { success: true } })

    await expect(fetchClineSubscriptionModels()).resolves.toEqual([])
  })

  test('surfaces the backend message when the catalogue request fails', async () => {
    getMock.mockResolvedValueOnce({
      data: { success: false, message: 'forbidden' },
    })

    await expect(fetchClineSubscriptionModels()).rejects.toThrow('forbidden')
  })
})

describe('probeClineChannel', () => {
  test('posts to the channel probe route with the selected models', async () => {
    postMock.mockResolvedValueOnce({ data: probeBody() })

    await probeClineChannel(7, ['cline-pass/deepseek-v4.1-flash'])

    expect(postMock).toHaveBeenCalledWith(
      '/api/channel/7/cline/probe',
      { models: ['cline-pass/deepseek-v4.1-flash'] },
      expect.objectContaining({ timeout: expect.any(Number) })
    )
  })

  test('sends an empty body so the backend probes every configured model', async () => {
    postMock.mockResolvedValueOnce({ data: probeBody() })

    await probeClineChannel(7)

    expect(postMock).toHaveBeenCalledWith(
      '/api/channel/7/cline/probe',
      {},
      expect.anything()
    )
  })

  test('returns the parsed probe data', async () => {
    postMock.mockResolvedValueOnce({ data: probeBody() })

    const result = await probeClineChannel(7)

    expect(result.models).toHaveLength(1)
    expect(result.models[0].model).toBe('cline-pass/deepseek-v4.1-flash')
    expect(result.models[0].pinnable).toBe(true)
    expect(result.models[0].pinnedHeld).toBe(false)
  })

  test('keeps pin-ignored distinct from reachability so the UI cannot imply success', async () => {
    postMock.mockResolvedValueOnce({ data: probeBody() })

    const [model] = (await probeClineChannel(7)).models

    // The model works, but the pin was not honoured: both must be visible.
    expect(model.reachable).toBe(true)
    expect(model.pinnedHeld).toBe(false)
    expect(model.finalProvider).toBe('deepseek')
  })

  test('surfaces the backend message when the response is unsuccessful', async () => {
    postMock.mockResolvedValueOnce({
      data: { success: false, message: 'No enabled channel key' },
    })

    await expect(probeClineChannel(7)).rejects.toThrow('No enabled channel key')
  })

  test('surfaces a failure when the response carries no data', async () => {
    postMock.mockResolvedValueOnce({ data: { success: true } })

    await expect(probeClineChannel(7)).rejects.toThrow('Probe failed')
  })
})
