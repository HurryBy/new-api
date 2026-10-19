package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// clineProbeRequest 是探测请求体；models 为空时使用渠道已配置的模型列表。
type clineProbeRequest struct {
	Models []string `json:"models"`
	// Persist 为 true 时把探测结果连同钉选一起写回渠道，让下次打开界面还能看到。
	Persist *bool `json:"persist,omitempty"`
}

// ProbeClineChannelUpstreams 探测一个 Cline Pass 渠道的可用上游，并如实校验
// 「钉选到底有没有生效」。它不是测试按钮的替代品：这里返回的是网关自报的真实
// 路由结果，因此能明确区分「钉住了」和「网关忽略了钉选」。
//
// 默认把结果持久化到渠道的 cline_probe_snapshot，这样用户下次打开渠道编辑页
// 仍能看到上次探测到的候选渠道，不必每次重新探测。
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

	persist := req.Persist == nil || *req.Persist
	if persist {
		// 快照只用于展示，写失败不应让探测结果一起丢掉：前端仍能显示本次结果，
		// 只是下次打开页面时看到的是旧快照。
		if err := service.SaveClineProbeSnapshot(channel, result.Snapshot()); err != nil {
			common.SysLog(common.LogText("failed to save Cline probe snapshot: channel_id=%d, error=%v", channel.Id, err))
		}
	}
	common.ApiSuccess(c, result)
}

// GetClineProbeSnapshot 返回渠道上持久化的最近一次探测快照与当前钉选配置。
//
// 前端打开 Cline 渠道编辑页时用它恢复上次探测到的候选渠道与已保存的钉选，
// 让钉选不必依赖本次会话的探测结果。
func GetClineProbeSnapshot(c *gin.Context) {
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
	common.ApiSuccess(c, gin.H{
		"snapshot": service.ReadClineProbeSnapshot(channel),
		// 钉选可能由旧版本手工写入、从未探测过，因此与快照分开返回。
		"pins": service.ReadClineUpstreamPins(channel),
	})
}

// clinePinsRequest 是保存钉选配置的请求体。
//
// 前端把「点了哪些上游」拼成 ClineUpstreamPins 提交，后端只做校验与落库，
// 用户不再需要手写 JSON。
type clinePinsRequest struct {
	// Pins 是完整的钉选映射；提交即整体替换，传空对象表示清空全部钉选。
	Pins map[string]dto.ClineUpstreamPin `json:"pins"`
}

// UpdateClineUpstreamPins 保存 Cline 渠道的钉选配置。
//
// 只改 setting.cline_upstream_pins 一个字段，其余 setting 原样保留，
// 因此它不会覆盖用户在抽屉里同时改的其它设置。
func UpdateClineUpstreamPins(c *gin.Context) {
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

	var req clinePinsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("Invalid parameters"))
		return
	}
	pins, err := service.NormalizeClineUpstreamPins(req.Pins)
	if err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, err)
		return
	}

	if err := service.SaveClineUpstreamPins(channel, pins); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, pins)
}

// GetClineSubscriptionModels 返回 Cline Pass 订阅模型的建议清单。
//
// api.cline.bot 的 /v1/models 不含 cline-pass/* 订阅模型，管理端无法从上游枚举，
// 因此由后端给出这份清单供直接勾选；它只是建议，探测结果才是模型是否可用的依据。
func GetClineSubscriptionModels(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, service.ClineSubscriptionModels)
}
