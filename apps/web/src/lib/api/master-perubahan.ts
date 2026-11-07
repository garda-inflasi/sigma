/**
 * Usulan perubahan master data (komoditas dan pasar): pemetaan respons API Go, label bidang, validasi dan
 * penyusunan isi usulan, pesan galat per kode domain, dan riwayat versi.
 * Endpoint: /v1/master/perubahan, POST .../:id/{setujui,tolak,batalkan}, GET /v1/master/komoditas/:kode.
 */

import type { MasterKomoditasRow, MasterPasarRow } from "@/lib/data/types";
import type { HasilValidasi } from "@/lib/form/skema";
import { formatAngka, formatRupiah } from "@/lib/format";

export type EntitasPerubahan = "komoditas" | "pasar";
export type JenisPerubahan = "TAMBAH" | "UBAH" | "TINJAU";
export type StatusPerubahan = "MENUNGGU" | "DISETUJUI" | "DITOLAK";

export interface OrangRef {
  id: string;
  nama: string;
}

export interface PerubahanMasterResponse {
  id: string;
  kode?: string | null;
  entitas: string;
  entitas_id: string;
  entitas_nama?: string | null;
  jenis: string;
  versi: number;
  status: string;
  alasan?: string | null;
  alasan_penolakan?: string | null;
  perubahan?: Record<string, unknown> | null;
  sebelum?: Record<string, unknown> | null;
  diff?: { bidang: string; sebelum?: unknown; sesudah?: unknown }[] | null;
  diajukan_oleh?: OrangRef | null;
  dibuat_pada: string;
  disetujui_oleh?: OrangRef | null;
  disetujui_pada?: string | null;
  ditolak_oleh?: OrangRef | null;
  ditolak_pada?: string | null;
}

export interface DiffBaris {
  bidang: string;
  /** null = belum ada nilai sebelumnya (usulan TAMBAH atau bidang yang tadinya kosong). */
  sebelum: unknown;
  sesudah: unknown;
}

export interface PerubahanMaster {
  /** ID BIGINT sebagai string desimal. */
  id: string;
  kode: string | null;
  entitas: EntitasPerubahan;
  entitasId: string;
  entitasNama: string | null;
  jenis: JenisPerubahan;
  versi: number;
  status: StatusPerubahan;
  alasan: string | null;
  alasanPenolakan: string | null;
  perubahan: Record<string, unknown>;
  sebelum: Record<string, unknown>;
  diff: DiffBaris[];
  diajukanOleh: OrangRef | null;
  dibuatPada: string;
  disetujuiOleh: OrangRef | null;
  disetujuiPada: string | null;
  ditolakOleh: OrangRef | null;
  ditolakPada: string | null;
}

const JENIS: readonly JenisPerubahan[] = ["TAMBAH", "UBAH", "TINJAU"];
const STATUS: readonly StatusPerubahan[] = ["MENUNGGU", "DISETUJUI", "DITOLAK"];

const orang = (o: OrangRef | null | undefined): OrangRef | null =>
  o && o.id ? { id: String(o.id), nama: o.nama ?? "" } : null;

export function toPerubahanMaster(r: PerubahanMasterResponse): PerubahanMaster {
  const entitas = String(r.entitas).toLowerCase();
  return {
    id: String(r.id),
    kode: r.kode ?? null,
    entitas: (entitas === "pasar" ? "pasar" : "komoditas") as EntitasPerubahan,
    entitasId: r.entitas_id,
    entitasNama: r.entitas_nama ?? null,
    jenis: (JENIS as readonly string[]).includes(r.jenis)
      ? (r.jenis as JenisPerubahan)
      : "UBAH",
    versi: r.versi,
    status: (STATUS as readonly string[]).includes(r.status)
      ? (r.status as StatusPerubahan)
      : "MENUNGGU",
    alasan: r.alasan ?? null,
    alasanPenolakan: r.alasan_penolakan ?? null,
    perubahan: r.perubahan ?? {},
    sebelum: r.sebelum ?? {},
    diff: (r.diff ?? []).map((d) => ({
      bidang: d.bidang,
      sebelum: d.sebelum ?? null,
      sesudah: d.sesudah ?? null,
    })),
    diajukanOleh: orang(r.diajukan_oleh),
    dibuatPada: r.dibuat_pada,
    disetujuiOleh: orang(r.disetujui_oleh),
    disetujuiPada: r.disetujui_pada ?? null,
    ditolakOleh: orang(r.ditolak_oleh),
    ditolakPada: r.ditolak_pada ?? null,
  };
}

