package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

// Cline 网关对一部分回答会套一层 {"data": {...}} 信封。
// 移植自 dsh-cline-pass 的 unwrapEnvelope：解析前必须先剥掉信封，
// 否则顶层读不到 choices，响应会退化成空内容。
//
// 判定条件与上游项目保持一致：只有 data.choices 存在时才认为是信封，
// 其它带 data 字段的正常应答原样放行。
func UnwrapClineEnvelope(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil || len(envelope.Data) == 0 {
		return body
	}
	var inner struct {
		Choices json.RawMessage `json:"choices"`
	}
	if err := common.Unmarshal(envelope.Data, &inner); err != nil || len(inner.Choices) == 0 {
		return body
	}
	return envelope.Data
}

// UnwrapClineEnvelopeResponse 读入整个响应体、剥掉 Cline 信封后写回 resp，
// 供非流式路径在交给通用响应处理器之前调用。
func UnwrapClineEnvelopeResponse(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		return closeErr
	}
	unwrapped := UnwrapClineEnvelope(raw)
	resp.Body = io.NopCloser(bytes.NewReader(unwrapped))
	resp.ContentLength = int64(len(unwrapped))
	resp.Header.Set("Content-Length", strconv.Itoa(len(unwrapped)))
	return nil
}
