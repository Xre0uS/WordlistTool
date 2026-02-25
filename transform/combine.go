package transform

// CombineTransformer chains multiple transformers: output of one feeds the next.
// This enables multi-stage transformations like leet -> append symbol -> append year.
type CombineTransformer struct {
	Stages          []Transformer
	MaxPerWord      int // max total output variants (0 = unlimited)
	MaxIntermediate int // max variants carried between non-final stages (0 = unlimited)
}

func (c *CombineTransformer) Name() string { return "combine" }

// Transform returns all results in a single slice. Used internally by other transformers.
// For pipeline use, prefer TransformStream to avoid large allocations.
func (c *CombineTransformer) Transform(input string) []string {
	var all []string
	c.TransformStream(input, func(batch []string) {
		all = append(all, batch...)
	})
	return all
}

// TransformStream processes the chain and emits final-stage results in chunks via emit.
// Intermediate stages are materialized (bounded by MaxIntermediate), but the final stage
// streams chunk-by-chunk so millions of results never need to live in memory at once.
func (c *CombineTransformer) TransformStream(input string, emit func([]string)) {
	maxPerWord := c.MaxPerWord
	maxIntermediate := c.MaxIntermediate

	if len(c.Stages) == 0 {
		emit([]string{input})
		return
	}

	// Build intermediate results through all stages except the last
	current := []string{input}
	for i := 0; i < len(c.Stages)-1; i++ {
		stage := c.Stages[i]
		cap := maxIntermediate // 0 = unlimited

		seen := make(map[string]struct{}, len(current)*2)
		var next []string
		for _, word := range current {
			for _, variant := range stage.Transform(word) {
				if _, exists := seen[variant]; !exists {
					seen[variant] = struct{}{}
					next = append(next, variant)
					if cap > 0 && len(next) >= cap {
						goto done
					}
				}
			}
		}
	done:
		current = next
	}

	// Stream the final stage in chunks
	{
		finalStage := c.Stages[len(c.Stages)-1]
		var emitted int
		const chunkSize = 64 * 1024 // flush every 64K candidates

		chunk := make([]string, 0, chunkSize)

		for _, word := range current {
			variants := finalStage.Transform(word)
			for _, v := range variants {
				chunk = append(chunk, v)
				if len(chunk) >= chunkSize {
					emit(chunk)
					emitted += len(chunk)
					if maxPerWord > 0 && emitted >= maxPerWord {
						return
					}
					chunk = make([]string, 0, chunkSize)
				}
			}
		}
		// Flush remaining
		if len(chunk) > 0 {
			if maxPerWord > 0 && emitted+len(chunk) > maxPerWord {
				chunk = chunk[:maxPerWord-emitted]
			}
			if len(chunk) > 0 {
				emit(chunk)
			}
		}
	}
}

// Streamer is an optional interface transformers can implement to support
// chunk-based streaming output instead of materializing everything.
type Streamer interface {
	TransformStream(input string, emit func([]string))
}

// MergeTransformer runs multiple transformers independently and merges results
type MergeTransformer struct {
	Transformers []Transformer
}

func (m *MergeTransformer) Name() string { return "merge" }

func (m *MergeTransformer) Transform(input string) []string {
	return ApplyAll(m.Transformers, input)
}
