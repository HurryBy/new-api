package constant

type MultiKeyMode string

const (
	MultiKeyModeRandom  MultiKeyMode = "random"  // 随机
	MultiKeyModePolling MultiKeyMode = "polling" // 轮询
	MultiKeyModeSticky  MultiKeyMode = "sticky"  // 粘性：始终使用第一个可用的 key，直到它不可用才切换到下一个
)
