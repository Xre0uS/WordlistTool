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
- **Dedup Modes**: Duplicate removal at write time — approximate (memory-capped) or exact
- **Configurable Data**: All symbols, words, leet map, years, etc. defined in `config.toml`, and every list in it reaches the output
- **Merge Mode**: Concatenate multiple wordlists into one file
- **Diff Mode**: Strip words already covered by previously tested wordlists, in three memory/accuracy modes
- **Streaming Architecture**: Memory-efficient chunked output
- **Multithreaded**: Fan-out worker pool with configurable thread count

## Installation

### Pre-built binaries

Download from [Releases](https://git.xreous.io/xreous/WordlistTool/releases):

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

An unknown key is an error rather than a silent omission, since a mistyped key would otherwise take its whole list out of the generated candidates. Empty or missing lists are reported on stderr for the same reason.

Symbols and separators are tiered. `symbols` and `symbols_extended` both feed the direct append/prepend stages and are unioned; `symbols_compact` is the smaller set used where a stage multiplies out. Separators run in three tiers, each nested in the next: `separators` against the cheap lists (bare separator, digits, two-digit numbers, years), `separators_mid` against suffix and prefix words and number sequences, `separators_compact` against the three-digit range. Every separator in the config therefore appears in the output, and only the widest cross-products are restricted to the small set.

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

-dedupe <mode>    Duplicate removal: approx (default), fingerprint, full
-dedupe-mem <MB>  Memory cap for -dedupe approx (default: 512)
```

### Dedup modes

| Mode | Memory | Behaviour |
|---|---|---|
| `approx` | capped by `-dedupe-mem` | Bloom filter. A false positive counts a unique candidate as a dupe and drops it; the run prints the rate and an upper bound on the loss |
| `fingerprint` | ~16 B per unique candidate | 63-bit fingerprints, collision risk about n²/2⁶⁴ |
| `full` | ~16 B plus the candidate itself | Whole candidates, nothing dropped |

`approx` stays the default because a large run holds hundreds of millions of candidates. Watch the `Up to N unique candidates dropped as dupes` line: if it is not 0, raise `-dedupe-mem` or switch to `-dedupe fingerprint`.

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

## Usage — Diff

A wordlist is exhausted and the next one is going in. `diff` strips every word the previous wordlists already covered and writes what is left to a new file, in input order, so the next hashcat run does not repeat work.

```bash
# one exhausted list
./wordlisttool diff new.txt -x rockyou.txt -o remaining.txt -v

# several exhausted lists, several new ones
./wordlisttool diff new1.txt new2.txt -x rockyou.txt -x run1.txt -x run2.txt -o remaining.txt -y

# no collision risk, at the cost of holding the words themselves
./wordlisttool diff new.txt -x tested.txt -mode full -v

# cap memory on a very large tested corpus
./wordlisttool diff huge.txt -x tested.txt -mode approx -mem 4096 -o remaining.txt -v
```

### Options

```
-x <file>      Previously tested wordlist (repeat for more)
-o <file>      Output file (default: remaining.txt)
-f             Overwrite the output file if it exists
-y             Skip confirmation prompt
-v             Show progress
-keep-dupes    Keep repeats within the new wordlist(s)

-mode <mode>   fingerprint (default), full, or approx
-mem <MB>      Memory cap for -mode approx (default: 1024)
-fpr <rate>    False-positive rate for -mode approx (default: 0.0001)
```

### Modes

Comparison is byte-exact and case-sensitive; blank lines are skipped. Repeats within the new wordlists are dropped too, unless `-keep-dupes` is given.

| Mode | Memory per unique word | Can drop an untested word |
|------|------------------------|---------------------------|
| `fingerprint` (default) | ~16 B | On a fingerprint collision |
| `full` | ~16 B + the word | Never |
| `approx` | capped by `-mem` | At the `-fpr` rate |

The failure that matters here is dropping a candidate that was never tried, so the three modes differ only in how they risk it.

`fingerprint` stores 63-bit hashes rather than words. Two distinct words sharing a hash costs you one untested candidate, at odds of roughly `n² / 2^64` — about 1 in a million at 4M unique words, 1 in 3,700 at 100M, and about 5% at 1B. The exact figure for your run is printed before anything is allocated.

`full` keeps the words themselves in an arena and confirms every probe against the stored bytes, so no collision can drop anything. It costs the word bytes on top of the index and runs roughly 30% slower. Use it when the tested corpus is large enough that the collision odds stop being comfortable, or when a missed crack is expensive.

`approx` is a bloom filter with a hard memory ceiling. At the default `-fpr 0.0001` it discards roughly 1 in 10,000 untested candidates; the run prints the effective rate and the expected loss up front. It is the fallback for when neither exact mode fits in RAM.

Run it once with `-y` omitted: the plan table shows the estimated memory and the mode's risk before anything is allocated.

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
  hashset.go       Fingerprint set — compact, small collision risk
  wordset.go       Whole-word set — arena backed, no collision risk
  estimate.go      Dry-run candidate count estimation
  merge.go         Sequential wordlist merge
  diff.go          Strip already-tested words from a new wordlist

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