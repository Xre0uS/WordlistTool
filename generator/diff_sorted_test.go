package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLines writes the given lines, one per row, to dir/name; the empty
// string is a blank line.
func writeLines(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

func readOut(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output %s: %v", path, err)
	}
	return string(data)
}

// The two paths must agree: the sorted merge-join and the exact full-word
// filter over the same data, byte for byte.
func TestDiffSortedMergeMatchesFull(t *testing.T) {
	cases := []struct {
		name      string
		input     []string
		excludes  [][]string
		keepDupes bool
		want      string
	}{
		{
			name:     "sorted",
			input:    []string{"apple", "avocado", "banana", "cherry", "date", "grape"},
			excludes: [][]string{{"avocado", "cherry", "fig"}, {"apple", "zeta"}},
			want:     "banana\ndate\ngrape\n",
		},
		{
			name:     "adjacent duplicates collapse",
			input:    []string{"apple", "banana", "banana", "cherry", "cherry", "date"},
			excludes: [][]string{{"banana"}},
			want:     "apple\ncherry\ndate\n",
		},
		{
			name:      "keep duplicates",
			input:     []string{"apple", "banana", "banana", "cherry"},
			excludes:  [][]string{{"apple"}},
			keepDupes: true,
			want:      "banana\nbanana\ncherry\n",
		},
		{
			name:     "repeats of an excluded word",
			input:    []string{"bravo", "bravo", "charlie"},
			excludes: [][]string{{"bravo"}},
			want:     "charlie\n",
		},
		{
			name:      "repeats of an excluded word, keep duplicates",
			input:     []string{"bravo", "bravo", "charlie"},
			excludes:  [][]string{{"bravo"}},
			keepDupes: true,
			want:      "charlie\n",
		},
		{
			name:     "blank lines on both sides",
			input:    []string{"", "apple", "", "banana", "", "cherry"},
			excludes: [][]string{{"", "apple", "", "zeta"}},
			want:     "banana\ncherry\n",
		},
		{
			name:     "all covered",
			input:    []string{"apple", "banana"},
			excludes: [][]string{{"apple", "banana", "cherry"}},
			want:     "",
		},
		{
			name:     "byte order is not lex order",
			input:    []string{"ABC", "Abc", "aBC", "abc"},
			excludes: [][]string{{"Abc"}},
			want:     "ABC\naBC\nabc\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			in := writeLines(t, dir, "new.txt", c.input)
			var excl []string
			for i, x := range c.excludes {
				excl = append(excl, writeLines(t, dir, fmt.Sprintf("x%d.txt", i), x))
			}

			mergeOut := filepath.Join(dir, "merge.txt")
			mcfg := &DiffConfig{
				InputFiles:   []string{in},
				ExcludeFiles: excl,
				OutputFile:   mergeOut,
				NoConfirm:    true,
				Mode:         DiffFull,
				Sorted:       SortedForce,
				KeepDupes:    c.keepDupes,
			}
			if err := RunDiff(mcfg); err != nil {
				t.Fatalf("merge path: %v", err)
			}

			fullOut := filepath.Join(dir, "full.txt")
			fcfg := &DiffConfig{
				InputFiles:   []string{in},
				ExcludeFiles: excl,
				OutputFile:   fullOut,
				NoConfirm:    true,
				Mode:         DiffFull,
				Sorted:       SortedOff,
				KeepDupes:    c.keepDupes,
			}
			if err := RunDiff(fcfg); err != nil {
				t.Fatalf("full path: %v", err)
			}

			gotMerge := readOut(t, mergeOut)
			gotFull := readOut(t, fullOut)
			if gotMerge != gotFull {
				t.Fatalf("paths disagree:\nmerge: %q\nfull:  %q", gotMerge, gotFull)
			}
			if gotMerge != c.want {
				t.Fatalf("output = %q, want %q", gotMerge, c.want)
			}
		})
	}
}

