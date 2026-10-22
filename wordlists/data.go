package wordlists

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Symbols           []string `toml:"symbols"`
	SymbolsCompact    []string `toml:"symbols_compact"`
	SymbolsExtended   []string `toml:"symbols_extended"`
	SuffixWords       []string `toml:"suffix_words"`
	PrefixWords       []string `toml:"prefix_words"`
	Separators        []string `toml:"separators"`
	SeparatorsMid     []string `toml:"separators_mid"`
	SeparatorsCompact []string `toml:"separators_compact"`
	KeyboardWalks     []string `toml:"keyboard_walks"`
	NumberSequences   []string `toml:"number_sequences"`

	Years    yearsConfig         `toml:"years"`
	Patterns patternsConfig      `toml:"patterns"`
	Leet     map[string][]string `toml:"leet"`
}

type yearsConfig struct {
	FullStart   int `toml:"full_start"`
	FullEnd     int `toml:"full_end"`
	ShortStart  int `toml:"short_start"`
	ShortEnd    int `toml:"short_end"`
	ShortStart2 int `toml:"short_start2"`
	ShortEnd2   int `toml:"short_end2"`
}

type patternsConfig struct {
	Symbols      []string `toml:"symbols"`
	SymbolPairs  []string `toml:"symbol_pairs"`
	MultiSymbols []string `toml:"multi_symbols"`
	Numbers      []string `toml:"numbers"`
	Years        []string `toml:"years"`
}

// Exported variables populated by LoadFromTOML, referenced by transform/ and
// main.go. Symbols is the raw config list rather than a tier the builders read;
// it is what TestConfiguredTokensReachable asserts the tiers still cover.
var (
	Symbols           []string
	SymbolsShort      []string
	SymbolsFull       []string
	CommonSuffixWords []string
	CommonPrefixWords []string
	Separators        []string
	SeparatorsMid     []string
	SeparatorsShort   []string
	KeyboardWalks     []string
	NumberSequences   []string

	Digits         []string
	TwoDigits      []string
	ThreeDigits    []string
	FourDigits     []string
	Years          []string
	CommonPatterns []string

	LeetMap map[rune][]rune
)

// LoadFromTOML reads a TOML config file and populates all exported variables.
func LoadFromTOML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	cfg := &Config{}
	md, err := toml.Decode(string(data), cfg)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	// A key this build does not know is almost always a typo, and the data
	// under it would otherwise vanish from the output without a word.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			keys = append(keys, k.String())
		}
		return fmt.Errorf("unknown key(s) in %s: %s", path, strings.Join(keys, ", "))
	}

	warnEmpty(cfg)
	apply(cfg)
	return nil
}

// warnEmpty reports lists that are missing or empty. Each one is a whole
// category of candidates that will not appear in the output.
func warnEmpty(cfg *Config) {
	empty := []string{}
	check := func(name string, n int) {
		if n == 0 {
			empty = append(empty, name)
		}
	}
	check("symbols", len(cfg.Symbols))
	check("symbols_compact", len(cfg.SymbolsCompact))
	check("symbols_extended", len(cfg.SymbolsExtended))
	check("suffix_words", len(cfg.SuffixWords))
	check("prefix_words", len(cfg.PrefixWords))
	check("separators", len(cfg.Separators))
	check("separators_mid", len(cfg.SeparatorsMid))
	check("separators_compact", len(cfg.SeparatorsCompact))
	check("keyboard_walks", len(cfg.KeyboardWalks))
	check("number_sequences", len(cfg.NumberSequences))
	check("patterns.symbols", len(cfg.Patterns.Symbols))
	check("leet", len(cfg.Leet))
	if cfg.Years.FullEnd < cfg.Years.FullStart {
		empty = append(empty, "years.full_start/full_end")
	}
	if len(empty) > 0 {
		fmt.Fprintf(os.Stderr, "[!] Config: empty or missing: %s\n", strings.Join(empty, ", "))
	}
}

