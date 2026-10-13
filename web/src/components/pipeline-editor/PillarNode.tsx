import type { ReactNode } from "react";
import { Handle, Position } from "@xyflow/react";
import { CheckCircle2, Circle, Clock } from "lucide-react";
import { motion } from "framer-motion";

const statusStyles: Record<string, { border: string; glow: string; icon: ReactNode }> = {
  completed: {
    border: "border-l-emerald-400",
    glow: "shadow-emerald-500/20",
    icon: <CheckCircle2 className="text-emerald-500" size={16} />,
  },
  "in-progress": {
    border: "border-l-amber-400",
    glow: "shadow-amber-500/20",
    icon: <Clock className="text-amber-500" size={16} />,
  },
  pending: {
    border: "border-l-rose-400",
    glow: "shadow-rose-500/20",
    icon: <Circle className="text-rose-500" size={16} />,
  },
};

export function PillarNode({ data }: { data: { icon?: ReactNode; label: string; description: string; id: string; status: string } }) {
  const style = statusStyles[data.status] ?? statusStyles.pending;

  return (
    <motion.div
      whileHover={{ scale: 1.02, y: -2 }}
      transition={{ type: "spring", stiffness: 400, damping: 25 }}
      className={`glass-strong w-64 rounded-2xl border-l-4 p-4 ${style.border} shadow-lg ${style.glow}`}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!h-3 !w-3 !border-2 !border-[var(--background)] !bg-sky-500"
      />

      <div className="mb-2 flex items-start justify-between">
        <div className="flex items-center gap-2">
          {data.icon && <div className="text-sky-500">{data.icon}</div>}
          <h3 className="text-sm font-bold leading-tight text-[var(--text-active)]">{data.label}</h3>
        </div>
        {style.icon}
      </div>

      <p className="line-clamp-2 text-xs text-[var(--text-muted)]">{data.description}</p>

      <div className="mt-3 flex items-center justify-between border-t border-[var(--panel-border)] pt-3 text-[10px] font-mono text-[var(--text-muted)]">
        <span>{data.id}</span>
        {data.status === "completed" && <span className="font-bold text-emerald-500">LIVE</span>}
      </div>

      <Handle
        type="source"
        position={Position.Right}
        className="!h-3 !w-3 !border-2 !border-[var(--background)] !bg-violet-500"
      />
    </motion.div>
  );
}
