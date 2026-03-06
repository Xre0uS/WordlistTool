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

func estimateBloomMB(candidates int64) int {
	if candidates <= 0 {
		candidates = 1_000_000
	}
	n := float64(candidates)
	fpr := 0.001
	m := math.Ceil(-n * math.Log(fpr) / (math.Ln2 * math.Ln2))
	bytes := m / 8
	mb := int(math.Ceil(bytes / (1024 * 1024)))
	if mb > 512 {
		mb = 512
	}
	return mb
}

func PrintEstimateTable(results []EstimateResult, keywords []string) {
	fmt.Fprintf(os.Stderr, "\n[*] Estimated output for %d keyword(s):\n\n", len(keywords))
	fmt.Fprintf(os.Stderr, "    %-12s %18s %12s %12s %12s\n",
		"Preset", "Candidates", "File Size", "Bloom RAM", "Est. Time")
	fmt.Fprintf(os.Stderr, "    ──────────────────────────────────────────────────────────────────\n")
	for _, r := range results {
		bloomMB := estimateBloomMB(r.Count)
		fmt.Fprintf(os.Stderr, "    %-12s %18s %12s %10d MB %12s\n",
			r.Preset,
			commaFmt(r.Count),
			humanSize(r.TotalBytes),
			bloomMB,
			r.Duration.Round(time.Millisecond))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "    Bloom RAM is the dedup filter used during generation.")
	fmt.Fprintln(os.Stderr, "    Actual output may be slightly less after dedup.")
}
