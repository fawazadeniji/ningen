#!/usr/bin/env bash
# Evaluation harness for the Ningen recommendation API.
# Exercises all SIGNAL pipeline stages and prints pass/fail with latency.
#
# Usage:
#   ./scripts/eval.sh                     # target localhost:8080
#   ./scripts/eval.sh http://host:port    # custom target
#   PROVIDER=kimi ./scripts/eval.sh       # test a specific LLM provider

set -euo pipefail

BASE="${1:-http://localhost:8080}"
PROVIDER="${PROVIDER:-gemini}"

# ── colours ────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

pass() { echo -e "  ${GREEN}✓${RESET} $*"; }
fail() { echo -e "  ${RED}✗${RESET} $*"; FAILURES=$((FAILURES + 1)); }
info() { echo -e "  ${CYAN}→${RESET} $*"; }
header() { echo -e "\n${BOLD}${YELLOW}$*${RESET}"; }

FAILURES=0
TOTAL=0

# ── helpers ────────────────────────────────────────────────────────────────
# call <label> <expected_field> <json_body>
# Posts to /recommend, checks the expected top-level JSON field is present and non-empty.
call() {
  local label="$1" expected="$2" body="$3"
  TOTAL=$((TOTAL + 1))

  local start elapsed response http_code

  start=$(date +%s%N)
  response=$(curl -s -w "\n%{http_code}" -X POST "$BASE/recommend" \
    -H "Content-Type: application/json" \
    -d "$body" 2>/dev/null)
  elapsed=$(( ($(date +%s%N) - start) / 1000000 ))

  http_code=$(echo "$response" | tail -1)
  local body_only
  body_only=$(echo "$response" | sed '$d')

  if [[ "$http_code" != "200" ]]; then
    fail "$label → HTTP $http_code (${elapsed}ms)"
    info "$(echo "$body_only" | python3 -m json.tool 2>/dev/null || echo "$body_only")"
    return
  fi

  local value items reasoning
  value=$(echo "$body_only" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('$expected',''))" 2>/dev/null)
  [[ $? -ne 0 ]] && value=""

  if [[ -z "$value" || "$value" == "False" || "$value" == "[]" || "$value" == "None" ]]; then
    fail "$label → '$expected' missing or empty (${elapsed}ms)"
    echo "$body_only" | python3 -m json.tool 2>/dev/null | head -20 | sed 's/^/    /'
  else
    pass "$label (${elapsed}ms)"
    items=$(echo "$body_only" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('recommendations',[])))" 2>/dev/null)
    [[ $? -ne 0 ]] && items="?"
    reasoning=$(echo "$body_only" | python3 -c "import sys,json; d=json.load(sys.stdin); r=d.get('reasoning',''); print(r[:120]+'...' if len(r)>120 else r)" 2>/dev/null)
    [[ $? -ne 0 ]] && reasoning=""
    [[ -n "$items" && "$items" != "0" ]] && info "items returned: $items"
    [[ -n "$reasoning" ]] && info "reasoning: $reasoning"
  fi
}

# ── health check ───────────────────────────────────────────────────────────
header "0. Health Check"
TOTAL=$((TOTAL + 1))
hc=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/health")
if [[ "$hc" == "200" ]]; then
  pass "GET /health → 200"
else
  fail "GET /health → $hc (API not up?)"
  echo -e "\n${RED}API is not responding. Aborting.${RESET}"
  exit 1
fi

# ── cold-start ─────────────────────────────────────────────────────────────
header "1. Cold-Start Gate (empty history → clarifying question)"

call "Empty history returns requires_input" "requires_input" \
  "{\"user_persona\":\"A curious reader from Abuja\",\"history\":[],\"provider\":\"$PROVIDER\"}"

# ── single-turn recommendations ────────────────────────────────────────────
header "2. Single-Turn Recommendations"

call "Books — sci-fi for a long flight" "recommendations" \
  "{
    \"user_persona\": \"A Lagos software engineer in her 30s who loves science fiction and long hauls.\",
    \"history\": [{\"role\":\"user\",\"content\":\"I need something to read on a 7-hour flight, preferably sci-fi or thriller.\"}],
    \"limit\": 5,
    \"provider\": \"$PROVIDER\"
  }"

