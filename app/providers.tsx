"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MotionConfig } from "motion/react";
import { ThemeProvider } from "next-themes";
import { useState, type ReactNode } from "react";
import { Toaster } from "sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { SessionProvider } from "@/lib/auth/session-context";
import type { SessionUser } from "@/lib/auth/resolve";

export function Providers({ user, children }: { user: SessionUser | null; children: ReactNode }) {
  const [qc] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { staleTime: 20_000, retry: 1, refetchOnWindowFocus: false },
          mutations: { retry: 0 },
        },
      }),
  );

  return (
    <ThemeProvider attribute="class" defaultTheme="light" enableSystem disableTransitionOnChange>
      <QueryClientProvider client={qc}>
        <SessionProvider user={user}>
          <MotionConfig reducedMotion="user">
            <TooltipProvider delayDuration={250}>
              {children}
              <Toaster
                position="top-right"
                gap={10}
                offset={20}
                toastOptions={{
                  classNames: {
                    toast: "!glass-strong !rounded-xl !border-0 !font-sans",
                    title: "!font-display !font-semibold",
                    description: "!text-muted-foreground",
                  },
                }}
              />
            </TooltipProvider>
          </MotionConfig>
        </SessionProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
