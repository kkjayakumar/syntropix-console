"use client";

import { Activity, GitBranch, ShieldCheck, Zap } from "lucide-react";
import { motion } from "framer-motion";

const nodes = [
  { id: "detect", label: "Detect", icon: Activity, color: "#0ea5e9" },
  { id: "correlate", label: "Correlate", icon: Zap, color: "#8b5cf6" },
  { id: "fix", label: "Fix", icon: GitBranch, color: "#f472b6" },
  { id: "verify", label: "Verify", icon: ShieldCheck, color: "#10b981" },
];

export function FlowHero() {
  return (
    <div className="relative overflow-hidden rounded-2xl glass-strong p-6 md:p-8">
      <div className="absolute -right-8 -top-8 h-40 w-40 rounded-full bg-violet-500/10 blur-3xl" />
      <div className="absolute -bottom-6 -left-6 h-32 w-32 rounded-full bg-sky-500/10 blur-3xl" />

      <p className="relative text-[11px] font-bold uppercase tracking-[0.25em] text-sky-500">Closed-loop orchestration</p>
      <h2 className="relative mt-2 text-xl font-bold text-[var(--text-active)] md:text-2xl">
        Incidents, <span className="text-gradient">built to resolve.</span>
      </h2>
      <p className="relative mt-2 max-w-lg text-sm text-[var(--text-muted)]">
        Telemetry flows from Spotting and Copilot into correlated issues, IDE fixes, and sentinel-verified deploys.
      </p>

      <div className="relative mt-8 flex flex-wrap items-center justify-center gap-2 md:gap-0">
        <svg className="absolute left-[12%] right-[12%] top-1/2 hidden h-2 -translate-y-1/2 md:block" viewBox="0 0 400 8" preserveAspectRatio="none">
          <line x1="0" y1="4" x2="400" y2="4" className="flow-line" stroke="url(#flowGrad)" strokeWidth="2" />
          <defs>
            <linearGradient id="flowGrad" x1="0%" y1="0%" x2="100%" y2="0%">
              <stop offset="0%" stopColor="#0ea5e9" />
              <stop offset="50%" stopColor="#8b5cf6" />
              <stop offset="100%" stopColor="#10b981" />
            </linearGradient>
          </defs>
        </svg>

        {nodes.map((node, i) => {
          const Icon = node.icon;
          return (
            <motion.div
              key={node.id}
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: i * 0.1, duration: 0.4 }}
              className="relative z-10 flex flex-col items-center px-3 md:px-6"
            >
              <div
                className="flow-node-glow flex h-14 w-14 items-center justify-center rounded-2xl ring-1 ring-white/20"
                style={{ background: `linear-gradient(135deg, ${node.color}22, ${node.color}08)` }}
              >
                <Icon size={24} style={{ color: node.color }} />
              </div>
              <span className="mt-2 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                {node.label}
              </span>
            </motion.div>
          );
        })}
      </div>
    </div>
  );
}
