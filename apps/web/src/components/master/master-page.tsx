"use client";

import { Plus } from "lucide-react";
import { useState } from "react";
import { HalamanPas } from "@/components/garda/halaman-pas";
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

const DESKRIPSI: Record<Entitas, string> = {
  komoditas: "Diubah lewat usulan.",
  wilayah: "Kode kabupaten/kota BPS, hanya baca.",
  pasar: "Diubah lewat usulan.",
  kecamatan: "Per kabupaten/kota, hanya baca.",
  satuan: "Satuan komoditas, hanya baca.",
  pedagang: "Pedagang per pasar, hanya baca.",
};

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
    <HalamanPas>
      <PageHeader
        title="Master data"
        description={DESKRIPSI[entitas]}
        tabs={<MasterTabs aktif={entitas} />}
        actions={
          bisaTambah && (
            <SpringButton
              variant="primary"
              onClick={() =>
                setTambah({ entitas: entitas as "komoditas" | "pasar" })
              }
            >
              <Plus />{" "}
              {/* Di 1024–1279px baris kepala tidak muat untuk tab dan label penuh; nama aksesibel tetap utuh. */}
              <span className="lg:max-xl:sr-only">
                {entitas === "komoditas" ? "Tambah komoditas" : "Tambah pasar"}
              </span>
            </SpringButton>
          )
        }
      />
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
    </HalamanPas>
  );
}
