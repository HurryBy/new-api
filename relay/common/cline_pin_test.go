package common

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

func newClineRelayInfo(model string, pins map[string]dto.ClineUpstreamPin) *RelayInfo {
	info := &RelayInfo{}
	info.ChannelMeta = &ChannelMeta{ChannelType: constant.ChannelTypeCline}
	info.UpstreamModelName = model
	info.OriginModelName = model
	info.ChannelSetting = dto.ChannelSettings{ClineUpstreamPins: pins}
	return info
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("结果不是合法 JSON: %v (%s)", err, string(raw))
	}
	return body
}

func strSlice(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("期望数组，实际 %#v", value)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("数组元素不是字符串: %#v", item)
		}
		out = append(out, s)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 非 Cline 渠道必须完全不动请求体
func TestApplyClineUpstreamPinSkipsNonClineChannel(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {Upstreams: []string{"alibaba"}},
	})
	info.ChannelType = constant.ChannelTypeOpenAI

	input := []byte(`{"model":"cline-pass/glm-5.2","messages":[]}`)
	out, err := ApplyClineUpstreamPin(input, info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if string(out) != string(input) {
		t.Fatalf("非 Cline 渠道不应改写请求体, got %s", out)
	}
}

// 该模型没有配置钉选时必须原样放行
func TestApplyClineUpstreamPinNoConfigForModel(t *testing.T) {
	info := newClineRelayInfo("cline-pass/unknown", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {Upstreams: []string{"alibaba"}},
	})
	input := []byte(`{"model":"cline-pass/unknown"}`)
	out, err := ApplyClineUpstreamPin(input, info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if string(out) != string(input) {
		t.Fatalf("无该模型配置时不应改写, got %s", out)
	}
}

// 空配置（只有 exclude 没有 upstreams）无法编译成 allow-list，必须原样放行
func TestApplyClineUpstreamPinExcludeOnlyIsNoop(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {Exclude: []string{"baseten"}},
	})
	input := []byte(`{"model":"cline-pass/glm-5.2"}`)
	out, err := ApplyClineUpstreamPin(input, info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if string(out) != string(input) {
		t.Fatalf("仅 exclude 时应原样放行, got %s", out)
	}
}

// strict：只钉第一个，两种管道写法都要下发
func TestApplyClineUpstreamPinStrict(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {
			Upstreams: []string{"alibaba", "baseten"},
			PinMode:   "strict",
		},
	})
	out, err := ApplyClineUpstreamPin([]byte(`{"model":"cline-pass/glm-5.2","messages":[]}`), info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	body := decodeBody(t, out)

	provider, ok := body["provider"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 provider: %s", out)
	}
	if got := strSlice(t, provider["only"]); !equalStrings(got, []string{"alibaba"}) {
		t.Fatalf("provider.only 期望 [alibaba], 实际 %v", got)
	}
	if _, exists := provider["order"]; exists {
		t.Fatalf("strict 不应写入 provider.order: %s", out)
	}

	options, ok := body["providerOptions"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 providerOptions: %s", out)
	}
	gateway, ok := options["gateway"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 providerOptions.gateway: %s", out)
	}
	if got := strSlice(t, gateway["only"]); !equalStrings(got, []string{"alibaba"}) {
		t.Fatalf("gateway.only 期望 [alibaba], 实际 %v", got)
	}

	// 原有字段必须保留
	if body["model"] != "cline-pass/glm-5.2" {
		t.Fatalf("原有 model 字段被破坏: %s", out)
	}
	if _, exists := body["messages"]; !exists {
		t.Fatalf("原有 messages 字段被破坏: %s", out)
	}
}

// preferred：按顺序下发 order + only（收窄到配置集合），两种管道都要
func TestApplyClineUpstreamPinPreferred(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {
			Upstreams: []string{"alibaba", "baseten", "fireworks"},
			PinMode:   "preferred",
			Sort:      "cost",
		},
	})
	out, err := ApplyClineUpstreamPin([]byte(`{"model":"cline-pass/glm-5.2"}`), info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	body := decodeBody(t, out)

	provider := body["provider"].(map[string]any)
	if got := strSlice(t, provider["order"]); !equalStrings(got, []string{"alibaba", "baseten", "fireworks"}) {
		t.Fatalf("provider.order 顺序不对: %v", got)
	}
	if got := strSlice(t, provider["only"]); !equalStrings(got, []string{"alibaba", "baseten", "fireworks"}) {
		t.Fatalf("provider.only 应收窄到配置集合: %v", got)
	}
	// OpenRouter 的排序取值是 price/latency/throughput
	if provider["sort"] != "price" {
		t.Fatalf("provider.sort 期望 cost->price, 实际 %v", provider["sort"])
	}

	gateway := body["providerOptions"].(map[string]any)["gateway"].(map[string]any)
	if gateway["sort"] != "cost" {
		t.Fatalf("gateway.sort 应保持原值 cost, 实际 %v", gateway["sort"])
	}
	if got := strSlice(t, gateway["order"]); !equalStrings(got, []string{"alibaba", "baseten", "fireworks"}) {
		t.Fatalf("gateway.order 顺序不对: %v", got)
	}
}

