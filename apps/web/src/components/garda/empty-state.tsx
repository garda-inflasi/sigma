import type { LucideIcon } from "lucide-react";
import { Inbox } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** Keadaan kosong: ikon dalam kotak lembut, judul, penjelasan satu kalimat, dan aksi opsional. */
export function EmptyState({
  title,
  description,
  icon: Ikon = Inbox,
  action,
  className,
}: {
  title: string;
  description?: ReactNode;
  icon?: LucideIcon;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div role="status" className={cn("flex flex-col items-center justify-center px-6 py-12 text-center", className)}>
      <span className="grid size-10 place-items-center rounded-lg border border-border bg-secondary text-muted-foreground">
        <Ikon className="size-5" strokeWidth={1.75} aria-hidden />
      </span>
      <p className="mt-3 text-sm font-medium text-foreground">{title}</p>
      {description && <p className="mt-1 max-w-sm text-[13px] text-muted-foreground">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
