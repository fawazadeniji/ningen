package models

// ConversationTurn represents a single message in a multi-turn chat history.
type ConversationTurn struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

// RecommendRequest is the payload for POST /recommend.
type RecommendRequest struct {
	UserPersona string             `json:"user_persona"`
	History     []ConversationTurn `json:"history"`
	CrossDomain bool               `json:"cross_domain"`
	Limit       int                `json:"limit"`
	// Provider selects the LLM backend: "kimi" | "gemini" | "openai"
	Provider string `json:"provider"`
	// NigerianFlavor controls whether the humanizer uses Nigerian cultural voice.
	// Defaults to true. Set to false to receive warm, neutral English instead.
	// Humanization always runs — this only switches the cultural register.
	NigerianFlavor *bool `json:"nigerian_flavor,omitempty"`
}

// RecommendedItem is a single item returned by the recommendation engine.
type RecommendedItem struct {
	ItemID     string  `json:"item_id"`
	Domain     string  `json:"domain"`
	SearchText string  `json:"search_text"`
	Score      float64 `json:"score"`
	Reasoning  string  `json:"reasoning,omitempty"` // per-item psychographic fit explanation
}

// RecommendResponse is the payload returned by POST /recommend.
// When RequiresInput is true, the engine lacks enough context to recommend;
// Question contains a humanized clarifying question for the user.
type RecommendResponse struct {
	Recommendations []RecommendedItem `json:"recommendations,omitempty"`
	Reasoning       string            `json:"reasoning,omitempty"`
	RequiresInput   bool              `json:"requires_input,omitempty"`
	Question        string            `json:"question,omitempty"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// DetailedErrorResponse provides structured error information with additional context.
type DetailedErrorResponse struct {
	Error   string         `json:"error"`
	Details map[string]any `json:"details,omitempty"`
}
