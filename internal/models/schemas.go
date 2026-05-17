package models

// ConversationTurn represents a single message in a multi-turn chat history.
type ConversationTurn struct {
	Role    string `json:"role"`    // "user" | "assistant"
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
