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
import {
  Activity,
  BarChart3,
  Box,
  Check,
  Code2,
  Coins,
  FileText,
  Gauge,
  Hash,
  HeartPulse,
  KeyRound,
  LayoutDashboard,
  MessageSquare,
  Settings2,
  SlidersHorizontal,
  Users,
  Wallet,
  Zap,
} from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Mockup } from '@/components/layout/components/mockup'
import { IconBadge } from '@/components/ui/icon-badge'
import { useMediaQuery } from '@/hooks/use-media-query'
import { useSystemConfig } from '@/hooks/use-system-config'
import { toIntlLocale } from '@/i18n/languages'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatCompactNumber, formatNumber } from '@/lib/format'

// A static console illustration: these values demonstrate the dashboard layout.
const consumption = [76, 16, 28, 8, 44, 20, 57, 32, 68, 43, 88, 61].map(
  (height, hour) => ({ hour: `${hour * 2}:00`, height })
)

export function GatewayPreview() {
  const { t, i18n } = useTranslation()
  const { systemName, logo } = useSystemConfig()
  const scene = useRef<HTMLDivElement>(null)
  const reduceMotion = useMediaQuery('(prefers-reduced-motion: reduce)')
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const spend = formatCurrencyFromUSD(9.73, { locale, abbreviate: false })
  const metrics = [
    {
      label: t('Total Count'),
      value: formatNumber(535, locale),
      description: t('Request count'),
      icon: Hash,
      tone: 'info',
    },
    {
      label: t('Total Quota'),
      value: spend,
      description: t('Total spending'),
      icon: Coins,
      tone: 'success',
    },
    {
      label: t('Total Tokens'),
      value: formatCompactNumber(160000000, locale),
      description: t('Token usage'),
      icon: Box,
      tone: 'primary',
    },
    {
      label: t('Average RPM'),
      value: formatNumber(0.37, locale),
      description: t('Requests per minute'),
      icon: Gauge,
      tone: 'info',
    },
    {
      label: t('Average TPM'),
      value: formatCompactNumber(113000, locale),
      description: t('Tokens per minute'),
      icon: Zap,
      tone: 'warning',
    },
  ] as const
  const navigation = [
    { label: t('Overview'), icon: Activity },
    { label: t('Data Dashboard'), icon: LayoutDashboard, active: true },
    { label: t('API Keys'), icon: KeyRound },
    { label: t('Usage Logs'), icon: FileText },
    { label: t('Wallet'), icon: Wallet },
    { label: t('Channels'), icon: SlidersHorizontal },
    { label: t('Models'), icon: Box },
    { label: t('Users'), icon: Users },
  ]
  useEffect(() => {
    const element = scene.current
    if (!element || reduceMotion) return
    let frame = 0
    const move = (event: PointerEvent) => {
      if (event.pointerType === 'touch') return
      const bounds = element.getBoundingClientRect()
      const x = Math.max(
        -0.5,
        Math.min(0.5, (event.clientX - bounds.left) / bounds.width - 0.5)
      )
      const y = Math.max(
        -0.5,
        Math.min(0.5, (event.clientY - bounds.top) / bounds.height - 0.5)
      )
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        element.style.setProperty('--tilt-x', `${-y * 14}deg`)
        element.style.setProperty('--tilt-y', `${x * 18}deg`)
        element.style.setProperty('--shine-x', `${(x + 0.5) * 100}%`)
        element.style.setProperty('--shine-y', `${(y + 0.5) * 100}%`)
      })
    }
    const reset = () => {
      cancelAnimationFrame(frame)
      element.style.removeProperty('--tilt-x')
      element.style.removeProperty('--tilt-y')
      element.style.removeProperty('--shine-x')
      element.style.removeProperty('--shine-y')
    }
    element.addEventListener('pointermove', move)
    element.addEventListener('pointerleave', reset)
    element.addEventListener('pointercancel', reset)
    return () => {
      reset()
      element.removeEventListener('pointermove', move)
      element.removeEventListener('pointerleave', reset)
      element.removeEventListener('pointercancel', reset)
    }
  }, [reduceMotion])

  return (
    <div
      ref={scene}
      className='gateway-scene'
      role='img'
      aria-label={t(
        'Interactive preview of usage analytics in the {{systemName}} console',
        { systemName }
      )}
    >
      <div className='gateway-orbit gateway-orbit-outer' aria-hidden='true' />
      <div className='gateway-orbit gateway-orbit-inner' aria-hidden='true' />
      <div className='gateway-perspective'>
        <Mockup className='gateway-window'>
          <div className='gateway-window-bar'>
            <div className='gateway-window-dots' aria-hidden='true'>
              <i />
              <i />
              <i />
            </div>
            <span>
              {systemName} / {t('Console')}
            </span>
            <span className='gateway-demo-label'>{t('Demo preview')}</span>
          </div>
          <div className='gateway-window-body'>
            <div className='gateway-sidebar'>
              <div className='gateway-sidebar-brand'>
                <img src={logo} alt='' className='gateway-brand' />
                <strong>{systemName}</strong>
              </div>
              <span className='gateway-nav-group'>{t('Chat')}</span>
              <span className='gateway-nav-item'>
                <MessageSquare size={12} aria-hidden='true' />
                {t('Playground')}
              </span>
              <span className='gateway-nav-group'>{t('General')}</span>
              {navigation.map((item) => (
                <span
                  key={item.label}
                  className={`gateway-nav-item${item.active ? ' gateway-sidebar-active' : ''}`}
                >
                  <item.icon size={12} aria-hidden='true' />
                  <span>{item.label}</span>
                </span>
              ))}
              <span className='gateway-nav-item gateway-sidebar-bottom'>
                <Settings2 size={12} aria-hidden='true' />
                {t('Settings')}
              </span>
            </div>
            <div className='gateway-workspace'>
              <div className='gateway-heading'>
                <h3>{t('Model Usage Analytics')}</h3>
                <span className='gateway-status-dot' aria-hidden='true' />
              </div>
              <div className='gateway-toolbar'>
                <div className='gateway-view-tabs'>
                  <span>{t('Model Usage Analytics')}</span>
                  <span>{t('Flow')}</span>
                  <span>{t('User Statistics')}</span>
                </div>
                <SlidersHorizontal size={12} aria-hidden='true' />
              </div>
              <div className='gateway-metrics'>
                {metrics.map((metric) => (
                  <div key={metric.label}>
                    <span className='gateway-metric-title'>
                      <IconBadge tone={metric.tone} size='xs'>
                        <metric.icon />
                      </IconBadge>
                      <span>{metric.label}</span>
                    </span>
                    <strong>{metric.value}</strong>
                    <small>{metric.description}</small>
                  </div>
                ))}
              </div>
              <div className='gateway-health'>
                <strong>
                  <HeartPulse size={12} aria-hidden='true' />
                  {t('Performance')}
                </strong>
                <span>
                  {t('Success rate')} <b>{formatNumber(99.26, locale)}%</b>
                </span>
                <span>
                  {t('Average latency')}{' '}
                  <b>
                    {t('{{value}} s', { value: formatNumber(6.29, locale) })}
                  </b>
                </span>
                <span>
                  {t('Throughput')}{' '}
                  <b>
                    {t('{{value}} tokens/s', {
                      value: formatNumber(193, locale),
                    })}
                  </b>
                </span>
              </div>
              <div className='gateway-consumption'>
                <div className='gateway-panel-label'>
                  <span>
                    <IconBadge tone='success' size='xs'>
                      <Wallet />
                    </IconBadge>
                    {t('Quota Distribution')}
                    <small>
                      {t('Total:')} {spend}
                    </small>
                  </span>
                  <BarChart3 size={12} aria-hidden='true' />
                </div>
                <div className='gateway-bar-chart' aria-hidden='true'>
                  <div className='gateway-chart-axis'>
                    {[10, 5, 0].map((value) => (
                      <span key={value}>{formatNumber(value, locale)}</span>
                    ))}
                  </div>
                  <div className='gateway-chart-plot'>
                    <div className='gateway-chart-grid' />
                    <div className='gateway-chart-bars'>
                      {consumption.map((bucket) => (
                        <div
                          key={bucket.hour}
                          style={{ height: `${bucket.height}%` }}
                        >
                          <i />
                          <i />
                        </div>
                      ))}
                    </div>
                    <div className='gateway-chart-times'>
                      <span>00:00</span>
                      <span>06:00</span>
                      <span>12:00</span>
                      <span>18:00</span>
                    </div>
                  </div>
                </div>
                <div className='gateway-chart-legend'>
                  <span>
                    <i />
                    DeepSeek
                  </span>
                  <span>
                    <i />
                    Claude
                  </span>
                  <span>
                    <i />
                    Gemini
                  </span>
                </div>
              </div>
              <div className='gateway-mini-trend'>
                <span>
                  <Activity size={12} aria-hidden='true' />
                  {t('Request activity')}
                </span>
                <svg viewBox='0 0 160 26' fill='none' aria-hidden='true'>
                  <path
                    d='M0 21L12 19L22 23L35 12L47 17L59 8L71 15L83 9L95 13L107 4L120 10L133 2L146 7L160 1'
                    stroke='currentColor'
                    strokeWidth='1.5'
                  />
                </svg>
              </div>
            </div>
          </div>
        </Mockup>
        <div className='gateway-floating-code'>
          <span className='gateway-code-icon'>
            <Code2 size={19} aria-hidden='true' />
          </span>
          <div>
            <small>{t('One connection, many models')}</small>
            <code>POST /v1/chat/completions</code>
          </div>
          <span className='gateway-code-ok'>
            <Check size={10} aria-hidden='true' />
            200 OK
          </span>
        </div>
      </div>
    </div>
  )
}
