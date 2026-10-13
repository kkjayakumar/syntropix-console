"use client";

import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "success" | "violet";

const variants: Record<Variant, string> = {
  primary:
    "bg-[var(--accent)] text-white hover:opacity-90 active:scale-[0.98] border-0",
  secondary:
    "bg-[var(--surface-raised)] text-[var(--text-active)] hover:bg-[var(--glass-bg)] border border-[var(--panel-border)] active:scale-[0.99]",
  ghost:
    "text-[var(--text-muted)] hover:text-[var(--text-active)] hover:bg-[var(--glass-bg)] border-0",
  danger: "bg-rose-500/15 text-rose-300 border border-rose-400/30 hover:bg-rose-500/25",
  success: "bg-emerald-500 text-white hover:bg-emerald-400 border-0",
  violet: "bg-[var(--accent-soft)] text-[var(--accent)] border border-[var(--panel-border)] hover:opacity-90",
};

export function Button({
  children,
  variant = "secondary",
  className = "",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; children: ReactNode }) {
  return (
    <button
      type={type}
      className={`inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold transition-all disabled:cursor-not-allowed disabled:opacity-50 ${variants[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}
