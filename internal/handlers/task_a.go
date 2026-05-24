package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ningen/internal/pipeline"
	"ningen/internal/pipeline/nodes"
	"ningen/internal/validation"
)

// GenerateReviewRequest is the payload for POST /generate-review.
type GenerateReviewRequest struct {
	UserHistory    []ReviewHistoryEntry `json:"user_history"`
	TargetProduct  ReviewTargetProduct  `json:"target_product"`
	Provider       string               `json:"provider"`
	ModelOverrides map[string]string    `json:"model_overrides"` // Optional: per-node model overrides
	// NigerianFlavor controls whether the review uses Nigerian vernacular and localized product context.
	// Defaults to true when omitted.
	NigerianFlavor *bool `json:"nigerian_flavor,omitempty"`
}

// ReviewHistoryEntry describes one item from the user's prior review history.
type ReviewHistoryEntry = pipeline.HistoryEntry

// ReviewTargetProduct describes the item for which a review is being generated.
type ReviewTargetProduct = pipeline.TargetProduct

// ReviewGenerationResponse is the payload returned by POST /generate-review.
type ReviewGenerationResponse struct {
	GeneratedReview string                `json:"generated_review"`
	PredictedRating float64               `json:"predicted_rating"`
	CriticVerdict   string                `json:"critic_verdict"` // "PASS" or "MAX_ITERATIONS"
	IterationsUsed  int                   `json:"iterations_used"`
	ExecutionTiming nodes.ExecutionTiming `json:"execution_timing"`
}

// ReviewUserProfile captures the behavioral profile inferred from history.
type ReviewUserProfile = pipeline.UserProfile

// GenerateReviewHandler serves POST /generate-review.
// It validates the request, enforces a timeout, and runs the review workflow through the pipeline.
// The handler applies strict input validation and returns detailed execution diagnostics.
const workflowTimeout = 90 * time.Second

func GenerateReviewHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req GenerateReviewRequest
		if !decode(w, r, &req) {
			return
		}

		// Validate user history
		if err := validation.ValidateUserHistory(req.UserHistory); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate target product
		if err := validation.ValidateTargetProduct(req.TargetProduct); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Normalize and validate provider
		providerKey := normalizeProvider(req.Provider)
		model, exists := d.LLM[providerKey]
		if !exists {
			availableProviders := make([]string, 0, len(d.LLM))
			for k := range d.LLM {
				availableProviders = append(availableProviders, k)
			}
			writeDetailedError(w, http.StatusBadRequest, "requested provider not available", map[string]interface{}{
				"requested_provider":  providerKey,
				"available_providers": availableProviders,
			})
			return
		}

		// Create a context with timeout for the workflow
		ctx, cancel := context.WithTimeout(r.Context(), workflowTimeout)
		defer cancel()

		nigerian := req.NigerianFlavor == nil || *req.NigerianFlavor
		state, err := pipeline.ExecuteWorkflow(ctx, model, pipeline.AgentState{
			UserHistory:    req.UserHistory,
			TargetProduct:  req.TargetProduct,
			ModelOverrides: req.ModelOverrides,
			NigerianFlavor: nigerian,
		})
		if err != nil {
			// Check if it's a workflow error with node information
			var wfErr *pipeline.WorkflowError
			if errors.As(err, &wfErr) {
				writeDetailedError(w, http.StatusInternalServerError, "workflow execution failed", map[string]interface{}{
					"failed_node": wfErr.Node,
					"cause":       wfErr.Cause.Error(),
				})
				return
			}
			writeError(w, http.StatusInternalServerError, "review workflow failed: "+err.Error())
			return
		}

		// Ensure FinalReview is always set (should be set by pipeline, but be defensive)
		finalReview := state.FinalReview
		if finalReview == "" {
			finalReview = state.DraftReview
		}

		writeJSON(w, http.StatusOK, ReviewGenerationResponse{
			GeneratedReview: finalReview,
			PredictedRating: state.PredictedRating,
			CriticVerdict:   state.CriticVerdict,
			IterationsUsed:  state.Iterations,
			ExecutionTiming: state.ExecutionTiming,
		})
	}
}

// normalizeProvider converts user input to lowercase and defaults to "kimi".
func normalizeProvider(p string) string {
	normalized := strings.TrimSpace(strings.ToLower(p))
	if normalized == "" {
		return "openai"
	}
	return normalized
}
