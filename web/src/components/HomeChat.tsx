"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowUp,
  AtSign,
  Bot,
  ChevronDown,
  FolderKanban,
  GitBranch,
  KeyRound,
  Loader2,
  Network,
  Paperclip,
  Plus,
  ShieldCheck,
  Sparkles,
  Workflow,
} from "lucide-react";
import {
  ApiError,
  graphIngest,
  isUnauthorizedError,
  llmComplete,
  llmStatus,
  projectAccessDecision,
  workspaceBootstrap,
} from "@/lib/api";
import type { AccessDecision, WorkspaceEnvironment, WorkspaceProject } from "@/lib/types";

type ChatMessage = { role: "user" | "assistant"; content: string; meta?: string };

const PROJECT_KEY = "aegis_workspace_project_id";
const ENV_KEY = "aegis_workspace_env_id";
const AGENT_KEY = "aegis_preferred_agent";

const ALL_PROVIDERS = ["openai", "anthropic", "google", "cursor"] as const;

const suggestions = [
  { label: "Investigate open incidents", prompt: "Investigate the open incidents and rank them by impact." },
  { label: "Review CI failures", prompt: "Review recent CI failures and suggest the safest fix." },
  { label: "Plan a new feature", prompt: "Help me turn a new feature requirement into an implementation plan." },
  { label: "Check deployment health", prompt: "Check deployment and telemetry health for this environment." },
];

function greeting() {
  const hour = new Date().getHours();
  if (hour < 12) return "Good morning";
  if (hour < 18) return "Good afternoon";
  return "Good evening";
}

function formatChatError(cause: unknown): string {
  if (isUnauthorizedError(cause)) {
    return "API token rejected. Paste AEGISOPS_API_TOKEN under Organization settings.";
  }
  if (cause instanceof ApiError) {
    if (cause.status === 403) return cause.message || "Chat not allowed for your role or daily quota exceeded.";
    if (cause.status === 400) return cause.message;
    if (cause.status === 502) return cause.message || "Provider request failed — check the API key and model.";
    return cause.message;
  }
  return cause instanceof Error ? cause.message : "AI request failed";
}

