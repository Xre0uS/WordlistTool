# WordlistTool

Generates targeted wordlists for cracking organisation-specific passwords. Given a few keywords (company name, slogans, product names), it produces candidates that mirror the password patterns companies actually use — case variations, l33t speak, appended years/symbols, and deep multi-stage combinations. Not a replacement for broad wordlists and designed to be used **alongside hashcat rules** for maximum coverage.

## Features

- **Case Transformations**: lower, upper, title, inverted, toggle, alternating, per-position capitalization
- **L33t Speak**: Multi-level substitutions with smart prioritization
- **Append/Prepend**: Symbols, digits (0-9999), years, keyboard walks, number sequences, common words
- **Mangling**: Reverse, duplicate, vowel strip, truncation, character doubling, stutter, char swaps
- **Combination Chains**: Multi-stage transforms for deep candidates like `!@cm3Corp@2026#`
- **3-Tier Presets**: `short` (fast), `standard` (balanced), `long` (thorough)
- **Min-Length Filter**: Discard candidates shorter than a threshold (default: 8)
- **Bloom Filter Dedup**: Approximate duplicate removal at write time
- **Configurable Data**: All symbols, words, leet map, years, etc. defined in `config.toml`
- **Merge Mode**: Concatenate multiple wordlists into one file
- **Streaming Architecture**: Memory-efficient chunked output
- **Multithreaded**: Fan-out worker pool with configurable thread count

## Installation

### Pre-built binaries

Download from [GitHub Releases](https://github.com/Xre0uS/wordlisttool/releases):

| Platform | File |
|----------|------|
| Linux x86_64 | `wordlisttool` |
| Windows x86_64 | `wordlisttool.exe` |

### Build from source

```bash
go build -o wordlisttool .
```

## Configuration

All generation data (symbols, words, leet substitutions, year ranges, keyboard walks, etc.) is defined in `config.toml`. The file must be present — either in the current directory or specified with `-config`.

```bash
# Uses ./config.toml by default
./wordlisttool generate keywords.txt -o wordlist.txt -v

# Use a custom config
./wordlisttool generate keywords.txt -config custom.toml
```

See `config.toml` for all available keys and their descriptions.

## Usage — Estimate

```bash
./wordlisttool estimate keywords.txt
```

## Usage — Generate

```bash
./wordlisttool generate keywords.txt -o wordlist.txt -v
./wordlisttool generate keywords.txt -o wordlist.txt -preset short
./wordlisttool generate keywords.txt -o wordlist.txt -preset long -v
./wordlisttool generate keywords.txt -o wordlist.txt -v -min-length 10
./wordlisttool generate keywords.txt -o wordlist.txt -no-leet -no-prepend
./wordlisttool generate keywords.txt -o wordlist.txt -max 10000000
```

### Options

```
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
```

### Presets

| Preset | Leet Budget | Description |
|--------|-------------|-------------|
| `short` | 128 | Fewer intermediate variants — fast |
| `standard` | 512 | Balanced — default |
| `long` | 2048 | High leet budget, 4-stage combos — thorough |

## Usage — Merge

```bash
./wordlisttool merge list1.txt list2.txt list3.txt -o combined.txt
./wordlisttool merge list1.txt list2.txt -o combined.txt -y
```

## Input Format

One keyword per line:
```
acmecorp
phoenix
atlas
```

## Example Output

From keyword `acmecorp`:
```
acmecorp, Acmecorp, ACMECORP, aCMECORP, AcMeCoRp, ...
@cm3corp, @cm3c0rp, 4(m3(02p, ...
Acmecorp2026, acmecorp!@#, ACMECORP123, ...
@cm3corp@2026!, 123Acmecorp#, !@cm3corp2026, ...
procemca, cmcrp, aaccmmeeccoorrpp, aacmecorp, ...
acmecorpadmin1, Acmecorp_2026, ...
!@cm3Corp@2026#, 123!#aCMECORP, ...
```

## Architecture

```
main.go            Subcommand dispatch, chain configuration
config.toml        Data config: symbols, words, leet map, years, patterns

config/
  config.go        CLI flag parsing, preset system

transform/
  types.go         Transformer interface, StreamAll
  case.go          Case transformations
  leet.go          L33t speak substitutions
  append.go        Suffix appending
  prepend.go       Prefix prepending
  mangle.go        String mangling
  combine.go       Multi-stage chaining

generator/
  pipeline.go      Multithreaded fan-out pipeline
  bloom.go         Bloom filter for deduplication
  estimate.go      Dry-run candidate count estimation
  merge.go         Sequential wordlist merge

wordlists/
  data.go          TOML loader, exported variables
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
