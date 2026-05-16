package pipeline

import (
	"context"
	"errors"
	"fmt"

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
func InvokeGraph(ctx context.Context, workflow *Workflow, initialState AgentState) (AgentState, error) {
	if workflow == nil {
		return AgentState{}, errors.New("workflow is nil")
	}

	state := initialState
	var err error

	state, err = nodes.Profiler(workflow.llm)(ctx, state)
	if err != nil {
		return AgentState{}, fmt.Errorf("profiler step failed: %w", err)
	}

	state, err = nodes.Rater(workflow.llm)(ctx, state)
	if err != nil {
		return AgentState{}, fmt.Errorf("rater step failed: %w", err)
	}

	for {
		state, err = nodes.Drafter(workflow.llm)(ctx, state)
		if err != nil {
			return AgentState{}, fmt.Errorf("drafter step failed: %w", err)
		}

		state, err = nodes.Critic(workflow.llm)(ctx, state)
		if err != nil {
			return AgentState{}, fmt.Errorf("critic step failed: %w", err)
		}

		if state.CriticVerdict == "PASS" || state.Iterations >= maxLoops {
			if state.FinalReview == "" {
				state.FinalReview = state.DraftReview
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
