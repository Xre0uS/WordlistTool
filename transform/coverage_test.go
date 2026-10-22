package transform

import (
	"testing"

	"github.com/Xre0uS/wordlisttool/wordlists"
)

// Every token in the config must reach the output somewhere. A list the config
// defines but no builder reads is the bug this guards: it leaves candidates
// unreachable with nothing on stderr to say so.
func TestConfiguredTokensReachable(t *testing.T) {
	if err := wordlists.LoadFromTOML("../config.toml"); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	suffixes := index(buildDefaultSuffixes())
	prefixes := index(buildDefaultPrefixes())

	cases := []struct {
		name  string
		list  []string
		in    map[string]struct{}
		which string
	}{
		{"symbols", wordlists.Symbols, suffixes, "suffixes"},
		{"symbols", wordlists.Symbols, prefixes, "prefixes"},
		{"symbols_compact", wordlists.SymbolsShort, suffixes, "suffixes"},
		{"symbols_extended", wordlists.SymbolsFull, suffixes, "suffixes"},
		{"separators", wordlists.Separators, suffixes, "suffixes"},
		{"separators", wordlists.Separators, prefixes, "prefixes"},
		{"separators_mid", wordlists.SeparatorsMid, suffixes, "suffixes"},
		{"separators_compact", wordlists.SeparatorsShort, suffixes, "suffixes"},
		{"suffix_words", wordlists.CommonSuffixWords, suffixes, "suffixes"},
		{"prefix_words", wordlists.CommonPrefixWords, prefixes, "prefixes"},
		{"keyboard_walks", wordlists.KeyboardWalks, suffixes, "suffixes"},
		{"number_sequences", wordlists.NumberSequences, suffixes, "suffixes"},
		{"years", wordlists.Years, suffixes, "suffixes"},
	}

	for _, c := range cases {
		if len(c.list) == 0 {
			t.Errorf("%s: empty, nothing to cover", c.name)
			continue
		}
		for _, token := range c.list {
			if token == "" {
				continue
			}
			if _, ok := c.in[token]; !ok {
				t.Errorf("%s: %q never reaches the default %s", c.name, token, c.which)
			}
		}
	}
}

// Leet runs after a case stage, so the map has to answer for both cases or
// upper-case and title-case variants get no substitutions at all.
func TestLeetMapCoversBothCases(t *testing.T) {
	if err := wordlists.LoadFromTOML("../config.toml"); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	for _, pair := range [][2]rune{{'a', 'A'}, {'s', 'S'}, {'e', 'E'}, {'o', 'O'}} {
		lower, ok := wordlists.LeetMap[pair[0]]
		if !ok {
			continue
		}
		upper, ok := wordlists.LeetMap[pair[1]]
		if !ok {
			t.Errorf("leet: %q has substitutions but %q has none", string(pair[0]), string(pair[1]))
			continue
		}
		if len(upper) != len(lower) {
			t.Errorf("leet: %q has %d substitutions, %q has %d",
				string(pair[0]), len(lower), string(pair[1]), len(upper))
		}
	}

	leet := &LeetTransformer{MaxVariants: 256}
	if got := leet.Transform("SMBADMIN"); len(got) < 2 {
		t.Errorf("leet on an upper-case word returned %d variants: %v", len(got), got)
	}
}

func index(list []string) map[string]struct{} {
	out := make(map[string]struct{}, len(list))
	for _, v := range list {
		out[v] = struct{}{}
	}
	return out
}
