package redact_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSetting installs a setting for one test and restores the previous state so
// tests never depend on execution order.
func withSetting(t *testing.T, setting RedactSetting) {
	t.Helper()
	previous := redactSetting
	t.Cleanup(func() {
		redactSetting = previous
		rebuildMatcher()
	})
	redactSetting = setting
	rebuildMatcher()
}

func TestMaskErrorReplacesConfiguredKeywords(t *testing.T) {
	withSetting(t, RedactSetting{
		Enabled:           true,
		Patterns:          []string{"acme-internal"},
		Mask:              "***",
		MaskErrorMessages: true,
		MaskModelNames:    true,
	})

	require.Equal(t,
		"upstream *** rejected the request",
		MaskError("upstream acme-internal rejected the request"),
	)
}

func TestMaskErrorSkipsQuotedKeywordInRegularExpression(t *testing.T) {
	withSetting(t, RedactSetting{
		Enabled:           true,
		Patterns:          []string{`re:acme-[0-9]+`},
		Mask:              "[redacted]",
		MaskErrorMessages: true,
		MaskModelNames:    false,
	})

	require.Equal(t,
		"provider [redacted] is unavailable, [redacted] again",
		MaskError("provider acme-42 is unavailable, acme-7 again"),
	)
}

func TestMaskErrorLeavesTextWhenDisabled(t *testing.T) {
	withSetting(t, RedactSetting{
		Enabled:           false,
		Patterns:          []string{"acme-internal"},
		Mask:              "***",
		MaskErrorMessages: true,
		MaskModelNames:    true,
	})

	require.Equal(t,
		"upstream acme-internal rejected the request",
		MaskError("upstream acme-internal rejected the request"),
	)
}

func TestMaskErrorHonoursPerSurfaceSwitch(t *testing.T) {
	withSetting(t, RedactSetting{
		Enabled:           true,
		Patterns:          []string{"acme-internal"},
		Mask:              "***",
		MaskErrorMessages: false,
		MaskModelNames:    true,
	})

	// Error redaction is off, so the error text survives untouched...
	require.Equal(t,
		"upstream acme-internal failed",
		MaskError("upstream acme-internal failed"),
	)
	// ...while the model-name surface still masks.
	require.Equal(t, "***-chat", MaskModel("acme-internal-chat"))
}

func TestMaskSkipsInvalidAndDuplicatePatterns(t *testing.T) {
	withSetting(t, RedactSetting{
		Enabled:           true,
		Patterns:          []string{"dup", "dup", "re:[unclosed", "", "   ", "secret"},
		Mask:              "***",
		MaskErrorMessages: true,
	})

	// One good literal must survive a neighbouring invalid regex; otherwise a
	// single typo would silently disable the whole feature.
	require.Equal(t, "*** value", MaskError("dup value"))
	require.Equal(t, "*** value", MaskError("secret value"))
}

func TestMaskErrorWithExtraRedactsUpstreamName(t *testing.T) {
	withSetting(t, RedactSetting{
		Enabled:           true,
		Patterns:          []string{"api-key"},
		Mask:              "***",
		MaskErrorMessages: true,
	})

	require.Equal(t,
		"model *** failed for deepseek/deepseek-v4.1-flash",
		MaskErrorWithExtra(
			"model cline-pass/deepseek-v4.1-flash failed for deepseek/deepseek-v4.1-flash",
			"cline-pass/deepseek-v4.1-flash",
		),
	)
}

func TestDefaultMaskIsUsedWhenMaskIsBlank(t *testing.T) {
	withSetting(t, RedactSetting{
		Enabled:           true,
		Patterns:          []string{"acme-internal"},
		Mask:              "",
		MaskErrorMessages: true,
	})

	require.Equal(t, "*** ok", MaskError("acme-internal ok"))
}

func TestBuildMatcherReturnsNilWithoutPatterns(t *testing.T) {
	assert.Nil(t, buildMatcher(RedactSetting{Enabled: true}))
	assert.Nil(t, buildMatcher(RedactSetting{Enabled: false, Patterns: []string{"x"}}))
}
