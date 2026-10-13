"use client";

import { useEffect } from "react";
import { getOrCreateRequestId } from "@/components/CorrelationBridge";

declare global {
  interface Window {
    SpottingCapture?: {
      init: (opts: {
        publicKey: string;
        apiBaseUrl: string;
        dashboardUrl?: string;
        buttonLabel?: string;
        headless?: boolean;
      }) => void;
    };
  }
}

/** Loads Spotting capture SDK when NEXT_PUBLIC_SPOTTING_PUBLIC_KEY is set at build time. */
export function SpottingCapture() {
  useEffect(() => {
    const publicKey = process.env.NEXT_PUBLIC_SPOTTING_PUBLIC_KEY;
    const apiBaseUrl = process.env.NEXT_PUBLIC_SPOTTING_API_URL ?? "http://localhost:3000";
    const dashboardUrl = process.env.NEXT_PUBLIC_SPOTTING_DASHBOARD_URL ?? "http://127.0.0.1:3001";
    if (!publicKey) return;

    getOrCreateRequestId();

    const initSdk = () => {
      window.SpottingCapture?.init({
        publicKey,
        apiBaseUrl,
        dashboardUrl,
        buttonLabel: "Report bug",
        headless: false,
      });
    };

    if (window.SpottingCapture) {
      initSdk();
      return;
    }

    const script = document.createElement("script");
    script.src = "/spotting-capture.js";
    script.async = true;
    script.onload = initSdk;
    document.body.appendChild(script);
    return () => {
      script.remove();
    };
  }, []);

  return null;
}
