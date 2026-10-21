package common

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// Cline Pass 渠道的「钉上游渠道」实现。
//
// 语义移植自 dsh-cline-pass 的 lib/protocol.js（injectPrefs / buildAttempts）：
// 网关有两条管道，对 pin 的写法不同——
//   - direct（OpenRouter）：顶层 provider.only / provider.order / provider.sort
//   - planner（Vercel AI Gateway）：只有 providerOptions.gateway.* 会被识别
// new-api 在转发前无法预知这通请求会落到哪条管道，因此两种写法同时下发，
// 各自忽略对方不认识的字段（与 dsh-cline-pass 处理未知管道的行为一致）。

// clineOpenRouterSort 把三种排序指标名映射为 OpenRouter 的线上取值。
var clineOpenRouterSort = map[string]string{
	"cost": "price",
	"ttft": "latency",
	"tps":  "throughput",
}

// normalizeClineSort 归一化排序配置：空串与 "none" 都表示不排序。
func normalizeClineSort(value string) string {
	sort := strings.TrimSpace(value)
	if sort == "" || sort == "none" {
		return ""
	}
	return sort
}

// filterClineUpstreams 去重并剔除被排除的供应商，保持配置顺序。
func filterClineUpstreams(upstreams, exclude []string) []string {
	if len(upstreams) == 0 {
		return nil
	}
	excluded := make(map[string]struct{}, len(exclude))
	for _, name := range exclude {
		if name = strings.TrimSpace(name); name != "" {
			excluded[name] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(upstreams))
	result := make([]string, 0, len(upstreams))
	for _, name := range upstreams {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		if _, skip := excluded[name]; skip {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

// lookupClinePin 按上游模型名取出钉选配置，回退到用户请求的原始模型名。
func lookupClinePin(info *RelayInfo) (dto.ClineUpstreamPin, bool) {
	if info == nil || len(info.ChannelSetting.ClineUpstreamPins) == 0 {
		return dto.ClineUpstreamPin{}, false
	}
	pins := info.ChannelSetting.ClineUpstreamPins
	for _, key := range []string{info.UpstreamModelName, info.OriginModelName} {
		if key == "" {
			continue
		}
		if pin, ok := pins[key]; ok {
			return pin, true
		}
	}
	return dto.ClineUpstreamPin{}, false
}

// ApplyClineUpstreamPin 依据渠道上按模型配置的钉选，重写出站请求体。
// 非 Cline 渠道、未配置该模型、或配置为空时原样返回。
func ApplyClineUpstreamPin(jsonData []byte, info *RelayInfo) ([]byte, error) {
	if info == nil || info.ChannelType != constant.ChannelTypeCline {
		return jsonData, nil
	}
	pin, ok := lookupClinePin(info)
	if !ok {
		return jsonData, nil
	}
	wanted := filterClineUpstreams(pin.Upstreams, pin.Exclude)
	sort := normalizeClineSort(pin.Sort)
	if len(wanted) == 0 && sort == "" {
		return jsonData, nil
	}

	body := make(map[string]any)
	if err := json.Unmarshal(jsonData, &body); err != nil {
		// 非 JSON 请求体：不阻断转发，原样放行
		return jsonData, nil
	}

	// strict：只钉第一个；preferred：按顺序下发，交给网关在首个 token 前故障转移
	strict := !strings.EqualFold(strings.TrimSpace(pin.PinMode), "preferred")
	head := ""
	var rest []string
	if len(wanted) > 0 {
		head = wanted[0]
		if !strict {
			rest = wanted[1:]
		}
	}

	injectClineGatewayPin(body, head, rest, wanted, strict, sort)
	injectClineProviderPin(body, head, rest, wanted, strict, sort)

	out, err := json.Marshal(body)
	if err != nil {
		return jsonData, nil
	}
	return out, nil
}

// ApplyClineUpstreamPinReader 仅在有有效钉选时读取并重写 JSON 请求体。
// 无需钉选时保留原 reader；重写后返回可再次读取的 reader。
func ApplyClineUpstreamPinReader(body io.Reader, info *RelayInfo) io.Reader {
	if body == nil || info == nil || info.ChannelType != constant.ChannelTypeCline {
		return body
	}
	// Most requests have no matching pin. Preserve the original storage and its
	// replay metadata instead of copying a potentially disk-backed agent context.
	pin, ok := lookupClinePin(info)
	if !ok || len(filterClineUpstreams(pin.Upstreams, pin.Exclude)) == 0 && normalizeClineSort(pin.Sort) == "" {
		return body
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		return bytes.NewReader(raw)
	}
	rewritten, err := ApplyClineUpstreamPin(raw, info)
	if err != nil || bytes.Equal(rewritten, raw) {
		return bytes.NewReader(raw)
	}
	return bytes.NewReader(rewritten)
}

// injectClineGatewayPin 写入 planner 管道认的 providerOptions.gateway.*
func injectClineGatewayPin(body map[string]any, head string, rest, allowed []string, strict bool, sort string) {
	gateway := make(map[string]any, 3)
	switch {
	case head != "" && strict:
		gateway["only"] = []string{head}
	case head != "":
		gateway["order"] = append([]string{head}, rest...)
		gateway["only"] = allowed
	case len(allowed) > 0:
		gateway["only"] = allowed
	}
	if sort != "" {
		gateway["sort"] = sort
	}
	if len(gateway) == 0 {
		return
	}
	options, ok := body["providerOptions"].(map[string]any)
	if !ok || options == nil {
		options = make(map[string]any, 1)
	}
	merged := make(map[string]any, len(gateway)+1)
	if existing, ok := options["gateway"].(map[string]any); ok {
		for k, v := range existing {
			merged[k] = v
		}
	}
	for k, v := range gateway {
		merged[k] = v
	}
	options["gateway"] = merged
	body["providerOptions"] = options
}

// injectClineProviderPin 写入 direct(OpenRouter) 管道认的顶层 provider.*
func injectClineProviderPin(body map[string]any, head string, rest, allowed []string, strict bool, sort string) {
	provider := make(map[string]any, 3)
	if existing, ok := body["provider"].(map[string]any); ok {
		for k, v := range existing {
			provider[k] = v
		}
	}
	switch {
	case head != "" && strict:
		provider["only"] = []string{head}
	case head != "":
		provider["order"] = append([]string{head}, rest...)
		provider["only"] = allowed
	case len(allowed) > 0:
		provider["only"] = allowed
	}
	if sort != "" {
		if mapped, ok := clineOpenRouterSort[sort]; ok {
			provider["sort"] = mapped
		} else {
			provider["sort"] = sort
		}
	}
	if len(provider) == 0 {
		return
	}
	body["provider"] = provider
}
