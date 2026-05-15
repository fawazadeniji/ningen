package models

// SimulatedReview represents a generated review from Task A.
type SimulatedReview struct {
	Rating     float64 `json:"rating"`      // 1.0 - 5.0
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Sentiment  string  `json:"sentiment"`   // "positive", "neutral", "negative"
	Confidence float64 `json:"confidence"`  // 0.0 - 1.0
}

// Recommendation represents a single product recommendation from Task B.
type Recommendation struct {
	Product   Product `json:"product"`
	Score     float64 `json:"score"`      // relevance score
	Reasoning string  `json:"reasoning"`  // explanation for the recommendation
}

// ReviewResponse is the API response for POST /simulate-review.
type ReviewResponse struct {
	Review  SimulatedReview `json:"review"`
	Persona Persona        `json:"persona"`
	Product Product        `json:"product"`
}

// RecommendResponse is the API response for POST /recommend.
type RecommendResponse struct {
	Recommendations []Recommendation `json:"recommendations"`
	Persona         Persona          `json:"persona"`
}