func apply(cfg *Config) {
	Symbols = cfg.Symbols
	SymbolsShort = cfg.SymbolsCompact
	// symbols and symbols_extended both feed the direct append/prepend stages,
	// so take the union — anything listed in either must be reachable.
	SymbolsFull = dedupe(cfg.Symbols, cfg.SymbolsExtended)
	CommonSuffixWords = cfg.SuffixWords
	CommonPrefixWords = cfg.PrefixWords
	// Three live tiers, each nested in the next, so every separator in the
	// config reaches the output even if only through the cheap pairings.
	SeparatorsShort = dedupe(cfg.SeparatorsCompact)
	SeparatorsMid = dedupe(cfg.SeparatorsMid, SeparatorsShort)
	Separators = dedupe(cfg.Separators, SeparatorsMid)
	KeyboardWalks = cfg.KeyboardWalks
	NumberSequences = cfg.NumberSequences

	Digits = make([]string, 10)
	for i := 0; i <= 9; i++ {
		Digits[i] = fmt.Sprintf("%d", i)
	}

	TwoDigits = make([]string, 100)
	for i := 0; i <= 99; i++ {
		TwoDigits[i] = fmt.Sprintf("%02d", i)
	}

	ThreeDigits = make([]string, 1000)
	for i := 0; i <= 999; i++ {
		ThreeDigits[i] = fmt.Sprintf("%03d", i)
	}

	FourDigits = make([]string, 10000)
	for i := 0; i <= 9999; i++ {
		FourDigits[i] = fmt.Sprintf("%04d", i)
	}

	Years = nil
	for y := cfg.Years.FullStart; y <= cfg.Years.FullEnd; y++ {
		Years = append(Years, fmt.Sprintf("%d", y))
	}
	for y := cfg.Years.ShortStart; y <= cfg.Years.ShortEnd; y++ {
		Years = append(Years, fmt.Sprintf("%02d", y))
	}
	if cfg.Years.ShortStart2 != 0 || cfg.Years.ShortEnd2 != 0 {
		for y := cfg.Years.ShortStart2; y <= cfg.Years.ShortEnd2; y++ {
			Years = append(Years, fmt.Sprintf("%02d", y))
		}
	}

	CommonPatterns = nil
	for _, s := range cfg.Patterns.Symbols {
		for _, y := range Years {
			CommonPatterns = append(CommonPatterns, y+s, s+y)
		}
	}
	for _, s := range cfg.Patterns.Symbols {
		for _, d := range Digits {
			CommonPatterns = append(CommonPatterns, d+s, s+d)
		}
	}
	for _, s1 := range cfg.Patterns.SymbolPairs {
		for _, s2 := range cfg.Patterns.SymbolPairs {
			for _, d := range cfg.Patterns.Numbers {
				CommonPatterns = append(CommonPatterns, s1+d+s2)
			}
		}
	}
	for _, s := range cfg.Patterns.MultiSymbols {
		for _, y := range cfg.Patterns.Years {
			CommonPatterns = append(CommonPatterns, s+y, y+s)
		}
	}

	LeetMap = make(map[rune][]rune)
	for k, vals := range cfg.Leet {
		if len(k) == 0 {
			continue
		}
		key := []rune(k)[0]
		var runes []rune
		for _, v := range vals {
			if len(v) == 0 {
				continue
			}
			vr := []rune(v)
			if len(vr) > 1 {
				fmt.Fprintf(os.Stderr, "[!] Config: leet %q = %q is multi-character, using %q\n",
					k, v, string(vr[0]))
			}
			runes = append(runes, vr[0])
		}
		LeetMap[key] = runes
	}

	// The leet stage always runs after a case stage, so without the folded
	// keys an upper-case or title-case variant gets no substitutions at all.
	for k := range cfg.Leet {
		if len(k) == 0 {
			continue
		}
		key := []rune(k)[0]
		for _, alias := range []rune{unicode.ToLower(key), unicode.ToUpper(key)} {
			if alias == key {
				continue
			}
			if _, explicit := cfg.Leet[string(alias)]; explicit {
				continue
			}
			LeetMap[alias] = LeetMap[key]
		}
	}
}

// dedupe concatenates lists, dropping repeats and keeping first-seen order.
func dedupe(lists ...[]string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, list := range lists {
		for _, v := range list {
			if _, exists := seen[v]; exists {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}
