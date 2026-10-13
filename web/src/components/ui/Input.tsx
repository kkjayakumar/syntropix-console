import type { InputHTMLAttributes, TextareaHTMLAttributes } from "react";

export function Input({ className = "", ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={`w-full rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-4 py-3 text-sm text-[var(--text-active)] outline-none transition placeholder:text-[var(--text-muted)] focus:border-sky-400/50 focus:ring-2 focus:ring-sky-400/20 ${className}`}
      {...props}
    />
  );
}

export function TextArea({ className = "", ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={`w-full min-h-[140px] resize-y rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-4 py-3 text-sm text-[var(--text-active)] outline-none transition placeholder:text-[var(--text-muted)] focus:border-sky-400/50 focus:ring-2 focus:ring-sky-400/20 ${className}`}
      {...props}
    />
  );
}

export function FieldLabel({ children }: { children: React.ReactNode }) {
  return (
    <label className="mb-1.5 block text-[11px] font-bold uppercase tracking-widest text-[var(--text-muted)]">
      {children}
    </label>
  );
}
