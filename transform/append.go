package transform

import (
	"github.com/Xre0uS/wordlisttool/wordlists"
)

// AppendTransformer appends common suffixes to inputs
type AppendTransformer struct {
	// Suffixes to append; if nil, uses default wordlists
	Suffixes []string
}

func (a *AppendTransformer) Name() string { return "append" }

func (a *AppendTransformer) Transform(input string) []string {
	suffixes := a.Suffixes
	if suffixes == nil {
		suffixes = buildDefaultSuffixes()
	}

	results := make([]string, 0, len(suffixes)+1)
	seen := make(map[string]struct{})

	add := func(s string) {
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			results = append(results, s)
		}
	}

	// Original
	add(input)

	for _, suffix := range suffixes {
		add(input + suffix)
	}

	return results
}

func buildDefaultSuffixes() []string {
	var suffixes []string

	// Full symbols
	suffixes = append(suffixes, wordlists.SymbolsFull...)

	// Single digits
	suffixes = append(suffixes, wordlists.Digits...)

	// All two-digit numbers 00-99
	suffixes = append(suffixes, wordlists.TwoDigits...)

	// All three-digit numbers 000-999
	suffixes = append(suffixes, wordlists.ThreeDigits...)

	// All four-digit numbers 0000-9999
	suffixes = append(suffixes, wordlists.FourDigits...)

	// Number sequences
	suffixes = append(suffixes, wordlists.NumberSequences...)

	// Years
	suffixes = append(suffixes, wordlists.Years...)

	// Keyboard walks
	suffixes = append(suffixes, wordlists.KeyboardWalks...)

	// Common suffix words
	suffixes = append(suffixes, wordlists.CommonSuffixWords...)

	// Common patterns (symbol+number combos)
	suffixes = append(suffixes, wordlists.CommonPatterns...)

	// Separator + number/year/word combos
	for _, sep := range wordlists.SeparatorsShort {
		if sep == "" {
			continue
		}
		for _, y := range wordlists.Years {
			suffixes = append(suffixes, sep+y)
		}
		for _, d := range wordlists.Digits {
			suffixes = append(suffixes, sep+d)
		}
		for _, d := range wordlists.TwoDigits {
			suffixes = append(suffixes, sep+d)
		}
		for _, d := range wordlists.ThreeDigits {
			suffixes = append(suffixes, sep+d)
		}
		for _, w := range wordlists.CommonSuffixWords {
			suffixes = append(suffixes, sep+w)
		}
		for _, ns := range wordlists.NumberSequences {
			suffixes = append(suffixes, sep+ns)
		}
	}

	// Number + symbol combos
	for _, n := range wordlists.Digits {
		for _, s := range wordlists.SymbolsShort {
			suffixes = append(suffixes, n+s)
			suffixes = append(suffixes, s+n)
		}
	}
	for _, n := range wordlists.TwoDigits {
		for _, s := range wordlists.SymbolsShort {
			suffixes = append(suffixes, n+s)
		}
	}
	for _, n := range wordlists.ThreeDigits {
		for _, s := range wordlists.SymbolsShort {
			suffixes = append(suffixes, n+s)
		}
	}

	// Year + symbol
	for _, y := range wordlists.Years {
		for _, s := range wordlists.SymbolsFull {
			suffixes = append(suffixes, y+s)
		}
	}

	// Symbol + year
	for _, s := range wordlists.SymbolsFull {
		for _, y := range wordlists.Years {
			suffixes = append(suffixes, s+y)
		}
	}

	// Word + digit, word + year
	for _, w := range wordlists.CommonSuffixWords {
		for _, d := range wordlists.Digits {
			suffixes = append(suffixes, w+d)
		}
		for _, y := range wordlists.Years {
			suffixes = append(suffixes, w+y)
		}
		for _, s := range []string{"!", "@", "#", "$", "123", "1234"} {
			suffixes = append(suffixes, w+s)
		}
	}

	// Keyboard walk + suffix
	for _, kw := range wordlists.KeyboardWalks {
		for _, s := range []string{"!", "@", "#", "1", "123"} {
			suffixes = append(suffixes, kw+s)
		}
	}

	return suffixes
}
