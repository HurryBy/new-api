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
  ArrowUpRight,
  BookOpen,
  Code2,
  FileText,
  MessageSquare,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function ModelUniverse() {
  const { t } = useTranslation()
  const categories = [
    {
      id: 'language',
      label: t('Conversation & reasoning'),
      icon: MessageSquare,
      models: [
        {
          name: 'GPT',
          provider: 'OpenAI',
          icon: OpenAI,
          detail: t('Conversation & reasoning'),
        },
        {
          name: 'Claude',
          provider: 'Anthropic',
          icon: Claude,
          detail: t('Writing & analysis'),
        },
        {
          name: 'Gemini',
          provider: 'Google',
          icon: Gemini,
          detail: t('Long-context analysis'),
        },
        {
          name: 'DeepSeek',
          provider: 'DeepSeek',
          icon: DeepSeek,
          detail: t('Conversation & reasoning'),
        },
      ],
    },
    {
      id: 'code',
      label: t('Code & agents'),
      icon: Code2,
      models: [
        {
          name: 'Claude',
          provider: 'Anthropic',
          icon: Claude,
          detail: t('Code & agents'),
        },
        {
          name: 'GPT',
          provider: 'OpenAI',
          icon: OpenAI,
          detail: t('Code & agents'),
        },
        {
          name: 'Qwen Coder',
          provider: 'Alibaba',
          icon: Qwen,
          detail: t('Code & agents'),
        },
        {
          name: 'DeepSeek',
          provider: 'DeepSeek',
          icon: DeepSeek,
          detail: t('Code & agents'),
        },
      ],
    },
    {
      id: 'writing',
      label: t('Writing & analysis'),
      icon: FileText,
      models: [
        {
          name: 'Claude',
          provider: 'Anthropic',
          icon: Claude,
          detail: t('Writing & analysis'),
        },
        {
          name: 'GPT',
          provider: 'OpenAI',
          icon: OpenAI,
          detail: t('Writing & analysis'),
        },
        {
          name: 'Gemini',
          provider: 'Google',
          icon: Gemini,
          detail: t('Writing & analysis'),
        },
        {
          name: 'Qwen',
          provider: 'Alibaba',
          icon: Qwen,
          detail: t('Writing & analysis'),
        },
      ],
    },
    {
      id: 'context',
      label: t('Long-context analysis'),
      icon: BookOpen,
      models: [
        {
          name: 'Gemini',
          provider: 'Google',
          icon: Gemini,
          detail: t('Long-context analysis'),
        },
        {
          name: 'Claude',
          provider: 'Anthropic',
          icon: Claude,
          detail: t('Long-context analysis'),
        },
        {
          name: 'GPT',
          provider: 'OpenAI',
          icon: OpenAI,
          detail: t('Long-context analysis'),
        },
        {
          name: 'Qwen',
          provider: 'Alibaba',
          icon: Qwen,
          detail: t('Long-context analysis'),
        },
      ],
    },
  ]

  return (
    <section
      className='astral-section astral-models'
      id='model-universe'
      aria-labelledby='model-universe-title'
    >
      <AnimateInView className='astral-section-heading'>
        <div>
          <p className='astral-eyebrow'>{t('A universe of models')}</p>
          <h2 id='model-universe-title'>
            {t('The right intelligence.')}
            <br />
            <span>{t('For everything you imagine.')}</span>
          </h2>
        </div>
        <p className='astral-section-description'>
          {t(
            'Reason, write, code and understand long documents. Explore leading text models through one unified API.'
          )}
        </p>
      </AnimateInView>
      <Tabs defaultValue='language' className='astral-model-browser'>
        <TabsList
          className='astral-model-tabs'
          aria-label={t('Model capabilities')}
        >
          {categories.map((category) => (
            <TabsTrigger key={category.id} value={category.id}>
              <category.icon size={16} aria-hidden='true' />
              {category.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {categories.map((category) => (
          <TabsContent
            key={category.id}
            value={category.id}
            className='astral-model-panel'
          >
            <div className='astral-model-grid'>
              {category.models.map((model) => (
                <Link
                  to='/pricing'
                  className='astral-model-card'
                  key={model.name}
                >
                  <div className='astral-model-card-top'>
                    <span className='astral-provider-icon'>
                      <model.icon size={30} aria-hidden='true' />
                    </span>
                    <ArrowUpRight size={17} aria-hidden='true' />
                  </div>
                  <span className='astral-model-provider'>
                    {model.provider}
                  </span>
                  <h3>{model.name}</h3>
                  <p>{model.detail}</p>
                  <span className='astral-model-link'>
                    {t('Explore models')}{' '}
                    <ArrowUpRight size={13} aria-hidden='true' />
                  </span>
                </Link>
              ))}
            </div>
          </TabsContent>
        ))}
        <div className='astral-model-browser-footer'>
          <p>
            {t(
              'Model families shown as examples. See the catalog for current availability and pricing.'
            )}
          </p>
          <Button variant='ghost' render={<Link to='/pricing' />}>
            {t('View all models')}
            <ArrowUpRight size={15} aria-hidden='true' />
          </Button>
        </div>
      </Tabs>
    </section>
  )
}