/* ── Label dan tampilan ─────────────────────────────────────── */

export const LABEL_BIDANG: Record<string, string> = {
  kode: "Kode",
  nama: "Nama",
  kategori: "Kategori",
  satuan: "Satuan",
  harga_acuan: "Harga acuan",
  shelf_life_hari: "Umur simpan (hari)",
  pilot: "Komoditas pilot",
  kabupaten_kode: "Kabupaten/kota",
  kecamatan_kode: "Kecamatan",
  lat: "Lintang",
  lng: "Bujur",
  radius_meter: "Radius geofence (m)",
  aktif: "Aktif",
  status: "Status",
};

/** "shelf_life_x" tanpa label khusus → "Shelf life x". */
export function labelBidang(bidang: string): string {
  const k = LABEL_BIDANG[bidang];
  if (k) return k;
  const teks = bidang.replace(/_/g, " ").trim();
  return teks.charAt(0).toUpperCase() + teks.slice(1);
}

export const JENIS_LABEL: Record<JenisPerubahan, string> = {
  TAMBAH: "Tambah",
  UBAH: "Ubah",
  TINJAU: "Tinjau status",
};

export const ENTITAS_LABEL: Record<EntitasPerubahan, string> = {
  komoditas: "Komoditas",
  pasar: "Pasar",
};

/** Nilai satu bidang untuk dibaca manusia; nilai kosong tampil "—". */
export function formatNilaiBidang(bidang: string, nilai: unknown): string {
  if (nilai === null || nilai === undefined || nilai === "") return "—";
  if (typeof nilai === "boolean") return nilai ? "Ya" : "Tidak";
  const angka =
    typeof nilai === "string" && bidang === "harga_acuan" && /^\d+$/.test(nilai)
      ? Number(nilai)
      : nilai;
  if (typeof angka === "number") {
    if (bidang === "harga_acuan") return formatRupiah(angka);
    if (bidang === "radius_meter") return `${formatAngka(angka)} m`;
    if (bidang === "shelf_life_hari") return `${formatAngka(angka)} hari`;
    if (bidang === "lat" || bidang === "lng") return String(angka);
    return Number.isInteger(angka) ? formatAngka(angka) : String(angka);
  }
  if (typeof nilai === "string") return nilai;
  return JSON.stringify(nilai);
}

/** Ringkasan satu baris untuk daftar antrean: bidang yang berubah, paling banyak `maks` nama. */
export function ringkasDiff(
  p: Pick<PerubahanMaster, "jenis" | "diff">,
  maks = 2,
): string {
  if (p.jenis === "TINJAU") return "Tinjauan status";
  if (p.jenis === "TAMBAH") return "Data baru";
  if (p.diff.length === 0) return "Tanpa perubahan isi";
  const nama = p.diff.map((d) => labelBidang(d.bidang));
  if (nama.length <= maks) return nama.join(", ");
  return `${nama.slice(0, maks).join(", ")} dan ${nama.length - maks} lainnya`;
}

const AWALAN_BATAL = "Dibatalkan pengaju";

/** Usulan ditolak karena pengaju membatalkannya sendiri (bukan keputusan penyetuju). */
export function adalahDibatalkan(
  p: Pick<PerubahanMaster, "status" | "alasanPenolakan">,
): boolean {
  return (
    p.status === "DITOLAK" && (p.alasanPenolakan ?? "").startsWith(AWALAN_BATAL)
  );
}

export type TonePerubahan = "waspada" | "normal" | "defisit" | "netral";