// Several -x excludes merged at once; each contributes its own hits.
func TestDiffSortedMergeManyExcludes(t *testing.T) {
	dir := t.TempDir()
	in := writeLines(t, dir, "new.txt",
		[]string{"a", "b", "c", "d", "e", "f", "g", "h"})
	ex1 := writeLines(t, dir, "x1.txt", []string{"b", "f", "z"})
	ex2 := writeLines(t, dir, "x2.txt", []string{"d", "h"})
	ex3 := writeLines(t, dir, "x3.txt", []string{"c"})
	out := filepath.Join(dir, "out.txt")

	cfg := &DiffConfig{
		InputFiles:   []string{in},
		ExcludeFiles: []string{ex1, ex2, ex3},
		OutputFile:   out,
		NoConfirm:    true,
		Sorted:       SortedForce,
	}
	if err := RunDiff(cfg); err != nil {
		t.Fatalf("RunDiff: %v", err)
	}
	if got, want := readOut(t, out), "a\ne\ng\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// An input sorted at the head then going backwards mid-file: the streaming
// verifier stops with an error and no partial output is left behind.
func TestDiffSortedMergeVerifiesMidFile(t *testing.T) {
	dir := t.TempDir()
	in := writeLines(t, dir, "new.txt",
		[]string{"alpha", "bravo", "charlie", "bravo-tail"})
	ex := writeLines(t, dir, "x.txt", []string{"alpha"})
	out := filepath.Join(dir, "out.txt")

	cfg := &DiffConfig{
		InputFiles:   []string{in},
		ExcludeFiles: []string{ex},
		OutputFile:   out,
		NoConfirm:    true,
		Sorted:       SortedForce,
	}
	err := RunDiff(cfg)
	if err == nil {
		t.Fatalf("expected an out-of-order error, got nil")
	}
	const (
		file = "new.txt"
		ln1  = "line 4"
		ln2  = "line 3"
		w1   = "bravo-tail"
		w2   = "charlie"
		hint = "-no-sorted"
	)
	for _, s := range []string{file, ln1, ln2, w1, w2, hint} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q does not mention %q", err, s)
		}
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("partial output file left behind: %v", err)
	}
}

// A backwards exclude stream is caught the same way.
func TestDiffSortedMergeVerifiesExcludes(t *testing.T) {
	dir := t.TempDir()
	in := writeLines(t, dir, "new.txt", []string{"m", "n", "o"})
	ex := writeLines(t, dir, "x.txt", []string{"a", "c", "b"})
	out := filepath.Join(dir, "out.txt")

	cfg := &DiffConfig{
		InputFiles:   []string{in},
		ExcludeFiles: []string{ex},
		OutputFile:   out,
		NoConfirm:    true,
		Sorted:       SortedForce,
	}
	err := RunDiff(cfg)
	if err == nil {
		t.Fatalf("expected an out-of-order error, got nil")
	}
	if !strings.Contains(err.Error(), "x.txt") {
		t.Errorf("error %q does not name the exclude file", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("partial output file left behind: %v", err)
	}
}

// The probe finds an obviously unsorted input and the run falls back to the
// exact filter; the answer must still be right.
func TestDiffSortedAutoUnsortedFallsBack(t *testing.T) {
	dir := t.TempDir()
	// Sorted-looking at the head, backwards well inside the 32 MB sample.
	lines := make([]string, 0, 3)
	lines = append(lines, "0001", "0002", "0003", "0000")
	lines = append(lines, "0010", "0011")
	in := writeLines(t, dir, "new.txt", lines)
	ex := writeLines(t, dir, "x.txt", []string{"0003", "0011"})
	out := filepath.Join(dir, "out.txt")

	cfg := &DiffConfig{
		InputFiles:   []string{in},
		ExcludeFiles: []string{ex},
		OutputFile:   out,
		NoConfirm:    true,
		Mode:         DiffFull,
		Sorted:       SortedAuto,
	}
	if err := RunDiff(cfg); err != nil {
		t.Fatalf("RunDiff: %v", err)
	}
	// The filter path writes in input order, so the unsorted run keeps it.
	if got, want := readOut(t, out), "0001\n0002\n0000\n0010\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// -sorted forces the merge path: more than one input file is a hard error,
// not a silent fallback.
func TestDiffSortedForceMultipleInputs(t *testing.T) {
	dir := t.TempDir()
	a := writeLines(t, dir, "a.txt", []string{"a", "b"})
	b := writeLines(t, dir, "b.txt", []string{"c", "d"})
	ex := writeLines(t, dir, "x.txt", []string{"b"})
	out := filepath.Join(dir, "out.txt")

	cfg := &DiffConfig{
		InputFiles:   []string{a, b},
		ExcludeFiles: []string{ex},
		OutputFile:   out,
		NoConfirm:    true,
		Sorted:       SortedForce,
	}
	err := RunDiff(cfg)
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "-sorted") {
		t.Errorf("error %q does not mention -sorted", err)
	}
}
