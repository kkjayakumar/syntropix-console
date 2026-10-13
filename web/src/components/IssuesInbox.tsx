"use client";

import { useCallback, useEffect, useState } from "react";
import { Inbox, RefreshCw } from "lucide-react";
import { gatewayApprove, gatewayReject, getIssue, getIssueBundle, listIssues, openPR, startFeature, startFix } from "@/lib/api";
import type { AegisIssue, Deployment, TelemetryLink } from "@/lib/types";
import { GlassCard, Shell } from "@/components/Shell";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";

export function IssuesInbox() {
  const [tab, setTab] = useState<"fix" | "feat">("fix");
  const [issues, setIssues] = useState<AegisIssue[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [links, setLinks] = useState<TelemetryLink[]>([]);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [fixBranch, setFixBranch] = useState("");
  const [bundle, setBundle] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionMsg, setActionMsg] = useState("");
  const [starting, setStarting] = useState(false);
  const [openingPR, setOpeningPR] = useState(false);
  const [gatewayBusy, setGatewayBusy] = useState(false);

  const canGateway = (status: string) => ["detected", "correlated"].includes(status);

  const loadIssues = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await listIssues(tab);
      setIssues(data.issues ?? []);
      if (data.issues?.length) {
        setSelectedId((prev) => prev ?? data.issues[0].id);
      } else {
        setSelectedId(null);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load issues");
    } finally {
      setLoading(false);
    }
  }, [tab]);

  const loadDetail = useCallback(async (id: number) => {
    try {
      const [detail, bundleText] = await Promise.all([getIssue(id), getIssueBundle(id)]);
      setLinks(detail.telemetryLinks ?? []);
      setDeployments(detail.recentDeployments ?? []);
      setFixBranch(detail.fixBranch ?? "");
      setBundle(bundleText);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load issue detail");
    }
  }, []);

  useEffect(() => {
    loadIssues();
  }, [loadIssues]);

  useEffect(() => {
    if (selectedId != null) loadDetail(selectedId);
  }, [selectedId, loadDetail]);

  async function handleStart() {
    if (selectedId == null) return;
    setStarting(true);
    setActionMsg("");
    try {
      const res = tab === "feat" ? await startFeature(selectedId) : await startFix(selectedId);
      setActionMsg(res.message);
      await loadIssues();
      await loadDetail(selectedId);
    } catch (e) {
      setActionMsg(e instanceof Error ? e.message : "Action failed");
    } finally {
      setStarting(false);
    }
  }

  async function handleGateway(action: "approve" | "reject") {
    if (selectedId == null) return;
    setGatewayBusy(true);
    setActionMsg("");
    try {
      const res = action === "approve" ? await gatewayApprove(selectedId) : await gatewayReject(selectedId);
      setActionMsg(res.message);
      await loadIssues();
      await loadDetail(selectedId);
    } catch (e) {
      setActionMsg(e instanceof Error ? e.message : "Gateway action failed");
    } finally {
      setGatewayBusy(false);
    }
  }

  async function handleOpenPR() {
    if (selectedId == null) return;
    setOpeningPR(true);
    setActionMsg("");
    try {
      const res = await openPR(selectedId);
      setActionMsg(res.message);
      await loadIssues();
      await loadDetail(selectedId);
    } catch (e) {
      setActionMsg(e instanceof Error ? e.message : "Open PR failed");
    } finally {
      setOpeningPR(false);
    }
  }

  const selected = issues.find((i) => i.id === selectedId);

  return (
    <Shell
      icon={Inbox}
      eyebrow="Issue Broker"
      title="Issues Inbox"
      subtitle="Correlated incidents and feature requests"
      actions={
        <Button variant="ghost" onClick={() => void loadIssues()} className="!px-3">
          <RefreshCw size={16} />
          Refresh
        </Button>
      }
    >
      <div className="mb-6 inline-flex rounded-xl bg-[var(--glass-bg)] p-1 ring-1 ring-[var(--panel-border)]">
        {(["fix", "feat"] as const).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setTab(t)}
            className={`rounded-lg px-5 py-2 text-sm font-semibold capitalize transition ${
              tab === t
                ? "bg-gradient-to-r from-sky-500/20 to-violet-500/20 text-sky-600 shadow-sm dark:text-sky-300"
                : "text-[var(--text-muted)] hover:text-[var(--text-active)]"
            }`}
          >
            {t === "fix" ? "Fix" : "Feature"}
          </button>
        ))}
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(300px,1fr)_2fr]">
        <GlassCard label="Inbox" noPadding className="p-4">
          {loading && <p className="px-2 text-sm text-[var(--text-muted)]">Loading…</p>}
          {error && <p className="px-2 text-sm text-rose-400">{error}</p>}
          {!loading && issues.length === 0 && (
            <p className="px-2 text-sm text-[var(--text-muted)]">No {tab} issues yet.</p>
          )}
          <ul className="space-y-2">
            {issues.map((issue) => (
              <li key={issue.id}>
                <button
                  type="button"
                  onClick={() => setSelectedId(issue.id)}
                  className={`w-full rounded-xl border px-4 py-3 text-left transition ${
                    selectedId === issue.id
                      ? "border-sky-400/40 bg-gradient-to-r from-sky-500/10 to-violet-500/5 ring-1 ring-sky-400/20"
                      : "border-[var(--panel-border)] bg-[var(--glass-bg)] hover:border-sky-400/20"
                  }`}
                >
                  <p className="truncate text-sm font-semibold text-[var(--text-active)]">{issue.title}</p>
                  <div className="mt-2">
                    <Badge status={issue.status} />
                  </div>
                </button>
              </li>
            ))}
          </ul>
        </GlassCard>

        <GlassCard label="Telemetry inspector">
          {!selected ? (
            <p className="text-sm text-[var(--text-muted)]">Select an issue to inspect.</p>
          ) : (
            <>
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <p className="text-xs font-mono text-[var(--text-muted)]">#{selected.id}</p>
                  <h2 className="text-xl font-bold text-[var(--text-active)]">{selected.title}</h2>
                  <p className="mt-1 font-mono text-xs text-[var(--text-muted)]">{selected.correlationId}</p>
                  <div className="mt-2">
                    <Badge status={selected.status} />
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  {tab === "fix" && canGateway(selected.status) && (
                    <>
                      <Button variant="success" disabled={gatewayBusy} onClick={() => void handleGateway("approve")}>
                        {gatewayBusy ? "Working…" : "Approve Fix"}
                      </Button>
                      <Button variant="danger" disabled={gatewayBusy} onClick={() => void handleGateway("reject")}>
                        Reject
                      </Button>
                    </>
                  )}
                  {selected.status === "in_progress" && tab === "fix" && (
                    <span className="rounded-xl border border-emerald-400/30 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-700 dark:text-emerald-200">
                      Push <code className="font-mono text-emerald-600 dark:text-emerald-100">{fixBranch || `aegis/fix-${selected.id}`}</code>
                    </span>
                  )}
                  {["in_progress", "pr_created"].includes(selected.status) && tab === "fix" && (
                    <Button variant="violet" disabled={openingPR} onClick={() => void handleOpenPR()}>
                      {openingPR ? "Opening…" : "Open PR now"}
                    </Button>
                  )}
                  {!["fixed", "closed", "in_progress"].includes(selected.status) && (
                    <Button variant="primary" disabled={starting} onClick={() => void handleStart()}>
                      {starting ? "Starting…" : tab === "feat" ? "Start Feature in IDE" : "Start Fix"}
                    </Button>
                  )}
                </div>
              </div>
              {actionMsg && (
                <p className="mt-4 rounded-xl border border-sky-400/30 bg-sky-500/10 px-4 py-3 text-sm text-sky-800 dark:text-sky-100">
                  {actionMsg}
                </p>
              )}
              {links.length > 0 && (
                <ul className="mt-4 flex flex-wrap gap-2">
                  {links.map((l) => (
                    <li
                      key={l.id}
                      className="rounded-lg bg-[var(--glass-bg)] px-3 py-1.5 font-mono text-xs text-[var(--text-muted)] ring-1 ring-[var(--panel-border)]"
                    >
                      {l.sentryIssueId && <span>Sentry {l.sentryIssueId} </span>}
                      {l.spottingReportId && <span>Spotting {l.spottingReportId}</span>}
                    </li>
                  ))}
                </ul>
              )}
              {deployments.length > 0 && (
                <div className="mt-4 rounded-xl bg-[var(--glass-bg)] p-4 ring-1 ring-[var(--panel-border)]">
                  <p className="mb-3 text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">CI/CD</p>
                  <ul className="space-y-2 text-xs">
                    {deployments.map((d) => (
                      <li key={d.id} className="flex flex-wrap gap-x-2 gap-y-1 text-[var(--text-muted)]">
                        <span className="font-semibold text-sky-600 dark:text-sky-300">{d.workflowName}</span>
                        <Badge status={d.status === "success" ? "success" : d.status === "failure" ? "failure" : "default"} label={d.status} />
                        <span className="font-mono">{d.branch} · {d.commitSha.slice(0, 7)}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              <pre className="mt-4 max-h-[420px] overflow-auto rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap text-[var(--text-muted)]">
                {bundle || "No bundle."}
              </pre>
            </>
          )}
        </GlassCard>
      </div>
    </Shell>
  );
}
