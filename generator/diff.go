package generator

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// DiffMode selects the membership structure behind a diff.
type DiffMode int

const (
	DiffFingerprint DiffMode = iota // 63-bit fingerprints, small collision risk
	DiffFull                        // whole words, no collision risk
	DiffApprox                      // bloom filter, memory capped up front
)

func ParseDiffMode(s string) (DiffMode, error) {
	switch strings.ToLower(s) {
	case "fingerprint", "fp", "":
		return DiffFingerprint, nil
	case "full", "word", "words":
		return DiffFull, nil
	case "approx", "bloom":
		return DiffApprox, nil
	default:
		return DiffFingerprint, fmt.Errorf("unknown mode %q (use fingerprint, full, approx)", s)
	}
}

// DiffConfig drives RunDiff: keep only the words no tested wordlist holds.
type DiffConfig struct {
	InputFiles   []string // new wordlist(s), read in order
	ExcludeFiles []string // wordlists already run against the hashes
	OutputFile   string
	NoConfirm    bool
	Force        bool
	Verbose      bool

	Mode       DiffMode
	MemLimitMB int     // memory cap for approx mode
	FPR        float64 // target false-positive rate for approx mode

	KeepDupes bool // do not drop repeats within the new wordlists
}

// diffFilter is the membership structure behind the three modes.
type diffFilter struct {
	set   *hashSet
	words *wordSet
	bf    *bloom
}

// load records a word from a tested wordlist.
func (d *diffFilter) load(line []byte) {
	switch {
	case d.set != nil:
		d.set.Insert(fingerprint63(line), false)
	case d.words != nil:
		d.words.Insert(line, false)
	default:
		d.bf.TestAndAddBytes(line)
	}
}

// check reports whether line has been seen; record marks an unseen one emitted.
// Approx mode cannot flag, so every hit there reads as already tested.
func (d *diffFilter) check(line []byte, record bool) (present, wasEmitted bool) {
	switch {
	case d.set != nil:
		fp := fingerprint63(line)
		if record {
			return d.set.Insert(fp, true)
		}
		return d.set.Lookup(fp)
	case d.words != nil:
		if record {
			return d.words.Insert(line, true)
		}
		return d.words.Lookup(line)
	default:
		if record {
			return d.bf.TestAndAddBytes(line), false
		}
		return d.bf.TestBytes(line), false
	}
}

// len reports unique words held, or -1 if the structure cannot count them.
func (d *diffFilter) len() int64 {
	switch {
	case d.set != nil:
		return d.set.Len()
	case d.words != nil:
		return d.words.Len()
	default:
		return -1
	}
}

func (d *diffFilter) memoryBytes() int64 {
	switch {
	case d.set != nil:
		return d.set.memoryBytes()
	case d.words != nil:
		return d.words.memoryBytes()
	default:
		return d.bf.memoryBytes()
	}
}

type diffFileStat struct {
	path    string
	size    int64
	est     int64
	read    int64
	blank   int64
	tested  int64
	dupes   int64
	written int64
}

