"use client";

import { AnimatePresence, motion } from "motion/react";
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  ChevronsUpDown,
  Download,
  Factory,
  Inbox,
  Loader2,
  RotateCcw,
  Search,
  Truck,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import {
  BulkActionBar,
  type BulkAction,
} from "@/components/data-table/bulk-action-bar";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { TableSkeleton } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { urgensiLot } from "@/lib/api/cold-storage";
import {
  COLD_STORAGE_MAX_BULK_IDS,
  type ColdStorageParams,
  type ColdStorageSort,
} from "@/lib/api/cold-storage-query";
import { api } from "@/lib/api/endpoints";
import { exportCsv } from "@/lib/csv";
import type { ColdStorageView, StokColdStorage } from "@/lib/data/types";
import { formatAngka, formatTanggal } from "@/lib/format";
import { fadeIn } from "@/lib/motion/presets";
import { useColdStorageBulk } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { Pagination } from "./pagination";
import { RiskBar, ShelfLifeBadge, UrgencyStripe } from "./lot-cells";

const SEMUA = "__semua";
const UKURAN_HALAMAN = [10, 25, 50];

const KOLOM: { key: ColdStorageSort; label: string; kanan?: boolean }[] = [
  { key: "komoditas", label: "Komoditas" },
  { key: "fasilitas", label: "Fasilitas" },
  { key: "volume", label: "Volume", kanan: true },
  { key: "masuk", label: "Masuk" },
  { key: "sisa_masa_simpan", label: "Sisa masa simpan" },
  { key: "risiko", label: "Risiko food loss" },
  { key: "pemilik", label: "Pemilik" },
];

/** Angka dan tanggal lebih berguna bila klik pertama menampilkan nilai terbesar/terbaru. */
export const URUTAN_AWAL_MENURUN: Record<ColdStorageSort, boolean> = {
  sisa_masa_simpan: false,
  komoditas: false,
  fasilitas: false,
  volume: true,
  masuk: true,
  risiko: true,
  pemilik: false,
};

const jumlahVolume = (rows: StokColdStorage[]) =>
  rows.reduce((a, r) => a + r.volumeTon, 0);

/** Sel tabel dengan tinggi baris tetap; baris tabel tidak boleh membungkus agar tetap rapat. */
const SEL = "h-14 border-b border-border/50 px-3 align-middle";

interface Props {
  view: ColdStorageView | undefined;
  /** Belum ada data sama sekali (muatan pertama). */
  isLoading: boolean;
  /** Sedang memuat ulang; data lama tetap tampil. */
  isFetching: boolean;
  error: Error | null;
  onRetry: () => void;
  params: ColdStorageParams;
  /** Mengubah kriteria; pemanggil mengembalikan ke halaman 1 bila yang berubah bukan halaman. */
  onParams: (patch: Partial<ColdStorageParams>) => void;
  search: string;
  onSearch: (v: string) => void;
  onResetFilters: () => void;
}

/**
 * Tabel stok tersimpan: data, urutan, pencarian, dan halaman ditentukan server;
 * pilihan baris bertahan lintas halaman.
 *
 * Tata letak mengikuti lebar panel (container query), bukan lebar layar, karena
 * sidebar mengubah ruang yang tersedia: tabel dari 56rem ke atas, di bawahnya
 * daftar kartu (dua kolom dari 42rem).
 */
