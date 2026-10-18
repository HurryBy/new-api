package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// Cline Pass 渠道的「上游探测与钉选校验」。
//
// 实测（2026-10，api.cline.bot）两条管道的钉选能力并不一致：
//
//   - 普通 OpenRouter 风格模型（如 z-ai/glm-5.2）：providerOptions.gateway.only
//     能精确钉住上游，假名字会直接 500，是唯一可靠的钉选手段。
//   - 订阅模型（cline-pass/*）：网关用「系统凭据」自行规划执行顺序，
//     provider.only / provider.order / providerOptions.gateway.only 全部被静默忽略，
//     响应里 provider_metadata.gateway.routing.finalProvider 恒为同一个上游。
//
// 因此这里只做「如实观测」：把网关真正选了谁、有哪些候选渠道报出来，
// 不假装能钉住钉不住的模型。前端据此提示用户，而不是给一个假的成功。

// ClineUpstreamStatus 是单个上游渠道在一次探测里的结论。
type ClineUpstreamStatus string

const (
	// ClineUpstreamOK 表示钉住该渠道后请求成功到达模型。
	ClineUpstreamOK ClineUpstreamStatus = "ok"
	// ClineUpstreamLimited 表示渠道有效但当前被限流。
	ClineUpstreamLimited ClineUpstreamStatus = "limited"
	// ClineUpstreamBad 表示该渠道不可用/不可钉（模型未覆盖该渠道等）。
	ClineUpstreamBad ClineUpstreamStatus = "bad"
	// ClineUpstreamAuth 表示账号密钥本身有问题，与渠道无关。
	ClineUpstreamAuth ClineUpstreamStatus = "auth"
	// ClineUpstreamUnknown 表示失败原因无法归类。
	ClineUpstreamUnknown ClineUpstreamStatus = "unknown"
)

// ClineModelProbe 是单个模型的探测结果。
type ClineModelProbe struct {
	Model string `json:"model"`
	// Reachable 表示这次探测请求本身成功拿到了模型回复。
	Reachable bool `json:"reachable"`
	// Pipeline 是网关实际走的管道：gateway（规划器）或 direct（OpenRouter 直连）。
	Pipeline string `json:"pipeline,omitempty"`
	// Pinnable 表示实测该模型的上游能否被客户端钉住。
	Pinnable bool `json:"pinnable"`
	// PinnedHeld 表示「下发了钉选」且「网关确实按钉选执行」；只有它为 true
	// 才说明钉住了。Pinnable 为 false 时恒为 false。
	PinnedHeld bool `json:"pinned_held"`
	// RequestedPin 是这次校验下发的渠道名（为空表示没下发）。
	RequestedPin string `json:"requested_pin,omitempty"`
	// FinalProvider 是**未下发钉选**时网关自报的真实上游，代表它的默认选择。
	// 它与 RequestedPin 来自两次不同的请求，两者不相等是正常的；判断是否钉住
	// 要看 Upstreams 里各次尝试的 FinalProvider。
	FinalProvider string `json:"final_provider,omitempty"`
	// CanonicalSlug 是网关内部的真实模型 ID。
	CanonicalSlug string `json:"canonical_slug,omitempty"`
	// Available 是网关报出的候选上游清单（含主选与可回退项）。
	Available []string `json:"available,omitempty"`
	// Upstreams 是逐个渠道实测得到的可用性。
	Upstreams []ClineUpstreamResult `json:"upstreams,omitempty"`
	// Error 是探测失败时的归类原因。
	Error string `json:"error,omitempty"`
	// Detail 是给排查用的原始错误文本（已截断，不含密钥）。
	Detail string `json:"detail,omitempty"`
	// ElapsedMs 是主探测请求的耗时。
	ElapsedMs int64 `json:"elapsed_ms"`
}

