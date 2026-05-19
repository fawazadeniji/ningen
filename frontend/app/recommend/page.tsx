/* eslint-disable @typescript-eslint/no-explicit-any */
"use client";

import { useState, useRef, useEffect } from "react";
import {
  recommend,
  type Message,
  type RecommendResponse,
  type RecommendItem,
} from "@/lib/api";

import {
  Send,
  Sparkles,
  Trash2,
  Settings,
  MessageSquare,
} from "lucide-react";

/* ---------------- CONFIG ---------------- */

const PROVIDERS = ["gemini", "openai", "kimi"];

const DOMAIN_COLORS: Record<string, string> = {
  goodreads: "bg-violet-50 text-violet-700 border-violet-100",
  amazon: "bg-blue-50 text-blue-700 border-blue-100",
  yelp: "bg-orange-50 text-orange-700 border-orange-100",
};

/* ---------------- ITEM CARD ---------------- */

function ItemCard({ item }: { item: RecommendItem }) {
  const preview = item.search_text.slice(0, 140);

  const color =
    DOMAIN_COLORS[item.domain] ??
    "bg-[#F6F5F3] text-[#555] border-[#E5E4DF]";

  return (
    <div className="rounded-xl border border-[#EAE9E4] bg-white p-4 hover:shadow-sm transition">
      <div className="flex items-center justify-between mb-2">
        <span className={`text-[10px] uppercase tracking-wider px-2 py-0.5 rounded-full border ${color}`}>
          {item.domain}
        </span>
        <span className="text-[10px] text-[#999]">
          {(1 - item.score).toFixed(2)} match
        </span>
      </div>

      <p className="text-xs text-[#666] leading-relaxed">
        {preview}...
      </p>
    </div>
  );
}

/* ---------------- TYPES ---------------- */

interface ChatBubble {
  role: "user" | "assistant";
  content: string;
  response?: RecommendResponse;
}

/* ---------------- PAGE ---------------- */

