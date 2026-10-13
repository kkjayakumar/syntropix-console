"use client";

import { FormEvent, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import { AtSign, Plus, ArrowUp, ChevronDown, Activity } from "lucide-react";
import { DogfoodPanel } from "@/components/DogfoodPanel";
import { BRAND } from "@/lib/brand";

const quickPrompts = [
  { label: "Open incidents", href: "/issues?status=detected" },
  { label: "CI / CD failures", href: "/cicd" },
  { label: "Start a fix", href: "/issues" },
  { label: "Workspace secrets", href: "/workspace" },
];

function greeting() {
  const h = new Date().getHours();
  if (h < 12) return "Good morning";
  if (h < 18) return "Good afternoon";
  return "Good evening";
}

export function ExecutiveDashboard() {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const hello = useMemo(() => greeting(), []);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const q = query.trim().toLowerCase();
    if (!q) return;
    if (q.includes("ci") || q.includes("deploy") || q.includes("github")) {
      router.push("/cicd");
      return;
    }
    if (q.includes("workspace") || q.includes("secret") || q.includes("connector")) {
      router.push("/workspace");
      return;
    }
    if (q.includes("feature") || q.includes("new")) {
      router.push("/issues/new");
      return;
    }
    if (q.includes("pipeline") || q.includes("architecture")) {
      router.push("/pipeline");
      return;
    }
    router.push("/issues");
  }

  return (
    <div className="relative flex min-h-full flex-col">
      <div className="mx-auto flex w-full max-w-3xl flex-1 flex-col justify-center px-6 py-16 md:px-10">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }}>
          <h1 className="text-center text-3xl font-semibold tracking-tight text-[var(--text-active)] md:text-4xl">
            {hello}.
          </h1>
          <p className="mt-2 text-center text-base text-[var(--text-muted)]">What should we look into?</p>
        </motion.div>

        <motion.form
          onSubmit={onSubmit}
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.08, duration: 0.4 }}
          className="cmd-bar mt-10 p-3"
        >
          <div className="flex items-center gap-2 px-1">
            <button type="button" className="rounded-lg p-1.5 text-[var(--text-muted)] hover:bg-[var(--glass-bg)]" title="New">
              <Plus size={16} />
            </button>
            <button type="button" className="rounded-lg p-1.5 text-[var(--text-muted)] hover:bg-[var(--glass-bg)]" title="Mention">
              <AtSign size={16} />
            </button>
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="What changed in CI recently? · tab to use"
              className="min-w-0 flex-1 bg-transparent py-2 text-sm text-[var(--text-active)] outline-none placeholder:text-[var(--text-muted)]"
            />
          </div>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2 border-t border-[var(--panel-border)] px-1 pt-2">
            <div className="flex items-center gap-2 text-[11px] text-[var(--text-muted)]">
              <Activity size={12} />
              <span>Observe read-only</span>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-lg border border-[var(--panel-border)] px-2.5 py-1 text-[11px] font-medium text-[var(--text-muted)]"
              >
                aegis-expert
                <ChevronDown size={12} />
              </button>
              <button
                type="submit"
                className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--accent)] text-white transition hover:opacity-90"
                aria-label="Submit"
              >
                <ArrowUp size={16} />
              </button>
            </div>
          </div>
        </motion.form>

        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.16 }}
          className="mt-5 flex flex-wrap justify-center gap-2"
        >
          {quickPrompts.map((p) => (
            <button key={p.href} type="button" className="pill" onClick={() => router.push(p.href)}>
              {p.label}
            </button>
          ))}
        </motion.div>

        <p className="mt-10 text-center text-xs text-[var(--text-muted)]">
          Open <span className="font-mono text-[var(--text-active)]">/workspace</span> to connect git, CI, telemetry, and secrets for{" "}
          <span className="font-medium text-[var(--text-active)]">{BRAND.name}</span>.
        </p>
      </div>

      <div className="mx-auto w-full max-w-3xl px-6 pb-10 md:px-10">
        <DogfoodPanel />
      </div>
    </div>
  );
}
