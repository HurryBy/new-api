package model_setting

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type ChatCompletionsToResponsesPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

func (p ChatCompletionsToResponsesPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

// ResponsesToChatCompletionsPolicy 是上面 ChatCompletionsToResponsesPolicy 的反向策略：
// 客户端用 /v1/responses 请求，而目标上游只提供 /v1/chat/completions 时，
// 把请求降级成 chat completions 转发，再把上游的响应（含 SSE 流）转换回 Responses 格式。
//
// 必须开启的场景（否则 /v1/responses 会被原样透传，上游返回 404）：
//   - Cline Pass（api.cline.bot）只暴露 /v1/chat/completions 与 /v1/models；
//   - 自建/中转网关只实现了 OpenAI 的 chat completions（例如 wb2api）。
//
// 与正向策略的区别：ModelPatterns 留空表示「该渠道下所有模型都转换」，
// 因为「上游不支持 responses」这件事通常与具体模型无关。
type ResponsesToChatCompletionsPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

func (p ResponsesToChatCompletionsPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

type GlobalSettings struct {
	PassThroughRequestEnabled        bool                             `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist           []string                         `json:"thinking_model_blacklist"`
	ChatCompletionsToResponsesPolicy ChatCompletionsToResponsesPolicy `json:"chat_completions_to_responses_policy"`
	ResponsesToChatCompletionsPolicy ResponsesToChatCompletionsPolicy `json:"responses_to_chat_completions_policy"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled: false,
	ThinkingModelBlacklist: []string{
		"moonshotai/kimi-k2-thinking",
		"kimi-k2-thinking",
	},
	ChatCompletionsToResponsesPolicy: ChatCompletionsToResponsesPolicy{
		Enabled:     false,
		AllChannels: true,
	},
	ResponsesToChatCompletionsPolicy: ResponsesToChatCompletionsPolicy{
		Enabled: false,
	},
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings() *GlobalSettings {
	return &globalSettings
}

// ShouldPreserveThinkingSuffix 判断模型是否配置为保留 thinking/-nothinking/-low/-high/-medium 后缀
func ShouldPreserveThinkingSuffix(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	for _, entry := range globalSettings.ThinkingModelBlacklist {
		if strings.TrimSpace(entry) == target {
			return true
		}
	}
	return false
}
