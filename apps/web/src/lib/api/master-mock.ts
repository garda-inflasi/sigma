/**
 * Backend palsu master data untuk mode mock: menyimpan komoditas, pasar, dan antrean usulan di memori dan
 * menjawab dengan bentuk respons API Go (snake_case), jadi pemetaan UI sama dengan mode nyata. Mengajukan
 * usulan menambah antrean; menyetujui menerapkannya ke baris (versi naik, riwayat versi bertambah).
 * Galat berupa `ApiError` dengan amplop `{ code, message }` seperti server, agar percabangan menurut kode berjalan.
 * Keadaan hilang saat halaman dimuat ulang.
 */

import { KABUPATEN, KOMODITAS, PASAR, USERS } from "@/lib/data/reference";
import type { Role } from "@/lib/rbac/roles";
import { ApiError } from "./client";
import type {
  KecamatanResponse,
  KomoditasDetailResponse,
  MasterBulkAksi,
  MasterBulkResponse,
  MasterKomoditasResponse,
  MasterPasarResponse,
  MasterWilayahResponse,
  PasarDetailResponse,
  PedagangResponse,
  SatuanResponse,
  WilayahDetailResponse,
} from "./master";
import type {
  OrangRef,
  PerubahanMasterResponse,
  PerubahanParams,
  RiwayatVersiResponse,
  UsulanBody,
} from "./master-perubahan";

/** Pelaku aksi (pengguna sesi). Mode nyata tidak memakainya: server membaca dari token. */
export type AktorMaster = OrangRef;

interface KomoditasDb extends MasterKomoditasResponse {
  riwayat: RiwayatVersiResponse[];
  /** Sudah punya data harga: tidak boleh dihapus, hanya diarsipkan. */
  dirujuk: boolean;
}

interface PasarDb extends MasterPasarResponse {
  kecamatan_kode: string | null;
  kecamatan_nama: string | null;
  versi: number;
  dirujuk: boolean;
}

interface Db {
  komoditas: KomoditasDb[];
  pasar: PasarDb[];
  perubahan: PerubahanMasterResponse[];
  pedagang: PedagangResponse[];
  seq: number;
}

/* ── Galat ──────────────────────────────────────────────────── */

function galat(status: number, code: string, message: string): never {
  throw new ApiError(status, message, { code, message, data: null });
}

/* ── Data awal ──────────────────────────────────────────────── */

const hariIni = () => new Date().toISOString().slice(0, 10);
const sekarang = () => new Date().toISOString();

const namaResmi = (k: (typeof KABUPATEN)[number]) =>
  `${k.jenis === "Kota" ? "KOTA" : "KABUPATEN"} ${k.nama.toUpperCase()}`;

const BANDA_ACEH = [
  "Meuraxa",
  "Jaya Baru",
  "Banda Raya",
  "Baiturrahman",
  "Lueng Bata",
  "Kuta Alam",
  "Kuta Raja",
  "Syiah Kuala",
  "Ulee Kareng",
];

/** Kecamatan contoh: Banda Aceh dengan nama sebenarnya, kabupaten/kota lain berupa isian agar paginasi dapat dicoba. */
function buatKecamatan(): KecamatanResponse[] {
  return KABUPATEN.flatMap((kab) => {
    const jumlah = kab.kode === "1171" ? BANDA_ACEH.length : 5;
    return Array.from({ length: jumlah }, (_, i) => {
      const nn = String(i + 1).padStart(2, "0");
      return {
        kode: `${kab.kode}${String((i + 1) * 10).padStart(3, "0")}`,
        nama:
          kab.kode === "1171"
            ? BANDA_ACEH[i]
            : `Kecamatan ${kab.nama} ${i + 1}`,
        kabupaten_kode: kab.kode,
        kabupaten_nama: namaResmi(kab),
        kode_dagri: `${kab.kode.slice(0, 2)}.${kab.kode.slice(2)}.${nn}`,
      };
    });
  });
}

const KECAMATAN = buatKecamatan();
const kecamatanByKode = new Map(KECAMATAN.map((k) => [k.kode, k]));

const SATUAN_NAMA: Record<string, string> = {
  kg: "Kilogram",
  liter: "Liter",
  butir: "Butir",
  ikat: "Ikat",
};

const NAMA_PEDAGANG = [
  "Bu Aminah",
  "Pak Syahrul",
  "Kios Sinar Baru",
  "Bu Nurlaila",
  "Pak Zulkifli",
  "UD Berkah",
];

