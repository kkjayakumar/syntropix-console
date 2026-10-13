"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  Bot,
  Check,
  CircleDot,
  Cloud,
  Database,
  GitBranch,
  GitPullRequest,
  KeyRound,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  Server,
  Settings2,
  ShieldCheck,
  Trash2,
  Workflow,
  Wrench,
} from "lucide-react";
import {
  bulkUpsertSecrets,
  getPipeline,
  listEnvVariables,
  projectAccessDecision,
  workspaceBootstrap,
  workspaceCreateEnvironment,
  workspaceCreateProject,
  workspaceDeleteEnvironment,
  workspaceApplyConnectorDefaults,
  workspaceListConnectors,
  workspaceListSecrets,
  workspaceUpsertConnector,
  workspaceUpsertSecret,
  upsertEnvVariable,
  upsertPipeline,
} from "@/lib/api";
import type {
  AccessDecision,
  WorkspaceConnector,
  WorkspaceEnvironment,
  WorkspaceProject,
  WorkspaceSecret,
} from "@/lib/types";

type Tab = "loop" | "connections" | "pipeline" | "configuration";

const PROJECT_KEY = "aegis_workspace_project_id";
const ENV_KEY = "aegis_workspace_env_id";

const MAX_ENVS = 5;

const connectorCatalog = [
  { type: "git", label: "Source", detail: "GitHub repository", icon: GitBranch },
  { type: "pull_request", label: "Pull request", detail: "Review & approval", icon: GitPullRequest },
  { type: "cicd", label: "CI / CD", detail: "Build & test", icon: Workflow },
  { type: "deployment", label: "Deploy", detail: "Server / cloud", icon: Cloud },
  { type: "telemetry", label: "Telemetry", detail: "Logs & performance", icon: Activity },
  { type: "mcp", label: "IDE / MCP", detail: "Cursor · Antigravity · Claude", icon: Wrench },
  { type: "issue_agent", label: "AI agent", detail: "Any LLM — no lock-in", icon: Bot },
] as const;

const canonicalStageNames: Record<string, string> = Object.fromEntries(
  connectorCatalog.map((item) => [item.type, item.label]),
);

type FieldDef = {
  key: string;
  label: string;
  placeholder?: string;
  kind?: "text" | "number" | "boolean" | "list";
};

const configFields: Record<string, FieldDef[]> = {
  git: [
    { key: "provider", label: "Provider", placeholder: "github" },
    { key: "repository", label: "Repository", placeholder: "owner/repo" },
    { key: "defaultBranch", label: "Default branch", placeholder: "main" },
  ],
  pull_request: [
    { key: "baseBranch", label: "Base branch", placeholder: "main" },
    { key: "requiredApprovals", label: "Required approvals", kind: "number" },
    { key: "autoOpen", label: "Auto-open PRs", kind: "boolean" },
  ],
  cicd: [
    { key: "provider", label: "Provider", placeholder: "github_actions" },
    { key: "workflows", label: "Workflows", placeholder: "ci.yml, deploy-ec2.yml", kind: "list" },
  ],
  deployment: [
    { key: "provider", label: "Provider", placeholder: "ec2_ssm" },
    { key: "region", label: "Region", placeholder: "ap-south-1" },
    { key: "target", label: "Target", placeholder: "i-…" },
    { key: "verifyUrl", label: "Verify URL", placeholder: "https://…/healthz" },
    { key: "secretsManagerId", label: "Secrets Manager id", placeholder: "dev/syntropix" },
  ],
  telemetry: [
    { key: "provider", label: "Provider", placeholder: "syntropix" },
    { key: "healthUrl", label: "Health URL", placeholder: "/healthz" },
    { key: "copilotApiUrl", label: "Copilot API", placeholder: "http://localhost:5000" },
    { key: "spottingApiUrl", label: "Spotting API", placeholder: "http://localhost:3000" },
  ],
  mcp: [
    { key: "ide", label: "IDE", placeholder: "cursor" },
    { key: "transport", label: "Transport", placeholder: "stdio" },
    { key: "tools", label: "Tools", placeholder: "aegisops" },
  ],
  issue_agent: [
    { key: "provider", label: "Provider", placeholder: "openai" },
    { key: "route", label: "Route", placeholder: "api" },
    { key: "useGraph", label: "Use graph memory", kind: "boolean" },
    { key: "allowAnyAgent", label: "Allow any agent", kind: "boolean" },
  ],
};

function configString(cfg: Record<string, unknown>, key: string): string {
  const value = cfg[key];
  if (value == null) return "";
  if (typeof value === "string") return value.trim();
  if (Array.isArray(value)) return value.map(String).join(", ");
  return String(value).trim();
}