// ClineUpstreamResult 是「钉到某个具体渠道」这一次尝试的结论。
type ClineUpstreamResult struct {
	Upstream string              `json:"upstream"`
	Status   ClineUpstreamStatus `json:"status"`
	// FinalProvider 仅在该次请求成功时给出，用于校验是否真的落在期望渠道上。
	FinalProvider string `json:"final_provider,omitempty"`
	Note          string `json:"note,omitempty"`
}

// ClineProbeResult 是一次渠道探测的完整结果。
type ClineProbeResult struct {
	SampledAt int64             `json:"sampled_at"`
	Models    []ClineModelProbe `json:"models"`
}

// maxClineProbeResponseBytes 限制读取的上游响应体积，避免异常响应吃满内存。
const maxClineProbeResponseBytes = 4 << 20

// clineProbeMaxTokens 必须留足预算：推理型模型在 max_tokens 过小时会返回
// "empty response content"（实测 max_tokens=16 必现），那是预算问题不是渠道问题。
const clineProbeMaxTokens = 2048

// clineProbeModelConcurrency 限制并发探测的模型数，避免触发上游限流。
const clineProbeModelConcurrency = 2

// ProbeClineChannel 探测一个 Cline 渠道：对每个模型各发一次普通请求记录真实
// 走的管道与上游，再逐个候选渠道下发钉选来验证「是否真的钉住了」。
//
// 只读取该渠道已保存的密钥与地址，绝不会把密钥写进结果。
func ProbeClineChannel(ctx context.Context, channel *model.Channel, models []string) (*ClineProbeResult, error) {
	if channel == nil {
		return nil, errors.New("channel is required")
	}
	if channel.Type != constant.ChannelTypeCline {
		return nil, errors.New("This operation is only supported for Cline channels")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(channel.GetBaseURL()), "/")
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(constant.ChannelTypeCline)
	}
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Host == "" || parsedURL.User != nil ||
		(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, errors.New("Invalid Cline gateway address")
	}

	// 这是一次只读探测，不需要跟踪「本次请求已试过的密钥」，传 nil 即可。
	key, _, keyErr := channel.GetNextEnabledKey(nil)
	if keyErr != nil {
		return nil, errors.New("No enabled channel key")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("No enabled channel key")
	}

	settings := channel.GetSetting()
	client, err := GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		return nil, errors.New("Invalid channel proxy")
	}
	// 不修改共享 client，也不把渠道凭据带进重定向。
	probeClient := *client
	probeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	if len(models) == 0 {
		models = parseClineProbeModels(channel.Models)
	}
	if len(models) == 0 {
		return nil, errors.New("No models to probe on this channel")
	}

	probe := &clineProber{
		client:  &probeClient,
		baseURL: baseURL,
		key:     key,
	}
	result := &ClineProbeResult{
		SampledAt: time.Now().UnixMilli(),
		Models:    make([]ClineModelProbe, len(models)),
	}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, clineProbeModelConcurrency)
	for i, name := range models {
		wg.Go(func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			result.Models[i] = probe.probeModel(ctx, name)
		})
	}
	wg.Wait()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil, context.Canceled
	}
	return result, nil
}

// ClineSubscriptionModels 是 Cline Pass 订阅模型的已知清单。
//
// /v1/models 只返回通用 OpenRouter 目录，其中**不含** cline-pass/* 订阅模型，
// 因此无法从上游枚举出来。这份清单让管理端可以直接勾选，而不必手写模型名。
// 网关随时可能增减模型，所以它只是建议清单：探测会如实报告每个模型是否真的可用，
// 前端也允许手工填写清单之外的模型名。
var ClineSubscriptionModels = []string{
	"cline-pass/glm-5.3-flash",
	"cline-pass/glm-5.3",
	"cline-pass/glm-5.2",
	"cline-pass/kimi-k3",
	"cline-pass/kimi-k2.7-code",
	"cline-pass/kimi-k2.6",
	"cline-pass/deepseek-v4.1-flash",
	"cline-pass/deepseek-v4-flash",
	"cline-pass/deepseek-v4-pro",
	"cline-pass/qwen3.8-max",
	"cline-pass/qwen3.7-max",
	"cline-pass/qwen3.7-plus",
	"cline-pass/minimax-m3",
	"cline-pass/mimo-v2.5-pro",
	"cline-pass/mimo-v2.5",
}

