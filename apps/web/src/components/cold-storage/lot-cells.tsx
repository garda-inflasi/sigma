import { nadaRisiko, urgensiLot, type Urgensi } from "@/lib/api/cold-storage";
import { toneClass } from "@/lib/design/tokens";
import { formatPersen } from "@/lib/format";
import { cn } from "@/lib/utils";

/** Garis vertikal di tepi kiri baris: merah untuk kritis, kuning untuk waspada, tanpa garis bila aman. */
export function UrgencyStripe({
  urgensi,
  className,
}: {
  urgensi: Urgensi;
  className?: string;
}) {
  if (urgensi === "aman") return null;
  return (
    <span
      aria-hidden
      data-urgensi={urgensi}
      className={cn(
        "absolute inset-y-2 left-0 w-[3px] rounded-r-full",
        urgensi === "kritis" ? "bg-defisit" : "bg-waspada",
        className,
      )}
    />
  );
}

/** Sisa masa simpan dalam hari, diwarnai menurut urgensi. */
export function ShelfLifeBadge({ hari }: { hari: number }) {
  const urgensi = urgensiLot(hari);
  return (
    <span
      className={cn(
        "num inline-flex h-6 items-center rounded-full px-2 text-xs font-medium whitespace-nowrap",
        urgensi === "kritis" &&
          "bg-defisit-soft text-defisit-ink dark:bg-defisit/15 dark:text-rose-300",
        urgensi === "waspada" &&
          "bg-waspada-soft text-waspada-ink dark:bg-waspada/15 dark:text-amber-300",
        urgensi === "aman" &&
          "bg-mist-100 text-muted-foreground dark:bg-white/5",
      )}
    >
      {hari} hari
    </span>
  );
}

/** Batang risiko food loss dengan angka persen; `ambang` menentukan batas merah. */
export function RiskBar({
  risiko,
  ambang,
}: {
  risiko: number;
  ambang: number;
}) {
  const persen = Math.round(risiko * 100);
  return (
    <span
      className="flex items-center gap-2"
      title={`Risiko food loss ${persen}%`}
    >
      <span
        aria-hidden
        className="h-1.5 w-16 shrink-0 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"
      >
        <span
          className={cn(
            "block h-full rounded-full",
            toneClass[nadaRisiko(risiko, ambang)].bg,
          )}
          style={{ width: `${Math.min(persen, 100)}%` }}
        />
      </span>
      <span className="num w-9 text-xs">
        {formatPersen(persen, { digits: 0 })}
      </span>
    </span>
  );
}
