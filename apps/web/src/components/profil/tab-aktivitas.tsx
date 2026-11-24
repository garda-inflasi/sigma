"use client";

import {
  History,
  Laptop,
  Loader2,
  RotateCcw,
  ShieldAlert,
  Smartphone,
} from "lucide-react";
import { useId, useState } from "react";
import { FormAlert } from "@/components/form/form-field";
import { PanelPas } from "@/components/garda/halaman-pas";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import { DateField } from "@/components/ui/date-field";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
const UKURAN_FILTER = "h-11 sm:h-9 md:text-[13px]";
const SEMUA_OPSI = "__semua";

const adalahPonsel = (perangkat: string) =>
  /Android|iOS|iPadOS/.test(perangkat);

function Kolom({
  label,
  children,
}: {
  label: string;
  children: (id: string) => React.ReactNode;
}) {
  const id = useId();
  return (
    <div className="grid min-w-0 gap-1">
      <Label htmlFor={id} className="text-[11px] text-muted-foreground">
        {label}
      </Label>
      {children(id)}
    </div>
  );
}

function Memuat({ nama }: { nama: string }) {
  return (
    <div role="status" aria-label={`Memuat ${nama}`} className="space-y-2.5">
      {[0, 1, 2].map((i) => (
        <Shimmer key={i} className="h-14 w-full rounded-xl" />
      ))}
    </div>
  );
}

function Galat({ nama, onUlang }: { nama: string; onUlang: () => void }) {
  return (
    <FormAlert className="flex flex-col items-start gap-3 p-4">
      <p>Gagal memuat {nama}. Coba lagi beberapa saat.</p>
      <Button variant="outline" size="sm" onClick={onUlang}>
        Coba lagi
      </Button>
    </FormAlert>
  );
}