export function statusTampil(
  p: Pick<PerubahanMaster, "status" | "alasanPenolakan">,
): { tone: TonePerubahan; label: string } {
  if (p.status === "MENUNGGU") return { tone: "waspada", label: "Menunggu" };
  if (p.status === "DISETUJUI") return { tone: "normal", label: "Disetujui" };
  return adalahDibatalkan(p)
    ? { tone: "netral", label: "Dibatalkan" }
    : { tone: "defisit", label: "Ditolak" };
}

/** Pengaju sendiri (id sesi = id pengaju) boleh membatalkan usulan yang masih menunggu. */
export function bolehBatalkan(
  p: Pick<PerubahanMaster, "status" | "diajukanOleh">,
  idPengguna: string | undefined,
): boolean {
  return (
    p.status === "MENUNGGU" && !!idPengguna && p.diajukanOleh?.id === idPengguna
  );
}

/** Pengaju tidak boleh memutuskan usulannya sendiri (pemisahan tugas); server tetap menegakkannya. */
export function pengajuSendiri(
  p: Pick<PerubahanMaster, "diajukanOleh">,
  idPengguna: string | undefined,
): boolean {
  return !!idPengguna && p.diajukanOleh?.id === idPengguna;
}

/* ── Kueri ──────────────────────────────────────────────────── */

/** Query string dari pasangan nama → nilai; nilai kosong dibuang. Hasil diawali "?" atau kosong. */
export function queryString(
  params: Record<string, string | number | boolean | null | undefined>,
): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === "") continue;
    q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

export interface PerubahanParams {
  status?: StatusPerubahan;
  entitas?: EntitasPerubahan;
  entitasId?: string;
  page?: number;
  limit?: number;
}

export function toPerubahanQuery(p: PerubahanParams = {}): string {
  return queryString({
    status: p.status,
    entitas: p.entitas,
    entitas_id: p.entitasId,
    page: p.page,
    limit: p.limit,
  });
}

/* ── Galat ──────────────────────────────────────────────────── */

export const KODE_USULAN_KEDALUWARSA = "USULAN_KEDALUWARSA";
export const KODE_MASTER_DIRUJUK = "MASTER_DIRUJUK";
export const KODE_MASTER_ADA_USULAN = "MASTER_ADA_USULAN";
export const KODE_PEMISAHAN_TUGAS = "PEMISAHAN_TUGAS";

/** Pesan bawaan per kode domain; dipakai bila server tidak mengirim pesan yang berguna. */
export const PESAN_KODE_GALAT: Record<string, string> = {
  USULAN_MENUNGGU:
    "Data ini sudah punya usulan yang menunggu keputusan. Putuskan atau batalkan usulan itu lebih dulu.",
  KODE_SUDAH_ADA: "Kode sudah dipakai data lain. Gunakan kode yang berbeda.",
  STATUS_TIDAK_SESUAI: "Komoditas yang sudah diarsipkan tidak dapat diubah.",
  TANPA_PERUBAHAN:
    "Tidak ada bidang yang berbeda dari data saat ini. Ubah sedikitnya satu bidang.",
  PEMISAHAN_TUGAS:
    "Pengaju tidak boleh memutuskan usulannya sendiri. Minta pemegang izin persetujuan lain.",
  BUKAN_MENUNGGU:
    "Usulan ini sudah diputuskan atau dibatalkan. Muat ulang daftar untuk melihat keadaan terbaru.",
  USULAN_KEDALUWARSA:
    "Data sudah berubah sejak usulan diajukan. Tolak usulan ini dan minta pengaju mengajukan ulang.",
  MASTER_DIRUJUK:
    "Data masih dipakai data lain sehingga tidak dapat dihapus. Arsipkan komoditas atau nonaktifkan pasar sebagai gantinya.",
  MASTER_ADA_USULAN:
    "Masih ada usulan yang menunggu untuk data ini. Putuskan atau batalkan usulan itu dulu.",
};

