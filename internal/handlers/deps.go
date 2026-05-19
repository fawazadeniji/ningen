package handlers

import (
	"encoding/json"
	"net/http"

	"ningen/embed"
	"ningen/internal/llm"
	"ningen/internal/models"
	"ningen/internal/rag"
)

// Deps holds all shared dependencies injected into every handler.
// Adding a new dependency here propagates to all handlers without changing signatures.
type Deps struct {
	LLM    llm.Registry
	Vector *rag.VectorStore
	Embed  embed.Embedder
}

// writeJSON serialises v as JSON into w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// writeError writes a standard error envelope.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, models.ErrorResponse{Error: msg})
}

// writeDetailedError writes an error envelope with additional context details.
func writeDetailedError(w http.ResponseWriter, status int, msg string, details map[string]interface{}) {
	writeJSON(w, status, models.DetailedErrorResponse{
		Error:   msg,
		Details: details,
	})
}

// decode unmarshals the request body into dst and returns false on error,
// having already written the appropriate error response.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB cap
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}