// parseClineProbeModels 从渠道模型列表里取出要探测的模型名，保持原有顺序并去重。
func parseClineProbeModels(raw string) []string {
	var models []string
	seen := make(map[string]struct{})
	for name := range strings.SplitSeq(raw, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
	}
	return models
}

type clineProber struct {
	client  *http.Client
	baseURL string
	key     string
}

// clineProbeReply 是网关响应里我们关心的部分。
type clineProbeReply struct {
	// pipeline 是 gateway 或 direct；空表示既没有 routing 也没有顶层 provider。
	pipeline      string
	finalProvider string
	canonicalSlug string
	available     []string
	// errMessage 是网关返回的 error 文本（成功时为空）。
	errMessage string
}

func (p *clineProber) probeModel(ctx context.Context, name string) ClineModelProbe {
	probe := ClineModelProbe{Model: name}

	startedAt := time.Now()
	reply, err := p.chat(ctx, name, nil)
	probe.ElapsedMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		status, detail := classifyClineProbeError(err)
		probe.Error = string(status)
		probe.Detail = detail
		return probe
	}
	probe.Pipeline = reply.pipeline
	probe.FinalProvider = reply.finalProvider
	probe.CanonicalSlug = reply.canonicalSlug
	probe.Available = reply.available
	probe.Reachable = reply.errMessage == ""

	// 模型没通就别再拿它去逐个渠道试，否则只会把失败原因重复一遍。
	if !probe.Reachable {
		status, detail := classifyClineProbeError(errors.New(reply.errMessage))
		probe.Error = string(status)
		probe.Detail = detail
		return probe
	}
	// 只有规划器管道认 gateway.only；直连管道实测不给客户端钉选能力。
	probe.Pinnable = reply.pipeline == "gateway"
	if !probe.Pinnable || len(reply.available) == 0 {
		return probe
	}

	// 逐个候选渠道下发钉选，看网关是否真的照做。
	candidates := pickClinePinCandidates(reply)
	if len(candidates) == 0 {
		return probe
	}
	probe.Upstreams = make([]ClineUpstreamResult, len(candidates))
	var wg sync.WaitGroup
	for i, upstream := range candidates {
		wg.Go(func() {
			probe.Upstreams[i] = p.verifyPin(ctx, name, upstream)
		})
	}
	wg.Wait()

	// 「钉住了」的定义：至少一个渠道被下发了钉选，且网关自报的 finalProvider
	// 等于被钉的渠道。订阅模型恒返回同一个上游，因此这里会是 false。
	for _, attempt := range probe.Upstreams {
		if attempt.Status == ClineUpstreamOK && attempt.FinalProvider == attempt.Upstream {
			probe.PinnedHeld = true
			probe.RequestedPin = attempt.Upstream
			break
		}
	}
	return probe
}

// pickClinePinCandidates 选出值得实测钉选的渠道。
//
// 关键：必须排除网关「本来就会选中」的那个上游（routing.finalProvider）。
// 否则把一个请求钉到它自己会落到的地方，即使网关完全忽略钉选也会显示成功，
// 那就等于给用户一个假的「已钉住」。这里只保留「非默认」的候选。
func pickClinePinCandidates(reply clineProbeReply) []string {
	candidates := make([]string, 0, len(reply.available))
	for _, name := range reply.available {
		name = strings.TrimSpace(name)
		if name == "" || name == reply.finalProvider {
			continue
		}
		if slices.Contains(candidates, name) {
			continue
		}
		candidates = append(candidates, name)
	}
	// 逐个试全部候选可能很慢且容易触发限流；两个「非默认」候选足以判定。
	const maxCandidates = 2
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}
	return candidates
}

