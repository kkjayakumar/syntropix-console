"use client";

import { useState } from "react";
import { FlaskConical } from "lucide-react";
import { BRAND } from "@/lib/brand";
import { getOrCreateRequestId } from "@/components/CorrelationBridge";
import { GlassCard } from "@/components/Shell";
import { Button } from "@/components/ui/Button";

export function DogfoodPanel() {
  const allowDogfood = process.env.NEXT_PUBLIC_AEGISOPS_ALLOW_DOGFOOD === "true";
  const enabled = process.env.NEXT_PUBLIC_AEGISOPS_ENV !== "production" || allowDogfood;
  const [status, setStatus] = useState("");
  const [busy, setBusy] = useState(false);

  if (!enabled) return null;

  async function triggerBackendError() {
    setBusy(true);
    setStatus("");
    const requestId = getOrCreateRequestId();
    try {
      const res = await fetch("/api/v1/debug/trigger-error", {
        method: "POST",
        headers: { "X-Request-Id": requestId },
      });
      const body = await res.json().catch(() => ({}));
      if (res.status >= 500) {
        setStatus(`Backend returned ${res.status} (requestId=${body.requestId ?? requestId}). Check inbox in ~10s.`);
      } else {
        setStatus(`Unexpected ${res.status}: ${JSON.stringify(body)}`);
      }
    } catch (e) {
      setStatus(e instanceof Error ? e.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }

  function triggerFrontendError() {
    const requestId = getOrCreateRequestId();
    setStatus(`Throwing client error (requestId=${requestId})…`);
    setTimeout(() => {
      throw new Error(`${BRAND.name} dogfood: intentional UI error (${requestId})`);
    }, 0);
  }

  return (
    <GlassCard label="Dogfood telemetry">
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-500/15 ring-1 ring-amber-400/30">
          <FlaskConical size={20} className="text-amber-500" />
        </div>
        <div className="flex-1">
          <p className="text-sm text-[var(--text-muted)]">
            Trigger real errors on the dashboard. Spotting captures network/console; Copilot ingests backend events.
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            <Button variant="secondary" disabled={busy} onClick={() => void triggerBackendError()}>
              Trigger backend 500
            </Button>
            <Button variant="violet" onClick={triggerFrontendError}>
              Trigger UI error
            </Button>
          </div>
          {status && <p className="mt-3 text-xs text-[var(--text-muted)]">{status}</p>}
        </div>
      </div>
    </GlassCard>
  );
}
