package transform

// LeetTransformer generates l33t speak variations
type LeetTransformer struct {
	MaxVariants int // cap to prevent combinatorial explosion
}

// Standard leet substitution map: each character maps to possible replacements
var leetMap = map[rune][]rune{
	'a': {'@', '4'},
	'A': {'@', '4'},
	'b': {'8'},
	'B': {'8'},
	'c': {'('},
	'C': {'('},
	'd': {'|'},
	'D': {'|'},
	'e': {'3'},
	'E': {'3'},
	'g': {'9', '6'},
	'G': {'9', '6'},
	'h': {'#'},
	'H': {'#'},
	'i': {'1', '!'},
	'I': {'1', '!'},
	'l': {'1', '|'},
	'L': {'1', '|'},
	'n': {'~'},
	'N': {'~'},
	'o': {'0'},
	'O': {'0'},
	'r': {'2'},
	'R': {'2'},
	's': {'5', '$'},
	'S': {'5', '$'},
	't': {'7', '+'},
	'T': {'7', '+'},
	'u': {'v'},
	'U': {'v'},
	'x': {'%'},
	'X': {'%'},
	'z': {'2'},
	'Z': {'2'},
}

func (l *LeetTransformer) Name() string { return "leet" }

func (l *LeetTransformer) Transform(input string) []string {
	maxVariants := l.MaxVariants
	if maxVariants <= 0 {
		maxVariants = 2048
	}

	runes := []rune(input)
	seen := make(map[string]struct{})
	var results []string

	add := func(s string) bool {
		if len(results) >= maxVariants {
			return false
		}
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			results = append(results, s)
		}
		return true
	}

	// Include original
	add(input)

	// Find positions that have leet substitutions
	var positions []leetPos
	for i, r := range runes {
		if subs, ok := leetMap[r]; ok {
			positions = append(positions, leetPos{index: i, subs: subs})
		}
	}

	if len(positions) == 0 {
		return results
	}

	// Priority 1: Full leet variants (all positions substituted with each first sub)
	// This generates the most "recognizable" leet like g00gle, p@$$w0rd
	for _, subIdx := range []int{0, 1} {
		fullLeet := make([]rune, len(runes))
		copy(fullLeet, runes)
		for _, p := range positions {
			idx := subIdx
			if idx >= len(p.subs) {
				idx = 0
			}
			fullLeet[p.index] = p.subs[idx]
		}
		if !add(string(fullLeet)) {
			return results
		}
	}

	// Priority 2: Single-position substitutions
	for _, p := range positions {
		for _, sub := range p.subs {
			variant := make([]rune, len(runes))
			copy(variant, runes)
			variant[p.index] = sub
			if !add(string(variant)) {
				return results
			}
		}
	}

	// Priority 3: Two-position substitutions (every pair)
	for i := 0; i < len(positions); i++ {
		for j := i + 1; j < len(positions); j++ {
			for _, sub1 := range positions[i].subs {
				for _, sub2 := range positions[j].subs {
					variant := make([]rune, len(runes))
					copy(variant, runes)
					variant[positions[i].index] = sub1
					variant[positions[j].index] = sub2
					if !add(string(variant)) {
						return results
					}
				}
			}
		}
	}

	// Priority 4: Full combinatorial exploration for remaining budget
	l.generateCombinations(runes, positions, 0, make([]rune, len(runes)), seen, &results, maxVariants)

	return results
}

func (l *LeetTransformer) generateCombinations(
	original []rune,
	positions []leetPos,
	posIdx int,
	current []rune,
	seen map[string]struct{},
	results *[]string,
	maxVariants int,
) {
	if len(*results) >= maxVariants {
		return
	}

	if posIdx == 0 {
		copy(current, original)
	}

	if posIdx >= len(positions) {
		s := string(current)
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			*results = append(*results, s)
		}
		return
	}

	p := positions[posIdx]

	// Keep original character at this position
	l.generateCombinations(original, positions, posIdx+1, current, seen, results, maxVariants)

	// Try each substitution at this position
	origChar := current[p.index]
	for _, sub := range p.subs {
		current[p.index] = sub
		l.generateCombinations(original, positions, posIdx+1, current, seen, results, maxVariants)
		if len(*results) >= maxVariants {
			current[p.index] = origChar
			return
		}
	}
	current[p.index] = origChar
}

type leetPos struct {
	index int
	subs  []rune
}
