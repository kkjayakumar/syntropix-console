"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTheme } from "next-themes";
import {
  Bot,
  Building2,
  Check,
  KeyRound,
  Link2,
  Moon,
  Palette,
  ShieldCheck,
  Sparkles,
  Sun,
  Terminal,
  Users,
} from "lucide-react";
import {
  createRbacUser,
  getAPIToken,
  isUnauthorizedError,
  listRbacAccess,
  listRbacUsers,
  llmStatus,
  projectAccessDecision,
  setAPIToken,
  upsertRbacAccess,
  workspaceBootstrap,
  workspaceListConnectors,
  workspaceListSecrets,
  workspaceUpsertConnector,
  workspaceUpsertSecret,
} from "@/lib/api";
import type {
  AccessDecision,
  ProjectAccess,
  RolePolicy,
  WorkspaceConnector,
  WorkspaceEnvironment,
  WorkspaceProject,
  WorkspaceSecret,
} from "@/lib/types";

type Section = "general" | "appearance" | "models" | "agents" | "members" | "security";

const menu = [
  { id: "general", label: "General", icon: Building2 },
  { id: "appearance", label: "Appearance", icon: Palette },
  { id: "models", label: "Models & API keys", icon: Sparkles },
  { id: "agents", label: "Agents & IDE", icon: Terminal },
  { id: "members", label: "Members & access", icon: Users },
  { id: "security", label: "Security", icon: ShieldCheck },
] as const;

const LLM_PROVIDERS = [
  {
    id: "openai",
    label: "OpenAI",
    keyName: "OPENAI_API_KEY",
    hint: "ChatGPT / GPT-4o family for in-app chat and issue_agent API route",
  },
  {
    id: "anthropic",
    label: "Anthropic",
    keyName: "ANTHROPIC_API_KEY",
    hint: "Claude models for chat and Claude agentic workflows",
  },
  {
    id: "google",
    label: "Google",
    keyName: "GOOGLE_API_KEY",
    hint: "Gemini models — also used with Antigravity-style agents",
  },
  {
    id: "cursor",
    label: "Cursor",
    keyName: "CURSOR_API_KEY",
    hint: "Cursor API / OpenAI-compatible endpoint for agentic IDE loops",
  },
] as const;

const IDE_AGENTS = [
  {
    id: "cursor",
    name: "Cursor",
    detail: "Connect Cursor via MCP for closed-loop fix & feature work",
    connectorType: "mcp",
    connectorName: "cursor",
    defaults: { ide: "cursor", transport: "stdio", tools: "aegisops" },
  },
  {
    id: "antigravity",
    name: "Antigravity",
    detail: "Google Antigravity agentic IDE bridge over MCP",
    connectorType: "mcp",
    connectorName: "antigravity",
    defaults: { ide: "antigravity", transport: "stdio", tools: "aegisops" },
  },
  {
    id: "claude",
    name: "Claude agentic",
    detail: "Claude Code / Claude agentic session — MCP or Anthropic API",
    connectorType: "issue_agent",
    connectorName: "claude-agentic",
    defaults: { ide: "claude", route: "api", provider: "anthropic", useGraph: true },
  },
  {
    id: "generic",
    name: "Generic IDE",
    detail: "Any MCP-capable IDE or agent runtime — no vendor lock-in",
    connectorType: "mcp",
    connectorName: "generic-ide",
    defaults: { ide: "generic", transport: "stdio", tools: "aegisops" },
  },
] as const;