const orangDari = (role: Role): OrangRef => {
  const u = USERS.find((x) => x.role === role) ?? USERS[0];
  return { id: u.id, nama: u.nama };
};

function buatRiwayat(
  k: (typeof KOMODITAS)[number],
  versi: number,
): RiwayatVersiResponse[] {
  const awal = ["2026-01-01", "2026-05-01", "2026-08-01", "2026-10-01"];
  return Array.from({ length: versi }, (_, i) => {
    const v = i + 1;
    return {
      id: `${k.kode}-${v}`,
      versi: v,
      nama: k.nama,
      kategori: k.kategori,
      satuan: k.satuan,
      harga_acuan:
        Math.round((k.hargaAcuan * (1 - 0.04 * (versi - v))) / 100) * 100,
      shelf_life_hari: k.shelfLifeHari,
      pilot: k.pilot,
      berlaku_dari: awal[i],
      berlaku_sampai: v < versi ? awal[i + 1] : null,
      perubahan_id: null,
    };
  });
}

function buatDb(): Db {
  const komoditas: KomoditasDb[] = KOMODITAS.map((k, i) => {
    const versi = 1 + (i % 3);
    return {
      kode: k.kode,
      nama: k.nama,
      kategori: k.kategori,
      satuan: k.satuan,
      harga_acuan: k.hargaAcuan,
      pilot: k.pilot,
      shelf_life_hari: k.shelfLifeHari,
      versi,
      status: i === 3 ? "MENUNGGU_PERSETUJUAN" : "AKTIF",
      riwayat: buatRiwayat(k, versi),
      dirujuk: true,
    };
  });
  const pasar: PasarDb[] = PASAR.map((p) => ({
    id: p.id,
    nama: p.nama,
    kabupaten_kode: p.kabupatenKode,
    lat: p.lat,
    lng: p.lng,
    radius_meter: p.radiusMeter,
    aktif: true,
    // Peunayong dan Lamdingin berada di Kecamatan Kuta Alam.
    kecamatan_kode: p.id === "PSR-1171-01" ? "1171060" : null,
    kecamatan_nama: p.id === "PSR-1171-01" ? "Kuta Alam" : null,
    versi: 1,
    dirujuk: true,
  }));
  const pedagang: PedagangResponse[] = PASAR.flatMap((p, pi) =>
    Array.from({ length: 3 }, (_, n) => ({
      id: String(pi * 3 + n + 1),
      pasar_id: p.id,
      pasar_nama: p.nama,
      kode: `${p.id.replace("PSR", "PDG")}-${n + 1}`,
      nama: n === 2 ? null : NAMA_PEDAGANG[(pi + n) % NAMA_PEDAGANG.length],
      aktif: !(pi % 4 === 3 && n === 1),
      jumlah_entri: ((pi * 7 + n * 13) % 60) + 3,
    })),
  );
  const db: Db = { komoditas, pasar, perubahan: [], pedagang, seq: 100 };

  const pengajuData = orangDari("MANAJER_DATA");
  const pengajuProgram = orangDari("MANAJER_PROGRAM");
  const penyetuju = orangDari("STRATEGIS_BPS");
  const tinjau = komoditas[3];
  tambahPerubahan(db, {
    entitas: "komoditas",
    entitas_id: tinjau.kode,
    entitas_nama: tinjau.nama,
    jenis: "TINJAU",
    versi: tinjau.versi + 1,
    diajukan_oleh: pengajuData,
  });
  const bwm = komoditas.find((k) => k.kode === "BWM")!;
  tambahPerubahan(db, {
    entitas: "komoditas",
    entitas_id: "BWM",
    entitas_nama: bwm.nama,
    jenis: "UBAH",
    versi: bwm.versi + 1,
    alasan: "Penyesuaian harga acuan mengikuti rilis BPS Oktober.",
    perubahan: { harga_acuan: 43500 },
    sebelum: { harga_acuan: bwm.harga_acuan },
    diajukan_oleh: pengajuProgram,
  });
  const lamdingin = pasar.find((p) => p.id === "PSR-1171-02")!;
  tambahPerubahan(db, {
    entitas: "pasar",
    entitas_id: lamdingin.id,
    entitas_nama: lamdingin.nama,
    jenis: "UBAH",
    versi: lamdingin.versi + 1,
    alasan:
      "Pasar berada di Kecamatan Kuta Alam; radius geofence diperlebar karena area parkir.",
    perubahan: { kecamatan_kode: "1171060", radius_meter: 450 },
    sebelum: { kecamatan_kode: null, radius_meter: lamdingin.radius_meter },
    diajukan_oleh: pengajuData,
  });
  const prm = komoditas.find((k) => k.kode === "BRS-PRM")!;
  const awalPrm = prm.riwayat[0];
  const disetujui = tambahPerubahan(db, {
    entitas: "komoditas",
    entitas_id: "BRS-PRM",
    entitas_nama: prm.nama,
    jenis: "UBAH",
    versi: 2,
    alasan: "Harga gabah naik; acuan beras premium disesuaikan.",
    perubahan: { harga_acuan: prm.harga_acuan },
    sebelum: { harga_acuan: awalPrm.harga_acuan },
    diajukan_oleh: pengajuProgram,
  });
  Object.assign(disetujui, {
    status: "DISETUJUI",
    disetujui_oleh: penyetuju,
    disetujui_pada: "2026-05-01T03:20:00Z",
    dibuat_pada: "2026-04-29T08:00:00Z",
  });
  prm.riwayat[1].perubahan_id = disetujui.id;
  const tlr = komoditas.find((k) => k.kode === "TLR")!;
  const ditolak = tambahPerubahan(db, {
    entitas: "komoditas",
    entitas_id: "TLR",
    entitas_nama: tlr.nama,
    jenis: "UBAH",
    versi: tlr.versi + 1,
    alasan: "Telur naik mendadak.",
    perubahan: { harga_acuan: 35000 },
    sebelum: { harga_acuan: tlr.harga_acuan },
    diajukan_oleh: pengajuData,
  });
  Object.assign(ditolak, {
    status: "DITOLAK",
    ditolak_oleh: penyetuju,
    ditolak_pada: "2026-09-12T02:10:00Z",
    alasan_penolakan:
      "Kenaikan sesaat belum didukung data pasar; ajukan lagi bila bertahan.",
    dibuat_pada: "2026-09-10T08:00:00Z",
  });
  return db;
}

