package oairesponses

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归：客户端续接会话时会回传上一轮的 output 项，其中包含 reasoning。
// reasoning 项带的是 summary（没有 content），早期实现会把它转成一条
// 内容为空的 user 消息，上游随即 400「user message must have content」，
// 表现为「一个来回就结束」。这里锁定它必须被跳过。
func TestResponsesRequestToChatCompletionsRequestSkipsReasoningItems(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(context.Background(), &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{"role": "user", "content": "What is the weather in Paris?"},
			{
				"type":    "reasoning",
				"id":      "rs_1",
				"summary": []map[string]any{{"type": "summary_text", "text": "Need the weather tool."}},
			},
			{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "get_weather",
				"arguments": map[string]any{"city": "Paris"},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  "18C sunny",
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 3)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "What is the weather in Paris?", got.Messages[0].StringContent())
	assert.Equal(t, "assistant", got.Messages[1].Role)
	require.Len(t, got.Messages[1].ParseToolCalls(), 1)
	assert.Equal(t, "tool", got.Messages[2].Role)
	assert.Equal(t, "call_1", got.Messages[2].ToolCallId)

	// 关键断言：没有任何内容为空的消息被送出去
	for i, message := range got.Messages {
		if message.Role == "tool" || len(message.ToolCalls) > 0 {
			continue
		}
		assert.Falsef(t, chatMessageContentEmpty(message.Content), "message %d should not be empty: %+v", i, message)
	}
}

// reasoning 项即使带 content 数组（部分客户端会带 reasoning_text）也不应产生消息。
func TestResponsesRequestToChatCompletionsRequestSkipsReasoningWithContent(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(context.Background(), &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{"role": "user", "content": "hi"},
			{
				"type":    "reasoning",
				"id":      "rs_2",
				"content": []map[string]any{{"type": "reasoning_text", "text": "thinking"}},
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "hi", got.Messages[0].StringContent())
}

// 显式传入的空 user/system 消息也应被丢弃（上游对空内容直接 400）。
func TestResponsesRequestToChatCompletionsRequestDropsEmptyMessages(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(context.Background(), &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{"role": "user", "content": "hi"},
			{"role": "user", "content": ""},
			{"role": "user", "content": "   "},
			{"role": "assistant", "content": "ok"},
			{"role": "assistant", "content": ""},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 2)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "hi", got.Messages[0].StringContent())
	assert.Equal(t, "assistant", got.Messages[1].Role)
	assert.Equal(t, "ok", got.Messages[1].StringContent())
}

// 工具调用与工具结果的配对不能被「丢空消息」破坏：
// assistant 只有 tool_calls、content 为空，tool 结果为空字符串，都必须保留。
func TestResponsesRequestToChatCompletionsRequestKeepsToolPairing(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(context.Background(), &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{"role": "user", "content": "call the tool"},
			{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "lookup",
				"arguments": map[string]any{},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  "",
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 3)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "assistant", got.Messages[1].Role)
	require.Len(t, got.Messages[1].ParseToolCalls(), 1)
	assert.Equal(t, "tool", got.Messages[2].Role)
	assert.Equal(t, "call_1", got.Messages[2].ToolCallId)
}

// 其它不支持的条目类型：既没有 role 也没有 content 时跳过，
// 带 role 或 content 的正常消息仍然保留。
func TestResponsesRequestToChatCompletionsRequestSkipsUnknownItemsWithoutRoleOrContent(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(context.Background(), &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{"type": "web_search_call", "id": "ws_1", "status": "completed"},
			{"type": "item_reference", "id": "msg_1"},
			{"role": "user", "content": "hello"},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "hello", got.Messages[0].StringContent())
}