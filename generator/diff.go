package generator

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// DiffMode selects the membership structure behind a diff. A mode only matters
// when a filter is needed at all; whether that is the case is DiffSorted.
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

// DiffSorted picks whether the merge-join fast path is used: the exact
// set difference of byte-sorted inputs, streamed with no membership
// structure at all.
type DiffSorted int

const (
	SortedAuto  DiffSorted = iota // probe the heads; merge when every sample is sorted
	SortedForce                   // merge unconditionally
	SortedOff                     // never merge
)

func ParseDiffSorted(s string) (DiffSorted, error) {
	switch strings.ToLower(s) {
	case "auto", "":
		return SortedAuto, nil
	case "force":
		return SortedForce, nil
	case "off":
		return SortedOff, nil
	default:
		return SortedAuto, fmt.Errorf("unknown value %q (use auto, force, off)", s)
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

	Sorted    DiffSorted // merge-join fast path: auto, force, or off
	KeepDupes bool       // do not drop repeats within the new wordlists
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
	inputs, inputBytes, inputEst, excludes, excludeBytes, excludeEst, err := diffPreflight(cfg)
	if err != nil {
		return err
	}

	// The exact set also holds what gets written, so size it for both sides;
	// the merge path skips this entirely.
	totalEst := excludeEst
	if !cfg.KeepDupes {
		totalEst += inputEst
	}

	fmt.Fprintf(os.Stderr, "[*] Diff %d new wordlist(s) against %d tested → %s\n\n",
		len(inputs), len(excludes), cfg.OutputFile)
	printDiffGroup("Already tested", excludes, excludeBytes, excludeEst)
	printDiffGroup("New", inputs, inputBytes, inputEst)

	// Selection happens before any set is allocated: a byte-sorted input
	// against byte-sorted excludes is a streaming merge-join, exact and
	// constant memory, with no filter at all.
	useMerge := false
	if cfg.Sorted != SortedOff && len(cfg.InputFiles) == 1 {
		if cfg.Sorted == SortedForce {
			useMerge = true
		} else {
			sorted, v, err := probeSorted(append([]string{cfg.InputFiles[0]}, cfg.ExcludeFiles...))
			if err != nil {
				return err
			}
			if sorted {
				useMerge = true
			} else {
				fmt.Fprintf(os.Stderr, "[*] %s is not sorted as words are read: line %d %s is out of order after line %d %s (a trailing CR is stripped before the compare, so a CR-terminated line can break the order); falling back to the usual modes\n",
					v.path, v.line, qWord(v.word), v.prevLine, qWord(v.prev))
			}
		}
	}

	if useMerge {
		fmt.Fprintln(os.Stderr, "[*] Mode: sorted merge-join (exact, constant memory)")
		fmt.Fprintln(os.Stderr, "[*] Byte order (LC_ALL=C); adjacent duplicates collapse unless -keep-dupes")
		fmt.Fprintln(os.Stderr)
		if !diffConfirm(cfg) {
			return nil
		}
		return runDiffMerge(cfg, inputs[0], excludes)
	}

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
			commaFmt(int64(float64(excludeEst)*effective)))

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

	if !diffConfirm(cfg) {
		return nil
	}

	return runDiffFilter(cfg, inputs, excludes, totalEst, bf)
}

// diffPreflight checks the arguments and the output destination, before
// anything is allocated.
func diffPreflight(cfg *DiffConfig) ([]diffFileStat, int64, int64, []diffFileStat, int64, int64, error) {
	if len(cfg.InputFiles) == 0 {
		return nil, 0, 0, nil, 0, 0, fmt.Errorf("no new wordlist specified")
	}
	if len(cfg.ExcludeFiles) == 0 {
		return nil, 0, 0, nil, 0, 0, fmt.Errorf("no tested wordlist specified (use -x)")
	}

	if cfg.Sorted == SortedForce && len(cfg.InputFiles) > 1 {
		return nil, 0, 0, nil, 0, 0, fmt.Errorf("-sorted takes a single new wordlist, but %d were given (drop -sorted to run the usual modes)", len(cfg.InputFiles))
	}

	inputs, inputBytes, inputEst, err := statWordlists(cfg.InputFiles)
	if err != nil {
		return nil, 0, 0, nil, 0, 0, err
	}
	excludes, excludeBytes, excludeEst, err := statWordlists(cfg.ExcludeFiles)
	if err != nil {
		return nil, 0, 0, nil, 0, 0, err
	}

	if err := checkOutputCollision(cfg); err != nil {
		return nil, 0, 0, nil, 0, 0, err
	}
	if !cfg.Force {
		if _, err := os.Stat(cfg.OutputFile); err == nil {
			return nil, 0, 0, nil, 0, 0, fmt.Errorf("output file already exists: %s (use -f to overwrite)", cfg.OutputFile)
		}
	}
	return inputs, inputBytes, inputEst, excludes, excludeBytes, excludeEst, nil
}

