import type {
  DeploymentsListResponse,
  IssueDetailResponse,
  IssuesListResponse,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";
export const API_TOKEN_STORAGE_KEY = "aegisops_api_token";
/** Dispatched on window when any authenticated API call returns 401. */
export const API_UNAUTHORIZED_EVENT = "aegis:api-unauthorized";

export class ApiError extends Error {
  status: number;
  path: string;

  constructor(path: string, status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.path = path;
    this.status = status;
  }
}

export function isUnauthorizedError(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401;
}

function correlationHeaders(): HeadersInit {
  if (typeof window === "undefined") return {};
  const id = sessionStorage.getItem("aegisops_request_id");
  return id ? { "X-Request-Id": id } : {};
}

/** Bearer token for API auth — localStorage (Settings) wins over NEXT_PUBLIC bake-in. */
export function getAPIToken(): string {
  if (typeof window !== "undefined") {
    const stored = window.localStorage.getItem(API_TOKEN_STORAGE_KEY);
    if (stored && stored.trim()) return stored.trim();
  }
  return (process.env.NEXT_PUBLIC_AEGISOPS_API_TOKEN ?? "").trim();
}

export function setAPIToken(token: string): void {
  if (typeof window === "undefined") return;
  const t = token.trim();
  if (t) window.localStorage.setItem(API_TOKEN_STORAGE_KEY, t);
  else window.localStorage.removeItem(API_TOKEN_STORAGE_KEY);
  window.dispatchEvent(new CustomEvent("aegis:api-token-changed", { detail: { present: Boolean(t) } }));
}

function authHeaders(): HeadersInit {
  const token = getAPIToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

function notifyUnauthorized(path: string, message: string) {
  if (typeof window === "undefined") return;
  window.dispatchEvent(
    new CustomEvent(API_UNAUTHORIZED_EVENT, { detail: { path, message } }),
  );
}

async function apiErrorMessage(path: string, res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string; message?: string };
    const detail = body.error ?? body.message;
    if (detail) return detail;
  } catch {
    // ignore non-JSON bodies
  }
  return `API ${path} failed: ${res.status}`;
}

async function throwApiError(path: string, res: Response): Promise<never> {
  const message = await apiErrorMessage(path, res);
  if (res.status === 401) notifyUnauthorized(path, message);
  throw new ApiError(path, res.status, message);
}

export async function fetchAPI<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    cache: "no-store",
    ...init,
    headers: { ...correlationHeaders(), ...authHeaders(), ...(init?.headers ?? {}) },
  });
  if (!res.ok) await throwApiError(path, res);
  return res.json() as Promise<T>;
}

export async function fetchText(path: string): Promise<string> {
  const res = await fetch(`${API_BASE}${path}`, {
    cache: "no-store",
    headers: { ...correlationHeaders(), ...authHeaders() },
  });
  if (!res.ok) await throwApiError(path, res);
  return res.text();
}

/** Public — no bearer required. Used to detect whether staging enforces API token auth. */
export async function fetchVersion(): Promise<{
  auth?: { enforced?: boolean; mode?: string; tokenConfigured?: boolean };
  environment?: string;
  version?: string;
  service?: string;
}> {
  const res = await fetch(`${API_BASE}/api/v1/version`, { cache: "no-store" });
  if (!res.ok) throw new ApiError("/api/v1/version", res.status, await apiErrorMessage("/api/v1/version", res));
  return res.json();
}

export function listIssues(type?: string, status?: string) {
  const params = new URLSearchParams();
  if (type) params.set("type", type);
  if (status) params.set("status", status);
  const q = params.toString();
  return fetchAPI<IssuesListResponse>(`/api/v1/issues${q ? `?${q}` : ""}`);
}

export function getIssue(id: number) {
  return fetchAPI<IssueDetailResponse>(`/api/v1/issues/${id}`);
}

export function getIssueBundle(id: number) {
  return fetchText(`/api/v1/issues/${id}/bundle`);
}

export function startFix(id: number) {
  return fetchAPI<{ issue: IssuesListResponse["issues"][0]; message: string }>(
    `/api/v1/issues/${id}/start-fix`,
    { method: "POST" },
  );
}

export function openPR(id: number) {
  return fetchAPI<{
    issue: IssuesListResponse["issues"][0];
    pullRequest: { number: number; url: string; branch: string };
    message: string;
  }>(`/api/v1/issues/${id}/open-pr`, { method: "POST" });
}