export function StockTable({
  view,
  isLoading,
  isFetching,
  error,
  onRetry,
  params,
  onParams,
  search,
  onSearch,
  onResetFilters,
}: Props) {
  const { mutateAsync: jalankanBulk } = useColdStorageBulk();
  const [selected, setSelected] = useState<Record<string, StokColdStorage>>({});
  const [semuaHasil, setSemuaHasil] = useState(false);
  const [memilihSemua, setMemilihSemua] = useState(false);
  const lastIndex = useRef<number | null>(null);

  const rows = view?.stok ?? [];
  const halaman = view?.halaman;
  const total = halaman?.total ?? 0;
  const ambang = view?.ringkasan.ambangRisikoTinggi ?? 0.6;
  const selectedRows = useMemo(() => Object.values(selected), [selected]);
  const adaFilter = Boolean(params.q || params.fasilitasId);

  const semuaDiHalaman = rows.length > 0 && rows.every((r) => selected[r.id]);
  const sebagianDiHalaman = rows.some((r) => selected[r.id]);

  const clear = useCallback(() => {
    setSelected({});
    setSemuaHasil(false);
    lastIndex.current = null;
  }, []);

  // Esc membatalkan pilihan (pola yang sama dengan tabel Garda lainnya)
  useEffect(() => {
    if (!selectedRows.length) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && clear();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selectedRows.length, clear]);

  function toggleRow(row: StokColdStorage, index: number, shift: boolean) {
    setSemuaHasil(false);
    setSelected((s) => {
      const next = { ...s };
      if (shift && lastIndex.current !== null) {
        const [a, b] = [lastIndex.current, index].sort((x, y) => x - y);
        const nyalakan = !s[row.id];
        rows
          .slice(a, b + 1)
          .forEach((r) => (nyalakan ? (next[r.id] = r) : delete next[r.id]));
      } else if (next[row.id]) {
        delete next[row.id];
      } else {
        next[row.id] = row;
      }
      return next;
    });
    lastIndex.current = index;
  }

  function toggleHalaman(nyalakan: boolean) {
    setSemuaHasil(false);
    setSelected((s) => {
      const next = { ...s };
      rows.forEach((r) => (nyalakan ? (next[r.id] = r) : delete next[r.id]));
      return next;
    });
  }

  async function pilihSemuaHasil() {
    setMemilihSemua(true);
    try {
      const semua = await api.coldStorage.semua({
        fasilitasId: params.fasilitasId,
        q: params.q,
        sort: params.sort,
        desc: params.desc,
      });
      setSelected((s) => ({
        ...s,
        ...Object.fromEntries(semua.map((r) => [r.id, r])),
      }));
      setSemuaHasil(true);
    } catch (err) {
      toast.error("Gagal memilih seluruh hasil", {
        description:
          err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    } finally {
      setMemilihSemua(false);
    }
  }

  function urutkan(key: ColdStorageSort) {
    if (params.sort === key) onParams({ desc: !params.desc });
    else onParams({ sort: key, desc: URUTAN_AWAL_MENURUN[key] });
  }

  const aksi = useMemo<BulkAction<StokColdStorage>[]>(
    () => [
      {
        id: "distribusi",
        label: "Tentukan prioritas penyaluran",
        icon: Truck,
        perm: "logistik.kelola",
        tone: "data",
        run: (r) =>
          jalankanBulk({
            action: "rencanakan_distribusi",
            rows: r,
            label: "Prioritas penyaluran dicatat",
          }),
      },
      {
        id: "olahan",
        label: "Rekomendasikan olahan",
        icon: Factory,
        perm: "coldstorage.kelola",
        run: (r) =>
          jalankanBulk({
            action: "rekomendasi_olahan",
            rows: r,
            label: "Rekomendasi olahan dicatat",
          }),
      },
      {
        id: "ekspor",
        label: "Ekspor CSV",
        icon: Download,
        run: (r) =>
          exportCsv(
            r,
            [
              { header: "Komoditas", value: (x) => x.komoditasNama },
              { header: "Fasilitas", value: (x) => x.fasilitasNama },
              { header: "Volume (ton)", value: (x) => x.volumeTon },
              { header: "Masuk", value: (x) => x.masuk.slice(0, 10) },
              {
                header: "Sisa masa simpan (hari)",
                value: (x) => x.sisaShelfLifeHari,
              },
              {
                header: "Risiko food loss (%)",
                value: (x) => Math.round(x.risikoFoodLoss * 100),
              },
              { header: "Pemilik", value: (x) => x.pemilik },
            ],
            "stok-cold-storage",
          ),
      },
    ],
    [jalankanBulk],
  );

  const dari =
    halaman && total > 0 ? (halaman.page - 1) * halaman.limit + 1 : 0;
  const sampai = halaman ? Math.min(total, dari + rows.length - 1) : 0;
  const sibuk = isFetching && !isLoading;
  const volumeHalaman = jumlahVolume(rows);
  const volumeHasil = halaman?.volumeHasilTon ?? 0;
  const pilihSemuaHalaman = (
    <Checkbox
      aria-label="Pilih semua lot di halaman ini"
      checked={
        semuaDiHalaman ? true : sebagianDiHalaman ? "indeterminate" : false
      }
      onCheckedChange={(v: boolean | "indeterminate") =>
        toggleHalaman(v === true)
      }
      className="rounded-md"
    />
  );

  return (
    <>
      {/* Wadah container query; sengaja terpisah dari panel kaca dan BulkActionBar: keduanya
          (backdrop-filter / containment) menjadi containing block bagi elemen `fixed`. */}
      <div className="@container">
        <div
          className="glass relative overflow-hidden rounded-xl"
          aria-busy={sibuk}
        >
          {/* Penanda muat ulang (pindah halaman, urutan, atau filter) */}
          <AnimatePresence>
            {sibuk && (
              <motion.div
                variants={fadeIn}
                initial="hidden"
                animate="show"
                exit="exit"
                aria-hidden
                className="absolute inset-x-0 top-0 z-20 h-0.5 overflow-hidden bg-data-500/15"
              >
                <span className="block h-full w-1/3 animate-pulse rounded-full bg-data-500" />
              </motion.div>
            )}
          </AnimatePresence>

          {/* Toolbar: pencarian + filter fasilitas */}
          <div className="flex flex-col gap-3 border-b border-border/70 p-4 @2xl:flex-row @2xl:items-center">
            <label className="relative flex-1 @2xl:max-w-sm">
              <span className="sr-only">Cari komoditas atau pemilik</span>
              <Search
                className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
                aria-hidden
              />
              <input
                type="search"
                value={search}
                onChange={(e) => onSearch(e.target.value)}
                maxLength={100}
                placeholder="Cari komoditas atau pemilik"
                className="h-10 w-full rounded-md border border-input bg-white/70 pr-3 pl-9 text-sm outline-none transition-shadow placeholder:text-muted-foreground focus:shadow-glow-data dark:bg-white/5"
              />
            </label>

            <div className="flex flex-wrap items-center gap-2">
              <Select
                value={params.fasilitasId ?? SEMUA}
                onValueChange={(v: string) =>
                  onParams({ fasilitasId: v === SEMUA ? undefined : v })
                }
              >
                <SelectTrigger
                  aria-label="Filter fasilitas"
                  className="h-10 min-w-36 rounded-md bg-white/70 dark:bg-white/5"
                >
                  <SelectValue placeholder="Fasilitas" />
                </SelectTrigger>
                <SelectContent className="rounded-xl">
                  <SelectItem value={SEMUA}>Fasilitas: semua</SelectItem>
                  {(view?.fasilitas ?? [])
                    .filter((f) => f.status !== "RENCANA")
                    .map((f) => (
                      <SelectItem key={f.id} value={f.id}>
                        {f.nama}
                      </SelectItem>
                    ))}
                </SelectContent>
              </Select>

              {/* Tanpa header yang dapat diklik (tampilan daftar), urutan dipilih lewat kontrol ini */}
              <div className="flex items-center gap-1 @4xl:hidden">
                <Select
                  value={params.sort ?? "sisa_masa_simpan"}
                  onValueChange={(v: string) => urutkan(v as ColdStorageSort)}
                >
                  <SelectTrigger
                    aria-label="Urutkan menurut"
                    className="h-10 min-w-36 rounded-md bg-white/70 dark:bg-white/5"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className="rounded-xl">
                    {KOLOM.map((k) => (
                      <SelectItem key={k.key} value={k.key}>
                        Urutkan: {k.label.toLowerCase()}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <button
                  onClick={() => onParams({ desc: !params.desc })}
                  aria-label={
                    params.desc
                      ? "Urutan menurun, ubah ke menaik"
                      : "Urutan menaik, ubah ke menurun"
                  }
                  className="grid size-10 place-items-center rounded-md border border-input bg-white/70 text-muted-foreground hover:text-foreground dark:bg-white/5"
                >
                  {params.desc ? (
                    <ArrowDown className="size-4" />
                  ) : (
                    <ArrowUp className="size-4" />
                  )}
                </button>
              </div>

              {adaFilter && (
                <button
                  onClick={onResetFilters}
                  className="inline-flex h-10 items-center gap-1.5 rounded-md px-2.5 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground"
                >
                  <RotateCcw className="size-3.5" /> Hapus filter
                </button>
              )}
            </div>
          </div>

          {/* Ringkasan pilihan + pilih seluruh hasil lintas halaman */}
          <AnimatePresence initial={false}>
            {selectedRows.length > 0 && (
              <motion.div
                variants={fadeIn}
                initial="hidden"
                animate="show"
                exit="exit"
                className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 border-b border-border/70 bg-data-50/70 px-4 py-2 text-sm text-data-800 dark:bg-data-500/10 dark:text-data-200"
              >
                {semuaHasil ? (
                  <span>
                    Seluruh{" "}
                    <span className="num font-semibold">
                      {total.toLocaleString("id-ID")}
                    </span>{" "}
                    lot terpilih
                  </span>
                ) : (
                  <span>
                    <span className="num font-semibold">
                      {selectedRows.length.toLocaleString("id-ID")}
                    </span>{" "}
                    lot terpilih
                  </span>
                )}
                <span className="num">
                  {formatAngka(jumlahVolume(selectedRows), 1)} ton
                </span>
                {!semuaHasil &&
                  semuaDiHalaman &&
                  total > rows.length &&
                  total <= COLD_STORAGE_MAX_BULK_IDS && (
                    <button
                      onClick={pilihSemuaHasil}
                      disabled={memilihSemua}
                      className="inline-flex items-center gap-1.5 font-semibold underline-offset-4 hover:underline disabled:opacity-60"
                    >
                      {memilihSemua && (
                        <Loader2 className="size-3.5 animate-spin" />
                      )}
                      Pilih semua {total.toLocaleString("id-ID")} lot
                    </button>
                  )}
                <button
                  onClick={clear}
                  className="font-semibold underline-offset-4 hover:underline"
                >
                  Batalkan pilihan
                </button>
              </motion.div>
            )}
          </AnimatePresence>

          {/* Galat memuat ulang saat data lama masih ada */}
          {error && view && (
            <div
              role="alert"
              className="flex flex-wrap items-center justify-center gap-x-3 border-b border-border/70 bg-defisit-soft/60 px-4 py-2 text-sm text-defisit-ink dark:bg-defisit/10 dark:text-rose-300"
            >
              <span>
                Gagal memperbarui data; menampilkan data terakhir.{" "}
                {error.message}
              </span>
              <button
                onClick={onRetry}
                className="font-semibold underline-offset-4 hover:underline"
              >
                Coba lagi
              </button>
            </div>
          )}

          {isLoading ? (
            <TableSkeleton rows={8} cols={6} />
          ) : error && !view ? (
            <div
              role="alert"
              className="grid place-items-center gap-2 px-6 py-16 text-center"
            >
              <span className="grid size-12 place-items-center rounded-xl bg-defisit-soft text-defisit-ink dark:bg-defisit/15">
                <AlertTriangle className="size-5" />
              </span>
              <p className="font-display font-semibold">
                Stok belum dapat dimuat
              </p>
              <p className="max-w-sm text-sm text-muted-foreground">
                {error.message}
              </p>
              <SpringButton variant="soft" size="sm" onClick={onRetry}>
                <RotateCcw /> Coba lagi
              </SpringButton>
            </div>
          ) : rows.length === 0 ? (
            <div className="grid place-items-center gap-2 px-6 py-16 text-center">
              <span className="grid size-12 place-items-center rounded-xl bg-mist-100 text-muted-foreground dark:bg-white/5">
                <Inbox className="size-5" />
              </span>
              <p className="font-display font-semibold">
                {adaFilter
                  ? "Tidak ada lot yang cocok"
                  : "Belum ada stok tersimpan"}
              </p>
              <p className="max-w-sm text-sm text-muted-foreground">
                {adaFilter
                  ? "Ubah kata kunci atau pilih fasilitas lain untuk melihat stok."
                  : "Stok akan tampil di sini setelah komoditas masuk ke cold storage."}
              </p>
              {adaFilter && (
                <SpringButton variant="soft" size="sm" onClick={onResetFilters}>
                  <RotateCcw /> Hapus filter
                </SpringButton>
              )}
            </div>
          ) : (
            <div
              className={cn(
                "transition-opacity duration-200",
                sibuk && "opacity-60",
              )}
            >
              {/* Panel ≥ 56rem: tabel */}
              <div className="scrollbar-thin hidden overflow-x-auto @4xl:block">
                <table className="w-full min-w-[880px] border-separate border-spacing-0 text-sm">
                  <caption className="sr-only">
                    Stok tersimpan di cold storage
                  </caption>
                  <thead className="bg-white/50 dark:bg-white/[0.02]">
                    <tr>
                      <th
                        scope="col"
                        className="h-11 w-11 border-b border-border/70 pl-5 text-left"
                      >
                        {pilihSemuaHalaman}
                      </th>
                      {KOLOM.map((k) => {
                        const aktif = params.sort === k.key;
                        return (
                          <th
                            key={k.key}
                            scope="col"
                            aria-sort={
                              aktif
                                ? params.desc
                                  ? "descending"
                                  : "ascending"
                                : "none"
                            }
                            className={cn(
                              "h-11 border-b border-border/70 px-3 text-xs font-medium whitespace-nowrap text-muted-foreground",
                              k.kanan ? "pr-8 text-right" : "text-left",
                            )}
                          >
                            <button
                              onClick={() => urutkan(k.key)}
                              className={cn(
                                "inline-flex items-center gap-1 hover:text-foreground",
                                aktif && "text-foreground",
                              )}
                            >
                              {k.label}
                              {aktif ? (
                                params.desc ? (
                                  <ArrowDown className="size-3" aria-hidden />
                                ) : (
                                  <ArrowUp className="size-3" aria-hidden />
                                )
                              ) : (
                                <ChevronsUpDown
                                  className="size-3 opacity-40"
                                  aria-hidden
                                />
                              )}
                            </button>
                          </th>
                        );
                      })}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((r, i) => {
                      const dipilih = Boolean(selected[r.id]);
                      return (
                        <tr
                          key={r.id}
                          data-state={dipilih ? "selected" : undefined}
                          className={cn(
                            "transition-colors",
                            dipilih
                              ? "bg-data-50/80 dark:bg-data-500/10"
                              : "hover:bg-white/60 dark:hover:bg-white/[0.03]",
                          )}
                        >
                          <td className={cn(SEL, "relative w-11 pl-5")}>
                            <UrgencyStripe
                              urgensi={urgensiLot(r.sisaShelfLifeHari)}
                            />
                            <span
                              onClick={(e) => {
                                e.stopPropagation();
                                toggleRow(r, i, e.shiftKey);
                              }}
                              className="inline-flex"
                            >
                              <Checkbox
                                checked={dipilih}
                                aria-label={`Pilih ${r.komoditasNama} di ${r.fasilitasNama}`}
                                className="rounded-md"
                              />
                            </span>
                          </td>
                          <td className={cn(SEL, "whitespace-nowrap")}>
                            <span className="flex items-center gap-2 font-medium">
                              <KomoditasIcon
                                kode={r.komoditasKode}
                                ikon={r.ikon}
                                className="text-muted-foreground"
                              />
                              {r.komoditasNama}
                            </span>
                          </td>
                          <td className={cn(SEL, "whitespace-nowrap")}>
                            {r.fasilitasNama}
                          </td>
                          <td
                            className={cn(
                              SEL,
                              "num pr-8 text-right whitespace-nowrap",
                            )}
                          >
                            {formatAngka(r.volumeTon, 1)} ton
                          </td>
                          <td className={cn(SEL, "whitespace-nowrap")}>
                            {formatTanggal(r.masuk)}
                          </td>
                          <td className={SEL}>
                            <ShelfLifeBadge hari={r.sisaShelfLifeHari} />
                          </td>
                          <td className={SEL}>
                            <RiskBar
                              risiko={r.risikoFoodLoss}
                              ambang={ambang}
                            />
                          </td>
                          <td
                            className={cn(
                              SEL,
                              "whitespace-nowrap text-muted-foreground",
                            )}
                          >
                            {r.pemilik}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                  <tfoot>
                    <tr className="bg-mist-50/80 dark:bg-white/[0.03]">
                      <th
                        scope="row"
                        colSpan={3}
                        className="px-5 py-3 text-left text-sm font-medium"
                      >
                        Total volume ditampilkan
                      </th>
                      <td className="num px-3 py-3 pr-8 text-right font-semibold whitespace-nowrap">
                        {formatAngka(volumeHalaman, 1)} ton
                      </td>
                      <td
                        colSpan={4}
                        className="px-3 py-3 text-xs text-muted-foreground"
                      >
                        dari{" "}
                        <span className="num text-foreground">
                          {formatAngka(volumeHasil, 1)} ton
                        </span>{" "}
                        pada{" "}
                        <span className="num text-foreground">
                          {total.toLocaleString("id-ID")}
                        </span>{" "}
                        lot {adaFilter ? "hasil filter" : "tersimpan"}
                      </td>
                    </tr>
                  </tfoot>
                </table>
              </div>

              {/* Panel < 56rem: daftar kartu (dua kolom dari 42rem) */}
              <div className="@4xl:hidden">
                <label className="flex items-center gap-3 border-b border-border/50 bg-white/50 px-4 py-2 dark:bg-white/[0.02]">
                  {pilihSemuaHalaman}
                  <span className="text-xs text-muted-foreground">
                    Pilih semua di halaman ini
                  </span>
                </label>
                <ul className="grid @2xl:grid-cols-2">
                  {rows.map((r, i) => {
                    const dipilih = Boolean(selected[r.id]);
                    return (
                      <li
                        key={r.id}
                        className={cn(
                          "relative flex gap-3 border-b border-border/50 py-3.5 pr-4 pl-4 @2xl:even:border-l",
                          dipilih && "bg-data-50/80 dark:bg-data-500/10",
                        )}
                      >
                        <UrgencyStripe
                          urgensi={urgensiLot(r.sisaShelfLifeHari)}
                        />
                        <span
                          onClick={(e) => toggleRow(r, i, e.shiftKey)}
                          className="mt-0.5 inline-flex h-fit"
                        >
                          <Checkbox
                            checked={dipilih}
                            aria-label={`Pilih ${r.komoditasNama} di ${r.fasilitasNama}`}
                            className="rounded-md"
                          />
                        </span>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-start justify-between gap-2">
                            <p className="flex min-w-0 items-center gap-2 font-medium">
                              <KomoditasIcon
                                kode={r.komoditasKode}
                                ikon={r.ikon}
                                className="shrink-0 text-muted-foreground"
                              />
                              <span className="truncate">
                                {r.komoditasNama}
                              </span>
                            </p>
                            <ShelfLifeBadge hari={r.sisaShelfLifeHari} />
                          </div>
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            {r.fasilitasNama}
                          </p>
                          <div className="mt-2.5 flex items-center justify-between gap-3">
                            <span className="num text-sm font-medium">
                              {formatAngka(r.volumeTon, 1)} ton
                            </span>
                            <RiskBar
                              risiko={r.risikoFoodLoss}
                              ambang={ambang}
                            />
                          </div>
                          <p className="mt-1.5 text-xs text-muted-foreground">
                            Masuk {formatTanggal(r.masuk)} · {r.pemilik}
                          </p>
                        </div>
                      </li>
                    );
                  })}
                </ul>
                <div className="flex flex-wrap items-baseline justify-between gap-x-3 bg-mist-50/80 px-4 py-3 text-sm dark:bg-white/[0.03]">
                  <span className="font-medium">Total volume ditampilkan</span>
                  <span className="num font-semibold">
                    {formatAngka(volumeHalaman, 1)} ton
                  </span>
                  <span className="w-full text-xs text-muted-foreground">
                    dari{" "}
                    <span className="num text-foreground">
                      {formatAngka(volumeHasil, 1)} ton
                    </span>{" "}
                    pada{" "}
                    <span className="num text-foreground">
                      {total.toLocaleString("id-ID")}
                    </span>{" "}
                    lot
                  </span>
                </div>
              </div>
            </div>
          )}

          {/* Footer: posisi halaman + paginasi */}
          {view && total > 0 && halaman && (
            <div className="flex flex-col items-center justify-between gap-3 border-t border-border/70 px-5 py-3 text-sm text-muted-foreground @xl:flex-row">
              <span className="text-center @xl:text-left">
                Menampilkan{" "}
                <span className="num text-foreground">
                  {dari.toLocaleString("id-ID")}–
                  {sampai.toLocaleString("id-ID")}
                </span>{" "}
                dari{" "}
                <span className="num text-foreground">
                  {total.toLocaleString("id-ID")}
                </span>{" "}
                lot
                {selectedRows.length > 0 && (
                  <span className="ml-2 rounded-full bg-data-50 px-2 py-0.5 text-xs text-data-700 dark:bg-data-500/15 dark:text-data-300">
                    <span className="num">
                      {selectedRows.length.toLocaleString("id-ID")}
                    </span>{" "}
                    terpilih
                  </span>
                )}
              </span>
              <div className="flex flex-wrap items-center justify-center gap-3">
                <Select
                  value={String(halaman.limit)}
                  onValueChange={(v: string) => onParams({ limit: Number(v) })}
                >
                  <SelectTrigger
                    aria-label="Jumlah baris per halaman"
                    className="h-8 w-[136px] rounded-md"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {UKURAN_HALAMAN.map((n) => (
                      <SelectItem key={n} value={String(n)}>
                        {n} / halaman
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Pagination
                  page={halaman.page}
                  totalPages={halaman.totalPages}
                  onChange={(p) => onParams({ page: p })}
                  disabled={sibuk}
                />
              </div>
            </div>
          )}
        </div>
      </div>

      <BulkActionBar
        rows={selectedRows}
        actions={aksi}
        entityLabel="lot"
        onClear={clear}
      />
    </>
  );
}
