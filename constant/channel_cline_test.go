package constant

import "testing"

// 固化 Cline 渠道类型的注册不变量。
//
// 这里特意覆盖 ChannelTypeDummy：Go 常量块里不带表达式的行会复用上一行的值，
// 所以 Dummy 等于最后一个真实类型，并被当作**闭区间**上界使用
// （见 controller/model.go 的 `i <= constant.ChannelTypeDummy`）。
// 如果新增类型后把它挪到 Dummy 之后，Dummy 会留在旧值上，
// 遍历就会漏掉新类型；如果 Cline 沿用旧的 61，则会与 ChannelTypeTaskPlugin 撞号。
func TestClineChannelTypeRegistration(t *testing.T) {
	// Cline 取 64：上游已把 61/62/63 分配给 TaskPlugin/VLLM/SGLang。
	if ChannelTypeCline != 64 {
		t.Fatalf("ChannelTypeCline 期望 64，实际 %d", ChannelTypeCline)
	}
	if ChannelTypeCline == ChannelTypeTaskPlugin {
		t.Fatal("ChannelTypeCline 不能与上游的 ChannelTypeTaskPlugin 撞号")
	}
	// Dummy 必须仍然等于最后一个真实类型，遍历上界才覆盖得到 Cline。
	if ChannelTypeDummy != ChannelTypeCline {
		t.Fatalf("ChannelTypeDummy 必须等于最后一个类型 %d（实际 %d）",
			ChannelTypeCline, ChannelTypeDummy)
	}
	if got := GetChannelTypeName(ChannelTypeCline); got != "Cline" {
		t.Fatalf("渠道类型名称期望 Cline，实际 %q", got)
	}
	if _, ok := ChannelTypeNames[ChannelTypeCline]; !ok {
		t.Fatal("ChannelTypeNames 缺少 Cline 条目")
	}
	if len(ChannelBaseURLs) <= ChannelTypeCline {
		t.Fatalf("ChannelBaseURLs 长度 %d 不足以为类型 %d 提供默认值",
			len(ChannelBaseURLs), ChannelTypeCline)
	}
	if got := ChannelBaseURLs[ChannelTypeCline]; got != "https://api.cline.bot/api" {
		t.Fatalf("Cline 默认 BaseURL 期望 https://api.cline.bot/api（不含版本段，"+
			"请求路径会自带 /v1），实际 %q", got)
	}
}