export function gatewayApprove(id: number) {
  return fetchAPI<{ issue: IssuesListResponse["issues"][0]; action: string; message: string }>(
    `/api/v1/issues/${id}/gateway/approve`,
    { method: "POST" },
  );
}

export function gatewayReject(id: number) {
  return fetchAPI<{ issue: IssuesListResponse["issues"][0]; action: string; message: string }>(
    `/api/v1/issues/${id}/gateway/reject`,
    { method: "POST" },
  );
}

export function startFeature(id: number) {
  return fetchAPI<{ issue: IssuesListResponse["issues"][0]; message: string }>(
    `/api/v1/issues/${id}/start-feature`,
    { method: "POST" },
  );
}

export function createIssue(body: { title: string; description: string; issueType: string }) {
  return fetchAPI<{ issue: IssuesListResponse["issues"][0] }>("/api/v1/issues", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function fetchStats() {
  return fetchAPI<import("./types").StatsSnapshot>("/api/v1/stats");
}
export const getStats = fetchStats;

export async function triggerDogfoodError(): Promise<{ correlationId: string; status: number }> {
  const res = await fetch("/api/v1/debug/trigger-error", { method: "POST" });
  const body = (await res.json().catch(() => ({}))) as { correlationId?: string; requestId?: string };
  return { correlationId: body.correlationId || body.requestId || "req_dogfood", status: res.status };
}

export function listDeployments(limit?: number) {
  const q = limit ? `?limit=${limit}` : "";
  return fetchAPI<DeploymentsListResponse>(`/api/v1/deployments${q}`);
}

export function getDeploymentLogs(id: number) {
  return fetchText(`/api/v1/deployments/${id}/logs`);
}

export function workspaceBootstrap() {
  return fetchAPI<import("./types").WorkspaceBootstrap>("/api/v1/workspace/bootstrap");
}

export function workspaceListConnectors(envId: number) {
  return fetchAPI<{
    connectors: import("./types").WorkspaceConnector[];
    stages?: Record<string, import("./types").WorkspaceConnector>;
    types: string[];
  }>(`/api/v1/workspace/environments/${envId}/connectors`);
}

export function workspaceConnectorDefaults() {
  return fetchAPI<{
    defaults: Record<string, Record<string, unknown>>;
    note: string;
    types: string[];
  }>("/api/v1/workspace/connectors/defaults");
}

export function workspaceApplyConnectorDefaults(
  envId: number,
  body: { types?: string[]; enable?: boolean } = { enable: true },
) {
  return fetchAPI<{
    connectors: import("./types").WorkspaceConnector[];
    ready: number;
    total: number;
    message: string;
  }>(`/api/v1/workspace/environments/${envId}/connectors/apply-defaults`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function workspaceUpsertConnector(
  envId: number,
  body: { connectorType: string; name: string; enabled: boolean; config: Record<string, unknown> },
) {
  return fetchAPI<{ connector: import("./types").WorkspaceConnector }>(
    `/api/v1/workspace/environments/${envId}/connectors`,
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) },
  );
}

export function workspaceListSecrets(envId: number) {
  return fetchAPI<{ secrets: import("./types").WorkspaceSecret[]; providers: string[] }>(
    `/api/v1/workspace/environments/${envId}/secrets`,
  );
}

export function workspaceUpsertSecret(
  envId: number,
  body: { provider: string; keyName: string; value: string },
) {
  return fetchAPI<{ secret: import("./types").WorkspaceSecret }>(
    `/api/v1/workspace/environments/${envId}/secrets`,
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) },
  );
}

export function workspaceDeleteSecret(envId: number, secretId: number) {
  return fetchAPI<{ status: string }>(
    `/api/v1/workspace/environments/${envId}/secrets/${secretId}`,
    { method: "DELETE" },
  );
}

