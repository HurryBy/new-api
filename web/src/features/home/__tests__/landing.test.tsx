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
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { AstralLanding } from '../components/astral-landing'
import { GatewayPreview } from '../components/gateway-preview'

const clients: QueryClient[] = []

afterEach(async () => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  vi.unstubAllGlobals()
  await i18next.changeLanguage('en')
})

async function renderLanding(isAuthenticated = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  clients.push(client)
  client.setQueryData(STATUS_QUERY_KEY, {
    docs_link: 'https://docs.newapi.pro',
  })
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => <AstralLanding isAuthenticated={isAuthenticated} />,
    }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('heading', { level: 1 })
  return router
}

function renderPreview() {
  const client = new QueryClient()
  clients.push(client)
  render(
    <QueryClientProvider client={client}>
      <GatewayPreview />
    </QueryClientProvider>
  )
}

describe('landing page interactions', () => {
  it('switches model categories with both pointer and keyboard and keeps the selected panel accessible', async () => {
    const user = userEvent.setup()
    await renderLanding()
    await user.click(screen.getByRole('tab', { name: 'Long-context analysis' }))
    expect(
      screen.getByRole('tab', { name: 'Long-context analysis' })
    ).toHaveAttribute('aria-selected', 'true')
    expect(
      within(screen.getByRole('tabpanel')).getByRole('heading', {
        name: 'Gemini',
      })
    ).toBeVisible()
    await user.keyboard('{ArrowLeft}{Enter}')
    await waitFor(() =>
      expect(
        screen.getByRole('tab', { name: 'Writing & analysis' })
      ).toHaveAttribute('aria-selected', 'true')
    )
    expect(
      within(screen.getByRole('tabpanel')).getByRole('heading', {
        name: 'Claude',
      })
    ).toBeVisible()
  })

  it.each([false, true])(
    'sends the primary action to the correct destination when authenticated=%s',
    async (authenticated) => {
      const user = userEvent.setup()
      const router = await renderLanding(authenticated)
      const label = authenticated ? 'Go to Dashboard' : 'Start building'
      await user.click(screen.getAllByRole('button', { name: label })[0])
      await waitFor(() =>
        expect(router.state.location.pathname).toBe(
          authenticated ? '/dashboard' : '/sign-up'
        )
      )
    }
  )

  it('opens model pricing from the value section and focuses the catalog on text capabilities', async () => {
    const user = userEvent.setup()
    const router = await renderLanding()
    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      'Conversation & reasoning',
      'Code & agents',
      'Writing & analysis',
      'Long-context analysis',
    ])
    await user.click(
      screen.getByRole('button', { name: 'Explore model pricing' })
    )
    await waitFor(() => expect(router.state.location.pathname).toBe('/pricing'))
  })

  it('updates every landing brand label and logo when system settings change', async () => {
    useSystemConfigStore.getState().setConfig({
      systemName: 'Test Gateway',
      logo: 'https://example.com/brand.svg',
    })
    await renderLanding()
    expect(
      screen.getByRole('img', {
        name: 'Interactive preview of usage analytics in the Test Gateway console',
      })
    ).toBeVisible()
    expect(
      screen.getByText('Test Gateway / More value per token')
    ).toBeVisible()
    expect(screen.getAllByAltText('', { exact: true })).toHaveLength(2)
    for (const img of screen.getAllByAltText('', { exact: true })) {
      expect(img).toHaveAttribute('src', 'https://example.com/brand.svg')
    }
    act(() =>
      useSystemConfigStore.getState().setConfig({
        systemName: 'Updated Gateway',
        logo: 'https://example.com/updated.svg',
      })
    )
    expect(
      screen.getByRole('img', {
        name: 'Interactive preview of usage analytics in the Updated Gateway console',
      })
    ).toBeVisible()
    expect(
      screen.getByText('Updated Gateway / More value per token')
    ).toBeVisible()
    for (const img of screen.getAllByAltText('', { exact: true })) {
      expect(img).toHaveAttribute('src', 'https://example.com/updated.svg')
    }
  })

  it.each([
    ['zhCN', '1.6亿'],
    ['zhTW', '1.6億'],
    ['en', '160M'],
    ['fr', '160\u00a0M'],
    ['ja', '1.6億'],
    ['ru', '160\u00a0млн'],
    ['vi', '160\u00a0Tr'],
    [
      'invalid_locale',
      Intl.NumberFormat(undefined, {
        notation: 'compact',
        maximumFractionDigits: 1,
      }).format(160000000),
    ],
  ])(
    'formats console token usage for interface language %s',
    async (language, expected) => {
      await i18next.changeLanguage(language)
      renderPreview()
      expect(
        screen.getByText(expected, { normalizer: (value) => value })
      ).toBeVisible()
    }
  )

  it('reformats console values when the interface language changes', async () => {
    await i18next.changeLanguage('en')
    renderPreview()
    expect(screen.getByText('160M')).toBeVisible()
    await act(() => i18next.changeLanguage('zhTW'))
    expect(screen.getByText('1.6億')).toBeVisible()
    expect(screen.queryByText('160M')).not.toBeInTheDocument()
  })

  it('leaves the preview still when reduced motion is enabled', () => {
    renderPreview()
    const scene = screen.getByRole('img', { name: /Interactive preview/ })
    fireEvent(
      scene,
      new MouseEvent('pointermove', { clientX: 300, clientY: 200 })
    )
    expect(scene.style.getPropertyValue('--tilt-x')).toBe('')
    expect(scene.style.getPropertyValue('--tilt-y')).toBe('')
  })

  it('tilts the preview on pointer movement and resets its angle when the pointer leaves', () => {
    const matchMedia = window.matchMedia
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      ...matchMedia(query),
      matches: false,
    }))
    let frame: FrameRequestCallback | undefined
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      frame = callback
      return 1
    })
    vi.stubGlobal('cancelAnimationFrame', () => {
      frame = undefined
    })
    renderPreview()
    const scene = screen.getByRole('img', { name: /Interactive preview/ })
    vi.spyOn(scene, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 0, 400, 400)
    )
    fireEvent(
      scene,
      new MouseEvent('pointermove', { clientX: 350, clientY: 300 })
    )
    act(() => frame?.(0))
    expect(scene.style.getPropertyValue('--tilt-x')).not.toBe('')
    expect(scene.style.getPropertyValue('--tilt-y')).not.toBe('')
    fireEvent(scene, new MouseEvent('pointerleave'))
    expect(scene.style.getPropertyValue('--tilt-x')).toBe('')
    expect(scene.style.getPropertyValue('--tilt-y')).toBe('')
  })
})
