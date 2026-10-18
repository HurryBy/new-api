package service

import (
	"regexp"
	"sync"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// Chat→Responses upgrade policy is host routing logic (it decides *whether*
// to convert, reading host settings), so it lives here, not in relayconvert.

var chatResponsesRegexCache sync.Map // map[string]*regexp.Regexp

func matchAnyModelPattern(patterns []string, model string) bool {
	if len(patterns) == 0 || model == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		re, ok := chatResponsesRegexCache.Load(pattern)
		if !ok {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				// Treat invalid patterns as non-matching to avoid breaking runtime traffic.
				continue
			}
			re = compiled
			chatResponsesRegexCache.Store(pattern, re)
		}
		if re.(*regexp.Regexp).MatchString(model) {
			return true
		}
	}
	return false
}

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	if !policy.IsChannelEnabled(channelID, channelType) {
		return false
	}
	return matchAnyModelPattern(policy.ModelPatterns, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return ShouldChatCompletionsUseResponsesPolicy(
		model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy,
		channelID,
		channelType,
		model,
	)
}

func ShouldResponsesUseChatCompletionsPolicy(policy model_setting.ResponsesToChatCompletionsPolicy, channelID int, channelType int, model string) bool {
	if !policy.IsChannelEnabled(channelID, channelType) {
		return false
	}
	// 空模式列表表示该渠道下所有模型都降级（见策略注释）。
	if len(policy.ModelPatterns) == 0 {
		return true
	}
	return matchAnyModelPattern(policy.ModelPatterns, model)
}

// ShouldResponsesUseChatCompletionsGlobal 判断一次 /v1/responses 请求是否要降级成
// chat completions 发给上游。
//
// Cline Pass 渠道无条件返回 true：api.cline.bot 只提供 /v1/chat/completions 与
// /v1/models，把 /v1/responses 原样透传只会拿到 404 Not Found。其余渠道按
// ResponsesToChatCompletionsPolicy 决定，因此新增 Cline 渠道不需要额外配置。
func ShouldResponsesUseChatCompletionsGlobal(channelID int, channelType int, model string) bool {
	if channelType == constant.ChannelTypeCline {
		return true
	}
	return ShouldResponsesUseChatCompletionsPolicy(
		model_setting.GetGlobalSettings().ResponsesToChatCompletionsPolicy,
		channelID,
		channelType,
		model,
	)
}
