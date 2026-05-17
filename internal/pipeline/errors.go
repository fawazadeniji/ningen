package pipeline

import "fmt"

// WorkflowError represents a failure during pipeline node execution.
type WorkflowError struct {
	Node  string // Name of the failed node: "profiler", "rater", "drafter", "critic"
	Cause error  // The underlying error
}

func (e *WorkflowError) Error() string {
	return fmt.Sprintf("%s step failed: %v", e.Node, e.Cause)
}

func (e *WorkflowError) Unwrap() error {
	return e.Cause
}

// NewWorkflowError creates a new WorkflowError with the given node name and cause.
func NewWorkflowError(node string, cause error) *WorkflowError {
	return &WorkflowError{Node: node, Cause: cause}
}
