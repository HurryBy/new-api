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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { CHANNEL_TYPE_CLINE, CHANNEL_TYPE_OPTIONS } from '../../constants'
import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  buildSettingJSON,
  channelFormSchema,
  parseClineUpstreamPins,
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
  test('registers selection, icon, and versionless default Base URL', () => {
    assert.deepEqual(
      CHANNEL_TYPE_OPTIONS.find((item) => item.value === CHANNEL_TYPE_CLINE),
      { value: 61, label: 'Cline' }
    )

    const config = getChannelTypeConfig(CHANNEL_TYPE_CLINE)

    // new-api appends the request path (which already carries /v1) to the base URL.
    assert.equal(config.defaultBaseUrl, 'https://api.cline.bot/api')
    assert.equal(getChannelTypeIcon(CHANNEL_TYPE_CLINE), 'OpenAI')
    assert.equal(config.hints?.models, '例如 cline-pass/glm-5.2')
  })

  test('accepts a blank Base URL so the backend default gateway applies', () => {
    assert.equal(channelFormSchema.safeParse(clineForm()).success, true)
    assert.equal(
      channelFormSchema.safeParse(clineForm({ base_url: '  ' })).success,
      true
    )
  })

  test('normalizes configured pins and drops empty fields', () => {
    const pins = parseClineUpstreamPins(
      '{"cline-pass/glm-5.2":{"upstreams":[" alibaba ","baseten",""],"exclude":["legacy"],"pin_mode":"preferred","sort":"cost"},"cline-pass/other":{"upstreams":[],"sort":""}}'
    )

    assert.deepEqual(pins, {
      'cline-pass/glm-5.2': {
        upstreams: ['alibaba', 'baseten'],
        exclude: ['legacy'],
        pin_mode: 'preferred',
        sort: 'cost',
      },
      'cline-pass/other': {},
    })
    assert.equal(parseClineUpstreamPins('   '), null)
    assert.equal(parseClineUpstreamPins(undefined), null)
  })

  test('rejects pins that are not a model-to-pin object', () => {
    for (const invalid of [
      '[]',
      '"model"',
      '{"cline-pass/glm-5.2": 1}',
      '{"   ": {}}',
      '{"cline-pass/glm-5.2": {"upstreams": "alibaba"}}',
      '{"cline-pass/glm-5.2": {"upstreams": [7]}}',
      '{"cline-pass/glm-5.2": {"pin_mode": "sequential"}}',
      '{"cline-pass/glm-5.2": {"sort": "latency"}}',
      '{not json}',
    ]) {
      assert.throws(() => parseClineUpstreamPins(invalid), invalid)
      const result = channelFormSchema.safeParse(
        clineForm({ cline_upstream_pins: invalid })
      )
      assert.equal(result.success, false, invalid)
      if (!result.success) {
        assert.equal(result.error.issues[0]?.path[0], 'cline_upstream_pins')
      }
    }
  })

  test('writes pins into the setting JSON only when configured', () => {
    const withoutPins = JSON.parse(buildSettingJSON(clineForm()))

    assert.equal('cline_upstream_pins' in withoutPins, false)

    const withPins = JSON.parse(
      buildSettingJSON(
        clineForm({
          cline_upstream_pins:
            '{"cline-pass/glm-5.2":{"upstreams":["alibaba"],"pin_mode":"strict"}}',
        })
      )
    )

    assert.deepEqual(withPins.cline_upstream_pins, {
      'cline-pass/glm-5.2': { upstreams: ['alibaba'], pin_mode: 'strict' },
    })
  })

  test('round-trips pins between the stored setting and the form field', () => {
    const stored =
      '{"cline-pass/glm-5.2":{"upstreams":["alibaba","baseten"],"pin_mode":"preferred","sort":"cost"}}'
    const form = transformChannelToFormDefaults(
      clineChannel(`{"force_format":false,"cline_upstream_pins":${stored}}`)
    )

    assert.deepEqual(
      JSON.parse(form.cline_upstream_pins || ''),
      JSON.parse(stored)
    )
    assert.deepEqual(
      JSON.parse(buildSettingJSON(form)).cline_upstream_pins,
      JSON.parse(stored)
    )

    assert.equal(
      transformChannelToFormDefaults(clineChannel('{"force_format":false}'))
        .cline_upstream_pins,
      ''
    )
  })
})