func RunDiff(cfg *DiffConfig) error {
	if len(cfg.InputFiles) == 0 {
		return fmt.Errorf("no new wordlist specified")
	}
	if len(cfg.ExcludeFiles) == 0 {
		return fmt.Errorf("no tested wordlist specified (use -x)")
	}

	inputs, inputBytes, inputEst, err := statWordlists(cfg.InputFiles)
	if err != nil {
		return err
	}
	excludes, excludeBytes, excludeEst, err := statWordlists(cfg.ExcludeFiles)
	if err != nil {
		return err
	}

	if err := checkOutputCollision(cfg); err != nil {
		return err
	}
	if !cfg.Force {
		if _, err := os.Stat(cfg.OutputFile); err == nil {
			return fmt.Errorf("output file already exists: %s (use -f to overwrite)", cfg.OutputFile)
		}
	}

	// The exact set also holds what gets written, so size it for both sides.
	totalEst := excludeEst
	if !cfg.KeepDupes {
		totalEst += inputEst
	}

	fmt.Fprintf(os.Stderr, "[*] Diff %d new wordlist(s) against %d tested → %s\n\n",
		len(inputs), len(excludes), cfg.OutputFile)
	printDiffGroup("Already tested", excludes, excludeBytes, excludeEst)
	printDiffGroup("New", inputs, inputBytes, inputEst)

	var bf *bloom
	switch cfg.Mode {
	case DiffApprox:
		fpr := cfg.FPR
		if fpr <= 0 || fpr >= 1 {
			fpr = 0.0001
		}
		memMB := cfg.MemLimitMB
		if memMB < 1 {
			memMB = 1024
		}
		bf = newBloom(totalEst, fpr, memMB)

		effective := bf.fpr(totalEst)
		fmt.Fprintf(os.Stderr, "[*] Mode: approx (bloom) | Memory: %s | Effective FPR: %.5f%%\n",
			humanSize(bf.memoryBytes()), effective*100)
		fmt.Fprintf(os.Stderr, "[!] Approx mode silently discards roughly %s untested candidate(s).\n",
			commaFmt(int64(float64(inputEst)*effective)))

	case DiffFull:
		// A word costs a length byte in the arena; the newline it drops from the
		// file roughly pays for it, so file sizes stand in.
		arenaEst := excludeBytes
		if !cfg.KeepDupes {
			arenaEst += inputBytes
		}
		estBytes := wordSetBytesFor(totalEst, arenaEst)
		fmt.Fprintf(os.Stderr, "[*] Mode: full (whole words, no collision risk) | Estimated memory: %s\n",
			humanSize(estBytes))
		warnLargeSet(estBytes)

	default:
		estBytes := int64(hashSetSlotsFor(totalEst)) * 8
		fmt.Fprintf(os.Stderr, "[*] Mode: fingerprint (63-bit) | Estimated memory: %s | Collision risk: %s\n",
			humanSize(estBytes), collisionOdds(totalEst))
		warnLargeSet(estBytes)
	}
	fmt.Fprintln(os.Stderr)

	if !cfg.NoConfirm {
		fmt.Fprint(os.Stderr, "[?] Proceed? (y/N): ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "[!] Aborted.")
			return nil
		}
	}

	start := time.Now()

	var filter diffFilter
	switch cfg.Mode {
	case DiffApprox:
		filter.bf = bf
	case DiffFull:
		filter.words = newWordSet(totalEst)
	default:
		filter.set = newHashSet(totalEst)
	}

	var loaded, loadDupes int64
	fmt.Fprintln(os.Stderr, "[*] Loading tested wordlists ...")
	for i := range excludes {
		f := &excludes[i]
		fmt.Fprintf(os.Stderr, "\r    %d/%d %s ...                    ", i+1, len(excludes), f.path)

		err := eachLine(f.path, func(line []byte) error {
			f.read++
			if len(line) == 0 {
				f.blank++
				return nil
			}
			filter.load(line)
			loaded++
			return nil
		})
		if err != nil {
			return err
		}
	}
	fmt.Fprint(os.Stderr, "\r                                                             \r")

	if unique := filter.len(); unique >= 0 {
		loadDupes = loaded - unique
		fmt.Fprintf(os.Stderr, "[+] Tested set: %s unique of %s read (%s duplicates) | %s\n",
			commaFmt(unique), commaFmt(loaded), commaFmt(loadDupes),
			humanSize(filter.memoryBytes()))
	} else {
		fmt.Fprintf(os.Stderr, "[+] Tested set: %s words loaded | %s\n",
			commaFmt(loaded), humanSize(filter.memoryBytes()))
	}

	outFile, err := os.Create(cfg.OutputFile)
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriterSize(outFile, 8*1024*1024)

	var read, tested, dupes, written, blank int64
	var progRead, progWritten int64

	stopProgress := make(chan struct{})
	if cfg.Verbose {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					r := atomic.LoadInt64(&progRead)
					w := atomic.LoadInt64(&progWritten)
					elapsed := time.Since(start).Seconds()
					fmt.Fprintf(os.Stderr, "\r[*] Read: %s | New: %s | Rate: %.0f/s          ",
						commaFmt(r), commaFmt(w), float64(r)/elapsed)
				case <-stopProgress:
					return
				}
			}
		}()
	}

	fmt.Fprintln(os.Stderr, "[*] Filtering ...")
	record := !cfg.KeepDupes

	for i := range inputs {
		f := &inputs[i]
		var sinceFlush int64

		err := eachLine(f.path, func(line []byte) error {
			f.read++
			read++
			if len(line) == 0 {
				f.blank++
				blank++
				return nil
			}

			present, wasEmitted := filter.check(line, record)
			if present {
				if wasEmitted {
					f.dupes++
					dupes++
				} else {
					f.tested++
					tested++
				}
				return nil
			}

			if _, err := writer.Write(line); err != nil {
				return err
			}
			if err := writer.WriteByte('\n'); err != nil {
				return err
			}
			f.written++
			written++

			sinceFlush++
			if sinceFlush >= 65536 {
				atomic.StoreInt64(&progRead, read)
				atomic.StoreInt64(&progWritten, written)
				sinceFlush = 0
			}
			return nil
		})
		if err != nil {
			return err
		}
		atomic.StoreInt64(&progRead, read)
		atomic.StoreInt64(&progWritten, written)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing output: %w", err)
	}
	close(stopProgress)

	elapsed := time.Since(start)
	if cfg.Verbose {
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintf(os.Stderr, "[+] Complete in %s\n", elapsed.Round(time.Millisecond))
	if cfg.Mode == DiffApprox {
		fmt.Fprintf(os.Stderr, "[+] Read: %s | Already seen: %s | New: %s\n",
			commaFmt(read), commaFmt(tested+dupes), commaFmt(written))
	} else {
		fmt.Fprintf(os.Stderr, "[+] Read: %s | Already tested: %s | Duplicates: %s | New: %s\n",
			commaFmt(read), commaFmt(tested), commaFmt(dupes), commaFmt(written))
	}
	if blank > 0 {
		fmt.Fprintf(os.Stderr, "[+] Blank lines skipped: %s\n", commaFmt(blank))
	}
	if read > 0 {
		fmt.Fprintf(os.Stderr, "[+] Reduction: %.1f%% removed, %.1f%% left to try\n",
			float64(read-written)/float64(read)*100, float64(written)/float64(read)*100)
	}
	if info, err := os.Stat(cfg.OutputFile); err == nil {
		fmt.Fprintf(os.Stderr, "[+] Output: %s (%s)\n", cfg.OutputFile, humanSize(info.Size()))
	} else {
		fmt.Fprintf(os.Stderr, "[+] Output: %s\n", cfg.OutputFile)
	}

	if written == 0 {
		fmt.Fprintln(os.Stderr, "[!] Nothing left: every candidate in the new wordlist(s) has already been tested.")
	}

	if len(inputs) > 1 {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "[*] Breakdown:")
		for i, f := range inputs {
			pct := 0.0
			if f.read > 0 {
				pct = float64(f.written) / float64(f.read) * 100
			}
			fmt.Fprintf(os.Stderr, "    %d. %-35s %12s read   %12s new   %5.1f%%\n",
				i+1, f.path, commaFmt(f.read), commaFmt(f.written), pct)
		}
	}

	return nil
}