function stageReady(type: string, enabled: boolean, cfg: Record<string, unknown>): boolean {
  if (!enabled) return false;
  switch (type) {
    case "git":
      return configString(cfg, "repository") !== "";
    case "pull_request":
      return configString(cfg, "baseBranch") !== "";
    case "cicd":
      return configString(cfg, "provider") !== "" && configString(cfg, "workflows") !== "";
    case "deployment":
      return configString(cfg, "region") !== "" && configString(cfg, "target") !== "";
    case "telemetry":
      return configString(cfg, "healthUrl") !== "";
    case "mcp":
      return configString(cfg, "ide") !== "";
    case "issue_agent":
      return configString(cfg, "provider") !== "" || configString(cfg, "route") !== "";
    default:
      return true;
  }
}

function connectorForType(
  connectors: WorkspaceConnector[],
  type: string,
  stages?: Record<string, WorkspaceConnector>,
): WorkspaceConnector | undefined {
  if (stages?.[type]) return stages[type];
  const ofType = connectors.filter((c) => c.connectorType === type);
  const canonical = canonicalStageNames[type];
  const byName = canonical ? ofType.find((c) => c.name === canonical) : undefined;
  if (byName) return byName;
  const ready = ofType.find((c) => c.ready ?? stageReady(type, c.enabled, c.config ?? {}));
  if (ready) return ready;
  return ofType[0];
}

function draftFromConnector(type: string, connection?: WorkspaceConnector): Record<string, unknown> {
  // Use stored config only — do not merge platform presets (avoids fake "ready" form values).
  return { ...(connection?.config ?? {}) };
}

