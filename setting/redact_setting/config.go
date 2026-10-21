package redact_setting

import (
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/setting/config"
)

// DefaultMask is the replacement written over every matched keyword. Stars are
// used because the redacted text is user-facing and must not read as a valid
// model or provider name.
const DefaultMask = "***"

// maxRedactPatterns bounds the configured list so a malformed setting cannot make
// every relayed error pay an unbounded regex scan.
const maxRedactPatterns = 256

// RedactSetting controls keyword redaction for user-facing error messages and
// model names. It deliberately never touches response content: only the two
// surfaces an operator can leak provider identity through are rewritten.
type RedactSetting struct {
	// Enabled turns every rule below on or off.
	Enabled bool `json:"enabled"`
	// Patterns are literal keywords or "re:"-prefixed regular expressions.
	Patterns []string `json:"patterns"`
	// Mask replaces each match.
	Mask string `json:"mask"`
	// MaskErrorMessages redacts relay error messages returned to clients.
	MaskErrorMessages bool `json:"mask_error_messages"`
	// MaskModelNames redacts model names in responses and in user-visible logs.
	MaskModelNames bool `json:"mask_model_names"`
	// UseChannelUpstreamNames automatically adds the selected channel's upstream
	// model name, so leaking it does not require listing it by hand.
	UseChannelUpstreamNames bool `json:"use_channel_upstream_names"`
}

var redactSetting = RedactSetting{
	Enabled:                 false,
	Patterns:                nil,
	Mask:                    DefaultMask,
	MaskErrorMessages:       true,
	MaskModelNames:          true,
	UseChannelUpstreamNames: true,
}

// compiled holds the active matcher so the relay path never rebuilds regexes per
// request. A nil matcher means redaction is off.
var compiled atomic.Pointer[matcher]

// matcher is an immutable keyword/regex set built from RedactSetting.Patterns.
type matcher struct {
	literals []string
	regexps  []*regexp.Regexp
	mask     string
}

func init() {
	config.GlobalConfig.Register("redact_setting", &redactSetting)
	rebuildMatcher()
}

// GetRedactSetting returns the live setting. Callers must not mutate it; use
// UpdateAndSync to change it.
func GetRedactSetting() RedactSetting {
	return redactSetting
}

// ApplySetting replaces the active setting and rebuilds the matcher. It exists for
// callers that build a setting programmatically, such as tests and configuration
// migration, rather than through the option map.
func ApplySetting(setting RedactSetting) {
	redactSetting = setting
	rebuildMatcher()
}

// relaykitMasker adapts the compiled matcher to the relaykit hook, which cannot
// import host packages. Only error-message redaction flows through it; model-name
// redaction stays on the host side because relaykit never rewrites model names.
type relaykitMasker struct{}

func (relaykitMasker) Mask(text string) string {
	return MaskError(text)
}

// installRelaykitMasker publishes the current setting to relaykit. It is called on
// every rebuild so a settings change reaches the relay path immediately.
func installRelaykitMasker() {
	if activeMatcher() == nil || !redactSetting.MaskErrorMessages {
		kitutil.SetKeywordMasker(nil)
		return
	}
	kitutil.SetKeywordMasker(relaykitMasker{})
}

// UpdateAndSync must be called after the setting is loaded from the database or
// changed through the console so the compiled matcher matches the configuration.
func UpdateAndSync() {
	rebuildMatcher()
}

// buildMatcher compiles the configured patterns. Invalid regular expressions and
// blank entries are skipped instead of failing the whole feature, so one typo in
// the console cannot silently disable redaction or break relay requests.
func buildMatcher(setting RedactSetting) *matcher {
	if !setting.Enabled {
		return nil
	}
	mask := setting.Mask
	if mask == "" {
		mask = DefaultMask
	}
	built := &matcher{mask: mask}
	seen := make(map[string]struct{}, len(setting.Patterns))
	for _, raw := range setting.Patterns {
		if len(built.literals)+len(built.regexps) >= maxRedactPatterns {
			break
		}
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		if _, duplicate := seen[pattern]; duplicate {
			continue
		}
		seen[pattern] = struct{}{}

		if expr, ok := strings.CutPrefix(pattern, "re:"); ok {
			expr = strings.TrimSpace(expr)
			if expr == "" {
				continue
			}
			compiledExpr, err := regexp.Compile(expr)
			if err != nil {
				continue
			}
			built.regexps = append(built.regexps, compiledExpr)
			continue
		}
		built.literals = append(built.literals, pattern)
	}
	if len(built.literals) == 0 && len(built.regexps) == 0 {
		return nil
	}
	return built
}

func rebuildMatcher() {
	compiled.Store(buildMatcher(redactSetting))
	installRelaykitMasker()
}

// activeMatcher returns the compiled matcher, or nil when redaction is off.
func activeMatcher() *matcher {
	return compiled.Load()
}

// Mask replaces every configured pattern in text. It is safe to call when
// redaction is disabled or unconfigured: the input is returned unchanged.
func (m *matcher) Mask(text string) string {
	if m == nil || text == "" {
		return text
	}
	masked := text
	for _, literal := range m.literals {
		masked = strings.ReplaceAll(masked, literal, m.mask)
	}
	for _, expr := range m.regexps {
		masked = expr.ReplaceAllString(masked, m.mask)
	}
	return masked
}

// MaskErrorRenders reports whether error messages must be redacted.
func MaskErrorRenders() bool {
	return maskedFor(func(setting RedactSetting) bool { return setting.MaskErrorMessages })
}

// MaskModelNames reports whether model names must be redacted.
func MaskModelNames() bool {
	return maskedFor(func(setting RedactSetting) bool { return setting.MaskModelNames })
}

func maskedFor(pick func(RedactSetting) bool) bool {
	if activeMatcher() == nil {
		return false
	}
	return pick(redactSetting)
}

// MaskError redacts a user-facing error message.
func MaskError(text string) string {
	if !MaskErrorRenders() {
		return text
	}
	return activeMatcher().Mask(text)
}

// MaskModel applies model-name redaction to one name.
func MaskModel(name string) string {
	if !MaskModelNames() {
		return name
	}
	return activeMatcher().Mask(name)
}

// MaskErrorWithExtra redacts a user-facing error message against the configured
// patterns plus caller-supplied names that must never reach the client, such as
// the selected channel's upstream model name. Extra names are matched literally
// and are not persisted.
func MaskErrorWithExtra(text string, extra ...string) string {
	if !MaskErrorRenders() {
		return text
	}
	active := activeMatcher()
	if active == nil {
		return text
	}
	masked := active.Mask(text)
	for _, name := range extra {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		masked = strings.ReplaceAll(masked, name, active.mask)
	}
	return masked
}
