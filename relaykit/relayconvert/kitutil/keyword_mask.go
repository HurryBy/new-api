package kitutil

import (
	"strings"
	"sync/atomic"
)

// KeywordMasker is an optional redaction hook for user-facing text.
//
// relaykit must stay independently buildable, so it cannot import the host's
// settings. The host installs its compiled keyword matcher here at startup; the
// zero value (no installer) leaves every redaction path a no-op, which is the
// behaviour a standalone relaykit consumer observes.
type KeywordMasker interface {
	// Mask replaces every configured keyword in text and returns the result.
	Mask(text string) string
}

// keywordMasker holds the installed masker. A nil value disables keyword masking.
var keywordMasker atomic.Pointer[KeywordMasker]

// SetKeywordMasker installs the host-provided masker. Passing nil removes it.
func SetKeywordMasker(masker KeywordMasker) {
	if masker == nil {
		keywordMasker.Store(nil)
		return
	}
	keywordMasker.Store(&masker)
}

// MaskKeywords applies the installed keyword masker to text. It is safe to call
// when no masker is installed.
func MaskKeywords(text string) string {
	if text == "" {
		return text
	}
	installed := keywordMasker.Load()
	if installed == nil || *installed == nil {
		return text
	}
	return (*installed).Mask(text)
}

// MaskKeywordsWithExtra applies the installed masker, then redacts caller-supplied
// literal names that must never reach the client, such as the selected channel's
// upstream model name. Extra names are matched literally and never persisted.
func MaskKeywordsWithExtra(text string, extra ...string) string {
	masked := MaskKeywords(text)
	if len(extra) == 0 {
		return masked
	}
	for _, name := range extra {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		masked = strings.ReplaceAll(masked, name, "***")
	}
	return masked
}
