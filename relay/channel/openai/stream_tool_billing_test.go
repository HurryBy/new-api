package openai

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chatFlushRecorder struct {
	*httptest.ResponseRecorder
	flushed chan string
}

func (w *chatFlushRecorder) Flush() {
	w.ResponseRecorder.Flush()
	select {
	case w.flushed <- w.Body.String():
	default:
	}
}

func TestChatStreamForwardsFirstDeltaBeforeNextUpstreamEvent(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	for _, tc := range []struct {
		name, delta, want string
	}{
		{"content", `{"content":"hello"}`, "hello"},
		{"role", `{"role":"assistant"}`, "assistant"},
		{"reasoning", `{"reasoning_content":"thinking"}`, "thinking"},
		{"legacy reasoning", `{"reasoning_content":"","reasoning":"thinking"}`, "thinking"},
		{"tool", `{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"weather","arguments":"{}"}}]}`, "weather"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			recorder := &chatFlushRecorder{httptest.NewRecorder(), make(chan string, 8)}
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{
				OriginModelName: "requested", RelayFormat: types.RelayFormatOpenAI,
				RelayMode: relayconstant.RelayModeChatCompletions, IsStream: true,
				ShouldIncludeUsage: true, DisablePing: true,
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "upstream"},
			}
			var usage *dto.Usage
			var apiErr *types.NewAPIError
			done := make(chan struct{})
			go func() {
				usage, apiErr = OaiStreamHandler(c, info, &http.Response{Body: reader, Header: make(http.Header)})
				close(done)
			}()
			_, err := fmt.Fprintf(writer, "data: {\"id\":\"chat_1\",\"model\":\"upstream\",\"choices\":[{\"index\":0,\"delta\":%s}]}\n\n", tc.delta)
			require.NoError(t, err)
			select {
			case chunk := <-recorder.flushed:
				assert.Contains(t, chunk, tc.want)
				assert.Contains(t, chunk, `"model":"requested"`)
			case <-time.After(2 * time.Second):
				t.Error("first delta was held while the upstream was idle")
			}
			_, err = fmt.Fprint(writer, "data: {\"id\":\"chat_1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"id\":\"chat_1\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\ndata: [DONE]\n\n")
			require.NoError(t, err)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("stream did not finish")
			}
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, 10, usage.PromptTokens)
			assert.Equal(t, 2, usage.CompletionTokens)
			assert.Equal(t, 1, strings.Count(recorder.Body.String(), tc.want))
			assert.Equal(t, 1, strings.Count(recorder.Body.String(), "[DONE]"))
			assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
		})
	}
}

func BenchmarkChatStreamRelay(b *testing.B) {
	oldTimeout := constant.StreamingTimeout
	b.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	constant.StreamingTimeout = 30
	gin.SetMode(gin.ReleaseMode)
	chunk := "data: {\"id\":\"chat_1\",\"model\":\"upstream\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n"
	stream := strings.Repeat(chunk, 256) + "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":256,\"total_tokens\":266}}\n\ndata: [DONE]\n\n"
	b.ReportAllocs()
	for b.Loop() {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		info := &relaycommon.RelayInfo{OriginModelName: "requested", RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeChatCompletions, IsStream: true, ShouldIncludeUsage: true, DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "upstream"}}
		usage, err := OaiStreamHandler(c, info, &http.Response{Body: io.NopCloser(strings.NewReader(stream)), Header: make(http.Header)})
		require.Nil(b, err)
		require.Equal(b, 256, usage.CompletionTokens)
	}
}

func TestClineExplicitThinkingDisabled(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		channel          int
	}{
		{"disabled", `{"thinking":{"type":"disabled"}}`, "none", constant.ChannelTypeCline},
		{"default", `{}`, "", constant.ChannelTypeCline},
		{"enabled", `{"thinking":{"type":"enabled"}}`, "", constant.ChannelTypeCline},
		{"explicit effort wins", `{"thinking":{"type":"disabled"},"reasoning_effort":"high"}`, "high", constant.ChannelTypeCline},
		{"nested reasoning wins", `{"thinking":{"type":"disabled"},"reasoning":{"effort":"high"}}`, "high", constant.ChannelTypeCline},
		{"other channel", `{"thinking":{"type":"disabled"}}`, "", constant.ChannelTypeOpenAI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var request dto.GeneralOpenAIRequest
			require.NoError(t, common.UnmarshalJsonStr(tc.body, &request))
			request.Model = "cline-pass/deepseek-v4.1-flash-1017"
			info := &relaycommon.RelayInfo{OriginModelName: request.Model, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: tc.channel, UpstreamModelName: request.Model}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			_, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &request)
			require.NoError(t, err)
			assert.Equal(t, tc.want, request.ReasoningEffort)
		})
	}
}

func TestResponseModelRewritePreservesOtherJSONValues(t *testing.T) {
	body := []byte(`{"model":"upstream","id":9007199254740993,"choices":[{"delta":{"content":"upstream"}}],"optional":null}`)
	want := `{"model":"requested","id":9007199254740993,"choices":[{"delta":{"content":"upstream"}}],"optional":null}`
	assert.JSONEq(t, want, string(replaceResponseModel(body, "requested")))
	assert.Contains(t, string(replaceResponseModel(body, "requested")), `9007199254740993`)
}

func TestObserveStreamChoicesDedupesFunctionCallNames(t *testing.T) {
	info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}
	seen := make(map[string]struct{})
	var names []string

	chunks := []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"c2","type":"function","function":{"name":"get_time","arguments":""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{}"}}]}}]}`,
	}
	for _, chunk := range chunks {
		observeStreamChoices(info, chunk, seen, &names)
	}

	require.Len(t, names, 2)
	assert.Equal(t, []string{"get_weather", "get_time"}, names)
	assert.Empty(t, info.StreamStatus.ResponseOutcome(), "tool call deltas carry no finish reason")

	observeStreamChoices(info, `{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`, seen, &names)
	assert.Equal(t, "completed", info.StreamStatus.ResponseOutcome())
	assert.False(t, info.PerformanceBusinessRejection)

	filtered := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}
	observeStreamChoices(filtered, `{"choices":[{"index":0,"delta":{},"finish_reason":"content_filter"}]}`, map[string]struct{}{}, &names)
	assert.True(t, filtered.PerformanceBusinessRejection)
}
