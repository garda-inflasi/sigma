"use client";

import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import { sortableKeyboardCoordinates } from "@dnd-kit/sortable";
import {
  ChevronsRight,
  Download,
  LayoutGrid,
  ListChecks,
  PanelRight,
  Plus,
  Rows3,
  ShieldAlert,
  TriangleAlert,
  UserPlus,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { usePathname, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import {
  BulkActionBar,
  type BulkAction,
} from "@/components/data-table/bulk-action-bar";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import {
  HalamanPas,
  IsiPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { Segmented } from "@/components/garda/segmented";
import { ViewTabs } from "@/components/garda/view-tabs";
import { Button } from "@/components/ui/button";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/endpoints";
import {
  alurUntukTahap,
  bisaMemutus,
  ringkasRantai,
} from "@/lib/api/intervensi";
import { Can, useSession } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import { komoditasByKode } from "@/lib/data/reference";
import { namaWilayah } from "@/lib/data/generators";
import { SIKLUS_TAHAP, SIKLUS_URUTAN } from "@/lib/data/siklus";
import type { KartuIntervensi } from "@/lib/data/types";
import { siklusMeta, type Siklus } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import {
  useAjukanPersetujuan,
  useAksiKartu,
  useAlurPersetujuan,
  useIntervensi,
  usePersetujuan,
  useTahapIntervensi,
} from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { can, type Permission } from "@/lib/rbac/permissions";
import { useKanban } from "@/lib/stores/kanban-store";
import { labelKode } from "@/lib/kode";
import { cn } from "@/lib/utils";
import { KanbanCardView } from "./kanban-card";
import { KanbanColumn } from "./kanban-column";
import { KartuDetailSheet } from "./kartu-detail-sheet";
import { AlurPersetujuanPanel, PersetujuanPanel } from "./persetujuan-panel";
import { TambahIntervensiDialog } from "./tambah-intervensi-dialog";
import { kartuSelesai, kartuTerlambat, labelTenggat } from "./tenggat";
import type { ColumnDef } from "@tanstack/react-table";

const TAB = ["papan", "tabel", "persetujuan"] as const;
type Saring = "terlambat" | "tinggi" | "eskalasi";

const COCOK: Record<Saring, (k: KartuIntervensi) => boolean> = {
  terlambat: kartuTerlambat,
  tinggi: (k) => !kartuSelesai(k) && k.prioritas === "TINGGI",
  eskalasi: (k) => !kartuSelesai(k) && k.dieskalasi,
};

const IZIN_LABEL: Partial<Record<Permission, string>> = {
  "intervensi.setujui": "Butuh persetujuan Dewan Pengarah",
  "intervensi.setujui_fiskal": "Butuh persetujuan Kanwil DJPb",
  "intervensi.kelola": "Butuh peran pengelola intervensi",
};

const SIKLUS_PILL: Record<Siklus, string> = {
  normal: "bg-normal",
  tekanan: "bg-defisit",
  surplus: "bg-surplus",
};

/** Kanban intervensi 3 siklus — warna adaptif per siklus, drag-drop sadar izin, aksi massal. */
export function KanbanBoard() {
  const sesi = useSession();
  const pathname = usePathname();
  const params = useSearchParams();
  const siklusUrl = params.get("siklus") as Siklus | null;
  const siklus: Siklus =
    siklusUrl && SIKLUS_URUTAN.includes(siklusUrl) ? siklusUrl : "tekanan";
  const [view] = useTabHalaman(TAB, "papan");
  // Tab dan siklus berbagi query string. History API dipakai karena router.replace yang masih tertunda
  // dapat menimpa perubahan parameter lain bila keduanya diklik beruntun.
  const ubahUrl = (ubah: (q: URLSearchParams) => void) => {
    const q = new URLSearchParams(window.location.search);
    ubah(q);
    const s = q.toString();
    window.history.replaceState(null, "", s ? `${pathname}?${s}` : pathname);
  };
  const setView = (t: (typeof TAB)[number]) =>
    ubahUrl((q) => (t === "papan" ? q.delete("tab") : q.set("tab", t)));
  const [saring, setSaring] = useState<Saring>();
  const { data, isLoading } = useIntervensi();
  const tahapQ = useTahapIntervensi();
  const alurQ = useAlurPersetujuan(can(sesi, "intervensi.lihat"));
  const antrean = usePersetujuan(can(sesi, "intervensi.lihat"));
  const [detailId, setDetailId] = useState<string | null>(null);
  const [tambahBuka, setTambahBuka] = useState(false);
  const qc = useQueryClient();
  const aksiKartu = useAksiKartu();
  const ajukan = useAjukanPersetujuan();
  const k = useKanban();
  const [activeId, setActiveId] = useState<string | null>(null);
  const [overTahap, setOverTahap] = useState<string | null>(null);
  const snap = useRef<ReturnType<typeof k.snapshot> | null>(null);
  const asal = useRef<string | null>(null);

  // Data server menggantikan papan, kecuali saat kartu sedang diseret atau disimpan,
  // agar polling tidak mengembalikan kartu yang baru dipindahkan.
  const sibuk = useRef(false);
  // Kolom berasal dari GET /v1/intervensi/tahap; SIKLUS_TAHAP hanya cadangan bila endpoint gagal.
  const tahapPerSiklus = tahapQ.data ?? SIKLUS_TAHAP;
  const semuaTahap = useMemo(
    () => Object.values(tahapPerSiklus).flat(),
    [tahapPerSiklus],
  );
  const tahapSiap = !tahapQ.isLoading;
  useEffect(() => {
    if (data && tahapSiap && !sibuk.current)
      useKanban.getState().sync(
        data,
        semuaTahap.map((t) => t.id),
      );
  }, [data, tahapSiap, semuaTahap]);

  const tahap = tahapPerSiklus[siklus] ?? [];
  const izinMasuk = (tahapId: string): Permission =>
    semuaTahap.find((t) => t.id === tahapId)?.izinMasuk ?? "intervensi.kelola";
  /** Alasan kolom terkunci; memakai rantai persetujuan dari server bila tahap punya alur. */
  const labelIzin = (tahapId: string) => {
    const alur = alurQ.data ? alurUntukTahap(alurQ.data, tahapId) : undefined;
    return alur
      ? `Butuh persetujuan berjenjang: ${ringkasRantai(alur)}`
      : IZIN_LABEL[izinMasuk(tahapId)];
  };
  const bolehMasuk = (tahapId: string) => can(sesi, izinMasuk(tahapId));
  const bisaSeret =
    can(sesi, "intervensi.kelola") ||
    can(sesi, "intervensi.setujui") ||
    can(sesi, "intervensi.setujui_fiskal");

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  const findTahap = (id: string) => (k.order[id] ? id : k.cards[id]?.tahap);

  function gantiSiklus(s: Siklus) {
    k.clearSelection();
    ubahUrl((q) => q.set("siklus", s));
  }

  function gantiSaring(s: Saring) {
    k.clearSelection();
    setSaring((v) => (v === s ? undefined : s));
    if (view === "persetujuan") setView("papan");
  }

  function onDragStart({ active }: DragStartEvent) {
    sibuk.current = true;
    setActiveId(String(active.id));
    snap.current = k.snapshot();
    asal.current = k.cards[String(active.id)]?.tahap ?? null;
  }

  function onDragOver({ active, over }: DragOverEvent) {
    if (!over) return setOverTahap(null);
    const from = findTahap(String(active.id));
    const to = findTahap(String(over.id));
    setOverTahap(to ?? null);
    if (!from || !to || from === to || !bolehMasuk(to)) return;
    const idx = k.order[to].indexOf(String(over.id));
    k.move(String(active.id), to, idx >= 0 ? idx : k.order[to].length);
  }

  async function onDragEnd(event: DragEndEvent) {
    try {
      await simpanDrag(event);
    } finally {
      sibuk.current = false;
    }
  }

  async function simpanDrag({ active, over }: DragEndEvent) {
    const id = String(active.id);
    setActiveId(null);
    setOverTahap(null);
    if (!over) return snap.current && k.restore(snap.current);
    const target = findTahap(String(over.id));
    if (target && !bolehMasuk(target)) {
      snap.current && k.restore(snap.current);
      // Pemegang kelola dapat meminta persetujuan alih-alih memindahkan langsung (alur berjenjang di server).
      toast.error("Tidak dapat dipindahkan", {
        description: labelIzin(target),
        action: can(sesi, "intervensi.kelola")
          ? {
              label: "Ajukan persetujuan",
              onClick: () => ajukan.mutate({ id, tahap: target }),
            }
          : undefined,
      });
      return;
    }
    const tahapKini = k.cards[id]?.tahap;
    if (tahapKini && tahapKini === findTahap(String(over.id))) {
      const i = k.order[tahapKini].indexOf(String(over.id));
      if (i >= 0 && String(over.id) !== id) k.move(id, tahapKini, i);
    }
    const akhir = k.cards[id]?.tahap;
    if (akhir && akhir !== asal.current) {
      const judul = tahap.find((t) => t.id === akhir)?.judul ?? akhir;
      try {
        const { versi } = await api.intervensi.pindah(
          id,
          akhir,
          k.order[akhir].indexOf(id),
          k.cards[id].versi,
        );
        k.setVersi(id, versi);
        toast.success(`Dipindahkan ke “${judul}”`, {
          description: `${k.cards[id].judul}. Tercatat di riwayat kartu.`,
        });
      } catch (err) {
        snap.current && k.restore(snap.current);
        gagalPindah(err);
      }
    }
  }

  /** Menjelaskan penyebab pemindahan ditolak server; konflik versi memuat ulang papan. */
  function gagalPindah(err: unknown) {
    if (err instanceof ApiError && err.status === 409) {
      toast.error("Kartu sudah diubah pengguna lain", {
        description: "Papan dimuat ulang. Ulangi pemindahan bila masih perlu.",
      });
      qc.invalidateQueries({ queryKey: qk.intervensi.list() });
    } else if (err instanceof ApiError && err.status === 403) {
      toast.error("Tidak dapat dipindahkan", { description: err.message });
    } else {
      toast.error("Gagal menyimpan perpindahan. Posisi kartu dikembalikan.", {
        description: err instanceof Error ? err.message : undefined,
      });
    }
  }

  // Saringan hanya memengaruhi tampilan; indeks pemindahan tetap dihitung dari urutan penuh di store.
  const cocok = (c: KartuIntervensi) => !saring || COCOK[saring](c);
  const cardsOf = (tahapId: string) =>
    (k.order[tahapId] ?? [])
      .map((id) => k.cards[id])
      .filter((c) => c && cocok(c));
  const semuaKartu = useMemo(
    () =>
      tahap.flatMap((t) =>
        (k.order[t.id] ?? []).map((id) => k.cards[id]).filter(Boolean),
      ),
    [tahap, k.order, k.cards],
  );
  const kartuTampil = useMemo(
    () => (saring ? semuaKartu.filter(COCOK[saring]) : semuaKartu),
    [semuaKartu, saring],
  );
  const terpilih = kartuTampil.filter((c) => k.selected.includes(c.id));
  const aktif = activeId ? k.cards[activeId] : null;

  const berjalan = semuaKartu.filter((c) => !kartuSelesai(c)).length;
  const jumlah = (s: Saring) => semuaKartu.filter(COCOK[s]).length;
  const permintaan = antrean.data ?? [];
  const bisaDiputus = permintaan.filter((p) =>
    bisaMemutus(p, sesi.id, can(sesi, p.izinLangkahKini as Permission)),
  ).length;
  const memuatPapan = isLoading || !tahapSiap || !k.ready;
  const nilaiKartu = (n: number) => (memuatPapan ? "–" : `${n} kartu`);
  const ringkasan: ItemRingkas[] = [
    {
      label: "Terlambat",
      nilai: nilaiKartu(jumlah("terlambat")),
      keterangan: "Lewat tenggat, belum selesai",
      tone: "defisit",
      arti: "Kartu pada siklus ini yang tenggatnya sudah lewat. Klik untuk menyaring.",
      aktif: saring === "terlambat",
      onClick: () => gantiSaring("terlambat"),
    },
    {
      label: "Butuh persetujuan",
      nilai: antrean.isLoading ? "–" : `${permintaan.length} permintaan`,
      keterangan: bisaDiputus
        ? `${bisaDiputus} bisa Anda putuskan`
        : "Semua siklus",
      tone: "waspada",
      arti: "Permintaan pemindahan atau eskalasi yang menunggu keputusan. Klik untuk membuka antrean.",
      aktif: view === "persetujuan",
      onClick: () => setView(view === "persetujuan" ? "papan" : "persetujuan"),
    },
    {
      label: "Prioritas tinggi",
      nilai: nilaiKartu(jumlah("tinggi")),
      keterangan: "Belum selesai",
      tone: "kritis",
      arti: "Kartu prioritas tinggi yang masih berjalan. Klik untuk menyaring.",
      aktif: saring === "tinggi",
      onClick: () => gantiSaring("tinggi"),
    },
    {
      label: "Dieskalasi",
      nilai: nilaiKartu(jumlah("eskalasi")),
      keterangan: "Ke Dewan Pengarah",
      tone: "surplus",
      arti: "Kartu berjalan yang sudah dieskalasi ke Dewan Pengarah. Klik untuk menyaring.",
      aktif: saring === "eskalasi",
      onClick: () => gantiSaring("eskalasi"),
    },
    {
      label: "Sedang berjalan",
      nilai: nilaiKartu(berjalan),
      keterangan: `${semuaKartu.length - berjalan} selesai pada siklus ini`,
      tone: "normal",
    },
  ];

  const bulkActions: BulkAction<KartuIntervensi>[] = [
    {
      id: "majukan",
      label: "Majukan satu tahap",
      icon: ChevronsRight,
      tone: "data",
      enabled: (rows) => rows.every((r) => !r.tahap.endsWith("selesai")),
      run: async (rows) => {
        const rencana = rows.map((r) => {
          const ts = tahapPerSiklus[r.siklus] ?? [];
          return { r, next: ts[ts.findIndex((t) => t.id === r.tahap) + 1] };
        });
        const ditolak = rencana.filter(
          (p) => !p.next || !bolehMasuk(p.next.id),
        );
        if (ditolak.length) {
          toast.error(`${ditolak.length} kartu tidak dapat dimajukan`, {
            description:
              labelIzin(ditolak[0].next?.id ?? "") ??
              "Tahap berikutnya di luar kewenangan peran Anda.",
          });
          return;
        }
        const sebelum = k.snapshot();
        rencana.forEach((p) =>
          k.move(p.r.id, p.next!.id, Number.MAX_SAFE_INTEGER),
        );
        sibuk.current = true;
        try {
          for (const p of rencana) {
            const { versi } = await api.intervensi.pindah(
              p.r.id,
              p.next!.id,
              Number.MAX_SAFE_INTEGER,
              p.r.versi,
            );
            k.setVersi(p.r.id, versi);
          }
          toast.success(`${rencana.length} kartu dimajukan satu tahap`);
        } catch (err) {
          k.restore(sebelum);
          gagalPindah(err);
          qc.invalidateQueries({ queryKey: qk.intervensi.list() });
        } finally {
          sibuk.current = false;
        }
      },
    },
    {
      id: "tugaskan",
      label: "Tugaskan ke saya",
      icon: UserPlus,
      perm: "intervensi.kelola",
      run: (rows) =>
        aksiKartu.mutateAsync({
          aksi: "tugaskan",
          ids: rows.map((r) => r.id),
          label: "Ditugaskan kepada Anda",
        }),
    },
    {
      id: "eskalasi",
      label: "Eskalasi ke Dewan Pengarah",
      icon: ShieldAlert,
      perm: "intervensi.kelola",
      confirm: {
        title: "Eskalasi kartu terpilih?",
        description: (n) =>
          `${n} kartu ditandai dieskalasi ke Dewan Pengarah dan prioritasnya dinaikkan ke tinggi. Tercatat atas nama Anda.`,
        actionLabel: "Eskalasi",
      },
      run: (rows) =>
        aksiKartu.mutateAsync({
          aksi: "eskalasi",
          ids: rows.map((r) => r.id),
          label: "Dieskalasi ke Dewan Pengarah",
        }),
    },
    {
      id: "ekspor",
      label: "Ekspor CSV",
      icon: Download,
      run: (rows) =>
        exportCsv(
          rows,
          [
            { header: "ID", value: (r) => labelKode(r) },
            { header: "Judul", value: (r) => r.judul },
            { header: "Siklus", value: (r) => r.siklus },
            { header: "Tahap", value: (r) => r.tahap },
            { header: "Prioritas", value: (r) => r.prioritas },
            { header: "Tenggat", value: (r) => r.tenggat },
          ],
          `intervensi-${siklus}`,
        ),
    },
  ];

  const kolomTabel = useMemo<ColumnDef<KartuIntervensi>[]>(
    () => [
      {
        id: "intervensi",
        accessorFn: (r) =>
          `${r.judul} ${komoditasByKode[r.komoditasKode]?.nama ?? ""} ${namaWilayah(r.kabupatenKode)}`,
        header: "Intervensi",
        cell: ({ row: { original: r } }) => (
          <span className="block max-w-[28rem] min-w-44">
            <SelTeks className="max-w-full font-medium">{r.judul}</SelTeks>
            <SelTeks className="max-w-full text-xs text-muted-foreground">
              {`${komoditasByKode[r.komoditasKode]?.nama ?? r.komoditasKode} · ${namaWilayah(r.kabupatenKode)}`}
            </SelTeks>
          </span>
        ),
      },
      {
        accessorKey: "tahap",
        header: "Tahap",
        cell: ({ getValue }) => (
          <SelTeks className="max-w-[13rem] text-muted-foreground">
            {tahap.find((t) => t.id === getValue<string>())?.judul}
          </SelTeks>
        ),
      },
      {
        accessorKey: "prioritas",
        header: "Prioritas",
        cell: ({ getValue }) => {
          const p = getValue<string>();
          return (
            <StatusBadge
              tone={
                p === "TINGGI"
                  ? "defisit"
                  : p === "SEDANG"
                    ? "waspada"
                    : "netral"
              }
            >
              {p.charAt(0) + p.slice(1).toLowerCase()}
            </StatusBadge>
          );
        },
      },
      {
        accessorKey: "tenggat",
        header: "Tenggat",
        cell: ({ row: { original: r } }) => {
          const t = labelTenggat(r);
          return (
            <span className="block whitespace-nowrap">
              <span className="num block">{formatTanggal(r.tenggat)}</span>
              <span className={cn("block text-xs", t.cls)}>{t.text}</span>
            </span>
          );
        },
      },
      {
        id: "detail",
        header: () => <span className="sr-only">Detail</span>,
        enableSorting: false,
        meta: { sematKanan: true },
        cell: ({ row }) => (
          <button
            type="button"
            aria-label={`Buka detail ${row.original.judul}`}
            onClick={(e) => {
              e.stopPropagation();
              setDetailId(row.original.id);
            }}
            className="grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground"
          >
            <PanelRight className="size-4" />
          </button>
        ),
      },
    ],
    [tahap],
  );

  return (
    <div className="contents" data-siklus={siklus}>
      <HalamanPas>
        <PageHeader
          title="Intervensi"
          description={
            siklusMeta[siklus].deskripsi +
            ". Tarik kartu untuk memindahkan tahap."
          }
          actions={
            <>
              <Can perm="intervensi.kelola">
                <SpringButton
                  variant="primary"
                  size="sm"
                  onClick={() => setTambahBuka(true)}
                >
                  <Plus /> Tambah intervensi
                </SpringButton>
              </Can>
            </>
          }
          tabs={
            <ViewTabs
              layoutId="kanban-view"
              label="Tampilan intervensi"
              value={view}
              onChange={setView}
              tabs={[
                {
                  value: "papan",
                  label: "Papan",
                  icon: <LayoutGrid />,
                  count: kartuTampil.length,
                },
                { value: "tabel", label: "Tabel", icon: <Rows3 /> },
                {
                  value: "persetujuan",
                  label: "Persetujuan",
                  icon: <ListChecks />,
                  count: permintaan.length,
                },
              ]}
            />
          }
        />
        <div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2">
          <Segmented
            layoutId="kanban-siklus"
            value={siklus}
            onChange={gantiSiklus}
            className="w-full sm:w-auto"
            options={SIKLUS_URUTAN.map((s) => ({
              value: s,
              activeClass: SIKLUS_PILL[s],
              label: (
                <>
                  {siklusMeta[s].label}
                  <span className="num rounded-full bg-black/10 px-1.5 text-[11px]">
                    {data?.filter((c) => c.siklus === s).length ?? 0}
                  </span>
                </>
              ),
            }))}
          />
          {saring && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                k.clearSelection();
                setSaring(undefined);
              }}
            >
              Reset filter
            </Button>
          )}
        </div>
        <RingkasanKeputusan
          items={ringkasan}
          label="Ringkasan intervensi"
          className="[&>li:last-child:nth-child(odd)]:col-span-2 lg:[&>li:last-child:nth-child(odd)]:col-auto"
        />

        {tahapQ.isError && (
          <p
            role="alert"
            className="flex shrink-0 items-center gap-2 rounded-lg bg-waspada-soft/80 px-3 py-2 text-[13px] text-waspada-ink dark:bg-waspada/15 dark:text-amber-300"
          >
            <TriangleAlert className="size-4 shrink-0" /> Definisi tahap dari
            server belum dapat dimuat; papan memakai tahap bawaan. Muat ulang
            halaman bila perlu.
          </p>
        )}
        {view === "persetujuan" ? (
          <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_minmax(0,26rem)]">
            <PersetujuanPanel />
            <AlurPersetujuanPanel />
          </IsiPas>
        ) : memuatPapan ? (
          <div className="flex gap-3 overflow-hidden lg:min-h-0 lg:flex-1">
            {Array.from({ length: 4 }).map((_, i) => (
              <Shimmer
                key={i}
                className="h-96 w-[272px] shrink-0 rounded-lg lg:h-full"
              />
            ))}
          </div>
        ) : view === "tabel" ? (
          <DataTable
            isiPenuh
            data={kartuTampil}
            columns={kolomTabel}
            getRowId={(r) => r.id}
            entityLabel="intervensi"
            bulkActions={bulkActions}
            onRowClick={(r) => setDetailId(r.id)}
            searchPlaceholder="Cari intervensi, komoditas, wilayah"
            empty={
              saring
                ? {
                    title: "Tidak ada kartu yang cocok",
                    description:
                      "Reset filter ringkasan untuk melihat semua kartu pada siklus ini.",
                  }
                : {
                    title: "Belum ada intervensi",
                    description:
                      "Kartu muncul saat EWS terpicu atau saat Anda menambah intervensi.",
                  }
            }
          />
        ) : (
          <DndContext
            sensors={bisaSeret ? sensors : []}
            collisionDetection={closestCorners}
            onDragStart={onDragStart}
            onDragOver={onDragOver}
            onDragEnd={onDragEnd}
            onDragCancel={() => {
              sibuk.current = false;
              setActiveId(null);
              setOverTahap(null);
              snap.current && k.restore(snap.current);
            }}
          >
            <div className="scrollbar-thin flex min-w-0 gap-3 overflow-x-auto pb-2 lg:min-h-0 lg:flex-1">
              {tahap.map((t, i) => (
                <KanbanColumn
                  key={t.id}
                  tahap={t}
                  index={i}
                  total={tahap.length}
                  cards={cardsOf(t.id)}
                  selected={k.selected}
                  onToggle={k.toggleSelect}
                  onBuka={setDetailId}
                  canDrop={bolehMasuk(t.id)}
                  isTarget={
                    !!activeId && overTahap === t.id && asal.current !== t.id
                  }
                  izinLabel={labelIzin(t.id)}
                  tersaring={!!saring}
                />
              ))}
            </div>
            <DragOverlay
              dropAnimation={{
                duration: 220,
                easing: "cubic-bezier(0.22,1,0.36,1)",
              }}
            >
              {aktif ? <KanbanCardView card={aktif} overlay /> : null}
            </DragOverlay>
          </DndContext>
        )}
        <KartuDetailSheet
          kartu={detailId ? (k.cards[detailId] ?? null) : null}
          tahapSiklus={
            detailId && k.cards[detailId]
              ? (tahapPerSiklus[k.cards[detailId].siklus] ?? [])
              : []
          }
          semuaTahap={semuaTahap}
          alur={alurQ.data}
          onClose={() => setDetailId(null)}
        />
        {tambahBuka && (
          <TambahIntervensiDialog
            open
            onOpenChange={setTambahBuka}
            siklusAwal={siklus}
            onDibuat={gantiSiklus}
          />
        )}
        {view === "papan" && (
          <BulkActionBar
            rows={terpilih}
            actions={bulkActions}
            entityLabel="kartu"
            onClear={k.clearSelection}
          />
        )}
      </HalamanPas>
    </div>
  );
}
