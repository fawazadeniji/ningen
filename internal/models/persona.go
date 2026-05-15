package models

// Persona represents a user persona used for review simulation and recommendations.
type Persona struct {
	Name        string   `json:"name"`
	Age         int      `json:"age,omitempty"`
	Gender      string   `json:"gender,omitempty"`
	Location    string   `json:"location,omitempty"`
	Interests   []string `json:"interests,omitempty"`
	Occupation  string   `json:"occupation,omitempty"`
	TechSavvy   string   `json:"tech_savvy,omitempty"`   // e.g., "low", "medium", "high"
	BudgetRange string   `json:"budget_range,omitempty"` // e.g., "$0-50", "$50-200"
	Preferences map[string]string `json:"preferences,omitempty"`
}
