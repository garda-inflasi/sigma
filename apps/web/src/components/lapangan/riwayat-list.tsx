"use client";

import { AnimatePresence, motion } from "motion/react";
import { useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Download,
  RefreshCw,
  Send,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  BulkActionBar,
  type BulkAction,
} from "@/components/data-table/bulk-action-bar";
import { EmptyState } from "@/components/garda/empty-state";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { Segmented } from "@/components/garda/segmented";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { SpringButton, SpringLink } from "@/components/garda/spring-button";
import { useSession } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import { komoditasByKode, pasarById } from "@/lib/data/reference";
import type { Tone } from "@/lib/design/tokens";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { feedItemFlat, spring } from "@/lib/motion/presets";
import { bulk } from "@/lib/api/endpoints";
import { ringkasSebab } from "@/lib/api/harga";
import { useRiwayatHargaSaya } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import {
  useFieldQueue,
  type EntriLokal,
  type StatusLokal,
} from "@/lib/stores/field-queue-store";
import { cn } from "@/lib/utils";

interface Baris {
  id: string;
  komoditasKode: string;
  pasarNama: string;
  tipe: string;
  harga: number;
  satuan: string;
  waktu: string;
  status: { label: string; tone: Tone; spin?: boolean };
  alasan?: string;
  /** Status di server untuk baris yang sudah terkirim. */
  serverStatus?: string;
  grup: "aksi" | "menunggu" | "selesai";
  lokal?: EntriLokal;
}

const LOKAL: Record<StatusLokal, Baris["status"] & { grup: Baris["grup"] }> = {
  DRAF: { label: "Draf", tone: "netral", grup: "aksi" },
  ANTRE: { label: "Menunggu kirim", tone: "waspada", grup: "menunggu" },
  MENYINKRON: { label: "Mengirim", tone: "data", spin: true, grup: "menunggu" },
  TERSINKRON: {
    label: "Terkirim, menunggu validasi",
    tone: "data",
    grup: "selesai",
  },
  KONFLIK: { label: "Konflik data", tone: "defisit", grup: "aksi" },
  GAGAL: { label: "Gagal kirim", tone: "defisit", grup: "aksi" },
};

/** Langkah berikutnya untuk baris yang butuh tindakan pengguna. */
function langkahBaris(r: Baris): string | null {
  if (r.lokal?.status === "KONFLIK")
    return "Ketuk untuk memilih versi yang benar.";
  if (r.lokal?.status === "DRAF" || r.lokal?.status === "GAGAL")
    return "Tekan Pilih, lalu Kirim ulang.";
  if (r.serverStatus === "DITOLAK") return "Survei ulang, lalu catat harganya.";
  if (r.serverStatus === "DRAF") return "Tekan Pilih, lalu Ajukan ulang.";
  return null;
}