/** Kode domain dari amplop galat `{ code, message, data }`; undefined bila tidak ada (atau hanya status HTTP). */
export function kodeGalatMaster(err: unknown): string | undefined {
  const payload = (err as { payload?: unknown } | null)?.payload;
  const code = (payload as { code?: unknown } | null | undefined)?.code;
  return typeof code === "string" && /^[A-Z][A-Z0-9_]+$/.test(code)
    ? code
    : undefined;
}

const PESAN_UMUM = "Permintaan gagal. Coba lagi beberapa saat.";

/**
 * Pesan galat untuk pengguna. Pesan dari server didahulukan (sudah menjelaskan penyebabnya); bila kosong atau
 * hanya "Permintaan gagal (…)" dipakai pesan menurut kode domain, lalu menurut status HTTP.
 */
export function pesanGalatMaster(err: unknown): string {
  if (!(err instanceof Error)) return PESAN_UMUM;
  const status = (err as { status?: unknown }).status;
  if (typeof status === "number" && status >= 500)
    return "Server sedang bermasalah. Coba lagi beberapa saat.";
  const pesan = err.message.trim();
  if (pesan && !pesan.startsWith("Permintaan gagal (")) return pesan;
  const kode = kodeGalatMaster(err);
  if (kode && PESAN_KODE_GALAT[kode]) return PESAN_KODE_GALAT[kode];
  if (status === 403) return "Anda tidak punya izin untuk aksi ini.";
  if (status === 404) return "Data tidak ditemukan. Muat ulang halaman.";
  return pesan || PESAN_UMUM;
}

/* ── Isian dan validasi usulan ──────────────────────────────── */

export const BATAS = {
  alasan: { min: 3, max: 1000 },
  nama: { min: 1, max: 255 },
  kategori: { min: 1, max: 100 },
  shelfLife: { min: 1, max: 3650 },
  radius: { min: 1, max: 10000 },
  kodeKomoditas: { min: 2, max: 30 },
} as const;

const POLA_KODE_KOMODITAS = /^[A-Z0-9-]{2,30}$/;
const POLA_ID_PASAR = /^[A-Z0-9-]{2,30}$/;

/** Isian dialog komoditas; nama field sama dengan nama bidang API agar galat 400 server cocok per field. */
export interface IsianKomoditas {
  kode: string;
  nama: string;
  kategori: string;
  satuan: string;
  harga_acuan: string;
  shelf_life_hari: string;
  pilot: boolean;
  alasan: string;
}

export interface IsianPasar {
  id: string;
  nama: string;
  kabupaten_kode: string;
  lat: string;
  lng: string;
  radius_meter: string;
  aktif: boolean;
  /** "" = tanpa kecamatan. */
  kecamatan_kode: string;
  alasan: string;
}

export const isianKomoditasKosong = (): IsianKomoditas => ({
  kode: "",
  nama: "",
  kategori: "",
  satuan: "",
  harga_acuan: "",
  shelf_life_hari: "",
  pilot: false,
  alasan: "",
});

export const isianKomoditasDari = (k: MasterKomoditasRow): IsianKomoditas => ({
  kode: k.kode,
  nama: k.nama,
  kategori: k.kategori,
  satuan: k.satuan,
  harga_acuan: String(k.hargaAcuan),
  shelf_life_hari: String(k.shelfLifeHari),
  pilot: k.pilot,
  alasan: "",
});

export const isianPasarKosong = (): IsianPasar => ({
  id: "",
  nama: "",
  kabupaten_kode: "",
  lat: "",
  lng: "",
  radius_meter: "400",
  aktif: true,
  kecamatan_kode: "",
  alasan: "",
});

export const isianPasarDari = (p: MasterPasarRow): IsianPasar => ({
  id: p.id,
  nama: p.nama,
  kabupaten_kode: p.kabupatenKode,
  lat: String(p.lat),
  lng: String(p.lng),
  radius_meter: String(p.radiusMeter),
  aktif: p.aktif,
  kecamatan_kode: p.kecamatanKode ?? "",
  alasan: "",
});

