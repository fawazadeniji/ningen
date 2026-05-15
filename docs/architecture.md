# Architecture Notes

## Overview

Ningen is an AI-powered product review simulation and recommendation engine built with Go, Gin, and LangChainGo.

## Task A: Simulate Review (`POST /simulate-review`)

Given a user persona and product details, generate a realistic simulated review including rating, title, body, and sentiment.

## Task B: Recommend (`POST /recommend`)

Given a user persona, return a ranked list of contextual product recommendations with reasoning.

## Agent Architecture

- **Modeling Agent** (`internal/agents/modeling/`): LangChainGo/LangGraph workflows for user modeling and review generation.
- **Recommender Agent** (`internal/agents/recommender/`): Contextual retrieval and reasoning for product recommendations.
- **Tools** (`internal/agents/tools/`): Shared LangChain tools (search, DB lookup, etc.).

## Ablation Studies

_TODO: Document experiments and ablation studies here._