let db: Db | null = null;
const state = (): Db => (db ??= buatDb());

/** Mengembalikan keadaan awal (untuk tes). */
export function resetMasterMock(): void {
  db = null;
}

/* ── Usulan: pembentukan ────────────────────────────────────── */

interface BahanPerubahan {
  entitas: "komoditas" | "pasar";
  entitas_id: string;
  entitas_nama: string | null;
  jenis: "TAMBAH" | "UBAH" | "TINJAU";
  versi: number;
  alasan?: string;
  perubahan?: Record<string, unknown>;
  sebelum?: Record<string, unknown>;
  diajukan_oleh: OrangRef;
}

function tambahPerubahan(d: Db, b: BahanPerubahan): PerubahanMasterResponse {
  const perubahan = b.perubahan ?? {};
  const sebelum = b.sebelum ?? {};
  const baris: PerubahanMasterResponse = {
    id: String(++d.seq),
    kode: null,
    entitas: b.entitas,
    entitas_id: b.entitas_id,
    entitas_nama: b.entitas_nama,
    jenis: b.jenis,
    versi: b.versi,
    status: "MENUNGGU",
    alasan: b.alasan ?? null,
    alasan_penolakan: null,
    perubahan,
    sebelum,
    diff: Object.entries(perubahan).map(([bidang, sesudah]) => ({
      bidang,
      sebelum: sebelum[bidang] ?? null,
      sesudah,
    })),
    diajukan_oleh: b.diajukan_oleh,
    dibuat_pada: sekarang(),
    disetujui_oleh: null,
    disetujui_pada: null,
    ditolak_oleh: null,
    ditolak_pada: null,
  };
  d.perubahan.push(baris);
  return baris;
}

const menunggu = (d: Db, entitas: string, id: string) =>
  d.perubahan.find(
    (p) =>
      p.entitas === entitas && p.entitas_id === id && p.status === "MENUNGGU",
  );

/* ── Daftar dan detail ──────────────────────────────────────── */

const sinkir = (db0: KomoditasDb): MasterKomoditasResponse => {
  const { riwayat: _r, dirujuk: _d, ...baris } = db0;
  void _r;
  void _d;
  return baris;
};

const sinkirPasar = (p: PasarDb): MasterPasarResponse => {
  const { dirujuk: _d, ...baris } = p;
  void _d;
  return baris;
};

export const mockKomoditasList = (): MasterKomoditasResponse[] =>
  state().komoditas.map(sinkir);

export const mockWilayahList = (): MasterWilayahResponse[] =>
  KABUPATEN.map((k) => ({
    kode: k.kode,
    nama: namaResmi(k),
    jenis: k.jenis,
    ibukota: k.ibukota,
    lat: k.lat,
    lng: k.lng,
    provinsi_kode: "11",
  }));

