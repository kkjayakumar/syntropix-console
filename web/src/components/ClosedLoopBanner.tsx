"use client";

import Link from "next/link";
import {
  PlusCircle,
  Wrench,
  GitBranch,
  GitPullRequest,
  Activity,
  Server,
  LineChart,
  ArrowRight,
} from "lucide-react";

const steps = [
  {
    title: "Create",
    desc: "Feature or fix issue",
    href: "/issues/new",
    icon: PlusCircle,
  },
  {
    title: "Develop",
    desc: "Chat / MCP start-fix",
    href: "/",
    icon: Wrench,
  },
  {
    title: "Commit",
    desc: "Branch aegis/fix-*",
    href: "/issues",
    icon: GitBranch,
  },
  {
    title: "Pull request",
    desc: "Auto-PR or Open PR",
    href: "/issues",
    icon: GitPullRequest,
  },
  {
    title: "CI/CD",
    desc: "Actions + monitor",
    href: "/cicd",
    icon: Activity,
  },
  {
    title: "Deploy",
    desc: "GHCR → EC2 / SSM",
    href: "/cicd",
    icon: Server,
  },
  {
    title: "Monitor",
    desc: "Spotting · Copilot · health",
    href: "/workspace",
    icon: LineChart,
  },
];

export function ClosedLoopBanner() {
  return (
    <div className="border-b border-[var(--panel-border)] bg-[var(--surface)] px-4 py-3 md:px-6">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--text-muted)]">
          Syntropix closed loop · demo path
        </p>
        <Link href="/settings" className="text-[11px] font-medium text-[var(--accent)] hover:underline">
          RBAC & encryption policy
        </Link>
      </div>
      <div className="flex gap-1 overflow-x-auto pb-1">
        {steps.map((s, i) => {
          const Icon = s.icon;
          return (
            <div key={s.title} className="flex items-center gap-1">
              <Link
                href={s.href}
                className="flex min-w-[108px] flex-col rounded-xl border border-[var(--panel-border)] bg-[var(--background)] px-2.5 py-2 transition hover:border-[var(--accent)]"
              >
                <span className="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-active)]">
                  <Icon size={13} className="text-[var(--accent)]" />
                  {s.title}
                </span>
                <span className="mt-0.5 text-[10px] text-[var(--text-muted)]">{s.desc}</span>
              </Link>
              {i < steps.length - 1 && <ArrowRight size={12} className="shrink-0 text-[var(--text-muted)]" />}
            </div>
          );
        })}
      </div>
    </div>
  );
}
