import type { Metadata } from "next";
import { Outfit } from "next/font/google";
import { ThemeProvider } from "next-themes";
import "./globals.css";
import { ApiAuthGate } from "@/components/ApiAuthGate";
import { CorrelationBridge } from "@/components/CorrelationBridge";
import { SpottingCapture } from "@/components/SpottingCapture";
import { Sidebar } from "@/components/Sidebar";
import { AnimatedBackground } from "@/components/ui/AnimatedBackground";
import { BRAND } from "@/lib/brand";

const outfit = Outfit({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

export const metadata: Metadata = {
  title: BRAND.name,
  description: `${BRAND.tagline} — ${BRAND.domain}`,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${outfit.variable} font-sans antialiased`}>
        <ThemeProvider attribute="data-theme" defaultTheme="system" enableSystem>
          <CorrelationBridge />
          <SpottingCapture />
          <AnimatedBackground />
          <ApiAuthGate>
            <div className="relative z-10 flex h-screen min-h-0 overflow-hidden">
              <Sidebar />
              <main className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[var(--background)]">
                {children}
              </main>
            </div>
          </ApiAuthGate>
        </ThemeProvider>
      </body>
    </html>
  );
}
