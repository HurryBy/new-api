package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func newClineURLInfo(baseURL, requestPath string) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeCline,
		ChannelBaseUrl: baseURL,
	}
	info.RequestURLPath = requestPath
	info.UpstreamModelName = "cline-pass/glm-5.2"
	return info
}

// Cline 渠道的 URL 拼接：new-api 的请求路径自带 /v1，而官方文档给的 base 也带 /api/v1，
// 两种填法都必须收敛到同一个正确 URL，不能出现 /v1/v1。
func TestClineGetRequestURL(t *testing.T) {
	ad := &Adaptor{}
	cases := []struct {
		name        string
		base        string
		requestPath string
		want        string
	}{
		{
			name:        "默认 base 不含版本段（empty 时的回退值）",
			base:        constant.ChannelBaseURLs[constant.ChannelTypeCline],
			requestPath: "/v1/chat/completions",
			want:        "https://api.cline.bot/api/v1/chat/completions",
		},
		{
			name:        "用户按官方文档填 /api/v1",
			base:        "https://api.cline.bot/api/v1",
			requestPath: "/v1/chat/completions",
			want:        "https://api.cline.bot/api/v1/chat/completions",
		},
		{
			name:        "带结尾斜杠",
			base:        "https://api.cline.bot/api/v1/",
			requestPath: "/v1/chat/completions",
			want:        "https://api.cline.bot/api/v1/chat/completions",
		},
		{
			name:        "Claude 格式路径",
			base:        "https://api.cline.bot/api/v1",
			requestPath: "/v1/messages",
			want:        "https://api.cline.bot/api/v1/messages",
		},
		{
			name:        "模型列表",
			base:        constant.ChannelBaseURLs[constant.ChannelTypeCline],
			requestPath: "/v1/models",
			want:        "https://api.cline.bot/api/v1/models",
		},
		{
			name:        "默认 base + Claude 格式路径",
			base:        constant.ChannelBaseURLs[constant.ChannelTypeCline],
			requestPath: "/v1/messages",
			want:        "https://api.cline.bot/api/v1/messages",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ad.GetRequestURL(newClineURLInfo(tc.base, tc.requestPath))
			if err != nil {
				t.Fatalf("GetRequestURL 不应报错: %v", err)
			}
			if got != tc.want {
				t.Fatalf("URL 拼接错误\n  期望: %s\n  实际: %s", tc.want, got)
			}
		})
	}
}

// 官方文档里给的就是 .../api/v1/chat/completions，这里直接钉住这个最终形态，
// 避免以后有人"顺手"把默认 base 改回 .../api/v1 而没人发现。
func TestClineDefaultBaseYieldsDocumentedChatURL(t *testing.T) {
	ad := &Adaptor{}
	got, err := ad.GetRequestURL(newClineURLInfo(
		constant.ChannelBaseURLs[constant.ChannelTypeCline],
		"/v1/chat/completions",
	))
	if err != nil {
		t.Fatalf("GetRequestURL 不应报错: %v", err)
	}
	const want = "https://api.cline.bot/api/v1/chat/completions"
	if got != want {
		t.Fatalf("默认配置必须拼出官方文档地址 %s，实际 %s", want, got)
	}
}
