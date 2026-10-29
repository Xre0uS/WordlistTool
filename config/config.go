package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
)

type Preset int

const (
	PresetShort Preset = iota
	PresetStandard
	PresetLong
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

type Config struct {
	InputFile  string
	OutputFile string
	ConfigFile string
	Threads    int
	Preset     Preset
	MinLength  int

	EnableCase    bool
	EnableLeet    bool
	EnableAppend  bool
	EnablePrepend bool
	EnableCombine bool

	LeetMax       int
	MaxCandidates int64
	Verbose       bool

	DedupeMode  string // fingerprint, full or approx — parsed by the caller
	DedupeMemMB int    // memory ceiling for the dedupe structure, 0 to derive it
	NoMemCap    bool   // dedupe without a ceiling at all
}

func Parse(args []string) (*Config, error) {
	cfg := &Config{}

	var presetStr string
	var noCaseFlag, noLeetFlag, noAppendFlag, noPrependFlag, noCombineFlag bool

	fs := flag.NewFlagSet("generate", flag.ContinueOnError)

	fs.StringVar(&cfg.OutputFile, "o", "wordlist.txt", "")
	fs.StringVar(&cfg.ConfigFile, "config", "", "")
	fs.IntVar(&cfg.Threads, "t", runtime.NumCPU(), "")
	fs.StringVar(&presetStr, "preset", "standard", "")
	fs.IntVar(&cfg.MinLength, "min-length", 8, "")
	fs.Int64Var(&cfg.MaxCandidates, "max", 0, "")
	fs.BoolVar(&noCaseFlag, "no-case", false, "")
	fs.BoolVar(&noLeetFlag, "no-leet", false, "")
	fs.BoolVar(&noAppendFlag, "no-append", false, "")
	fs.BoolVar(&noPrependFlag, "no-prepend", false, "")
	fs.BoolVar(&noCombineFlag, "no-combine", false, "")
	fs.IntVar(&cfg.LeetMax, "leet-max", 0, "")
	fs.StringVar(&cfg.DedupeMode, "dedupe", "approx", "")
	fs.IntVar(&cfg.DedupeMemMB, "dedupe-mem", 0, "")
	fs.BoolVar(&cfg.NoMemCap, "no-mem-cap", false, "")
	fs.BoolVar(&cfg.Verbose, "v", false, "")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool generate <keywords.txt> [options]

Options:
  -o  <file>        Output file (default: wordlist.txt)
  -t  <n>           Worker threads (default: %d)
  -preset <preset>  Preset: short, standard, long (default: standard)
  -min-length <n>   Minimum candidate length (default: 8)
  -max <n>          Max candidates, 0 = unlimited (default: 0)
  -config <file>    Data config TOML (default: config.toml)
  -v                Show progress

  -no-case          Disable case transforms
  -no-leet          Disable l33t speak
  -no-append        Disable append
  -no-prepend       Disable prepend
  -no-combine       Disable combination chains
  -leet-max <n>     Max leet variants per word (0 = preset default)

  -dedupe <mode>    Duplicate removal: approx (default), fingerprint, full
                    approx is memory-capped but drops a few unique candidates
                    as false dupes; the other two drop none
  -dedupe-mem <MB>  Memory ceiling for the dedupe structure. Unset, it is 75%% of
                    the memory free at the time; approx sizes its filter to this
                    (512 MB when unset and there is room)
  -no-mem-cap       Dedupe without a memory ceiling
`, runtime.NumCPU())
	}

	valueFlags := map[string]bool{
		"-o": true, "-t": true, "-preset": true,
		"-min-length": true, "-max": true, "-leet-max": true,
		"-config": true, "-dedupe": true, "-dedupe-mem": true, "-no-mem-cap": true,
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

	preset, err := ParsePreset(presetStr)
	if err != nil {
		return nil, err
	}
	cfg.Preset = preset

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

	cfg.EnableCase = !noCaseFlag
	cfg.EnableLeet = !noLeetFlag
	cfg.EnableAppend = !noAppendFlag
	cfg.EnablePrepend = !noPrependFlag
	cfg.EnableCombine = !noCombineFlag

	return cfg, nil
}