call "Food — jollof rice near Port Harcourt" "recommendations" \
  "{
    \"user_persona\": \"A food lover based in Port Harcourt who eats out often.\",
    \"history\": [{\"role\":\"user\",\"content\":\"Where can I get the best jollof rice or pepper soup? Something local and affordable.\"}],
    \"limit\": 5,
    \"provider\": \"$PROVIDER\"
  }"

call "Products — home office setup on a budget" "recommendations" \
  "{
    \"user_persona\": \"A remote worker who just moved to Ibadan and needs a home office setup.\",
    \"history\": [{\"role\":\"user\",\"content\":\"I need a good chair, monitor, and headphones for remote work. Budget is tight.\"}],
    \"limit\": 5,
    \"provider\": \"$PROVIDER\"
  }"

# ── multi-turn conversation ────────────────────────────────────────────────
header "3. Multi-Turn Conversation (context carries over)"

call "Multi-turn refinement" "recommendations" \
  "{
    \"user_persona\": \"A 28-year-old teacher in Lagos who reads historical fiction and loves Nigerian authors.\",
    \"history\": [
      {\"role\":\"user\",      \"content\":\"I want a book recommendation.\"},
      {\"role\":\"assistant\", \"content\":\"Sure! What genre or mood are you in for?\"},
      {\"role\":\"user\",      \"content\":\"Something historical, preferably set in Africa or written by an African author.\"}
    ],
    \"limit\": 5,
    \"provider\": \"$PROVIDER\"
  }"

# ── cross-domain ───────────────────────────────────────────────────────────
header "4. Cross-Domain Mode"

call "Cross-domain: books + products for a student" "recommendations" \
  "{
    \"user_persona\": \"A final-year engineering student in Kaduna preparing for exams and job hunting.\",
    \"history\": [{\"role\":\"user\",\"content\":\"I need things that will help me study better and prepare for interviews.\"}],
    \"cross_domain\": true,
    \"limit\": 8,
    \"provider\": \"$PROVIDER\"
  }"

# ── ambiguous intent (should trigger clarify or degrade gracefully) ─────────
header "5. Ambiguous Intent Handling"

TOTAL=$((TOTAL + 1))
resp=$(curl -s -X POST "$BASE/recommend" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_persona\": \"Someone.\",
    \"history\": [{\"role\":\"user\",\"content\":\"I want something good.\"}],
    \"provider\": \"$PROVIDER\"
  }")

has_recs=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(bool(d.get('recommendations')))" 2>/dev/null)
[[ $? -ne 0 ]] && has_recs="False"
has_q=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(bool(d.get('requires_input')))" 2>/dev/null)
[[ $? -ne 0 ]] && has_q="False"

if [[ "$has_recs" == "True" || "$has_q" == "True" ]]; then
  pass "Ambiguous intent handled (got recs or clarifying question)"
  [[ "$has_q" == "True" ]] && info "Gate/Extractor chose to ask: $(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('question',''))" 2>/dev/null)"
else
  fail "Ambiguous intent returned neither recommendations nor a question"
fi

# ── provider fallback ──────────────────────────────────────────────────────
header "6. Provider Selection"

for p in gemini kimi openai; do
  call "Provider: $p" "recommendations" \
    "{
      \"user_persona\": \"A Nairobi-based Nigerian expat who reads novels and watches Nollywood.\",
      \"history\": [{\"role\":\"user\",\"content\":\"Recommend me a novel to read this weekend.\"}],
      \"limit\": 3,
      \"provider\": \"$p\"
    }"
done

# ── summary ────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}────────────────────────────────${RESET}"
PASSED=$((TOTAL - FAILURES))
if [[ "$FAILURES" -eq 0 ]]; then
  echo -e "${GREEN}${BOLD}All $TOTAL tests passed.${RESET}"
else
  echo -e "${RED}${BOLD}$FAILURES/$TOTAL tests failed.${RESET}"
fi
echo ""

exit $FAILURES
