package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newClineProbeTestChannel builds a Cline channel pointing at a stub server.
func newClineProbeTestChannel(baseURL string) *model.Channel {
	return &model.Channel{
		Id:      7,
		Type:    64,
		Key:     "sk_test_key",
		Models:  "cline-pass/model-a,cline-pass/model-b",
		BaseURL: &baseURL,
	}
}

// gatewayReply is the shape the Cline gateway returns for the planner pipeline:
// routing metadata lives on choices[0].message.provider_metadata.gateway.routing.
func gatewayReply(finalProvider string, fallbacks []string) map[string]any {
	return map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content": "OK",
					"provider_metadata": map[string]any{
						"gateway": map[string]any{
							"routing": map[string]any{
								"finalProvider":      finalProvider,
								"canonicalSlug":      "deepseek/deepseek-v4.1-flash",
								"fallbacksAvailable": fallbacks,
							},
						},
					},
				},
				"finish_reason": "stop",
			},
		},
	}
}

// TestProbeClineChannelReportsHeldPin verifies that a pin is reported as held only
// when the gateway names the pinned upstream as the provider it actually used.
func TestProbeClineChannelReportsHeldPin(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk_test_key", r.Header.Get("Authorization"))

		var body struct {
			ProviderOptions struct {
				Gateway struct {
					Only []string `json:"only"`
				} `json:"gateway"`
			} `json:"providerOptions"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		// Honour the pin the way a working gateway would: the reported provider
		// is exactly the one that was requested.
		if pinned := body.ProviderOptions.Gateway.Only; len(pinned) > 0 {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": gatewayReply(pinned[0], []string{"fireworks", "baseten", "deepseek"}),
			}))
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": gatewayReply("deepseek", []string{"fireworks", "baseten"}),
		}))
	}))
	defer upstream.Close()

	result, err := ProbeClineChannel(context.Background(), newClineProbeTestChannel(upstream.URL), nil)
	require.NoError(t, err)
	require.Len(t, result.Models, 2)

	for _, probe := range result.Models {
		assert.True(t, probe.Reachable)
		assert.Equal(t, "gateway", probe.Pipeline)
		assert.True(t, probe.Pinnable)
		// The unpinned baseline is where the gateway routes by itself.
		assert.Equal(t, "deepseek", probe.FinalProvider)
		// The stub honours every pin, so the pin must be reported as held.
		assert.True(t, probe.PinnedHeld, "model %s should report a held pin", probe.Model)
		// Only non-default upstreams are worth pinning: the baseline already
		// lands on deepseek, so a pin there would prove nothing.
		assert.NotEmpty(t, probe.RequestedPin)
		assert.NotEqual(t, "deepseek", probe.RequestedPin)
		// The pinned attempt must have landed on the pinned provider.
		for _, attempt := range probe.Upstreams {
			require.Equal(t, ClineUpstreamOK, attempt.Status)
			assert.Equal(t, attempt.Upstream, attempt.FinalProvider)
		}
	}
}

// TestProbeClineChannelReportsIgnoredPin verifies the subscription-model case the
// live gateway exhibits: the pin is accepted but silently ignored, so it must NOT
// be reported as held.
func TestProbeClineChannelReportsIgnoredPin(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always answer with the same provider, ignoring any requested pin.
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": gatewayReply("deepseek", []string{"fireworks", "baseten"}),
		}))
	}))
	defer upstream.Close()

	result, err := ProbeClineChannel(context.Background(), newClineProbeTestChannel(upstream.URL), nil)
	require.NoError(t, err)
	require.Len(t, result.Models, 2)

	for _, probe := range result.Models {
		assert.True(t, probe.Reachable)
		assert.True(t, probe.Pinnable, "the gateway pipeline is nominally pinnable")
		assert.False(t, probe.PinnedHeld, "an ignored pin must never be reported as held")
		assert.Equal(t, "deepseek", probe.FinalProvider)
		require.NotEmpty(t, probe.Upstreams)
		for _, attempt := range probe.Upstreams {
			assert.Equal(t, ClineUpstreamOK, attempt.Status)
			// Each attempt landed on the pinned name's successor, not the pin.
			assert.Equal(t, "deepseek", attempt.FinalProvider)
		}
	}
}

// TestProbeClineChannelClassifiesUpstreamFailures verifies that a pinned provider
// which cannot serve the model is classified as bad rather than ok.
func TestProbeClineChannelClassifiesUpstreamFailures(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProviderOptions struct {
				Gateway struct {
					Only []string `json:"only"`
				} `json:"gateway"`
			} `json:"providerOptions"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if len(body.ProviderOptions.Gateway.Only) > 0 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"failed to invoke model: no available providers"}`))
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": gatewayReply("deepseek", []string{"fireworks"}),
		}))
	}))
	defer upstream.Close()

	result, err := ProbeClineChannel(context.Background(), newClineProbeTestChannel(upstream.URL), []string{"cline-pass/model-a"})
	require.NoError(t, err)
	require.Len(t, result.Models, 1)

	probe := result.Models[0]
	assert.True(t, probe.Reachable)
	assert.False(t, probe.PinnedHeld)
	require.NotEmpty(t, probe.Upstreams)
	for _, attempt := range probe.Upstreams {
		assert.Equal(t, ClineUpstreamBad, attempt.Status, "upstream %s note=%s", attempt.Upstream, attempt.Note)
	}
}

// TestProbeClineChannelRejectsNonClineChannel pins the guard that keeps this probe
// from being pointed at an arbitrary upstream.
func TestProbeClineChannelRejectsNonClineChannel(t *testing.T) {
	baseURL := "https://example.test/api"
	channel := &model.Channel{Type: 1, Key: "sk", BaseURL: &baseURL}

	_, err := ProbeClineChannel(context.Background(), channel, nil)
	require.Error(t, err)
}

// TestParseClineProbeModels pins ordering, trimming, and de-duplication.
func TestParseClineProbeModels(t *testing.T) {
	assert.Equal(t,
		[]string{"a", "b", "c"},
		parseClineProbeModels(" a , b ,, a , c "),
	)
	assert.Nil(t, parseClineProbeModels("  "))
}

// TestClineProbeResultSnapshotKeepsCandidates verifies the persisted snapshot is
// usable on its own: the drawer renders pins from the snapshot, so it must keep
// the gateway's default upstream, every measured candidate, and the untested
// candidates that were truncated from measurement.
func TestClineProbeResultSnapshotKeepsCandidates(t *testing.T) {
	result := &ClineProbeResult{
		SampledAt: 1_700_000_000_000,
		Models: []ClineModelProbe{
			{
				Model:         "cline-pass/model-a",
				Reachable:     true,
				Pipeline:      "gateway",
				Pinnable:      true,
				PinnedHeld:    true,
				RequestedPin:  "fireworks",
				FinalProvider: "deepseek",
				CanonicalSlug: "deepseek/deepseek-v4.1-flash",
				Available:     []string{"fireworks", "baseten", "openai"},
				Upstreams: []ClineUpstreamResult{
					{Upstream: "fireworks", Status: ClineUpstreamOK, FinalProvider: "fireworks"},
					{Upstream: "baseten", Status: ClineUpstreamBad, Note: "no capacity"},
				},
			},
		},
	}

	snapshot := result.Snapshot()

	// Unix seconds, not milliseconds, so the UI can format it directly.
	assert.Equal(t, int64(1_700_000_000), snapshot.SampledAt)
	require.Len(t, snapshot.Models, 1)

	model := snapshot.Models[0]
	assert.Equal(t, "cline-pass/model-a", model.Model)
	assert.Equal(t, "deepseek", model.DefaultProvider)
	assert.Equal(t, "deepseek/deepseek-v4.1-flash", model.CanonicalSlug)
	assert.True(t, model.Pinnable)

	byName := make(map[string]dto.ClineProbeUpstreamSnapshot, len(model.Upstreams))
	for _, item := range model.Upstreams {
		byName[item.Upstream] = item
	}

	// The default upstream is the baseline the UI contrasts a pin against.
	require.Contains(t, byName, "deepseek")
	assert.True(t, byName["deepseek"].Held)
	assert.Equal(t, "default", byName["deepseek"].Note)

	// A pin the gateway obeyed is marked held.
	assert.True(t, byName["fireworks"].Held)
	// A pin that failed is kept with its reason, not dropped.
	assert.Equal(t, string(ClineUpstreamBad), byName["baseten"].Status)
	assert.Equal(t, "no capacity", byName["baseten"].Note)
	// Candidates that were listed but never measured stay offered and unknown.
	assert.Equal(t, string(ClineUpstreamUnknown), byName["openai"].Status)
	assert.False(t, byName["openai"].Held)

	// Ordering puts the gateway default first so the UI can label it.
	assert.Equal(t, "deepseek", model.Upstreams[0].Upstream)
}

// TestNormalizeClineUpstreamPins covers the validation the save endpoint relies on.
func TestNormalizeClineUpstreamPins(t *testing.T) {
	t.Run("trims, de-duplicates and defaults the mode", func(t *testing.T) {
		pins, err := NormalizeClineUpstreamPins(map[string]dto.ClineUpstreamPin{
			"  cline-pass/model-a  ": {Upstreams: []string{" fireworks ", "fireworks", "", "baseten"}},
		})
		require.NoError(t, err)
		require.Contains(t, pins, "cline-pass/model-a")
		assert.Equal(t, []string{"fireworks", "baseten"}, pins["cline-pass/model-a"].Upstreams)
		// Strict is the default because it matches "pin exactly this" intent.
		assert.Equal(t, "strict", pins["cline-pass/model-a"].PinMode)
	})

	t.Run("drops excluded upstreams from the pin list", func(t *testing.T) {
		pins, err := NormalizeClineUpstreamPins(map[string]dto.ClineUpstreamPin{
			"m": {Upstreams: []string{"a", "b"}, Exclude: []string{"a"}},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"b"}, pins["m"].Upstreams)
		assert.Equal(t, []string{"a"}, pins["m"].Exclude)
	})

	t.Run("rejects invalid mode and sort", func(t *testing.T) {
		_, err := NormalizeClineUpstreamPins(map[string]dto.ClineUpstreamPin{
			"m": {Upstreams: []string{"a"}, PinMode: "sequential"},
		})
		require.Error(t, err)

		_, err = NormalizeClineUpstreamPins(map[string]dto.ClineUpstreamPin{
			"m": {Upstreams: []string{"a"}, Sort: "latency"},
		})
		require.Error(t, err)
	})

	t.Run("returns nil for empty and contentless pins", func(t *testing.T) {
		pins, err := NormalizeClineUpstreamPins(nil)
		require.NoError(t, err)
		assert.Nil(t, pins)

		pins, err = NormalizeClineUpstreamPins(map[string]dto.ClineUpstreamPin{
			"m": {},
		})
		require.NoError(t, err)
		assert.Nil(t, pins, "a pin with no fields must not be stored")
	})
}