export function HomeChat() {
  const router = useRouter();
  const [projects, setProjects] = useState<WorkspaceProject[]>([]);
  const [environments, setEnvironments] = useState<WorkspaceEnvironment[]>([]);
  const [projectId, setProjectId] = useState<number | null>(null);
  const [envId, setEnvId] = useState<number | null>(null);
  const [providers, setProviders] = useState<string[]>([]);
  const [models, setModels] = useState<Record<string, string>>({});
  const [provider, setProvider] = useState("openai");
  const [model, setModel] = useState("gpt-4o-mini");
  const [access, setAccess] = useState<AccessDecision | null>(null);
  const [accessError, setAccessError] = useState<string | null>(null);
  const [graphNodeCount, setGraphNodeCount] = useState(0);
  const [statusLoading, setStatusLoading] = useState(false);
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const empty = messages.length === 0;

  const activeProject = projects.find((p) => p.id === projectId);
  const projectEnvs = environments.filter((e) => e.projectId === projectId);
  const activeEnv = projectEnvs.find((e) => e.id === envId);
  const providerOptions = ALL_PROVIDERS;

  const load = useCallback(async () => {
    try {
      const data = await workspaceBootstrap();
      const nextProjects = data.projects ?? [];
      const nextEnvironments = data.environments ?? [];
      setProjects(nextProjects);
      setEnvironments(nextEnvironments);
      const savedProject = Number(localStorage.getItem(PROJECT_KEY) || 0);
      const pid = nextProjects.some((p) => p.id === savedProject) ? savedProject : nextProjects[0]?.id ?? null;
      setProjectId(pid);
      const available = nextEnvironments.filter((e) => e.projectId === pid);
      const savedEnv = Number(localStorage.getItem(ENV_KEY) || 0);
      setEnvId(
        available.some((e) => e.id === savedEnv)
          ? savedEnv
          : available.find((e) => e.slug === "dev")?.id ?? available[0]?.id ?? null,
      );
      const preferred = localStorage.getItem(AGENT_KEY);
      if (preferred && ALL_PROVIDERS.includes(preferred as (typeof ALL_PROVIDERS)[number])) {
        setProvider(preferred);
      }
      setError(null);
    } catch (cause) {
      setError(formatChatError(cause) || (cause instanceof Error ? cause.message : "Unable to load workspace"));
    }
  }, []);

  useEffect(() => void load(), [load]);
  useEffect(() => endRef.current?.scrollIntoView({ behavior: "smooth" }), [messages, busy]);

  useEffect(() => {
    const onProject = (event: Event) => setProjectId((event as CustomEvent<number>).detail);
    const onEnv = (event: Event) => setEnvId((event as CustomEvent<number>).detail);
    const onNewSession = () => {
      setMessages([]);
      setInput("");
      setError(null);
      setBusy(false);
    };
    window.addEventListener("aegis:project-change", onProject);
    window.addEventListener("aegis:env-change", onEnv);
    window.addEventListener("aegis:new-chat-session", onNewSession);
    return () => {
      window.removeEventListener("aegis:project-change", onProject);
      window.removeEventListener("aegis:env-change", onEnv);
      window.removeEventListener("aegis:new-chat-session", onNewSession);
    };
  }, []);

  useEffect(() => {
    if (!projectId) return;
    localStorage.setItem(PROJECT_KEY, String(projectId));
    const available = environments.filter((e) => e.projectId === projectId);
    if (!available.some((e) => e.id === envId)) {
      setEnvId(available.find((e) => e.slug === "dev")?.id ?? available[0]?.id ?? null);
    }
    projectAccessDecision(projectId)
      .then((dec) => {
        setAccess(dec);
        setAccessError(null);
      })
      .catch((cause) => {
        setAccess(null);
        setAccessError(formatChatError(cause));
      });
  }, [projectId, environments, envId]);

  useEffect(() => {
    if (!envId) return;
    localStorage.setItem(ENV_KEY, String(envId));
    let cancelled = false;
    setStatusLoading(true);
    llmStatus(envId)
      .then((status) => {
        if (cancelled) return;
        const available = status.providers ?? [];
        const recommended = status.recommendedModels ?? {};
        setProviders(available);
        setModels(recommended);
        setGraphNodeCount(status.graphNodeCount ?? 0);
        const preferred = localStorage.getItem(AGENT_KEY) || provider;
        const next = available.includes(preferred) ? preferred : available[0] ?? preferred ?? "openai";
        setProvider(next);
        setModel(recommended[next] || recommended.openai || "gpt-4o-mini");
        setError(null);
      })
      .catch((cause) => {
        if (cancelled) return;
        setProviders([]);
        setGraphNodeCount(0);
        setError(formatChatError(cause));
      })
      .finally(() => {
        if (!cancelled) setStatusLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [envId]); // eslint-disable-line react-hooks/exhaustive-deps

  const canSend = Boolean(input.trim() && projectId && envId && !busy);
  const scopeLabel = useMemo(
    () => `${activeProject?.name ?? "Select project"} / ${activeEnv?.slug ?? "env"}`,
    [activeProject, activeEnv],
  );
  const hasKey = providers.includes(provider);
  const canChat = access ? access.canChat : !accessError;

  function selectProject(id: number) {
    setProjectId(id);
    localStorage.setItem(PROJECT_KEY, String(id));
    window.dispatchEvent(new CustomEvent("aegis:project-change", { detail: id }));
  }

  function selectEnv(id: number) {
    setEnvId(id);
    localStorage.setItem(ENV_KEY, String(id));
    window.dispatchEvent(new CustomEvent("aegis:env-change", { detail: id }));
  }

  async function ensureGraph(targetEnv: number) {
    if (graphNodeCount > 0) return;
    try {
      const res = await graphIngest(targetEnv, projectId ?? undefined);
      setGraphNodeCount(res.graph?.stats?.nodeCount ?? res.ingested ?? 0);
    } catch {
      // Backend Complete also auto-ingests; surface a soft hint only.
      setError((prev) => prev ?? "Graph was empty — ingest will retry with the chat request.");
    }
  }

  async function submit(event?: FormEvent) {
    event?.preventDefault();
    const prompt = input.trim();
    if (!prompt || !envId || !projectId || busy) return;
    if (!hasKey) {
      setError(`No API key for ${provider}. Add ${provider.toUpperCase()}_API_KEY under Organization settings → Models & API keys (env: ${activeEnv?.slug ?? envId}).`);
      return;
    }
    if (accessError) {
      setError(accessError);
      return;
    }
    if (access && !access.canChat) {
      setError(
        access.chatUsedToday >= access.chatLimitDaily
          ? `Daily chat quota exhausted (${access.chatUsedToday}/${access.chatLimitDaily}).`
          : "Your project role does not allow AI chat.",
      );
      return;
    }
    setError(null);
    setInput("");
    setMessages((current) => [...current, { role: "user", content: prompt }]);
    setBusy(true);
    try {
      await ensureGraph(envId);
      const result = await llmComplete(envId, {
        provider,
        model: model || models[provider] || undefined,
        prompt,
        projectId,
        useGraph: true,
      });
      const note = result.tokenSavingNote ? ` · ${result.tokenSavingNote}` : "";
      setMessages((current) => [
        ...current,
        {
          role: "assistant",
          content: result.response,
          meta: `${result.provider}/${result.model} · ${result.usage.promptTokens + result.usage.completionTokens} tokens · ${result.usage.graphNodesUsed} graph nodes${note}`,
        },
      ]);
      if ((result.usage.graphNodesUsed ?? 0) > 0) {
        setGraphNodeCount((n) => Math.max(n, result.usage.graphNodesUsed));
      }
    } catch (cause) {
      setError(formatChatError(cause));
    } finally {
      setBusy(false);
    }
  }

  function chooseSuggestion(prompt: string) {
    setInput(prompt);
  }

  function chooseProvider(next: string) {
    setProvider(next);
    localStorage.setItem(AGENT_KEY, next);
    setModel(models[next] || "");
    setError(null);
  }

  const scopeControls = (
    <>
      <label className="agent-select">
        <FolderKanban size={13} />
        <select value={projectId ?? ""} onChange={(e) => selectProject(Number(e.target.value))}>
          {!projects.length && <option value="">No projects</option>}
          {projects.map((project) => (
            <option key={project.id} value={project.id}>
              {project.name}
            </option>
          ))}
        </select>
        <ChevronDown size={12} />
      </label>

      <label className="agent-select">
        <GitBranch size={13} />
        <select value={envId ?? ""} onChange={(e) => selectEnv(Number(e.target.value))}>
          {!projectEnvs.length && <option value="">No env</option>}
          {projectEnvs.map((environment) => (
            <option key={environment.id} value={environment.id}>
              {environment.slug}
            </option>
          ))}
        </select>
        <ChevronDown size={12} />
      </label>

      <label className="agent-select" title="Choose any chat model — no agent lock-in">
        <Bot size={13} />
        <select value={provider} onChange={(e) => chooseProvider(e.target.value)}>
          {providerOptions.map((item) => (
            <option key={item} value={item}>
              {item}
              {providers.includes(item) ? "" : " · needs key"}
            </option>
          ))}
        </select>
        <ChevronDown size={12} />
      </label>
    </>
  );

  const statusHints = (
    <>
      {error && <p className="mt-3 text-center text-xs text-rose-400">{error}</p>}
      {!error && accessError && <p className="mt-3 text-center text-xs text-rose-400">{accessError}</p>}
      {!error && !accessError && !hasKey && !statusLoading && (
        <p className="mt-3 flex items-center justify-center gap-1.5 text-center text-xs text-[var(--text-muted)]">
          <KeyRound size={12} />
          Add a {provider} key in{" "}
          <button type="button" className="text-indigo-400 hover:underline" onClick={() => router.push("/settings")}>
            Organization settings → Models & API keys
          </button>
          {activeEnv ? ` for ${activeEnv.slug}` : ""}
        </p>
      )}
      {!error && !accessError && hasKey && graphNodeCount === 0 && !statusLoading && (
        <p className="mt-3 flex items-center justify-center gap-1.5 text-center text-xs text-[var(--text-muted)]">
          <Network size={12} />
          Context graph empty — first message will auto-ingest compact project nodes to cut token usage.
        </p>
      )}
    </>
  );

  return (
    <div className="agent-sky relative flex h-full min-h-0 flex-1 flex-col overflow-hidden">
      <div className="pointer-events-none absolute inset-0 opacity-60" aria-hidden />

      {!empty && (
        <div className="relative z-10 flex h-14 items-center justify-between border-b border-[var(--panel-border)] bg-[var(--surface)]/80 px-5 backdrop-blur-xl">
          <div>
            <p className="text-sm font-medium text-[var(--text-active)]">{scopeLabel}</p>
            <p className="text-[10px] text-[var(--text-muted)]">
              Graph-assisted · {graphNodeCount} nodes · scoped to {scopeLabel} only
              {statusLoading ? " · refreshing…" : ""}
            </p>
          </div>
          <button
            type="button"
            className="agent-chip"
            onClick={() => window.dispatchEvent(new Event("aegis:new-chat-session"))}
          >
            <Plus size={13} /> New session
          </button>
        </div>
      )}

      <div className={`relative z-10 flex-1 overflow-y-auto ${empty ? "flex items-center" : ""}`}>
        {empty ? (
          <div className="mx-auto w-full max-w-[760px] px-6 pb-16 pt-10">
            <div className="mb-9 text-center">
              <div className="mx-auto mb-5 flex h-12 w-12 items-center justify-center rounded-2xl border border-indigo-400/20 bg-indigo-500/10 text-indigo-400 shadow-2xl shadow-indigo-500/10">
                <Sparkles size={22} />
              </div>
              <h1 className="text-4xl font-medium tracking-[-0.04em] text-[var(--text-active)] md:text-5xl">
                {greeting()}, <span className="text-indigo-400">builder.</span>
              </h1>
              <p className="mt-3 text-sm text-[var(--text-muted)]">What should we build, fix, or investigate?</p>
              <p className="mt-2 flex items-center justify-center gap-1.5 text-[11px] text-[var(--text-muted)]">
                <ShieldCheck size={12} className="text-emerald-500/80" />
                Context graph & chat stay isolated to the selected project and environment (compliance).
              </p>
            </div>

            <form onSubmit={submit} className="agent-composer">
              <textarea
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    void submit();
                  }
                }}
                rows={3}
                placeholder="Ask Aegis about your application…"
                className="w-full resize-none bg-transparent px-5 pt-5 text-[15px] text-[var(--text-active)] outline-none placeholder:text-[var(--text-muted)]"
              />
              <div className="flex flex-wrap items-center gap-2 border-t border-[var(--panel-border)] px-3 py-3">
                <button type="button" className="agent-icon-button" aria-label="Attach context">
                  <Paperclip size={15} />
                </button>
                <button type="button" className="agent-icon-button" aria-label="Mention resource">
                  <AtSign size={15} />
                </button>

                {scopeControls}

                <div className="ml-auto flex items-center gap-2">
                  <button type="submit" disabled={!canSend || !canChat} className="agent-send" aria-label="Send message">
                    <ArrowUp size={16} />
                  </button>
                </div>
              </div>
            </form>

            {statusHints}

            <div className="mt-5 flex flex-wrap justify-center gap-2">
              {suggestions.map((suggestion) => (
                <button
                  key={suggestion.label}
                  type="button"
                  onClick={() => chooseSuggestion(suggestion.prompt)}
                  className="agent-chip"
                >
                  <Workflow size={12} /> {suggestion.label}
                </button>
              ))}
            </div>

            <div className="mt-14 flex flex-wrap items-center justify-center gap-5 text-[11px] text-[var(--text-muted)]">
              <span className="flex items-center gap-1.5">
                <ShieldCheck size={13} className="text-emerald-500" /> Project-scoped RBAC
              </span>
              <span>Graph context enabled</span>
              <button type="button" onClick={() => router.push("/workspace")} className="hover:text-indigo-400">
                Configure project →
              </button>
            </div>
          </div>
        ) : (
          <div className="mx-auto max-w-[820px] space-y-5 px-6 py-8">
            {messages.map((message, index) => (
              <div key={`${message.role}-${index}`} className={message.role === "user" ? "flex justify-end" : "flex gap-3"}>
                {message.role === "assistant" && (
                  <div className="mt-1 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-indigo-500/15 text-indigo-400">
                    <Bot size={14} />
                  </div>
                )}
                <div className={message.role === "user" ? "chat-user" : "chat-assistant"}>
                  <p className="whitespace-pre-wrap">{message.content}</p>
                  {message.meta && <p className="mt-2 text-[10px] text-[var(--text-muted)]">{message.meta}</p>}
                </div>
              </div>
            ))}
            {busy && (
              <div className="flex items-center gap-2 text-sm text-[var(--text-muted)]">
                <Loader2 size={15} className="animate-spin text-indigo-400" /> Aegis is reasoning with project context…
              </div>
            )}
            <div ref={endRef} />
          </div>
        )}
      </div>

      {!empty && (
        <div className="relative z-10 border-t border-[var(--panel-border)] bg-[var(--surface)]/90 p-4 backdrop-blur-xl">
          {error && <p className="mx-auto mb-2 max-w-[820px] text-xs text-rose-400">{error}</p>}
          <form onSubmit={submit} className="agent-composer mx-auto max-w-[820px]">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  void submit();
                }
              }}
              rows={2}
              placeholder={`Message Aegis · ${scopeLabel}`}
              className="min-h-12 w-full resize-none bg-transparent px-3 pt-3 text-sm text-[var(--text-active)] outline-none"
            />
            <div className="flex flex-wrap items-center gap-2 border-t border-[var(--panel-border)] px-2 py-2">
              {scopeControls}
              <button type="submit" disabled={!canSend || !canChat} className="agent-send ml-auto" aria-label="Send message">
                <ArrowUp size={16} />
              </button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
