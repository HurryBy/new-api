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
import { Link } from '@tanstack/react-router'
import { ArrowUpRight, Check, Coins, Wallet, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'
import { useSystemConfig } from '@/hooks/use-system-config'

const modelSources = [
  { name: 'OpenAI', icon: OpenAI },
  { name: 'Claude', icon: Claude },
  { name: 'Gemini', icon: Gemini },
  { name: 'DeepSeek', icon: DeepSeek },
]

export function ModelValue() {
  const { t } = useTranslation()
  const { logo, systemName } = useSystemConfig()

  return (
    <section
      className='astral-section astral-value'
      aria-labelledby='astral-value-title'
    >
      <AnimateInView className='astral-value-copy'>
        <p className='astral-eyebrow'>{t('More value per token')}</p>
        <h2 id='astral-value-title'>
          {t('All your models.')}
          <br />
          <span className='astral-gradient-text'>
            {t('Less of your budget.')}
          </span>
        </h2>
        <p className='astral-section-description'>
          {t(
            'Leading text models, gathered in one place. Choose the right balance of capability and price, and put your budget into the work that matters.'
          )}
        </p>
        <ul className='astral-value-benefits'>
          <li>
            <Wallet size={18} aria-hidden='true' />
            <div>
              <strong>{t('One balance, more choice')}</strong>
              <p>
                {t(
                  'Use multiple model families without juggling separate accounts and balances.'
                )}
              </p>
            </div>
          </li>
          <li>
            <Zap size={18} aria-hidden='true' />
            <div>
              <strong>{t('Match the model to your budget')}</strong>
              <p>
                {t(
                  'Choose lightweight models for daily tasks and advanced models for complex work.'
                )}
              </p>
            </div>
          </li>
          <li>
            <Coins size={18} aria-hidden='true' />
            <div>
              <strong>{t('Clear prices, informed choices')}</strong>
              <p>
                {t(
                  'Compare model prices before you call. See current rates in the model catalog.'
                )}
              </p>
            </div>
          </li>
        </ul>
        <Button
          variant='outline'
          className='astral-secondary-button'
          render={<Link to='/pricing' />}
        >
          {t('Explore model pricing')}
          <ArrowUpRight size={16} aria-hidden='true' />
        </Button>
      </AnimateInView>
      <AnimateInView className='astral-value-visual' delay={100}>
        <div className='astral-value-caption'>
          <span className='astral-eyebrow'>
            {t('Many models. One balance.')}
          </span>
          <Coins size={18} aria-hidden='true' />
        </div>
        <div className='astral-model-convergence'>
          <div className='astral-convergence-sources'>
            {modelSources.map((source) => (
              <div key={source.name}>
                <source.icon size={21} aria-hidden='true' />
                <span>{source.name}</span>
              </div>
            ))}
          </div>
          <div className='astral-convergence-tracks' aria-hidden='true'>
            <svg viewBox='0 0 160 260' preserveAspectRatio='none' fill='none'>
              <path d='M0 30C95 30 55 130 160 130M0 96C75 96 75 130 160 130M0 164C75 164 75 130 160 130M0 230C95 230 55 130 160 130' />
            </svg>
            <i />
            <i />
            <i />
            <i />
          </div>
          <div className='astral-convergence-hub'>
            <div className='astral-hub-logo'>
              <img src={logo} alt='' />
            </div>
            <strong>{systemName}</strong>
            <span>{t('Unified access')}</span>
          </div>
        </div>
        <div className='astral-value-wallet'>
          <span className='astral-wallet-icon'>
            <Wallet size={26} aria-hidden='true' />
          </span>
          <div>
            <span>{t('Less overhead. More intelligence.')}</span>
            <strong>{t('A smarter way to spend on AI')}</strong>
          </div>
          <ArrowUpRight size={22} aria-hidden='true' />
        </div>
        <div className='astral-value-footer'>
          <Check size={13} aria-hidden='true' />
          {t('Pay as you go. Choose as you grow.')}
        </div>
      </AnimateInView>
    </section>
  )
}