// diffConfirm asks the confirmation prompt; it reports whether to proceed.
func diffConfirm(cfg *DiffConfig) bool {
	if cfg.NoConfirm {
		return true
	}
	fmt.Fprint(os.Stderr, "[?] Proceed? (y/N): ")
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(os.Stderr, "[!] Aborted.")
		return false
	}
	return true
}

// runDiffFilter runs the three modes behind their membership structure.
func runDiffFilter(cfg *DiffConfig, inputs, excludes []diffFileStat, totalEst int64, bf *bloom) error {
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

	return diffReport(cfg, inputs, start, read, tested, dupes, written, blank)
}

// diffReport prints the shared summary: timing, the counters, the reduction,
// the output line and the per-file breakdown.
func diffReport(cfg *DiffConfig, inputs []diffFileStat, start time.Time, read, tested, dupes, written, blank int64) error {
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

// ── Sorted fast path ────────────────────────────────────────────────────────

// runDiffMerge is the exact set difference of a byte-sorted input against
// byte-sorted excludes, streamed as a merge-join: an input word is emitted
// unless some exclude stream holds it. No membership structure is allocated,
// so the run stays in a few MB at any input size.
func runDiffMerge(cfg *DiffConfig, input diffFileStat, excludes []diffFileStat) error {
	start := time.Now()

	// The input cursor is verified mid-file too: a stream that goes backwards
	// is no longer sorted, so the merge would be wrong.
	in, err := newLineCursor(input.path, func(line, prev []byte) bool {
		return bytes.Compare(line, prev) >= 0
	})
	if err != nil {
		return err
	}
	defer in.Close()

	var exCursors []*lineCursor
	for _, f := range excludes {
		c, err := newLineCursor(f.path, inOrder)
		if err != nil {
			return err
		}
		exCursors = append(exCursors, c)
	}

	// The k-way heap keeps the exclude fronts; only the front's word matters,
	// so each one is copied out of the cursor's line buffer at its step.
	exHeap := make([]diffFront, 0, len(exCursors))
	for i, c := range exCursors {
		if err := c.Next(); err != nil {
			closeAllCursors(exCursors)
			if v, ok := err.(*notSortedErr); ok {
				return notSortedError(v)
			}
			return err
		}
		c.stat.read = int64(c.line)
		c.stat.blank = c.blank
		if !c.done {
			exHeap = append(exHeap, diffFront{word: copyWord(c.word), c: i})
		}
	}
	// The fronts were added in file order, not heap order; sift from the
	// leaves up.
	for i := len(exHeap)/2 - 1; i >= 0; i-- {
		siftDownFront(exHeap, i)
	}

	var read, tested, dupes, written, blank int64
	var progRead, progWritten int64
	var advanceErr error

	// advanceExclude pulls the next word of the front stream and re-sifts the
	// front down; the heap order below it is intact.
	advanceExclude := func() {
		i := exHeap[0].c
		c := exCursors[i]
		if advanceErr = c.Next(); advanceErr != nil {
			return
		}
		c.stat.read = int64(c.line)
		c.stat.blank = c.blank
		if c.done {
			exHeap[0] = exHeap[len(exHeap)-1]
			exHeap = exHeap[:len(exHeap)-1]
			if len(exHeap) > 0 {
				siftDownFront(exHeap, 0)
			}
			return
		}
		exHeap[0] = diffFront{word: copyWord(c.word), c: i}
		siftDownFront(exHeap, 0)
	}

	outFile, err := os.Create(cfg.OutputFile)
	if err != nil {
		closeAllCursors(exCursors)
		return fmt.Errorf("creating output: %w", err)
	}

	writer := bufio.NewWriterSize(outFile, 8*1024*1024)

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

	fmt.Fprintln(os.Stderr, "[*] Merging ...")

	// Sorted inputs keep duplicates adjacent, so the filter path's set is not
	// needed: a word is a dupe only when it repeats one already emitted, and a
	// repeat of an excluded word reads as tested again. Both are decided from
	// the previous input word and whether that one was excluded.
	var prevInput []byte
	prevExcluded := false
	sinceFlush := int64(0)

	for {
		if err := in.Next(); err != nil {
			return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, err)
		}
		in.stat.read = int64(in.line)
		read = int64(in.line)
		in.stat.blank = in.blank
		blank = in.blank
		if in.done {
			break
		}
		word := in.word

		if prevInput != nil && bytes.Equal(word, prevInput) {
			// Repeat of the previous word: a copy that was excluded reads as
			// tested again; one that was kept is written under -keep-dupes
			// and dropped as a dupe otherwise.
			switch {
			case prevExcluded:
				in.stat.tested++
				tested++
			case cfg.KeepDupes:
				if _, err := writer.Write(word); err != nil {
					return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, err)
				}
				if err := writer.WriteByte('\n'); err != nil {
					return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, err)
				}
				in.stat.written++
				written++

				sinceFlush++
				if sinceFlush >= 65536 {
					atomic.StoreInt64(&progRead, read)
					atomic.StoreInt64(&progWritten, written)
					sinceFlush = 0
				}
			default:
				in.stat.dupes++
				dupes++
			}
		} else {
			// Advance every exclude stream past word: a front smaller than word
			// cannot match word or anything later, so it is pulled off as well.
			testedNow := false
			for len(exHeap) > 0 && bytes.Compare(exHeap[0].word, word) <= 0 {
				if !testedNow && bytes.Equal(exHeap[0].word, word) {
					testedNow = true
					in.stat.tested++
					tested++
				}
				advanceExclude()
				if advanceErr != nil {
					return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, advanceErr)
				}
			}
			if testedNow {
				prevExcluded = true
			} else {
				// Emitted: copy it out; it is held until the next step.
				prevInput = append(prevInput[:0], word...)
				if _, err := writer.Write(word); err != nil {
					return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, err)
				}
				if err := writer.WriteByte('\n'); err != nil {
					return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, err)
				}
				in.stat.written++
				written++
				prevExcluded = false

				sinceFlush++
				if sinceFlush >= 65536 {
					atomic.StoreInt64(&progRead, read)
					atomic.StoreInt64(&progWritten, written)
					sinceFlush = 0
				}
			}
		}
		// Remember the word for the next step; excluded-ness only changes in
		// the non-repeat branch, where it is set to testedNow.
		prevInput = append(prevInput[:0], word...)
	}

	if err := writer.Flush(); err != nil {
		return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, fmt.Errorf("flushing output: %w", err))
	}

	// The merge left each exclude file's tail unread, and an unsorted tail
	// could still have matched a later input word; read it off and verify,
	// holding nothing.
	for _, c := range exCursors {
		for !c.done {
			if err := c.Next(); err != nil {
				return diffMergeAbort(cfg, outFile, stopProgress, in, exCursors, err)
			}
			c.stat.read = int64(c.line)
			c.stat.blank = c.blank
		}
	}
	close(stopProgress)

	// The excludes stream with the input; no set is held, so there is no
	// size to report.
	fmt.Fprintf(os.Stderr, "[+] Excludes: %d file(s) streamed with the input\n",
		len(exCursors))

	return diffReport(cfg, []diffFileStat{in.stat}, start, read, tested, dupes, written, blank)
}