export default function RecommendPage() {
  const [persona, setPersona] = useState(
    "A Lagos-based tech worker who loves sci-fi and jollof rice."
  );

  const [crossDomain, setCrossDomain] = useState(false);
  const [limit, setLimit] = useState(5);
  const [provider, setProvider] = useState("gemini");

  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [bubbles, setBubbles] = useState<ChatBubble[]>([]);

  const [loading, setLoading] = useState(false);
  const [stage, setStage] = useState<"idle" | "thinking" | "responding">("idle");

  const [error, setError] = useState<string | null>(null);

  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [bubbles, stage]);

  /* ---------------- SEND ---------------- */

  const send = async () => {
    if (!input.trim() || loading) return;

    const userMsg: Message = { role: "user", content: input.trim() };
    const nextMessages = [...messages, userMsg];

    setMessages(nextMessages);
    setBubbles((b) => [...b, { role: "user", content: input.trim() }]);

    setInput("");
    setLoading(true);
    setError(null);
    setStage("thinking");

    try {
      // UX delay for realism
      await new Promise((r) => setTimeout(r, 500));

      setStage("responding");

      const res = await recommend({
        user_persona: persona,
        history: nextMessages,
        cross_domain: crossDomain,
        limit,
        provider,
      });

      const content =
        res.requires_input
          ? res.question ?? "Can you clarify?"
          : res.reasoning ?? "Here are recommendations.";

      const assistantBubble: ChatBubble = {
        role: "assistant",
        content,
        response: res,
      };

      setMessages((m) => [...m, { role: "assistant", content }]);
      setBubbles((b) => [...b, assistantBubble]);

      setStage("idle");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
      setStage("idle");
    } finally {
      setLoading(false);
    }
  };

  const reset = () => {
    setMessages([]);
    setBubbles([]);
    setError(null);
    setStage("idle");
  };

  const handleKey = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  };

  /* ---------------- UI ---------------- */

  return (
    <main className="min-h-screen bg-[#FAFAF9] text-[#111]">

      <div className="max-w-6xl mx-auto px-8 pt-24 pb-16 grid lg:grid-cols-[320px_1fr] gap-8">

        {/* ---------------- SIDEBAR ---------------- */}
        <aside className="space-y-5">

          <div>
            <h1 className="text-2xl font-semibold tracking-tight flex items-center gap-2">
              <Sparkles className="w-5 h-5 text-emerald-500" />
              Recommend
            </h1>
            <p className="text-xs text-[#777] mt-1">
              Context-aware recommendation engine
            </p>
          </div>

          {/* PERSONA */}
          <Panel title="Persona" icon={<MessageSquare className="w-4 h-4" />}>
            <textarea
              value={persona}
              onChange={(e) => setPersona(e.target.value)}
              rows={4}
              className="w-full text-xs bg-white border border-[#EAE9E4] rounded-xl px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-100"
            />
          </Panel>

          {/* SETTINGS */}
          <Panel title="Settings" icon={<Settings className="w-4 h-4" />}>

            <Row label="Cross-domain">
              <button
                onClick={() => setCrossDomain(!crossDomain)}
                className={`w-10 h-5 rounded-full flex transition relative ${crossDomain ? "bg-emerald-400" : "bg-[#DDD]"
                  }`}
              >
                <span
                  className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition ${crossDomain ? "translate-x-5" : "translate-x-0.5 mr-auto"
                    }`}
                />
              </button>
            </Row>

            <Row label="Results">
              <input
                type="number"
                value={limit}
                onChange={(e) => setLimit(Number(e.target.value))}
                className="w-14 text-xs text-center border border-[#EAE9E4] rounded-md py-1"
              />
            </Row>

            <Row label="Provider">
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                className="text-xs border border-[#EAE9E4] rounded-md px-2 py-1"
              >
                {PROVIDERS.map((p) => (
                  <option key={p}>{p}</option>
                ))}
              </select>
            </Row>
          </Panel>

          {bubbles.length > 0 && (
            <button
              onClick={reset}
              className="w-full text-xs text-red-400 border border-red-100 rounded-xl py-2 hover:bg-red-50 transition"
            >
              <Trash2 className="w-3 h-3 inline mr-1" />
              Clear chat
            </button>
          )}
        </aside>

        {/* ---------------- CHAT ---------------- */}
        <section className="flex flex-col min-h-[70vh]">

          {/* MESSAGES */}
          <div className="flex-1 space-y-5 overflow-y-auto pr-2">

            {bubbles.length === 0 && (
              <div className="h-full flex flex-col items-center justify-center text-center text-[#999]">
                <div className="w-12 h-12 rounded-full bg-emerald-50 flex items-center justify-center mb-3">
                  💬
                </div>
                Ask for recommendations to begin
              </div>
            )}

            {bubbles.map((b: any, i: number) => (
              <div key={i} className="space-y-3">

                {/* MESSAGE */}
                <div className={`flex ${b.role === "user" ? "justify-end" : "justify-start"}`}>
                  <div
                    className={`max-w-md text-sm px-4 py-3 rounded-2xl leading-relaxed ${b.role === "user"
                      ? "bg-black text-white rounded-br-sm"
                      : "bg-white border border-[#EAE9E4] text-[#333] rounded-bl-sm"
                      }`}
                  >
                    {b.content}
                  </div>
                </div>

                {/* RESULTS LAYER */}
                {b?.role === "assistant" && b?.response?.recommendations?.length > 0 && (
                  <div className="mt-2 space-y-2 animate-in fade-in duration-500">

                    <p className="text-[10px] uppercase tracking-widest text-[#999]">
                      AI Recommendations
                    </p>

                    <div className="grid gap-2">
                      {b?.response.recommendations.map((item: any) => (
                        <ItemCard key={item.item_id} item={item} />
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ))}

            {/* THINKING STATE */}
            {stage === "thinking" && (
              <div className="flex items-center gap-2 text-sm text-[#999]">
                <div className="w-2 h-2 bg-emerald-400 rounded-full animate-bounce" />
                Thinking about user preferences...
              </div>
            )}

            {error && (
              <div className="text-xs text-red-500 bg-red-50 p-3 rounded-xl">
                {error}
              </div>
            )}

            <div ref={bottomRef} />
          </div>

          {/* INPUT */}
          <div className="mt-4 flex gap-2 border border-[#EAE9E4] rounded-xl p-2 bg-white">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKey}
              rows={1}
              className="flex-1 text-sm px-2 py-1 resize-none focus:outline-none"
              placeholder="Ask for recommendations..."
            />

            <button
              onClick={send}
              disabled={!input.trim() || loading}
              className="bg-black text-white px-4 py-2 rounded-lg text-sm disabled:opacity-40 flex items-center gap-1"
            >
              <Send className="w-4 h-4" />
              Send
            </button>
          </div>
        </section>
      </div>
    </main>
  );
}

/* ---------------- SMALL COMPONENTS ---------------- */

function Panel({ title, icon, children }: any) {
  return (
    <div className="bg-white border border-[#EAE9E4] rounded-xl p-4 space-y-3">
      <div className="flex items-center gap-2 text-xs text-[#777] font-medium">
        {icon}
        {title}
      </div>
      {children}
    </div>
  );
}

function Row({ label, children }: any) {
  return (
    <div className="flex items-center justify-between text-xs text-[#555]">
      <span>{label}</span>
      {children}
    </div>
  );
}