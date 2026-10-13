export type AegisIssue = {
  id: number;
  githubIssueNumber?: number;
  githubRepo: string;
  title: string;
  issueType: string;
  description?: string;
  status: string;
  correlationId: string;
  telemetryBundle?: string;
  createdAt: string;
  updatedAt: string;
};

export type TelemetryLink = {
  id: number;
  aegisIssueId: number;
  spottingReportId?: string;
  sentryIssueId?: string;
};

export type Deployment = {
  id: number;
  githubRepo: string;
  runId: string;
  workflowName: string;
  commitSha: string;
  branch: string;
  status: string;
  failureLogs?: string;
  createdAt: string;
};

export type IssuesListResponse = { issues: AegisIssue[]; total: number };
export type IssueDetailResponse = {
  issue: AegisIssue;
  telemetryLinks: TelemetryLink[];
  recentDeployments?: Deployment[];
  fixBranch?: string;
};
export type DeploymentsListResponse = { deployments: Deployment[]; total: number };

export type StatsSnapshot = {
  issuesTotal: number;
  issuesByStatus: Record<string, number>;
  issuesFixCount: number;
  issuesFeatCount: number;
  deploymentsTotal: number;
  deployFailures: number;
  openIncidents: number;
};

export type WorkspaceTenant = {
  id: number;
  slug: string;
  name: string;
  createdAt: string;
};

export type WorkspaceProject = {
  id: number;
  tenantId: number;
  slug: string;
  name: string;
  description?: string;
  createdAt: string;
};

export type WorkspaceEnvironment = {
  id: number;
  projectId: number;
  slug: string;
  name: string;
  sortOrder: number;
  createdAt: string;
};

export type WorkspaceConnector = {
  id: number;
  environmentId: number;
  connectorType: string;
  name: string;
  enabled: boolean;
  ready?: boolean;
  config: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
};

export type WorkspaceSecret = {
  id: number;
  environmentId: number;
  provider: string;
  keyName: string;
  maskedValue: string;
  createdAt: string;
  updatedAt: string;
};

export type WorkspaceBootstrap = {
  tenant: WorkspaceTenant;
  projects: WorkspaceProject[];
  environments: WorkspaceEnvironment[];
};

export type GraphNode = {
  id: number;
  environmentId: number;
  kind: string;
  nodeKey: string;
  title: string;
  summary: string;
  tokensEstimate: number;
  updatedAt: string;
};

export type GraphEdge = {
  id: number;
  environmentId: number;
  fromNodeId: number;
  toNodeId: number;
  relation: string;
};

export type LLMRun = {
  id: number;
  environmentId: number;
  issueId?: number;
  provider: string;
  model: string;
  promptTokens: number;
  completionTokens: number;
  graphNodesUsed: number;
  status: string;
  responseSummary: string;
  createdAt: string;
};

export type RolePolicy = {
  role: string;
  label: string;
  description: string;
  allows: string[];
  denies: string[];
};

export type AccessDecision = {
  userId: number;
  projectId: number;
  role: string;
  chatLimitDaily: number;
  chatUsedToday: number;
  canChat: boolean;
  canWriteSecret: boolean;
  canEditPipeline: boolean;
  canManageRBAC: boolean;
  canReadSecretValue: boolean;
  policies: RolePolicy[];
};

export type ProjectAccess = {
  id: number;
  projectId: number;
  principalType: string;
  principalId: number;
  principalLabel?: string;
  role: string;
  chatLimitDaily: number;
};
