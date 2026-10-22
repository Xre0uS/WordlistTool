package generator

import (
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Xre0uS/wordlisttool/transform"
)

type EstimateResult struct {
	Preset     string
	Count      int64
	TotalBytes int64 // sum of candidate lengths + newlines
	Duration   time.Duration
}

func RunEstimate(keywords []string, transformers []transform.Transformer, presetName string, threads int, minLen int) EstimateResult {
	start := time.Now()
	var total int64
	var totalBytes int64

	keywordCh := make(chan string, threads*2)

	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for keyword := range keywordCh {
				transform.StreamAll(transformers, keyword, func(batch []string) {
					var n int64
					var bytes int64
					for _, c := range batch {
						if minLen <= 0 || len(c) >= minLen {
							n++
							bytes += int64(len(c)) + 1 // +1 for newline
						}
					}
					atomic.AddInt64(&total, n)
					atomic.AddInt64(&totalBytes, bytes)
				})
			}
		}()
	}

	for _, kw := range keywords {
		keywordCh <- kw
	}
	close(keywordCh)
	wg.Wait()

	return EstimateResult{
		Preset:     presetName,
		Count:      atomic.LoadInt64(&total),
		TotalBytes: atomic.LoadInt64(&totalBytes),
		Duration:   time.Since(start),
	}
}

// bloomMBFor returns the filter size candidates needs for a 0.1% false-positive
// rate, and whether that exceeds the default -dedupe-mem cap. Reporting the
// capped figure instead would hide the dedupe loss a capped filter causes.
func bloomMBFor(candidates int64) (mb int, overCap bool) {
	if candidates <= 0 {
		candidates = 1_000_000
	}
	n := float64(candidates)
	fpr := 0.001
	m := math.Ceil(-n * math.Log(fpr) / (math.Ln2 * math.Ln2))
	mb = int(math.Ceil(m / 8 / (1024 * 1024)))
	return mb, mb > DefaultDedupeMemMB
}

func PrintEstimateTable(results []EstimateResult, keywords []string) {
	fmt.Fprintf(os.Stderr, "\n[*] Estimated output for %d keyword(s):\n\n", len(keywords))
	fmt.Fprintf(os.Stderr, "    %-12s %18s %12s %12s %12s\n",
		"Preset", "Candidates", "File Size", "Bloom RAM", "Est. Time")
	fmt.Fprintf(os.Stderr, "    ──────────────────────────────────────────────────────────────────\n")
	anyOverCap := false
	for _, r := range results {
		bloomMB, overCap := bloomMBFor(r.Count)
		marker := " "
		if overCap {
			marker = "*"
			anyOverCap = true
		}
		fmt.Fprintf(os.Stderr, "    %-12s %18s %12s %9d MB%s %12s\n",
			r.Preset,
			commaFmt(r.Count),
			humanSize(r.TotalBytes),
			bloomMB,
			marker,
			r.Duration.Round(time.Millisecond))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "    Bloom RAM is what -dedupe approx needs for a 0.1% false-positive rate.")
	fmt.Fprintln(os.Stderr, "    Counts are before dedup, so the written output is lower.")
	if anyOverCap {
		fmt.Fprintf(os.Stderr, "    * over the %d MB -dedupe-mem default: raise it, or use -dedupe\n", DefaultDedupeMemMB)
		fmt.Fprintln(os.Stderr, "      fingerprint, or unique candidates get dropped as false dupes.")
	}
}
