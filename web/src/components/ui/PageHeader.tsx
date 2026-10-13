"use client";

import { motion } from "framer-motion";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

export function PageHeader({
  icon: Icon,
  eyebrow,
  title,
  subtitle,
  actions,
}: {
  icon?: LucideIcon;
  eyebrow?: string;
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <motion.header
      initial={{ opacity: 0, y: -6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
      className="mb-6 flex flex-wrap items-start justify-between gap-4"
    >
      <div className="flex items-start gap-3">
        {Icon && (
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-[var(--accent-soft)] text-[var(--accent)]">
            <Icon className="h-4 w-4" strokeWidth={2} />
          </div>
        )}
        <div>
          {eyebrow && (
            <p className="text-[11px] font-medium tracking-wide text-[var(--text-muted)]">{eyebrow}</p>
          )}
          <h1 className="text-xl font-semibold tracking-tight text-[var(--text-active)] md:text-2xl">{title}</h1>
          {subtitle && <p className="mt-1 max-w-2xl text-sm text-[var(--text-muted)]">{subtitle}</p>}
        </div>
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </motion.header>
  );
}
