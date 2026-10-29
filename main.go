package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"git.xreous.io/xreous/WordlistTool/config"
	"git.xreous.io/xreous/WordlistTool/generator"
	"git.xreous.io/xreous/WordlistTool/transform"
	"git.xreous.io/xreous/WordlistTool/wordlists"
)

const defaultConfigFile = "config.toml"

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
	case "diff":
		runDiff(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "[!] Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  wordlisttool estimate <keywords.txt> [options]    Estimate output per preset
  wordlisttool generate <keywords.txt> [options]    Generate wordlist from keywords
  wordlisttool merge <files...> [options]            Merge multiple wordlists
  wordlisttool diff <new...> -x <tested...> [options]  Strip already-tested words

━━━ Estimate ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool estimate <keywords.txt> [-config <file>]

  Dry-run: estimates candidate counts, output file size, and memory
  usage for all three presets. Run this first to size your job.

━━━ Generate ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool generate <keywords.txt> [options]

  -o  <file>        Output file (default: wordlist.txt)
  -t  <n>           Worker threads (default: CPU count)
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
  -dedupe-mem <MB>  Memory cap for -dedupe approx (default: 512)

━━━ Merge ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool merge <file1> <file2> [file3 ...] [options]

  -o <file>         Output file (default: merged.txt)
  -y                Skip confirmation prompt

━━━ Diff ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool diff <new.txt> [new2.txt ...] -x <tested.txt> [options]

  A wordlist is exhausted and the next one is going in. Strips every
  word the previous wordlists already covered and writes what is left
  to a new file, in input order.

  -x <file>         Previously tested wordlist (repeat for more)
  -o <file>         Output file (default: remaining.txt)
  -f                Overwrite the output file if it exists
  -y                Skip confirmation prompt
  -v                Show progress
  -keep-dupes       Keep repeats within the new wordlist(s)

  -mode <mode>      fingerprint (default), full, or approx — see -h
  -mem <MB>         Memory ceiling for the run (default: 75%% of free memory)
  -no-mem-cap       Plan without a memory ceiling
  -fpr <rate>       False-positive rate for -mode approx (default: 0.0001)

  -sorted           Take the merge-join fast path for one new wordlist:
                    exact, constant memory, needs the input and every -x
                    file sorted in byte order (LC_ALL=C)
  -no-sorted        Never take the fast path

━━━ Examples ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  wordlisttool estimate keywords.txt
  wordlisttool generate keywords.txt -o wordlist.txt -preset long -v
  wordlisttool generate keywords.txt -config custom.toml -preset short
  wordlisttool merge list1.txt list2.txt list3.txt -o combined.txt
  wordlisttool diff new.txt -x rockyou.txt -x run1.txt -o remaining.txt -v
  wordlisttool diff big.txt -x tested.txt -mode approx -mem 2048 -y
  wordlisttool diff new.txt -x tested.txt -sorted -y
`)
}

func loadConfig(path string) {
	if path == "" {
		path = defaultConfigFile
	}
	if err := wordlists.LoadFromTOML(path); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error loading %s: %v\n", path, err)
		os.Exit(1)
	}
}

func runEstimate(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool estimate <keywords.txt> [-config <file>]

Estimate candidate counts for all three presets without generating output.

Options:
  -config <file>  Data config TOML (default: config.toml)
`)
		os.Exit(0)
	}

	var inputFile, configFile string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-config":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "[!] Error: -config requires a filename")
				os.Exit(1)
			}
			i++
			configFile = args[i]
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				fmt.Fprintf(os.Stderr, "[!] Error: unknown flag: %s\n", args[i])
				os.Exit(1)
			}
			if inputFile == "" {
				inputFile = args[i]
			}
		}
	}
	if inputFile == "" {
		fmt.Fprintln(os.Stderr, "[!] Error: keyword file is required")
		os.Exit(1)
	}
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] Error: file not found: %s\n", inputFile)
		os.Exit(1)
	}

	loadConfig(configFile)

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

