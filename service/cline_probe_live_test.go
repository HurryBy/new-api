package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestProbeClineChannelLive is opt-in: it spends a few tokens against the real
// Cline gateway, so it only runs when CLINE_PROBE_LIVE_KEY is set.
//
//	$env:CLINE_PROBE_LIVE_KEY="sk_..."; go test ./service/ -run TestProbeClineChannelLive -v
func TestProbeClineChannelLive(t *testing.T) {
	key := os.Getenv("CLINE_PROBE_LIVE_KEY")
	if key == "" {
		t.Skip("CLINE_PROBE_LIVE_KEY not set")
	}

	baseURL := "https://api.cline.bot/api"
	channel := &model.Channel{
		Id:      7,
		Type:    64,
		Key:     key,
		BaseURL: &baseURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// One model behind system credentials and one plain OpenRouter model, so the
	// test observes both pinning behaviours the gateway exhibits.
	result, err := ProbeClineChannel(ctx, channel, []string{
		"cline-pass/deepseek-v4.1-flash",
		"z-ai/glm-5.2",
	})
	require.NoError(t, err)
	require.Len(t, result.Models, 2)

	for _, probe := range result.Models {
		t.Logf("model=%s reachable=%v pipeline=%s pinnable=%v pinnedHeld=%v final=%s available=%v err=%s",
			probe.Model, probe.Reachable, probe.Pipeline, probe.Pinnable,
			probe.PinnedHeld, probe.FinalProvider, probe.Available, probe.Error)
		for _, attempt := range probe.Upstreams {
			t.Logf("    upstream=%-12s status=%-8s final=%-12s note=%s",
				attempt.Upstream, attempt.Status, attempt.FinalProvider, attempt.Note)
		}
		require.True(t, probe.Reachable, "model %s should be reachable", probe.Model)
	}
}
