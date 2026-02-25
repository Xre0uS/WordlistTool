package transform

import (
	"github.com/Xre0uS/wordlisttool/wordlists"
)

// PrependTransformer prepends common prefixes to inputs
type PrependTransformer struct {
	// Prefixes to prepend; if nil, uses default wordlists
	Prefixes []string
}

func (p *PrependTransformer) Name() string { return "prepend" }

func (p *PrependTransformer) Transform(input string) []string {
	prefixes := p.Prefixes
	if prefixes == nil {
		prefixes = buildDefaultPrefixes()
	}

	results := make([]string, 0, len(prefixes)+1)
	seen := make(map[string]struct{})

	add := func(s string) {
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			results = append(results, s)
		}
	}

	// Original
	add(input)

	for _, prefix := range prefixes {
		add(prefix + input)
	}

	return results
}

func buildDefaultPrefixes() []string {
	var prefixes []string

	// Full symbols
	prefixes = append(prefixes, wordlists.SymbolsFull...)

	// Single digits
	prefixes = append(prefixes, wordlists.Digits...)

	// All two-digit numbers
	prefixes = append(prefixes, wordlists.TwoDigits...)

	// All three-digit numbers
	prefixes = append(prefixes, wordlists.ThreeDigits...)

	// All four-digit numbers
	prefixes = append(prefixes, wordlists.FourDigits...)

	// Number sequences
	prefixes = append(prefixes, wordlists.NumberSequences...)

	// Years
	prefixes = append(prefixes, wordlists.Years...)

	// Keyboard walks
	prefixes = append(prefixes, wordlists.KeyboardWalks...)

	// Common prefix words
	prefixes = append(prefixes, wordlists.CommonPrefixWords...)

	// Common patterns
	prefixes = append(prefixes, wordlists.CommonPatterns...)

	// Symbol + number combos
	for _, s := range wordlists.SymbolsShort {
		for _, n := range wordlists.Digits {
			prefixes = append(prefixes, s+n, n+s)
		}
		for _, n := range wordlists.TwoDigits {
			prefixes = append(prefixes, s+n, n+s)
		}
		for _, n := range wordlists.ThreeDigits {
			prefixes = append(prefixes, s+n)
		}
	}

	// Symbol + year
	for _, s := range wordlists.SymbolsFull {
		for _, y := range wordlists.Years {
			prefixes = append(prefixes, s+y, y+s)
		}
	}

	// Separator + word/number combos
	for _, sep := range wordlists.SeparatorsShort {
		if sep == "" {
			continue
		}
		for _, w := range wordlists.CommonPrefixWords {
			prefixes = append(prefixes, w+sep)
		}
		for _, d := range wordlists.Digits {
			prefixes = append(prefixes, d+sep)
		}
		for _, d := range wordlists.TwoDigits {
			prefixes = append(prefixes, d+sep)
		}
		for _, d := range wordlists.ThreeDigits {
			prefixes = append(prefixes, d+sep)
		}
		for _, y := range wordlists.Years {
			prefixes = append(prefixes, y+sep)
		}
		for _, ns := range wordlists.NumberSequences {
			prefixes = append(prefixes, ns+sep)
		}
	}

	// Word + number
	for _, w := range wordlists.CommonPrefixWords {
		for _, d := range wordlists.Digits {
			prefixes = append(prefixes, w+d)
		}
		for _, y := range wordlists.Years {
			prefixes = append(prefixes, w+y)
		}
		for _, s := range []string{"!", "@", "#", "123", "1234"} {
			prefixes = append(prefixes, w+s)
		}
	}

	return prefixes
}