/** Bilangan bulat dari teks; titik pemisah ribuan dan spasi diabaikan. null bila bukan bilangan bulat. */
export function parseBulat(teks: string): number | null {
  const t = teks.replace(/[.\s]/g, "");
  if (!/^-?\d+$/.test(t)) return null;
  const n = Number(t);
  return Number.isSafeInteger(n) ? n : null;
}

/** Bilangan desimal dari teks (koma atau titik sebagai pemisah desimal). null bila bukan bilangan. */
export function parseDesimal(teks: string): number | null {
  const t = teks.trim().replace(",", ".");
  if (!/^-?\d+(\.\d+)?$/.test(t)) return null;
  const n = Number(t);
  return Number.isFinite(n) ? n : null;
}

type GalatIsian<T> = Partial<Record<keyof T, string>>;

function panjang(
  nilai: string,
  min: number,
  max: number,
  label: string,
): string | undefined {
  const n = nilai.trim().length;
  if (n < min) return `${label} wajib diisi.`;
  if (n > max) return `${label} paling banyak ${max} karakter.`;
  return undefined;
}

export function validasiAlasan(alasan: string): string | undefined {
  const n = alasan.trim().length;
  if (n < BATAS.alasan.min)
    return `Alasan wajib diisi, minimal ${BATAS.alasan.min} karakter.`;
  if (n > BATAS.alasan.max)
    return `Alasan paling banyak ${BATAS.alasan.max} karakter.`;
  return undefined;
}

const TANPA_PERUBAHAN =
  "Tidak ada perubahan. Ubah sedikitnya satu bidang sebelum mengajukan.";

function bersih<T extends object>(g: GalatIsian<T>): GalatIsian<T> | null {
  return Object.values(g).some(Boolean) ? g : null;
}

/** Aturan komoditas yang sama dengan API: nama, kategori, satuan, harga acuan, umur simpan, dan (tambah) kode. */
export function validasiKomoditas(
  i: IsianKomoditas,
  opsi: { awal?: MasterKomoditasRow; satuan?: readonly string[] },
): HasilValidasi {
  const g: GalatIsian<IsianKomoditas> = {};
  if (!opsi.awal) {
    const kode = i.kode.trim();
    g.kode = !POLA_KODE_KOMODITAS.test(kode)
      ? `Kode ${BATAS.kodeKomoditas.min}–${BATAS.kodeKomoditas.max} karakter: huruf kapital, angka, atau tanda hubung.`
      : undefined;
  }
  g.nama = panjang(i.nama, BATAS.nama.min, BATAS.nama.max, "Nama");
  g.kategori = panjang(
    i.kategori,
    BATAS.kategori.min,
    BATAS.kategori.max,
    "Kategori",
  );
  if (!i.satuan) g.satuan = "Pilih satuan.";
  else if (opsi.satuan && !opsi.satuan.includes(i.satuan))
    g.satuan = "Satuan tidak ada di daftar satuan.";
  const harga = parseBulat(i.harga_acuan);
  if (harga === null || harga <= 0)
    g.harga_acuan = "Harga acuan harus bilangan bulat lebih dari 0.";
  const umur = parseBulat(i.shelf_life_hari);
  if (umur === null || umur < BATAS.shelfLife.min || umur > BATAS.shelfLife.max)
    g.shelf_life_hari = `Umur simpan ${BATAS.shelfLife.min}–${BATAS.shelfLife.max} hari.`;
  const galatAlasan = validasiAlasan(i.alasan);
  if (galatAlasan) g.alasan = galatAlasan;
  const galat = bersih(g);
  if (galat) return galat;
  if (opsi.awal && Object.keys(perubahanKomoditas(i, opsi.awal)).length === 0)
    return TANPA_PERUBAHAN;
  return null;
}

