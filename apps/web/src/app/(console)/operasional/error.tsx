"use client";

import { useEffect } from "react";
import { AlertCircle, RotateCcw } from "lucide-react";
import { SpringButton } from "@/components/garda/spring-button";

export default function OperasionalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Operasional route error:", error);
  }, [error]);

  return (
    <div className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center p-6 text-center">
      <div className="grid size-12 place-items-center rounded-lg bg-defisit-soft text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
        <AlertCircle className="size-6" />
      </div>
      <h2 className="mt-4 font-display text-xl font-bold tracking-tight text-foreground">
        Halaman operasional belum dapat dimuat
      </h2>
      <p className="mt-2 text-sm text-muted-foreground">
        {error.message || "Terjadi kesalahan saat memuat data pantauan operasional. Silakan muat ulang atau coba lagi."}
      </p>
      <div className="mt-6 flex flex-wrap justify-center gap-2">
        <SpringButton onClick={() => reset()} variant="primary" className="gap-2">
          <RotateCcw className="size-4" />
          Coba lagi
        </SpringButton>
        <SpringButton onClick={() => window.location.reload()} variant="soft">
          Muat ulang penuh
        </SpringButton>
      </div>
    </div>
  );
}
