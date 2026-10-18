// Reproduce the ResponsesHelper guard evaluation for the failing WS test case.
package relay

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// TestGuardEvaluationIsNilSafe pins that the responses->chat downgrade guard can be
// evaluated on a RelayInfo whose ChannelMeta was never populated. That is the shape a
// WebSocket-dialed native Responses channel has: the handler runs before
// InitChannelMeta fills the embedded *ChannelMeta, so the guard must not dereference
// an embedded field through a nil pointer.
func TestGuardEvaluationIsNilSafe(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponses,
		OriginModelName: "ws-billing",
	}
	// ChannelMeta stays nil on purpose.

	settings := model_setting.GetGlobalSettings()
	if settings == nil {
		t.Fatal("GetGlobalSettings returned nil")
	}

	// Same expression shape as ResponsesHelper's guard.
	_ = info.RelayMode != relayconstant.RelayModeResponsesCompact &&
		!settings.PassThroughRequestEnabled &&
		shouldDowngradeResponsesToChat(info)
}
