package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogOtherScopesAndMerges(t *testing.T) {
	var other LogOther

	assert.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	other.MergePublic(map[string]any{
		"zero": 0,
	})
	assert.True(t, other.SetAdmin("use_channel", []string{"channel-a"}))
	other.MergeAdmin(map[string]any{
		"rejected": false,
	})
	assert.True(t, other.SetRoot("upstream_request_id", "upstream-private"))
	other.MergeRoot(map[string]any{
		"generation": 0,
	})
	assert.True(t, other.SetAudit("method", "POST"))
	other.MergeAudit(map[string]any{
		"success": false,
	})

	require.JSONEq(t, `{
		"request_path": "/v1/chat/completions",
		"zero": 0,
		"admin_info": {
			"use_channel": ["channel-a"],
			"rejected": false
		},
		"root_info": {
			"upstream_request_id": "upstream-private",
			"generation": 0
		},
		"audit_info": {
			"method": "POST",
			"success": false
		}
	}`, other.JSONString())
}

func TestLogOtherRejectsSensitivePublicFields(t *testing.T) {
	other := NewLogOther()

	for _, key := range []string{
		"admin_info",
		"root_info",
		"audit_info",
		"channel_id",
		"channel_name",
		"channel_type",
		"reject_reason",
	} {
		assert.False(t, other.SetPublic(key, "must-not-leak"), key)
	}
	other.MergePublic(map[string]any{
		"request_path": "/v1/responses",
		"channel_name": "still-must-not-leak",
		"admin_info":   map[string]any{"secret": true},
	})

	require.JSONEq(t, `{"request_path":"/v1/responses"}`, other.JSONString())
	require.JSONEq(t, `{}`, NewLogOther().JSONString())
}

func TestLogOtherJSONStringDoesNotMutateReceiver(t *testing.T) {
	other := NewLogOther()
	require.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	require.True(t, other.SetAdmin("rejected", false))

	before := other.Snapshot()
	first := other.JSONString()
	after := other.Snapshot()
	second := other.JSONString()

	require.Equal(t, before, after)
	require.Equal(t, first, second)
}

// TestUserProjectionHidesUpstreamIdentity guards the rule that a non-admin caller
// never sees which provider served the request, while the requested model stays
// visible and administrators keep the diagnostics they need to investigate.
func TestUserProjectionHidesUpstreamIdentity(t *testing.T) {
	other := NewLogOther()
	require.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	require.True(t, other.SetAdmin("upstream_model_name", "cline-pass/deepseek-v4.1-flash"))
	require.True(t, other.SetAdmin("response_model", map[string]any{
		"requested_model": "deepseek-v4.1-flash",
		"upstream_model":  "cline-pass/deepseek-v4.1-flash",
		"returned_model":  "deepseek/deepseek-v4.1-flash",
	}))
	require.True(t, other.SetAdmin("use_channel", []string{"channel-a"}))
	stored := other.JSONString()

	userVisible := formatLogOtherJSON(stored, logOtherVisibilityUser)
	require.JSONEq(t, `{"request_path":"/v1/chat/completions"}`, userVisible)

	adminVisible := formatLogOtherJSON(stored, logOtherVisibilityAdmin)
	assert.NotContains(t, adminVisible, `"root_info"`)
	assert.Contains(t, adminVisible, "cline-pass/deepseek-v4.1-flash")

	rootVisible := formatLogOtherJSON(stored, logOtherVisibilityRoot)
	assert.Contains(t, rootVisible, "cline-pass/deepseek-v4.1-flash")
}

// TestLegacyUpstreamFieldsAreStrippedFromUserView covers rows written before the
// administrator-scope split: they carry the upstream name in the public top level,
// so the projection itself has to remove it rather than relying on the writer.
func TestLegacyUpstreamFieldsAreStrippedFromUserView(t *testing.T) {
	legacy := `{"request_path":"/v1/chat/completions","is_model_mapped":true,` +
		`"upstream_model_name":"cline-pass/deepseek-v4.1-flash",` +
		`"response_model":{"requested_model":"deepseek-v4.1-flash",` +
		`"upstream_model":"cline-pass/deepseek-v4.1-flash",` +
		`"returned_model":"deepseek/deepseek-v4.1-flash"}}`

	userVisible := formatLogOtherJSON(legacy, logOtherVisibilityUser)
	require.JSONEq(t, `{"request_path":"/v1/chat/completions","is_model_mapped":true}`, userVisible)
	assert.NotContains(t, userVisible, "cline-pass")

	// Administrators and root keep reading the historical row as written.
	for name, visibility := range map[string]logOtherVisibility{
		"admin": logOtherVisibilityAdmin,
		"root":  logOtherVisibilityRoot,
	} {
		assert.Contains(t, formatLogOtherJSON(legacy, visibility), "cline-pass", name)
	}

	// A writer must not be able to recreate the old shape.
	assert.False(t, NewLogOther().SetPublic("upstream_model_name", "cline-pass/x"))
	assert.False(t, NewLogOther().SetPublic("response_model", map[string]any{}))
}