// verifyPin 钉到指定渠道发一次请求，并如实报告网关最终用了谁。
func (p *clineProber) verifyPin(ctx context.Context, name, upstream string) ClineUpstreamResult {
	result := ClineUpstreamResult{Upstream: upstream}
	reply, err := p.chat(ctx, name, []string{upstream})
	if err != nil {
		status, _ := classifyClineProbeError(err)
		result.Status = status
		result.Note = truncateClineProbeText(err.Error())
		return result
	}
	if reply.errMessage != "" {
		status, _ := classifyClineProbeError(errors.New(reply.errMessage))
		result.Status = status
		result.Note = truncateClineProbeText(reply.errMessage)
		return result
	}
	result.Status = ClineUpstreamOK
	result.FinalProvider = reply.finalProvider
	return result
}

// chat 向网关发一次 chat completions 请求。
//
// pin 非空时只下发 providerOptions.gateway.only —— 实测这是唯一被网关尊重的
// 钉选字段（顶层的 provider.only / provider.order 对任何模型都不生效）。
func (p *clineProber) chat(ctx context.Context, model string, pin []string) (clineProbeReply, error) {
	body := map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with the single word OK"}},
		"max_tokens": clineProbeMaxTokens,
	}
	if len(pin) > 0 {
		body["providerOptions"] = map[string]any{
			"gateway": map[string]any{"only": pin},
		}
	}
	payload, err := common.Marshal(body)
	if err != nil {
		return clineProbeReply{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		return clineProbeReply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.key)
	resp, err := p.client.Do(req)
	if err != nil {
		return clineProbeReply{}, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxClineProbeResponseBytes+1))
	if err != nil {
		return clineProbeReply{}, err
	}
	if len(raw) > maxClineProbeResponseBytes {
		return clineProbeReply{}, errors.New("upstream response too large")
	}
	return parseClineProbeReply(resp.StatusCode, raw)
}

// parseClineProbeReply 解析网关响应，提取管道、真实上游与候选清单。
//
// 网关会把非流式响应包一层 {"data":{...}}，因此先解包再取 choices。
func parseClineProbeReply(statusCode int, raw []byte) (clineProbeReply, error) {
	var envelope struct {
		Error any `json:"error"`
		// 网关的包装层把正文放在 data 里；这里保留原始 JSON 字节，
		// 不能用 []byte（encoding/json 会把 []byte 当 base64 解码）。
		Data json.RawMessage `json:"data"`
	}
	if err := common.Unmarshal(raw, &envelope); err != nil {
		// 非 JSON 响应：把它当作上游错误文本，避免 panic。
		return clineProbeReply{errMessage: truncateClineProbeText(string(raw))}, nil
	}
	// 网关的包装层：错误在顶层 error 字段里，正文在 data 里。
	if envelope.Error != nil {
		return clineProbeReply{errMessage: truncateClineProbeText(stringifyClineProbeError(envelope.Error))}, nil
	}
	if statusCode != http.StatusOK {
		return clineProbeReply{errMessage: truncateClineProbeText(string(raw))}, nil
	}

	payload := raw
	if len(envelope.Data) > 0 && envelope.Data[0] == '{' {
		payload = envelope.Data
	}
	var completion dto.OpenAITextResponse
	if err := common.Unmarshal(payload, &completion); err != nil {
		return clineProbeReply{errMessage: truncateClineProbeText(string(raw))}, nil
	}
	if len(completion.Choices) == 0 {
		return clineProbeReply{errMessage: "upstream returned no choices"}, nil
	}

	reply := clineProbeReply{}
	// 管道判定依据路由元数据出现的位置：
	//   planner 管道 → choices[0].message.provider_metadata.gateway.routing
	//   direct 管道 → 顶层 provider + 真实 model
	if routing := extractClineGatewayRouting(payload); routing != nil {
		reply.pipeline = "gateway"
		reply.finalProvider = routing.finalProvider
		reply.canonicalSlug = routing.canonicalSlug
		reply.available = routing.available
		return reply, nil
	}
	var direct struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := common.Unmarshal(payload, &direct); err == nil && direct.Provider != "" {
		reply.pipeline = "direct"
		reply.finalProvider = strings.ToLower(strings.ReplaceAll(direct.Provider, " ", "-"))
		if strings.Contains(direct.Model, "/") {
			reply.canonicalSlug = direct.Model
		}
	}
	return reply, nil
}

