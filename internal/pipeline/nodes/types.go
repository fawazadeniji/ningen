package nodes

// ExecutionTiming tracks the duration of each pipeline node in milliseconds.
type ExecutionTiming struct {
	ProfilerMs int64 `json:"profiler_ms"`
	RaterMs    int64 `json:"rater_ms"`
	DrafterMs  int64 `json:"drafter_ms"`
	CriticMs   int64 `json:"critic_ms"`
}

// AgentState represents the core state passed through the review workflow.
type AgentState struct {
	UserHistory   []HistoryEntry `json:"user_history"`
	TargetProduct TargetProduct  `json:"target_product"`
	// ModelOverrides allows specifying per-node model overrides for a single run.
	// Keys are node names: "profiler", "rater", "drafter", "critic".
	// Values are model identifiers (e.g. "gpt-5.4-mini"). Empty means use provider default.
	ModelOverrides map[string]string `json:"model_overrides"`
	// NigerianFlavor controls whether the drafter injects Nigerian vernacular and localizes
	// product details. Defaults to true (same behaviour as before the field existed).
	NigerianFlavor bool         `json:"nigerian_flavor"`
	UserProfile    *UserProfile `json:"user_profile"`

	PredictedRating float64 `json:"predicted_rating"`
	RatingReasoning string  `json:"rating_reasoning"`

	DraftReview string `json:"draft_review"`

	CriticVerdict  string `json:"critic_verdict"`
	CriticFeedback string `json:"critic_feedback"`
	Iterations     int    `json:"iterations"`

	FinalReview string `json:"final_review"`

	ExecutionTiming ExecutionTiming `json:"execution_timing"`
}

// ModelFor returns the model override for a given node, or empty string if none.
func (s *AgentState) ModelFor(node string) string {
	if s == nil || s.ModelOverrides == nil {
		return ""
	}
	return s.ModelOverrides[node]
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
	ConsumerPersona     string              `json:"consumer_persona"`
	FormattingQuirks    FormattingQuirks    `json:"formatting_quirks"`
	CulturalHooks       []string            `json:"cultural_hooks"`
}

// ProfilerResponse represents the structured output from the Profiler node.
type ProfilerResponse struct {
	UserID              string             `json:"user_id"`
	OverallTendency     string             `json:"overall_tendency"`
	ConsumerPersona     string             `json:"consumer_persona"`
	PreferredCategories []string           `json:"preferred_categories"`
	FormattingQuirks    FormattingQuirks   `json:"formatting_quirks"`
	ReviewStyle         ReviewStyle        `json:"review_style"`
	BehavioralMarkers   []BehavioralMarker `json:"behavioral_markers"`
	ToneProfile         ToneProfile        `json:"tone_profile"`
	TopicPreferences    []TopicPreference  `json:"topic_preferences"`
	CulturalHooks       []string           `json:"cultural_hooks"`
}

type FormattingQuirks struct {
	PunctuationHabits   string `json:"punctuation_habits"`
	CapitalizationStyle string `json:"capitalization_style"`
	EmojiUsage          string `json:"emoji_usage"`
}

type ReviewStyle struct {
	VerbosityLevel   string `json:"verbosity_level"`
	UseEmotionalLang bool   `json:"use_emotional_lang"`
	UseTechLanguage  bool   `json:"use_tech_language"`
}

type BehavioralMarker struct {
	Marker      string  `json:"marker"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
}

type ToneProfile struct {
	Cheerfulness float64 `json:"cheerfulness"`
	Sarcasm      float64 `json:"sarcasm"`
	Urgency      float64 `json:"urgency"`
	Formality    float64 `json:"formality"`
}

type TopicPreference struct {
	Topic      string `json:"topic"`
	Sentiment  string `json:"sentiment"`
	Importance string `json:"importance"`
}

type RatingPatterns struct {
	RatingsDistribution map[string]int   `json:"ratings_distribution"`
	RatingThresholds    RatingThresholds `json:"rating_thresholds"`
}

type RatingThresholds struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

type ReviewLengthProfile struct {
	AverageLength int `json:"average_length"`
	MinLength     int `json:"min_length"`
	MaxLength     int `json:"max_length"`
}
