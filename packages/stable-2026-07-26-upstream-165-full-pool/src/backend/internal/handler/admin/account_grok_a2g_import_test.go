package admin

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestParseGrokA2GTokensFromTxt(t *testing.T) {
	tokens, errs := parseGrokA2GTokens(GrokA2GImportRequest{
		Content: "sso=AAA111BBB222CCC333DDD\nother-token-xyz-1234567890\nAAA111BBB222CCC333DDD\n",
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 unique tokens, got %d: %#v", len(tokens), tokens)
	}
	if tokens[0] != "AAA111BBB222CCC333DDD" {
		t.Fatalf("token0=%q", tokens[0])
	}
}

func TestParseGrokA2GTokensFromPoolJSON(t *testing.T) {
	raw := `{
		"basic": [{"token":"tok-basic-aaaaaaaa","tags":["nsfw"]}],
		"super": ["tok-super-bbbbbbbb"]
	}`
	tokens, errs := parseGrokA2GTokens(GrokA2GImportRequest{Content: raw})
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2, got %d %#v", len(tokens), tokens)
	}
}

func TestParseGrokA2GTokensFromSub2ExportAccounts(t *testing.T) {
	raw := `{
		"type":"sub2api-data",
		"accounts":[
			{"name":"a","platform":"grok","credentials":{"sso":"sso-token-aaaaaaaa","refresh_token":"rt1"}},
			{"name":"b","platform":"grok","credentials":{"sso_token":"sso-token-bbbbbbbb"}}
		]
	}`
	tokens, errs := parseGrokA2GTokens(GrokA2GImportRequest{Content: raw})
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2, got %d %#v", len(tokens), tokens)
	}
	joined := strings.Join(tokens, ",")
	if !strings.Contains(joined, "sso-token-aaaaaaaa") || !strings.Contains(joined, "sso-token-bbbbbbbb") {
		t.Fatalf("unexpected tokens: %s", joined)
	}
}

func TestExtractAccountSSOTokens(t *testing.T) {
	acc := service.Account{
		Name: "x",
		Credentials: map[string]any{
			"sso": "sso=cookie-value-zzzzzzzz",
		},
	}
	got := extractAccountSSOTokens(acc)
	if len(got) != 1 || got[0] != "cookie-value-zzzzzzzz" {
		t.Fatalf("got %#v", got)
	}
}

func TestMaskSSOToken(t *testing.T) {
	short := maskSSOToken("short")
	if short != "short" {
		t.Fatalf("short=%q", short)
	}
	long := "abcdefghij1234567890xyz"
	got := maskSSOToken(long)
	if !strings.Contains(got, "...") {
		t.Fatalf("expected masked, got %q", got)
	}
}

func TestDefaultGrokA2GMaxConvertPositive(t *testing.T) {
	if defaultGrokA2GMaxConvert <= 0 {
		t.Fatalf("default max convert should be positive, got %d", defaultGrokA2GMaxConvert)
	}
	if defaultGrokA2GMaxConvert > 200 {
		t.Fatalf("default max convert too large for interactive import: %d", defaultGrokA2GMaxConvert)
	}
}

func TestNormalizeGrokEmail(t *testing.T) {
	if got := normalizeGrokEmail("  Foo@Bar.COM "); got != "foo@bar.com" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeGrokEmail("not-an-email"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
