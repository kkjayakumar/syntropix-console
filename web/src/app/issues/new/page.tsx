"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Sparkles, Shield, Zap, GitBranch } from "lucide-react";
import { motion } from "framer-motion";
import { createIssue } from "@/lib/api";
import { BRAND } from "@/lib/brand";
import { GlassCard, Shell } from "@/components/Shell";
import { Button } from "@/components/ui/Button";
import { FieldLabel, Input, TextArea } from "@/components/ui/Input";

const highlights = [
  { icon: Zap, text: "AI-assisted implementation via MCP" },
  { icon: GitBranch, text: "Auto branch and PR workflow" },
  { icon: Shield, text: "Gateway approval before deploy" },
];

export default function NewFeaturePage() {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      await createIssue({ title, description, issueType: "feat" });
      router.push("/issues");
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="grid min-h-full lg:grid-cols-2">
        {/* Hero panel */}
        <div className="relative hidden overflow-hidden border-r border-[var(--panel-border)] lg:flex lg:flex-col lg:justify-between">
          <div className="absolute inset-0 bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-900" />
          <div className="mesh-orb mesh-orb-1 !opacity-60" style={{ position: "absolute" }} />
          <div className="mesh-orb mesh-orb-2 !opacity-50" style={{ position: "absolute" }} />

          <div className="relative z-10 p-10">
            <div className="flex items-center gap-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-sky-500 to-violet-500">
                <Shield size={18} className="text-white" />
              </div>
              <span className="font-bold text-white">{BRAND.name}</span>
            </div>

            <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.5 }} className="mt-16">
              <span className="rounded-full bg-sky-500/20 px-3 py-1 text-[10px] font-bold uppercase tracking-widest text-sky-300 ring-1 ring-sky-400/30">
                Feature requests
              </span>
              <h2 className="mt-4 text-3xl font-bold leading-tight text-white">
                Ship features, <span className="text-gradient">built to flow.</span>
              </h2>
              <p className="mt-3 max-w-md text-sm text-slate-400">
                Describe what you need. {BRAND.name} routes it through correlation, IDE agents, and verified deploys.
              </p>
            </motion.div>

            <ul className="relative z-10 mt-12 space-y-4">
              {highlights.map(({ icon: Icon, text }, i) => (
                <motion.li
                  key={text}
                  initial={{ opacity: 0, x: -12 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: 0.2 + i * 0.1 }}
                  className="flex items-center gap-3 text-sm text-slate-300"
                >
                  <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-white/10 ring-1 ring-white/10">
                    <Icon size={16} className="text-sky-400" />
                  </div>
                  {text}
                </motion.li>
              ))}
            </ul>
          </div>

          <p className="relative z-10 p-10 text-xs text-slate-500">
            Secured orchestration · {BRAND.domain}
          </p>
        </div>

        {/* Form panel */}
        <div className="px-6 py-10 md:px-12">
          <Shell
            icon={Sparkles}
            eyebrow="Create"
            title="New feature request"
            subtitle="Submit a feat issue for AI-assisted implementation"
            noPadding
          >
            <GlassCard className="max-w-xl">
              <form onSubmit={submit} className="space-y-5">
                <div>
                  <FieldLabel>Title</FieldLabel>
                  <Input
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="User-facing feature summary"
                    required
                  />
                </div>
                <div>
                  <FieldLabel>Description</FieldLabel>
                  <TextArea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="Acceptance criteria, context, links…"
                    required
                  />
                </div>
                {error && (
                  <p className="rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 py-2 text-sm text-rose-400">{error}</p>
                )}
                <Button type="submit" variant="primary" disabled={loading} className="w-full sm:w-auto">
                  {loading ? "Creating…" : "Create feature issue"}
                </Button>
              </form>
            </GlassCard>
          </Shell>
        </div>
      </div>
    </div>
  );
}
