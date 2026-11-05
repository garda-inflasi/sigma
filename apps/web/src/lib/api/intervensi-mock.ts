/**
 * Data mock modul intervensi (mode tanpa backend): definisi tahap dari cadangan klien, alur persetujuan
 * seperti seed `intervensi.reference`, dan riwayat yang disusun dari posisi kartu mock.
 */

import { SIKLUS_TAHAP, SIKLUS_URUTAN } from "@/lib/data/siklus";
import type { KartuIntervensi } from "@/lib/data/types";
import type {
  AlurResponse,
  RiwayatResponse,
  TahapResponse,
} from "./intervensi";

export function tahapMock(): TahapResponse[] {
  return SIKLUS_URUTAN.flatMap((siklus) =>
    SIKLUS_TAHAP[siklus].map((t, i) => ({
      siklus: siklus.toUpperCase(),
      kode_tahap: t.id,
      urutan: i + 1,
      label: t.judul,
      petunjuk: t.petunjuk,
      izin_dibutuhkan: t.izinMasuk ?? "intervensi.kelola",
      sla_hari: t.slaHari ?? null,
    })),
  );
}

const DEWAN = {
  urutan: 1,
  label: "Persetujuan Dewan Pengarah",
  izin_dibutuhkan: "intervensi.setujui",
};

export const alurMock: AlurResponse[] = [
  {
    kode: "pindah-tekanan-petakan",
    nama: "Petakan suplai sentra surplus",
    jenis: "PINDAH_TAHAP",
    siklus: "TEKANAN",
    tahap_tujuan: "t-petakan",
    aktif: true,
    langkah: [DEWAN],
  },
  {
    kode: "pindah-tekanan-fiskal",
    nama: "Dukungan instrumen belanja",
    jenis: "PINDAH_TAHAP",
    siklus: "TEKANAN",
    tahap_tujuan: "t-fiskal",
    aktif: true,
    langkah: [
      DEWAN,
      {
        urutan: 2,
        label: "Persetujuan Kanwil DJPb",
        izin_dibutuhkan: "intervensi.setujui_fiskal",
      },
    ],
  },
  {
    kode: "pindah-tekanan-selesai",
    nama: "Penutupan intervensi tekanan harga",
    jenis: "PINDAH_TAHAP",
    siklus: "TEKANAN",
    tahap_tujuan: "t-selesai",
    aktif: true,
    langkah: [DEWAN],
  },
  {
    kode: "pindah-surplus-selesai",
    nama: "Penutupan intervensi surplus",
    jenis: "PINDAH_TAHAP",
    siklus: "SURPLUS",
    tahap_tujuan: "s-selesai",
    aktif: true,
    langkah: [DEWAN],
  },
  {
    kode: "eskalasi",
    nama: "Telaah eskalasi kartu",
    jenis: "ESKALASI",
    siklus: null,
    tahap_tujuan: null,
    aktif: true,
    langkah: [
      {
        urutan: 1,
        label: "Telaah Dewan Pengarah",
        izin_dibutuhkan: "intervensi.setujui",
      },
    ],
  },
];

/** Riwayat kartu mock: dibuat di tahap pertama lalu dipindah satu per satu sampai tahap sekarang. */
export function riwayatMock(kartu: KartuIntervensi): RiwayatResponse[] {
  const tahap = SIKLUS_TAHAP[kartu.siklus];
  const sampai = Math.max(
    0,
    tahap.findIndex((t) => t.id === kartu.tahap),
  );
  const pelaku = kartu.penanggungJawab[0]?.nama ?? null;
  const mulai = new Date(kartu.tenggat).getTime() - 14 * 86_400_000;
  return tahap.slice(0, sampai + 1).map((t, i) => ({
    id: i + 1,
    aksi: i === 0 ? "BUAT" : "PINDAH_TAHAP",
    dari: i === 0 ? null : tahap[i - 1].id,
    ke: t.id,
    oleh_id: null,
    oleh_nama: pelaku,
    waktu: new Date(mulai + i * 86_400_000).toISOString(),
    alasan: "",
  }));
}