export const mockPasarList = (): MasterPasarResponse[] =>
  state().pasar.map(sinkirPasar);

export function mockKomoditasDetail(kode: string): KomoditasDetailResponse {
  const d = state();
  const k = d.komoditas.find((x) => x.kode === kode);
  if (!k) galat(404, "404", "Komoditas tidak ditemukan");
  return {
    ...sinkir(k),
    riwayat_versi: [...k.riwayat].sort((a, b) => b.versi - a.versi),
    perubahan_menunggu: menunggu(d, "komoditas", kode) ?? null,
  };
}

export function mockWilayahDetail(kode: string): WilayahDetailResponse {
  const w = mockWilayahList().find((x) => x.kode === kode);
  if (!w) galat(404, "404", "Wilayah tidak ditemukan");
  return {
    ...w,
    jumlah_kecamatan: KECAMATAN.filter((k) => k.kabupaten_kode === kode).length,
    jumlah_pasar: state().pasar.filter((p) => p.kabupaten_kode === kode).length,
  };
}

export function mockPasarDetail(kode: string): PasarDetailResponse {
  const d = state();
  const p = d.pasar.find((x) => x.id === kode);
  if (!p) galat(404, "404", "Pasar tidak ditemukan");
  const kab = KABUPATEN.find((k) => k.kode === p.kabupaten_kode);
  return {
    ...sinkirPasar(p),
    kabupaten_nama: kab ? namaResmi(kab) : p.kabupaten_kode,
    jumlah_pedagang: d.pedagang.filter((x) => x.pasar_id === kode).length,
  };
}

function potong<T>(
  rows: T[],
  page = 1,
  limit = 25,
): { rows: T[]; total: number } {
  const mulai = (Math.max(1, page) - 1) * limit;
  return { rows: rows.slice(mulai, mulai + limit), total: rows.length };
}

const cocok = (
  q: string | undefined,
  ...kolom: (string | null | undefined)[]
) => {
  const kata = q?.trim().toLowerCase();
  return !kata || kolom.some((k) => k?.toLowerCase().includes(kata));
};

export function mockKecamatanList(
  p: { kabupatenKode?: string; q?: string; page?: number; limit?: number } = {},
) {
  const rows = KECAMATAN.filter(
    (k) =>
      (!p.kabupatenKode || k.kabupaten_kode === p.kabupatenKode) &&
      cocok(p.q, k.nama, k.kode, k.kode_dagri),
  ).sort((a, b) => a.kode.localeCompare(b.kode));
  return potong(rows, p.page, p.limit);
}

export function mockKecamatanDetail(kode: string): KecamatanResponse {
  const k = kecamatanByKode.get(kode);
  if (!k) galat(404, "404", "Kecamatan tidak ditemukan");
  return k;
}

export function mockSatuanList(): SatuanResponse[] {
  const hitung = new Map<string, number>();
  for (const k of state().komoditas)
    hitung.set(k.satuan, (hitung.get(k.satuan) ?? 0) + 1);
  const kode = [...new Set([...Object.keys(SATUAN_NAMA), ...hitung.keys()])];
  return kode.map((s) => ({
    kode: s,
    nama: SATUAN_NAMA[s] ?? s,
    jumlah_komoditas: hitung.get(s) ?? 0,
  }));
}

export function mockPedagangList(
  p: {
    pasarId?: string;
    q?: string;
    aktif?: boolean;
    page?: number;
    limit?: number;
  } = {},
) {
  const rows = state().pedagang.filter(
    (x) =>
      (!p.pasarId || x.pasar_id === p.pasarId) &&
      (p.aktif === undefined || x.aktif === p.aktif) &&
      cocok(p.q, x.kode, x.nama, x.pasar_nama),
  );
  return potong(rows, p.page, p.limit);
}

export function mockPerubahanList(p: PerubahanParams = {}) {
  const semua = state().perubahan.filter(
    (x) =>
      (!p.status || x.status === p.status) &&
      (!p.entitas || x.entitas === p.entitas) &&
      (!p.entitasId || x.entitas_id === p.entitasId),
  );
  // Antrean: terlama dahulu (adil bagi yang menunggu paling lama); riwayat: terbaru dahulu.
  const urut = [...semua].sort((a, b) => {
    const waktu =
      a.dibuat_pada.localeCompare(b.dibuat_pada) || Number(a.id) - Number(b.id);
    return p.status === "MENUNGGU" ? waktu : -waktu;
  });
  return potong(urut, p.page, p.limit);
}

