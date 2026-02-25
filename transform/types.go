package transform

// Transformer is the interface that all transformation modules implement.
// Given an input string, it returns all transformed variants.
type Transformer interface {
	// Name returns a human-readable name for this transformer
	Name() string
	// Transform takes an input string and returns all generated variants
	Transform(input string) []string
}

// ApplyAll runs every transformer on the input and collects unique results.
// NOTE: Only used for small-output contexts (e.g. base transforms).
// For the pipeline, use StreamAll instead.
func ApplyAll(transformers []Transformer, input string) []string {
	seen := make(map[string]struct{})
	var results []string

	for _, t := range transformers {
		for _, variant := range t.Transform(input) {
			if _, exists := seen[variant]; !exists {
				seen[variant] = struct{}{}
				results = append(results, variant)
			}
		}
	}

	return results
}

// StreamAll runs every transformer on the input and calls emit for each batch.
// If a transformer implements the Streamer interface, its streaming method is used
// to avoid materializing millions of results at once. Otherwise falls back to Transform.
// Returns the total number of candidates emitted.
func StreamAll(transformers []Transformer, input string, minLen int, emit func([]string)) int64 {
	var total int64

	filterAndEmit := func(results []string) {
		if minLen > 0 {
			n := 0
			for _, v := range results {
				if len(v) >= minLen {
					results[n] = v
					n++
				}
			}
			results = results[:n]
		}
		if len(results) > 0 {
			total += int64(len(results))
			emit(results)
		}
	}

	for _, t := range transformers {
		if s, ok := t.(Streamer); ok {
			// Use streaming path — emits in chunks, never materializes all at once
			s.TransformStream(input, filterAndEmit)
		} else {
			// Fallback: materialize and emit
			results := t.Transform(input)
			filterAndEmit(results)
		}
	}

	return total
}

// ApplyChain applies transformers in sequence: output of one feeds the next.
func ApplyChain(transformers []Transformer, input string) []string {
	current := []string{input}

	for _, t := range transformers {
		seen := make(map[string]struct{})
		var next []string
		for _, word := range current {
			for _, variant := range t.Transform(word) {
				if _, exists := seen[variant]; !exists {
					seen[variant] = struct{}{}
					next = append(next, variant)
				}
			}
		}
		current = next
	}

	return current
}
