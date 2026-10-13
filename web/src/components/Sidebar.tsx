"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import {
  Bot,
  Box,
  FolderKanban,
  Moon,
  Plus,
  Settings2,
  Sun,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { BRAND } from "@/lib/brand";
import { workspaceBootstrap } from "@/lib/api";
import type { WorkspaceProject } from "@/lib/types";

const PROJECT_KEY = "aegis_workspace_project_id";
const ENV_KEY = "aegis_workspace_env_id";

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  const [projects, setProjects] = useState<WorkspaceProject[]>([]);
  const [activeProject, setActiveProject] = useState<number | null>(null);
  const [loadingProjects, setLoadingProjects] = useState(true);

  const loadBootstrap = useCallback(async () => {
    const data = await workspaceBootstrap();
    const nextProjects = data.projects ?? [];
    const nextEnvironments = data.environments ?? [];
    setProjects(nextProjects);
    const stored = Number(localStorage.getItem(PROJECT_KEY) || 0);
    const pid = nextProjects.some((p) => p.id === stored) ? stored : nextProjects[0]?.id ?? null;
    setActiveProject(pid);
    if (pid != null) {
      const available = nextEnvironments
        .filter((e) => e.projectId === pid)
        .sort((a, b) => a.sortOrder - b.sortOrder);
      const storedEnv = Number(localStorage.getItem(ENV_KEY) || 0);
      const nextEnv = available.some((e) => e.id === storedEnv) ? storedEnv : available[0]?.id ?? null;
      if (nextEnv != null && nextEnv !== storedEnv) {
        localStorage.setItem(ENV_KEY, String(nextEnv));
      }
    }
    return { projects: nextProjects, environments: nextEnvironments };
  }, []);

  useEffect(() => setMounted(true), []);
  useEffect(() => {
    loadBootstrap()
      .catch(() => {
        setProjects([]);
      })
      .finally(() => setLoadingProjects(false));
  }, [loadBootstrap]);

  useEffect(() => {
    const onProject = (event: Event) => {
      setActiveProject((event as CustomEvent<number>).detail);
    };
    const onRefresh = () => {
      void loadBootstrap().catch(() => undefined);
    };
    window.addEventListener("aegis:project-change", onProject);
    window.addEventListener("aegis:workspace-refresh", onRefresh);
    return () => {
      window.removeEventListener("aegis:project-change", onProject);
      window.removeEventListener("aegis:workspace-refresh", onRefresh);
    };
  }, [loadBootstrap]);

  function openProject(id: number) {
    localStorage.setItem(PROJECT_KEY, String(id));
    setActiveProject(id);
    window.dispatchEvent(new CustomEvent("aegis:project-change", { detail: id }));
    router.push("/workspace");
  }

  return (
    <aside className="flex h-full w-[232px] shrink-0 flex-col border-r border-[var(--panel-border)] bg-[var(--sidebar-bg)]">
      <div className="flex h-14 items-center gap-2 border-b border-[var(--panel-border)] px-4">
        <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-500 text-white shadow-lg shadow-indigo-500/20">
          <Bot size={15} />
        </div>
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-[var(--text-active)]">{BRAND.name}</p>
          <p className="text-[10px] text-[var(--text-muted)]">Agentic delivery platform</p>
        </div>
      </div>

      <nav className="space-y-1 p-3">
        <Link href="/" className="nav-item" data-active={pathname === "/"}>
          <Bot size={16} />
          <span>AI workspace</span>
        </Link>
        <Link href="/settings" className="nav-item" data-active={(pathname ?? "").startsWith("/settings")}>
          <Settings2 size={16} />
          <span>Organization settings</span>
        </Link>
      </nav>

      <div className="mx-3 border-t border-[var(--panel-border)]" />

      <div className="flex min-h-0 flex-1 flex-col px-3 pt-4">
        <div className="mb-2 flex items-center justify-between px-2">
          <p className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--text-muted)]">Projects</p>
          <Link
            href="/workspace"
            className="rounded-md p-1 text-[var(--text-muted)] hover:bg-[var(--glass-bg)] hover:text-[var(--text-active)]"
            title="Open workspace"
          >
            <Plus size={14} />
          </Link>
        </div>
        <div className="space-y-1 overflow-y-auto">
          {projects.map((project) => {
            const active = pathname?.startsWith("/workspace") && activeProject === project.id;
            return (
              <button
                key={project.id}
                type="button"
                onClick={() => openProject(project.id)}
                className="nav-item w-full text-left"
                data-active={active}
              >
                <FolderKanban size={15} />
                <span className="min-w-0 flex-1 truncate">{project.name}</span>
              </button>
            );
          })}
          {loadingProjects && (
            <div className="rounded-xl border border-dashed border-[var(--panel-border)] p-3 text-xs text-[var(--text-muted)]">
              Loading projects…
            </div>
          )}
          {!loadingProjects && !projects.length && (
            <div className="rounded-xl border border-dashed border-[var(--panel-border)] p-3 text-xs text-[var(--text-muted)]">
              No projects yet
            </div>
          )}
        </div>
      </div>

      <div className="border-t border-[var(--panel-border)] p-3">
        <div className="mb-2 flex items-center gap-2 rounded-xl bg-[var(--glass-bg)] px-2.5 py-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-full bg-indigo-500/15 text-indigo-500">
            <Box size={14} />
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-xs font-medium text-[var(--text-active)]">Syntropix</p>
            <p className="text-[10px] text-[var(--text-muted)]">Organization</p>
          </div>
        </div>
        {mounted && (
          <button
            type="button"
            onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
            className="nav-item w-full"
          >
            {resolvedTheme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
            <span>{resolvedTheme === "dark" ? "Light appearance" : "Dark appearance"}</span>
          </button>
        )}
      </div>
    </aside>
  );
}
