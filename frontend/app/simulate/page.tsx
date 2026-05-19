/* eslint-disable @typescript-eslint/no-explicit-any */
"use client";

import { useState } from "react";
import {
  simulate,
  type SimulateRequest,
  type SimulateResponse,
  type ReviewHistoryItem,
} from "@/lib/api";

import {
  Plus,
  Trash2,
  Sparkles,
  User,
  Package,
  Bot,
  Loader2,
} from "lucide-react";

/* ------------------------------ DATA -------------------------------- */

const PROVIDERS = ["gemini", "openai", "kimi"];

const DEFAULT_HISTORY: ReviewHistoryItem[] = [
  {
    item: "Logitech MX Master 3 Mouse",
    rating: 5,
    review:
      "Absolute beast of a mouse. The scroll wheel alone is worth the price.",
  },
];

/* ------------------------------ UI -------------------------------- */

function StarDisplay({ rating }: { rating: number }) {
  return (
    <div className="flex items-center gap-1">
      {[1, 2, 3, 4, 5].map((s) => (
        <div
          key={s}
          className={`w-4 h-4 rounded-sm ${s <= rating ? "bg-amber-400" : "bg-[#E5E4DF]"
            }`}
        />
      ))}
      <span className="ml-2 text-2xl font-semibold">{rating}</span>
      <span className="text-[#999] text-sm">/5</span>
    </div>
  );
}

/* ------------------------------ PAGE -------------------------------- */

