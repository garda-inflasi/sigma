import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Kepala halaman ringkas: di desktop judul, deskripsi satu baris, tab, dan aksi sejajar dalam satu baris agar ruang
 * vertikal dipakai untuk isi. Di layar kecil tersusun bertumpuk dan deskripsi tidak dipotong.
 */
export function PageHeader({
  title,
  description,
  actions,
  tabs,
  className,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  /** Tab tampilan, sebaris dengan judul di desktop. */
  tabs?: ReactNode;
  className?: string;
}) {
  return (
    <header
      className={cn(
        "flex shrink-0 flex-col gap-2 lg:flex-row lg:items-center lg:gap-4",
        className,
      )}
    >
      <div className="min-w-0 lg:flex-1">
        <h1 className="truncate text-display-md font-semibold text-navy-900 dark:text-white">
          {title}
        </h1>
        {description && (
          <p
            title={typeof description === "string" ? description : undefined}
            className="mt-0.5 text-[13px] leading-snug text-muted-foreground lg:truncate"
          >
            {description}
          </p>
        )}
      </div>
      {tabs && <div className="max-w-full min-w-0 lg:shrink-0">{tabs}</div>}
      {actions && (
        <div className="flex flex-wrap items-center gap-2 lg:shrink-0 lg:flex-nowrap">
          {actions}
        </div>
      )}
    </header>
  );
}

export function PanelTitle({
  title,
  description,
  actions,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
      <div className="min-w-0 flex-1 basis-56">
        <h2 className="text-display-sm font-semibold text-navy-900 dark:text-white">
          {title}
        </h2>
        {description && (
          <p className="mt-0.5 text-[13px] text-muted-foreground">
            {description}
          </p>
        )}
      </div>
      {actions}
    </div>
  );
}
