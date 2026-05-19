#!/usr/bin/env bash
# spot_eval.sh — 4-scenario deep inspection of the SIGNAL pipeline.
# Shows the complete API response for each test case so you can judge quality
# by eye: what was asked, what was recommended, and the full reasoning.
#
# Usage:
#   ./scripts/spot_eval.sh                      # localhost:8080, gemini
#   ./scripts/spot_eval.sh http://host:port      # custom target
#   PROVIDER=kimi ./scripts/spot_eval.sh

set -euo pipefail

BASE="${1:-http://localhost:8080}"
PROVIDER="${PROVIDER:-gemini}"
SEP="$(printf '%.0s─' {1..70})"
DSEP="$(printf '%.0s═' {1..70})"

hdr() { printf '\n\033[1;33m%s\033[0m\n' "$*"; }
dim() { printf '\033[0;36m%s\033[0m\n'   "$*"; }
fail(){ printf '\033[0;31m✗\033[0m %s\n' "$*"; }
bold(){ printf '\n\033[1m%s\033[0m\n' "$*"; }

show_response() {
  local response="$1"

  bold "Full JSON response:"
  echo "$response" | python3 -m json.tool 2>/dev/null || echo "$response"

  if command -v jq &>/dev/null; then
    local requires_input
    requires_input="$(echo "$response" | jq -r '.requires_input // false')"

    if [ "$requires_input" = "true" ]; then
      printf '\n\033[1;33m→ requires_input=true. Question:\033[0m\n'
      echo "$response" | jq -r '.question'
    else
      bold "Reasoning:"
      echo "$response" | jq -r '.reasoning // "(none)"'
      local count
      count="$(echo "$response" | jq '.recommendations | length')"
      bold "Recommendations ($count items):"
      echo "$response" | jq -r '.recommendations[] |
        "  [\(.domain)] \(.item_id)\n  \(.search_text // "(no text)")\n  Reason: \(.reasoning // "-")\n  Score: \(.score)\n"'
    fi
  fi
}

call_and_show() {
  local label="$1"
  local payload="$2"

  hdr "SCENARIO: $label"
  dim "Provider: $PROVIDER | Target: $BASE"
  printf '%s\n' "$SEP"

  local response
  response="$(curl -sf -X POST "$BASE/recommend" \
    -H 'Content-Type: application/json' \
    -d "$payload")" || { fail "HTTP request failed"; return 1; }

  show_response "$response"
  printf '%s\n' "$SEP"
}

# Multi-turn: first call may return requires_input; if so, send follow-up and show final result.
call_multiturn() {
  local label="$1"
  local persona="$2"
  local first_user_msg="$3"
  local followup_answer="$4"

  hdr "SCENARIO: $label"
  dim "Provider: $PROVIDER | Target: $BASE"
  printf '%s\n' "$SEP"

  # ── Turn 1 ──
  bold "Turn 1 — initial message:"
  printf '  user: %s\n' "$first_user_msg"

  local payload1
  payload1="$(jq -n \
    --arg persona "$persona" \
    --arg msg "$first_user_msg" \
    --arg provider "$PROVIDER" \
    '{user_persona:$persona, history:[{role:"user",content:$msg}], limit:5, provider:$provider}')"

  local r1
  r1="$(curl -sf -X POST "$BASE/recommend" \
    -H 'Content-Type: application/json' \
    -d "$payload1")" || { fail "Turn 1 HTTP request failed"; return 1; }

  local requires_input
  requires_input="$(echo "$r1" | jq -r '.requires_input // false' 2>/dev/null || echo 'false')"

  if [ "$requires_input" = "true" ]; then
    bold "Turn 1 response (clarification request):"
    echo "$r1" | python3 -m json.tool 2>/dev/null || echo "$r1"

    local question
    question="$(echo "$r1" | jq -r '.question' 2>/dev/null || echo '')"
    printf '\n\033[1;33m→ API asked: %s\033[0m\n' "$question"

    # ── Turn 2 ──
    bold "Turn 2 — follow-up answer:"
    printf '  user: %s\n' "$followup_answer"

    local payload2
    payload2="$(jq -n \
      --arg persona "$persona" \
      --arg msg "$first_user_msg" \
      --arg q "$question" \
      --arg ans "$followup_answer" \
      --arg provider "$PROVIDER" \
      '{user_persona:$persona, history:[
        {role:"user",content:$msg},
        {role:"assistant",content:$q},
        {role:"user",content:$ans}
      ], limit:5, provider:$provider}')"

    local r2
    r2="$(curl -sf -X POST "$BASE/recommend" \
      -H 'Content-Type: application/json' \
      -d "$payload2")" || { fail "Turn 2 HTTP request failed"; return 1; }

    bold "Turn 2 response (final recommendations):"
    show_response "$r2"
  else
    printf '\n\033[0;32m→ Resolved in one turn (no clarification needed)\033[0m\n'
    show_response "$r1"
  fi

  printf '%s\n' "$SEP"
}

# ── Scenario 1: Cold-start (empty history) ────────────────────────────────────

call_and_show "Cold-start — empty history" "$(cat <<EOF
{
  "user_persona": "A busy Abuja professional in her late 20s.",
  "history": [],
  "limit": 5,
  "provider": "$PROVIDER"
}
EOF
)"

# ── Scenario 2: Ambiguous → follow-up → recommendations ──────────────────────

call_multiturn \
  "Multi-turn — vague ask resolved by follow-up" \
  "A 28-year-old man in Port Harcourt who works in oil and gas, reads occasionally, likes action and suspense." \
  "I want something good for the weekend." \
  "I want to read something — maybe a thriller or action novel, something gripping that I can finish in a day or two."

# ── Scenario 3: Books — single turn ──────────────────────────────────────────

call_and_show "Books — Nigerian reader, literary mood" "$(cat <<EOF
{
  "user_persona": "A 32-year-old Lagos-based software engineer who loves literary fiction, reads during commutes, and has been exploring African authors lately.",
  "history": [
    {"role": "user", "content": "I just finished Chimamanda's Americanah and I need something new. I want something that makes me think but isn't too heavy. Maybe something with a strong female protagonist or about identity."}
  ],
  "limit": 5,
  "provider": "$PROVIDER"
}
EOF
)"

# ── Scenario 4: Food — single turn ───────────────────────────────────────────

call_and_show "Food — expat, comfort craving" "$(cat <<EOF
{
  "user_persona": "A Nigerian expat living in London, misses home food, occasionally experiments with fusion, prefers cosy spots over fine dining.",
  "history": [
    {"role": "user", "content": "I want somewhere to eat this weekend. Something warm and filling. Not fast food but also not too fancy. I've been craving flavours from back home but I'm open to something new if it hits the same way."}
  ],
  "limit": 5,
  "provider": "$PROVIDER"
}
EOF
)"

printf '\n%s\n' "$DSEP"
printf 'spot_eval done. Review the recommendations above.\n'
printf 'For scored metrics: SEEDS_PER_DOMAIN=2 go run ./cmd/holdout_eval\n'
printf '%s\n' "$DSEP"
