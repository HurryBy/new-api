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
import Claude from '@lobehub/icons/es/Claude'
import DeepSeek from '@lobehub/icons/es/DeepSeek'
import Gemini from '@lobehub/icons/es/Gemini'
import OpenAI from '@lobehub/icons/es/OpenAI'
import Qwen from '@lobehub/icons/es/Qwen'
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  ArrowUpRight,
  GitBranch,
  MessageSquareText,
  ReceiptText,
  Sparkles,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'
import { useSystemConfig } from '@/hooks/use-system-config'

import { AstralAtmosphere } from './astral-atmosphere'
import { GatewayPreview } from './gateway-preview'
import { ModelUniverse } from './model-universe'
import { ModelValue } from './model-value'

const providers = [
  { name: 'OpenAI', icon: OpenAI },
  { name: 'Anthropic', icon: Claude },
  { name: 'Google Gemini', icon: Gemini },
  { name: 'DeepSeek', icon: DeepSeek },
  { name: 'Qwen', icon: Qwen },
]

export function AstralLanding(props: { isAuthenticated: boolean }) {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()
  const destination = props.isAuthenticated ? '/dashboard' : '/sign-up'
  const actionLabel = props.isAuthenticated
    ? t('Go to Dashboard')
    : t('Start building')

  return (
    <main className='astral-main'>
      <AstralAtmosphere />
      <section className='astral-hero' aria-labelledby='astral-title'>
        <div className='astral-hero-inner'>
          <div className='astral-hero-copy'>
            <span className='astral-hero-badge'>
              <Sparkles size={13} aria-hidden='true' />
              {t('One gateway. Every possibility.')}
            </span>
            <h1 id='astral-title'>
              {t('A world of AI.')}
              <br />
              {t('One API.')}
              <br />
              <span className='astral-gradient-text'>
                {t('Build beyond limits.')}
              </span>
            </h1>
            <p className='astral-hero-description'>
              {t(
                'Bring GPT, Claude, Gemini and DeepSeek together. Write, reason and code with leading text models, at prices that go further.'
              )}
            </p>
            <div className='astral-actions'>
              <Button
                className='astral-primary-button'
                render={<Link to={destination} />}
              >
                {actionLabel}
                <ArrowRight size={16} aria-hidden='true' />
              </Button>
              <Button
                variant='outline'
                className='astral-secondary-button'
                render={<a href='#model-universe' />}
              >
                {t('Explore models')}
                <ArrowUpRight size={16} aria-hidden='true' />
              </Button>
            </div>
            <div className='astral-hero-footnote'>
              <span />
              {t('Leading text models. One balance. More value.')}
            </div>
          </div>
          <GatewayPreview />
        </div>
        <div className='astral-provider-rail'>
          <p>{t('Great models. Connected in one place.')}</p>
          <div>
            {providers.map((provider) => (
              <span key={provider.name}>
                <provider.icon size={26} aria-hidden='true' />
                {provider.name}
              </span>
            ))}
            <span className='astral-provider-more'>+ {t('More')}</span>
          </div>
        </div>
      </section>

      <ModelUniverse />
      <ModelValue />

      <section
        className='astral-section astral-capabilities'
        aria-label={t('Platform capabilities')}
      >
        <AnimateInView>
          <GitBranch size={23} aria-hidden='true' />
          <h3>{t('One connection, many models')}</h3>
          <p>
            {t(
              'Move between providers through a unified gateway, with model routing that adapts to your setup.'
            )}
          </p>
        </AnimateInView>
        <AnimateInView delay={80}>
          <MessageSquareText size={23} aria-hidden='true' />
          <h3>{t('Built for text and code')}</h3>
          <p>
            {t(
              'From careful reasoning and long-form writing to everyday coding, find the text model that fits the task.'
            )}
          </p>
        </AnimateInView>
        <AnimateInView delay={160}>
          <ReceiptText size={23} aria-hidden='true' />
          <h3>{t('Know where every token goes')}</h3>
          <p>
            {t(
              'Track requests, token usage and spending by model. Make your next model choice with a clear view of the cost.'
            )}
          </p>
        </AnimateInView>
      </section>

      <section className='astral-final-cta' aria-labelledby='astral-cta-title'>
        <div className='astral-cta-halo' aria-hidden='true' />
        <p className='astral-eyebrow'>
          {systemName} / {t('More value per token')}
        </p>
        <h2 id='astral-cta-title'>
          {t('Great models. Lighter on your budget.')}
        </h2>
        <p>{t('Start with one API. Find your next favorite model.')}</p>
        <Button
          className='astral-primary-button'
          render={<Link to={destination} />}
        >
          {actionLabel}
          <ArrowRight size={16} aria-hidden='true' />
        </Button>
      </section>
    </main>
  )
}