// diffMergeAbort closes the streams and the partial output and reports the
// failure, naming the file, lines and words for an order violation.
func diffMergeAbort(cfg *DiffConfig, outFile *os.File, stopProgress chan struct{}, in *lineCursor, exCursors []*lineCursor, err error) error {
	close(stopProgress)
	outFile.Close()
	closeAllCursors(exCursors)
	os.Remove(cfg.OutputFile)

	if v, ok := err.(*notSortedErr); ok {
		return notSortedError(v)
	}
	return fmt.Errorf("sorted merge-join: %w", err)
}

// notSortedError is the user-facing error for a stream that went backwards:
// it names the file and both words and points at -no-sorted.
func notSortedError(v *notSortedErr) error {
	return fmt.Errorf("%s is not sorted in byte order: line %d %s is out of order after line %d %s (re-run with -no-sorted)",
		v.path, v.line, qWord(v.word), v.prevLine, qWord(v.prev))
}

func qWord(b []byte) string {
	if len(b) > 40 {
		return fmt.Sprintf("%q...", b[:40])
	}
	return fmt.Sprintf("%q", b)
}

// notSortedErr marks a stream that went strictly backwards mid-file.
type notSortedErr struct {
	path     string
	line     int
	word     []byte
	prevLine int
	prev     []byte
}

func (e *notSortedErr) Error() string {
	return fmt.Sprintf("not sorted: %q on line %d after %q on line %d",
		e.word, e.line, e.prev, e.prevLine)
}

// inOrder reports non-decreasing byte order; equal words are in order.
func inOrder(line, prev []byte) bool {
	return bytes.Compare(line, prev) >= 0
}

func closeAllCursors(cs []*lineCursor) {
	for _, c := range cs {
		c.Close()
	}
}

