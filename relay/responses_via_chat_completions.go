package relay

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// responsesViaChatCompletions 服务「客户端用 /v1/responses、上游只有 chat completions」
// 的渠道：先由 relayconvert 把 Responses 请求转成 chat completions 下发，
// 上游返回后再把响应（含 SSE 流）转换回 Responses 格式。
//
// 这是 relay/chat_completions_via_responses.go 的镜像实现；
// 触发条件见 service.ShouldResponsesUseChatCompletionsGlobal。
//
// 调用方（ResponsesHelper）负责计费收尾：本函数只返回 usage。
func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	result, err := service.ConvertRequestVia(c, info, request, types.RelayFormatOpenAIResponses, types.RelayFormatOpenAI)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, types.NewError(fmt.Errorf("expected OpenAI chat request, got %T", result.Value), types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	// 出站期间把 relay 语义临时切到 chat completions + /v1/chat/completions，
	// 让渠道按 chat completions 拼 URL（例如 Cline 只认 /v1/chat/completions）。
	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, chatRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	jsonData = nil
	var requestBody io.Reader = body

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	httpResp := resp.(*http.Response)
	if httpResp.StatusCode != http.StatusOK {
		newApiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return nil, newApiErr
	}

	// Cline 网关可能给非流式回答套一层 {"data": {...}} 信封；Responses 转换器
	// 按顶层 choices 解析，不剥掉信封会得到空 output。
	if !info.IsStream && info.ChannelType == constant.ChannelTypeCline {
		if unwrapErr := openaichannel.UnwrapClineEnvelopeResponse(httpResp); unwrapErr != nil {
			return nil, types.NewOpenAIError(unwrapErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
		}
	}

	var usage *dto.Usage
	var newApiErr *types.NewAPIError
	if info.IsStream {
		usage, newApiErr = openaichannel.OaiChatToResponsesStreamHandler(c, info, httpResp)
	} else {
		usage, newApiErr = openaichannel.OaiChatToResponsesHandler(c, info, httpResp)
	}
	if newApiErr != nil {
		service.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return nil, newApiErr
	}
	return usage, nil
}
