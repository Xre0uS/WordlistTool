package main

import (
	"fmt"
	"os"
	"runtime"

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

	if len(os.Args) <= 1 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "estimate":
		runEstimate(os.Args[2:])
	case "generate":
		runGenerate(os.Args[2:])
	case "merge":
		runMerge(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "[!] Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  wordlisttool estimate <keywords.txt>              Estimate output per preset (run first)
  wordlisttool generate <keywords.txt> [options]    Generate wordlist from keywords
  wordlisttool merge <files...> [options]            Merge multiple wordlists into one

━━━ Estimate ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool estimate <keywords.txt>

  Dry-run: estimates candidate counts, output file size, and memory
  usage for all three presets. Run this first to size your job.

━━━ Generate ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool generate <keywords.txt> [options]

  -o  <file>        Output wordlist file (default: wordlist.txt)
  -t  <n>           Worker threads (default: CPU count)
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

━━━ Merge ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool merge <file1> <file2> [file3 ...] [options]

  -o <file>         Output file (default: merged.txt)
  -y                Skip confirmation prompt

  Concatenates wordlists in order. Shows total size and word count.
  Output file must not already exist.

━━━ Examples ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool estimate keywords.txt
  wordlisttool generate keywords.txt -o wordlist.txt -preset long -v
  wordlisttool merge list1.txt list2.txt list3.txt -o combined.txt
`)
}

// ---------------------------------------------------------------
// estimate subcommand
// ---------------------------------------------------------------

func runEstimate(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool estimate <keywords.txt>

Estimate candidate counts for all three presets without generating output.
Useful for sizing a run before committing disk space and time.
`)
		os.Exit(0)
	}

	inputFile := args[0]
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] Error: file not found: %s\n", inputFile)
		os.Exit(1)
	}

	keywords, err := generator.ReadKeywords(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error reading keywords: %v\n", err)
		os.Exit(1)
	}
	if len(keywords) == 0 {
		fmt.Fprintln(os.Stderr, "[!] Error: no keywords found in input file")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[*] Loaded %d keywords: ", len(keywords))
	for i, kw := range keywords {
		if i > 0 {
			fmt.Fprint(os.Stderr, ", ")
		}
		fmt.Fprint(os.Stderr, kw)
	}
	fmt.Fprintln(os.Stderr)

	threads := runtime.NumCPU()
	minLen := 8

	presets := []struct {
		name    string
		preset  config.Preset
		leetMax int
	}{
		{"short", config.PresetShort, 128},
		{"standard", config.PresetStandard, 512},
		{"long", config.PresetLong, 2048},
	}

	var results []generator.EstimateResult
	for _, p := range presets {
		fmt.Fprintf(os.Stderr, "[*] Estimating %s preset ...\n", p.name)

		cfg := &config.Config{
			Preset:        p.preset,
			LeetMax:       p.leetMax,
			MinLength:     minLen,
			EnableCase:    true,
			EnableLeet:    true,
			EnableAppend:  true,
			EnablePrepend: true,
			EnableCombine: true,
		}
		transformers := buildTransformers(cfg)
		result := generator.RunEstimate(keywords, transformers, p.name, threads, minLen)
		results = append(results, result)
	}

	generator.PrintEstimateTable(results, keywords)
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

func runGenerate(args []string) {
	cfg, err := config.Parse(args)
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

	// ── Case → Append(word+pattern) → Prepend(short) — long only ──
	// Produces: 0SGEadmin!@#, 1SGEadmin!@#, !SGEpassword2026, etc.
	if isLong || cfg.Preset == config.PresetStandard {
		shortPrepends := []string{
			"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
			"!", "@", "#", "$", "*",
		}
		if isLong {
			shortPrepends = append(shortPrepends,
				"00", "01", "10", "11", "12", "13", "21", "23", "42", "69",
				"!!", "@@", "##", "!@", "@!", "#!",
				"123", "1234", "!@#", "!@#$",
				"420", "666", "777", "000",
			)
		}
		// Case → Append(word+sym) → Prepend(short digit/sym)
		// Intermediates: 12 case × ~18K suffixes — need high cap to preserve variety
		add(&transform.CombineTransformer{
			Stages: []transform.Transformer{
				&transform.CaseTransformer{},
				&transform.AppendTransformer{Suffixes: buildWordPatternSuffixes()},
				&transform.PrependTransformer{Prefixes: shortPrepends},
			},
			MaxIntermediate: capBudget(cfg, 0, 50_000, 200_000),
			MaxPerWord:      capBudget(cfg, 0, 3_000_000, 8_000_000),
		})

		// Case → Append(sym+year) → Prepend(short digit/sym)
		add(&transform.CombineTransformer{
			Stages: []transform.Transformer{
				&transform.CaseTransformer{},
				&transform.AppendTransformer{Suffixes: buildSymbolYearSuffixes()},
				&transform.PrependTransformer{Prefixes: shortPrepends},
			},
			MaxIntermediate: capBudget(cfg, 0, 50_000, 200_000),
			MaxPerWord:      capBudget(cfg, 0, 3_000_000, 8_000_000),
		})
	}

	// ── Leet → Append(sym+year) → Prepend(short) — long only ──
	if isLong {
		add(&transform.CombineTransformer{
			Stages: []transform.Transformer{
				&transform.LeetTransformer{MaxVariants: 200},
				&transform.AppendTransformer{Suffixes: buildSymbolYearSuffixes()},
				&transform.PrependTransformer{Prefixes: []string{
					"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
					"!", "@", "#", "$", "!!", "!@#",
					"00", "12", "69", "123", "1234",
				}},
			},
			MaxIntermediate: 200_000,
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

// buildWordPatternSuffixes builds word + symbol/number combo suffixes
// e.g. "admin!@#", "password2026", "admin!@#0", "secure!", "admin123!"
func buildWordPatternSuffixes() []string {
	var s []string
	syms := []string{"!", "@", "#", "$", "!!", "!@#", "!@#$", "!@#$%", "@!", "#!"}
	nums := []string{"0", "1", "2", "12", "123", "1234", "2024", "2025", "2026", "2027"}

	for _, w := range wordlists.CommonSuffixWords {
		// word alone
		s = append(s, w)
		// word + symbol
		for _, sym := range syms {
			s = append(s, w+sym)
		}
		// word + number
		for _, n := range nums {
			s = append(s, w+n)
		}
		// word + symbol + number
		for _, sym := range []string{"!", "@", "#"} {
			for _, n := range []string{"0", "1", "2", "123", "2026", "2027"} {
				s = append(s, w+sym+n)
				s = append(s, w+n+sym)
			}
		}
		// word + number + symbol combo
		for _, p := range wordlists.CommonPatterns[:min(len(wordlists.CommonPatterns), 50)] {
			s = append(s, w+p)
		}
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
