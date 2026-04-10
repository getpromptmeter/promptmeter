package datagen

import "math/rand/v2"

// TagTemplate defines a set of possible values for a tag key.
type TagTemplate struct {
	Key    string
	Values []string
}

// DefaultTags defines the tag templates used for data generation.
var DefaultTags = []TagTemplate{
	{Key: "feature", Values: []string{"chat-support", "doc-generation", "code-review", "search", "summarization"}},
	{Key: "team", Values: []string{"backend", "frontend", "ml", "product", "support"}},
	{Key: "environment", Values: []string{"production", "staging"}},
}

// generateTags generates a realistic set of tags for an event.
// feature and team are always present; environment is included 70% of the time.
// Environment uses an 80/20 split (production/staging).
func generateTags(rng *rand.Rand) map[string]string {
	tags := make(map[string]string, 3)

	// feature -- always present, uniform distribution.
	tags["feature"] = DefaultTags[0].Values[rng.IntN(len(DefaultTags[0].Values))]

	// team -- always present, uniform distribution.
	tags["team"] = DefaultTags[1].Values[rng.IntN(len(DefaultTags[1].Values))]

	// environment -- 70% of events.
	if rng.Float64() < 0.70 {
		if rng.Float64() < 0.80 {
			tags["environment"] = "production"
		} else {
			tags["environment"] = "staging"
		}
	}

	return tags
}
