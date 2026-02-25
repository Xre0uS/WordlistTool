package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Preset controls how many candidates are generated
type Preset int

const (
	PresetShort    Preset = iota // Fewer chains, lower leet budget — fast
	PresetStandard               // Balanced — default
	PresetLong                   // All chains, high leet budget — thorough
)

func ParsePreset(s string) (Preset, error) {
	switch strings.ToLower(s) {
	case "short", "s":
		return PresetShort, nil
	case "standard", "std", "":
		return PresetStandard, nil
	case "long", "l":
		return PresetLong, nil
	default:
		return PresetStandard, fmt.Errorf("unknown preset %q (use short, standard, long)", s)
	}
}

// Config holds all configuration for the wordlist generator
type Config struct {
	InputFile  string
	OutputFile string
	Threads    int
	Preset     Preset
	MinLength  int

	// Transform toggles (true = enabled)
	EnableCase    bool
	EnableLeet    bool
	EnableAppend  bool
	EnablePrepend bool
	EnableCombine bool

	// Limits
	LeetMax       int
	MaxCandidates int64
	Verbose       bool
}

// Parse parses command-line flags into a Config
func Parse() (*Config, error) {
	cfg := &Config{}

	var presetStr string
	var noCaseFlag, noLeetFlag, noAppendFlag, noPrependFlag, noCombineFlag bool

	// ── I/O ──
	flag.StringVar(&cfg.InputFile, "i", "", "Input keyword file (one word per line)")
	flag.StringVar(&cfg.OutputFile, "o", "wordlist.txt", "Output wordlist file")
	flag.IntVar(&cfg.Threads, "t", runtime.NumCPU(), "Worker threads")

	// ── Preset ──
	flag.StringVar(&presetStr, "preset", "standard", "Generation preset: short, standard, long")

	// ── Filters ──
	flag.IntVar(&cfg.MinLength, "min-length", 8, "Minimum candidate length (discard shorter)")
	flag.Int64Var(&cfg.MaxCandidates, "max", 0, "Maximum candidates to generate (0 = unlimited)")

	// ── Disable transforms ──
	flag.BoolVar(&noCaseFlag, "no-case", false, "Disable case transformations")
	flag.BoolVar(&noLeetFlag, "no-leet", false, "Disable l33t speak transformations")
	flag.BoolVar(&noAppendFlag, "no-append", false, "Disable append transformations")
	flag.BoolVar(&noPrependFlag, "no-prepend", false, "Disable prepend transformations")
	flag.BoolVar(&noCombineFlag, "no-combine", false, "Disable combination chains")

	// ── Tuning ──
	flag.IntVar(&cfg.LeetMax, "leet-max", 0, "Max leet variants per word (0 = use preset default)")
	flag.BoolVar(&cfg.Verbose, "v", false, "Show progress during generation")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool -i <input> [options]

Generates optimized wordlists for hash cracking from keywords.

Options:
  -i  <file>        Input keyword file (required)
  -o  <file>        Output wordlist file (default: wordlist.txt)
  -t  <n>           Worker threads (default: %d)
  -preset <preset>  Generation preset: short, standard, long (default: standard)
  -min-length <n>   Minimum candidate length (default: 8)
  -max <n>          Maximum candidates, 0 = unlimited (default: 0)
  -v                Show progress during generation

  -no-case          Disable case transformations
  -no-leet          Disable l33t speak transformations
  -no-append        Disable append transformations
  -no-prepend       Disable prepend transformations
  -no-combine       Disable combination chains

  -leet-max <n>     Max leet variants per word (0 = use preset default)
`, runtime.NumCPU())
	}

	flag.Parse()

	// ── Validate input ──
	if cfg.InputFile == "" {
		return nil, fmt.Errorf("input file is required (-i <file>)")
	}
	if _, err := os.Stat(cfg.InputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file does not exist: %s", cfg.InputFile)
	}
	if cfg.Threads < 1 {
		cfg.Threads = 1
	}

	// ── Parse preset ──
	preset, err := ParsePreset(presetStr)
	if err != nil {
		return nil, err
	}
	cfg.Preset = preset

	// ── Apply preset defaults for LeetMax if not explicitly set ──
	if cfg.LeetMax == 0 {
		switch cfg.Preset {
		case PresetShort:
			cfg.LeetMax = 128
		case PresetStandard:
			cfg.LeetMax = 512
		case PresetLong:
			cfg.LeetMax = 2048
		}
	}

	// ── Invert no-* flags ──
	cfg.EnableCase = !noCaseFlag
	cfg.EnableLeet = !noLeetFlag
	cfg.EnableAppend = !noAppendFlag
	cfg.EnablePrepend = !noPrependFlag
	cfg.EnableCombine = !noCombineFlag

	return cfg, nil
}
