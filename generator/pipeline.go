package generator

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Xre0uS/wordlisttool/config"
	"github.com/Xre0uS/wordlisttool/transform"
)

// Pipeline orchestrates multithreaded wordlist generation.
type Pipeline struct {
	cfg          *config.Config
	transformers []transform.Transformer
}

// NewPipeline creates a generation pipeline with the given config and transformers.
func NewPipeline(cfg *config.Config, transformers []transform.Transformer) *Pipeline {
	return &Pipeline{
		cfg:          cfg,
		transformers: transformers,
	}
}

func (p *Pipeline) Run() error {
	start := time.Now()

	keywords, err := ReadKeywords(p.cfg.InputFile)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	if len(keywords) == 0 {
		return fmt.Errorf("no keywords found in input file")
	}

	presetName := "standard"
	switch p.cfg.Preset {
	case config.PresetShort:
		presetName = "short"
	case config.PresetLong:
		presetName = "long"
	}

	fmt.Fprintf(os.Stderr, "[*] Loaded %d keywords\n", len(keywords))
	fmt.Fprintf(os.Stderr, "[*] Preset: %s | Threads: %d | Min length: %d\n",
		presetName, p.cfg.Threads, p.cfg.MinLength)
	fmt.Fprintf(os.Stderr, "[*] Running %d transform stages\n", len(p.transformers))

	keywordCh := make(chan string, p.cfg.Threads*2)

	// Batches channel: each batch is one transformer's output for one keyword.
	// This keeps memory bounded — at most (threads + buffer) batches alive.
	batchCh := make(chan []string, p.cfg.Threads*2)

	var totalGenerated int64
	var totalWritten int64
	var totalDupes int64

	var workerWg sync.WaitGroup
	for i := 0; i < p.cfg.Threads; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for keyword := range keywordCh {
				transform.StreamAll(p.transformers, keyword, func(batch []string) {
					n := int64(len(batch))
					atomic.AddInt64(&totalGenerated, n)

					if p.cfg.MaxCandidates > 0 && atomic.LoadInt64(&totalWritten) >= p.cfg.MaxCandidates {
						return
					}

					batchCh <- batch
				})
			}
		}()
	}

	writerDone := make(chan error, 1)
	go func() {
		writerDone <- p.writeOutput(batchCh, &totalWritten, &totalDupes)
	}()

	stopProgress := make(chan struct{})
	if p.cfg.Verbose {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					gen := atomic.LoadInt64(&totalGenerated)
					wrt := atomic.LoadInt64(&totalWritten)
					dup := atomic.LoadInt64(&totalDupes)
					elapsed := time.Since(start).Seconds()
					rate := float64(wrt) / elapsed
					fmt.Fprintf(os.Stderr, "\r[*] Generated: %d | Written: %d | Dupes: %d | Rate: %.0f/s     ",
						gen, wrt, dup, rate)
				case <-stopProgress:
					return
				}
			}
		}()
	}

	for _, kw := range keywords {
		keywordCh <- kw
		if p.cfg.MaxCandidates > 0 && atomic.LoadInt64(&totalGenerated) >= p.cfg.MaxCandidates {
			break
		}
	}
	close(keywordCh)

	workerWg.Wait()
	close(batchCh)

	if err := <-writerDone; err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	close(stopProgress)

	elapsed := time.Since(start)
	if p.cfg.Verbose {
		fmt.Fprintln(os.Stderr)
	}
	fmt.Fprintf(os.Stderr, "[+] Complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "[+] Generated: %s | Dupes removed: %s\n",
		commaFmt(atomic.LoadInt64(&totalGenerated)), commaFmt(atomic.LoadInt64(&totalDupes)))
	fmt.Fprintf(os.Stderr, "[+] Candidates written: %s\n", commaFmt(atomic.LoadInt64(&totalWritten)))
	if info, err := os.Stat(p.cfg.OutputFile); err == nil {
		fmt.Fprintf(os.Stderr, "[+] Output: %s (%s)\n", p.cfg.OutputFile, humanSize(info.Size()))
	} else {
		fmt.Fprintf(os.Stderr, "[+] Output: %s\n", p.cfg.OutputFile)
	}

	return nil
}

func (p *Pipeline) writeOutput(batches <-chan []string, totalWritten *int64, totalDupes *int64) error {
	f, err := os.Create(p.cfg.OutputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriterSize(f, 4*1024*1024)
	defer writer.Flush()

	minLen := p.cfg.MinLength

	var expectedItems int64 = 200_000_000
	if p.cfg.MaxCandidates > 0 && p.cfg.MaxCandidates < expectedItems {
		expectedItems = p.cfg.MaxCandidates
	}
	bf := newBloom(expectedItems, 0.001, 512)
	fmt.Fprintf(os.Stderr, "[*] Bloom filter: %d MB\n", bf.memoryMB())

	for batch := range batches {
		var written int64
		var dupes int64
		for _, candidate := range batch {
			if minLen > 0 && len(candidate) < minLen {
				continue
			}
			if bf.TestAndAdd(candidate) {
				dupes++
				continue
			}
			_, _ = writer.WriteString(candidate)
			_ = writer.WriteByte('\n')
			written++
		}
		atomic.AddInt64(totalWritten, written)
		atomic.AddInt64(totalDupes, dupes)
	}

	return nil
}

func ReadKeywords(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var keywords []string
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		line = trimSpace(line)
		if line == "" {
			continue
		}
		if _, exists := seen[line]; !exists {
			seen[line] = struct{}{}
			keywords = append(keywords, line)
		}
	}

	return keywords, scanner.Err()
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func commaFmt(n int64) string {
	if n < 0 {
		return "-" + commaFmt(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
