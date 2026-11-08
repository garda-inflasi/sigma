import { StatusBadge } from "@/components/garda/status-badge";
import { formatTanggal } from "@/lib/format";
import { inisialNama } from "@/lib/nama";
import { infoPeran, labelStatusAkun } from "@/lib/profil/penugasan";

/** Tanggal yang sah, atau undefined: format tanggal tidak boleh membuat halaman galat karena data aneh. */
function tanggalBergabung(iso: string | undefined): string | undefined {
  if (!iso || Number.isNaN(new Date(iso).getTime())) return undefined;
  return formatTanggal(iso);
}

/** Ringkasan akun di puncak tab Edit Profil: avatar, nama, peran, status, dan tanggal bergabung. */
export function RingkasanAkun({
  nama,
  peran,
  status,
  dibuatPada,
}: {
  nama: string;
  /** Kode peran (mis. OPS_KOORDINATOR). */
  peran: string;
  /** Status akun dari API (AKTIF, MENUNGGU, NONAKTIF). */
  status: string;
  /** Awal akun (ISO 8601); disembunyikan bila tidak diketahui. */
  dibuatPada?: string;
}) {
  const statusAkun = labelStatusAkun(status);
  const bergabung = tanggalBergabung(dibuatPada);

  return (
    <section
      aria-label="Ringkasan akun"
      className="glass-strong flex flex-col items-center gap-4 rounded-xl p-5 text-center sm:flex-row sm:gap-5 sm:p-6 sm:text-left"
    >
      <div
        aria-hidden
        className="grid size-20 shrink-0 place-items-center rounded-2xl bg-gradient-to-br from-navy-700 to-navy-900 font-display text-2xl font-semibold text-white shadow-ambient dark:from-data-600 dark:to-data-800"
      >
        {inisialNama(nama)}
      </div>
      <div className="min-w-0 flex-1 space-y-2.5">
        <p
          title={nama}
          className="truncate font-display text-xl font-semibold text-navy-900 dark:text-white"
        >
          {nama}
        </p>
        <div className="flex flex-wrap items-center justify-center gap-2 sm:justify-start">
          <span className="inline-flex h-6 items-center rounded-full bg-navy-50 px-2.5 text-xs font-medium text-navy-700 dark:bg-white/10 dark:text-navy-100">
            {infoPeran(peran).label}
          </span>
          <StatusBadge tone={statusAkun.tone}>{statusAkun.label}</StatusBadge>
        </div>
        {bergabung && (
          <p className="text-sm text-muted-foreground">{`Bergabung ${bergabung}`}</p>
        )}
      </div>
    </section>
  );
}
