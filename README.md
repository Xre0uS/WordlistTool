# WordlistTool

Advanced wordlist generator for hash cracking. Generates high-quality password candidates from keywords (company names, slogans, etc.) by applying modular, multi-stage transformations that go far beyond basic hashcat rules.

## Features

- **Case Transformations**: lower, upper, title, inverted, toggle, alternating, per-position capitalization
- **L33t Speak**: Multi-level substitutions (a→@/4, e→3, o→0, s→5/$, t→7/+, c→(, h→#, r→2, etc.) with smart prioritization
- **Append/Prepend**: Symbols, all digits (0-9999), years (1950-2050), keyboard walks, number sequences, common words, separator combos
- **Mangling**: Reverse, duplicate, vowel strip, truncation, character doubling, stutter, char swaps
- **Combination Chains**: Multi-stage transforms (e.g., case→leet→prepend→append) for deep candidates like `!G00gle@2026#`
- **3-Tier Presets**: `short` (fast), `standard` (balanced), `long` (thorough)
- **Min-Length Filter**: Discard candidates shorter than a threshold (default: 8) — no wasted output
- **Merge Mode**: Concatenate multiple wordlists into one file, with size preview and confirmation
- **Streaming Architecture**: Memory-efficient chunked output — runs comfortably on low-end systems
- **Modular Architecture**: Each transform is a pluggable module implementing the `Transformer` interface
- **Multithreaded**: Fan-out worker pool with configurable thread count

## Installation

Pre-built binaries:
- `wordlisttool` — Linux x86_64
- `wordlisttool.exe` — Windows x86_64

Build from source:
```bash
# Native
go build -o wordlisttool .

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o wordlisttool .
GOOS=windows GOARCH=amd64 go build -o wordlisttool.exe .
```

## Usage — Generate

```bash
# Standard generation
./wordlisttool -i keywords.txt -o wordlist.txt

# Quick run (fewer chains, lower leet budget)
./wordlisttool -i keywords.txt -o wordlist.txt -preset short

# Thorough run (all chains, high leet budget, 4-stage combos)
./wordlisttool -i keywords.txt -o wordlist.txt -preset long

# With progress, custom min length
./wordlisttool -i keywords.txt -o wordlist.txt -v -min-length 10

# Disable specific transforms
./wordlisttool -i keywords.txt -o wordlist.txt -no-leet -no-prepend

# Limit output
./wordlisttool -i keywords.txt -o wordlist.txt -max 10000000
```

## Usage — Merge

Concatenate multiple wordlists into a single file, preserving order. Shows total size and asks for confirmation before writing.

```bash
# Merge 3 wordlists into one
./wordlisttool merge -o combined.txt list1.txt list2.txt list3.txt

# Skip confirmation prompt
./wordlisttool merge -y -o combined.txt list1.txt list2.txt list3.txt
```

Merge options:
```
-o <file>   Output file (default: merged.txt)
-y          Skip confirmation prompt
```

## Input Format

One keyword per line:
```
google
chrome
android
```

## Generate Options

```
-i  <file>        Input keyword file (required)
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
```

## Presets

| Preset | Leet Budget | Description |
|--------|-------------|-------------|
| `short` | 128 | Fewer intermediate variants — fast |
| `standard` | 512 | Balanced — default |
| `long` | 2048 | High leet budget, 4-stage combos — thorough |

## Example Output

From keyword `google`:
```
google, Google, GOOGLE, gOOGLE, gOoGlE, ...
g00gle, g009le, g0o9l3, ...
Google2026, google!@#, GOOGLE123, ...
G00gle@2026!, 123Google#, !g00gle2026, ...
elgoog, ggl, ggooooggllee, ggoogle, ...
googleadmin1, Google_2026, ...
!G00gle@2026#, 123!#gOOGLE, ...
```

## Architecture

```
main.go          - Subcommand dispatch (generate / merge), chain configuration

transform/
  types.go       - Transformer interface, StreamAll (chunked streaming)
  case.go        - Case transformations (12+ variants per word)
  leet.go        - L33t speak substitutions (prioritized full/partial/combo)
  append.go      - Suffix appending (symbols, numbers, years, words, patterns)
  prepend.go     - Prefix prepending (symbols, numbers, years, words, patterns)
  mangle.go      - String mangling (reverse, duplicate, truncate, vowel-strip)
  combine.go     - Multi-stage chaining with chunked final-stage streaming

generator/
  pipeline.go    - Multithreaded fan-out pipeline with min-length filtering
  merge.go       - Sequential wordlist merge with size preview

wordlists/
  data.go        - Embedded data: symbols, digits 0-9999, years, keyboard walks,
                   number sequences, common words, pattern combinations

config/
  config.go      - CLI flag parsing, preset system, configuration
```

## Adding Custom Transforms

Implement the `Transformer` interface:

```go
type Transformer interface {
    Name() string
    Transform(input string) []string
}
```

Then add it to the transformer list in `main.go`.
