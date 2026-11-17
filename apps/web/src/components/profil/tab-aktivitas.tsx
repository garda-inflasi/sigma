"use client";

import { History, Laptop, Loader2, ShieldAlert, Smartphone } from "lucide-react";
import { useId, useState } from "react";
import { PanelTitle } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import {
  AKSI_FILTER,
  LABEL_HASIL,
  LABEL_STATUS_SESI,
  labelAksi,
  type Aktivitas,
  type FilterAktivitas,
  type FilterStatusSesi,
  type RiwayatSesi,
} from "@/lib/api/aktivitas";
import { formatRelatif, formatTanggal } from "@/lib/format";
import { useAktivitasSaya, useRiwayatSesi } from "@/lib/query/aktivitas-hooks";
import { cn } from "@/lib/utils";

const POLA_WAKTU = "d MMM yyyy, HH:mm";
const KELAS_KONTROL =
  "h-11 w-full rounded-lg border border-input bg-background px-3 text-sm focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none sm:h-10";

const adalahPonsel = (perangkat: string) => /Android|iOS|iPadOS/.test(perangkat);

function Kolom({ label, children }: { label: string; children: (id: string) => React.ReactNode }) {
  const id = useId();
  return (
    <div className="min-w-0 space-y-1.5">
      <label htmlFor={id} className="text-xs font-medium text-muted-foreground">
        {label}
      </label>
      {children(id)}
    </div>
  );
}

function Memuat({ nama }: { nama: string }) {
  return (
    <div role="status" aria-label={`Memuat ${nama}`} className="space-y-3">
      {[0, 1, 2].map((i) => (
        <Shimmer key={i} className="h-14 w-full rounded-lg" />
      ))}
    </div>
  );
}

function Galat({ nama, onUlang }: { nama: string; onUlang: () => void }) {
  return (
    <div role="alert" className="flex flex-col items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm">
      <p>Gagal memuat {nama}. Coba lagi beberapa saat.</p>
      <Button variant="outline" size="sm" onClick={onUlang}>
        Coba lagi
      </Button>
    </div>
  );
}

function MuatLagi({ ada, memuat, onMuat }: { ada: boolean; memuat: boolean; onMuat: () => void }) {
  if (!ada) return null;
  return (
    <div className="mt-4 flex justify-center">
      <Button variant="outline" onClick={onMuat} disabled={memuat} className="min-h-11">
        {memuat && <Loader2 className="animate-spin" aria-hidden />}
        Muat lebih banyak
      </Button>
    </div>
  );
}

// ── Riwayat sesi ───────────────────────────────────────────────────

function BarisSesi({ sesi }: { sesi: RiwayatSesi }) {
  const Ikon = adalahPonsel(sesi.perangkat) ? Smartphone : Laptop;
  const status = LABEL_STATUS_SESI[sesi.status];
  return (
    <li className="flex items-start gap-3 rounded-lg border border-border/70 bg-white/60 p-3.5 dark:bg-white/5">
      <Ikon className="mt-0.5 size-5 shrink-0 text-muted-foreground" aria-hidden />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium text-navy-900 dark:text-white">{sesi.perangkat}</p>
          <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
          {sesi.saatIni && <StatusBadge tone="data">Sesi Ini</StatusBadge>}
        </div>
        <p className="mt-1 text-xs text-muted-foreground">
          {sesi.ip ? `IP ${sesi.ip} · ` : ""}Masuk{" "}
          <time dateTime={sesi.mulaiPada}>{formatTanggal(sesi.mulaiPada, POLA_WAKTU)}</time>
          {sesi.berakhirPada && (
            <>
              {" "}
              · Berakhir{" "}
              <time dateTime={sesi.berakhirPada}>{formatTanggal(sesi.berakhirPada, POLA_WAKTU)}</time>
            </>
          )}
        </p>
      </div>
    </li>
  );
}

export function RiwayatSesiPanel() {
  const [status, setStatus] = useState<FilterStatusSesi>("semua");
  const q = useRiwayatSesi(status);
  const items = q.data?.pages.flatMap((h) => h.items) ?? [];

  return (
    <section className="glass rounded-xl p-5 sm:p-6" aria-label="Riwayat masuk">
      <PanelTitle
        title="Riwayat masuk"
        description="Sesi masuk ke akun Anda, termasuk yang sudah berakhir. Tidak mengenali salah satunya? Ganti kata sandi dan cabut perangkat di tab Perangkat & Sesi Aktif."
      />
      <div className="mt-5 max-w-xs">
        <Kolom label="Status sesi">
          {(id) => (
            <select
              id={id}
              value={status}
              onChange={(e) => setStatus(e.target.value as FilterStatusSesi)}
              className={KELAS_KONTROL}
            >
              <option value="semua">Semua</option>
              <option value="aktif">Aktif</option>
              <option value="berakhir">Sudah berakhir</option>
            </select>
          )}
        </Kolom>
      </div>
      <div className="mt-5">
        {q.isPending ? (
          <Memuat nama="riwayat masuk" />
        ) : q.isError ? (
          <Galat nama="riwayat masuk" onUlang={() => void q.refetch()} />
        ) : items.length === 0 ? (
          <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
            Belum ada sesi yang cocok.
          </p>
        ) : (
          <ul aria-label="Riwayat sesi" className="space-y-2.5">
            {items.map((s) => (
              <BarisSesi key={s.id} sesi={s} />
            ))}
          </ul>
        )}
        <MuatLagi ada={!!q.hasNextPage} memuat={q.isFetchingNextPage} onMuat={() => void q.fetchNextPage()} />
      </div>
    </section>
  );
}

// ── Aktivitas akun ─────────────────────────────────────────────────

