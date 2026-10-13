"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { KeyRound, ShieldCheck } from "lucide-react";
import {
  API_UNAUTHORIZED_EVENT,
  fetchVersion,
  getAPIToken,
  setAPIToken,
  workspaceBootstrap,
} from "@/lib/api";
import { BRAND } from "@/lib/brand";
import { Button } from "@/components/ui/Button";
import { FieldLabel, Input } from "@/components/ui/Input";

/**
 * Blocks the dashboard when production API auth is enforced and the browser
 * has no (or an invalid) token. Entry works without authenticated bootstrap.
 */
export function ApiAuthGate({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [locked, setLocked] = useState(false);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [hint, setHint] = useState<string | null>(null);

  const evaluate = useCallback(async (forceLock = false) => {
    try {
      const version = await fetchVersion();
      const enforced = Boolean(version.auth?.enforced);
      const token = getAPIToken();
      if (!enforced) {
        setLocked(false);
        setHint(null);
        return;
      }
      if (!token) {
        setLocked(true);
        setHint("Staging requires an API token. Paste AEGISOPS_API_TOKEN from Secrets Manager (dev/syntropix).");
        return;
      }
      if (forceLock) {
        setLocked(true);
        setHint("That token was rejected. Confirm it matches AEGISOPS_API_TOKEN on the server.");
        return;
      }
      // Probe an authenticated route so a stale localStorage token still unlocks the gate.
      try {
        await workspaceBootstrap();
        setLocked(false);
        setError(null);
        setHint(null);
      } catch {
        setLocked(true);
        setHint("Saved token is invalid or expired. Paste the current AEGISOPS_API_TOKEN.");
      }
    } catch {
      // Version unreachable — don't trap the user; let pages show their own errors.
      setLocked(false);
    } finally {
      setReady(true);
    }
  }, []);

  useEffect(() => {
    void evaluate();
  }, [evaluate]);

  useEffect(() => {
    const onUnauthorized = () => {
      void evaluate(true);
    };
    const onTokenChanged = () => {
      void evaluate();
    };
    window.addEventListener(API_UNAUTHORIZED_EVENT, onUnauthorized);
    window.addEventListener("aegis:api-token-changed", onTokenChanged);
    return () => {
      window.removeEventListener(API_UNAUTHORIZED_EVENT, onUnauthorized);
      window.removeEventListener("aegis:api-token-changed", onTokenChanged);
    };
  }, [evaluate]);

  async function save() {
    const next = draft.trim();
    if (next.length < 16) {
      setError("Token must be at least 16 characters");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      setAPIToken(next);
      await workspaceBootstrap();
      setLocked(false);
      setHint(null);
      setDraft("");
      window.dispatchEvent(new Event("aegis:workspace-refresh"));
    } catch (cause) {
      setAPIToken("");
      setError(cause instanceof Error ? cause.message : "Token rejected by API");
      setLocked(true);
    } finally {
      setBusy(false);
    }
  }

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[var(--background)] text-sm text-[var(--text-muted)]">
        Checking API access…
      </div>
    );
  }

  if (!locked) return <>{children}</>;

  return (
    <div className="relative flex min-h-screen items-center justify-center px-4 py-10">
      <div className="glass-panel w-full max-w-md p-6 md:p-8">
        <div className="mb-6 flex items-start gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-[var(--accent-soft)] text-[var(--accent)]">
            <ShieldCheck size={20} />
          </div>
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--text-muted)]">{BRAND.name}</p>
            <h1 className="mt-1 text-xl font-semibold text-[var(--text-active)]">API access required</h1>
            <p className="mt-1.5 text-sm text-[var(--text-muted)]">
              {hint ?? "Enter the server API token to continue. It is stored only in this browser."}
            </p>
          </div>
        </div>

        <FieldLabel>AEGISOPS_API_TOKEN</FieldLabel>
        <Input
          type="password"
          autoComplete="off"
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") void save();
          }}
          placeholder="Paste token from Secrets Manager"
          className="mb-3 font-mono text-xs"
        />

        {error && (
          <p className="mb-3 rounded-xl border border-rose-500/25 bg-rose-500/10 px-3 py-2 text-sm text-rose-400">{error}</p>
        )}

        <Button variant="primary" className="w-full" disabled={busy} onClick={() => void save()}>
          <KeyRound size={16} />
          {busy ? "Verifying…" : "Save and continue"}
        </Button>

        <p className="mt-4 text-xs leading-relaxed text-[var(--text-muted)]">
          Recover from AWS Secrets Manager key <code className="text-[var(--text-active)]">dev/syntropix</code> →{" "}
          <code className="text-[var(--text-active)]">AEGISOPS_API_TOKEN</code>, or from the host{" "}
          <code className="text-[var(--text-active)]">/opt/aegisops/.env</code>. Prefer retrieving the token over
          disabling auth.
        </p>
      </div>
    </div>
  );
}