/** Aturan pasar yang sama dengan API: nama, koordinat, radius, kecamatan, dan (tambah) id serta kabupaten. */
export function validasiPasar(
  i: IsianPasar,
  opsi: { awal?: MasterPasarRow },
): HasilValidasi {
  const g: GalatIsian<IsianPasar> = {};
  if (!opsi.awal) {
    if (!POLA_ID_PASAR.test(i.id.trim()))
      g.id =
        "ID pasar: huruf kapital, angka, atau tanda hubung (mis. PSR-1171-05).";
    if (!/^\d{4}$/.test(i.kabupaten_kode))
      g.kabupaten_kode = "Pilih kabupaten/kota.";
  }
  g.nama = panjang(i.nama, BATAS.nama.min, BATAS.nama.max, "Nama");
  const lat = parseDesimal(i.lat);
  if (lat === null || lat < -90 || lat > 90)
    g.lat = "Lintang harus antara -90 dan 90.";
  const lng = parseDesimal(i.lng);
  if (lng === null || lng < -180 || lng > 180)
    g.lng = "Bujur harus antara -180 dan 180.";
  const radius = parseBulat(i.radius_meter);
  if (radius === null || radius < BATAS.radius.min || radius > BATAS.radius.max)
    g.radius_meter = `Radius ${BATAS.radius.min}–${BATAS.radius.max} meter.`;
  if (i.kecamatan_kode && !/^\d{7}$/.test(i.kecamatan_kode))
    g.kecamatan_kode = "Kode kecamatan 7 digit.";
  const galatAlasan = validasiAlasan(i.alasan);
  if (galatAlasan) g.alasan = galatAlasan;
  const galat = bersih(g);
  if (galat) return galat;
  if (opsi.awal && Object.keys(perubahanPasar(i, opsi.awal)).length === 0)
    return TANPA_PERUBAHAN;
  return null;
}

/**
 * Isi `perubahan` komoditas. Tanpa `awal` (usulan tambah): keenam bidang. Dengan `awal`: hanya bidang yang
 * berbeda dari data saat ini. Panggil sesudah validasi lolos.
 */
export function perubahanKomoditas(
  i: IsianKomoditas,
  awal?: MasterKomoditasRow,
): Record<string, unknown> {
  const baru: Record<string, unknown> = {
    nama: i.nama.trim(),
    kategori: i.kategori.trim(),
    satuan: i.satuan,
    harga_acuan: parseBulat(i.harga_acuan),
    shelf_life_hari: parseBulat(i.shelf_life_hari),
    pilot: i.pilot,
  };
  if (!awal) return baru;
  const lama: Record<string, unknown> = {
    nama: awal.nama,
    kategori: awal.kategori,
    satuan: awal.satuan,
    harga_acuan: awal.hargaAcuan,
    shelf_life_hari: awal.shelfLifeHari,
    pilot: awal.pilot,
  };
  return Object.fromEntries(
    Object.entries(baru).filter(([k, v]) => v !== lama[k]),
  );
}

/**
 * Isi `perubahan` pasar. Tambah: nama, kabupaten, koordinat, radius, dan kecamatan bila dipilih. Ubah: hanya
 * bidang yang berbeda; `kecamatan_kode` null mengosongkan kecamatan.
 */
export function perubahanPasar(
  i: IsianPasar,
  awal?: MasterPasarRow,
): Record<string, unknown> {
  const lat = parseDesimal(i.lat);
  const lng = parseDesimal(i.lng);
  const radius = parseBulat(i.radius_meter);
  if (!awal) {
    return {
      nama: i.nama.trim(),
      kabupaten_kode: i.kabupaten_kode,
      lat,
      lng,
      radius_meter: radius,
      ...(i.kecamatan_kode ? { kecamatan_kode: i.kecamatan_kode } : {}),
    };
  }
  const baru: Record<string, unknown> = {
    nama: i.nama.trim(),
    lat,
    lng,
    radius_meter: radius,
    aktif: i.aktif,
    kecamatan_kode: i.kecamatan_kode || null,
  };
  const lama: Record<string, unknown> = {
    nama: awal.nama,
    lat: awal.lat,
    lng: awal.lng,
    radius_meter: awal.radiusMeter,
    aktif: awal.aktif,
    kecamatan_kode: awal.kecamatanKode,
  };
  return Object.fromEntries(
    Object.entries(baru).filter(([k, v]) => v !== lama[k]),
  );
}

