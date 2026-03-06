package transform

import (
	"strings"
	"unicode"
)

// CaseTransformer generates case variations of input strings.
type CaseTransformer struct{}

func (c *CaseTransformer) Name() string { return "case" }

func (c *CaseTransformer) Transform(input string) []string {
	seen := make(map[string]struct{})
	var results []string

	add := func(s string) {
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			results = append(results, s)
		}
	}

	// Original, lowercase, uppercase
	add(input)
	add(strings.ToLower(input))
	add(strings.ToUpper(input))

	// Title case
	lower := strings.ToLower(input)
	if len(lower) > 0 {
		runes := []rune(lower)
		runes[0] = unicode.ToUpper(runes[0])
		add(string(runes))
	}

	// Inverted title
	upper := strings.ToUpper(input)
	if len(upper) > 0 {
		runes := []rune(upper)
		runes[0] = unicode.ToLower(runes[0])
		add(string(runes))
	}

	// Toggle case
	runes := []rune(input)
	toggled := make([]rune, len(runes))
	for i, r := range runes {
		if unicode.IsUpper(r) {
			toggled[i] = unicode.ToLower(r)
		} else {
			toggled[i] = unicode.ToUpper(r)
		}
	}
	add(string(toggled))

	// Capitalize each position individually
	lowerRunes := []rune(strings.ToLower(input))
	for i := range lowerRunes {
		if unicode.IsLetter(lowerRunes[i]) {
			variant := make([]rune, len(lowerRunes))
			copy(variant, lowerRunes)
			variant[i] = unicode.ToUpper(variant[i])
			add(string(variant))
		}
	}

	// Alternating case: aLtErNaTe and AlTeRnAtE
	alt1 := make([]rune, len(lowerRunes))
	alt2 := make([]rune, len(lowerRunes))
	for i, r := range lowerRunes {
		if i%2 == 0 {
			alt1[i] = unicode.ToLower(r)
			alt2[i] = unicode.ToUpper(r)
		} else {
			alt1[i] = unicode.ToUpper(r)
			alt2[i] = unicode.ToLower(r)
		}
	}
	add(string(alt1))
	add(string(alt2))

	// First and last letter capitalised
	if len(lowerRunes) > 1 {
		variant := make([]rune, len(lowerRunes))
		copy(variant, lowerRunes)
		variant[0] = unicode.ToUpper(variant[0])
		variant[len(variant)-1] = unicode.ToUpper(variant[len(variant)-1])
		add(string(variant))
	}

	return results
}
