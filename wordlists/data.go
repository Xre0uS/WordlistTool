package wordlists

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Symbols           []string `toml:"symbols"`
	SymbolsCompact    []string `toml:"symbols_compact"`
	SymbolsExtended   []string `toml:"symbols_extended"`
	SuffixWords       []string `toml:"suffix_words"`
	PrefixWords       []string `toml:"prefix_words"`
	Separators        []string `toml:"separators"`
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

// Exported variables populated by LoadFromTOML.
// These are referenced by transform/ and main.go.
var (
	Symbols           []string
	SymbolsShort      []string
	SymbolsFull       []string
	CommonSuffixWords []string
	CommonPrefixWords []string
	Separators        []string
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
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	apply(cfg)
	return nil
}

func apply(cfg *Config) {
	Symbols = cfg.Symbols
	SymbolsShort = cfg.SymbolsCompact
	SymbolsFull = cfg.SymbolsExtended
	CommonSuffixWords = cfg.SuffixWords
	CommonPrefixWords = cfg.PrefixWords
	Separators = cfg.Separators
	SeparatorsShort = cfg.SeparatorsCompact
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
	for y := cfg.Years.ShortStart2; y <= cfg.Years.ShortEnd2; y++ {
		Years = append(Years, fmt.Sprintf("%02d", y))
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
			if len(v) > 0 {
				runes = append(runes, []rune(v)[0])
			}
		}
		LeetMap[key] = runes
	}
}