export function mockPerubahanDetail(id: string): PerubahanMasterResponse {
  const p = state().perubahan.find((x) => x.id === id);
  if (!p) galat(404, "404", "Usulan tidak ditemukan");
  return p;
}

/* ── Validasi usulan ────────────────────────────────────────── */

const adaSatuan = (kode: unknown) =>
  typeof kode === "string" && mockSatuanList().some((s) => s.kode === kode);
const bulat = (v: unknown, min: number, max: number) =>
  typeof v === "number" && Number.isInteger(v) && v >= min && v <= max;
const teks = (v: unknown, min: number, max: number) =>
  typeof v === "string" && v.trim().length >= min && v.trim().length <= max;

/** Pesan galat untuk satu bidang, atau null bila nilainya sah. `kab` = kabupaten pasar (untuk kecamatan). */
function galatBidang(
  entitas: "komoditas" | "pasar",
  bidang: string,
  nilai: unknown,
  kab?: string,
): string | null {
  if (entitas === "komoditas") {
    switch (bidang) {
      case "nama":
        return teks(nilai, 1, 255) ? null : "nama harus 1–255 karakter";
      case "kategori":
        return teks(nilai, 1, 100) ? null : "kategori harus 1–100 karakter";
      case "satuan":
        return adaSatuan(nilai) ? null : "satuan tidak ada di daftar satuan";
      case "harga_acuan":
        return bulat(nilai, 1, Number.MAX_SAFE_INTEGER)
          ? null
          : "harga_acuan harus bilangan bulat lebih dari 0";
      case "shelf_life_hari":
        return bulat(nilai, 1, 3650) ? null : "shelf_life_hari harus 1–3650";
      case "pilot":
        return typeof nilai === "boolean"
          ? null
          : "pilot harus true atau false";
    }
  } else {
    switch (bidang) {
      case "nama":
        return teks(nilai, 1, 255) ? null : "nama harus 1–255 karakter";
      case "lat":
        return typeof nilai === "number" && nilai >= -90 && nilai <= 90
          ? null
          : "lat harus antara -90 dan 90";
      case "lng":
        return typeof nilai === "number" && nilai >= -180 && nilai <= 180
          ? null
          : "lng harus antara -180 dan 180";
      case "radius_meter":
        return bulat(nilai, 1, 10000) ? null : "radius_meter harus 1–10000";
      case "aktif":
        return typeof nilai === "boolean"
          ? null
          : "aktif harus true atau false";
      case "kabupaten_kode":
        return KABUPATEN.some((k) => k.kode === nilai)
          ? null
          : "kabupaten_kode tidak dikenal";
      case "kecamatan_kode": {
        if (nilai === null) return null;
        const k =
          typeof nilai === "string" ? kecamatanByKode.get(nilai) : undefined;
        return k && k.kabupaten_kode === kab
          ? null
          : "kecamatan_kode harus kecamatan di kabupaten pasar ini";
      }
    }
  }
  return `bidang ${bidang} tidak dapat diubah`;
}

const WAJIB_TAMBAH = {
  komoditas: [
    "nama",
    "kategori",
    "satuan",
    "harga_acuan",
    "shelf_life_hari",
    "pilot",
  ],
  pasar: ["nama", "kabupaten_kode", "lat", "lng"],
} as const;

/* ── Aksi usulan ────────────────────────────────────────────── */

