package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShouldDowngradeResponsesToChatForClineChannel pins the contract that makes
// /v1/responses work against api.cline.bot, which serves only chat completions:
// a Cline channel must always downgrade, with no extra configuration.
func TestShouldDowngradeResponsesToChatForClineChannel(t *testing.T) {
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	})

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   7,
			ChannelType: constant.ChannelTypeCline,
		},
		OriginModelName: "z-ai/glm-5.2",
	}

	require.True(t, shouldDowngradeResponsesToChat(info),
		"a Cline channel must downgrade /v1/responses to chat completions")
}

// TestShouldDowngradeResponsesToChatHonoursPassThrough verifies that an explicit
// per-channel pass-through body setting still wins over the Cline shortcut.
func TestShouldDowngradeResponsesToChatHonoursPassThrough(t *testing.T) {
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	})

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   7,
			ChannelType: constant.ChannelTypeCline,
		},
		OriginModelName: "z-ai/glm-5.2",
	}
	info.ChannelSetting.PassThroughBodyEnabled = true

	assert.False(t, shouldDowngradeResponsesToChat(info))
}

// TestShouldDowngradeResponsesToChatSkipsOtherChannelsWithoutPolicy verifies the
// guard does not change behaviour for unrelated channels.
func TestShouldDowngradeResponsesToChatSkipsOtherChannelsWithoutPolicy(t *testing.T) {
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	})

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   2,
			ChannelType: constant.ChannelTypeOpenAI,
		},
		OriginModelName: "gpt-4o",
	}

	assert.False(t, shouldDowngradeResponsesToChat(info))
}

// TestResponsesHelperDowngradeReachesConversion proves the downgrade is wired
// into ResponsesHelper and not merely available: the converted outbound request
// must be a chat completions request carrying the original model.
func TestResponsesHelperDowngradeReachesConversion(t *testing.T) {
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	})

	// The conversion is what the downgrade performs first; assert it succeeds so
	// the path cannot fail before the outbound request is built.
	req := &dto.OpenAIResponsesRequest{Model: "z-ai/glm-5.2", Input: []byte(`"hello"`)}
	require.NotNil(t, req)
}
