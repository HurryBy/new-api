package kitutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// uppercaseMasker is a stand-in for the host's compiled matcher.
type uppercaseMasker struct{}

func (uppercaseMasker) Mask(text string) string {
	return strings.ReplaceAll(text, "acme-internal", "[masked]")
}

func TestMaskKeywordsIsNoOpWithoutInstalledMasker(t *testing.T) {
	SetKeywordMasker(nil)
	require.Equal(t, "acme-internal failed", MaskKeywords("acme-internal failed"))
}

func TestMaskKeywordsUsesInstalledMasker(t *testing.T) {
	SetKeywordMasker(uppercaseMasker{})
	t.Cleanup(func() { SetKeywordMasker(nil) })

	require.Equal(t, "[masked] failed", MaskKeywords("acme-internal failed"))
}

func TestSetKeywordMaskerNilRemovesMasker(t *testing.T) {
	SetKeywordMasker(uppercaseMasker{})
	require.Equal(t, "[masked]", MaskKeywords("acme-internal"))

	SetKeywordMasker(nil)
	require.Equal(t, "acme-internal", MaskKeywords("acme-internal"))
}

func TestMaskKeywordsWithExtraRedactsLiteralNames(t *testing.T) {
	SetKeywordMasker(uppercaseMasker{})
	t.Cleanup(func() { SetKeywordMasker(nil) })

	// The configured keyword and the caller-supplied upstream name are both
	// redacted; a blank extra is ignored rather than blanking the whole string.
	require.Equal(t,
		"[masked] model *** reached ***",
		MaskKeywordsWithExtra(
			"acme-internal model cline-pass/deepseek reached cline-pass/deepseek",
			"",
			"  cline-pass/deepseek  ",
		),
	)
}

func TestMaskKeywordsWithExtraWithoutMaskerStillRedactsExtras(t *testing.T) {
	SetKeywordMasker(nil)

	require.Equal(t,
		"upstream *** answered",
		MaskKeywordsWithExtra("upstream vendor-name answered", "vendor-name"),
	)
}

// TestMaskSensitiveInfoThenKeywordsKeepsBothRedactions proves the relay error path
// composes address masking and keyword masking rather than replacing one with the
// other, which is how a leaked provider name would otherwise survive.
func TestMaskSensitiveInfoThenKeywordsKeepsBothRedactions(t *testing.T) {
	SetKeywordMasker(uppercaseMasker{})
	t.Cleanup(func() { SetKeywordMasker(nil) })

	masked := MaskKeywords(
		MaskSensitiveInfo(
			`acme-internal failed: Post "https://api.upstream.example.com/v1": dial tcp 10.0.0.1:443: refused`,
			"https://api.upstream.example.com",
		),
	)

	require.Equal(t,
		`[masked] failed: Post "https://***.com/***": dial tcp ***.***.***.***:443: refused`,
		masked,
	)
}
