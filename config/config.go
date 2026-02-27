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

// Parse parses the given args into a Config.
// The keyword file is a positional argument and can appear anywhere among the flags.
func Parse(args []string) (*Config, error) {
	cfg := &Config{}

	var presetStr string
	var noCaseFlag, noLeetFlag, noAppendFlag, noPrependFlag, noCombineFlag bool

	fs := flag.NewFlagSet("generate", flag.ContinueOnError)

	// ── I/O ──
	fs.StringVar(&cfg.OutputFile, "o", "wordlist.txt", "Output wordlist file")
	fs.IntVar(&cfg.Threads, "t", runtime.NumCPU(), "Worker threads")

	// ── Preset ──
	fs.StringVar(&presetStr, "preset", "standard", "Generation preset: short, standard, long")

	// ── Filters ──
	fs.IntVar(&cfg.MinLength, "min-length", 8, "Minimum candidate length (discard shorter)")
	fs.Int64Var(&cfg.MaxCandidates, "max", 0, "Maximum candidates to generate (0 = unlimited)")

	// ── Disable transforms ──
	fs.BoolVar(&noCaseFlag, "no-case", false, "Disable case transformations")
	fs.BoolVar(&noLeetFlag, "no-leet", false, "Disable l33t speak transformations")
	fs.BoolVar(&noAppendFlag, "no-append", false, "Disable append transformations")
	fs.BoolVar(&noPrependFlag, "no-prepend", false, "Disable prepend transformations")
	fs.BoolVar(&noCombineFlag, "no-combine", false, "Disable combination chains")

	// ── Tuning ──
	fs.IntVar(&cfg.LeetMax, "leet-max", 0, "Max leet variants per word (0 = use preset default)")
	fs.BoolVar(&cfg.Verbose, "v", false, "Show progress during generation")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool generate <keywords.txt> [options]

Generate optimized wordlists for hash cracking from keywords.
Run 'wordlisttool -h' for full help including all modes.

Options:
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

	// ── Extract positional file from args, pass remaining to flag parser ──
	// Flags that take a value: -o, -t, -preset, -min-length, -max, -leet-max
	valueFlags := map[string]bool{
		"-o": true, "-t": true, "-preset": true,
		"-min-length": true, "-max": true, "-leet-max": true,
	}
	var flagArgs []string
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-h" || a == "--help" {
			fs.Usage()
			os.Exit(0)
		}
		if len(a) > 0 && a[0] == '-' {
			flagArgs = append(flagArgs, a)
			if valueFlags[a] && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, a)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}

	// ── Input file is the positional argument ──
	if len(positional) == 0 {
		fs.Usage()
		return nil, fmt.Errorf("input keyword file is required")
	}
	cfg.InputFile = positional[0]

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
