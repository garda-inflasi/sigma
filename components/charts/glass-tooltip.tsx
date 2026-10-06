"use client";

import { formatTanggal } from "@/lib/format";

interface Item {
  name?: string | number;
  value?: number | string | readonly (number | string)[];
  color?: string;
  dataKey?: string | number | ((obj: unknown) => unknown);
}

/** Tooltip Recharts bergaya kaca. Dipakai lintas grafik. */
export function GlassTooltip({
  active, payload, label, valueFormat = (v) => String(v), labelFormat = (l) => formatTanggal(String(l), "EEEE, d MMM"),
}: {
  active?: boolean;
  payload?: readonly Item[];
  label?: string | number;
  valueFormat?: (v: number) => string;
  labelFormat?: (l: string | number) => string;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div className="glass-strong min-w-44 rounded-xl px-3 py-2.5 text-xs">
      {label !== undefined && <p className="mb-1.5 font-medium text-foreground">{labelFormat(label)}</p>}
      <ul className="space-y-1">
        {payload.filter((p) => typeof p.value === "number").map((p) => (
          <li key={String(p.dataKey)} className="flex items-center justify-between gap-4">
            <span className="inline-flex items-center gap-1.5 text-muted-foreground">
              <span className="size-2 rounded-full" style={{ background: p.color }} /> {p.name}
            </span>
            <span className="num font-semibold text-foreground">{valueFormat(p.value as number)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