func runMerge(args []string) {
	outputFile := "merged.txt"
	noConfirm := false
	var inputFiles []string

	usage := func() {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool merge [options] <file1> <file2> [file3 ...]

Merge multiple wordlists into one file, preserving order.

Options:
  -o <file>   Output file (default: merged.txt)
  -y          Skip confirmation prompt
  -h          Show this help
`)
	}

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

func runDiff(args []string) {
	cfg := &generator.DiffConfig{
		OutputFile: "remaining.txt",
		FPR:        0.0001,
	}

	usage := func() {
		fmt.Fprintf(os.Stderr, `Usage: wordlisttool diff <new.txt> [new2.txt ...] -x <tested.txt> [options]

Strip every word the already-tested wordlists cover from the new wordlist(s)
and write what is left to a new file, in input order.

Options:
  -x <file>      Previously tested wordlist (repeat for more)
  -o <file>      Output file (default: remaining.txt)
  -f             Overwrite the output file if it exists
  -y             Skip confirmation prompt
  -v             Show progress
  -keep-dupes    Keep repeats within the new wordlist(s)
  -h             Show this help

  -mode <mode>   fingerprint  63-bit fingerprints, ~16 B/word, small collision
                              risk that drops an untested word (default)
                 full         whole words, no collision risk, ~16 B/word plus
                              the word itself
                 approx       bloom filter, memory capped by -mem, drops
                              untested words at the -fpr rate
  -mem <MB>      Memory ceiling for the run. Unset, it is 75%% of the memory
                 free at the time, and a mode that would plan past it is
                 refused before it allocates. approx sizes its filter to this
                 (1024 MB when unset and there is room)
  -no-mem-cap    Plan without a ceiling
  -fpr <rate>    False-positive rate for -mode approx (default: 0.0001)

  -sorted        Take the merge-join fast path, instead of a filter: the
                 exact difference of one new wordlist and the -x files,
                 streamed in constant memory. Needs the input and every
                 -x file sorted in byte order (LC_ALL=C); a file sorted
                 under another locale is not sorted for this purpose.
                 The order required is the order of words after a trailing
                 CR is stripped, so a file with CR-terminated lines may need
                 them removed first.
                 By default the heads are probed and the path is taken
                 only when every sample is sorted; use -no-sorted to
                 forbid it and fall back to -mode.

Examples:
  wordlisttool diff new.txt -x rockyou.txt -o remaining.txt -v
  wordlisttool diff new1.txt new2.txt -x run1.txt -x run2.txt -y
  wordlisttool diff new.txt -x tested.txt -mode full -v
  wordlisttool diff huge.txt -x tested.txt -mode approx -mem 2048
  wordlisttool diff new.txt -x tested.txt -sorted -y
`)
	}

	needValue := func(i int, flagName string) string {
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr, "[!] Error: %s requires a value\n", flagName)
			os.Exit(1)
		}
		return args[i+1]
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			cfg.OutputFile = needValue(i, "-o")
			i++
		case "-x", "-exclude", "--exclude":
			cfg.ExcludeFiles = append(cfg.ExcludeFiles, needValue(i, args[i]))
			i++
		case "-mem":
			v := needValue(i, "-mem")
			i++
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				fmt.Fprintf(os.Stderr, "[!] Error: -mem needs a positive integer (MB), got %q\n", v)
				os.Exit(1)
			}
			cfg.MemLimitMB = n
		case "-fpr":
			v := needValue(i, "-fpr")
			i++
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f <= 0 || f >= 1 {
				fmt.Fprintf(os.Stderr, "[!] Error: -fpr needs a rate between 0 and 1, got %q\n", v)
				os.Exit(1)
			}
			cfg.FPR = f
		case "-mode":
			v := needValue(i, "-mode")
			i++
			mode, err := generator.ParseDiffMode(v)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
				os.Exit(1)
			}
			cfg.Mode = mode
		case "-sorted":
			cfg.Sorted = generator.SortedForce
		case "-no-sorted":
			cfg.Sorted = generator.SortedOff
		case "-no-mem-cap":
			cfg.NoMemCap = true
		case "-keep-dupes":
			cfg.KeepDupes = true
		case "-f":
			cfg.Force = true
		case "-y":
			cfg.NoConfirm = true
		case "-v":
			cfg.Verbose = true
		case "-h", "--help":
			usage()
			os.Exit(0)
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				fmt.Fprintf(os.Stderr, "[!] Error: unknown flag: %s\n", args[i])
				usage()
				os.Exit(1)
			}
			cfg.InputFiles = append(cfg.InputFiles, args[i])
		}
	}

	if len(cfg.InputFiles) == 0 {
		fmt.Fprintln(os.Stderr, "[!] Error: at least one new wordlist is required")
		usage()
		os.Exit(1)
	}
	if len(cfg.ExcludeFiles) == 0 {
		fmt.Fprintln(os.Stderr, "[!] Error: at least one tested wordlist is required (-x)")
		usage()
		os.Exit(1)
	}

	if err := generator.RunDiff(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		os.Exit(1)
	}
}

func runGenerate(args []string) {
	cfg, err := config.Parse(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		os.Exit(1)
	}

	dedupe, err := generator.ParseDiffMode(cfg.DedupeMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: -dedupe: %v\n", err)
		os.Exit(1)
	}

	loadConfig(cfg.ConfigFile)

	transformers := buildTransformers(cfg)

	if len(transformers) == 0 {
		fmt.Fprintln(os.Stderr, "[!] No transformations enabled. Enable at least one.")
		os.Exit(1)
	}

	pipeline := generator.NewPipeline(cfg, transformers)
	pipeline.Dedupe = dedupe
	if err := pipeline.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		os.Exit(1)
	}
}

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

func buildTransformers(cfg *config.Config) []transform.Transformer {
	var transformers []transform.Transformer

	var base []transform.Transformer
	if cfg.EnableCase {
		base = append(base, &transform.CaseTransformer{})
	}
	if cfg.EnableLeet {
		base = append(base, &transform.LeetTransformer{MaxVariants: cfg.LeetMax})
	}
	base = append(base, &transform.MangleTransformer{})
	transformers = append(transformers, &transform.MergeTransformer{Transformers: base})

	if cfg.EnableAppend {
		transformers = append(transformers, &transform.AppendTransformer{})
	}
	if cfg.EnablePrepend {
		transformers = append(transformers, &transform.PrependTransformer{})
	}

	if cfg.EnableCombine {
		addCombineChains(cfg, &transformers)
	}

	return transformers
}

func addCombineChains(cfg *config.Config, out *[]transform.Transformer) {
	add := func(t transform.Transformer) { *out = append(*out, t) }
	isLong := cfg.Preset == config.PresetLong

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.AppendTransformer{},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.PrependTransformer{},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 200, 500, 1000)},
			&transform.AppendTransformer{},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 200, 500, 1000)},
			&transform.PrependTransformer{},
		},
		MaxPerWord: capBudget(cfg, 5_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 50, 200, 500)},
			&transform.AppendTransformer{Suffixes: buildSymbolYearSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 500, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 50, 200, 500)},
			&transform.PrependTransformer{Prefixes: buildSymbolNumberPrefixes()},
		},
		MaxIntermediate: capBudget(cfg, 500, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 5_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.MangleTransformer{},
			&transform.AppendTransformer{Suffixes: buildCombineSuffixes()},
		},
		MaxPerWord: capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.MangleTransformer{},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 50, 100, 300)},
			&transform.AppendTransformer{Suffixes: buildCombineSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 3_000_000, 3_000_000, 8_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.MangleTransformer{},
			&transform.CaseTransformer{},
			&transform.AppendTransformer{Suffixes: buildCombineSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 3_000_000, 3_000_000, 8_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.PrependTransformer{Prefixes: buildSandwichPrefixes()},
			&transform.CaseTransformer{},
			&transform.AppendTransformer{Suffixes: buildSandwichSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 2000, 5000, 10000),
		MaxPerWord:      capBudget(cfg, 2_000_000, 5_000_000, 10_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.PrependTransformer{Prefixes: buildNumberPrefixes()},
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 30, 50, 200)},
			&transform.AppendTransformer{Suffixes: buildSymbolSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 2000, 5000),
		MaxPerWord:      capBudget(cfg, 3_000_000, 3_000_000, 8_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.CaseTransformer{},
			&transform.AppendTransformer{Suffixes: wordlists.CommonSuffixWords},
			&transform.AppendTransformer{Suffixes: buildNumberAndSymbolSuffixes()},
		},
		MaxIntermediate: capBudget(cfg, 1000, 3000, 5000),
		MaxPerWord:      capBudget(cfg, 1_000_000, 3_000_000, 8_000_000),
	})

	add(&transform.CombineTransformer{
		Stages: []transform.Transformer{
			&transform.LeetTransformer{MaxVariants: leetBudget(cfg, 100, 200, 500)},
			&transform.AppendTransformer{Suffixes: buildDoubleAppendSuffixes()},
		},
		MaxPerWord: capBudget(cfg, 5_000_000, 5_000_000, 10_000_000),
	})

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
		add(&transform.CombineTransformer{
			Stages: []transform.Transformer{
				&transform.CaseTransformer{},
				&transform.AppendTransformer{Suffixes: buildWordPatternSuffixes()},
				&transform.PrependTransformer{Prefixes: shortPrepends},
			},
			MaxIntermediate: capBudget(cfg, 0, 50_000, 200_000),
			MaxPerWord:      capBudget(cfg, 0, 3_000_000, 8_000_000),
		})

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

func buildWordPatternSuffixes() []string {
	var s []string
	syms := []string{"!", "@", "#", "$", "!!", "!@#", "!@#$", "!@#$%", "@!", "#!"}
	nums := []string{"0", "1", "2", "12", "123", "1234", "2024", "2025", "2026", "2027"}

	for _, w := range wordlists.CommonSuffixWords {
		s = append(s, w)
		for _, sym := range syms {
			s = append(s, w+sym)
		}
		for _, n := range nums {
			s = append(s, w+n)
		}
		for _, sym := range []string{"!", "@", "#"} {
			for _, n := range []string{"0", "1", "2", "123", "2026", "2027"} {
				s = append(s, w+sym+n, w+n+sym)
			}
		}
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