// exclude 从配置列表里剔除，并保持顺序
func TestApplyClineUpstreamPinExcludeFiltersUpstreams(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {
			Upstreams: []string{"alibaba", "baseten", "baseten", "fireworks"},
			Exclude:   []string{"baseten"},
			PinMode:   "preferred",
		},
	})
	out, err := ApplyClineUpstreamPin([]byte(`{"model":"cline-pass/glm-5.2"}`), info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	provider := decodeBody(t, out)["provider"].(map[string]any)
	if got := strSlice(t, provider["order"]); !equalStrings(got, []string{"alibaba", "fireworks"}) {
		t.Fatalf("exclude 与去重后期望 [alibaba fireworks], 实际 %v", got)
	}
}

// sort=none 与空串都表示不排序
func TestApplyClineUpstreamPinSortNoneIsDropped(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {
			Upstreams: []string{"alibaba"},
			PinMode:   "strict",
			Sort:      "none",
		},
	})
	out, err := ApplyClineUpstreamPin([]byte(`{"model":"cline-pass/glm-5.2"}`), info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	provider := decodeBody(t, out)["provider"].(map[string]any)
	if _, exists := provider["sort"]; exists {
		t.Fatalf("sort=none 不应写入 sort: %s", out)
	}
}

// 已存在的 provider/providerOptions 字段要合并而不是被覆盖
func TestApplyClineUpstreamPinMergesExistingFields(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {Upstreams: []string{"alibaba"}, PinMode: "strict"},
	})
	input := []byte(`{"model":"cline-pass/glm-5.2","provider":{"allow_fallbacks":false},"providerOptions":{"gateway":{"zdr":true}}}`)
	out, err := ApplyClineUpstreamPin(input, info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	body := decodeBody(t, out)

	provider := body["provider"].(map[string]any)
	if provider["allow_fallbacks"] != false {
		t.Fatalf("原有 provider.allow_fallbacks 丢失: %s", out)
	}
	if got := strSlice(t, provider["only"]); !equalStrings(got, []string{"alibaba"}) {
		t.Fatalf("provider.only 未写入: %v", got)
	}

	gateway := body["providerOptions"].(map[string]any)["gateway"].(map[string]any)
	if gateway["zdr"] != true {
		t.Fatalf("原有 providerOptions.gateway.zdr 丢失: %s", out)
	}
	if got := strSlice(t, gateway["only"]); !equalStrings(got, []string{"alibaba"}) {
		t.Fatalf("gateway.only 未写入: %v", got)
	}
}

// 用原始模型名也能命中（做模型映射时 UpstreamModelName 与 OriginModelName 不同）
func TestApplyClineUpstreamPinFallsBackToOriginModel(t *testing.T) {
	info := newClineRelayInfo("", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {Upstreams: []string{"alibaba"}, PinMode: "strict"},
	})
	info.UpstreamModelName = "mapped-upstream-name"
	info.OriginModelName = "cline-pass/glm-5.2"

	out, err := ApplyClineUpstreamPin([]byte(`{"model":"mapped-upstream-name"}`), info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	provider := decodeBody(t, out)["provider"].(map[string]any)
	if got := strSlice(t, provider["only"]); !equalStrings(got, []string{"alibaba"}) {
		t.Fatalf("应回退到 OriginModelName 命中配置, 实际 %v", got)
	}
}

// 非法 JSON 请求体不能阻断转发
func TestApplyClineUpstreamPinInvalidJSONIsPassedThrough(t *testing.T) {
	info := newClineRelayInfo("cline-pass/glm-5.2", map[string]dto.ClineUpstreamPin{
		"cline-pass/glm-5.2": {Upstreams: []string{"alibaba"}, PinMode: "strict"},
	})
	input := []byte("not-json")
	out, err := ApplyClineUpstreamPin(input, info)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if string(out) != string(input) {
		t.Fatalf("非法 JSON 应原样放行, got %s", out)
	}
}
