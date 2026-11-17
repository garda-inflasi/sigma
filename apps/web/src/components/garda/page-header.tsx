import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Kepala halaman: judul 20px, deskripsi 13px, aksi utama di kanan. `tabs` (mis. <ViewTabs/>) dan `kontrol`
 * (Filter & Sort, pencarian) tampil di bawah judul, sebelum konten.
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
  /** Tab tampilan di bawah judul. */
  tabs?: ReactNode;
  className?: string;
}) {
  return (
    <header className={cn("space-y-3", className)}>
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 max-w-2xl">
          <h1 className="text-display-md font-semibold text-navy-900 dark:text-white">{title}</h1>
          {description && <p className="mt-1 text-[13px] leading-relaxed text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {tabs}
    </header>
  );
}

export function PanelTitle({ title, description, actions }: { title: string; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <h2 className="text-display-sm font-semibold text-navy-900 dark:text-white">{title}</h2>
        {description && <p className="mt-0.5 text-[13px] text-muted-foreground">{description}</p>}
      </div>
      {actions}
    </div>
  );
}
