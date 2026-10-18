package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// clineProbeRequest 是探测请求体；models 为空时使用渠道已配置的模型列表。
type clineProbeRequest struct {
	Models []string `json:"models"`
}

// ProbeClineChannelUpstreams 探测一个 Cline Pass 渠道的可用上游，并如实校验
// 「钉选到底有没有生效」。它不是测试按钮的替代品：这里返回的是网关自报的真实
// 路由结果，因此能明确区分「钉住了」和「网关忽略了钉选」。
func ProbeClineChannelUpstreams(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("Invalid channel id"))
		return
	}
	channel, err := model.GetChannelById(id, true)
	if err != nil || channel == nil {
		common.ApiErrorStatus(c, http.StatusNotFound, errors.New("Channel not found"))
		return
	}
	if channel.Type != constant.ChannelTypeCline {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("This operation is only supported for Cline channels"))
		return
	}

	var req clineProbeRequest
	// 允许空请求体：不传就探测渠道上配置的全部模型。
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	models := make([]string, 0, len(req.Models))
	for _, name := range req.Models {
		if name = strings.TrimSpace(name); name != "" {
			models = append(models, name)
		}
	}

	result, err := service.ProbeClineChannel(c.Request.Context(), channel, models)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// GetClineSubscriptionModels 返回 Cline Pass 订阅模型的建议清单。
//
// api.cline.bot 的 /v1/models 不含 cline-pass/* 订阅模型，管理端无法从上游枚举，
// 因此由后端给出这份清单供直接勾选；它只是建议，探测结果才是模型是否可用的依据。
func GetClineSubscriptionModels(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, service.ClineSubscriptionModels)
}
