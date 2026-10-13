"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Boxes,
  GitBranch,
  KeyRound,
  Layers,
  Plug,
  Server,
  Radio,
  Bot,
  Workflow,
  CheckCircle2,
  Circle,
  Trash2,
  Plus,
  Sparkles,
  Network,
} from "lucide-react";
import { GlassCard } from "@/components/Shell";
import { Button } from "@/components/ui/Button";
import { FieldLabel, Input, TextArea } from "@/components/ui/Input";
import { Badge } from "@/components/ui/Badge";
import {
  workspaceBootstrap,
  workspaceCreateProject,
  workspaceDeleteSecret,
  workspaceListConnectors,
  workspaceListSecrets,
  workspaceUpsertConnector,
  workspaceUpsertSecret,
  llmStatus,
  llmComplete,
  llmRuns,
  graphSnapshot,
  graphIngest,
  graphUpsertNode,
  bulkUpsertSecrets,
  getPipeline,
} from "@/lib/api";
import type { GraphNode, LLMRun, WorkspaceConnector, WorkspaceEnvironment, WorkspaceProject, WorkspaceSecret } from "@/lib/types";

type Tab = "overview" | "environments" | "connectors" | "secrets" | "llm" | "graph" | "pipeline";

const CONNECTOR_META: Record<
  string,
  { label: string; icon: typeof GitBranch; description: string; defaults: Record<string, unknown> }
> = {
  git: {
    label: "Git source",
    icon: GitBranch,
    description: "Repository, default branch, GitHub App",
    defaults: { repo: "owner/repo", defaultBranch: "main", provider: "github" },
  },
  pull_request: {
    label: "Pull requests",
    icon: GitBranch,
    description: "Auto-PR branch pattern aegis/fix-{id}",
    defaults: { branchPrefix: "aegis/fix-", autoOpen: true },
  },
  cicd: {
    label: "CI/CD",
    icon: Workflow,
    description: "GitHub Actions workflows and filters",
    defaults: { provider: "github_actions", workflows: ["test", "docker"] },
  },
  deployment: {
    label: "Deployment",
    icon: Server,
    description: "EC2/SSM, GHCR, K8s targets",
    defaults: { provider: "ec2_ssm", instanceId: "", region: "ap-south-1" },
  },
  telemetry: {
    label: "Telemetry",
    icon: Radio,
    description: "Copilot/Sentry ingest, Spotting API",
    defaults: { copilotUrl: "", spottingUrl: "", sentryProject: "" },
  },
  mcp: {
    label: "MCP / IDE",
    icon: Plug,
    description: "Cursor MCP server and tool routing",
    defaults: { serverName: "aegisops", transport: "stdio" },
  },
  issue_agent: {
    label: "Issue → AI agent",
    icon: Bot,
    description: "Prefer LLM API keys (OpenAI/Claude/Google/Cursor). MCP/IDE is optional.",
    defaults: { provider: "openai", route: "api", model: "gpt-4o-mini", fallbackMcp: true },
  },
};

const PROVIDER_KEYS: Record<string, { label: string; keys: string[] }> = {
  openai: { label: "OpenAI", keys: ["OPENAI_API_KEY", "OPENAI_ORG_ID"] },
  anthropic: { label: "Anthropic (Claude)", keys: ["ANTHROPIC_API_KEY"] },
  google: { label: "Google", keys: ["GOOGLE_API_KEY", "GOOGLE_CLOUD_PROJECT"] },
  cursor: { label: "Cursor", keys: ["CURSOR_API_KEY"] },
  github: { label: "GitHub", keys: ["GITHUB_TOKEN", "GITHUB_APP_ID", "GITHUB_WEBHOOK_SECRET"] },
  aws: { label: "AWS", keys: ["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"] },
  webhook: { label: "Webhooks", keys: ["AEGIS_WEBHOOK_SECRET", "GITHUB_WEBHOOK_SECRET"] },
  custom: { label: "Custom", keys: [] },
};

