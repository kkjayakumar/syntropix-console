"use client";

import { useEffect } from "react";

const REQUEST_ID_KEY = "aegisops_request_id";

export function getOrCreateRequestId(): string {
  if (typeof window === "undefined") return "";
  let id = sessionStorage.getItem(REQUEST_ID_KEY);
  if (!id) {
    id = `req_ui_${crypto.randomUUID().replace(/-/g, "").slice(0, 12)}`;
    sessionStorage.setItem(REQUEST_ID_KEY, id);
  }
  return id;
}

/** Tags dashboard API calls with X-Request-Id for Spotting/Copilot correlation. */
export function CorrelationBridge() {
  useEffect(() => {
    getOrCreateRequestId();
  }, []);
  return null;
}
