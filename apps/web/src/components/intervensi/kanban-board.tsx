"use client";

import {
  DndContext, DragOverlay, KeyboardSensor, PointerSensor, closestCorners, useSensor, useSensors,
  type DragEndEvent, type DragOverEvent, type DragStartEvent,
} from "@dnd-kit/core";
import { sortableKeyboardCoordinates } from "@dnd-kit/sortable";
import { ChevronsRight, Download, LayoutGrid, PanelRight, Plus, Rows3, ShieldAlert, TriangleAlert, UserPlus } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { BulkActionBar, type BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { Segmented } from "@/components/garda/segmented";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/endpoints";
import { alurUntukTahap, ringkasRantai } from "@/lib/api/intervensi";
import { Can, useSession } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import { komoditasByKode } from "@/lib/data/reference";
import { namaWilayah } from "@/lib/data/generators";
import { SIKLUS_TAHAP, SIKLUS_URUTAN } from "@/lib/data/siklus";
import type { KartuIntervensi } from "@/lib/data/types";
import { siklusMeta, type Siklus } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { useAjukanPersetujuan, useAksiKartu, useAlurPersetujuan, useIntervensi, useTahapIntervensi } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { can, type Permission } from "@/lib/rbac/permissions";
import { useKanban } from "@/lib/stores/kanban-store";
import { KanbanCardView } from "./kanban-card";
import { KanbanColumn } from "./kanban-column";
import { KartuDetailSheet } from "./kartu-detail-sheet";
import { AlurPersetujuanPanel, PersetujuanPanel } from "./persetujuan-panel";
import { TambahIntervensiDialog } from "./tambah-intervensi-dialog";
import type { ColumnDef } from "@tanstack/react-table";

const IZIN_LABEL: Partial<Record<Permission, string>> = {
  "intervensi.setujui": "Butuh persetujuan Dewan Pengarah",
  "intervensi.setujui_fiskal": "Butuh persetujuan Kanwil DJPb",
  "intervensi.kelola": "Butuh peran pengelola intervensi",
};

const SIKLUS_PILL: Record<Siklus, string> = { normal: "bg-normal", tekanan: "bg-defisit", surplus: "bg-surplus" };

/** Kanban intervensi 3 siklus — warna adaptif per siklus, drag-drop sadar izin, aksi massal. */
export function KanbanBoard() {
  const { role } = useSession();
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [siklus, setSiklus] = useState<Siklus>((params.get("siklus") as Siklus) ?? "tekanan");
  const [view, setView] = useState<"papan" | "tabel">("papan");
  const { data, isLoading } = useIntervensi();
  const tahapQ = useTahapIntervensi();
  const alurQ = useAlurPersetujuan(can(role, "intervensi.lihat"));
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
  const semuaTahap = useMemo(() => Object.values(tahapPerSiklus).flat(), [tahapPerSiklus]);
  const tahapSiap = !tahapQ.isLoading;
  useEffect(() => {
    if (data && tahapSiap && !sibuk.current) useKanban.getState().sync(data, semuaTahap.map((t) => t.id));
  }, [data, tahapSiap, semuaTahap]);

  const tahap = tahapPerSiklus[siklus] ?? [];
  const izinMasuk = (tahapId: string): Permission => semuaTahap.find((t) => t.id === tahapId)?.izinMasuk ?? "intervensi.kelola";
  /** Alasan kolom terkunci; memakai rantai persetujuan dari server bila tahap punya alur. */
  const labelIzin = (tahapId: string) => {
    const alur = alurQ.data ? alurUntukTahap(alurQ.data, tahapId) : undefined;
    return alur ? `Butuh persetujuan berjenjang: ${ringkasRantai(alur)}` : IZIN_LABEL[izinMasuk(tahapId)];
  };
  const bolehMasuk = (tahapId: string) => can(role, izinMasuk(tahapId));
  const bisaSeret = can(role, "intervensi.kelola") || can(role, "intervensi.setujui") || can(role, "intervensi.setujui_fiskal");

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const findTahap = (id: string) => (k.order[id] ? id : k.cards[id]?.tahap);

  function gantiSiklus(s: Siklus) {
    setSiklus(s);
    k.clearSelection();
    router.replace(`${pathname}?siklus=${s}`, { scroll: false });
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
        action: can(role, "intervensi.kelola") ? { label: "Ajukan persetujuan", onClick: () => ajukan.mutate({ id, tahap: target }) } : undefined,
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
        const { versi } = await api.intervensi.pindah(id, akhir, k.order[akhir].indexOf(id), k.cards[id].versi);
        k.setVersi(id, versi);
        toast.success(`Dipindahkan ke “${judul}”`, { description: `${k.cards[id].judul}. Tercatat di riwayat kartu.` });
      } catch (err) {
        snap.current && k.restore(snap.current);
        gagalPindah(err);
      }
    }
  }

  /** Menjelaskan penyebab pemindahan ditolak server; konflik versi memuat ulang papan. */
  function gagalPindah(err: unknown) {
    if (err instanceof ApiError && err.status === 409) {
      toast.error("Kartu sudah diubah pengguna lain", { description: "Papan dimuat ulang. Ulangi pemindahan bila masih perlu." });
      qc.invalidateQueries({ queryKey: qk.intervensi.list() });
    } else if (err instanceof ApiError && err.status === 403) {
      toast.error("Tidak dapat dipindahkan", { description: err.message });
    } else {
      toast.error("Gagal menyimpan perpindahan. Posisi kartu dikembalikan.", { description: err instanceof Error ? err.message : undefined });
    }
  }

  const cardsOf = (tahapId: string) => (k.order[tahapId] ?? []).map((id) => k.cards[id]).filter(Boolean);
  const semuaKartu = useMemo(() => tahap.flatMap((t) => (k.order[t.id] ?? []).map((id) => k.cards[id]).filter(Boolean)), [tahap, k.order, k.cards]);
  const terpilih = semuaKartu.filter((c) => k.selected.includes(c.id));
  const aktif = activeId ? k.cards[activeId] : null;

  const bulkActions: BulkAction<KartuIntervensi>[] = [
    {
      id: "majukan", label: "Majukan satu tahap", icon: ChevronsRight, tone: "data",
      enabled: (rows) => rows.every((r) => !r.tahap.endsWith("selesai")),
      run: async (rows) => {
        const rencana = rows.map((r) => {
          const ts = tahapPerSiklus[r.siklus] ?? [];
          return { r, next: ts[ts.findIndex((t) => t.id === r.tahap) + 1] };
        });
        const ditolak = rencana.filter((p) => !p.next || !bolehMasuk(p.next.id));
        if (ditolak.length) {
          toast.error(`${ditolak.length} kartu tidak dapat dimajukan`, { description: labelIzin(ditolak[0].next?.id ?? "") ?? "Tahap berikutnya di luar kewenangan peran Anda." });
          return;
        }
        const sebelum = k.snapshot();
        rencana.forEach((p) => k.move(p.r.id, p.next!.id, Number.MAX_SAFE_INTEGER));
        sibuk.current = true;
        try {
          for (const p of rencana) {
            const { versi } = await api.intervensi.pindah(p.r.id, p.next!.id, Number.MAX_SAFE_INTEGER, p.r.versi);
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
      id: "tugaskan", label: "Tugaskan ke saya", icon: UserPlus, perm: "intervensi.kelola",
      run: (rows) => aksiKartu.mutateAsync({ aksi: "tugaskan", ids: rows.map((r) => r.id), label: "Ditugaskan kepada Anda" }),
    },
    {
      id: "eskalasi", label: "Eskalasi ke Dewan Pengarah", icon: ShieldAlert, perm: "intervensi.kelola",
      confirm: { title: "Eskalasi kartu terpilih?", description: (n) => `${n} kartu ditandai dieskalasi ke Dewan Pengarah dan prioritasnya dinaikkan ke tinggi. Tercatat atas nama Anda.`, actionLabel: "Eskalasi" },
      run: (rows) => aksiKartu.mutateAsync({ aksi: "eskalasi", ids: rows.map((r) => r.id), label: "Dieskalasi ke Dewan Pengarah" }),
    },
    {
      id: "ekspor", label: "Ekspor CSV", icon: Download,
      run: (rows) => exportCsv(rows, [
        { header: "ID", value: (r) => r.id }, { header: "Judul", value: (r) => r.judul }, { header: "Siklus", value: (r) => r.siklus },
        { header: "Tahap", value: (r) => r.tahap }, { header: "Prioritas", value: (r) => r.prioritas }, { header: "Tenggat", value: (r) => r.tenggat },
      ], `intervensi-${siklus}`),
    },
  ];

  const kolomTabel = useMemo<ColumnDef<KartuIntervensi>[]>(() => [
    { accessorKey: "judul", header: "Intervensi", cell: ({ getValue }) => <span className="font-medium">{getValue<string>()}</span> },
    { accessorKey: "tahap", header: "Tahap", cell: ({ getValue }) => <span className="text-muted-foreground">{tahap.find((t) => t.id === getValue<string>())?.judul}</span> },
    { id: "komoditas", accessorFn: (r) => komoditasByKode[r.komoditasKode]?.nama, header: "Komoditas" },
    { id: "wilayah", accessorFn: (r) => namaWilayah(r.kabupatenKode), header: "Wilayah" },
    { accessorKey: "prioritas", header: "Prioritas", cell: ({ getValue }) => { const p = getValue<string>(); return <StatusBadge tone={p === "TINGGI" ? "defisit" : p === "SEDANG" ? "waspada" : "netral"}>{p.charAt(0) + p.slice(1).toLowerCase()}</StatusBadge>; } },
    { accessorKey: "tenggat", header: "Tenggat", cell: ({ getValue }) => <span className="whitespace-nowrap">{formatTanggal(getValue<string>())}</span> },
    {
      id: "detail", header: () => <span className="sr-only">Detail</span>, enableSorting: false,
      cell: ({ row }) => (
        <button type="button" aria-label={`Buka detail ${row.original.judul}`} onClick={(e) => { e.stopPropagation(); setDetailId(row.original.id); }} className="grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground">
          <PanelRight className="size-4" />
        </button>
      ),
    },
  ], [tahap]);

  return (
    <div className="mx-auto max-w-[1800px] space-y-5" data-siklus={siklus}>
      <PageHeader
        title="Intervensi"
        description={siklusMeta[siklus].deskripsi + ". Tarik kartu antartahap; tahap yang membutuhkan persetujuan ditandai gembok."}
        actions={
          <>
            <Can perm="intervensi.kelola">
              <SpringButton variant="primary" size="sm" onClick={() => setTambahBuka(true)}><Plus /> Tambah intervensi</SpringButton>
            </Can>
            <Segmented layoutId="kanban-view" size="sm" value={view} onChange={setView} options={[{ value: "papan", label: <><LayoutGrid className="size-3.5" />Papan</> }, { value: "tabel", label: <><Rows3 className="size-3.5" />Tabel</> }]} />
          </>
        }
      />
      <Segmented
        layoutId="kanban-siklus"
        value={siklus}
        onChange={gantiSiklus}
        className="w-full sm:w-auto"
        options={SIKLUS_URUTAN.map((s) => ({
          value: s,
          activeClass: SIKLUS_PILL[s],
          label: <>{siklusMeta[s].label}<span className="num rounded-full bg-black/10 px-1.5 text-[11px]">{data?.filter((c) => c.siklus === s).length ?? 0}</span></>,
        }))}
      />

      {tahapQ.isError && (
        <p role="alert" className="flex items-center gap-2 rounded-xl bg-waspada-soft/80 px-4 py-2.5 text-sm text-waspada-ink dark:bg-waspada/15 dark:text-amber-300">
          <TriangleAlert className="size-4 shrink-0" /> Definisi tahap dari server belum dapat dimuat; papan memakai tahap bawaan. Muat ulang halaman bila perlu.
        </p>
      )}
      {isLoading || !tahapSiap || !k.ready ? (
        <div className="flex gap-4 overflow-hidden">{Array.from({ length: 4 }).map((_, i) => <Shimmer key={i} className="h-[520px] w-[296px] shrink-0 rounded-xl" />)}</div>
      ) : view === "tabel" ? (
        <DataTable data={semuaKartu} columns={kolomTabel} getRowId={(r) => r.id} entityLabel="intervensi" bulkActions={bulkActions} onRowClick={(r) => setDetailId(r.id)} searchPlaceholder="Cari intervensi, komoditas, wilayah" />
      ) : (
        <DndContext sensors={bisaSeret ? sensors : []} collisionDetection={closestCorners} onDragStart={onDragStart} onDragOver={onDragOver} onDragEnd={onDragEnd} onDragCancel={() => { sibuk.current = false; setActiveId(null); setOverTahap(null); snap.current && k.restore(snap.current); }}>
          <div className="scrollbar-thin -mx-1 flex gap-4 overflow-x-auto px-1 pb-4" style={{ minHeight: 540 }}>
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
                isTarget={!!activeId && overTahap === t.id && asal.current !== t.id}
                izinLabel={labelIzin(t.id)}
              />
            ))}
          </div>
          <DragOverlay dropAnimation={{ duration: 220, easing: "cubic-bezier(0.22,1,0.36,1)" }}>
            {aktif ? <KanbanCardView card={aktif} overlay /> : null}
          </DragOverlay>
        </DndContext>
      )}
      <PersetujuanPanel />
      <AlurPersetujuanPanel />
      <KartuDetailSheet
        kartu={detailId ? (k.cards[detailId] ?? null) : null}
        tahapSiklus={detailId && k.cards[detailId] ? (tahapPerSiklus[k.cards[detailId].siklus] ?? []) : []}
        semuaTahap={semuaTahap}
        alur={alurQ.data}
        onClose={() => setDetailId(null)}
      />
      {tambahBuka && (
        <TambahIntervensiDialog open onOpenChange={setTambahBuka} siklusAwal={siklus} onDibuat={gantiSiklus} />
      )}
      {view === "papan" && <BulkActionBar rows={terpilih} actions={bulkActions} entityLabel="kartu" onClear={k.clearSelection} />}
    </div>
  );
}
