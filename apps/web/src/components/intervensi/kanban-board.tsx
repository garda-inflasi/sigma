"use client";

import {
  DndContext, DragOverlay, KeyboardSensor, PointerSensor, closestCorners, useSensor, useSensors,
  type DragEndEvent, type DragOverEvent, type DragStartEvent,
} from "@dnd-kit/core";
import { sortableKeyboardCoordinates } from "@dnd-kit/sortable";
import { ChevronsRight, Download, LayoutGrid, Rows3, ShieldAlert, UserPlus } from "lucide-react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { BulkActionBar, type BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { Segmented } from "@/components/garda/segmented";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { api } from "@/lib/api/endpoints";
import { useSession } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import { komoditasByKode } from "@/lib/data/reference";
import { namaWilayah } from "@/lib/data/generators";
import { SIKLUS_TAHAP, SIKLUS_URUTAN } from "@/lib/data/siklus";
import type { KartuIntervensi } from "@/lib/data/types";
import { siklusMeta, type Siklus } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { useIntervensi } from "@/lib/query/hooks";
import { can, type Permission } from "@/lib/rbac/permissions";
import { useKanban } from "@/lib/stores/kanban-store";
import { KanbanCardView } from "./kanban-card";
import { KanbanColumn } from "./kanban-column";
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
  const k = useKanban();
  const [activeId, setActiveId] = useState<string | null>(null);
  const [overTahap, setOverTahap] = useState<string | null>(null);
  const snap = useRef<ReturnType<typeof k.snapshot> | null>(null);
  const asal = useRef<string | null>(null);

  useEffect(() => { if (data) k.init(data); }, [data, k]);

  const tahap = SIKLUS_TAHAP[siklus];
  const izinMasuk = (tahapId: string): Permission => Object.values(SIKLUS_TAHAP).flat().find((t) => t.id === tahapId)?.izinMasuk ?? "intervensi.kelola";
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

  async function onDragEnd({ active, over }: DragEndEvent) {
    const id = String(active.id);
    setActiveId(null);
    setOverTahap(null);
    if (!over) return snap.current && k.restore(snap.current);
    const target = findTahap(String(over.id));
    if (target && !bolehMasuk(target)) {
      snap.current && k.restore(snap.current);
      toast.error("Tidak dapat dipindahkan", { description: IZIN_LABEL[izinMasuk(target)] });
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
        await api.intervensi.pindah(id, akhir, k.order[akhir].indexOf(id));
        toast.success(`Dipindahkan ke “${judul}”`, { description: `${k.cards[id].judul}. Penanggung jawab tahap berikutnya telah diberi tahu.` });
      } catch {
        snap.current && k.restore(snap.current);
        toast.error("Gagal menyimpan perpindahan. Posisi kartu dikembalikan.");
      }
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
          const ts = SIKLUS_TAHAP[r.siklus];
          return { r, next: ts[ts.findIndex((t) => t.id === r.tahap) + 1] };
        });
        const ditolak = rencana.filter((p) => !p.next || !bolehMasuk(p.next.id));
        if (ditolak.length) {
          toast.error(`${ditolak.length} kartu tidak dapat dimajukan`, { description: IZIN_LABEL[izinMasuk(ditolak[0].next?.id ?? "")] ?? "Tahap berikutnya di luar kewenangan peran Anda." });
          return;
        }
        rencana.forEach((p) => k.move(p.r.id, p.next!.id, Number.MAX_SAFE_INTEGER));
        await Promise.all(rencana.map((p) => api.intervensi.pindah(p.r.id, p.next!.id, 0)));
        toast.success(`${rencana.length} kartu dimajukan satu tahap`);
      },
    },
    { id: "tugaskan", label: "Tugaskan ke saya", icon: UserPlus, perm: "intervensi.kelola", run: (rows) => { toast.success(`${rows.length} kartu ditugaskan kepada Anda`); } },
    { id: "eskalasi", label: "Eskalasi ke Dewan Pengarah", icon: ShieldAlert, perm: "intervensi.kelola", run: (rows) => { toast.success(`${rows.length} kartu dieskalasi`, { description: "Masuk ke agenda telaah Dewan Pengarah." }); } },
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
  ], [tahap]);

  return (
    <div className="mx-auto max-w-[1800px] space-y-5" data-siklus={siklus}>
      <PageHeader
        title="Intervensi"
        description={siklusMeta[siklus].deskripsi + ". Tarik kartu antartahap; tahap yang membutuhkan persetujuan ditandai gembok."}
        actions={
          <>
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

      {isLoading || !k.ready ? (
        <div className="flex gap-4 overflow-hidden">{Array.from({ length: 4 }).map((_, i) => <Shimmer key={i} className="h-[520px] w-[296px] shrink-0 rounded-xl" />)}</div>
      ) : view === "tabel" ? (
        <DataTable data={semuaKartu} columns={kolomTabel} getRowId={(r) => r.id} entityLabel="intervensi" bulkActions={bulkActions} searchPlaceholder="Cari intervensi, komoditas, wilayah" />
      ) : (
        <DndContext sensors={bisaSeret ? sensors : []} collisionDetection={closestCorners} onDragStart={onDragStart} onDragOver={onDragOver} onDragEnd={onDragEnd} onDragCancel={() => { setActiveId(null); setOverTahap(null); snap.current && k.restore(snap.current); }}>
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
                canDrop={bolehMasuk(t.id)}
                isTarget={!!activeId && overTahap === t.id && asal.current !== t.id}
                izinLabel={IZIN_LABEL[izinMasuk(t.id)]}
              />
            ))}
          </div>
          <DragOverlay dropAnimation={{ duration: 220, easing: "cubic-bezier(0.22,1,0.36,1)" }}>
            {aktif ? <KanbanCardView card={aktif} overlay /> : null}
          </DragOverlay>
        </DndContext>
      )}
      {view === "papan" && <BulkActionBar rows={terpilih} actions={bulkActions} entityLabel="kartu" onClear={k.clearSelection} />}
    </div>
  );
}
