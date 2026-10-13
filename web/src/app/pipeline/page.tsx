"use client";

import { Workflow } from "lucide-react";
import { Shell } from "@/components/Shell";
import { FlowCanvas } from "@/components/pipeline-editor/FlowCanvas";

export default function PipelinePage() {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <Shell
        icon={Workflow}
        eyebrow="Seven pillars"
        title="Architecture"
        subtitle="Detect, correlate, approve, fix, deploy, verify"
        noPadding
      >
        <div className="glass-panel mx-6 mb-8 overflow-hidden md:mx-10" style={{ height: "calc(100vh - 11rem)" }}>
          <FlowCanvas />
        </div>
      </Shell>
    </div>
  );
}
