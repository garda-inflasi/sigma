/**
 * Pemetaan ringkasan eksekutif dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/dashboard/eksekutif.
 */

import type { RingkasanEksekutif, SumberAngka } from "@/lib/data/types";
import type { Severity, Siklus } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";

export interface RingkasanEksekutifResponse {
  demo: boolean;
  inflasi: {
    periode: string;
    sumber: string;
    diimpor_pada: string;
    ihk: number;
    yoy: number;
    mtm: number;
    ytd: number;
    sasaran: { tahun: number; bawah: number; atas: number };
    seri: { periode: string; yoy: number }[];
    penyumbang: { kode: string; nama: string; andil: number }[];
  } | null;
  neraca: {
    periode: string;
    sumber: string;
    items: {
      komoditas_kode: string;
      nama: string;
      produksi_ton: number;
      konsumsi_ton: number;
      stok_ton: number | null;
    }[];
  } | null;
  fiskal: {
    sumber: string;
    per_tanggal: string;
    pagu: number;
    realisasi: number;
    skor: number;
    status: string;
    instrumen: { nama: string; pagu: number; realisasi: number }[];
  } | null;
  ews: Record<string, number>;
  intervensi: Record<string, { aktif: number; selesai: number }>;
  siklus_dominan: string;
}

const SUMBER = ["BPS", "BI", "DJPB", "BAPANAS", "DEMO"] as const;
const FISKAL = ["MEMADAI", "TERBATAS", "KRITIS"] as const;
type StatusFiskal = "MEMADAI" | "TERBATAS" | "KRITIS";
const SIKLUS: readonly Siklus[] = ["normal", "tekanan", "surplus"];
const SEVERITY: readonly Severity[] = ["INFO", "WASPADA", "SIAGA", "KRITIS"];

/** Tanggal "YYYY-MM-DD" dari API → ISO lokal siang hari, agar bulan/tanggal tidak bergeser oleh zona waktu. */
export function tanggalLokal(tanggal: string): string {
  return `${tanggal.slice(0, 10)}T12:00:00`;
}

/** Sumber tak dikenal diperlakukan sebagai DEMO: lebih baik berlabel demo daripada tampak resmi. */
function toSumber(sumber: string): SumberAngka {
  return (SUMBER as readonly string[]).includes(sumber)
    ? (sumber as SumberAngka)
    : "DEMO";
}

export function toRingkasanEksekutif(
  r: RingkasanEksekutifResponse,
): RingkasanEksekutif {
  return {
    demo: r.demo,
    inflasi: r.inflasi && {
      periode: tanggalLokal(r.inflasi.periode),
      sumber: toSumber(r.inflasi.sumber),
      diimporPada: r.inflasi.diimpor_pada,
      yoy: r.inflasi.yoy,
      mtm: r.inflasi.mtm,
      ytd: r.inflasi.ytd,
      sasaran: { bawah: r.inflasi.sasaran.bawah, atas: r.inflasi.sasaran.atas },
      seri: r.inflasi.seri.map((t) => ({
        bulan: formatTanggal(tanggalLokal(t.periode), "MMM"),
        yoy: t.yoy,
      })),
      penyumbang: r.inflasi.penyumbang.map((p) => ({
        nama: p.nama,
        andil: p.andil,
      })),
    },
    neraca: r.neraca && {
      periode: tanggalLokal(r.neraca.periode),
      sumber: toSumber(r.neraca.sumber),
      items: r.neraca.items.map((i) => ({
        komoditasKode: i.komoditas_kode,
        nama: i.nama,
        produksi: i.produksi_ton,
        kebutuhan: i.konsumsi_ton,
      })),
    },
    fiskal: r.fiskal && {
      sumber: toSumber(r.fiskal.sumber),
      pagu: r.fiskal.pagu,
      realisasi: r.fiskal.realisasi,
      skor: r.fiskal.skor,
      status: (FISKAL as readonly string[]).includes(r.fiskal.status)
        ? (r.fiskal.status as StatusFiskal)
        : "KRITIS",
      perTanggal: tanggalLokal(r.fiskal.per_tanggal),
      instrumen: r.fiskal.instrumen.map((i) => ({
        nama: i.nama,
        pagu: i.pagu,
        realisasi: i.realisasi,
      })),
    },
    ews: Object.fromEntries(SEVERITY.map((s) => [s, r.ews[s] ?? 0])) as Record<
      Severity,
      number
    >,
    intervensi: Object.fromEntries(
      SIKLUS.map((s) => [s, r.intervensi[s] ?? { aktif: 0, selesai: 0 }]),
    ) as Record<Siklus, { aktif: number; selesai: number }>,
    siklusDominan: (SIKLUS as readonly string[]).includes(r.siklus_dominan)
      ? (r.siklus_dominan as Siklus)
      : "normal",
  };
}
