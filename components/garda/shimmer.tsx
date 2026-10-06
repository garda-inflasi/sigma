import { cn } from "@/lib/utils";

/** Skeleton dengan kilau bergerak (utilitas `shimmer` di globals.css). */
export function Shimmer({ className }: { className?: string }) {
  return <div aria-hidden className={cn("shimmer rounded-lg", className)} />;
}

export function KpiSkeleton() {
  return (
    <div className="glass rounded-xl p-5">
      <Shimmer className="h-3.5 w-28" />
      <Shimmer className="mt-4 h-9 w-32 rounded-xl" />
      <div className="mt-5 flex items-end justify-between">
        <Shimmer className="h-3 w-24" />
        <Shimmer className="h-9 w-28 rounded-lg" />
      </div>
    </div>
  );
}

export function TableSkeleton({ rows = 8, cols = 6 }: { rows?: number; cols?: number }) {
  return (
    <div className="space-y-2.5 p-4" role="status" aria-label="Memuat data">
      {Array.from({ length: rows }).map((_, r) => (
        <div key={r} className="flex items-center gap-4">
          <Shimmer className="size-4 rounded-md" />
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
    <div className={cn("glass rounded-xl p-6", className)}>
      <Shimmer className="h-5 w-44" />
      <Shimmer className="mt-2 h-3.5 w-72 max-w-full" />
      <Shimmer className="mt-6 h-56 w-full rounded-xl" />
    </div>
  );
}