export function workspaceCreateProject(body: { slug: string; name: string; description?: string }) {
  return fetchAPI<{ project: import("./types").WorkspaceProject }>("/api/v1/workspace/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function workspaceCreateEnvironment(
  projectId: number,
  body: { slug: string; name: string; sortOrder: number },
) {
  return fetchAPI<{ environment: import("./types").WorkspaceEnvironment }>(
    `/api/v1/workspace/projects/${projectId}/environments`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
}

export function workspaceDeleteEnvironment(envId: number) {
  return fetchAPI<{ status: string }>(`/api/v1/workspace/environments/${envId}`, {
    method: "DELETE",
  });
}

export function llmStatus(envId: number) {
  return fetchAPI<{
    providers: string[];
    secretKeysPresent: string[];
    modes: { id: string; label: string; description: string }[];
    defaultRoute: string;
    graphNodeCount?: number;
    recommendedModels: Record<string, string>;
  }>(`/api/v1/workspace/environments/${envId}/llm/status`);
}

export function llmComplete(
  envId: number,
  body: { provider: string; model?: string; prompt: string; issueId?: number; projectId?: number; useGraph?: boolean },
) {
  return fetchAPI<{
    response: string;
    provider: string;
    model: string;
    usage: { promptTokens: number; completionTokens: number; graphNodesUsed: number };
    tokenSavingNote: string;
    graphAutoIngested?: number;
  }>(`/api/v1/workspace/environments/${envId}/llm/complete`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function graphSnapshot(envId: number) {
  return fetchAPI<{
    nodes: import("./types").GraphNode[];
    edges: import("./types").GraphEdge[];
    stats: { nodeCount: number; edgeCount: number; tokensEstimate: number };
  }>(`/api/v1/workspace/environments/${envId}/graph`);
}

export function graphIngest(envId: number, projectId?: number) {
  const qs = projectId && projectId > 0 ? `?projectId=${projectId}` : "";
  return fetchAPI<{
    ingested: number;
    note: string;
    graph?: {
      nodes: import("./types").GraphNode[];
      edges: import("./types").GraphEdge[];
      stats: { nodeCount: number; edgeCount: number; tokensEstimate: number };
    };
  }>(`/api/v1/workspace/environments/${envId}/graph/ingest${qs}`, { method: "POST" });
}

export function graphUpsertNode(
  envId: number,
  body: { kind: string; nodeKey: string; title: string; summary: string },
) {
  return fetchAPI<{ node: import("./types").GraphNode }>(
    `/api/v1/workspace/environments/${envId}/graph/nodes`,
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) },
  );
}

export function llmRuns(envId: number) {
  return fetchAPI<{ runs: import("./types").LLMRun[] }>(
    `/api/v1/workspace/environments/${envId}/llm/runs`,
  );
}

export function projectAccessDecision(projectId: number) {
  return fetchAPI<import("./types").AccessDecision>(`/api/v1/rbac/projects/${projectId}/decision`);
}

export function listRbacUsers() {
  return fetchAPI<{ users: { id: number; email: string; displayName: string }[] }>("/api/v1/rbac/users");
}

export function createRbacUser(body: { email: string; name: string }) {
  return fetchAPI<{ user: { id: number; email: string; displayName: string } }>("/api/v1/rbac/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function listRbacAccess(projectId: number) {
  return fetchAPI<{ access: import("./types").ProjectAccess[]; policies: import("./types").RolePolicy[] }>(
    `/api/v1/rbac/projects/${projectId}/access`,
  );
}

export function upsertRbacAccess(
  projectId: number,
  body: { principalType: string; principalId: number; role: string; chatLimitDaily: number },
) {
  return fetchAPI<{ access: import("./types").ProjectAccess }>(`/api/v1/rbac/projects/${projectId}/access`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getPipeline(envId: number) {
  return fetchAPI<{ pipeline: { id: number; name: string; definition: Record<string, unknown> }; editPolicy: string }>(
    `/api/v1/workspace/environments/${envId}/pipeline`,
  );
}

export function upsertPipeline(
  envId: number,
  body: { projectId: number; name: string; definition: Record<string, unknown> },
) {
  return fetchAPI<{ pipeline: { id: number; name: string; definition: Record<string, unknown> } }>(
    `/api/v1/workspace/environments/${envId}/pipeline`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
}

export function upsertEnvVariable(envId: number, body: { keyName: string; valueText: string; projectId: number }) {
  return fetchAPI<{ variable: { id: number; keyName: string; valueText: string } }>(
    `/api/v1/workspace/environments/${envId}/variables`,
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) },
  );
}

export function listEnvVariables(envId: number) {
  return fetchAPI<{ variables: { id: number; keyName: string; valueText: string }[] }>(
    `/api/v1/workspace/environments/${envId}/variables`,
  );
}

export function bulkUpsertSecrets(
  envId: number,
  secrets: { provider: string; keyName: string; value: string }[],
) {
  return Promise.all(secrets.map((s) => workspaceUpsertSecret(envId, s)));
}
