"use client";

import { DataTable } from "@/components/data-table/data-table";
import { PanelTitle } from "@/components/garda/page-header";
import type { EntriHarga } from "@/lib/data/types";
import { useEntriHarga } from "@/lib/query/hooks";
import { useHargaBulkActions, useHargaColumns } from "./harga-table";
import { SpringLink } from "@/components/garda/spring-button";

const getId = (r: EntriHarga) => r.id;

export function ValidationQueue() {
  const { data, isLoading } = useEntriHarga();
  const columns = useHargaColumns({ ringkas: true });
  const actions = useHargaBulkActions();
  const antre = data?.filter((e) => e.status === "DIAJUKAN");
  return (
    <section className="space-y-3">
      <PanelTitle
        title="Antrean validasi"
        description="Entri lapangan yang menunggu verifikasi koordinator. Centang beberapa untuk memvalidasi sekaligus."
        actions={<SpringLink href="/data/harga" size="sm" variant="soft">Buka semua</SpringLink>}
      />
      <DataTable
        data={antre}
        isLoading={isLoading}
        columns={columns}
        getRowId={getId}
        entityLabel="entri"
        bulkActions={actions}
        pageSize={8}
        searchPlaceholder="Cari komoditas, pasar, atau petugas"
        initialSorting={[{ id: "deviasiPct", desc: true }]}
        rowTone={(r) => (Math.abs(r.deviasiPct) > 25 ? "defisit" : Math.abs(r.deviasiPct) > 10 ? "waspada" : undefined)}
        empty={{ title: "Antrean validasi kosong", description: "Semua entri lapangan sudah diverifikasi. Entri baru akan masuk otomatis saat agen lapangan menyinkronkan data." }}
      />
    </section>
  );
}
