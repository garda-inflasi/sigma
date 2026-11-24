"use client";

import { Plus } from "lucide-react";
import { useState } from "react";
import { HalamanPas } from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { useCan, useOptionalSession } from "@/lib/auth/session-context";
import type { Entitas } from "@/lib/data/master";
import { KecamatanPanel } from "./kecamatan-panel";
import { KomoditasPanel } from "./komoditas-panel";
import { LangsungDialog, type TargetLangsung } from "./langsung-dialog";
import { LokasiSpasialPanel } from "./lokasi-spasial-panel";
import { MasterTabs } from "./master-tabs";
import { PasarPanel } from "./pasar-panel";
import { PedagangPanel } from "./pedagang-panel";
import { SatuanPanel } from "./satuan-panel";
import { UsulanDialog, type TargetUsulan } from "./usulan-dialog";
import { WilayahPanel } from "./wilayah-panel";

const DESKRIPSI: Record<Entitas, string> = {
  komoditas: "Diubah lewat usulan atau langsung oleh Superadmin.",
  wilayah: "Kode kabupaten/kota BPS, hanya baca.",
  pasar: "Diubah lewat usulan atau langsung oleh Superadmin.",
  kecamatan: "Per kabupaten/kota; dapat ditambah Superadmin.",
  satuan: "Satuan komoditas; dapat ditambah Superadmin.",
  pedagang: "Pedagang per pasar; dapat ditambah Superadmin.",
  pertanian: "Titik koordinat sentra produksi pertanian Aceh.",
  perkampungan: "Titik koordinat gampong dan permukiman distribusi.",
};

const LABEL_TAMBAH: Partial<Record<Entitas, string>> = {
  komoditas: "Tambah komoditas",
  pasar: "Tambah pasar",
  kecamatan: "Tambah kecamatan",
  satuan: "Tambah satuan",
  pedagang: "Tambah pedagang",
  pertanian: "Tambah sentra",
  perkampungan: "Tambah kampung",
};

/**
 * Master data dengan versioning dan persetujuan (pengaju ≠ penyetuju), serta mode input langsung untuk Superadmin.
 */
export function MasterPage({ entitas }: { entitas: Entitas }) {
  const sesi = useOptionalSession();
  const isSuperadmin = sesi?.role === "ADMIN_SISTEM";
  const bisaAjukan = useCan("master.ajukan");
  const [tambahUsulan, setTambahUsulan] = useState<TargetUsulan | null>(null);
  const [tambahLangsung, setTambahLangsung] = useState<TargetLangsung | null>(null);

  const bisaTambah = isSuperadmin
    ? entitas !== "wilayah"
    : bisaAjukan && (entitas === "komoditas" || entitas === "pasar");

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
              onClick={() => {
                if (isSuperadmin && entitas !== "wilayah") {
                  setTambahLangsung({ entitas });
                } else if (entitas === "komoditas" || entitas === "pasar") {
                  setTambahUsulan({ entitas });
                }
              }}
            >
              <Plus />{" "}
              {/* Di 1024–1279px baris kepala tidak muat untuk tab dan label penuh; nama aksesibel tetap utuh. */}
              <span className="lg:max-xl:sr-only">
                {LABEL_TAMBAH[entitas] ?? "Tambah data"}
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
      {entitas === "pertanian" && <LokasiSpasialPanel jenis="PERTANIAN" />}
      {entitas === "perkampungan" && <LokasiSpasialPanel jenis="PERKAMPUNGAN" />}
      <UsulanDialog
        target={tambahUsulan}
        open={!!tambahUsulan}
        onOpenChange={(o) => !o && setTambahUsulan(null)}
      />
      <LangsungDialog
        target={tambahLangsung}
        open={!!tambahLangsung}
        onOpenChange={(o) => !o && setTambahLangsung(null)}
      />
    </HalamanPas>
  );
}