function MuatLagi({
  ada,
  memuat,
  onMuat,
}: {
  ada: boolean;
  memuat: boolean;
  onMuat: () => void;
}) {
  if (!ada) return null;
  return (
    <div className="mt-3 flex justify-center">
      <Button
        variant="outline"
        size="sm"
        onClick={onMuat}
        disabled={memuat}
        className="max-sm:h-11"
      >
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
    <li
      className={cn(
        "flex items-start gap-3 rounded-xl border px-3.5 py-2.5 transition-colors",
        sesi.saatIni
          ? "border-data-300 bg-data-50/50 dark:border-data-500/35 dark:bg-data-500/10"
          : "border-border bg-card hover:border-navy-300/80 dark:hover:border-white/15",
      )}
    >
      <span
        className={cn(
          "mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg",
          sesi.saatIni
            ? "bg-data-100 text-data-700 dark:bg-data-500/20 dark:text-data-300"
            : "bg-secondary text-muted-foreground",
        )}
      >
        <Ikon className="size-4" aria-hidden />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-[13px] font-medium text-navy-900 dark:text-white">
            {sesi.perangkat}
          </p>
          <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
          {sesi.saatIni && <StatusBadge tone="data">Sesi Ini</StatusBadge>}
        </div>
        <p className="num mt-0.5 text-xs text-muted-foreground">
          {sesi.ip ? `IP ${sesi.ip} · ` : ""}Masuk{" "}
          <time dateTime={sesi.mulaiPada}>
            {formatTanggal(sesi.mulaiPada, POLA_WAKTU)}
          </time>
          {sesi.berakhirPada && (
            <>
              {" "}
              · Berakhir{" "}
              <time dateTime={sesi.berakhirPada}>
                {formatTanggal(sesi.berakhirPada, POLA_WAKTU)}
              </time>
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
    <PanelPas
      aria-label="Riwayat masuk"
      judul="Riwayat masuk"
      deskripsi={
        <span className="whitespace-normal">
          Tidak mengenali salah satunya? Ganti kata sandi dan cabut perangkatnya
          di tab Perangkat & Sesi Aktif.
        </span>
      }
      tanpaPadding
      isiClassName="flex flex-col lg:overflow-hidden"
      className="rounded-xl lg:max-h-full lg:min-h-0"
    >
      <div className="shrink-0 border-b border-border bg-secondary/25 px-3.5 py-2.5 dark:bg-white/[0.02]">
        <Kolom label="Status sesi">
          {(id) => (
            <Select
              value={status}
              onValueChange={(v) => setStatus(v as FilterStatusSesi)}
            >
              <SelectTrigger id={id} className={UKURAN_FILTER}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="semua">Semua</SelectItem>
                <SelectItem value="aktif">Aktif</SelectItem>
                <SelectItem value="berakhir">Sudah berakhir</SelectItem>
              </SelectContent>
            </Select>
          )}
        </Kolom>
      </div>
      <div className="scrollbar-thin max-h-[60dvh] overflow-y-auto p-3.5 lg:max-h-none lg:min-h-0 lg:flex-1">
        {q.isPending ? (
          <Memuat nama="riwayat masuk" />
        ) : q.isError ? (
          <Galat nama="riwayat masuk" onUlang={() => void q.refetch()} />
        ) : items.length === 0 ? (
          <p className="rounded-xl border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
            Belum ada sesi yang cocok.
          </p>
        ) : (
          <ul aria-label="Riwayat sesi" className="space-y-2">
            {items.map((s) => (
              <BarisSesi key={s.id} sesi={s} />
            ))}
          </ul>
        )}
        <MuatLagi
          ada={!!q.hasNextPage}
          memuat={q.isFetchingNextPage}
          onMuat={() => void q.fetchNextPage()}
        />
      </div>
    </PanelPas>
  );
}

// ── Aktivitas akun ─────────────────────────────────────────────────

function BarisAktivitas({ a }: { a: Aktivitas }) {
  const hasil = LABEL_HASIL[a.hasil];
  const sasaran = a.sebagai === "sasaran";
  const Ikon = sasaran ? ShieldAlert : History;
  return (
    <li className="flex items-start gap-3 rounded-xl border border-border bg-card px-3.5 py-2.5 transition-colors hover:border-navy-300/80 dark:hover:border-white/15">
      <span
        className={cn(
          "mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg",
          sasaran
            ? "bg-waspada-soft text-waspada-ink dark:bg-waspada/15 dark:text-amber-300"
            : "bg-secondary text-muted-foreground",
        )}
      >
        <Ikon className="size-4" aria-hidden />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-[13px] font-medium text-navy-900 dark:text-white">
            {a.label}
          </p>
          <StatusBadge tone={hasil.tone}>{hasil.label}</StatusBadge>
          {sasaran && <StatusBadge tone="waspada">Oleh orang lain</StatusBadge>}
        </div>
        <p className="num mt-0.5 text-xs text-muted-foreground">
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
  const ubah = (bagian: Partial<FilterAktivitas>) =>
    setFilter((f) => ({ ...f, ...bagian }));

  return (
    <PanelPas
      aria-label="Aktivitas akun"
      judul="Aktivitas akun"
      deskripsi={
        <span className="whitespace-normal">
          Jejak tindakan di akun Anda, termasuk tindakan administrator terhadap
          akun Anda.
        </span>
      }
      aksi={
        ada && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setFilter(FILTER_KOSONG)}
          >
            <RotateCcw aria-hidden />
            Hapus saringan
          </Button>
        )
      }
      tanpaPadding
      isiClassName="flex flex-col lg:overflow-hidden"
      className="rounded-xl lg:max-h-full lg:min-h-0"
    >
      <div
        role="group"
        aria-label="Saring aktivitas"
        className="grid shrink-0 grid-cols-2 gap-2 border-b border-border bg-secondary/25 px-3.5 py-2.5 lg:grid-cols-3 2xl:grid-cols-5 dark:bg-white/[0.02]"
      >
        <Kolom label="Jenis aktivitas">
          {(id) => (
            <Select
              value={filter.aksi ?? SEMUA_OPSI}
              onValueChange={(v) => ubah({ aksi: v === SEMUA_OPSI ? undefined : v })}
            >
              <SelectTrigger id={id} className={UKURAN_FILTER}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={SEMUA_OPSI}>Semua aktivitas</SelectItem>
                {AKSI_FILTER.map((k) => (
                  <SelectItem key={k} value={k}>
                    {labelAksi(k)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </Kolom>
        <Kolom label="Hasil">
          {(id) => (
            <Select
              value={filter.hasil ?? SEMUA_OPSI}
              onValueChange={(v) =>
                ubah({
                  hasil: (v === SEMUA_OPSI ? undefined : v) as FilterAktivitas["hasil"],
                })
              }
            >
              <SelectTrigger id={id} className={UKURAN_FILTER}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={SEMUA_OPSI}>Semua hasil</SelectItem>
                <SelectItem value="BERHASIL">Berhasil</SelectItem>
                <SelectItem value="GAGAL">Gagal</SelectItem>
                <SelectItem value="DITOLAK">Ditolak</SelectItem>
              </SelectContent>
            </Select>
          )}
        </Kolom>
        <Kolom label="Pelaku">
          {(id) => (
            <Select
              value={filter.sebagai ?? "semua"}
              onValueChange={(v) =>
                ubah({ sebagai: v as FilterAktivitas["sebagai"] })
              }
            >
              <SelectTrigger id={id} className={UKURAN_FILTER}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="semua">Saya dan orang lain</SelectItem>
                <SelectItem value="pelaku">Dilakukan saya</SelectItem>
                <SelectItem value="sasaran">Dilakukan orang lain</SelectItem>
              </SelectContent>
            </Select>
          )}
        </Kolom>
        <Kolom label="Dari tanggal">
          {(id) => (
            <DateField
              id={id}
              value={filter.dari ?? ""}
              max={filter.sampai || undefined}
              onValueChange={(v) => ubah({ dari: v || undefined })}
              kelasInput={UKURAN_FILTER}
            />
          )}
        </Kolom>
        <Kolom label="Sampai tanggal">
          {(id) => (
            <DateField
              id={id}
              value={filter.sampai ?? ""}
              min={filter.dari || undefined}
              onValueChange={(v) => ubah({ sampai: v || undefined })}
              kelasInput={UKURAN_FILTER}
            />
          )}
        </Kolom>
      </div>

      <div className="scrollbar-thin max-h-[60dvh] overflow-y-auto p-3.5 lg:max-h-none lg:min-h-0 lg:flex-1">
        {q.isPending ? (
          <Memuat nama="aktivitas akun" />
        ) : q.isError ? (
          <Galat nama="aktivitas akun" onUlang={() => void q.refetch()} />
        ) : items.length === 0 ? (
          <p className="rounded-xl border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
            {ada
              ? "Tidak ada aktivitas yang cocok dengan saringan."
              : "Belum ada aktivitas tercatat."}
          </p>
        ) : (
          <ul aria-label="Aktivitas akun" className="space-y-2">
            {items.map((a) => (
              <BarisAktivitas key={a.id} a={a} />
            ))}
          </ul>
        )}
        <MuatLagi
          ada={!!q.hasNextPage}
          memuat={q.isFetchingNextPage}
          onMuat={() => void q.fetchNextPage()}
        />
      </div>
    </PanelPas>
  );
}

/** Tab "Aktivitas Akun": riwayat masuk (sesi) dan jejak audit milik akun sendiri. */
export function TabAktivitas() {
  return (
    <div className="flex min-w-0 flex-col gap-3 lg:grid lg:min-h-0 lg:flex-1 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] lg:items-start">
      <AktivitasAkunPanel />
      <RiwayatSesiPanel />
    </div>
  );
}
