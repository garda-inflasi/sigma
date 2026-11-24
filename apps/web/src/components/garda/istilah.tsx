"use client";

import type { ReactNode } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { GLOSARIUM, type IstilahGlosarium } from "@/lib/glosarium";
import { cn } from "@/lib/utils";

/** Istilah bergaris titik dengan penjelasan singkat saat kursor atau fokus keyboard berada di atasnya. */
export function Istilah({
  kunci,
  arti,
  children,
  className,
}: {
  kunci?: IstilahGlosarium;
  arti?: string;
  children: ReactNode;
  className?: string;
}) {
  const teks = arti ?? (kunci ? GLOSARIUM[kunci] : undefined);
  if (!teks) return <>{children}</>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          className={cn(
            "cursor-help underline decoration-muted-foreground/50 decoration-dotted underline-offset-2",
            className,
          )}
        >
          {children}
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-64 text-xs leading-relaxed">
        {teks}
      </TooltipContent>
    </Tooltip>
  );
}