function slugify(raw: string): string {
  return raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

function normalizeConfig(type: string, draft: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = { ...draft };
  for (const field of configFields[type] ?? []) {
    if (field.kind === "list") {
      const raw = out[field.key];
      if (typeof raw === "string") {
        out[field.key] = raw
          .split(",")
          .map((part) => part.trim())
          .filter(Boolean);
      }
    }
    if (field.kind === "number" && typeof out[field.key] === "string") {
      const n = Number(out[field.key]);
      out[field.key] = Number.isFinite(n) ? n : 0;
    }
  }
  return out;
}

export function ProjectWorkspace() {
  const [projects, setProjects] = useState<WorkspaceProject[]>([]);
  const [environments, setEnvironments] = useState<WorkspaceEnvironment[]>([]);
  const [projectId, setProjectId] = useState<number | null>(null);
  const [envId, setEnvId] = useState<number | null>(null);
  const [connectors, setConnectors] = useState<WorkspaceConnector[]>([]);
  const [stageConnectors, setStageConnectors] = useState<Record<string, WorkspaceConnector>>({});
  const [secrets, setSecrets] = useState<WorkspaceSecret[]>([]);
  const [variables, setVariables] = useState<{ id: number; keyName: string; valueText: string }[]>([]);
  const [access, setAccess] = useState<AccessDecision | null>(null);
  const [pipeline, setPipeline] = useState<Record<string, unknown>>({});
  const [tab, setTab] = useState<Tab>("loop");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newEnv, setNewEnv] = useState("");
  const [addingEnv, setAddingEnv] = useState(false);
  const [newProjectName, setNewProjectName] = useState("");
  const [addingProject, setAddingProject] = useState(false);
  const [secretName, setSecretName] = useState("");
  const [secretValue, setSecretValue] = useState("");
  const [secretJson, setSecretJson] = useState("");
  const [variableName, setVariableName] = useState("");
  const [variableValue, setVariableValue] = useState("");
  const [pipelineText, setPipelineText] = useState("");
  const [drafts, setDrafts] = useState<Record<string, Record<string, unknown>>>({});
  const [expanded, setExpanded] = useState<string | null>(null);

  const project = projects.find((p) => p.id === projectId);
  const projectEnvs = useMemo(
    () => environments.filter((environment) => environment.projectId === projectId).sort((a, b) => a.sortOrder - b.sortOrder),
    [environments, projectId],
  );
  const environment = projectEnvs.find((item) => item.id === envId);
  const enabledCount = connectorCatalog.filter((item) => {
    const connection = connectorForType(connectors, item.type, stageConnectors);
    if (!connection) return false;
    return connection.ready ?? stageReady(item.type, connection.enabled, connection.config ?? {});
  }).length;
  const loopHealthy = enabledCount === connectorCatalog.length;
  const canEditConnectors = Boolean(access?.canWriteSecret);
  const canManageEnv = Boolean(access?.canWriteSecret);

  const loadBootstrap = useCallback(async () => {
    const data = await workspaceBootstrap();
    const nextProjects = data.projects ?? [];
    const nextEnvironments = data.environments ?? [];
    setProjects(nextProjects);
    setEnvironments(nextEnvironments);
    const stored = Number(localStorage.getItem(PROJECT_KEY) || 0);
    const pid = nextProjects.some((p) => p.id === stored) ? stored : nextProjects[0]?.id ?? null;
    setProjectId(pid);
    const available = nextEnvironments.filter((e) => e.projectId === pid);
    const storedEnv = Number(localStorage.getItem(ENV_KEY) || 0);
    setEnvId(available.some((e) => e.id === storedEnv) ? storedEnv : available[0]?.id ?? null);
  }, []);

  const loadEnvironment = useCallback(async (id: number) => {
    setError(null);
    try {
      const [connections, secretData, variableData, pipelineData] = await Promise.all([
        workspaceListConnectors(id),
        workspaceListSecrets(id),
        listEnvVariables(id),
        getPipeline(id),
      ]);
      const next = connections.connectors ?? [];
      setConnectors(next);
      setStageConnectors(connections.stages ?? {});
      setDrafts((prev) => {
        const nextDrafts: Record<string, Record<string, unknown>> = { ...prev };
        for (const item of connectorCatalog) {
          nextDrafts[item.type] = draftFromConnector(
            item.type,
            connectorForType(next, item.type, connections.stages),
          );
        }
        return nextDrafts;
      });
      setSecrets(secretData.secrets ?? []);
      setVariables(variableData.variables ?? []);
      const definition = pipelineData.pipeline?.definition ?? {};
      setPipeline(definition);
      setPipelineText(JSON.stringify(definition, null, 2));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not load environment");
    }
  }, []);

  useEffect(() => {
    void loadBootstrap();
    const onProject = (event: Event) => setProjectId((event as CustomEvent<number>).detail);
    const onEnv = (event: Event) => setEnvId((event as CustomEvent<number>).detail);
    const onRefresh = () => {
      void loadBootstrap();
    };
    window.addEventListener("aegis:project-change", onProject);
    window.addEventListener("aegis:env-change", onEnv);
    window.addEventListener("aegis:workspace-refresh", onRefresh);
    return () => {
      window.removeEventListener("aegis:project-change", onProject);
      window.removeEventListener("aegis:env-change", onEnv);
      window.removeEventListener("aegis:workspace-refresh", onRefresh);
    };
  }, [loadBootstrap]);

  useEffect(() => {
    if (!projectId) return;
    localStorage.setItem(PROJECT_KEY, String(projectId));
    const available = environments.filter((item) => item.projectId === projectId);
    if (!available.some((item) => item.id === envId)) {
      const next = available[0]?.id ?? null;
      setEnvId(next);
      if (next != null) {
        localStorage.setItem(ENV_KEY, String(next));
        window.dispatchEvent(new CustomEvent("aegis:env-change", { detail: next }));
      }
    }
    projectAccessDecision(projectId).then(setAccess).catch(() => setAccess(null));
  }, [projectId, environments, envId]);

  useEffect(() => {
    if (!envId) return;
    localStorage.setItem(ENV_KEY, String(envId));
    void loadEnvironment(envId);
  }, [envId, loadEnvironment]);

  function selectProject(id: number) {
    localStorage.setItem(PROJECT_KEY, String(id));
    setProjectId(id);
    window.dispatchEvent(new CustomEvent("aegis:project-change", { detail: id }));
    const available = environments
      .filter((item) => item.projectId === id)
      .sort((a, b) => a.sortOrder - b.sortOrder);
    const storedEnv = Number(localStorage.getItem(ENV_KEY) || 0);
    const next = available.some((e) => e.id === storedEnv) ? storedEnv : available[0]?.id ?? null;
    setEnvId(next);
    if (next != null) {
      localStorage.setItem(ENV_KEY, String(next));
      window.dispatchEvent(new CustomEvent("aegis:env-change", { detail: next }));
    }
  }

  function selectEnv(id: number) {
    localStorage.setItem(ENV_KEY, String(id));
    setEnvId(id);
    window.dispatchEvent(new CustomEvent("aegis:env-change", { detail: id }));
  }

  async function saveConnector(type: string, enabled: boolean) {
    if (!envId || !canEditConnectors) return;
    setBusy(true);
    try {
      const existing = connectorForType(connectors, type, stageConnectors);
      const config = normalizeConfig(type, drafts[type] ?? draftFromConnector(type, existing));
      const canEnable = stageReady(type, true, config);
      await workspaceUpsertConnector(envId, {
        connectorType: type,
        name: existing?.name ?? connectorCatalog.find((item) => item.type === type)?.label ?? type,
        enabled: enabled && canEnable,
        config,
      });
      await loadEnvironment(envId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Connection update failed");
    } finally {
      setBusy(false);
    }
  }

  async function connectStage(type: string) {
    if (!envId || !canEditConnectors) return;
    const existing = connectorForType(connectors, type, stageConnectors);
    const draft = drafts[type] ?? draftFromConnector(type, existing);
    const hasDraft = stageReady(type, true, normalizeConfig(type, draft));
    if (hasDraft) {
      await saveConnector(type, true);
      return;
    }
    await connectFromPlatformDefaults([type]);
  }

  async function connectFromPlatformDefaults(types?: string[]) {
    if (!envId || !canEditConnectors) return;
    setBusy(true);
    setError(null);
    try {
      const result = await workspaceApplyConnectorDefaults(envId, { enable: true, types });
      await loadEnvironment(envId);
      if (result.ready < result.total) {
        setError(
          `${result.message}. Deploy needs EC2_INSTANCE_ID on the platform (or set target manually) before it can show Connected.`,
        );
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Apply defaults failed");
    } finally {
      setBusy(false);
    }
  }

  async function applyDefaultsToDraft(type: string) {
    if (!envId || !canEditConnectors) return;
    setBusy(true);
    setError(null);
    try {
      await workspaceApplyConnectorDefaults(envId, { enable: false, types: [type] });
      await loadEnvironment(envId);
      setExpanded(type);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Apply defaults failed");
    } finally {
      setBusy(false);
    }
  }

  function updateDraft(type: string, key: string, value: unknown) {
    setDrafts((prev) => ({
      ...prev,
      [type]: { ...(prev[type] ?? {}), [key]: value },
    }));
  }

  async function createProject() {
    const name = newProjectName.trim();
    if (!name || busy) return;
    const slug = slugify(name);
    if (!slug) {
      setError("Project name needs letters or numbers");
      return;
    }
    setBusy(true);
    try {
      const result = await workspaceCreateProject({ slug, name });
      setNewProjectName("");
      setAddingProject(false);
      await loadBootstrap();
      window.dispatchEvent(new Event("aegis:workspace-refresh"));
      localStorage.setItem(PROJECT_KEY, String(result.project.id));
      setProjectId(result.project.id);
      window.dispatchEvent(new CustomEvent("aegis:project-change", { detail: result.project.id }));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Project creation failed");
    } finally {
      setBusy(false);
    }
  }

  async function createEnvironment() {
    if (!projectId || !newEnv.trim() || projectEnvs.length >= MAX_ENVS || !canManageEnv) return;
    const name = newEnv.trim();
    const slug = slugify(name);
    if (!slug) {
      setError("Environment name needs letters or numbers");
      return;
    }
    setBusy(true);
    try {
      const result = await workspaceCreateEnvironment(projectId, {
        slug,
        name,
        sortOrder: projectEnvs.length + 1,
      });
      setNewEnv("");
      setAddingEnv(false);
      await loadBootstrap();
      window.dispatchEvent(new Event("aegis:workspace-refresh"));
      selectEnv(result.environment.id);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Environment creation failed");
    } finally {
      setBusy(false);
    }
  }

  async function deleteEnvironment() {
    if (!envId || !environment || !canManageEnv || projectEnvs.length <= 1) return;
    const ok = window.confirm(
      `Delete environment “${environment.name}” (${environment.slug})? Connectors and secrets for this env will be removed.`,
    );
    if (!ok) return;
    setBusy(true);
    try {
      await workspaceDeleteEnvironment(envId);
      const remaining = projectEnvs.filter((e) => e.id !== envId);
      const next = remaining[0]?.id ?? null;
      await loadBootstrap();
      window.dispatchEvent(new Event("aegis:workspace-refresh"));
      if (next != null) {
        selectEnv(next);
      } else {
        setEnvId(null);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Environment delete failed");
    } finally {
      setBusy(false);
    }
  }

  async function saveSecret() {
    if (!envId || !secretName || !secretValue) return;
    setBusy(true);
    try {
      await workspaceUpsertSecret(envId, { provider: "custom", keyName: secretName, value: secretValue });
      setSecretName("");
      setSecretValue("");
      await loadEnvironment(envId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Secret save failed");
    } finally {
      setBusy(false);
    }
  }

  async function importSecrets() {
    if (!envId || !secretJson.trim()) return;
    setBusy(true);
    try {
      const parsed = JSON.parse(secretJson) as Record<string, string>;
      const items = Object.entries(parsed).map(([keyName, value]) => ({
        provider: "custom",
        keyName,
        value: String(value),
      }));
      await bulkUpsertSecrets(envId, items);
      setSecretJson("");
      await loadEnvironment(envId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Secret import failed");
    } finally {
      setBusy(false);
    }
  }

  async function saveVariable() {
    if (!envId || !projectId || !variableName) return;
    setBusy(true);
    try {
      await upsertEnvVariable(envId, { keyName: variableName, valueText: variableValue, projectId });
      setVariableName("");
      setVariableValue("");
      await loadEnvironment(envId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Variable save failed");
    } finally {
      setBusy(false);
    }
  }

  async function savePipeline() {
    if (!envId || !projectId || !access?.canEditPipeline) return;
    setBusy(true);
    try {
      const definition = JSON.parse(pipelineText) as Record<string, unknown>;
      await upsertPipeline(envId, { projectId, name: "closed-loop", definition });
      setPipeline(definition);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Pipeline JSON is invalid");
    } finally {
      setBusy(false);
    }
  }

  if (!project) {
    return (
      <div className="p-8">
        <h1 className="text-xl font-semibold text-[var(--text-active)]">Workspace</h1>
        <p className="mt-2 text-sm text-[var(--text-muted)]">Create a project to get started.</p>
        <div className="mt-4 flex max-w-sm items-center gap-2">
          <input
            value={newProjectName}
            onChange={(e) => setNewProjectName(e.target.value)}
            placeholder="Project name"
            className="config-input flex-1"
            onKeyDown={(e) => {
              if (e.key === "Enter") void createProject();
            }}
          />
          <button type="button" disabled={busy || !newProjectName.trim()} onClick={() => void createProject()} className="agent-chip">
            <Plus size={12} /> Create
          </button>
        </div>
        {error && <div className="mt-4 rounded-xl border border-rose-500/25 bg-rose-500/10 px-4 py-3 text-sm text-rose-400">{error}</div>}
      </div>
    );
  }

  return (
    <div className="min-h-full bg-[var(--background)]">
      <header className="border-b border-[var(--panel-border)] bg-[var(--surface)] px-6 py-5 lg:px-8">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-[var(--text-muted)]">
              <span>Syntropix</span>
              <span>/</span>
              <label className="inline-flex items-center gap-1.5">
                <span className="sr-only">Project</span>
                <select
                  className="rounded-lg border border-[var(--panel-border)] bg-[var(--background)] px-2 py-1 text-xs font-medium text-[var(--text-active)] outline-none"
                  value={projectId ?? ""}
                  onChange={(e) => {
                    const id = Number(e.target.value);
                    if (Number.isFinite(id) && id > 0) selectProject(id);
                  }}
                >
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </label>
              {!addingProject ? (
                <button
                  type="button"
                  className="rounded-md px-1.5 py-0.5 text-[var(--text-muted)] hover:bg-[var(--glass-bg)] hover:text-[var(--text-active)]"
                  title="New project"
                  onClick={() => setAddingProject(true)}
                >
                  <Plus size={12} />
                </button>
              ) : (
                <span className="inline-flex items-center gap-1">
                  <input
                    autoFocus
                    value={newProjectName}
                    onChange={(e) => setNewProjectName(e.target.value)}
                    placeholder="New project name"
                    className="w-36 rounded-lg border border-[var(--panel-border)] bg-transparent px-2 py-1 text-xs outline-none"
                    onKeyDown={(e) => {
                      if (e.key === "Enter") void createProject();
                      if (e.key === "Escape") {
                        setAddingProject(false);
                        setNewProjectName("");
                      }
                    }}
                  />
                  <button type="button" disabled={busy || !newProjectName.trim()} onClick={() => void createProject()} className="agent-icon-button" title="Create project">
                    {busy ? <Loader2 size={12} className="animate-spin" /> : <Plus size={12} />}
                  </button>
                </span>
              )}
            </div>
            <h1 className="text-2xl font-semibold tracking-tight text-[var(--text-active)]">{project.name}</h1>
            <p className="mt-1 text-sm text-[var(--text-muted)]">{project.description || "Closed-loop application delivery"}</p>
          </div>
          {loopHealthy ? (
            <div className="status-pill status-pill-ok">
              <Check size={13} />
              Closed loop healthy
            </div>
          ) : (
            <div className="status-pill status-pill-warn">
              <AlertTriangle size={13} />
              {enabledCount === 0
                ? "Connections need setup"
                : `${connectorCatalog.length - enabledCount} connections need attention`}
            </div>
          )}
        </div>

        <div className="mt-5 flex flex-wrap items-center gap-2">
          <span className="mr-1 text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">Environment</span>
          {projectEnvs.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => selectEnv(item.id)}
              className={`env-pill ${item.id === envId ? "env-pill-active" : ""}`}
              title={item.name}
            >
              <span className={`h-1.5 w-1.5 rounded-full ${item.id === envId ? "bg-emerald-400" : "bg-slate-500"}`} />
              {item.name}
            </button>
          ))}
          {canManageEnv && projectEnvs.length < MAX_ENVS && (
            <div className="ml-1 flex items-center gap-1">
              {!addingEnv ? (
                <button type="button" onClick={() => setAddingEnv(true)} className="agent-chip" title="Add environment">
                  <Plus size={12} /> Add env
                </button>
              ) : (
                <>
                  <input
                    autoFocus
                    value={newEnv}
                    onChange={(e) => setNewEnv(e.target.value)}
                    placeholder="Name (e.g. Staging)"
                    className="w-36 rounded-lg border border-[var(--panel-border)] bg-transparent px-2 py-1.5 text-xs outline-none"
                    onKeyDown={(e) => {
                      if (e.key === "Enter") void createEnvironment();
                      if (e.key === "Escape") {
                        setAddingEnv(false);
                        setNewEnv("");
                      }
                    }}
                  />
                  {newEnv.trim() && (
                    <span className="text-[10px] text-[var(--text-muted)]">slug: {slugify(newEnv) || "—"}</span>
                  )}
                  <button type="button" onClick={() => void createEnvironment()} disabled={busy || !newEnv.trim()} className="agent-icon-button" title="Create environment">
                    {busy ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
                  </button>
                </>
              )}
            </div>
          )}
          {canManageEnv && envId && projectEnvs.length > 1 && (
            <button
              type="button"
              disabled={busy}
              onClick={() => void deleteEnvironment()}
              className="agent-chip ml-auto text-rose-400"
              title="Delete this environment"
            >
              <Trash2 size={12} /> Delete env
            </button>
          )}
        </div>
      </header>

      <div className="border-b border-[var(--panel-border)] bg-[var(--surface)] px-6 lg:px-8">
        <div className="flex gap-1 overflow-x-auto">
          {([
            ["loop", "Closed loop", CircleDot],
            ["connections", "Connections", Settings2],
            ["pipeline", "Pipeline", Workflow],
            ["configuration", "Secrets & variables", KeyRound],
          ] as const).map(([id, label, Icon]) => (
            <button
              key={id as string}
              type="button"
              onClick={() => setTab(id as Tab)}
              className={`project-tab ${tab === id ? "project-tab-active" : ""}`}
            >
              <Icon size={14} /> {label as string}
            </button>
          ))}
          <Link href="/issues" className="project-tab"><Wrench size={14} /> Issues</Link>
          <Link href="/issues/new" className="project-tab"><Plus size={14} /> New feature</Link>
          <Link href="/cicd" className="project-tab"><Activity size={14} /> CI / CD</Link>
        </div>
      </div>

      <main className="p-6 lg:p-8">
        {error && <div className="mb-4 rounded-xl border border-rose-500/25 bg-rose-500/10 px-4 py-3 text-sm text-rose-400">{error}</div>}

        {tab === "loop" && (
          <div className="space-y-6">
            <section className="project-panel overflow-hidden p-5">
              <div className="mb-6 flex items-center justify-between">
                <div>
                  <h2 className="font-semibold text-[var(--text-active)]">Environment connection flow</h2>
                  <p className="mt-1 text-xs text-[var(--text-muted)]">
                    Stages show Connected only when enabled with required config for {environment?.name ?? "this environment"}.
                  </p>
                </div>
                <button type="button" onClick={() => envId && loadEnvironment(envId)} className="agent-chip">
                  <RefreshCw size={12} /> Recheck
                </button>
              </div>
              <div className="connector-flow">
                {connectorCatalog.map((item, index) => {
                  const connection = connectorForType(connectors, item.type, stageConnectors);
                  const healthy = Boolean(
                    connection && (connection.ready ?? stageReady(item.type, connection.enabled, connection.config ?? {})),
                  );
                  const Icon = item.icon;
                  return (
                    <div key={item.type} className="flex min-w-0 flex-1 items-center">
                      <button
                        type="button"
                        onClick={() => {
                          setExpanded(item.type);
                          setTab("connections");
                        }}
                        className={`flow-node ${healthy ? "flow-node-ok" : "flow-node-warn"}`}
                      >
                        <div className="flex items-center justify-between">
                          <Icon size={17} />
                          <span className={`connection-dot ${healthy ? "connection-dot-ok" : "connection-dot-warn"}`} />
                        </div>
                        <p className="mt-3 text-sm font-semibold">{item.label}</p>
                        <p className="mt-0.5 text-[10px] opacity-65">{healthy ? "Connected" : "Needs setup"}</p>
                      </button>
                      {index < connectorCatalog.length - 1 && (
                        <div className={`flow-line-segment ${healthy ? "flow-line-ok" : ""}`}><ArrowRight size={12} /></div>
                      )}
                    </div>
                  );
                })}
              </div>
            </section>

            <div className="grid gap-4 md:grid-cols-3">
              <SummaryCard icon={ShieldCheck} label="Environment" value={environment?.slug ?? "—"} detail={`${enabledCount}/${connectorCatalog.length} connections healthy`} tone="emerald" />
              <SummaryCard icon={KeyRound} label="Secure configuration" value={`${secrets.length} secrets`} detail={`${variables.length} visible variables`} tone="indigo" />
              <SummaryCard icon={Activity} label="Delivery operations" value="Ready to observe" detail="Issues, CI/CD and deploy telemetry" tone="sky" />
            </div>
          </div>
        )}

        {tab === "connections" && (
          <div>
            <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-[var(--text-active)]">Connections</h2>
                <p className="text-sm text-[var(--text-muted)]">
                  Configure each stage for this environment. A stage is connected only when it is enabled and required fields are set.
                </p>
              </div>
              {canEditConnectors && (
                <button
                  type="button"
                  disabled={busy || !envId}
                  onClick={() => void connectFromPlatformDefaults()}
                  className="agent-chip"
                  title="Fill non-secret fields from platform settings and enable stages that have required config"
                >
                  {busy ? <Loader2 size={12} className="animate-spin" /> : <Server size={12} />}
                  Connect from platform defaults
                </button>
              )}
            </div>
            <div className="grid gap-3">
              {connectorCatalog.map((item) => {
                const connection = connectorForType(connectors, item.type, stageConnectors);
                const draft = drafts[item.type] ?? draftFromConnector(item.type, connection);
                const enabled = Boolean(connection?.enabled);
                const healthy = connection?.ready ?? stageReady(item.type, enabled, draft);
                const open = expanded === item.type;
                const Icon = item.icon;
                return (
                  <div key={item.type} className="project-panel overflow-hidden">
                    <div className="flex items-center gap-4 p-4">
                      <div className={`flex h-10 w-10 items-center justify-center rounded-xl ${healthy ? "bg-emerald-500/12 text-emerald-500" : "bg-amber-500/10 text-amber-500"}`}>
                        <Icon size={18} />
                      </div>
                      <button
                        type="button"
                        className="min-w-0 flex-1 text-left"
                        onClick={() => setExpanded(open ? null : item.type)}
                      >
                        <p className="text-sm font-semibold text-[var(--text-active)]">{item.label}</p>
                        <p className="text-xs text-[var(--text-muted)]">
                          {healthy ? "Connected" : enabled ? "Enabled — finish required settings" : item.detail}
                        </p>
                      </button>
                      <button
                        type="button"
                        disabled={busy || !canEditConnectors}
                        onClick={() => {
                          if (healthy) {
                            void saveConnector(item.type, false);
                          } else if (enabled) {
                            setExpanded(item.type);
                          } else {
                            void connectStage(item.type);
                          }
                        }}
                        className={`connection-action ${healthy ? "connection-action-ok" : ""}`}
                      >
                        {busy ? <Loader2 size={13} className="animate-spin" /> : healthy ? <Check size={13} /> : <Wrench size={13} />}
                        {healthy ? "Connected" : enabled ? "Complete setup" : "Connect"}
                      </button>
                    </div>
                    {open && (
                      <div className="border-t border-[var(--panel-border)] bg-[var(--background)]/40 px-4 py-4">
                        <div className="grid gap-3 sm:grid-cols-2">
                          {(configFields[item.type] ?? []).map((field) => {
                            if (field.kind === "boolean") {
                              return (
                                <label key={field.key} className="flex items-center gap-2 text-xs text-[var(--text-active)]">
                                  <input
                                    type="checkbox"
                                    checked={Boolean(draft[field.key])}
                                    disabled={!canEditConnectors}
                                    onChange={(e) => updateDraft(item.type, field.key, e.target.checked)}
                                  />
                                  {field.label}
                                </label>
                              );
                            }
                            const display =
                              field.kind === "list" && Array.isArray(draft[field.key])
                                ? (draft[field.key] as unknown[]).map(String).join(", ")
                                : draft[field.key] == null
                                  ? ""
                                  : String(draft[field.key]);
                            return (
                              <label key={field.key} className="block text-xs text-[var(--text-muted)]">
                                <span className="mb-1 block font-medium text-[var(--text-active)]">{field.label}</span>
                                <input
                                  value={display}
                                  disabled={!canEditConnectors}
                                  placeholder={field.placeholder}
                                  type={field.kind === "number" ? "number" : "text"}
                                  onChange={(e) =>
                                    updateDraft(
                                      item.type,
                                      field.key,
                                      field.kind === "number" ? e.target.value : e.target.value,
                                    )
                                  }
                                  className="config-input"
                                />
                              </label>
                            );
                          })}
                        </div>
                        {canEditConnectors ? (
                          <div className="mt-3 flex flex-wrap gap-2">
                            <button
                              type="button"
                              disabled={busy}
                              onClick={() => void applyDefaultsToDraft(item.type)}
                              className="agent-chip"
                            >
                              <Server size={12} /> Apply defaults
                            </button>
                            <button
                              type="button"
                              disabled={busy}
                              onClick={() => void connectFromPlatformDefaults([item.type])}
                              className="agent-chip"
                            >
                              <Check size={12} /> Connect with defaults
                            </button>
                            <button
                              type="button"
                              disabled={busy}
                              onClick={() => void saveConnector(item.type, true)}
                              className="agent-chip"
                            >
                              <Save size={12} /> Save & enable
                            </button>
                            {enabled && (
                              <button
                                type="button"
                                disabled={busy}
                                onClick={() => void saveConnector(item.type, false)}
                                className="agent-chip"
                              >
                                Disable
                              </button>
                            )}
                          </div>
                        ) : (
                          <p className="mt-3 text-xs text-[var(--text-muted)]">Admin or maintainer role required to edit connections.</p>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {tab === "pipeline" && (
          <div className="grid gap-5 lg:grid-cols-[1fr_320px]">
            <section className="project-panel p-5">
              <div className="mb-4 flex items-center justify-between">
                <div>
                  <h2 className="font-semibold text-[var(--text-active)]">Custom closed-loop pipeline</h2>
                  <p className="text-xs text-[var(--text-muted)]">Project + environment scoped. Admin-only edits.</p>
                </div>
                {access?.canEditPipeline && (
                  <button type="button" onClick={() => void savePipeline()} className="agent-chip"><Save size={12} /> Save</button>
                )}
              </div>
              <textarea
                value={pipelineText}
                onChange={(e) => setPipelineText(e.target.value)}
                readOnly={!access?.canEditPipeline}
                className="min-h-[360px] w-full resize-y rounded-xl border border-[var(--panel-border)] bg-[var(--background)] p-4 font-mono text-xs leading-6 text-[var(--text-active)] outline-none"
              />
            </section>
            <aside className="space-y-3">
              <SummaryCard icon={ShieldCheck} label="Edit policy" value={access?.canEditPipeline ? "Admin access" : "View only"} detail="Pipeline changes are audit scoped" tone="indigo" />
              <SummaryCard icon={Workflow} label="Steps" value={String(Array.isArray(pipeline.steps) ? pipeline.steps.length : 0)} detail="Detect through verify and sentinel" tone="emerald" />
            </aside>
          </div>
        )}

        {tab === "configuration" && (
          <div className="grid gap-5 xl:grid-cols-2">
            <section className="project-panel p-5">
              <div className="mb-4">
                <h2 className="font-semibold text-[var(--text-active)]">Encrypted secrets</h2>
                <p className="text-xs text-[var(--text-muted)]">Write-only AES-256-GCM values. Plaintext is never returned.</p>
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <input value={secretName} onChange={(e) => setSecretName(e.target.value)} placeholder="SECRET_NAME" className="config-input" />
                <input value={secretValue} onChange={(e) => setSecretValue(e.target.value)} placeholder="Secret value" type="password" className="config-input" />
              </div>
              <button type="button" onClick={() => void saveSecret()} className="mt-2 agent-chip"><KeyRound size={12} /> Add secret</button>
              <div className="mt-4 space-y-2">
                {secrets.map((secret) => (
                  <div key={secret.id} className="config-row">
                    <div><p className="font-mono text-xs text-[var(--text-active)]">{secret.keyName}</p><p className="text-[10px] text-[var(--text-muted)]">{secret.provider}</p></div>
                    <span className="font-mono text-xs text-[var(--text-muted)]">{secret.maskedValue}</span>
                  </div>
                ))}
              </div>
              <div className="mt-5 border-t border-[var(--panel-border)] pt-4">
                <p className="mb-2 text-xs font-medium text-[var(--text-active)]">Import JSON</p>
                <textarea value={secretJson} onChange={(e) => setSecretJson(e.target.value)} placeholder={'{"OPENAI_API_KEY":"…","GITHUB_TOKEN":"…"}'} className="config-textarea" />
                <button type="button" onClick={() => void importSecrets()} className="mt-2 agent-chip"><Database size={12} /> Import</button>
              </div>
            </section>

            <section className="project-panel p-5">
              <div className="mb-4">
                <h2 className="font-semibold text-[var(--text-active)]">Environment variables</h2>
                <p className="text-xs text-[var(--text-muted)]">Visible configuration for {project.name} / {environment?.slug}.</p>
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <input value={variableName} onChange={(e) => setVariableName(e.target.value)} placeholder="VARIABLE_NAME" className="config-input" />
                <input value={variableValue} onChange={(e) => setVariableValue(e.target.value)} placeholder="Value" className="config-input" />
              </div>
              <button type="button" onClick={() => void saveVariable()} className="mt-2 agent-chip"><Plus size={12} /> Add variable</button>
              <div className="mt-4 space-y-2">
                {variables.map((variable) => (
                  <div key={variable.id} className="config-row">
                    <span className="font-mono text-xs text-[var(--text-active)]">{variable.keyName}</span>
                    <span className="max-w-[55%] truncate font-mono text-xs text-[var(--text-muted)]">{variable.valueText}</span>
                  </div>
                ))}
              </div>
            </section>
          </div>
        )}
      </main>
    </div>
  );
}

function SummaryCard({
  icon: Icon,
  label,
  value,
  detail,
  tone,
}: {
  icon: typeof Server;
  label: string;
  value: string;
  detail: string;
  tone: "emerald" | "indigo" | "sky";
}) {
  const tones = {
    emerald: "bg-emerald-500/10 text-emerald-500",
    indigo: "bg-indigo-500/10 text-indigo-500",
    sky: "bg-sky-500/10 text-sky-500",
  };
  return (
    <div className="project-panel p-4">
      <div className={`mb-3 flex h-8 w-8 items-center justify-center rounded-lg ${tones[tone]}`}><Icon size={15} /></div>
      <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">{label}</p>
      <p className="mt-1 text-base font-semibold text-[var(--text-active)]">{value}</p>
      <p className="mt-1 text-xs text-[var(--text-muted)]">{detail}</p>
    </div>
  );
}