export function mockAjukan(
  input: UsulanBody,
  aktor: AktorMaster,
): PerubahanMasterResponse {
  const d = state();
  const { entitas, jenis, entitas_id: id, perubahan, alasan } = input;
  if (entitas !== "komoditas" && entitas !== "pasar")
    galat(400, "400", "entitas harus komoditas atau pasar");
  if (jenis !== "UBAH" && jenis !== "TAMBAH")
    galat(400, "400", "jenis harus UBAH atau TAMBAH");
  if (!teks(alasan, 3, 1000))
    galat(400, "400", "alasan wajib diisi, 3–1000 karakter");
  if (!perubahan || typeof perubahan !== "object")
    galat(400, "400", "perubahan wajib berupa objek");

  if (jenis === "TAMBAH") {
    if (!/^[A-Z0-9-]{2,30}$/.test(id))
      galat(
        400,
        "400",
        "entitas_id: huruf kapital, angka, atau tanda hubung, 2–30 karakter",
      );
    const ada =
      entitas === "komoditas"
        ? d.komoditas.some((k) => k.kode === id)
        : d.pasar.some((p) => p.id === id);
    if (ada) galat(409, "KODE_SUDAH_ADA", `Kode ${id} sudah dipakai`);
    if (menunggu(d, entitas, id))
      galat(
        409,
        "USULAN_MENUNGGU",
        `Sudah ada usulan tambah ${id} yang menunggu keputusan`,
      );
    for (const wajib of WAJIB_TAMBAH[entitas])
      if (!(wajib in perubahan))
        galat(400, "400", `${wajib} wajib diisi untuk data baru`);
    const kab = perubahan.kabupaten_kode as string | undefined;
    for (const [bidang, nilai] of Object.entries(perubahan)) {
      const g = galatBidang(entitas, bidang, nilai, kab);
      if (g) galat(400, "400", g);
    }
    const isi =
      entitas === "pasar" ? { radius_meter: 400, ...perubahan } : perubahan;
    return tambahPerubahan(d, {
      entitas,
      entitas_id: id,
      entitas_nama: String(perubahan.nama),
      jenis,
      versi: 1,
      alasan: alasan.trim(),
      perubahan: isi,
      diajukan_oleh: aktor,
    });
  }

  const komoditas =
    entitas === "komoditas"
      ? d.komoditas.find((k) => k.kode === id)
      : undefined;
  const pasar =
    entitas === "pasar" ? d.pasar.find((p) => p.id === id) : undefined;
  const lama = (komoditas ?? pasar) as (KomoditasDb | PasarDb) | undefined;
  if (!lama)
    galat(
      404,
      "404",
      `${entitas === "komoditas" ? "Komoditas" : "Pasar"} ${id} tidak ditemukan`,
    );
  if (komoditas?.status === "DIARSIPKAN")
    galat(
      409,
      "STATUS_TIDAK_SESUAI",
      "Komoditas yang diarsipkan tidak dapat diubah",
    );
  if (menunggu(d, entitas, id) || komoditas?.status === "MENUNGGU_PERSETUJUAN")
    galat(
      409,
      "USULAN_MENUNGGU",
      "Sudah ada usulan yang menunggu keputusan untuk data ini",
    );
  const sebelum: Record<string, unknown> = {};
  const berubah: Record<string, unknown> = {};
  for (const [bidang, nilai] of Object.entries(perubahan)) {
    if (bidang === "kabupaten_kode" || bidang === "kode" || bidang === "id")
      galat(400, "400", `bidang ${bidang} tidak dapat diubah`);
    const g = galatBidang(entitas, bidang, nilai, pasar?.kabupaten_kode);
    if (g) galat(400, "400", g);
    const sekarangNilai =
      (lama as unknown as Record<string, unknown>)[bidang] ?? null;
    if (sekarangNilai !== nilai) {
      berubah[bidang] = nilai;
      sebelum[bidang] = sekarangNilai;
    }
  }
  if (Object.keys(berubah).length === 0)
    galat(
      400,
      "TANPA_PERUBAHAN",
      "Tidak ada bidang yang berbeda dari data saat ini",
    );
  return tambahPerubahan(d, {
    entitas,
    entitas_id: id,
    entitas_nama: String(lama.nama),
    jenis,
    versi: lama.versi + 1,
    alasan: alasan.trim(),
    perubahan: berubah,
    sebelum,
    diajukan_oleh: aktor,
  });
}

function cariUsulan(id: string): PerubahanMasterResponse {
  const p = state().perubahan.find((x) => x.id === id);
  if (!p) galat(404, "404", "Usulan tidak ditemukan");
  return p;
}

function pastikanMenunggu(p: PerubahanMasterResponse) {
  if (p.status !== "MENUNGGU")
    galat(409, "BUKAN_MENUNGGU", "Usulan ini sudah diputuskan atau dibatalkan");
}

function pastikanBukanPengaju(
  p: PerubahanMasterResponse,
  aktor: AktorMaster,
  kata: string,
) {
  if (p.diajukan_oleh?.id === aktor.id)
    galat(
      403,
      "PEMISAHAN_TUGAS",
      `Pengaju tidak boleh ${kata} usulannya sendiri (pemisahan tugas)`,
    );
}

/** Komoditas yang statusnya menunggu karena tinjauan kembali AKTIF bila usulan TINJAU-nya selesai tanpa disetujui. */
function lepasTinjauan(d: Db, p: PerubahanMasterResponse) {
  if (p.jenis !== "TINJAU") return;
  const k = d.komoditas.find((x) => x.kode === p.entitas_id);
  if (k?.status === "MENUNGGU_PERSETUJUAN") k.status = "AKTIF";
}