// copyWord detaches a cursor line from the reader's buffer; the slice a line
// is yielded in is only valid until the next one.
func copyWord(b []byte) []byte {
	return append([]byte(nil), b...)
}

// diffFront is the head of one exclude stream in the merge's k-way heap.
type diffFront struct {
	word []byte
	c    int
}

// siftDownFront re-heapifies the front heap from i downwards.
func siftDownFront(hp []diffFront, i int) {
	for {
		l, r := 2*i+1, 2*i+2
		m := i
		if l < len(hp) && bytes.Compare(hp[l].word, hp[m].word) < 0 {
			m = l
		}
		if r < len(hp) && bytes.Compare(hp[r].word, hp[m].word) < 0 {
			m = r
		}
		if m == i {
			return
		}
		hp[i], hp[m] = hp[m], hp[i]
		i = m
	}
}

// lineCursor pulls a wordlist line by line, in trimEOL byte form, like
// eachLine but pull-style so several streams can be merged. word is only
// valid until the next call to Next.
type lineCursor struct {
	path string
	r    *bufio.Reader
	file *os.File
	long []byte // long lines that outgrow the reader buffer
	ver  func(line, prev []byte) bool

	word     []byte
	prev     []byte
	line     int
	prevLine int
	blank    int64
	done     bool

	stat diffFileStat
}

func newLineCursor(path string, ver func(line, prev []byte) bool) (*lineCursor, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return &lineCursor{
		path: path,
		r:    bufio.NewReaderSize(f, 4*1024*1024),
		file: f,
		ver:  ver,
		stat: diffFileStat{path: path, size: info.Size()},
	}, nil
}

// Next advances to the next word, skipping and counting blank lines; the
// stream is verified on the way when it was built with a verifier.
func (c *lineCursor) Next() error {
	for !c.done {
		line, err := c.r.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			c.long = append(c.long[:0], line...)
			for err == bufio.ErrBufferFull {
				line, err = c.r.ReadSlice('\n')
				c.long = append(c.long, line...)
			}
			line = c.long
		}

		switch {
		case err == nil:
		case err == io.EOF:
			if len(line) == 0 {
				c.done = true
				return nil
			}
		default:
			return fmt.Errorf("reading %s: %w", c.path, err)
		}

		c.line++
		word := trimEOL(line)
		if len(word) == 0 {
			c.blank++
			if err == io.EOF {
				c.done = true
			}
			continue
		}

		if c.ver != nil && !c.ver(word, c.prev) {
			return &notSortedErr{path: c.path, line: c.line, word: word, prevLine: c.prevLine, prev: c.prev}
		}
		c.prevLine = c.line
		c.prev = append(c.prev[:0], word...)
		c.word = word

		if err == io.EOF {
			c.done = true
		}
		return nil
	}
	return nil
}

func (c *lineCursor) Close() {
	if c.file != nil {
		c.file.Close()
	}
	c.file = nil
}

// probeSorted samples the head of each path and reports whether every sample
// is in non-decreasing byte order. The first path that is not decides: it is
// the one whose words the merge would walk backwards; when one is not, the
// violating pair comes back as a *notSortedErr, like the streaming verifier's.
func probeSorted(paths []string) (bool, *notSortedErr, error) {
	for _, path := range paths {
		ok, v, err := probeSortedOne(path)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, v, nil
		}
	}
	return true, nil, nil
}

// probeSortedOne reads the first few tens of MB of path and checks that the
// words are in non-decreasing byte order, after trimEOL has stripped any
// trailing CR.
func probeSortedOne(path string) (bool, *notSortedErr, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	const sample = 32 * 1024 * 1024
	r := bufio.NewReaderSize(f, sample)

	var buf []byte
	var prev []byte
	var lineNo, prevLine int
	var consumed int

	for {
		line, err := r.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			buf = append(buf[:0], line...)
			for err == bufio.ErrBufferFull {
				line, err = r.ReadSlice('\n')
				buf = append(buf, line...)
			}
			line = buf
		}

		switch {
		case err == nil:
		case err == io.EOF:
			if len(line) == 0 {
				return true, nil, nil
			}
		default:
			return false, nil, fmt.Errorf("reading %s: %w", path, err)
		}

		lineNo++
		word := trimEOL(line)
		if len(word) > 0 {
			if bytes.Compare(word, prev) < 0 {
				return false, &notSortedErr{path: path, line: lineNo, word: copyWord(word), prevLine: prevLine, prev: copyWord(prev)}, nil
			}
			prevLine = lineNo
			prev = append(prev[:0], word...)
		}

		consumed += len(line)
		if err == io.EOF || consumed >= sample {
			return true, nil, nil
		}
	}
}
