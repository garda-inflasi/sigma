import * as React from "react";
import { kelasKontrol } from "@/components/ui/kontrol";
import { cn } from "@/lib/utils";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        kelasKontrol,
        "h-9 w-full max-w-full min-w-0 px-3 py-1 text-base selection:bg-primary selection:text-primary-foreground file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground disabled:pointer-events-none md:text-sm [&[readonly]]:bg-muted/60",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
