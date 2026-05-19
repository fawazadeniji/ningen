const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// ─── Task A ───────────────────────────────────────────────────────────────────

export interface ReviewHistoryItem {
  item: string;
  rating: number;
  review: string;
}

export interface TargetItem {
  name: string;
  category: string;
  description: string;
}

export interface SimulateRequest {
  user_persona: string;
  review_history: ReviewHistoryItem[];
  target_item: TargetItem;
  provider?: string;
}

export interface SimulateResponse {
  rating: number;
  review: string;
  reasoning: string;
}

export async function simulate(req: SimulateRequest): Promise<SimulateResponse> {
  const res = await fetch(`${API_BASE}/simulate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error(`Simulate failed: ${res.status}`);
  return res.json();
}

// ─── Task B ───────────────────────────────────────────────────────────────────

export interface Message {
  role: "user" | "assistant";
  content: string;
}

export interface RecommendRequest {
  user_persona: string;
  history: Message[];
  cross_domain?: boolean;
  limit?: number;
  provider?: string;
}

export interface RecommendItem {
  item_id: string;
  domain: string;
  search_text: string;
  score: number;
}

export interface RecommendResponse {
  recommendations?: RecommendItem[];
  reasoning?: string;
  requires_input?: boolean;
  question?: string;
}

export async function recommend(req: RecommendRequest): Promise<RecommendResponse> {
  const res = await fetch(`${API_BASE}/recommend`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error(`Recommend failed: ${res.status}`);
  return res.json();
}