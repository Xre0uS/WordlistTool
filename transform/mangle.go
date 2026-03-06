package transform

import (
	"strings"
	"unicode"
)

// MangleTransformer applies string mangling: reverse, duplicate, truncate, etc.
type MangleTransformer struct{}

func (m *MangleTransformer) Name() string { return "mangle" }

func (m *MangleTransformer) Transform(input string) []string {
	seen := make(map[string]struct{})
	var results []string

	add := func(s string) {
		if s == "" {
			return
		}
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			results = append(results, s)
		}
	}

	add(input)

	// Reversed
	runes := []rune(input)
	reversed := make([]rune, len(runes))
	for i, r := range runes {
		reversed[len(runes)-1-i] = r
	}
	add(string(reversed))

	// Duplicated
	add(input + input)

	// Half-word repetition
	if len(runes) >= 2 {
		half := len(runes) / 2
		add(string(runes[:half]) + string(runes[:half]))
		add(string(runes[half:]) + string(runes[half:]))
	}

	// Reversed + original
	add(string(reversed) + input)
	// Original + reversed
	add(input + string(reversed))

	// Remove vowels
	var noVowels []rune
	for _, r := range runes {
		lr := unicode.ToLower(r)
		if lr != 'a' && lr != 'e' && lr != 'i' && lr != 'o' && lr != 'u' {
			noVowels = append(noVowels, r)
		}
	}
	if len(noVowels) > 0 {
		add(string(noVowels))
	}

	// Remove consonants
	var vowelsOnly []rune
	for _, r := range runes {
		lr := unicode.ToLower(r)
		if lr == 'a' || lr == 'e' || lr == 'i' || lr == 'o' || lr == 'u' || !unicode.IsLetter(r) {
			vowelsOnly = append(vowelsOnly, r)
		}
	}
	if len(vowelsOnly) > 0 {
		add(string(vowelsOnly))
	}

	// Truncations (first N / last N)
	for _, n := range []int{3, 4, 5, 6, 7, 8} {
		if n < len(runes) {
			add(string(runes[:n]))
		}
	}

	for _, n := range []int{3, 4, 5, 6} {
		if n < len(runes) {
			add(string(runes[len(runes)-n:]))
		}
	}

	// Double each character
	var doubled []rune
	for _, r := range runes {
		doubled = append(doubled, r, r)
	}
	add(string(doubled))

	// First + last character
	if len(runes) >= 2 {
		add(string([]rune{runes[0], runes[len(runes)-1]}))
	}

	// Replace spaces with common separators
	if strings.Contains(input, " ") {
		for _, sep := range []string{"", "_", "-", ".", "@"} {
			add(strings.ReplaceAll(input, " ", sep))
		}
	}

	lower := strings.ToLower(input)
	upper := strings.ToUpper(input)
	_ = lower
	_ = upper

	// Stutter: repeat first letter
	if len(runes) >= 1 {
		add(string([]rune{runes[0]}) + input)
		add(string([]rune{runes[0], runes[0]}) + input)
		add(string([]rune{runes[0], runes[0], runes[0]}) + input)
	}

	// Swap first two chars
	if len(runes) >= 2 {
		swapped := make([]rune, len(runes))
		copy(swapped, runes)
		swapped[0], swapped[1] = swapped[1], swapped[0]
		add(string(swapped))
	}

	// Swap last two chars
	if len(runes) >= 2 {
		swapped := make([]rune, len(runes))
		copy(swapped, runes)
		n := len(swapped)
		swapped[n-1], swapped[n-2] = swapped[n-2], swapped[n-1]
		add(string(swapped))
	}

	return results
}
