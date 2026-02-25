package main

import (
	"fmt"
	"os"

	"github.com/Xre0uS/wordlisttool/config"
	"github.com/Xre0uS/wordlisttool/generator"
	"github.com/Xre0uS/wordlisttool/transform"
	"github.com/Xre0uS/wordlisttool/wordlists"
)

func main() {
	fmt.Fprint(os.Stderr, `
 █░█░█ █▀█ █▀█ █▀▄ █░░ █ █▀ ▀█▀ ▀█▀ █▀█ █▀█ █░░
 ▀▄▀▄▀ █▄█ █▀▄ █▄▀ █▄▄ █ ▄█ ░█░ ░█░ █▄█ █▄█ █▄▄
                Advanced Wordlist Generator
`)

	// ── Subcommand dispatch ──
	if len(os.Args) > 1 && os.Args[1] == "merge" {
		runMerge(os.Args[2:])
		return
	}

	// Default: generate mode
	runGenerate()
}

// ---------------------------------------------------------------
// merge subcommand
// ---------------------------------------------------------------

func runMerge(args []string) {
	outputFile := "merged.txt"
	noConfirm := false
	var inputFiles []string

	usage := func() {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool merge [options] <file1> <file2> [file3 ...]

Merge multiple wordlists into one file, preserving order.
Lists are concatenated as-is: file1 first, then file2, etc.
Shows total size and asks for confirmation before writing.

Flags can appear anywhere — before, between, or after file arguments.

Options:
  -o <file>   Output file (default: merged.txt)
  -y          Skip confirmation prompt
  -h          Show this help

Example:
  wordlisttool merge list1.txt list2.txt list3.txt -o combined.txt
`)
	}

	// Manual parse so flags and files can be intermixed
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "[!] Error: -o requires a filename")
				os.Exit(1)
			}
			i++
			outputFile = args[i]
		case "-y":
			noConfirm = true
		case "-h", "--help":
			usage()
			os.Exit(0)
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				fmt.Fprintf(os.Stderr, "[!] Error: unknown flag: %s\n", args[i])
				usage()
				os.Exit(1)
			}
			inputFiles = append(inputFiles, args[i])
		}
	}

	if len(inputFiles) < 2 {
		fmt.Fprintln(os.Stderr, "[!] Error: merge requires at least 2 input files")
		usage()
		os.Exit(1)
	}

	cfg := &generator.MergeConfig{
		InputFiles: inputFiles,
		OutputFile: outputFile,
		NoConfirm:  noConfirm,
	}

	if err := generator.RunMerge(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------
// generate (default) subcommand
// ---------------------------------------------------------------

func runGenerate() {
	cfg, err := config.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		os.Exit(1)
	}

	transformers := buildTransformers(cfg)

	if len(transformers) == 0 {
		fmt.Fprintln(os.Stderr, "[!] No transformations enabled. Enable at least one.")
		os.Exit(1)
	}

	pipeline := generator.NewPipeline(cfg, transformers)
	if err := pipeline.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------
// Preset-aware leet budgets
// ---------------------------------------------------------------

func leetBudget(cfg *config.Config, short, standard, long int) int {
	switch cfg.Preset {
	case config.PresetShort:
		return short
	case config.PresetLong:
		return long
	default:
		return standard
	}
}

func capBudget(cfg *config.Config, short, standard, long int) int {
	switch cfg.Preset {
	case config.PresetShort:
		return short
	case config.PresetLong:
		return long
	default:
		return standard
	}
}

// ---------------------------------------------------------------
// Build transformer list based on config + preset
// ---------------------------------------------------------------

func buildTransformers(cfg *config.Config) []transform.Transformer {
	var transformers []transform.Transformer

	// ── Stage 1: Base variations (case + leet + mangle) ──
	var base []transform.Transformer
	if cfg.EnableCase {
		base = append(base, &transform.CaseTransformer{})
	}
	if cfg.EnableLeet {
		base = append(base, &transform.LeetTransformer{MaxVariants: cfg.LeetMax})
	}
	base = append(base, &transform.MangleTransformer{})
	transformers = append(transformers, &transform.MergeTransformer{Transformers: base})

	// ── Stage 2: Direct append / prepend ──
	if cfg.EnableAppend {
		transformers = append(transformers, &transform.AppendTransformer{})
	}
	if cfg.EnablePrepend {
		transformers = append(transformers, &transform.PrependTransformer{})
	}

	// ── Stage 3: Combination chains ──
	if cfg.EnableCombine {
		addCombineChains(cfg, &transformers)
	}

	return transformers
}

func addCombineChains(cfg *config.Config, out *[]transform.Transformer) {
	add := func(t transform.Transformer) { *out = append(*out, t) }

	isLong := cfg.Preset == config.PresetLong

	// ── Case → Append (full) — all presets ──
	// Case ~12 variants × ~10K suffixes ≈ 120K output — safe even unlimited
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.AppendTransformer{},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	// ── Case → Prepend (full) — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.PrependTransformer{},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	// ── Leet → Append — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 200, 500, 1000)},
			&transform.AppendTransformer{},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	// ── Leet → Prepend — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 200, 500, 1000)},
			&transform.PrependTransformer{},
		},
		MaxPerWord: capBudget(cfg, 5_000_000, 5_000_000, 10_000_000),
	})

	// ── Case → Leet → Append sym+year — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 50, 200, 500)},
			&transform.AppendTransformer{Suffixes: buildSymbolYearSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 500, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	// ── Case → Leet → Prepend sym+num — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 50, 200, 500)},
			&transform.PrependTransformer{Prefixes: buildSymbolNumberPrefixes()},
		},
		MaxIntermediate: capBudget(cfg, 500, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 5_000_000, 5_000_000, 10_000_000),
	})

	// ── Mangle → Append — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.MangleTransformer{},
			&transform.AppendTransformer{Suffixes: buildCombineSuffixes()},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	// ── Mangle → Leet → Append — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.MangleTransformer{},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 50, 100, 300)},
			&transform.AppendTransformer{Suffixes: buildCombineSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 3_000_000, 3_000_000, 8_000_000),
	})

	// ── Mangle → Case → Append — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.MangleTransformer{},
			&transform.CaseTransformer{},
			&transform.AppendTransformer{Suffixes: buildCombineSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 3_000_000, 3_000_000, 8_000_000),
	})

	// ── Prepend → Case → Append (sandwich) — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.PrependTransformer{Prefixes: buildSandwichPrefixes()},
			&transform.CaseTransformer{},
			&transform.AppendTransformer{Suffixes: buildSandwichSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 2000, 5000, 10000),
		MaxPerWord:      capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	// ── Prepend num → Leet → Append sym — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.PrependTransformer{Prefixes: buildNumberPrefixes()},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 30, 50, 200)},
			&transform.AppendTransformer{Suffixes: buildSymbolSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 3_000_000, 3_000_000, 8_000_000),
	})

	// ── Case → Append word → Append num+sym — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.AppendTransformer{Suffixes: wordlists.CommonSuffixWords},
			&transform.AppendTransformer{Suffixes: buildNumberAndSymbolSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 3000, 5000),
		MaxPerWord:      capBudget(cfg, 1_000_000, 3_000_000, 8_000_000),
	})

	// ── Leet → Append double (sym+year, year+sym) — all presets ──
	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 100, 200, 500)},
			&transform.AppendTransformer{Suffixes: buildDoubleAppendSuffixes()},
		},
		MaxPerWord: capBudget(cfg, 5_000_000, 5_000_000, 10_000_000),
	})

	// ── Case → Leet → Prepend → Append (4-stage) — long only ──
	if isLong {
		add(&transform.CombineTransformer{
			Stages: []transform.Transformer{
				&transform.CaseTransformer{},
				&transform.LeetTransformer{MaxVariants: 50},
				&transform.PrependTransformer{
					Prefixes: []string{
						"!", "@", "#", "$", "123", "1234", "!!", "!@#",
						"1", "2", "3", "12", "69", "420", "666", "777",
					},
				},
				&transform.AppendTransformer{
					Suffixes: []string{
						"!", "@", "#", "$", "1", "12", "123", "1234",
						"2024", "2025", "2026", "2027",
						"!!", "!@#", "@!", "#!", "!!!", "!@#$",
					},
				},
			},
			MaxIntermediate: 5000,
			MaxPerWord:      5_000_000,
		})
	}
}

// ---------------------------------------------------------------
// Suffix / prefix builders
// ---------------------------------------------------------------

func buildCombineSuffixes() []string {
	var s []string
	s = append(s, wordlists.SymbolsFull...)
	s = append(s, wordlists.NumberSequences...)
	s = append(s, wordlists.Years...)
	s = append(s, wordlists.Digits...)
	s = append(s, wordlists.TwoDigits...)
	s = append(s, wordlists.ThreeDigits...)
	s = append(s, wordlists.KeyboardWalks...)
	s = append(s, wordlists.CommonPatterns...)
	for _, y := range wordlists.Years {
		for _, sym := range wordlists.SymbolsShort {
			s = append(s, y+sym, sym+y)
		}
	}
	return s
}

func buildSymbolYearSuffixes() []string {
	var s []string
	for _, sym := range wordlists.SymbolsFull {
		for _, y := range wordlists.Years {
			s = append(s, sym+y, y+sym)
		}
		s = append(s, sym)
	}
	s = append(s, wordlists.Years...)
	for _, s1 := range wordlists.SymbolsShort {
		for _, s2 := range wordlists.SymbolsShort {
			for _, n := range []string{"1", "12", "123", "1234", "2025", "2026", "2027"} {
				s = append(s, s1+n+s2)
			}
		}
	}
	s = append(s, wordlists.CommonPatterns...)
	return s
}

func buildSymbolNumberPrefixes() []string {
	var p []string
	p = append(p, wordlists.SymbolsFull...)
	p = append(p, wordlists.NumberSequences...)
	for _, sym := range wordlists.SymbolsShort {
		for _, n := range wordlists.Digits {
			p = append(p, sym+n, n+sym)
		}
		for _, y := range wordlists.Years {
			p = append(p, sym+y, y+sym)
		}
	}
	p = append(p, wordlists.CommonPatterns...)
	return p
}

func buildSandwichPrefixes() []string {
	var p []string
	p = append(p, wordlists.SymbolsShort...)
	p = append(p, wordlists.Digits...)
	p = append(p, wordlists.NumberSequences...)
	p = append(p, wordlists.Years...)
	p = append(p, wordlists.CommonPrefixWords...)
	for _, sym := range wordlists.SymbolsShort {
		for _, n := range wordlists.Digits {
			p = append(p, n+sym, sym+n)
		}
	}
	return p
}

func buildSandwichSuffixes() []string {
	var s []string
	s = append(s, wordlists.SymbolsShort...)
	s = append(s, wordlists.Digits...)
	s = append(s, wordlists.NumberSequences...)
	s = append(s, wordlists.Years...)
	for _, sym := range wordlists.SymbolsShort {
		for _, n := range wordlists.Digits {
			s = append(s, n+sym, sym+n)
		}
	}
	for _, y := range wordlists.Years {
		for _, sym := range wordlists.SymbolsShort {
			s = append(s, y+sym)
		}
	}
	return s
}

func buildNumberPrefixes() []string {
	var p []string
	p = append(p, wordlists.Digits...)
	p = append(p, wordlists.TwoDigits...)
	p = append(p, wordlists.NumberSequences...)
	p = append(p, wordlists.Years...)
	p = append(p, wordlists.FourDigits...)
	return p
}

func buildSymbolSuffixes() []string {
	var s []string
	s = append(s, wordlists.SymbolsFull...)
	s = append(s, wordlists.CommonPatterns...)
	return s
}

func buildNumberAndSymbolSuffixes() []string {
	var s []string
	s = append(s, wordlists.Digits...)
	s = append(s, wordlists.TwoDigits...)
	s = append(s, wordlists.NumberSequences...)
	s = append(s, wordlists.Years...)
	for _, sym := range wordlists.SymbolsShort {
		s = append(s, sym)
		for _, d := range wordlists.Digits {
			s = append(s, sym+d)
		}
	}
	return s
}

func buildDoubleAppendSuffixes() []string {
	var s []string
	for _, sym := range wordlists.SymbolsFull {
		for _, y := range wordlists.Years {
			s = append(s, sym+y, y+sym)
		}
		for _, n := range wordlists.NumberSequences {
			s = append(s, sym+n, n+sym)
		}
	}
	for _, y := range []string{"2024", "2025", "2026", "2027"} {
		for _, sym := range wordlists.SymbolsShort {
			for _, d := range wordlists.Digits {
				s = append(s, y+sym+d, sym+y+d)
			}
		}
	}
	for _, s1 := range wordlists.SymbolsShort {
		for _, s2 := range wordlists.SymbolsShort {
			if s1 != s2 {
				s = append(s, s1+s2)
			}
		}
	}
	s = append(s, wordlists.CommonPatterns...)
	return s
}
