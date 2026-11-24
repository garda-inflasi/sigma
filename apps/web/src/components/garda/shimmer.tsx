import { cn } from "@/lib/utils";

/** Skeleton dengan kilau bergerak (utilitas `shimmer` di globals.css). */
export function Shimmer({ className }: { className?: string }) {
  return <div aria-hidden className={cn("shimmer rounded-md", className)} />;
}

export function KpiSkeleton() {
  return (
    <div className="glass rounded-lg p-4">
      <Shimmer className="h-3.5 w-28" />
      <Shimmer className="mt-4 h-8 w-32" />
      <div className="mt-5 flex items-end justify-between">
        <Shimmer className="h-3 w-24" />
        <Shimmer className="h-8 w-24" />
      </div>
    </div>
  );
}

export function TableSkeleton({ rows = 8, cols = 6 }: { rows?: number; cols?: number }) {
  return (
    <div className="space-y-2.5 p-4" role="status" aria-label="Memuat data">
      {Array.from({ length: rows }).map((_, r) => (
        <div key={r} className="flex items-center gap-4">
          <Shimmer className="size-4 shrink-0 rounded-md" />
          {Array.from({ length: cols }).map((__, c) => (
            <Shimmer key={c} className={cn("h-4", c === 0 ? "w-40" : c === cols - 1 ? "w-16" : "flex-1")} />
          ))}
        </div>
      ))}
    </div>
  );
}

export function PanelSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn("glass rounded-lg p-5", className)}>
      <Shimmer className="h-5 w-44" />
      <Shimmer className="mt-2 h-3.5 w-72 max-w-full" />
      <Shimmer className="mt-5 h-56 w-full" />
    </div>
  );
}

/** Daftar baris dengan ikon dan dua baris teks (aktivitas, antrean, riwayat). */
export function ListSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="space-y-3" role="status" aria-label="Memuat daftar">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-3">
          <Shimmer className="size-8 shrink-0 rounded-md" />
          <div className="flex-1 space-y-1.5">
            <Shimmer className="h-3.5 w-2/5" />
            <Shimmer className="h-3 w-3/5" />
          </div>
          <Shimmer className="h-5 w-14" />
        </div>
      ))}
    </div>
  );
}

/** Deretan kartu KPI yang sedang dimuat. */
export function KpiRowSkeleton({ count = 4 }: { count?: number }) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4" role="status" aria-label="Memuat ringkasan">
      {Array.from({ length: count }).map((_, i) => (
        <KpiSkeleton key={i} />
      ))}
    </div>
  );
}