/** Riwayat entri: gabungan antrean perangkat + status server (draf → validasi → publikasi). */
export function RiwayatList() {
  const user = useSession();
  const items = useFieldQueue((s) => s.items);
  const hydrated = useFieldQueue((s) => s.hydrated);
  const { hapus, antrekan, perbarui } = useFieldQueue.getState();
  const { data: server, isLoading } = useRiwayatHargaSaya();
  const qc = useQueryClient();
  const [filter, setFilter] = useState<
    "semua" | "aksi" | "menunggu" | "selesai"
  >("semua");
  const [pilih, setPilih] = useState(false);
  const [sel, setSel] = useState<string[]>([]);
  const [konflik, setKonflik] = useState<EntriLokal | null>(null);

  const rows = useMemo<Baris[]>(() => {
    const lokal = items.map<Baris>((i) => {
      const m = LOKAL[i.status];
      return {
        id: i.localId,
        komoditasKode: i.komoditasKode,
        pasarNama: pasarById[i.pasarId]?.nama ?? i.pasarId,
        tipe: i.tipe,
        harga: i.harga,
        satuan: i.satuan,
        waktu: i.dicatatPada,
        status: m,
        grup: m.grup,
        lokal: i,
      };
    });
    const st: Record<string, Baris["status"]> = {
      DIAJUKAN: { label: "Menunggu validasi", tone: "waspada" },
      DIVALIDASI: { label: "Divalidasi", tone: "data" },
      DIPUBLIKASI: { label: "Dipublikasi", tone: "normal" },
      DITOLAK: { label: "Ditolak, survei ulang", tone: "defisit" },
      DRAF: { label: "Dikembalikan", tone: "netral" },
    };
    const dariServer = (server ?? [])
      .filter((e) => e.petugasId === user.id)
      .map<Baris>((e) => ({
        id: e.id,
        komoditasKode: e.komoditasKode,
        pasarNama: e.pasarNama,
        tipe: e.tipe,
        harga: e.harga,
        satuan: e.satuan,
        waktu: e.dicatatPada,
        status: st[e.status],
        alasan: e.alasanPenolakan,
        serverStatus: e.status,
        grup:
          e.status === "DITOLAK"
            ? "aksi"
            : e.status === "DIAJUKAN"
              ? "menunggu"
              : "selesai",
      }));
    return [...lokal, ...dariServer].sort(
      (a, b) => +new Date(b.waktu) - +new Date(a.waktu),
    );
  }, [items, server, user.id]);

  const tampil = rows.filter((r) => filter === "semua" || r.grup === filter);
  const perHari = useMemo(() => {
    const m = new Map<string, Baris[]>();
    tampil.forEach((r) => {
      const d = new Date(r.waktu);
      const key =
        d.toDateString() === new Date().toDateString()
          ? "Hari ini"
          : formatTanggal(d, "EEEE, d MMMM");
      m.set(key, [...(m.get(key) ?? []), r]);
    });
    return [...m.entries()];
  }, [tampil]);
  const terpilih = rows.filter((r) => sel.includes(r.id));
  const toggle = (id: string) =>
    setSel((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  const actions: BulkAction<Baris>[] = [
    {
      id: "kirim",
      label: "Kirim ulang",
      icon: RefreshCw,
      tone: "data",
      enabled: (r) =>
        r.every(
          (x) => x.lokal && ["DRAF", "GAGAL", "ANTRE"].includes(x.lokal.status),
        ),
      run: (r) => {
        antrekan(r.map((x) => x.id));
        window.dispatchEvent(new CustomEvent("garda:sinkron"));
        toast.success(`${r.length} entri dimasukkan ke antrean kirim`);
      },
    },
    {
      id: "ajukan",
      label: "Ajukan ulang",
      icon: Send,
      tone: "data",
      enabled: (r) => r.every((x) => x.serverStatus === "DRAF"),
      run: async (r) => {
        const res = await bulk(
          "harga",
          "ajukan",
          r.map((x) => x.id),
        );
        await qc.invalidateQueries({ queryKey: qk.harga.all() });
        toast.success(`${res.berhasil} entri diajukan ulang`, {
          description: res.gagal
            ? `${res.gagal} dilewati: ${ringkasSebab(res.rincian)}.`
            : undefined,
        });
      },
    },
    {
      id: "hapus",
      label: "Hapus",
      icon: Trash2,
      tone: "danger",
      enabled: (r) =>
        r.every(
          (x) =>
            x.lokal && ["DRAF", "GAGAL", "KONFLIK"].includes(x.lokal.status),
        ),
      confirm: {
        title: "Hapus entri dari perangkat?",
        description: (n) =>
          `${n} entri yang belum terkirim akan dihapus permanen dari perangkat ini.`,
        actionLabel: "Hapus entri",
      },
      run: (r) => {
        hapus(r.map((x) => x.id));
        toast.success(`${r.length} entri dihapus`);
      },
    },
    {
      id: "ekspor",
      label: "Ekspor",
      icon: Download,
      run: (r) =>
        exportCsv(
          r,
          [
            {
              header: "Komoditas",
              value: (x) => komoditasByKode[x.komoditasKode]?.nama,
            },
            { header: "Pasar", value: (x) => x.pasarNama },
            { header: "Tipe", value: (x) => x.tipe },
            { header: "Harga", value: (x) => x.harga },
            { header: "Waktu", value: (x) => x.waktu },
            { header: "Status", value: (x) => x.status.label },
          ],
          "riwayat-lapangan",
        ),
    },
  ];

  const hitung = (g: Baris["grup"]) => rows.filter((r) => r.grup === g).length;

  return (
    <div className="space-y-4">
      <div className="flex items-end justify-between gap-3">
        <div>
          <h1 className="text-display-sm font-semibold">Riwayat entri</h1>
          <p className="text-sm text-muted-foreground">
            <span className="num">
              {
                rows.filter(
                  (r) =>
                    new Date(r.waktu).toDateString() ===
                    new Date().toDateString(),
                ).length
              }
            </span>{" "}
            entri hari ini
          </p>
        </div>
        <SpringButton
          className="h-11 px-4"
          variant={pilih ? "primary" : "soft"}
          onClick={() => {
            setPilih((p) => !p);
            setSel([]);
          }}
        >
          {pilih ? "Selesai" : "Pilih"}
        </SpringButton>
      </div>
      {hitung("aksi") > 0 && filter !== "aksi" && (
        <button
          type="button"
          onClick={() => setFilter("aksi")}
          className="flex min-h-11 w-full items-center gap-2.5 rounded-xl bg-defisit-soft px-4 py-2.5 text-left text-sm text-defisit-ink dark:bg-defisit/15 dark:text-rose-200"
        >
          <AlertTriangle className="size-4 shrink-0" aria-hidden />
          <span className="min-w-0 flex-1">
            <span className="num font-semibold">{hitung("aksi")}</span> entri
            perlu tindakan Anda
          </span>
          <span className="shrink-0 font-medium underline underline-offset-4">
            Lihat
          </span>
        </button>
      )}
      <Segmented
        layoutId="riwayat-filter"
        size="sm"
        value={filter}
        onChange={setFilter}
        className="w-full [&_button]:h-10"
        options={[
          { value: "semua", label: "Semua" },
          {
            value: "aksi",
            label: (
              <>
                Perlu tindakan{" "}
                {hitung("aksi") > 0 && (
                  <span className="num rounded-full bg-defisit px-1.5 text-[10px] text-white">
                    {hitung("aksi")}
                  </span>
                )}
              </>
            ),
          },
          { value: "menunggu", label: "Menunggu" },
          { value: "selesai", label: "Selesai" },
        ]}
      />
      {pilih && (
        <label className="flex min-h-11 cursor-pointer items-center gap-2 px-1 text-sm text-muted-foreground">
          <Checkbox
            checked={
              tampil.length > 0 && tampil.every((r) => sel.includes(r.id))
            }
            onCheckedChange={(v: boolean | "indeterminate") =>
              setSel(v === true ? tampil.map((r) => r.id) : [])
            }
            className="rounded-md"
          />
          Pilih semua yang tampil
        </label>
      )}

      {!hydrated || isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Shimmer key={i} className="h-16 w-full rounded-xl" />
          ))}
        </div>
      ) : tampil.length === 0 ? (
        <EmptyState
          title="Belum ada entri di sini"
          description="Entri baru tersimpan di perangkat dulu, lalu dikirim otomatis."
          action={<SpringLink href="/lapangan/input">Catat harga</SpringLink>}
          className="glass rounded-xl p-8"
        />
      ) : (
        perHari.map(([hari, list]) => (
          <section key={hari}>
            <h2 className="mb-2 px-1 font-sans text-xs font-medium text-muted-foreground">
              {hari}
            </h2>
            <ul className="glass overflow-hidden rounded-xl">
              <AnimatePresence initial={false}>
                {list.map((r) => (
                  <motion.li
                    key={r.id}
                    layout
                    variants={feedItemFlat}
                    initial="hidden"
                    animate="show"
                    exit="exit"
                    className="border-b border-border last:border-0"
                  >
                    <div
                      onClick={() =>
                        pilih
                          ? toggle(r.id)
                          : r.lokal?.status === "KONFLIK" && setKonflik(r.lokal)
                      }
                      {...(!pilih && r.lokal?.status === "KONFLIK"
                        ? {
                            role: "button",
                            tabIndex: 0,
                            onKeyDown: (e: React.KeyboardEvent) => {
                              if (e.key !== "Enter" && e.key !== " ") return;
                              e.preventDefault();
                              setKonflik(r.lokal ?? null);
                            },
                          }
                        : {})}
                      className={cn(
                        "flex min-h-16 items-start gap-3 px-4 py-3",
                        (pilih || r.lokal?.status === "KONFLIK") &&
                          "cursor-pointer",
                        sel.includes(r.id) &&
                          "bg-data-50/80 dark:bg-data-500/10",
                      )}
                    >
                      <AnimatePresence initial={false}>
                        {pilih && (
                          <motion.span
                            initial={{ width: 0, opacity: 0 }}
                            animate={{ width: 24, opacity: 1 }}
                            exit={{ width: 0, opacity: 0 }}
                            transition={spring.snappy}
                            className="mt-3 inline-flex shrink-0 overflow-hidden"
                          >
                            <Checkbox
                              checked={sel.includes(r.id)}
                              aria-label="Pilih entri"
                              className="rounded-md"
                            />
                          </motion.span>
                        )}
                      </AnimatePresence>
                      <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-navy-50 text-navy-700 dark:bg-white/10 dark:text-navy-200">
                        <KomoditasIcon kode={r.komoditasKode} />
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-baseline justify-between gap-2">
                          <p className="min-w-0 truncate text-sm font-medium">
                            {komoditasByKode[r.komoditasKode]?.nama}
                          </p>
                          <p className="num shrink-0 text-sm font-semibold">
                            {formatRupiah(r.harga)}
                          </p>
                        </div>
                        <p className="truncate text-xs text-muted-foreground">
                          {r.pasarNama}, {formatTanggal(r.waktu, "HH.mm")}
                        </p>
                        <StatusBadge
                          tone={r.status.tone}
                          className="mt-1.5 h-5 max-w-full px-2 text-[11px]"
                          dot={!r.status.spin}
                        >
                          {r.status.spin && (
                            <RefreshCw className="size-3 animate-spin-slow" />
                          )}
                          {r.status.label}
                        </StatusBadge>
                        {r.alasan && (
                          <p className="mt-1 text-xs text-defisit-ink dark:text-rose-300">
                            Alasan: {r.alasan}
                          </p>
                        )}
                        {!pilih && langkahBaris(r) && (
                          <p className="mt-1 text-xs font-medium text-foreground">
                            {langkahBaris(r)}
                          </p>
                        )}
                      </div>
                    </div>
                  </motion.li>
                ))}
              </AnimatePresence>
            </ul>
          </section>
        ))
      )}

      <BulkActionBar
        rows={terpilih}
        actions={actions}
        entityLabel="entri"
        onClear={() => setSel([])}
        placement="inline"
        className="fixed bottom-[calc(max(env(safe-area-inset-bottom),0.75rem)+5.25rem)] left-1/2 max-w-[calc(100vw-2rem)] -translate-x-1/2"
      />

      <Sheet
        open={!!konflik}
        onOpenChange={(o: boolean) => !o && setKonflik(null)}
      >
        <SheetContent side="bottom" className="mx-auto max-w-md pb-safe">
          <SheetHeader>
            <SheetTitle>Selesaikan konflik data</SheetTitle>
            <SheetDescription>
              Petugas lain sudah mengirim harga untuk komoditas & pasar yang
              sama. Pilih versi yang benar.
            </SheetDescription>
          </SheetHeader>
          {konflik?.versiServer && (
            <SheetBody>
              <div className="grid grid-cols-2 gap-3 px-5 py-4">
                <div className="min-w-0 rounded-lg border-2 border-data-400 p-3">
                  <p className="text-xs text-muted-foreground">
                    Versi perangkat Anda
                  </p>
                  <p className="num mt-1 text-xl font-semibold">
                    {formatRupiah(konflik.harga)}
                  </p>
                  <p className="num text-xs text-muted-foreground">
                    {formatTanggal(konflik.dicatatPada, "HH.mm")}
                  </p>
                </div>
                <div className="min-w-0 rounded-lg border border-border p-3">
                  <p className="text-xs break-words text-muted-foreground">
                    Versi server ({konflik.versiServer.petugas})
                  </p>
                  <p className="num mt-1 text-xl font-semibold">
                    {formatRupiah(konflik.versiServer.harga)}
                  </p>
                  <p className="num text-xs text-muted-foreground">
                    {formatTanggal(konflik.versiServer.dicatatPada, "HH.mm")}
                  </p>
                </div>
              </div>
            </SheetBody>
          )}
          <SheetFooter className="flex-col sm:flex-col sm:flex-nowrap">
            <SpringButton
              size="lg"
              onClick={() => {
                if (konflik) {
                  perbarui(konflik.localId, {
                    status: "ANTRE",
                    pertahankan: true,
                    galat: undefined,
                  });
                  window.dispatchEvent(new CustomEvent("garda:sinkron"));
                  toast.success("Versi Anda dikirim sebagai pembanding");
                }
                setKonflik(null);
              }}
            >
              Pertahankan versi saya
            </SpringButton>
            <SpringButton
              size="lg"
              variant="soft"
              onClick={() => {
                if (konflik) {
                  hapus([konflik.localId]);
                  toast("Versi server dipakai, entri perangkat dihapus");
                }
                setKonflik(null);
              }}
            >
              Gunakan versi server
            </SpringButton>
          </SheetFooter>
        </SheetContent>
      </Sheet>
      <p className="px-1 text-center text-xs text-muted-foreground">
        Data lama?{" "}
        <Link
          href="/lapangan/tugas"
          className="inline-block py-2 text-data-700 underline-offset-4 hover:underline dark:text-data-300"
        >
          Unduh data acuan
        </Link>{" "}
        dari halaman tugas.
      </p>
    </div>
  );
}
