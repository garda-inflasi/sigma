/**
 * Fixture respons nyata API Go untuk tes kontrak dashboard operasional.
 *
 * Asal: API Go (`apps/api`, `go run ./cmd/api`) di atas basis data demo hasil `make db/migrate`, `make db/seed/reference`,
 * dan `make db/seed/demo`, diambil lewat login akun demo (kata sandi di accounts.md). Tiap berkas adalah isi `data` dari
 * amplop `{code,message,data}` saja: tanpa header, token, email, atau telepon (API sendiri hanya mengirim nama tampilan
 * dan ID publik petugas; semua nama di sini nama akun demo fiktif dari seed).
 *
 *   provinsi/     peran tingkat provinsi (strategis.fmipa): seluruh Aceh
 *   koordinator/  Koordinator (koordinator.lapangan, kabupaten tugas 1171): hanya kabupatennya, kartu KPI bernada `data`
 *
 * Berkas biasa adalah GET /v1/dashboard/operasional/{kpi,heatmap,aktivitas-lapangan,pembanding-harga,cuaca,
 * aktivitas-petugas,stok,produsen,logistik} dan GET /v1/siklus/aktif tanpa parameter. Variannya:
 *   - `*-terfilter`: kabupaten_kode=1171, komoditas_kode=CBM-KRT, dari=2026-11-01, sampai=2026-11-05;
 *   - `*-kosong`: jawaban nyata untuk kabupaten di luar cakupan Koordinator (kabupaten_kode=1108);
 *   - `siklus-aktif-otomatis-*`: siklus yang dibuka mesin penentuan siklus (aturan_siklus diubah di basis data sementara
 *     agar mesin menetapkan TEKANAN, NORMAL, atau SURPLUS; siklus seed tidak pernah otomatis dan beralasan null);
 *   - `aktivitas-petugas-tanpa-entri`: ditambah satu petugas baru tanpa entri (entri_terakhir_pada null).
 *
 * Tanggal dalam berkas mengikuti hari pengambilan (2026-11-10), jadi tes tidak boleh bergantung pada "hari ini".
 * Memperbarui: nyalakan API di atas basis data demo yang sama, ambil ulang endpoint di atas dengan kedua akun, dan simpan
 * `data`-nya; tes kontrak (dashboard-kontrak.test.ts) memberi tahu bila bentuknya berubah.
 */
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

export type PersonaFixture = "provinsi" | "koordinator";

/** Membaca `__fixtures__/operasional/<persona>/<nama>.json`. */
export function muatFixture<T = unknown>(
  persona: PersonaFixture,
  nama: string,
): T {
  return JSON.parse(
    readFileSync(
      resolve(import.meta.dirname, "operasional", persona, `${nama}.json`),
      "utf8",
    ),
  ) as T;
}