const LS_PROJECT = "aegis_workspace_project_id";
const LS_ENV = "aegis_workspace_env_id";

export function WorkspaceManager() {
  const [tab, setTab] = useState<Tab>("overview");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tenantName, setTenantName] = useState("");
  const [projects, setProjects] = useState<WorkspaceProject[]>([]);
  const [environments, setEnvironments] = useState<WorkspaceEnvironment[]>([]);
  const [projectId, setProjectId] = useState<number | null>(null);
  const [envId, setEnvId] = useState<number | null>(null);
  const [connectors, setConnectors] = useState<WorkspaceConnector[]>([]);
  const [secrets, setSecrets] = useState<WorkspaceSecret[]>([]);
  const [busy, setBusy] = useState(false);

  const [newProjectSlug, setNewProjectSlug] = useState("");
  const [newProjectName, setNewProjectName] = useState("");
  const [secretProvider, setSecretProvider] = useState("openai");
  const [secretKey, setSecretKey] = useState("OPENAI_API_KEY");
  const [secretValue, setSecretValue] = useState("");
  const [secretJson, setSecretJson] = useState('{\n  "OPENAI_API_KEY": ""\n}');
  const [pipelineJson, setPipelineJson] = useState("");
  const [pipelineNote, setPipelineNote] = useState("");

  const [llmProviders, setLlmProviders] = useState<string[]>([]);
  const [llmModels, setLlmModels] = useState<Record<string, string>>({});
  const [llmProvider, setLlmProvider] = useState("openai");
  const [llmModel, setLlmModel] = useState("gpt-4o-mini");
  const [llmPrompt, setLlmPrompt] = useState("Summarize open incidents and the safest next fix step.");
  const [llmReply, setLlmReply] = useState("");
  const [llmUsage, setLlmUsage] = useState("");
  const [runs, setRuns] = useState<LLMRun[]>([]);
  const [graphNodes, setGraphNodes] = useState<GraphNode[]>([]);
  const [graphStats, setGraphStats] = useState({ nodeCount: 0, edgeCount: 0, tokensEstimate: 0 });
  const [nodeKind, setNodeKind] = useState("module");
  const [nodeKey, setNodeKey] = useState("");
  const [nodeTitle, setNodeTitle] = useState("");
  const [nodeSummary, setNodeSummary] = useState("");

  const projectEnvs = useMemo(
    () => environments.filter((e) => e.projectId === projectId).sort((a, b) => a.sortOrder - b.sortOrder),
    [environments, projectId],
  );
  const selectedProject = projects.find((p) => p.id === projectId);
  const selectedEnv = projectEnvs.find((e) => e.id === envId);

  const loadBootstrap = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const boot = await workspaceBootstrap();
      setTenantName(boot.tenant.name);
      setProjects(boot.projects);
      setEnvironments(boot.environments);
      const storedProject = Number(localStorage.getItem(LS_PROJECT) || 0);
      const storedEnv = Number(localStorage.getItem(LS_ENV) || 0);
      const pid =
        boot.projects.find((p) => p.id === storedProject)?.id ?? boot.projects.find((p) => p.slug === "aegis")?.id ?? boot.projects[0]?.id ?? null;
      setProjectId(pid);
      const envsForProject = boot.environments.filter((e) => e.projectId === pid);
      const eid =
        envsForProject.find((e) => e.id === storedEnv)?.id ??
        envsForProject.find((e) => e.slug === "dev")?.id ??
        envsForProject[0]?.id ??
        null;
      setEnvId(eid);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load workspace");
    } finally {
      setLoading(false);
    }
  }, []);

  const loadEnvDetails = useCallback(async (eid: number) => {
    try {
      const [c, s, status, snap, runList, pipe] = await Promise.all([
        workspaceListConnectors(eid),
        workspaceListSecrets(eid),
        llmStatus(eid).catch(() => null),
        graphSnapshot(eid).catch(() => null),
        llmRuns(eid).catch(() => null),
        getPipeline(eid).catch(() => null),
      ]);
      setConnectors(c.connectors);
      setSecrets(s.secrets);
      if (status) {
        setLlmProviders(status.providers);
        setLlmModels(status.recommendedModels);
        if (status.providers.length && !status.providers.includes(llmProvider)) {
          setLlmProvider(status.providers[0]);
          setLlmModel(status.recommendedModels[status.providers[0]] ?? "gpt-4o-mini");
        }
      }
      if (snap) {
        setGraphNodes(snap.nodes);
        setGraphStats(snap.stats);
      }
      if (runList) setRuns(runList.runs);
      if (pipe) {
        setPipelineJson(JSON.stringify(pipe.pipeline.definition, null, 2));
        setPipelineNote(pipe.editPolicy);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load environment");
    }
  }, [llmProvider]);

  useEffect(() => {
    void loadBootstrap();
  }, [loadBootstrap]);

  useEffect(() => {
    if (projectId) localStorage.setItem(LS_PROJECT, String(projectId));
  }, [projectId]);

  useEffect(() => {
    if (envId) {
      localStorage.setItem(LS_ENV, String(envId));
      void loadEnvDetails(envId);
    }
  }, [envId, loadEnvDetails]);

  useEffect(() => {
    if (!projectId) return;
    const envs = environments.filter((e) => e.projectId === projectId);
    if (!envs.some((e) => e.id === envId)) {
      setEnvId(envs.find((e) => e.slug === "dev")?.id ?? envs[0]?.id ?? null);
    }
  }, [projectId, environments, envId]);

  async function handleCreateProject() {
    if (!newProjectSlug.trim() || !newProjectName.trim()) return;
    setBusy(true);
    try {
      await workspaceCreateProject({ slug: newProjectSlug, name: newProjectName });
      setNewProjectSlug("");
      setNewProjectName("");
      await loadBootstrap();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create project");
    } finally {
      setBusy(false);
    }
  }

  async function toggleConnector(type: string) {
    if (!envId) return;
    const existing = connectors.find((c) => c.connectorType === type);
    const meta = CONNECTOR_META[type];
    setBusy(true);
    try {
      await workspaceUpsertConnector(envId, {
        connectorType: type,
        name: meta?.label ?? type,
        enabled: !(existing?.enabled ?? false),
        config: existing?.config ?? meta?.defaults ?? {},
      });
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update connector");
    } finally {
      setBusy(false);
    }
  }

  async function saveConnectorConfig(type: string, configJson: string) {
    if (!envId) return;
    let config: Record<string, unknown>;
    try {
      config = JSON.parse(configJson) as Record<string, unknown>;
    } catch {
      setError("Connector config must be valid JSON");
      return;
    }
    const existing = connectors.find((c) => c.connectorType === type);
    const meta = CONNECTOR_META[type];
    setBusy(true);
    try {
      await workspaceUpsertConnector(envId, {
        connectorType: type,
        name: existing?.name ?? meta?.label ?? type,
        enabled: existing?.enabled ?? false,
        config,
      });
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save connector");
    } finally {
      setBusy(false);
    }
  }

  async function handleSaveSecret() {
    if (!envId || !secretKey.trim() || !secretValue.trim()) return;
    setBusy(true);
    try {
      await workspaceUpsertSecret(envId, { provider: secretProvider, keyName: secretKey, value: secretValue });
      setSecretValue("");
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save secret");
    } finally {
      setBusy(false);
    }
  }

  async function handleImportJsonSecrets() {
    if (!envId) return;
    setBusy(true);
    setError(null);
    try {
      const obj = JSON.parse(secretJson) as Record<string, string>;
      const entries = Object.entries(obj).filter(([k, v]) => k && typeof v === "string" && v.length > 0);
      if (!entries.length) throw new Error("JSON must be { KEY: value, ... } with non-empty values");
      await bulkUpsertSecrets(
        envId,
        entries.map(([keyName, value]) => ({
          provider: secretProvider,
          keyName,
          value,
        })),
      );
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "JSON import failed");
    } finally {
      setBusy(false);
    }
  }

  async function handleDeleteSecret(id: number) {
    if (!envId) return;
    setBusy(true);
    try {
      await workspaceDeleteSecret(envId, id);
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to delete secret");
    } finally {
      setBusy(false);
    }
  }

  async function handleLlmComplete() {
    if (!envId || !llmPrompt.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const res = await llmComplete(envId, {
        provider: llmProvider,
        model: llmModel,
        prompt: llmPrompt,
        useGraph: true,
      });
      setLlmReply(res.response);
      setLlmUsage(
        `${res.usage.promptTokens} prompt · ${res.usage.completionTokens} completion · ${res.usage.graphNodesUsed} graph nodes — ${res.tokenSavingNote}`,
      );
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "LLM request failed");
    } finally {
      setBusy(false);
    }
  }

  async function handleGraphIngest() {
    if (!envId) return;
    setBusy(true);
    try {
      await graphIngest(envId);
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Graph ingest failed");
    } finally {
      setBusy(false);
    }
  }

  async function handleAddGraphNode() {
    if (!envId || !nodeKey.trim() || !nodeTitle.trim()) return;
    setBusy(true);
    try {
      await graphUpsertNode(envId, {
        kind: nodeKind,
        nodeKey: nodeKey,
        title: nodeTitle,
        summary: nodeSummary,
      });
      setNodeKey("");
      setNodeTitle("");
      setNodeSummary("");
      await loadEnvDetails(envId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to add node");
    } finally {
      setBusy(false);
    }
  }

  const tabs: { id: Tab; label: string; icon: typeof Layers }[] = [
    { id: "overview", label: "Overview", icon: Layers },
    { id: "environments", label: "Environments", icon: Boxes },
    { id: "connectors", label: "Connectors", icon: Plug },
    { id: "secrets", label: "Secrets", icon: KeyRound },
    { id: "llm", label: "LLM API", icon: Sparkles },
    { id: "graph", label: "Context graph", icon: Network },
    { id: "pipeline", label: "Pipeline", icon: Workflow },
  ];

  if (loading) {
    return <p className="text-sm text-[var(--text-muted)]">Loading workspace…</p>;
  }

  return (
    <div className="space-y-6">
      <GlassCard className="flex flex-wrap items-center gap-4">
        <div className="min-w-[140px]">
          <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--text-muted)]">Tenant</p>
          <p className="text-lg font-semibold text-[var(--text-active)]">{tenantName}</p>
        </div>
        <div className="h-8 w-px bg-[var(--panel-border)]" />
        <div>
          <FieldLabel>Project</FieldLabel>
          <select
            className="rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-3 py-2 text-sm text-[var(--text-active)]"
            value={projectId ?? ""}
            onChange={(e) => setProjectId(Number(e.target.value))}
          >
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.slug})
              </option>
            ))}
          </select>
        </div>
        <div>
          <FieldLabel>Environment</FieldLabel>
          <div className="flex gap-2">
            {projectEnvs.map((e) => (
              <button
                key={e.id}
                type="button"
                onClick={() => setEnvId(e.id)}
                className={`rounded-xl px-4 py-2 text-sm font-semibold transition ${
                  envId === e.id
                    ? "bg-sky-500/20 text-sky-400 ring-1 ring-sky-400/30"
                    : "bg-[var(--glass-bg)] text-[var(--text-muted)] hover:text-[var(--text-active)]"
                }`}
              >
                {e.slug}
              </button>
            ))}
          </div>
        </div>
        {selectedProject && selectedEnv && (
          <div className="ml-auto text-right text-xs text-[var(--text-muted)]">
            <p>
              Scope: <span className="font-mono text-[var(--text-active)]">{selectedProject.slug}/{selectedEnv.slug}</span>
            </p>
          </div>
        )}
      </GlassCard>

      {error && (
        <div className="rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-300">{error}</div>
      )}

      <div className="flex flex-wrap gap-2">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => setTab(id)}
            className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition ${
              tab === id
                ? "bg-violet-500/15 text-violet-300 ring-1 ring-violet-400/25"
                : "text-[var(--text-muted)] hover:bg-[var(--glass-bg)]"
            }`}
          >
            <Icon size={16} />
            {label}
          </button>
        ))}
      </div>

      {tab === "overview" && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <GlassCard label="Hierarchy">
            <p className="text-sm text-[var(--text-muted)]">
              <span className="text-[var(--text-active)]">{tenantName}</span>
              {" → "}
              <span className="text-[var(--text-active)]">{selectedProject?.slug ?? "—"}</span>
              {" → "}
              <span className="text-[var(--text-active)]">{selectedEnv?.slug ?? "—"}</span>
            </p>
            <p className="mt-3 text-xs text-[var(--text-muted)]">
              Each environment has isolated connectors and encrypted secrets. Map your staging `.env` into dev/qa/prod here.
            </p>
          </GlassCard>
          <GlassCard label="Connectors enabled">
            <p className="text-3xl font-bold tabular-nums text-[var(--text-active)]">
              {connectors.filter((c) => c.enabled).length}
              <span className="text-lg text-[var(--text-muted)]"> / {Object.keys(CONNECTOR_META).length}</span>
            </p>
          </GlassCard>
          <GlassCard label="Secrets stored">
            <p className="text-3xl font-bold tabular-nums text-[var(--text-active)]">{secrets.length}</p>
          </GlassCard>
        </div>
      )}

      {tab === "environments" && (
        <div className="grid gap-6 lg:grid-cols-2">
          <GlassCard label={`Environments — ${selectedProject?.name ?? "project"}`}>
            <ul className="space-y-2">
              {projectEnvs.map((e) => (
                <li
                  key={e.id}
                  className="flex items-center justify-between rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-4 py-3"
                >
                  <div>
                    <p className="font-semibold text-[var(--text-active)]">{e.name}</p>
                    <p className="font-mono text-xs text-[var(--text-muted)]">{e.slug}</p>
                  </div>
                  {envId === e.id && <Badge status="correlated" label="Active" />}
                </li>
              ))}
            </ul>
          </GlassCard>
          <GlassCard label="New project">
            <div className="space-y-3">
              <div>
                <FieldLabel>Slug</FieldLabel>
                <Input value={newProjectSlug} onChange={(e) => setNewProjectSlug(e.target.value)} placeholder="my-app" />
              </div>
              <div>
                <FieldLabel>Display name</FieldLabel>
                <Input value={newProjectName} onChange={(e) => setNewProjectName(e.target.value)} placeholder="My App" />
              </div>
              <Button variant="primary" disabled={busy} onClick={() => void handleCreateProject()}>
                <Plus size={16} /> Create project
              </Button>
            </div>
          </GlassCard>
        </div>
      )}

      {tab === "connectors" && (
        <div className="grid gap-4">
          {Object.entries(CONNECTOR_META).map(([type, meta]) => {
            const Icon = meta.icon;
            const row = connectors.find((c) => c.connectorType === type);
            const enabled = row?.enabled ?? false;
            return (
              <GlassCard key={type} noPadding>
                <div className="flex flex-wrap items-start gap-4 border-b border-[var(--panel-border)] p-5">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-violet-500/15 text-violet-400">
                    <Icon size={20} />
                  </div>
                  <div className="flex-1 min-w-[200px]">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold text-[var(--text-active)]">{meta.label}</h3>
                      <Badge status={enabled ? "success" : "default"} label={enabled ? "Enabled" : "Off"} />
                    </div>
                    <p className="mt-1 text-sm text-[var(--text-muted)]">{meta.description}</p>
                  </div>
                  <Button variant={enabled ? "secondary" : "violet"} disabled={busy || !envId} onClick={() => void toggleConnector(type)}>
                    {enabled ? <CheckCircle2 size={16} /> : <Circle size={16} />}
                    {enabled ? "Disable" : "Enable"}
                  </Button>
                </div>
                <div className="p-5">
                  <ConnectorConfigEditor
                    config={row?.config ?? meta.defaults}
                    onSave={(json) => void saveConnectorConfig(type, json)}
                    disabled={busy || !envId}
                  />
                </div>
              </GlassCard>
            );
          })}
        </div>
      )}

      {tab === "secrets" && (
        <div className="grid gap-6 lg:grid-cols-2">
          <GlassCard label=".ENV / secrets (per project · env)">
            <p className="mb-3 text-xs text-[var(--text-muted)]">
              AWS Secrets Manager style: name + value, or JSON bulk. Values encrypted at rest — never shown again (SOC2).
            </p>
            <div className="space-y-3">
              <div>
                <FieldLabel>Provider tag</FieldLabel>
                <select
                  className="w-full rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-3 py-2 text-sm"
                  value={secretProvider}
                  onChange={(e) => {
                    const p = e.target.value;
                    setSecretProvider(p);
                    const first = PROVIDER_KEYS[p]?.keys[0];
                    if (first) setSecretKey(first);
                  }}
                >
                  {Object.entries(PROVIDER_KEYS).map(([id, { label }]) => (
                    <option key={id} value={id}>
                      {label}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <FieldLabel>Name</FieldLabel>
                <Input value={secretKey} onChange={(e) => setSecretKey(e.target.value)} placeholder="OPENAI_API_KEY" />
                <div className="mt-2 flex flex-wrap gap-1">
                  {(PROVIDER_KEYS[secretProvider]?.keys ?? []).map((k) => (
                    <button
                      key={k}
                      type="button"
                      className="rounded-lg bg-[var(--glass-bg)] px-2 py-1 font-mono text-[10px] text-[var(--text-muted)] hover:text-sky-400"
                      onClick={() => setSecretKey(k)}
                    >
                      {k}
                    </button>
                  ))}
                </div>
              </div>
              <div>
                <FieldLabel>Value</FieldLabel>
                <Input type="password" value={secretValue} onChange={(e) => setSecretValue(e.target.value)} placeholder="sk-…" />
              </div>
              <Button variant="primary" disabled={busy || !envId} onClick={() => void handleSaveSecret()}>
                Save secret
              </Button>
              <div className="border-t border-[var(--panel-border)] pt-4">
                <FieldLabel>JSON import (like Secrets Manager)</FieldLabel>
                <TextArea value={secretJson} onChange={(e) => setSecretJson(e.target.value)} rows={6} className="font-mono text-xs" />
                <Button className="mt-2" variant="secondary" disabled={busy || !envId} onClick={() => void handleImportJsonSecrets()}>
                  Import JSON keys
                </Button>
              </div>
            </div>
          </GlassCard>
          <GlassCard label={`Stored — ${selectedEnv?.slug ?? "env"} (masked)`}>
            {secrets.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">No secrets yet for this environment.</p>
            ) : (
              <ul className="space-y-2">
                {secrets.map((s) => (
                  <li
                    key={s.id}
                    className="flex items-center justify-between gap-3 rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-4 py-3"
                  >
                    <div className="min-w-0">
                      <p className="font-mono text-sm text-[var(--text-active)]">{s.keyName}</p>
                      <p className="text-xs text-[var(--text-muted)]">
                        {s.provider} · {s.maskedValue}
                      </p>
                    </div>
                    <button
                      type="button"
                      className="shrink-0 text-rose-400 hover:text-rose-300"
                      disabled={busy}
                      onClick={() => void handleDeleteSecret(s.id)}
                    >
                      <Trash2 size={16} />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </GlassCard>
        </div>
      )}

      {tab === "pipeline" && (
        <GlassCard label="Closed-loop pipeline (admin edit only)">
          <p className="mb-3 text-sm text-[var(--text-muted)]">
            Custom per project/environment. Policy: {pipelineNote || "admin only"}. Other roles view only.
          </p>
          <TextArea className="font-mono text-xs" rows={14} value={pipelineJson} readOnly />
          <p className="mt-2 text-xs text-[var(--text-muted)]">
            Use Settings as admin to confirm role. Pipeline mutation API requires project admin.
          </p>
        </GlassCard>
      )}

      {tab === "llm" && (
        <div className="grid gap-6 lg:grid-cols-2">
          <GlassCard label="LLM via API keys (no IDE required)">
            <p className="mb-4 text-sm text-[var(--text-muted)]">
              Calls OpenAI, Anthropic, Google, or Cursor using secrets stored for this environment. Context graph is injected so the model does not re-read the whole repo.
            </p>
            {llmProviders.length === 0 ? (
              <p className="rounded-xl border border-amber-400/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-200">
                No LLM API keys detected. Add <span className="font-mono">OPENAI_API_KEY</span>,{" "}
                <span className="font-mono">ANTHROPIC_API_KEY</span>, <span className="font-mono">GOOGLE_API_KEY</span>, or{" "}
                <span className="font-mono">CURSOR_API_KEY</span> under Secrets.
              </p>
            ) : (
              <p className="mb-3 text-xs text-[var(--text-muted)]">
                Ready: {llmProviders.join(", ")}
              </p>
            )}
            <div className="mt-3 space-y-3">
              <div className="grid gap-3 sm:grid-cols-2">
                <div>
                  <FieldLabel>Provider</FieldLabel>
                  <select
                    className="w-full rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-3 py-2 text-sm"
                    value={llmProvider}
                    onChange={(e) => {
                      const p = e.target.value;
                      setLlmProvider(p);
                      setLlmModel(llmModels[p] ?? "");
                    }}
                  >
                    {["openai", "anthropic", "google", "cursor"].map((p) => (
                      <option key={p} value={p}>
                        {p}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <FieldLabel>Model</FieldLabel>
                  <Input value={llmModel} onChange={(e) => setLlmModel(e.target.value)} placeholder="gpt-4o-mini" />
                </div>
              </div>
              <div>
                <FieldLabel>Prompt</FieldLabel>
                <TextArea value={llmPrompt} onChange={(e) => setLlmPrompt(e.target.value)} rows={4} />
              </div>
              <Button variant="primary" disabled={busy || !envId} onClick={() => void handleLlmComplete()}>
                <Sparkles size={16} /> Run with graph context
              </Button>
              {llmUsage && <p className="text-xs text-[var(--text-muted)]">{llmUsage}</p>}
              {llmReply && (
                <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-xl border border-[var(--panel-border)] bg-[var(--surface)] p-3 text-xs text-[var(--text-active)]">
                  {llmReply}
                </pre>
              )}
            </div>
          </GlassCard>
          <GlassCard label="Recent LLM runs">
            {runs.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">No runs yet.</p>
            ) : (
              <ul className="space-y-2">
                {runs.map((r) => (
                  <li key={r.id} className="rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-3 py-2">
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-xs font-semibold text-[var(--text-active)]">
                        {r.provider}/{r.model || "—"}
                      </span>
                      <Badge status={r.status === "ok" ? "success" : "failure"} label={r.status} />
                    </div>
                    <p className="mt-1 text-[10px] text-[var(--text-muted)]">
                      {r.promptTokens}+{r.completionTokens} tokens · {r.graphNodesUsed} graph nodes
                    </p>
                    <p className="mt-1 line-clamp-2 text-xs text-[var(--text-muted)]">{r.responseSummary}</p>
                  </li>
                ))}
              </ul>
            )}
          </GlassCard>
        </div>
      )}

      {tab === "graph" && (
        <div className="grid gap-6 lg:grid-cols-2">
          <GlassCard label="Token-saving context graph">
            <p className="mb-3 text-sm text-[var(--text-muted)]">
              Store compact summaries (modules, paths, issues). LLM retrieval pulls only matching nodes instead of the full codebase.
            </p>
            <div className="mb-4 flex flex-wrap gap-3 text-sm">
              <span className="rounded-lg bg-[var(--accent-soft)] px-2 py-1 text-[var(--accent)]">{graphStats.nodeCount} nodes</span>
              <span className="rounded-lg bg-[var(--glass-bg)] px-2 py-1 text-[var(--text-muted)]">{graphStats.edgeCount} edges</span>
              <span className="rounded-lg bg-[var(--glass-bg)] px-2 py-1 text-[var(--text-muted)]">~{graphStats.tokensEstimate} tokens if all loaded</span>
            </div>
            <Button variant="primary" disabled={busy || !envId} onClick={() => void handleGraphIngest()}>
              Ingest project + issues
            </Button>
            <div className="mt-5 space-y-3 border-t border-[var(--panel-border)] pt-4">
              <FieldLabel>Add node</FieldLabel>
              <div className="grid gap-2 sm:grid-cols-2">
                <select
                  className="rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-3 py-2 text-sm"
                  value={nodeKind}
                  onChange={(e) => setNodeKind(e.target.value)}
                >
                  {["module", "path", "service", "symbol", "issue", "runbook"].map((k) => (
                    <option key={k} value={k}>
                      {k}
                    </option>
                  ))}
                </select>
                <Input value={nodeKey} onChange={(e) => setNodeKey(e.target.value)} placeholder="node key e.g. orchestrator" />
              </div>
              <Input value={nodeTitle} onChange={(e) => setNodeTitle(e.target.value)} placeholder="Title" />
              <TextArea value={nodeSummary} onChange={(e) => setNodeSummary(e.target.value)} placeholder="Short summary (keep under ~400 chars)" rows={3} />
              <Button variant="secondary" disabled={busy || !envId} onClick={() => void handleAddGraphNode()}>
                <Plus size={16} /> Save node
              </Button>
            </div>
          </GlassCard>
          <GlassCard label="Nodes">
            {graphNodes.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">Empty. Click “Ingest project + issues” to seed.</p>
            ) : (
              <ul className="max-h-[520px] space-y-2 overflow-y-auto">
                {graphNodes.map((n) => (
                  <li key={n.id} className="rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] px-3 py-2">
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-sm font-medium text-[var(--text-active)]">{n.title}</p>
                      <span className="font-mono text-[10px] text-[var(--text-muted)]">~{n.tokensEstimate} tok</span>
                    </div>
                    <p className="text-[10px] uppercase tracking-wide text-[var(--accent)]">
                      {n.kind} · {n.nodeKey}
                    </p>
                    <p className="mt-1 line-clamp-2 text-xs text-[var(--text-muted)]">{n.summary}</p>
                  </li>
                ))}
              </ul>
            )}
          </GlassCard>
        </div>
      )}
    </div>
  );
}

function ConnectorConfigEditor({
  config,
  onSave,
  disabled,
}: {
  config: Record<string, unknown>;
  onSave: (json: string) => void;
  disabled?: boolean;
}) {
  const [text, setText] = useState(() => JSON.stringify(config, null, 2));
  useEffect(() => {
    setText(JSON.stringify(config, null, 2));
  }, [config]);
  return (
    <div className="space-y-2">
      <FieldLabel>Config (JSON)</FieldLabel>
      <textarea
        className="min-h-[120px] w-full rounded-xl border border-[var(--panel-border)] bg-[var(--glass-bg)] p-3 font-mono text-xs text-[var(--text-active)]"
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      <Button variant="secondary" disabled={disabled} onClick={() => onSave(text)}>
        Save config
      </Button>
    </div>
  );
}
