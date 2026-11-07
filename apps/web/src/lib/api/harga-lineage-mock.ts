/** Lineage tiruan untuk mode tanpa backend: diturunkan dari entri mock dan status akhirnya. */

import { USERS } from "@/lib/data/reference";
import type {
  AksiVersiHarga,
  EntriHarga,
  LineageHarga,
  StatusEntri,
  VersiEntriHarga,
} from "@/lib/data/types";
import { toBendera } from "./harga";
import { mockEntri } from "./mock";

const JAM = 3_600_000;

/** Status → langkah versi yang telah dilewati, sama seperti pemicu basis data (aksi menurut status tujuan). */
const JALUR: Record<StatusEntri, AksiVersiHarga[]> = {
  DRAF: ["INPUT", "KEMBALIKAN"],
  DIAJUKAN: ["INPUT"],
  DIVALIDASI: ["INPUT", "VALIDASI"],
  DIPUBLIKASI: ["INPUT", "VALIDASI", "PUBLIKASI"],
  DITOLAK: ["INPUT", "TOLAK"],
};

const STATUS_AKSI: Record<AksiVersiHarga, StatusEntri> = {
  INPUT: "DIAJUKAN",
  AJUKAN: "DIAJUKAN",
  VALIDASI: "DIVALIDASI",
  PUBLIKASI: "DIPUBLIKASI",
  TOLAK: "DITOLAK",
  KEMBALIKAN: "DRAF",
  UBAH: "DIAJUKAN",
  MIGRASI: "DIAJUKAN",
};

const PERUBAHAN: Partial<Record<AksiVersiHarga, string[]>> = {
  VALIDASI: ["divalidasi_oleh", "status"],
  PUBLIKASI: ["dipublikasi_oleh", "status"],
  TOLAK: ["alasan_penolakan", "ditolak_oleh", "status"],
  KEMBALIKAN: ["divalidasi_oleh", "status"],
};

const AUDIT: Partial<Record<AksiVersiHarga, string>> = {
  INPUT: "HARGA_INPUT",
  VALIDASI: "HARGA_VALIDASI",
  PUBLIKASI: "HARGA_PUBLIKASI",
  TOLAK: "HARGA_TOLAK",
  KEMBALIKAN: "HARGA_KEMBALIKAN",
};

/** Lineage tiruan satu entri mock; waktu tahap tidak melampaui sekarang. */
export function mockLineage(e: EntriHarga): LineageHarga {
  const validator = USERS.find((u) => u.role === "OPS_KOORDINATOR")?.nama;
  const penerbit = USERS.find((u) => u.role === "MANAJER_DATA")?.nama;
  const dicatat = new Date(e.dicatatPada).getTime();
  const diterima = new Date(dicatat + 2 * 60_000).toISOString();
  const sekarang = Date.now();
  const saat = (jam: number) =>
    new Date(Math.min(dicatat + jam * JAM, sekarang)).toISOString();

  const pelaku: Partial<
    Record<AksiVersiHarga, { nama?: string; waktu: string }>
  > = {
    INPUT: { nama: e.petugasNama, waktu: diterima },
    VALIDASI: { nama: validator, waktu: saat(2) },
    PUBLIKASI: { nama: penerbit, waktu: saat(5) },
    TOLAK: { nama: validator, waktu: saat(2) },
    KEMBALIKAN: { nama: validator, waktu: saat(2) },
  };
  const alasan =
    e.alasanPenolakan ??
    "Harga di luar rentang wajar dan tanpa bukti pendukung";

  const aksi = JALUR[e.status];
  const versi: VersiEntriHarga[] = aksi.map((a, i) => ({
    versi: i + 1,
    aksi: a,
    status: STATUS_AKSI[a],
    harga: e.harga,
    ...(pelaku[a]?.nama ? { olehNama: pelaku[a].nama } : {}),
    waktu: pelaku[a]?.waktu ?? diterima,
    perubahan: PERUBAHAN[a] ?? [],
  }));

  const tahap = (a: AksiVersiHarga) => ({
    ...(pelaku[a]?.nama ? { olehNama: pelaku[a].nama } : {}),
    pada: pelaku[a]?.waktu,
  });
  const bendera = toBendera({
    di_luar_rentang: false,
    outlier: Math.abs(e.deviasiPct) > 25,
    duplikat: false,
    luar_geofence: !e.dalamGeofence,
    tanpa_lokasi: false,
    bentrok: false,
  }).map((b) => b.label);

  return {
    ...(e.petugasNama ? { petugasNama: e.petugasNama } : {}),
    dicatatPada: e.dicatatPada,
    diterimaPada: diterima,
    sumber: "LAPANGAN",
    tugas: [],
    bendera,
    skorAnomali: e.skorAnomali,
    deviasiPct: e.deviasiPct,
    ...(aksi.includes("VALIDASI") ? { validasi: tahap("VALIDASI") } : {}),
    ...(aksi.includes("PUBLIKASI") ? { publikasi: tahap("PUBLIKASI") } : {}),
    ...(aksi.includes("TOLAK")
      ? { penolakan: { ...tahap("TOLAK"), alasan } }
      : {}),
    versiTerkini: versi.length,
    versi,
    aksiAudit: aksi.map((a) => AUDIT[a]).filter((a): a is string => !!a),
  };
}

/** Lineage tiruan menurut ID entri mock; ID yang tak dikenal memakai entri pertama. */
export function mockLineageById(id: string): LineageHarga {
  const semua = mockEntri();
  return mockLineage(
    semua.find((e) => e.id === id || e.kode === id) ?? semua[0],
  );
}
