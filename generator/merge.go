package generator

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// MergeConfig holds configuration for the merge operation
type MergeConfig struct {
	InputFiles []string
	OutputFile string
	NoConfirm  bool
}

// RunMerge concatenates multiple wordlists into one output file in order.
// It stats all files first to show total size, asks for confirmation, then
// streams them sequentially with a large buffer for maximum throughput.
func RunMerge(cfg *MergeConfig) error {
	if len(cfg.InputFiles) == 0 {
		return fmt.Errorf("no input files specified")
	}

	// ── Stat all files and calculate total size ──
	type fileInfo struct {
		path string
		size int64
	}
	var files []fileInfo
	var totalBytes int64

	for _, path := range cfg.InputFiles {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("cannot stat %s: %w", path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory, not a file", path)
		}

		files = append(files, fileInfo{path: path, size: info.Size()})
		totalBytes += info.Size()
	}

	// ── Display summary ──
	fmt.Fprintf(os.Stderr, "[*] Merge %d wordlists → %s\n\n", len(files), cfg.OutputFile)
	for i, f := range files {
		fmt.Fprintf(os.Stderr, "    %d. %-40s %s\n", i+1, f.path, humanSize(f.size))
	}
	fmt.Fprintln(os.Stderr, "    "+strings.Repeat("─", 55))
	fmt.Fprintf(os.Stderr, "    Total: %s\n\n", humanSize(totalBytes))

	// ── Confirm ──
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

	// ── Merge ──
	start := time.Now()

	// Output must be a new file
	if _, err := os.Stat(cfg.OutputFile); err == nil {
		return fmt.Errorf("output file already exists: %s", cfg.OutputFile)
	}

	outFile, err := os.Create(cfg.OutputFile)
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
	}
	defer outFile.Close()

	// 8MB write buffer for throughput
	writer := bufio.NewWriterSize(outFile, 8*1024*1024)

	var totalWritten int64
	var totalLines int64
	buf := make([]byte, 8*1024*1024) // 8MB read buffer

	for i, f := range files {
		fmt.Fprintf(os.Stderr, "\r[*] Writing %d/%d: %s ...          ", i+1, len(files), f.path)

		inFile, err := os.Open(f.path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", f.path, err)
		}

		for {
			nr, readErr := inFile.Read(buf)
			if nr > 0 {
				chunk := buf[:nr]
				// Count newlines
				for _, b := range chunk {
					if b == '\n' {
						totalLines++
					}
				}
				nw, writeErr := writer.Write(chunk)
				totalWritten += int64(nw)
				if writeErr != nil {
					inFile.Close()
					return fmt.Errorf("writing %s: %w", f.path, writeErr)
				}
			}
			if readErr != nil {
				break
			}
		}
		inFile.Close()
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing output: %w", err)
	}

	elapsed := time.Since(start)
	rate := float64(totalWritten) / elapsed.Seconds() / (1024 * 1024)

	fmt.Fprintf(os.Stderr, "\r[+] Complete in %s                                    \n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "[+] Words: %s | Size: %s (%.0f MB/s)\n", commaFmt(totalLines), humanSize(totalWritten), rate)
	fmt.Fprintf(os.Stderr, "[+] Output: %s\n", cfg.OutputFile)

	return nil
}

// humanSize formats bytes into a human-readable string
func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