func warnLargeSet(estBytes int64) {
	if estBytes > 4*1024*1024*1024 {
		fmt.Fprintln(os.Stderr, "[!] Large working set. Use -mode approx -mem <MB> to cap memory at the cost of accuracy.")
	}
}

// collisionOdds is the chance any two of n words share a fingerprint.
func collisionOdds(n int64) string {
	if n < 2 {
		return "none"
	}
	// Birthday approximation: 1 - e^(-n^2 / 2^64).
	p := -math.Expm1(-(float64(n) * float64(n)) / math.Pow(2, 64))
	switch {
	case p < 1e-9:
		return "negligible"
	case p < 0.01:
		return fmt.Sprintf("~1 in %s", commaFmt(int64(1/p)))
	default:
		return fmt.Sprintf("~%.1f%%", p*100)
	}
}

func statWordlists(paths []string) ([]diffFileStat, int64, int64, error) {
	var out []diffFileStat
	var totalBytes, totalEst int64

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("cannot stat %s: %w", path, err)
		}
		if info.IsDir() {
			return nil, 0, 0, fmt.Errorf("%s is a directory, not a file", path)
		}

		est, err := estimateLines(path, info.Size())
		if err != nil {
			return nil, 0, 0, err
		}

		out = append(out, diffFileStat{path: path, size: info.Size(), est: est})
		totalBytes += info.Size()
		totalEst += est
	}

	return out, totalBytes, totalEst, nil
}

// checkOutputCollision refuses to write over a file being read.
func checkOutputCollision(cfg *DiffConfig) error {
	out, err := filepath.Abs(cfg.OutputFile)
	if err != nil {
		return err
	}
	for _, p := range append(append([]string{}, cfg.InputFiles...), cfg.ExcludeFiles...) {
		in, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		if in == out {
			return fmt.Errorf("output %s is also an input", cfg.OutputFile)
		}
	}
	return nil
}

func printDiffGroup(label string, files []diffFileStat, totalBytes, totalEst int64) {
	fmt.Fprintf(os.Stderr, "    %s:\n", label)
	for i, f := range files {
		fmt.Fprintf(os.Stderr, "      %d. %-35s %10s   ~%s words\n",
			i+1, f.path, humanSize(f.size), commaFmt(f.est))
	}
	fmt.Fprintln(os.Stderr, "      "+strings.Repeat("─", 63))
	fmt.Fprintf(os.Stderr, "      %-38s %10s   ~%s words\n\n",
		"Total", humanSize(totalBytes), commaFmt(totalEst))
}

// estimateLines approximates the line count from the first few MB, to size the
// filter up front.
func estimateLines(path string, size int64) (int64, error) {
	if size == 0 {
		return 0, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	const sample = 4 * 1024 * 1024
	n := int64(sample)
	if size < n {
		n = size
	}
	buf := make([]byte, n)

	read, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}

	var lines int64
	for _, b := range buf[:read] {
		if b == '\n' {
			lines++
		}
	}
	if lines == 0 {
		return 1, nil
	}

	avg := float64(read) / float64(lines)
	return int64(float64(size)/avg) + 1, nil
}

// eachLine streams path line by line without the trailing newline. The slice
// handed to fn is valid only until the next line.
func eachLine(path string, fn func(line []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 4*1024*1024)
	var long []byte

	for {
		line, err := r.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			long = append(long[:0], line...)
			for err == bufio.ErrBufferFull {
				line, err = r.ReadSlice('\n')
				long = append(long, line...)
			}
			line = long
		}

		switch {
		case err == nil:
		case err == io.EOF:
			if len(line) == 0 {
				return nil
			}
		default:
			return fmt.Errorf("reading %s: %w", path, err)
		}

		if cbErr := fn(trimEOL(line)); cbErr != nil {
			return cbErr
		}

		if err == io.EOF {
			return nil
		}
	}
}

func trimEOL(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
	}
	if n := len(b); n > 0 && b[n-1] == '\r' {
		b = b[:n-1]
	}
	return b
}