function BarisAktivitas({ a }: { a: Aktivitas }) {
  const hasil = LABEL_HASIL[a.hasil];
  const sasaran = a.sebagai === "sasaran";
  const Ikon = sasaran ? ShieldAlert : History;
  return (
    <li className="flex items-start gap-3 rounded-lg border border-border/70 bg-white/60 p-3.5 dark:bg-white/5">
      <Ikon
        className={cn("mt-0.5 size-5 shrink-0", sasaran ? "text-waspada-600" : "text-muted-foreground")}
        aria-hidden
      />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium text-navy-900 dark:text-white">{a.label}</p>
          <StatusBadge tone={hasil.tone}>{hasil.label}</StatusBadge>
          {sasaran && <StatusBadge tone="waspada">Oleh orang lain</StatusBadge>}
        </div>
        <p className="mt-1 text-xs text-muted-foreground">
          <time dateTime={a.waktu} title={formatTanggal(a.waktu, POLA_WAKTU)}>
            {formatRelatif(a.waktu)}
          </time>
          {sasaran && a.olehPeran ? ` · dilakukan ${a.olehPeran}` : ""}
          {a.perangkat ? ` · ${a.perangkat}` : ""}
          {a.ip ? ` · IP ${a.ip}` : ""}
        </p>
      </div>
    </li>
  );
}

const FILTER_KOSONG: FilterAktivitas = { sebagai: "semua" };

export function AktivitasAkunPanel() {
  const [filter, setFilter] = useState<FilterAktivitas>(FILTER_KOSONG);
  const q = useAktivitasSaya(filter);
  const items = q.data?.pages.flatMap((h) => h.items) ?? [];
  const ada = Object.values(filter).some((v) => v && v !== "semua");
  const ubah = (bagian: Partial<FilterAktivitas>) => setFilter((f) => ({ ...f, ...bagian }));

  return (
    <section className="glass rounded-xl p-5 sm:p-6" aria-label="Aktivitas akun">
      <PanelTitle
        title="Aktivitas akun"
        description="Jejak tindakan di akun Anda dari audit trail: masuk, ganti kata sandi, verifikasi 2 langkah, validasi, dan persetujuan, termasuk tindakan administrator terhadap akun Anda."
      />

      <div role="group" aria-label="Saring aktivitas" className="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <Kolom label="Jenis aktivitas">
          {(id) => (
            <select id={id} value={filter.aksi ?? ""} onChange={(e) => ubah({ aksi: e.target.value || undefined })} className={KELAS_KONTROL}>
              <option value="">Semua aktivitas</option>
              {AKSI_FILTER.map((k) => (
                <option key={k} value={k}>
                  {labelAksi(k)}
                </option>
              ))}
            </select>
          )}
        </Kolom>
        <Kolom label="Hasil">
          {(id) => (
            <select
              id={id}
              value={filter.hasil ?? ""}
              onChange={(e) => ubah({ hasil: (e.target.value || undefined) as FilterAktivitas["hasil"] })}
              className={KELAS_KONTROL}
            >
              <option value="">Semua hasil</option>
              <option value="BERHASIL">Berhasil</option>
              <option value="GAGAL">Gagal</option>
              <option value="DITOLAK">Ditolak</option>
            </select>
          )}
        </Kolom>
        <Kolom label="Pelaku">
          {(id) => (
            <select
              id={id}
              value={filter.sebagai ?? "semua"}
              onChange={(e) => ubah({ sebagai: e.target.value as FilterAktivitas["sebagai"] })}
              className={KELAS_KONTROL}
            >
              <option value="semua">Saya dan orang lain</option>
              <option value="pelaku">Dilakukan saya</option>
              <option value="sasaran">Dilakukan orang lain</option>
            </select>
          )}
        </Kolom>
        <Kolom label="Dari tanggal">
          {(id) => (
            <input id={id} type="date" value={filter.dari ?? ""} max={filter.sampai || undefined} onChange={(e) => ubah({ dari: e.target.value || undefined })} className={KELAS_KONTROL} />
          )}
        </Kolom>
        <Kolom label="Sampai tanggal">
          {(id) => (
            <input id={id} type="date" value={filter.sampai ?? ""} min={filter.dari || undefined} onChange={(e) => ubah({ sampai: e.target.value || undefined })} className={KELAS_KONTROL} />
          )}
        </Kolom>
      </div>
      {ada && (
        <div className="mt-3">
          <Button variant="ghost" size="sm" onClick={() => setFilter(FILTER_KOSONG)}>
            Hapus saringan
          </Button>
        </div>
      )}

      <div className="mt-5">
        {q.isPending ? (
          <Memuat nama="aktivitas akun" />
        ) : q.isError ? (
          <Galat nama="aktivitas akun" onUlang={() => void q.refetch()} />
        ) : items.length === 0 ? (
          <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
            {ada ? "Tidak ada aktivitas yang cocok dengan saringan." : "Belum ada aktivitas tercatat."}
          </p>
        ) : (
          <ul aria-label="Aktivitas akun" className="space-y-2.5">
            {items.map((a) => (
              <BarisAktivitas key={a.id} a={a} />
            ))}
          </ul>
        )}
        <MuatLagi ada={!!q.hasNextPage} memuat={q.isFetchingNextPage} onMuat={() => void q.fetchNextPage()} />
      </div>
    </section>
  );
}

/** Tab "Aktivitas Akun": riwayat masuk (sesi) dan jejak audit milik akun sendiri. */
export function TabAktivitas() {
  return (
    <div className="space-y-5">
      <AktivitasAkunPanel />
      <RiwayatSesiPanel />
    </div>
  );
}
