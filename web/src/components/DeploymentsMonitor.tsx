"use client";

import { useCallback, useEffect, useState } from "react";
import { GitBranch, RefreshCw } from "lucide-react";
import { getDeploymentLogs, listDeployments } from "@/lib/api";
import type { Deployment } from "@/lib/types";
import { GlassCard, Shell } from "@/components/Shell";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";

export function DeploymentsMonitor() {
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [logs, setLogs] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await listDeployments();
      setDeployments(data.deployments ?? []);
      if (data.deployments?.length && selectedId === null) {
        setSelectedId(data.deployments[0].id);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load deployments");
    } finally {
      setLoading(false);
    }
  }, [selectedId]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (selectedId == null) return;
    getDeploymentLogs(selectedId)
      .then(setLogs)
      .catch(() => setLogs(""));
  }, [selectedId]);

  const selected = deployments.find((d) => d.id === selectedId);

  return (
    <Shell
      icon={GitBranch}
      eyebrow="CI/CD Monitor"
      title="Pipeline Runs"
      subtitle="GitHub Actions runs and deploy verification"
      actions={
        <Button variant="ghost" onClick={() => void load()} className="!px-3">
          <RefreshCw size={16} />
          Refresh
        </Button>
      }
    >
      <div className="grid gap-6 lg:grid-cols-[minmax(300px,1fr)_2fr]">
        <GlassCard label="Workflow runs" noPadding className="p-4">
          {loading && <p className="px-2 text-sm text-[var(--text-muted)]">Loading…</p>}
          {error && <p className="px-2 text-sm text-rose-400">{error}</p>}
          {!loading && deployments.length === 0 && (
            <p className="px-2 text-sm text-[var(--text-muted)]">
              No CI runs yet. Enable <strong className="text-[var(--text-active)]">workflow_run</strong> and{" "}
              <strong className="text-[var(--text-active)]">push</strong> on your GitHub webhook.
            </p>
          )}
          <ul className="space-y-2">
            {deployments.map((dep) => (
              <li key={dep.id}>
                <button
                  type="button"
                  onClick={() => setSelectedId(dep.id)}
                  className={`w-full rounded-xl border px-4 py-3 text-left transition ${
                    selectedId === dep.id
                      ? "border-sky-400/40 bg-gradient-to-r from-sky-500/10 to-violet-500/5 ring-1 ring-sky-400/20"
                      : "border-[var(--panel-border)] bg-[var(--glass-bg)] hover:border-sky-400/20"
                  }`}
                >
                  <p className="truncate text-sm font-semibold text-[var(--text-active)]">{dep.workflowName}</p>
                  <div className="mt-2 flex items-center gap-2">
                    <Badge
                      status={dep.status === "success" ? "success" : dep.status === "failure" ? "failure" : "default"}
                      label={dep.status}
                    />
                    <span className="text-xs text-[var(--text-muted)]">{dep.branch}</span>
                  </div>
                </button>
              </li>
            ))}
          </ul>
        </GlassCard>

        <GlassCard label="Failure logs">
          {!selected ? (
            <p className="text-sm text-[var(--text-muted)]">Select a workflow run.</p>
          ) : (
            <>
              <h2 className="text-lg font-bold text-[var(--text-active)]">{selected.workflowName}</h2>
              <p className="mt-1 text-sm text-[var(--text-muted)]">
                {selected.githubRepo} · run {selected.runId} · {selected.commitSha.slice(0, 7)}
              </p>
              <pre className="mt-4 max-h-[480px] overflow-auto rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap text-[var(--text-muted)]">
                {logs || "No failure logs captured."}
              </pre>
            </>
          )}
        </GlassCard>
      </div>
    </Shell>
  );
}
