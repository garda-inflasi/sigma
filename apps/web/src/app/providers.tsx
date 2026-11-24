"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MotionConfig } from "motion/react";
import { ThemeProvider } from "next-themes";
import { useState, type ReactNode } from "react";
import { Toaster } from "sonner";
import { DialogKeluar } from "@/components/auth/dialog-keluar";
import { PenjagaDataOffline } from "@/components/auth/penjaga-data-offline";
import { PenyegarIzin } from "@/components/auth/penyegar-izin";
import { TooltipProvider } from "@/components/ui/tooltip";
import { SmoothScrollProvider } from "@/components/motion/smooth-scroll";
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
            <SmoothScrollProvider />
            <TooltipProvider delayDuration={250}>
              {children}
              <PenjagaDataOffline />
              <PenyegarIzin />
              <DialogKeluar />
              <Toaster
                position="bottom-right"
                gap={8}
                offset={20}
                toastOptions={{
                  classNames: {
                    toast: "!glass-strong !rounded-lg !font-sans !text-[13px]",
                    title: "!font-medium",
                    description: "!text-muted-foreground",
                    actionButton: "!rounded-md !bg-primary !text-primary-foreground",
                    cancelButton: "!rounded-md !bg-secondary !text-foreground",
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
