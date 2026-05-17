package handlers

import (
	"net/http"
	"strings"

	"ningen/internal/pipeline"
)

// GenerateReviewRequest is the payload for POST /generate-review.
type GenerateReviewRequest struct {
	UserHistory   []ReviewHistoryEntry `json:"user_history"`
	TargetProduct ReviewTargetProduct  `json:"target_product"`
	Provider      string               `json:"provider"`
}

// ReviewHistoryEntry describes one item from the user's prior review history.
type ReviewHistoryEntry = pipeline.HistoryEntry

// ReviewTargetProduct describes the item for which a review is being generated.
type ReviewTargetProduct = pipeline.TargetProduct

// ReviewGenerationResponse is the payload returned by POST /generate_review.
type ReviewGenerationResponse struct {
	GeneratedReview string             `json:"generated_review"`
	PredictedRating float64            `json:"predicted_rating"`
	RatingReasoning string             `json:"rating_reasoning"`
	UserProfile     *ReviewUserProfile `json:"user_profile"`
	Iterations      int                `json:"iterations"`
}

// ReviewUserProfile captures the behavioral profile inferred from history.
type ReviewUserProfile = pipeline.UserProfile

// GenerateReviewHandler serves POST /generate_review.
// It runs the review workflow through the pipeline package.
func GenerateReviewHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req GenerateReviewRequest
		if !decode(w, r, &req) {
			return
		}

		if len(req.UserHistory) == 0 {
			writeError(w, http.StatusBadRequest, "user_history is required")
			return
		}
		if req.TargetProduct.ProductID == "" {
			writeError(w, http.StatusBadRequest, "target_product with product_id is required")
			return
		}

		providerKey := strings.TrimSpace(strings.ToLower(req.Provider))
		if providerKey == "" {
			providerKey = "kimi" // default provider
		}

		model, exists := d.LLM[providerKey]
		if !exists {
			writeError(w, http.StatusBadRequest, "requested provider "+providerKey+" is not available")
			return
		}

		state, err := pipeline.ExecuteWorkflow(r.Context(), model, pipeline.AgentState{
			UserHistory:   req.UserHistory,
			TargetProduct: req.TargetProduct,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "review workflow failed: "+err.Error())
			return
		}

		generatedReview := state.FinalReview
		if generatedReview == "" {
			generatedReview = state.DraftReview
		}

		writeJSON(w, http.StatusOK, ReviewGenerationResponse{
			GeneratedReview: generatedReview,
			PredictedRating: state.PredictedRating,
			RatingReasoning: state.RatingReasoning,
			UserProfile:     state.UserProfile,
			Iterations:      state.Iterations,
		})
	}
}
