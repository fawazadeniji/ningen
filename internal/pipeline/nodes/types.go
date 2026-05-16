package nodes

// AgentState represents the core state passed through the review workflow.
type AgentState struct {
	UserHistory   []HistoryEntry `json:"user_history"`
	TargetProduct TargetProduct  `json:"target_product"`

	UserProfile *UserProfile `json:"user_profile"`

	PredictedRating float64 `json:"predicted_rating"`
	RatingReasoning string  `json:"rating_reasoning"`

	DraftReview string `json:"draft_review"`

	CriticVerdict  string `json:"critic_verdict"`
	CriticFeedback string `json:"critic_feedback"`
	Iterations     int    `json:"iterations"`

	FinalReview string `json:"final_review"`
}

// HistoryEntry represents one review from the user's history.
type HistoryEntry struct {
	ProductID       string  `json:"product_id"`
	ProductName     string  `json:"product_name"`
	ProductCategory string  `json:"product_category"`
	StarRating      float64 `json:"star_rating"`
	ReviewText      string  `json:"review_text"`
	ReviewDate      string  `json:"review_date"`
	Source          string  `json:"source"`
}

// TargetProduct is the item being reviewed.
type TargetProduct struct {
	ProductID       string   `json:"product_id"`
	ProductName     string   `json:"product_name"`
	ProductCategory string   `json:"product_category"`
	Description     string   `json:"description"`
	Price           float64  `json:"price"`
	Currency        string   `json:"currency"`
	Source          string   `json:"source"`
	Features        []string `json:"features"`
	Rating          float64  `json:"rating"`
	ReviewCount     int      `json:"review_count"`
}

// UserProfile captures the structured user behavior summary.
type UserProfile struct {
	UserID              string              `json:"user_id"`
	OverallTendency     string              `json:"overall_tendency"`
	AverageRating       float64             `json:"average_rating"`
	PreferredCategories []string            `json:"preferred_categories"`
	ReviewStyle         ReviewStyle         `json:"review_style"`
	BehavioralMarkers   []BehavioralMarker  `json:"behavioral_markers"`
	ToneProfile         ToneProfile         `json:"tone_profile"`
	RatingPatterns      RatingPatterns      `json:"rating_patterns"`
	TopicPreferences    []TopicPreference   `json:"topic_preferences"`
	ReviewLength        ReviewLengthProfile `json:"review_length"`
}

type ReviewStyle struct {
	DetailLevel      string `json:"detail_level"`
	UseEmotionalLang bool   `json:"use_emotional_lang"`
	UseTechLanguage  bool   `json:"use_tech_language"`
	ComparisonFreq   string `json:"comparison_frequency"`
}

type BehavioralMarker struct {
	Marker      string  `json:"marker"`
	Frequency   string  `json:"frequency"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
}

type ToneProfile struct {
	Cheerfulness float64 `json:"cheerfulness"`
	Sarcasm      float64 `json:"sarcasm"`
	Urgency      float64 `json:"urgency"`
	Formality    float64 `json:"formality"`
}

type RatingPatterns struct {
	RatingsDistribution map[string]int     `json:"ratings_distribution"`
	RatingThresholds    map[string]float64 `json:"rating_thresholds"`
}

type TopicPreference struct {
	Topic      string `json:"topic"`
	Sentiment  string `json:"sentiment"`
	Frequency  int    `json:"frequency"`
	Importance string `json:"importance"`
}

type ReviewLengthProfile struct {
	AverageLength float64 `json:"average_length"`
	MinLength     float64 `json:"min_length"`
	MaxLength     float64 `json:"max_length"`
}