function terapkan(d: Db, p: PerubahanMasterResponse) {
  const tgl = hariIni();
  if (p.entitas === "komoditas") {
    if (p.jenis === "TAMBAH") {
      const isi = p.perubahan as unknown as Omit<
        MasterKomoditasResponse,
        "kode" | "versi" | "status"
      >;
      const baru: KomoditasDb = {
        kode: p.entitas_id,
        ...isi,
        versi: 1,
        status: "AKTIF",
        riwayat: [],
        dirujuk: false,
      };
      baru.riwayat.push({ ...riwayatDari(baru, 1, tgl), perubahan_id: p.id });
      d.komoditas.push(baru);
      return;
    }
    const k = d.komoditas.find((x) => x.kode === p.entitas_id);
    if (!k) galat(404, "404", "Komoditas tidak ditemukan");
    Object.assign(k, p.perubahan);
    k.versi += 1;
    k.status = "AKTIF";
    const terakhir = k.riwayat.find((v) => v.berlaku_sampai === null);
    if (terakhir) terakhir.berlaku_sampai = tgl;
    k.riwayat.push({ ...riwayatDari(k, k.versi, tgl), perubahan_id: p.id });
    return;
  }
  if (p.jenis === "TAMBAH") {
    const isi = p.perubahan as unknown as Omit<
      MasterPasarResponse,
      "id" | "aktif"
    >;
    const kec = isi.kecamatan_kode
      ? kecamatanByKode.get(isi.kecamatan_kode)
      : undefined;
    d.pasar.push({
      id: p.entitas_id,
      ...isi,
      aktif: true,
      kecamatan_kode: isi.kecamatan_kode ?? null,
      kecamatan_nama: kec?.nama ?? null,
      versi: 1,
      dirujuk: false,
    });
    return;
  }
  const pasar = d.pasar.find((x) => x.id === p.entitas_id);
  if (!pasar) galat(404, "404", "Pasar tidak ditemukan");
  const ubah = p.perubahan ?? {};
  Object.assign(pasar, ubah);
  if ("kecamatan_kode" in ubah)
    pasar.kecamatan_nama =
      kecamatanByKode.get(String(ubah.kecamatan_kode))?.nama ?? null;
  pasar.versi += 1;
}

function riwayatDari(
  k: KomoditasDb,
  versi: number,
  dari: string,
): RiwayatVersiResponse {
  return {
    id: `${k.kode}-${versi}`,
    versi,
    nama: k.nama,
    kategori: k.kategori,
    satuan: k.satuan,
    harga_acuan: k.harga_acuan,
    shelf_life_hari: k.shelf_life_hari,
    pilot: k.pilot,
    berlaku_dari: dari,
    berlaku_sampai: null,
  };
}

function setujuiDi(d: Db, p: PerubahanMasterResponse, aktor: AktorMaster) {
  pastikanMenunggu(p);
  pastikanBukanPengaju(p, aktor, "menyetujui");
  if (p.jenis === "TAMBAH") {
    const ada =
      p.entitas === "komoditas"
        ? d.komoditas.some((k) => k.kode === p.entitas_id)
        : d.pasar.some((x) => x.id === p.entitas_id);
    if (ada) galat(409, "KODE_SUDAH_ADA", `Kode ${p.entitas_id} sudah dipakai`);
  } else {
    const versi = (
      p.entitas === "komoditas"
        ? d.komoditas.find((k) => k.kode === p.entitas_id)
        : d.pasar.find((x) => x.id === p.entitas_id)
    )?.versi;
    if (versi === undefined || versi + 1 !== p.versi)
      galat(
        409,
        "USULAN_KEDALUWARSA",
        "Data sudah berubah sejak usulan diajukan. Tolak usulan ini dan ajukan ulang",
      );
  }
  terapkan(d, p);
  p.status = "DISETUJUI";
  p.disetujui_oleh = aktor;
  p.disetujui_pada = sekarang();
  return p;
}

export function mockSetujui(
  id: string,
  aktor: AktorMaster,
): PerubahanMasterResponse {
  return setujuiDi(state(), cariUsulan(id), aktor);
}

export function mockTolak(
  id: string,
  alasan: string,
  aktor: AktorMaster,
): PerubahanMasterResponse {
  const d = state();
  const p = cariUsulan(id);
  if (!teks(alasan, 3, 1000))
    galat(400, "400", "alasan wajib diisi, 3–1000 karakter");
  pastikanMenunggu(p);
  pastikanBukanPengaju(p, aktor, "menolak");
  p.status = "DITOLAK";
  p.ditolak_oleh = aktor;
  p.ditolak_pada = sekarang();
  p.alasan_penolakan = alasan.trim();
  lepasTinjauan(d, p);
  return p;
}

