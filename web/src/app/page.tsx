"use client";

import { HomeChat } from "@/components/HomeChat";

export default function HomePage() {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden" style={{ height: "100%" }}>
      <HomeChat />
    </div>
  );
}
