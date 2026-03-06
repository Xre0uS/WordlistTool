package transform

// CombineTransformer chains multiple transformers: output of one feeds the next.
type CombineTransformer struct {
	Stages          []Transformer
	MaxPerWord      int // max total output per input word (0 = unlimited)
	MaxIntermediate int // max variants carried between non-final stages (0 = unlimited)
}

func (c *CombineTransformer) Name() string { return "combine" }

func (c *CombineTransformer) Transform(input string) []string {
	var all []string
	c.TransformStream(input, func(batch []string) {
		all = append(all, batch...)
	})
	return all
}

func (c *CombineTransformer) TransformStream(input string, emit func([]string)) {
	maxPerWord := c.MaxPerWord
	maxIntermediate := c.MaxIntermediate

	if len(c.Stages) == 0 {
		emit([]string{input})
		return
	}

	current := []string{input}
	for i := 0; i < len(c.Stages)-1; i++ {
		stage := c.Stages[i]
		cap := maxIntermediate

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

	// Stream the final stage
	{
		finalStage := c.Stages[len(c.Stages)-1]
		var emitted int
		const chunkSize = 64 * 1024

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

// Streamer is an optional interface for chunk-based streaming output.
type Streamer interface {
	TransformStream(input string, emit func([]string))
}

// MergeTransformer runs multiple transformers independently and merges results.
type MergeTransformer struct {
	Transformers []Transformer
}

func (m *MergeTransformer) Name() string { return "merge" }

func (m *MergeTransformer) Transform(input string) []string {
	return ApplyAll(m.Transformers, input)
}