export default function SimulatePage() {
  const [persona, setPersona] = useState(
    "A Lagos-based software engineer in his 30s, critical of quality and value."
  );

  const [history, setHistory] =
    useState<ReviewHistoryItem[]>(DEFAULT_HISTORY);

  const [target, setTarget] = useState({
    name: "Razer DeathAdder V3",
    category: "Electronics",
    description: "Ergonomic gaming mouse, lightweight, high precision",
  });

  const [provider, setProvider] = useState("gemini");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<SimulateResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  /* ------------------------- HANDLERS ------------------------- */

  const addHistoryItem = () =>
    setHistory([...history, { item: "", rating: 3, review: "" }]);

  const removeHistoryItem = (i: number) =>
    setHistory(history.filter((_, idx) => idx !== i));

  const updateHistory = (
    i: number,
    field: keyof ReviewHistoryItem,
    value: string | number
  ) => {
    const copy = [...history];
    copy[i] = { ...copy[i], [field]: value };
    setHistory(copy);
  };

  const handleSubmit = async () => {
    setLoading(true);
    setError(null);
    setResult(null);

    try {
      const req: SimulateRequest = {
        user_persona: persona,
        review_history: history.filter((h) => h.item && h.review),
        target_item: target,
        provider,
      };

      const res = await simulate(req);
      setResult(res);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  };

  /* --------------------------- UI LAYOUT --------------------------- */

  return (
    <main className="min-h-screen bg-[#FAFAF9] text-[#111]">

      <div className="max-w-6xl mx-auto px-8 pt-24 pb-16 grid lg:grid-cols-2 gap-12">

        {/* ---------------- LEFT: FORM ---------------- */}
        <div className="space-y-8">

          {/* HEADER */}
          <div>
            <h1 className="text-3xl font-semibold tracking-tight flex items-center gap-2">
              <Sparkles className="w-5 h-5 text-amber-500" />
              Simulate a Review
            </h1>
            <p className="text-sm text-[#666] mt-2 leading-relaxed">
              Define a user context and let the agent simulate how they would respond.
            </p>
          </div>

          {/* PERSONA */}
          <SectionCard icon={<User className="w-4 h-4" />} title="User Persona">
            <textarea
              value={persona}
              onChange={(e) => setPersona(e.target.value)}
              rows={3}
              className="w-full text-sm bg-white border border-[#E5E4DF] rounded-xl px-4 py-3 focus:outline-none focus:ring-2 focus:ring-amber-200"
            />
          </SectionCard>

          {/* HISTORY */}
          <SectionCard
            icon={
              <div className="flex items-center gap-2">
                <Bot className="w-4 h-4" />
                Review History
              </div>
            }
            action={
              <button
                onClick={addHistoryItem}
                className="text-xs flex items-center gap-1 text-amber-600 hover:text-amber-700"
              >
                <Plus className="w-3.5 h-3.5" />
                Add
              </button>
            }
          >
            <div className="space-y-4">
              {history.map((item, i) => (
                <div
                  key={i}
                  className="rounded-xl border border-[#EDECE7] bg-white p-4 space-y-3"
                >
                  <div className="flex gap-2 items-center">
                    <input
                      value={item.item}
                      onChange={(e) =>
                        updateHistory(i, "item", e.target.value)
                      }
                      placeholder="Item name"
                      className="flex-1 text-sm border border-[#E5E4DF] rounded-lg px-3 py-2"
                    />

                    <select
                      value={item.rating}
                      onChange={(e) =>
                        updateHistory(i, "rating", Number(e.target.value))
                      }
                      className="text-sm border border-[#E5E4DF] rounded-lg px-2 py-2"
                    >
                      {[1, 2, 3, 4, 5].map((r) => (
                        <option key={r}>{r}</option>
                      ))}
                    </select>

                    <button
                      onClick={() => removeHistoryItem(i)}
                      className="text-[#bbb] hover:text-red-400"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>

                  <textarea
                    value={item.review}
                    onChange={(e) =>
                      updateHistory(i, "review", e.target.value)
                    }
                    rows={2}
                    className="w-full text-sm border border-[#E5E4DF] rounded-lg px-3 py-2"
                  />
                </div>
              ))}
            </div>
          </SectionCard>

          {/* TARGET */}
          <SectionCard icon={<Package className="w-4 h-4" />} title="Target Item">
            <div className="space-y-3">
              <input
                value={target.name}
                onChange={(e) =>
                  setTarget({ ...target, name: e.target.value })
                }
                className="w-full text-sm border border-[#E5E4DF] rounded-xl px-4 py-2"
              />

              <div className="grid grid-cols-2 gap-3">
                <input
                  value={target.category}
                  onChange={(e) =>
                    setTarget({ ...target, category: e.target.value })
                  }
                  className="text-sm border border-[#E5E4DF] rounded-xl px-4 py-2"
                />

                <select
                  value={provider}
                  onChange={(e) => setProvider(e.target.value)}
                  className="text-sm border border-[#E5E4DF] rounded-xl px-3 py-2"
                >
                  {PROVIDERS.map((p) => (
                    <option key={p}>{p}</option>
                  ))}
                </select>
              </div>

              <input
                value={target.description}
                onChange={(e) =>
                  setTarget({ ...target, description: e.target.value })
                }
                className="w-full text-sm border border-[#E5E4DF] rounded-xl px-4 py-2"
              />
            </div>
          </SectionCard>

          {/* CTA */}
          <button
            onClick={handleSubmit}
            disabled={loading || !persona || !target.name}
            className="w-full bg-black text-white rounded-xl py-3 flex items-center justify-center gap-2 hover:opacity-90 disabled:opacity-40"
          >
            {loading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Simulating...
              </>
            ) : (
              "Simulate Review"
            )}
          </button>

          {error && (
            <p className="text-sm text-red-500 bg-red-50 p-3 rounded-xl">
              {error}
            </p>
          )}
        </div>

        {/* ---------------- RIGHT: OUTPUT ---------------- */}
        <div className="space-y-6">

          {!result && !loading && (
            <EmptyState />
          )}

          {loading && (
            <LoadingState />
          )}

          {result && (
            <div className="space-y-4 animate-in fade-in">

              <OutputCard title="Predicted Rating">
                <StarDisplay rating={result.rating} />
              </OutputCard>

              <OutputCard title="Simulated Review">
                <p className="text-sm leading-relaxed text-[#333]">
                  {result.review}
                </p>
              </OutputCard>

              <OutputCard title="Reasoning">
                <p className="text-xs text-[#666] leading-relaxed">
                  {result.reasoning}
                </p>
              </OutputCard>

            </div>
          )}
        </div>
      </div>
    </main>
  );
}

/* -------------------- COMPONENTS -------------------- */

function SectionCard({ icon, title, action, children }: any) {
  return (
    <div className="bg-white border border-[#E5E4DF] rounded-2xl p-5 space-y-4">
      <div className="flex items-center justify-between text-xs font-medium text-[#777]">
        <div className="flex items-center gap-2">
          {icon}
          {title}
        </div>
        {action}
      </div>
      {children}
    </div>
  );
}

function OutputCard({ title, children }: any) {
  return (
    <div className="bg-white border border-[#E5E4DF] rounded-2xl p-5">
      <p className="text-xs uppercase tracking-widest text-[#888] mb-3">
        {title}
      </p>
      {children}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="border border-dashed border-[#E5E4DF] rounded-2xl p-10 text-center text-[#999]">
      Fill the form to simulate a user review
    </div>
  );
}

function LoadingState() {
  return (
    <div className="border border-[#E5E4DF] rounded-2xl p-10 flex flex-col items-center gap-3">
      <Loader2 className="w-5 h-5 animate-spin text-amber-500" />
      <p className="text-sm text-[#888]">Simulating user behavior...</p>
    </div>
  );
}