/** Badan POST /v1/master/perubahan. */
export interface UsulanBody {
  entitas: EntitasPerubahan;
  jenis: "UBAH" | "TAMBAH";
  entitas_id: string;
  perubahan: Record<string, unknown>;
  alasan: string;
}

export function bodyUsulan(a: {
  entitas: EntitasPerubahan;
  jenis: "UBAH" | "TAMBAH";
  entitasId: string;
  perubahan: Record<string, unknown>;
  alasan: string;
}): UsulanBody {
  return {
    entitas: a.entitas,
    jenis: a.jenis,
    entitas_id: a.entitasId.trim(),
    perubahan: a.perubahan,
    alasan: a.alasan.trim(),
  };
}

/* ── Riwayat versi komoditas ────────────────────────────────── */

export interface RiwayatVersiResponse {
  id: string | number;
  versi: number;
  nama: string;
  kategori: string;
  satuan: string;
  harga_acuan: number;
  shelf_life_hari: number;
  pilot: boolean;
  berlaku_dari: string;
  berlaku_sampai: string | null;
  perubahan_id?: string | number | null;
}

export interface RiwayatVersi {
  id: string;
  versi: number;
  nama: string;
  kategori: string;
  satuan: string;
  hargaAcuan: number;
  shelfLifeHari: number;
  pilot: boolean;
  berlakuDari: string;
  /** null = versi yang sedang berlaku. */
  berlakuSampai: string | null;
  perubahanId: string | null;
}

export function toRiwayatVersi(r: RiwayatVersiResponse): RiwayatVersi {
  return {
    id: String(r.id),
    versi: r.versi,
    nama: r.nama,
    kategori: r.kategori,
    satuan: r.satuan,
    hargaAcuan: r.harga_acuan,
    shelfLifeHari: r.shelf_life_hari,
    pilot: r.pilot,
    berlakuDari: r.berlaku_dari,
    berlakuSampai: r.berlaku_sampai ?? null,
    perubahanId: r.perubahan_id == null ? null : String(r.perubahan_id),
  };
}

const BIDANG_VERSI: [bidang: string, ambil: (v: RiwayatVersi) => unknown][] = [
  ["nama", (v) => v.nama],
  ["kategori", (v) => v.kategori],
  ["satuan", (v) => v.satuan],
  ["harga_acuan", (v) => v.hargaAcuan],
  ["shelf_life_hari", (v) => v.shelfLifeHari],
  ["pilot", (v) => v.pilot],
];

export interface VersiDenganPerubahan {
  versi: RiwayatVersi;
  /** Bidang yang berbeda dari versi sebelumnya; kosong untuk versi pertama atau versi tanpa beda isi. */
  perubahan: DiffBaris[];
}

/**
 * Riwayat dengan beda tiap versi terhadap versi sebelumnya (nomor versi di bawahnya). Urutan hasil sama
 * dengan masukan; masukan boleh dalam urutan apa pun.
 */
export function perubahanAntarVersi(
  riwayat: readonly RiwayatVersi[],
): VersiDenganPerubahan[] {
  const naik = [...riwayat].sort((a, b) => a.versi - b.versi);
  const beda = new Map<string, DiffBaris[]>();
  naik.forEach((v, i) => {
    const sebelum = naik[i - 1];
    beda.set(
      v.id,
      sebelum
        ? BIDANG_VERSI.filter(([, ambil]) => ambil(sebelum) !== ambil(v)).map(
            ([bidang, ambil]) => ({
              bidang,
              sebelum: ambil(sebelum),
              sesudah: ambil(v),
            }),
          )
        : [],
    );
  });
  return riwayat.map((v) => ({ versi: v, perubahan: beda.get(v.id) ?? [] }));
}

/** Rentang berlaku "d MMM yyyy – sekarang" disusun pemanggil; fungsi ini hanya memilah versi aktif. */
export const versiBerjalan = (
  riwayat: readonly RiwayatVersi[],
): RiwayatVersi | undefined => riwayat.find((v) => v.berlakuSampai === null);
