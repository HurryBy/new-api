package constant

import "testing"

// 固化 Cline 渠道类型的注册不变量。
//
// 这里特意覆盖 ChannelTypeDummy：Go 常量块里不带表达式的行会复用上一行的值，
// 如果新增类型后忘了给 Dummy 显式赋值，它会和新增的类型撞成同一个数，
// 而 Dummy 被当作「类型总数 / 遍历上界」使用（见 controller/model.go）。
func TestClineChannelTypeRegistration(t *testing.T) {
	if ChannelTypeCline != 61 {
		t.Fatalf("ChannelTypeCline 期望 61，实际 %d", ChannelTypeCline)
	}
	if ChannelTypeDummy != ChannelTypeCline+1 {
		t.Fatalf("ChannelTypeDummy 必须是最后一个类型 + 1（期望 %d，实际 %d）——"+
			"常量块里不带表达式的行会复用上一行，请显式赋值",
			ChannelTypeCline+1, ChannelTypeDummy)
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
