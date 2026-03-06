package transform

import (
	"github.com/Xre0uS/wordlisttool/wordlists"
)

// AppendTransformer appends suffixes to inputs. Uses built-in defaults when Suffixes is nil.
type AppendTransformer struct {
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

	add(input)

	for _, suffix := range suffixes {
		add(input + suffix)
	}

	return results
}

func buildDefaultSuffixes() []string {
	var suffixes []string

	suffixes = append(suffixes, wordlists.SymbolsFull...)
	suffixes = append(suffixes, wordlists.Digits...)
	suffixes = append(suffixes, wordlists.TwoDigits...)
	suffixes = append(suffixes, wordlists.ThreeDigits...)
	suffixes = append(suffixes, wordlists.FourDigits...)
	suffixes = append(suffixes, wordlists.NumberSequences...)
	suffixes = append(suffixes, wordlists.Years...)
	suffixes = append(suffixes, wordlists.KeyboardWalks...)
	suffixes = append(suffixes, wordlists.CommonSuffixWords...)
	suffixes = append(suffixes, wordlists.CommonPatterns...)

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

	for _, y := range wordlists.Years {
		for _, s := range wordlists.SymbolsFull {
			suffixes = append(suffixes, y+s)
		}
	}

	for _, s := range wordlists.SymbolsFull {
		for _, y := range wordlists.Years {
			suffixes = append(suffixes, s+y)
		}
	}

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

	for _, kw := range wordlists.KeyboardWalks {
		for _, s := range []string{"!", "@", "#", "1", "123"} {
			suffixes = append(suffixes, kw+s)
		}
	}

	return suffixes
}
