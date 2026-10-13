"use client";

import { motion } from "framer-motion";
import type { ReactNode } from "react";
import { BRAND } from "@/lib/brand";
import { PageHeader } from "@/components/ui/PageHeader";
import type { LucideIcon } from "lucide-react";

export function Shell({
  children,
  title,
  subtitle,
  eyebrow = BRAND.name,
  icon,
  actions,
  noPadding,
}: {
  children: ReactNode;
  title: string;
  subtitle?: string;
  eyebrow?: string;
  icon?: LucideIcon;
  actions?: ReactNode;
  noPadding?: boolean;
}) {
  return (
    <div className={`relative min-h-full ${noPadding ? "" : "px-6 py-6 md:px-8 md:py-8"}`}>
      <PageHeader icon={icon} eyebrow={eyebrow} title={title} subtitle={subtitle} actions={actions} />
      <motion.div initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.04, duration: 0.3 }}>
        {children}
      </motion.div>
    </div>
  );
}

export function GlassCard({
  children,
  className = "",
  label,
  noPadding,
}: {
  children: ReactNode;
  className?: string;
  label?: string;
  noPadding?: boolean;
}) {
  return (
    <div className={`glass-panel ${noPadding ? "" : "p-5"} ${className}`}>
      {label && (
        <p className="mb-3 text-[11px] font-semibold uppercase tracking-[0.12em] text-[var(--text-muted)]">{label}</p>
      )}
      {children}
    </div>
  );
}

export function MetricTile({
  label,
  value,
  hint,
  accent = "sky",
}: {
  label: string;
  value: string | number;
  hint?: string;
  accent?: "sky" | "violet" | "emerald" | "amber";
}) {
  const accents = {
    sky: "text-sky-500",
    violet: "text-indigo-400",
    emerald: "text-emerald-500",
    amber: "text-amber-500",
  };

  return (
    <div className="glass-panel p-4">
      <p className="text-[11px] font-medium text-[var(--text-muted)]">{label}</p>
      <p className={`mt-1.5 text-2xl font-semibold tabular-nums ${accents[accent]}`}>{value}</p>
      {hint && <p className="mt-1 text-xs text-[var(--text-muted)]">{hint}</p>}
    </div>
  );
}
