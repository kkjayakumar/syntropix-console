const styles: Record<string, string> = {
  detected: "bg-amber-500/15 text-amber-700 border-amber-400/30 dark:text-amber-200",
  correlated: "bg-sky-500/15 text-sky-700 border-sky-400/30 dark:text-sky-200",
  in_progress: "bg-indigo-500/15 text-indigo-700 border-indigo-400/30 dark:text-indigo-200",
  pr_created: "bg-violet-500/15 text-violet-700 border-violet-400/30 dark:text-violet-200",
  fixed: "bg-emerald-500/15 text-emerald-700 border-emerald-400/30 dark:text-emerald-200",
  closed: "bg-slate-500/15 text-slate-600 border-slate-400/30 dark:text-slate-300",
  success: "bg-emerald-500/15 text-emerald-700 border-emerald-400/30 dark:text-emerald-200",
  failure: "bg-rose-500/15 text-rose-700 border-rose-400/30 dark:text-rose-200",
  default: "bg-slate-500/10 text-[var(--text-muted)] border-[var(--panel-border)]",
};

export function Badge({ status, label }: { status: string; label?: string }) {
  const text = label ?? status.replace(/_/g, " ");
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wide ${styles[status] ?? styles.default}`}
    >
      {text}
    </span>
  );
}
