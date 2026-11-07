"use client";

import { Plus } from "lucide-react";
import { useState } from "react";
import { PageHeader } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { useCan } from "@/lib/auth/session-context";
import type { Entitas } from "@/lib/data/master";
import { KecamatanPanel } from "./kecamatan-panel";
import { KomoditasPanel } from "./komoditas-panel";
import { MasterTabs } from "./master-tabs";
import { PasarPanel } from "./pasar-panel";
import { PedagangPanel } from "./pedagang-panel";
import { SatuanPanel } from "./satuan-panel";
import { UsulanDialog, type TargetUsulan } from "./usulan-dialog";
import { WilayahPanel } from "./wilayah-panel";

/**
 * Master data dengan versioning dan persetujuan (pengaju ≠ penyetuju). Komoditas dan pasar berubah lewat usulan
 * yang diputuskan di antrean persetujuan (`/master/perubahan`); wilayah, kecamatan, satuan, dan pedagang hanya
 * dibaca. "Tambah" mengajukan data baru (izin master.ajukan).
 */
export function MasterPage({ entitas }: { entitas: Entitas }) {
  const bisaAjukan = useCan("master.ajukan");
  const [tambah, setTambah] = useState<TargetUsulan | null>(null);
  const bisaTambah =
    bisaAjukan && (entitas === "komoditas" || entitas === "pasar");

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Master data"
        description="Standar kode wilayah BPS dan komoditas nasional. Setiap perubahan berversi dan membutuhkan persetujuan."
        actions={
          bisaTambah && (
            <SpringButton
              variant="primary"
              onClick={() =>
                setTambah({ entitas: entitas as "komoditas" | "pasar" })
              }
            >
              <Plus />{" "}
              {entitas === "komoditas" ? "Tambah komoditas" : "Tambah pasar"}
            </SpringButton>
          )
        }
      />
      <MasterTabs aktif={entitas} />
      {entitas === "komoditas" && <KomoditasPanel />}
      {entitas === "wilayah" && <WilayahPanel />}
      {entitas === "pasar" && <PasarPanel />}
      {entitas === "kecamatan" && <KecamatanPanel />}
      {entitas === "satuan" && <SatuanPanel />}
      {entitas === "pedagang" && <PedagangPanel />}
      <UsulanDialog
        target={tambah}
        open={!!tambah}
        onOpenChange={(o) => !o && setTambah(null)}
      />
    </div>
  );
}