export function SettingsPage() {
  const { resolvedTheme, setTheme } = useTheme();
  const [section, setSection] = useState<Section>("general");
  const [projects, setProjects] = useState<WorkspaceProject[]>([]);
  const [environments, setEnvironments] = useState<WorkspaceEnvironment[]>([]);
  const [projectId, setProjectId] = useState<number | null>(null);
  const [envId, setEnvId] = useState<number | null>(null);
  const [users, setUsers] = useState<{ id: number; email: string; displayName: string }[]>([]);
  const [access, setAccess] = useState<ProjectAccess[]>([]);
  const [decision, setDecision] = useState<AccessDecision | null>(null);
  const [policies, setPolicies] = useState<RolePolicy[]>([]);
  const [secrets, setSecrets] = useState<WorkspaceSecret[]>([]);
  const [connectors, setConnectors] = useState<WorkspaceConnector[]>([]);
  const [configuredProviders, setConfiguredProviders] = useState<string[]>([]);
  const [keyDrafts, setKeyDrafts] = useState<Record<string, string>>({});
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [grantUserId, setGrantUserId] = useState<number | "">("");
  const [role, setRole] = useState("developer");
  const [chatLimit, setChatLimit] = useState(50);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [apiTokenDraft, setApiTokenDraft] = useState("");
  const [apiTokenSaved, setApiTokenSaved] = useState(false);

  useEffect(() => {
    setApiTokenDraft(getAPIToken());
    setApiTokenSaved(Boolean(getAPIToken()));
  }, []);

  const projectEnvs = useMemo(
    () => environments.filter((environment) => environment.projectId === projectId).sort((a, b) => a.sortOrder - b.sortOrder),
    [environments, projectId],
  );

  const reload = useCallback(async () => {
    try {
      const [workspace, members] = await Promise.all([workspaceBootstrap(), listRbacUsers()]);
      const nextProjects = workspace.projects ?? [];
      setProjects(nextProjects);
      setEnvironments(workspace.environments ?? []);
      setUsers(members.users ?? []);
      setProjectId((current) => current ?? nextProjects[0]?.id ?? null);
      setError(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to load organization settings");
      // Token entry lives under Security and must stay reachable without bootstrap.
      if (isUnauthorizedError(cause) || !getAPIToken()) {
        setSection("security");
      }
    }
  }, []);

  const loadProjectAccess = useCallback(async (pid: number) => {
    try {
      const current = await projectAccessDecision(pid);
      setDecision(current);
      setPolicies(current.policies ?? []);
      if (current.canManageRBAC) {
        const grants = await listRbacAccess(pid);
        setAccess(grants.access ?? []);
        setPolicies(grants.policies ?? []);
      } else {
        setAccess([]);
      }
    } catch {
      setDecision(null);
      setAccess([]);
    }
  }, []);

  const loadEnvExtras = useCallback(async (id: number) => {
    try {
      const [secretData, connectorData, status] = await Promise.all([
        workspaceListSecrets(id),
        workspaceListConnectors(id),
        llmStatus(id),
      ]);
      setSecrets(secretData.secrets ?? []);
      setConnectors(connectorData.connectors ?? []);
      setConfiguredProviders(status.providers ?? []);
    } catch {
      setSecrets([]);
      setConnectors([]);
      setConfiguredProviders([]);
    }
  }, []);

  useEffect(() => void reload(), [reload]);

  useEffect(() => {
    if (!projectId) return;
    const available = environments.filter((e) => e.projectId === projectId);
    if (!available.some((e) => e.id === envId)) {
      setEnvId(available[0]?.id ?? null);
    }
    void loadProjectAccess(projectId);
  }, [projectId, environments, envId, loadProjectAccess]);

  useEffect(() => {
    if (!envId) return;
    void loadEnvExtras(envId);
  }, [envId, loadEnvExtras]);

  async function saveApiKey(provider: string, keyName: string) {
    if (!envId) return;
    const value = (keyDrafts[keyName] || "").trim();
    if (!value) {
      setError(`Paste a value for ${keyName} before saving`);
      return;
    }
    setBusy(true);
    setNotice(null);
    try {
      await workspaceUpsertSecret(envId, { provider, keyName, value });
      setKeyDrafts((current) => ({ ...current, [keyName]: "" }));
      setNotice(`${keyName} saved encrypted for this environment`);
      await loadEnvExtras(envId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not save API key");
    } finally {
      setBusy(false);
    }
  }

  async function toggleAgent(agent: (typeof IDE_AGENTS)[number], enabled: boolean) {
    if (!envId) return;
    setBusy(true);
    setNotice(null);
    try {
      await workspaceUpsertConnector(envId, {
        connectorType: agent.connectorType,
        name: agent.connectorName,
        enabled,
        config: { ...agent.defaults },
      });
      setNotice(`${agent.name} ${enabled ? "connected" : "disconnected"} — choose any agent freely`);
      await loadEnvExtras(envId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not update agent connection");
    } finally {
      setBusy(false);
    }
  }

  function agentConnected(agent: (typeof IDE_AGENTS)[number]) {
    return connectors.some(
      (connector) =>
        connector.connectorType === agent.connectorType &&
        connector.name === agent.connectorName &&
        connector.enabled,
    );
  }

  async function addUser() {
    if (!email || !name) return;
    setBusy(true);
    try {
      await createRbacUser({ email, name });
      setEmail("");
      setName("");
      await reload();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not add member");
    } finally {
      setBusy(false);
    }
  }

  async function grantAccess() {
    if (!projectId || grantUserId === "") return;
    setBusy(true);
    try {
      await upsertRbacAccess(projectId, {
        principalType: "user",
        principalId: Number(grantUserId),
        role,
        chatLimitDaily: chatLimit,
      });
      await reload();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Admin access is required");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-full bg-[var(--background)]">
      <aside className="w-52 shrink-0 border-r border-[var(--panel-border)] bg-[var(--sidebar-bg)] p-4">
        <h1 className="mb-5 px-2 text-sm font-semibold text-[var(--text-active)]">Organization settings</h1>
        <div className="space-y-1">
          {menu.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              type="button"
              onClick={() => setSection(id)}
              className="nav-item w-full"
              data-active={section === id}
            >
              <Icon size={15} />
              <span>{label}</span>
            </button>
          ))}
        </div>
      </aside>

      <main className="min-w-0 flex-1 px-6 py-8 lg:px-12">
        <div className="mx-auto max-w-3xl">
          {error && <div className="mb-4 rounded-xl border border-rose-500/25 bg-rose-500/10 px-4 py-3 text-sm text-rose-400">{error}</div>}
          {notice && <div className="mb-4 rounded-xl border border-emerald-500/25 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-500">{notice}</div>}

          {(section === "models" || section === "agents") && (
            <div className="mb-5 flex flex-wrap gap-2">
              <select
                className="config-input max-w-[220px]"
                value={projectId ?? ""}
                onChange={(e) => {
                  setProjectId(Number(e.target.value));
                  setEnvId(null);
                }}
              >
                {projects.map((project) => (
                  <option key={project.id} value={project.id}>
                    {project.name}
                  </option>
                ))}
              </select>
              <select className="config-input max-w-[160px]" value={envId ?? ""} onChange={(e) => setEnvId(Number(e.target.value))}>
                {projectEnvs.map((environment) => (
                  <option key={environment.id} value={environment.id}>
                    {environment.slug}
                  </option>
                ))}
              </select>
            </div>
          )}

          {section === "general" && (
            <SettingsSection
              title="General"
              subtitle="Organization identity and SaaS defaults shared by every project."
            >
              <SettingRow label="Organization" description="Tenant boundary for billing, members and audit logs">
                <div className="text-right">
                  <p className="text-sm font-medium text-[var(--text-active)]">Syntropix</p>
                  <p className="text-xs text-[var(--text-muted)]">syntropix</p>
                </div>
              </SettingRow>
              <SettingRow label="Projects" description="Project configuration lives inside each project workspace">
                <span className="text-sm font-medium text-[var(--text-active)]">{projects.length} active</span>
              </SettingRow>
              <SettingRow label="Default role" description="New members receive no project access until explicitly granted">
                <span className="status-pill status-pill-ok">Least privilege</span>
              </SettingRow>
            </SettingsSection>
          )}

          {section === "appearance" && (
            <SettingsSection title="Appearance" subtitle="Theme preferences apply across Chat, Projects and Settings.">
              <SettingRow label="Appearance" description="Follow your system or choose a fixed mode">
                <div className="flex rounded-lg border border-[var(--panel-border)] bg-[var(--background)] p-1">
                  {[
                    ["light", "Light", Sun],
                    ["dark", "Dark", Moon],
                  ].map(([id, label, Icon]) => (
                    <button
                      key={id as string}
                      type="button"
                      onClick={() => setTheme(id as string)}
                      className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs ${resolvedTheme === id ? "bg-[var(--surface)] text-[var(--text-active)] shadow-sm" : "text-[var(--text-muted)]"}`}
                    >
                      <Icon size={13} /> {label as string}
                    </button>
                  ))}
                </div>
              </SettingRow>
              <SettingRow label="Interface density" description="Compact project operations without decorative dashboard clutter">
                <span className="status-pill">Compact</span>
              </SettingRow>
            </SettingsSection>
          )}

          {section === "models" && (
            <div className="space-y-5">
              <SettingsSection
                title="Models & API keys"
                subtitle="Store chat LLM keys per environment. Keys are encrypted at rest and never returned in plaintext."
              >
                <div className="space-y-0">
                  {LLM_PROVIDERS.map((provider) => {
                    const existing = secrets.find((secret) => secret.keyName === provider.keyName);
                    const ready = configuredProviders.includes(provider.id);
                    return (
                      <div key={provider.id} className="border-b border-[var(--panel-border)] p-4 last:border-b-0">
                        <div className="mb-3 flex items-start justify-between gap-3">
                          <div>
                            <p className="text-sm font-medium text-[var(--text-active)]">{provider.label}</p>
                            <p className="mt-0.5 text-xs text-[var(--text-muted)]">{provider.hint}</p>
                            <p className="mt-1 font-mono text-[10px] text-[var(--text-muted)]">{provider.keyName}</p>
                          </div>
                          <span className={`status-pill ${ready ? "status-pill-ok" : ""}`}>
                            {ready ? "Configured" : "Not set"}
                          </span>
                        </div>
                        {existing && (
                          <p className="mb-2 text-[11px] text-[var(--text-muted)]">
                            Current · {existing.maskedValue} · updated {new Date(existing.updatedAt).toLocaleString()}
                          </p>
                        )}
                        <div className="flex flex-col gap-2 sm:flex-row">
                          <input
                            className="config-input flex-1"
                            type="password"
                            autoComplete="off"
                            placeholder={existing ? "Replace key (write-only)" : `Paste ${provider.keyName}`}
                            value={keyDrafts[provider.keyName] ?? ""}
                            onChange={(e) => setKeyDrafts((current) => ({ ...current, [provider.keyName]: e.target.value }))}
                          />
                          <button
                            type="button"
                            disabled={busy || !envId}
                            onClick={() => void saveApiKey(provider.id, provider.keyName)}
                            className="agent-chip justify-center"
                          >
                            <KeyRound size={13} /> Save key
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </SettingsSection>
              <p className="text-xs text-[var(--text-muted)]">
                Chat on the home page can use any provider with a key. There is no forced default agent — pick OpenAI, Claude, Google or Cursor freely.
              </p>
            </div>
          )}

          {section === "agents" && (
            <div className="space-y-5">
              <SettingsSection
                title="Agents & IDE connections"
                subtitle="Connect Cursor, Antigravity, Claude agentic, or any MCP IDE. Closed-loop develop → commit → PR uses whichever you enable — no restriction."
              >
                <div className="space-y-0">
                  {IDE_AGENTS.map((agent) => {
                    const connected = agentConnected(agent);
                    return (
                      <div key={agent.id} className="flex items-center justify-between gap-4 border-b border-[var(--panel-border)] p-4 last:border-b-0">
                        <div className="flex items-start gap-3">
                          <div className="mt-0.5 flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-500/10 text-indigo-400">
                            {agent.id === "claude" ? <Bot size={16} /> : <Link2 size={16} />}
                          </div>
                          <div>
                            <p className="text-sm font-medium text-[var(--text-active)]">{agent.name}</p>
                            <p className="mt-0.5 text-xs text-[var(--text-muted)]">{agent.detail}</p>
                            <p className="mt-1 text-[10px] uppercase tracking-wide text-[var(--text-muted)]">
                              {agent.connectorType} · {agent.connectorName}
                            </p>
                          </div>
                        </div>
                        <button
                          type="button"
                          disabled={busy || !envId}
                          onClick={() => void toggleAgent(agent, !connected)}
                          className={`agent-chip ${connected ? "border-emerald-500/40 text-emerald-500" : ""}`}
                        >
                          {connected ? (
                            <>
                              <Check size={13} /> Connected
                            </>
                          ) : (
                            "Connect"
                          )}
                        </button>
                      </div>
                    );
                  })}
                </div>
              </SettingsSection>
              <SettingsSection title="How the closed loop uses agents" subtitle="Detect → develop → commit → PR → CI/CD → deploy → monitor.">
                <div className="space-y-2 p-4 text-xs text-[var(--text-muted)]">
                  <p>1. Enable one or many IDEs above — users pick any agent with no lock-in.</p>
                  <p>2. Add LLM keys under Models & API keys for in-app chat and API-routed issue agents.</p>
                  <p>3. Point Cursor / Antigravity / Claude at the AegisOps MCP server (see docs/MCP.md).</p>
                  <p>4. Project workspace connectors keep the same loop healthy for each environment.</p>
                </div>
              </SettingsSection>
            </div>
          )}

          {section === "members" && (
            <div className="space-y-5">
              <SettingsSection title="Members & project access" subtitle="Invite people, then grant explicit access per project.">
                <div className="grid gap-2 p-4 sm:grid-cols-[1fr_1fr_auto]">
                  <input className="config-input" placeholder="name@company.com" value={email} onChange={(e) => setEmail(e.target.value)} />
                  <input className="config-input" placeholder="Display name" value={name} onChange={(e) => setName(e.target.value)} />
                  <button type="button" disabled={busy} onClick={() => void addUser()} className="agent-chip justify-center">
                    Invite
                  </button>
                </div>
              </SettingsSection>

              <SettingsSection title="Project grant" subtitle="Admin, maintainer, developer or read-only access with AI chat quota.">
                <div className="grid gap-2 p-4 md:grid-cols-4">
                  <select className="config-input" value={projectId ?? ""} onChange={(e) => setProjectId(Number(e.target.value))}>
                    {projects.map((project) => (
                      <option key={project.id} value={project.id}>
                        {project.name}
                      </option>
                    ))}
                  </select>
                  <select className="config-input" value={grantUserId} onChange={(e) => setGrantUserId(Number(e.target.value))}>
                    <option value="">Select member</option>
                    {users.map((user) => (
                      <option key={user.id} value={user.id}>
                        {user.displayName}
                      </option>
                    ))}
                  </select>
                  <select className="config-input" value={role} onChange={(e) => setRole(e.target.value)}>
                    {["admin", "maintainer", "developer", "readonly"].map((item) => (
                      <option key={item}>{item}</option>
                    ))}
                  </select>
                  <input className="config-input" type="number" value={chatLimit} onChange={(e) => setChatLimit(Number(e.target.value))} />
                </div>
                <div className="flex justify-end px-4 pb-4">
                  <button type="button" disabled={busy || !decision?.canManageRBAC} onClick={() => void grantAccess()} className="agent-chip">
                    Grant access
                  </button>
                </div>
                <div className="border-t border-[var(--panel-border)]">
                  {access.map((grant) => (
                    <div key={grant.id} className="flex items-center justify-between border-b border-[var(--panel-border)] px-4 py-3 last:border-b-0">
                      <div>
                        <p className="text-sm font-medium text-[var(--text-active)]">{grant.principalLabel || `Principal ${grant.principalId}`}</p>
                        <p className="text-xs text-[var(--text-muted)]">{grant.principalType}</p>
                      </div>
                      <p className="text-xs text-[var(--text-muted)]">
                        <strong className="text-[var(--text-active)]">{grant.role}</strong> · {grant.chatLimitDaily} chats/day
                      </p>
                    </div>
                  ))}
                </div>
              </SettingsSection>
            </div>
          )}

          {section === "security" && (
            <div className="space-y-5">
              <SettingsSection
                title="API access token"
                subtitle="Dashboard sends Authorization Bearer on every /api call. Match server AEGISOPS_API_TOKEN (Secrets Manager). Stored only in this browser."
              >
                <div className="space-y-3 p-4">
                  <input
                    type="password"
                    autoComplete="off"
                    value={apiTokenDraft}
                    onChange={(e) => setApiTokenDraft(e.target.value)}
                    placeholder="Paste AEGISOPS_API_TOKEN"
                    className="w-full rounded-md border border-[var(--panel-border)] bg-[var(--panel)] px-3 py-2 text-sm text-[var(--text-active)]"
                  />
                  <div className="flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => {
                        setAPIToken(apiTokenDraft);
                        setApiTokenSaved(Boolean(apiTokenDraft.trim()));
                        setNotice(apiTokenDraft.trim() ? "API token saved in this browser" : "API token cleared");
                        void reload();
                      }}
                      className="rounded-md bg-[var(--accent)] px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50"
                    >
                      Save token
                    </button>
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => {
                        setApiTokenDraft("");
                        setAPIToken("");
                        setApiTokenSaved(false);
                        setNotice("API token cleared");
                      }}
                      className="rounded-md border border-[var(--panel-border)] px-3 py-1.5 text-xs text-[var(--text-muted)]"
                    >
                      Clear
                    </button>
                    {apiTokenSaved ? (
                      <span className="flex items-center gap-1 text-xs text-emerald-500">
                        <Check size={13} /> Token present
                      </span>
                    ) : (
                      <span className="text-xs text-[var(--text-muted)]">No browser token — required when server auth is enforced</span>
                    )}
                  </div>
                </div>
              </SettingsSection>

              <SettingsSection title="Security & compliance" subtitle="Organization-wide controls. Project secrets remain in each project environment.">
                <SettingRow label="Secret encryption" description="AES-256-GCM with HKDF-derived key; plaintext is write-only">
                  <span className="flex items-center gap-1.5 text-xs font-medium text-emerald-500">
                    <Check size={13} /> Enforced
                  </span>
                </SettingRow>
                <SettingRow label="Audit events" description="RBAC, pipeline and secret mutations record actor and scope">
                  <span className="flex items-center gap-1.5 text-xs font-medium text-emerald-500">
                    <Check size={13} /> Active
                  </span>
                </SettingRow>
                <SettingRow label="Secret visibility" description="No role, including admin, can retrieve plaintext after save">
                  <span className="flex items-center gap-1.5 text-xs font-medium text-emerald-500">
                    <KeyRound size={13} /> Write only
                  </span>
                </SettingRow>
                <SettingRow label="HTTP API auth" description="Production requires AEGISOPS_API_TOKEN; X-User-* spoofing ignored when auth is on">
                  <span className="flex items-center gap-1.5 text-xs font-medium text-emerald-500">
                    <ShieldCheck size={13} /> Bearer gate
                  </span>
                </SettingRow>
              </SettingsSection>

              <SettingsSection title="Role policy" subtitle="Effective controls applied to every project grant.">
                {policies.map((policy) => (
                  <div key={policy.role} className="border-b border-[var(--panel-border)] p-4 last:border-b-0">
                    <p className="text-sm font-semibold capitalize text-[var(--text-active)]">{policy.label}</p>
                    <p className="mt-1 text-xs text-[var(--text-muted)]">{policy.description}</p>
                    <p className="mt-2 text-[11px] text-emerald-500">Allows · {policy.allows.join(" · ")}</p>
                    <p className="mt-1 text-[11px] text-rose-400">Blocks · {policy.denies.join(" · ")}</p>
                  </div>
                ))}
              </SettingsSection>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function SettingsSection({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <div className="mb-4">
        <h2 className="text-xl font-semibold tracking-tight text-[var(--text-active)]">{title}</h2>
        <p className="mt-1 text-sm text-[var(--text-muted)]">{subtitle}</p>
      </div>
      <div className="project-panel overflow-hidden">{children}</div>
    </section>
  );
}

function SettingRow({
  label,
  description,
  children,
}: {
  label: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-5 border-b border-[var(--panel-border)] p-4 last:border-b-0">
      <div>
        <p className="text-sm font-medium text-[var(--text-active)]">{label}</p>
        <p className="mt-0.5 text-xs text-[var(--text-muted)]">{description}</p>
      </div>
      {children}
    </div>
  );
}
