/**
 * Siklus ekonomi aktif: GET /v1/siklus/aktif. Siklus dominan wilayah ini menggerakkan warna ambient seluruh UI.
 * Sejak 08-B siklus ditetapkan mesin penentuan siklus (aturan, alert EWS, dan stok cold storage): `otomatis` menandai
 * baris yang dibuka mesin, dan `alasan` adalah foto keputusannya. Keduanya baru: `alasan` null untuk siklus dari seed
 * atau penetapan manual, dan server lama belum mengirim keduanya.
 */

import type { Siklus } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";

/** Satu aturan di dalam alasan: nilai ukur terhadap ambangnya. */
export interface PemicuSiklusResponse {
  /** Kode aturan, mis. SKL-TKN-LONJAKAN-HARGA. */
  aturan: string;
  siklus: Exclude<Siklus, "normal">;
  indikator: "ALERT_EWS" | "UTILISASI_STOK";
  /** Hasil ukur: jumlah alert, atau persen terisi. */
  nilai: number;
  /** Jumlah alert minimum, atau persen, yang membuat aturan memicu. */
  ambang: number;
  satuan: "alert" | "%";
  /** Angka kecil lebih diutamakan. */
  prioritas: number;
  /** Kode alert contoh (maksimal lima, hanya ALERT_EWS); selalu larik. */
  rujukan: string[];
  /** Keterangan angka mis. "780 dari 950 ton"; kosong bila tak ada. */
  rincian: string;
}

export interface AlasanSiklusResponse {
  /** Kalimat ringkas keputusan, mis. "Siklus awal TEKANAN: 6 alert LONJAKAN_HARGA ≥ SIAGA." */
  ringkasan: string;
  /** Aturan yang memicu siklus terpilih (kosong untuk NORMAL). */
  pemicu: PemicuSiklusResponse[];
  /** Aturan siklus lain yang ikut memicu tetapi kalah prioritas. */
  ditekan: PemicuSiklusResponse[];
}

export interface SiklusAktifResponse {
  siklus: Siklus;
  /** Tanggal mulai, YYYY-MM-DD waktu Aceh. */
  sejak: string;
  /** true bila dibuka mesin penentuan siklus; false untuk seed atau penetapan manual. */
  otomatis: boolean;
  alasan: AlasanSiklusResponse | null;
}

export type PemicuSiklus = PemicuSiklusResponse;

export interface AlasanSiklus {
  ringkasan: string;
  pemicu: PemicuSiklus[];
  ditekan: PemicuSiklus[];
}

export interface SiklusAktif {
  siklus: Siklus;
  sejak: string;
  otomatis: boolean;
  /** null untuk siklus yang tidak otomatis (seed atau manual): tidak ada penjelasan mesin. */
  alasan: AlasanSiklus | null;
}

function toPemicu(p: PemicuSiklusResponse): PemicuSiklus {
  return {
    aturan: p.aturan,
    siklus: p.siklus,
    indikator: p.indikator,
    nilai: p.nilai,
    ambang: p.ambang,
    satuan: p.satuan,
    prioritas: p.prioritas,
    rujukan: p.rujukan ?? [],
    rincian: p.rincian ?? "",
  };
}

/**
 * Respons siklus aktif → model UI. `otomatis` dan `alasan` boleh belum ada (server lama): dianggap siklus tidak
 * otomatis tanpa alasan, jadi tampilan lama tetap benar.
 */
export function toSiklusAktif(r: SiklusAktifResponse): SiklusAktif {
  return {
    siklus: r.siklus,
    sejak: r.sejak,
    otomatis: r.otomatis ?? false,
    alasan: r.alasan
      ? {
          ringkasan: r.alasan.ringkasan,
          pemicu: (r.alasan.pemicu ?? []).map(toPemicu),
          ditekan: (r.alasan.ditekan ?? []).map(toPemicu),
        }
      : null,
  };
}

/**
 * "YYYY-MM-DD" → "10 Nov 2026". Tanggal dibaca sebagai tanggal setempat (bukan UTC) supaya hari tidak bergeser di
 * zona waktu lain; null bila bukan tanggal yang sah (keterangan tooltip tidak boleh merusak tajuk halaman).
 */
function teksTanggal(ymd: string): string | null {
  const d = new Date(`${ymd}T00:00:00`);
  return Number.isNaN(d.getTime()) ? null : formatTanggal(d, "d MMM yyyy");
}

/**
 * Keterangan siklus untuk tooltip: sejak kapan dan, untuk siklus otomatis, bahwa mesin yang menentukannya beserta
 * ringkasan alasannya. Siklus dari seed atau penetapan manual hanya menyebut tanggal mulainya.
 */
export function keteranganSiklus(s: SiklusAktif): string {
  const tanggal = teksTanggal(s.sejak);
  const sejak = tanggal ? ` sejak ${tanggal}` : "";
  if (!s.otomatis) return tanggal ? `Sejak ${tanggal}` : "";
  const dasar = `Ditentukan otomatis${sejak}`;
  return s.alasan?.ringkasan ? `${dasar}. ${s.alasan.ringkasan}` : dasar;
}
