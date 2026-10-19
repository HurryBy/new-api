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
import { describe, expect, test } from 'vitest'

import { CHANNEL_TYPE_CLINE, CHANNEL_TYPE_OPTIONS } from '../../constants'
import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  buildSettingJSON,
  channelFormSchema,
  transformChannelToFormDefaults,
} from '../channel-form'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon } from '../channel-utils'

function clineForm(override: Record<string, unknown> = {}) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Cline Pass',
    type: CHANNEL_TYPE_CLINE,
    base_url: '',
    key: 'sk_test',
    models: 'cline-pass/glm-5.2',
    ...override,
  }
}

function clineChannel(setting: string): Channel {
  return {
    id: 1,
    type: CHANNEL_TYPE_CLINE,
    key: '',
    status: 1,
    name: 'Cline Pass',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance: 0,
    balance_updated_time: 0,
    used_quota: 0,
    other_info: '',
    remark: '',
    max_input_tokens: 0,
    setting,
    settings: '{}',
    channel_info: {
      is_multi_key: false,
      multi_key_size: 0,
      multi_key_polling_index: 0,
      multi_key_mode: 'random',
      multi_key_retry_times: 0,
    },
  } as Channel
}

describe('Cline channel', () => {
  test('registers a selection entry and config that do not collide with upstream types', () => {
    expect(
      CHANNEL_TYPE_OPTIONS.find((item) => item.value === CHANNEL_TYPE_CLINE)
    ).toEqual({ value: 64, label: 'Cline' })

    // 上游已把 61/62/63 分配给 Task Plugin / vLLM / SGLang，Cline 必须取 64，
    // 否则会和 ChannelTypeTaskPlugin 撞号，新建的 Cline 渠道会被当成任务插件。
    expect(CHANNEL_TYPE_CLINE).toBe(64)
    expect(CHANNEL_TYPE_CLINE).not.toBe(61)

    expect(getChannelTypeIcon(CHANNEL_TYPE_CLINE)).toBe('OpenAI')

    const config = getChannelTypeConfig(CHANNEL_TYPE_CLINE)
    expect(config.id).toBe(64)
    // 后端默认 base 不含版本段：new-api 拼接的请求路径自带 /v1，
    // 拼出来才是官方的 https://api.cline.bot/api/v1/chat/completions。
    expect(config.hints?.baseUrl).toBe(
      '留空则使用 Cline Pass 默认网关 https://api.cline.bot/api'
    )
    expect(config.hints?.models).toBe('例如 cline-pass/glm-5.2')
  })

  test('accepts a blank Base URL so the backend default gateway applies', () => {
    expect(channelFormSchema.safeParse(clineForm()).success).toBe(true)
    expect(
      channelFormSchema.safeParse(clineForm({ base_url: '  ' })).success
    ).toBe(true)
  })

  test('preserves the probe panel keys when the form saves an unrelated field', () => {
    const stored = JSON.stringify({
      force_format: false,
      cline_upstream_pins: {
        'cline-pass/glm-5.2': { upstreams: ['alibaba'], pin_mode: 'strict' },
      },
      cline_probe_snapshot: {
        sampled_at: 1700000000,
        models: [{ model: 'cline-pass/glm-5.2' }],
      },
    })

    const rebuilt = JSON.parse(buildSettingJSON(clineForm(), stored))

    expect(rebuilt.cline_upstream_pins).toEqual({
      'cline-pass/glm-5.2': { upstreams: ['alibaba'], pin_mode: 'strict' },
    })
    expect(rebuilt.cline_probe_snapshot).toEqual({
      sampled_at: 1700000000,
      models: [{ model: 'cline-pass/glm-5.2' }],
    })
  })

  test('omits the probe panel keys when the channel has none stored', () => {
    const rebuilt = JSON.parse(
      buildSettingJSON(clineForm(), '{"force_format":false}')
    )

    expect('cline_upstream_pins' in rebuilt).toBe(false)
    expect('cline_probe_snapshot' in rebuilt).toBe(false)
  })

  test('saves the rest of the setting even when the stored JSON is unreadable', () => {
    const rebuilt = JSON.parse(buildSettingJSON(clineForm(), '{not json}'))

    expect(rebuilt.force_format).toBe(false)
    expect('cline_upstream_pins' in rebuilt).toBe(false)
  })

  test('keeps pins out of the form so the probe panel owns them', () => {
    const form = transformChannelToFormDefaults(
      clineChannel(
        '{"force_format":false,"cline_upstream_pins":{"cline-pass/glm-5.2":{"upstreams":["alibaba"]}}}'
      )
    )

    expect('cline_upstream_pins' in form).toBe(false)
  })
})
