/* eslint-disable react-hooks/set-state-in-effect */
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import {
  Brain,
  Database,
  Cpu,
  Sparkles,
  ArrowRight,
  CheckCircle2,
} from "lucide-react";

let visited = false;

const ease = [0.16, 1, 0.3, 1] as const;

function Reveal({
  children,
  delay = 0,
}: {
  children: React.ReactNode;
  delay?: number;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 18 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-120px" }}
      transition={{ duration: 0.75, ease, delay }}
    >
      {children}
    </motion.div>
  );
}

const featuresA = [
  "Behavioral profiling from review history",
  "Fit scoring against target item",
  "Voice mimicry with few-shot examples",
];

const featuresB = [
  "Multi-turn dialogue support",
  "Cross-domain retrieval",
  "Cold-start & contextual handling",
];

const stack = [
  { icon: Cpu, label: "API Server", value: "Go + stdlib HTTP" },
  { icon: Database, label: "Vector DB", value: "PostgreSQL + pgvector" },
  { icon: Brain, label: "Embeddings", value: "MiniLM-L6 / ONNX" },
  { icon: Sparkles, label: "LLMs", value: "Gemini · Kimi · OpenAI" },
];

export default function Home() {
  const [firstLoad, setFirstLoad] = useState(true);

  useEffect(() => {
    if (visited) setFirstLoad(false);
    visited = true;
  }, []);

  return (
    <main className="relative min-h-screen max-md:text-center bg-[#fafaf9] text-[#111] selection:bg-amber-100 selection:text-amber-900 overflow-hidden">

      {/* SUBTLE DEPTH FIELD (VERY LIGHT — LEVEL 5 SIGNATURE) */}
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -top-56 left-1/2 h-[600px] w-[600px] -translate-x-1/2 rounded-full bg-amber-100/20 blur-[120px]" />
        <div className="absolute bottom-[-260px] right-[-160px] h-[600px] w-[600px] rounded-full bg-emerald-100/15 blur-[140px]" />
      </div>

      {/* HERO */}
      <section className="relative mx-auto max-w-5xl px-8 pt-36 pb-32">

        {/* BADGE */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease }}
          className="inline-flex items-center gap-2 rounded-full bg-white/60 px-4 py-1 text-xs text-[#444] shadow-sm backdrop-blur"
        >
          <Sparkles className="h-3.5 w-3.5 text-amber-500" />
          LLM Agent Challenge · May – June 2026
        </motion.div>

        {/* HERO TYPOGRAPHY (SYSTEMIZED + TIGHT + PRECISE) */}
        <div className="mt-10 space-y-2 sm:space-y-4">
          <h1 className="text-[44px] md:text-6xl font-semibold tracking-tight leading-[1.02]">
            AI that understands
          </h1>

          <h1 className="text-[44px] md:text-6xl font-semibold tracking-tight leading-[1.02] text-amber-600">
            how people behave.
          </h1>
        </div>

        {/* DESCRIPTION */}
        <Reveal delay={0.1}>
          <p className="mt-9 max-w-xl text-[15px] leading-relaxed text-[#5a5a5a]">
            Ningen models user behaviour from review data — simulating intent, tone,
            and preference patterns to predict what people truly want.
          </p>
        </Reveal>

        {/* CTA SYSTEM (CLEAR PRIMARY / SECONDARY HIERARCHY) */}
        <Reveal delay={0.15}>
          <div className="mt-11 flex items-center gap-5">

            {/* Primary */}
            <Link
              href="/simulate"
              className="group inline-flex items-center gap-2 rounded-xl bg-black px-6 py-3 text-sm text-white transition hover:bg-black/85"
            >
              Try User Modeling
              <ArrowRight className="h-4 w-4 transition group-hover:translate-x-0.5" />
            </Link>

            {/* Secondary */}
            <Link
              href="/recommend"
              className="text-sm text-[#555] transition hover:text-black"
            >
              Try Recommendations →
            </Link>
          </div>
        </Reveal>
      </section>

      {/* FEATURE SYSTEM (MORE “PRODUCT SURFACE”, LESS BOXES) */}
      <section className="mx-auto grid max-w-5xl grid-cols-1 md:grid-cols-2 gap-12 px-8 pb-32">

        {/* CARD A */}
        <Reveal>
          <div className="rounded-2xl bg-white/60 p-8 backdrop-blur shadow-sm transition hover:-translate-y-1">
            <div className="flex items-center gap-2 text-amber-600 text-xs font-medium">
              <CheckCircle2 className="h-4 w-4" />
              Task A
            </div>

            <h3 className="mt-4 text-xl font-semibold tracking-tight">
              User Modeling
            </h3>

            <p className="mt-3 text-sm text-[#666] leading-relaxed">
              Predicts how a user would rate and review unseen items using behavioral history.
            </p>

            <ul className="mt-7 space-y-2 text-sm text-[#555]">
              {featuresA.map((f) => (
                <li key={f} className="flex gap-2">
                  <span className="text-amber-500">•</span>
                  {f}
                </li>
              ))}
            </ul>
          </div>
        </Reveal>

        {/* CARD B */}
        <Reveal delay={0.05}>
          <div className="rounded-2xl bg-white/60 p-8 backdrop-blur shadow-sm transition hover:-translate-y-1">
            <div className="flex items-center gap-2 text-emerald-600 text-xs font-medium">
              <CheckCircle2 className="h-4 w-4" />
              Task B
            </div>

            <h3 className="mt-4 text-xl font-semibold tracking-tight">
              Recommendations
            </h3>

            <p className="mt-3 text-sm text-[#666] leading-relaxed">
              Context-aware recommendations using semantic retrieval + reasoning.
            </p>

            <ul className="mt-7 space-y-2 text-sm text-[#555]">
              {featuresB.map((f) => (
                <li key={f} className="flex gap-2">
                  <span className="text-emerald-500">•</span>
                  {f}
                </li>
              ))}
            </ul>
          </div>
        </Reveal>
      </section>

      {/* ARCHITECTURE (REFINED SYSTEM PRESENTATION) */}
      <section className="mx-auto max-w-5xl px-8 pb-32">
        <Reveal>
          <div className="rounded-2xl bg-white/70 p-8 backdrop-blur shadow-sm">

            <p className="text-[10px] uppercase tracking-[0.2em] text-[#888]">
              Architecture
            </p>

            <div className="mt-7 grid grid-cols-2 md:grid-cols-4 gap-7">
              {stack.map((t) => {
                const Icon = t.icon;

                return (
                  <div key={t.label} className="space-y-3">
                    <Icon className="h-4 w-4 text-[#666]" />

                    <div>
                      <p className="text-xs text-[#777]">{t.label}</p>
                      <p className="text-sm font-medium text-[#111]">
                        {t.value}
                      </p>
                    </div>
                  </div>
                );
              })}
            </div>

            <p className="mt-10 text-xs text-[#999]">
              Yelp · Amazon Reviews · Goodreads · 100k indexed items indexed via HNSW
            </p>
          </div>
        </Reveal>
      </section>

      {/* FOOTER (MINIMAL + QUIET) */}
      <footer className="border-t border-[#eee] px-8 py-6 text-xs text-[#888] flex justify-between">
        <span>ningen · LLM Agent Challenge</span>
        <span>Deadline: 24 May 2026</span>
      </footer>
    </main>
  );
}