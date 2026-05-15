package models

// Product represents product details used as input for Task A (review simulation).
type Product struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category,omitempty"`
	Brand       string   `json:"brand,omitempty"`
	Price       float64  `json:"price,omitempty"`
	Description string   `json:"description,omitempty"`
	Features    []string `json:"features,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
}
