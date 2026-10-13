"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { FileText, Plus, Search, Eye, AlertCircle, GitBranch, ArrowRight } from "lucide-react";
import { fetchStats, listIssues, listDeployments } from "@/lib/api";
import type { AegisIssue, Deployment, StatsSnapshot } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";

type MemoryTab = "pages" | "stats";

export function ContextPanel() {
  const pathname = usePathname();
  const [tab, setTab] = useState<MemoryTab>("pages");
  const [stats, setStats] = useState<StatsSnapshot | null>(null);
  const [issues, setIssues] = useState<AegisIssue[]>([]);
  const [deploys, setDeploys] = useState<Deployment[]>([]);

  useEffect(() => {
    fetchStats().then(setStats).catch(() => setStats(null));
    listIssues()
      .then((r) => setIssues(r.issues.filter((i) => !["fixed", "closed"].includes(i.status)).slice(0, 6)))
      .catch(() => setIssues([]));
    listDeployments()
      .then((r) => setDeploys(r.deployments.slice(0, 5)))
      .catch(() => setDeploys([]));
  }, [pathname]);

  return (
    <aside className="hidden h-full w-[var(--rail-width)] shrink-0 flex-col border-l border-[var(--panel-border)] bg-[var(--sidebar-bg)] xl:flex">
      <section className="flex min-h-0 flex-1 flex-col border-b border-[var(--panel-border)]">
        <div className="flex items-center justify-between px-4 pt-4">
          <div className="flex items-center gap-2">
            <FileText size={14} className="text-[var(--text-muted)]" />
            <h2 className="text-sm font-semibold text-[var(--text-active)]">Memory</h2>
          </div>
          <Link href="/issues/new" className="rounded-lg p-1 text-[var(--text-muted)] hover:bg-[var(--glass-bg)] hover:text-[var(--text-active)]">
            <Plus size={14} />
          </Link>
        </div>

        <div className="mt-3 flex gap-1 px-4">
          {(
            [
              ["pages", "Open issues"],
              ["stats", "Health"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              onClick={() => setTab(id)}
              className={`rounded-lg px-2.5 py-1 text-xs font-medium transition ${
                tab === id ? "bg-[var(--accent-soft)] text-[var(--accent)]" : "text-[var(--text-muted)] hover:text-[var(--text-active)]"
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        <div className="mt-3 px-4">
          <div className="flex items-center gap-2 rounded-xl border border-[var(--panel-border)] bg-[var(--surface)] px-3 py-2">
            <Search size={13} className="text-[var(--text-muted)]" />
            <input
              readOnly
              placeholder="Search memory…"
              className="w-full bg-transparent text-xs text-[var(--text-active)] outline-none placeholder:text-[var(--text-muted)]"
              onFocus={(e) => e.target.blur()}
              onClick={() => {
                window.location.href = "/issues";
              }}
            />
          </div>
        </div>

        <div className="mt-3 flex-1 overflow-y-auto px-3 pb-4">
          {tab === "pages" && (
            <>
              {issues.length === 0 ? (
                <div className="mx-1 mt-6 rounded-xl border border-dashed border-[var(--panel-border)] px-4 py-8 text-center">
                  <p className="text-sm font-medium text-[var(--text-active)]">No open issues yet</p>
                  <p className="mt-1 text-xs text-[var(--text-muted)]">
                    Aegis writes incidents and fix notes here as telemetry and CI correlate.
                  </p>
                  <Link href="/issues/new" className="mt-4 inline-flex items-center gap-1 text-xs font-semibold text-[var(--accent)]">
                    <Plus size={12} /> New feature
                  </Link>
                </div>
              ) : (
                <ul className="space-y-1">
                  {issues.map((issue) => (
                    <li key={issue.id}>
                      <Link
                        href={`/issues`}
                        className="block rounded-xl px-3 py-2.5 transition hover:bg-[var(--glass-bg)]"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <p className="line-clamp-2 text-xs font-medium text-[var(--text-active)]">{issue.title}</p>
                          <Badge status={issue.status} />
                        </div>
                        <p className="mt-1 font-mono text-[10px] text-[var(--text-muted)]">#{issue.id}</p>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </>
          )}

          {tab === "stats" && (
            <div className="space-y-2 px-1 pt-2">
              {[
                { label: "Open incidents", value: stats?.openIncidents ?? "—" },
                { label: "Fix issues", value: stats?.issuesFixCount ?? "—" },
                { label: "Features", value: stats?.issuesFeatCount ?? "—" },
                { label: "CI failures", value: stats?.deployFailures ?? "—" },
              ].map((row) => (
                <div
                  key={row.label}
                  className="flex items-center justify-between rounded-xl border border-[var(--panel-border)] bg-[var(--surface)] px-3 py-2.5"
                >
                  <span className="text-xs text-[var(--text-muted)]">{row.label}</span>
                  <span className="text-sm font-semibold tabular-nums text-[var(--text-active)]">{row.value}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      <section className="flex max-h-[42%] flex-col">
        <div className="flex items-center gap-2 px-4 pt-3">
          <Eye size={14} className="text-[var(--text-muted)]" />
          <h2 className="text-sm font-semibold text-[var(--text-active)]">Watchers</h2>
        </div>
        <p className="px-4 pt-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--text-muted)]">Watching</p>
        <div className="flex-1 overflow-y-auto px-3 pb-4 pt-2">
          {deploys.length === 0 ? (
            <div className="mx-1 rounded-xl border border-dashed border-[var(--panel-border)] px-3 py-5 text-center">
              <p className="text-xs text-[var(--text-muted)]">
                No watchers running. Connect CI/CD in Workspace and Aegis will surface workflow failures here.
              </p>
              <Link href="/cicd" className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-[var(--accent)]">
                Open CI/CD <ArrowRight size={12} />
              </Link>
            </div>
          ) : (
            <ul className="space-y-1">
              {deploys.map((d) => (
                <li key={d.id}>
                  <Link href="/cicd" className="flex items-start gap-2 rounded-xl px-3 py-2.5 hover:bg-[var(--glass-bg)]">
                    {d.status === "failure" ? (
                      <AlertCircle size={14} className="mt-0.5 shrink-0 text-rose-400" />
                    ) : (
                      <GitBranch size={14} className="mt-0.5 shrink-0 text-[var(--text-muted)]" />
                    )}
                    <div className="min-w-0">
                      <p className="truncate text-xs font-medium text-[var(--text-active)]">{d.workflowName}</p>
                      <p className="truncate font-mono text-[10px] text-[var(--text-muted)]">
                        {d.branch} · {d.status}
                      </p>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>
    </aside>
  );
}
