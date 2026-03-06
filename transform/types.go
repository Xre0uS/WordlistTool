package transform

// Transformer is the interface all transformation modules implement.
type Transformer interface {
	Name() string
	Transform(input string) []string
}

// ApplyAll runs every transformer and collects unique results.
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

// StreamAll runs transformers and emits results in batches via emit.
// Uses streaming where supported to avoid materializing everything in memory.
func StreamAll(transformers []Transformer, input string, emit func([]string)) int64 {
	var total int64

	for _, t := range transformers {
		if s, ok := t.(Streamer); ok {
			s.TransformStream(input, func(batch []string) {
				total += int64(len(batch))
				emit(batch)
			})
		} else {
			results := t.Transform(input)
			if len(results) > 0 {
				total += int64(len(results))
				emit(results)
			}
		}
	}

	return total
}

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
