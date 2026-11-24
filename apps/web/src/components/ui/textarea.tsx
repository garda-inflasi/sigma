import * as React from "react";
import { kelasKontrol } from "@/components/ui/kontrol";
import { cn } from "@/lib/utils";

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        kelasKontrol,
        "flex field-sizing-content min-h-16 w-full max-w-full min-w-0 px-3 py-2 text-base placeholder:text-muted-foreground md:text-sm [&[readonly]]:bg-muted/60",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
