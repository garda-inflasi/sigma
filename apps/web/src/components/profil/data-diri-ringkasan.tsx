import { Building2, Calendar, ShieldCheck } from "lucide-react";
import { StatusBadge } from "@/components/garda/status-badge";
import { formatTanggal } from "@/lib/format";
import { inisialNama } from "@/lib/nama";
import { infoPeran, labelStatusAkun } from "@/lib/profil/penugasan";
import { cn } from "@/lib/utils";

/** Tanggal yang sah, atau undefined: format tanggal tidak boleh membuat halaman galat karena data aneh. */
function tanggalBergabung(iso: string | undefined): string | undefined {
  if (!iso || Number.isNaN(new Date(iso).getTime())) return undefined;
  return formatTanggal(iso);
}

/** Ringkasan akun di puncak tab Edit Profil: avatar, nama, email/lembaga, peran, status, MFA, dan tanggal bergabung. */
export function RingkasanAkun({
  nama,
  email,
  lembaga,
  peran,
  status,
  dibuatPada,
  mfaAktif,
}: {
  nama: string;
  email?: string;
  lembaga?: string;
  /** Kode peran (mis. OPS_KOORDINATOR). */
  peran: string;
  /** Status akun dari API (AKTIF, MENUNGGU, NONAKTIF). */
  status: string;
  /** Awal akun (ISO 8601); disembunyikan bila tidak diketahui. */
  dibuatPada?: string;
  /** Status aktivasi verifikasi dua langkah bila tersedia dari API. */
  mfaAktif?: boolean;
}) {
  const statusAkun = labelStatusAkun(status);
  const bergabung = tanggalBergabung(dibuatPada);
  const aktif = status === "AKTIF";

  return (
    <section
      aria-label="Ringkasan akun"
      className="glass relative shrink-0 overflow-hidden rounded-xl p-4 sm:px-5 sm:py-4"
    >
      <div
        aria-hidden
        className="pointer-events-none absolute inset-y-0 right-0 w-1/3 bg-gradient-to-l from-data-50/70 to-transparent dark:from-data-500/5"
      />
      <div className="relative flex flex-col gap-3.5 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3.5">
          <div className="relative shrink-0">
            <div
              aria-hidden
              className="grid size-12 place-items-center rounded-xl bg-gradient-to-br from-navy-800 to-navy-950 font-display text-base font-semibold text-white ring-1 ring-black/5 dark:from-data-600 dark:to-data-800 dark:ring-white/10"
            >
              {inisialNama(nama)}
            </div>
            {aktif && (
              <span
                aria-hidden
                className="absolute -right-0.5 -bottom-0.5 size-3 rounded-full border-2 border-card bg-normal"
              />
            )}
          </div>

          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <p
                title={nama}
                className="truncate font-display text-base font-semibold tracking-tight text-navy-900 sm:text-[17px] dark:text-white"
              >
                {nama}
              </p>
              <span className="inline-flex h-6 items-center rounded-md border border-border bg-secondary/70 px-2 text-xs font-medium text-navy-800 dark:bg-white/10 dark:text-navy-100">
                {infoPeran(peran).label}
              </span>
              <StatusBadge tone={statusAkun.tone}>
                {statusAkun.label}
              </StatusBadge>
            </div>

            {(email || lembaga) && (
              <p className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-xs text-muted-foreground">
                {email && <span className="truncate">{email}</span>}
                {email && lembaga && <span aria-hidden>·</span>}
                {lembaga && (
                  <span className="inline-flex items-center gap-1 truncate">
                    <Building2 className="size-3 shrink-0" aria-hidden />
                    {lembaga}
                  </span>
                )}
              </p>
            )}
          </div>
        </div>

        {(bergabung || mfaAktif !== undefined) && (
          <div className="flex flex-wrap items-center gap-2 sm:shrink-0 sm:flex-col sm:items-end sm:gap-1.5">
            {mfaAktif !== undefined && (
              <span
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[11px] font-medium ring-1 ring-inset",
                  mfaAktif
                    ? "bg-normal-soft text-normal-ink ring-normal/25 dark:bg-normal/15 dark:text-emerald-300"
                    : "bg-waspada-soft text-waspada-ink ring-waspada/25 dark:bg-waspada/15 dark:text-amber-300",
                )}
              >
                <ShieldCheck className="size-3 shrink-0" aria-hidden />
                {mfaAktif ? "2FA terlindungi" : "2FA belum aktif"}
              </span>
            )}
            {bergabung && (
              <p className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                <Calendar className="size-3 shrink-0" aria-hidden />
                {`Bergabung ${bergabung}`}
              </p>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
