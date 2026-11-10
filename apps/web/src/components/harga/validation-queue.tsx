"use client";

import { DataTable } from "@/components/data-table/data-table";
import { PanelTitle } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import type { EntriHarga } from "@/lib/data/types";
import type { HargaParams } from "@/lib/api/harga";
import { useEntriHarga } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { useHargaBulkActions, useHargaColumns } from "./harga-table";
import { SpringLink } from "@/components/garda/spring-button";

const PARAMS: HargaParams = { status: "DIAJUKAN", urut: "skor" };
const getId = (r: EntriHarga) => r.id;

export function ValidationQueue() {
  const { data, isLoading, error, refetch } = useEntriHarga(PARAMS);
  const columns = useHargaColumns({ ringkas: true });
  const actions = useHargaBulkActions(qk.harga.list(PARAMS));
  const antre = data?.filter((e) => e.status === "DIAJUKAN");
  return (
    <section className="space-y-3">
      <PanelTitle
        title="Antrean validasi"
        description="Entri lapangan yang menunggu verifikasi koordinator, yang paling mencurigakan di atas. Centang beberapa untuk memvalidasi sekaligus."
        actions={<SpringLink href="/data/harga" size="sm" variant="soft">Buka semua</SpringLink>}
      />
      {/* Tanpa data setelah selesai memuat berarti gagal: DataTable akan menampilkannya sebagai "antrean kosong". */}
      {!isLoading && !data ? (
        <div className="glass rounded-xl"><PanelGalat judul="Antrean validasi gagal dimuat" error={error} onCoba={() => void refetch()} /></div>
      ) : (
        <DataTable
          data={antre}
          isLoading={isLoading}
          columns={columns}
          getRowId={getId}
          entityLabel="entri"
          bulkActions={actions}
          pageSize={8}
          searchPlaceholder="Cari komoditas, pasar, atau petugas"
          rowTone={(r) => (Math.abs(r.deviasiPct) > 25 ? "defisit" : Math.abs(r.deviasiPct) > 10 ? "waspada" : undefined)}
          empty={{ title: "Antrean validasi kosong", description: "Semua entri lapangan sudah diverifikasi. Entri baru akan masuk otomatis saat agen lapangan menyinkronkan data." }}
        />
      )}
    </section>
  );
}
