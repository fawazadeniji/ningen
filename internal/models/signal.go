package models

// UserSignal is the structured intent profile extracted by the Signal Extractor agent.
// It is the shared contract between all downstream stages of the SIGNAL pipeline.
type UserSignal struct {
	Intent        string   `json:"intent"`         // primary intent phrase
	Domain        string   `json:"domain"`         // "books" | "food" | "products" | "mixed"
	SearchQueries []string `json:"search_queries"` // 2 focused phrases for pgvector embedding
	Mood          string   `json:"mood"`           // emotional register: "adventurous", "cozy", etc.
	Constraints   []string `json:"constraints"`    // hard filters to respect: "no horror", "vegetarian"
	ClarifyNeeded bool     `json:"clarify_needed"` // true means history is too ambiguous to retrieve
	ClarifyReason string   `json:"clarify_reason"` // raw clarifying question; humanized before response
}
