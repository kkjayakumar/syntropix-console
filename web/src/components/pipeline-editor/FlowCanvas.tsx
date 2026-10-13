"use client";

import { useCallback, useMemo } from "react";
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  addEdge,
  Connection,
  Edge,
  BackgroundVariant,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  TerminalSquare,
  GitBranch,
  Activity,
  GitMerge,
  BrainCircuit,
  ShieldCheck,
  UserCheck,
} from "lucide-react";

import { PillarNode } from "./PillarNode";

/** Syntropix seven pillars — aligned with docs/PRODUCTION_READY.md */
const initialNodes = [
  {
    id: "p3",
    type: "pillar",
    position: { x: 40, y: 180 },
    data: {
      id: "P3",
      label: "Telemetry",
      description: "Spotting frontend capture + DevOps Copilot Sentry ingest.",
      status: "completed",
      icon: <Activity size={20} />,
    },
  },
  {
    id: "p4",
    type: "pillar",
    position: { x: 340, y: 180 },
    data: {
      id: "P4",
      label: "Router & Dedup",
      description: "Request-ID correlation and fingerprint deduplication.",
      status: "completed",
      icon: <GitMerge size={20} />,
    },
  },
  {
    id: "p5",
    type: "pillar",
    position: { x: 640, y: 180 },
    data: {
      id: "P5",
      label: "Issue Broker",
      description: "Correlates telemetry into fix/feat issues + GitHub.",
      status: "completed",
      icon: <BrainCircuit size={20} />,
    },
  },
  {
    id: "p7",
    type: "pillar",
    position: { x: 940, y: 80 },
    data: {
      id: "P7",
      label: "Gateway (HITL)",
      description: "Teams/Slack approve or reject before IDE fix.",
      status: "completed",
      icon: <UserCheck size={20} />,
    },
  },
  {
    id: "p1",
    type: "pillar",
    position: { x: 940, y: 280 },
    data: {
      id: "P1",
      label: "Smart IDE + MCP",
      description: "Cursor agent reads bundle via MCP and opens fix PR.",
      status: "completed",
      icon: <TerminalSquare size={20} />,
    },
  },
  {
    id: "p2",
    type: "pillar",
    position: { x: 1240, y: 180 },
    data: {
      id: "P2",
      label: "CI/CD Actions",
      description: "GitHub Actions test, GHCR publish, workflow webhooks.",
      status: "in-progress",
      icon: <GitBranch size={20} />,
    },
  },
  {
    id: "p6",
    type: "pillar",
    position: { x: 1540, y: 180 },
    data: {
      id: "P6",
      label: "Sentinel",
      description: "Post-deploy health check, rollback dispatch, temp monitors.",
      status: "completed",
      icon: <ShieldCheck size={20} />,
    },
  },
];

const initialEdges: Edge[] = [
  { id: "e3-4", source: "p3", target: "p4", animated: true, style: { stroke: "#0ea5e9", strokeWidth: 2 } },
  { id: "e4-5", source: "p4", target: "p5", animated: true, style: { stroke: "#8b5cf6", strokeWidth: 2 } },
  { id: "e5-7", source: "p5", target: "p7", animated: true, style: { stroke: "#f472b6", strokeWidth: 2 } },
  { id: "e5-1", source: "p5", target: "p1", animated: true, style: { stroke: "#64748b", strokeWidth: 2 } },
  { id: "e7-2", source: "p7", target: "p2", animated: true, style: { stroke: "#0ea5e9", strokeWidth: 2 } },
  { id: "e1-2", source: "p1", target: "p2", animated: true, style: { stroke: "#8b5cf6", strokeWidth: 2 } },
  { id: "e2-6", source: "p2", target: "p6", animated: true, style: { stroke: "#10b981", strokeWidth: 2 } },
];

export function FlowCanvas() {
  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>(initialEdges);

  const onConnect = useCallback(
    (params: Connection) =>
      setEdges((eds) =>
        addEdge({ ...params, animated: true, style: { stroke: "#6366f1" } }, eds),
      ),
    [setEdges],
  );

  const nodeTypes = useMemo(() => ({ pillar: PillarNode }), []);

  return (
    <div style={{ width: "100%", height: "100%" }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        nodeTypes={nodeTypes}
        fitView
        minZoom={0.4}
        className="bg-[var(--background)]"
      >
        <Controls className="border border-[var(--panel-border)] bg-[var(--glass-bg)] fill-[var(--text-active)]" />
        <MiniMap
          nodeColor={(n) => {
            if (n.data?.status === "completed") return "#10b981";
            if (n.data?.status === "in-progress") return "#f59e0b";
            return "#f43f5e";
          }}
          maskColor="rgba(0, 0, 0, 0.2)"
          className="border border-[var(--panel-border)] bg-[var(--glass-bg)]"
        />
        <Background variant={BackgroundVariant.Dots} gap={16} size={1} color="var(--text-muted)" />
      </ReactFlow>
    </div>
  );
}