export function mockBatalkan(
  id: string,
  alasan: string | undefined,
  aktor: AktorMaster,
): PerubahanMasterResponse {
  const d = state();
  const p = cariUsulan(id);
  if (p.diajukan_oleh?.id !== aktor.id)
    galat(403, "403", "Hanya pengaju yang dapat membatalkan usulan ini");
  pastikanMenunggu(p);
  p.status = "DITOLAK";
  p.ditolak_oleh = aktor;
  p.ditolak_pada = sekarang();
  p.alasan_penolakan = alasan?.trim()
    ? `Dibatalkan pengaju: ${alasan.trim()}`
    : "Dibatalkan pengaju";
  lepasTinjauan(d, p);
  return p;
}

export function mockHapus(entitas: "komoditas" | "pasar", kode: string): void {
  const d = state();
  const baris =
    entitas === "komoditas"
      ? d.komoditas.find((k) => k.kode === kode)
      : d.pasar.find((p) => p.id === kode);
  if (!baris)
    galat(
      404,
      "404",
      `${entitas === "komoditas" ? "Komoditas" : "Pasar"} ${kode} tidak ditemukan`,
    );
  const status = (baris as Partial<KomoditasDb>).status;
  if (menunggu(d, entitas, kode) || status === "MENUNGGU_PERSETUJUAN")
    galat(
      409,
      "MASTER_ADA_USULAN",
      "Masih ada usulan yang menunggu untuk data ini. Putuskan atau batalkan usulan itu dulu",
    );
  if (baris.dirujuk)
    galat(
      409,
      "MASTER_DIRUJUK",
      entitas === "komoditas"
        ? `Komoditas ${kode} masih dirujuk data harga dan tugas lapangan, jadi tidak dapat dihapus. Arsipkan komoditas ini sebagai gantinya.`
        : `Pasar ${kode} masih dirujuk data harga dan tugas lapangan, jadi tidak dapat dihapus. Nonaktifkan pasar ini (aktif = tidak) sebagai gantinya.`,
    );
  if (entitas === "komoditas")
    d.komoditas = d.komoditas.filter((k) => k.kode !== kode);
  else d.pasar = d.pasar.filter((p) => p.id !== kode);
}

/** POST /v1/master/komoditas/bulk: transisi dinilai per kode, kegagalan satu kode tidak menggagalkan yang lain. */
export function mockBulkKomoditas(
  aksi: MasterBulkAksi,
  kode: string[],
  aktor: AktorMaster,
): MasterBulkResponse {
  const d = state();
  const rincian = kode.map((kd) => {
    const k = d.komoditas.find((x) => x.kode === kd);
    if (!k)
      return { kode: kd, berhasil: false, pesan: "Komoditas tidak ditemukan" };
    try {
      if (aksi === "ajukan_perubahan") {
        if (k.status !== "AKTIF")
          galat(
            409,
            "STATUS_TIDAK_SESUAI",
            `Status ${k.status} tidak dapat diajukan`,
          );
        tambahPerubahan(d, {
          entitas: "komoditas",
          entitas_id: kd,
          entitas_nama: k.nama,
          jenis: "TINJAU",
          versi: k.versi + 1,
          diajukan_oleh: aktor,
        });
        k.status = "MENUNGGU_PERSETUJUAN";
      } else if (aksi === "setujui") {
        const p = menunggu(d, "komoditas", kd);
        if (!p) galat(409, "BUKAN_MENUNGGU", "Tidak ada usulan yang menunggu");
        setujuiDi(d, p, aktor);
      } else {
        if (k.status === "DIARSIPKAN")
          galat(409, "STATUS_TIDAK_SESUAI", "Komoditas sudah diarsipkan");
        const p = menunggu(d, "komoditas", kd);
        if (p)
          Object.assign(p, {
            status: "DITOLAK",
            ditolak_oleh: aktor,
            ditolak_pada: sekarang(),
            alasan_penolakan: "Komoditas diarsipkan",
          });
        k.status = "DIARSIPKAN";
      }
      return { kode: kd, berhasil: true };
    } catch (err) {
      return {
        kode: kd,
        berhasil: false,
        pesan: err instanceof Error ? err.message : "Ditolak",
      };
    }
  });
  const berhasil = rincian.filter((r) => r.berhasil).length;
  return { berhasil, gagal: rincian.length - berhasil, rincian };
}
