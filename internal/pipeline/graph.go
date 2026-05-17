package pipeline

import (
	"context"
	"errors"
	"time"

	"ningen/internal/llm"
	"ningen/internal/pipeline/nodes"
)

const maxLoops = 2

// Workflow is a lightweight wrapper around the node pipeline.
type Workflow struct {
	llm llm.LLMProvider
}

// BuildGraph prepares the review workflow using the configured LLM.
// The current implementation executes the steps sequentially and applies the
// critic loop in-process, which keeps the package compiling against the actual
// langgraphgo API surface used in this repository.
func BuildGraph(model llm.LLMProvider) (*Workflow, error) {
	if model == nil {
		return nil, errors.New("llm model is required")
	}
	return &Workflow{llm: model}, nil
}

// InvokeGraph runs the workflow from the provided initial state.
// It measures execution time for each node and ensures FinalReview is always set.
func InvokeGraph(ctx context.Context, workflow *Workflow, initialState AgentState) (AgentState, error) {
	if workflow == nil {
		return AgentState{}, errors.New("workflow is nil")
	}

	state := initialState
	state.ExecutionTiming = nodes.ExecutionTiming{}
	var err error

	// Profiler
	start := time.Now()
	state, err = nodes.Profiler(workflow.llm)(ctx, state)
	state.ExecutionTiming.ProfilerMs = time.Since(start).Milliseconds()
	if err != nil {
		return AgentState{}, NewWorkflowError("profiler", err)
	}

	// Rater
	start = time.Now()
	state, err = nodes.Rater(workflow.llm)(ctx, state)
	state.ExecutionTiming.RaterMs = time.Since(start).Milliseconds()
	if err != nil {
		return AgentState{}, NewWorkflowError("rater", err)
	}

	// Drafter + Critic loop
	for {
		start = time.Now()
		state, err = nodes.Drafter(workflow.llm)(ctx, state)
		state.ExecutionTiming.DrafterMs = time.Since(start).Milliseconds()
		if err != nil {
			return AgentState{}, NewWorkflowError("drafter", err)
		}

		start = time.Now()
		state, err = nodes.Critic(workflow.llm)(ctx, state)
		state.ExecutionTiming.CriticMs = time.Since(start).Milliseconds()
		if err != nil {
			return AgentState{}, NewWorkflowError("critic", err)
		}

		if state.CriticVerdict == "PASS" || state.Iterations >= maxLoops {
			// Ensure FinalReview is always set
			if state.FinalReview == "" {
				state.FinalReview = state.DraftReview
			}
			// Set verdict to indicate why the loop ended
			if state.CriticVerdict != "PASS" {
				state.CriticVerdict = "MAX_ITERATIONS"
			}
			return state, nil
		}
	}
}

// ExecuteWorkflow builds the workflow and runs it in one call.
func ExecuteWorkflow(ctx context.Context, model llm.LLMProvider, initialState AgentState) (AgentState, error) {
	workflow, err := BuildGraph(model)
	if err != nil {
		return AgentState{}, err
	}

	return InvokeGraph(ctx, workflow, initialState)
}