// clineGatewayRouting 是网关自报的路由决策。
type clineGatewayRouting struct {
	finalProvider string
	canonicalSlug string
	available     []string
}

// extractClineGatewayRouting 从响应里取出 provider_metadata.gateway.routing。
//
// 该对象可能挂在 choices[0].message 上，也可能挂在响应顶层，两种都认。
func extractClineGatewayRouting(payload []byte) *clineGatewayRouting {
	var probe struct {
		ProviderMetadata *clineProviderMetadata `json:"provider_metadata"`
		Choices          []struct {
			Message struct {
				ProviderMetadata *clineProviderMetadata `json:"provider_metadata"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(payload, &probe); err != nil {
		return nil
	}
	meta := probe.ProviderMetadata
	if len(probe.Choices) > 0 && probe.Choices[0].Message.ProviderMetadata != nil {
		meta = probe.Choices[0].Message.ProviderMetadata
	}
	if meta == nil || meta.Gateway == nil || meta.Gateway.Routing == nil {
		return nil
	}
	routing := meta.Gateway.Routing
	if routing.FinalProvider == "" {
		return nil
	}
	// 主选在前，其后是网关自己声明的可回退渠道。
	available := make([]string, 0, len(routing.FallbacksAvailable)+1)
	available = append(available, routing.FinalProvider)
	for _, name := range routing.FallbacksAvailable {
		if name = strings.TrimSpace(name); name != "" && !slices.Contains(available, name) {
			available = append(available, name)
		}
	}
	return &clineGatewayRouting{
		finalProvider: routing.FinalProvider,
		canonicalSlug: routing.CanonicalSlug,
		available:     available,
	}
}

type clineProviderMetadata struct {
	Gateway *struct {
		Routing *struct {
			FinalProvider      string   `json:"finalProvider"`
			CanonicalSlug      string   `json:"canonicalSlug"`
			FallbacksAvailable []string `json:"fallbacksAvailable"`
		} `json:"routing"`
	} `json:"gateway"`
}

// classifyClineProbeError 把上游错误归类成渠道可用性。
func classifyClineProbeError(err error) (ClineUpstreamStatus, string) {
	if err == nil {
		return ClineUpstreamUnknown, ""
	}
	message := strings.ToLower(err.Error())
	detail := truncateClineProbeText(err.Error())
	switch {
	case strings.Contains(message, "empty response content"):
		// 请求已到达模型，只是推理耗尽 token 导致内容为空——渠道本身可用。
		return ClineUpstreamOK, detail
	case strings.Contains(message, "429"), strings.Contains(message, "rate limit"),
		strings.Contains(message, "rate-limit"), strings.Contains(message, "temporarily rate"):
		return ClineUpstreamLimited, detail
	case strings.Contains(message, "invalid_request"), strings.Contains(message, "model not found"),
		strings.Contains(message, "no allowed providers"), strings.Contains(message, "no available providers"),
		strings.Contains(message, "not found"), strings.Contains(message, "unsupported"),
		strings.Contains(message, "failed to invoke model"):
		return ClineUpstreamBad, detail
	case strings.Contains(message, "unauthorized"), strings.Contains(message, "re-authenticate"),
		strings.Contains(message, "401"), strings.Contains(message, "invalid api key"):
		return ClineUpstreamAuth, detail
	}
	return ClineUpstreamUnknown, detail
}

// stringifyClineProbeError 把网关的 error 字段（字符串或对象）压成文本。
func stringifyClineProbeError(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := common.Marshal(value)
	if err != nil {
		return "upstream error"
	}
	return string(encoded)
}

// truncateClineProbeText 截断用于展示的错误文本，避免把整段响应塞进响应体。
func truncateClineProbeText(text string) string {
	const maxLength = 240
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "…"
}
