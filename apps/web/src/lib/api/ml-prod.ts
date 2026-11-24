/**
 * Pemetaan payload `ml` (MongoDB datasafe & dataunseen) ke model UI ketika pengguna memilih mode `Prod (Real)`.
 * Menjamin tidak pernah melakukan fallback diam-diam ke data bertanda DEMO dari PostgreSQL.
 */

import type { RingkasanEksekutifResponse } from "./eksekutif";
import type { EwsAlertResponse } from "./ews";
import type { BarisModel, BarisProyeksi, StatusMl } from "./prediksi";
import type { SiklusAktif, SiklusAktifResponse } from "./siklus";
import type { PipelineResponse, SumberDataResponse } from "./sumber";
import { ApiError } from "./client";
import { toRingkasanEksekutif, tanggalLokal } from "./eksekutif";
import { toEwsAlert } from "./ews";
import { toSiklusAktif } from "./siklus";
import { namaBaseline, toKelayakan } from "./ml";
import { labelModelForecast } from "@/lib/ml/label";
import { toPipeline, toSumberData } from "./sumber";
import { KOMODITAS, KABUPATEN } from "@/lib/data/reference";
import type {
  EwsAlert,
  JenisEws,
  PipelineSumber,
  ProyeksiXai,
  RingkasanEksekutif,
  SumberAngka,
  SumberData,
} from "@/lib/data/types";
import type { Severity, Siklus } from "@/lib/design/tokens";

export interface MetaMlResponse {
  tersedia: boolean;
  alasan?: string;
  sumber_data?: string;
  cycle_id?: string;
  cycle_timestamp?: string;
}

export interface SiklusMlResponse {
  cycle_id: string;
  cycle_timestamp: string;
  data_cutoff: string | null;
  status: string;
  target_inflasi: string | null;
  label_target: string | null;
}

export interface PenyumbangMlResponse {
  komoditas_kode: string;
  nama: string;
  andil: number | null;
}

export interface FaktorMlResponse {
  kelompok: string;
  label: string;
  kontribusi: number | null;
  arah: string;
  fitur: { nama: string; kontribusi: number | null }[];
}

export interface PenjelasanMlResponse {
  metode: string;
  faktor: FaktorMlResponse[];
  narasi: string;
}

export interface InflasiWilayahMlResponse {
  wilayah_kode: string;
  tingkat: string;
  periode: string | null;
  kelas: string;
  /** null pada dokumen lama; kelas/siklus hanya boleh dipakai sebagai prediksi bila kelayakan = LAYAK. */
  layak?: boolean | null;
  kelayakan?: string;
  probabilitas: number | null;
  skor_tekanan: number | null;
  siklus: string;
  target: string;
  label_target: string;
  bukan_inflasi_resmi: boolean;
  proyeksi: { yoy: number | null; mtm: number | null; ytd: number | null };
  aktual: {
    yoy: number | null;
    mtm: number | null;
    ytd: number | null;
    periode: string | null;
  };
  penyumbang: PenyumbangMlResponse[];
  penjelasan: PenjelasanMlResponse | null;
}

export interface TitikKurvaMlResponse {
  horizon: number;
  tanggal: string;
  p10: number | null;
  p50: number | null;
  p90: number | null;
  perubahan_p50_pct: number | null;
}

export interface ForecastMlResponse {
  komoditas_kode: string;
  kabupaten_kode: string;
  harga_terakhir: number | null;
  tanggal_terakhir: string | null;
  satuan: string;
  kurva: TitikKurvaMlResponse[];
  model: {
    nama: string;
    fallback: boolean;
    alasan: string | null;
    n_kandidat: number | null;
  };
  metrik: {
    mape: number | null;
    rmse: number | null;
    baseline: string | null;
    mape_baseline: number | null;
    mengalahkan_baseline: boolean | null;
    horizon: number[];
  };
}

export interface ForecastDetailMlResponse extends ForecastMlResponse {
  riwayat: { tanggal: string; nilai: number | null }[];
  penjelasan: PenjelasanMlResponse | null;
}

export interface SinyalEwsMlResponse {
  sinyal_kunci: string;
  severity: Severity;
  jenis: string;
  komoditas_kode: string | null;
  kabupaten_kode: string | null;
  nilai: number | null;
  ambang: number | null;
  metode: string | null;
  judul: string;
  pesan: string;
  aktif: boolean;
  menjadi_alert: boolean;
  alert: { id: string; kode: string | null; status: string } | null;
}

export interface SumberKesegaranMlResponse {
  sumber: string;
  dataset: string;
  kategori: "SEGAR" | "BASI" | "GAGAL" | "DILEWATI" | "BELUM_ADA_RUN_TERJADWAL";
  status: string | null;
  pesan: string;
  last_available_date: string | null;
  umur_hari: number | null;
  run_terakhir: {
    run_id: string;
    crawl_date: string;
    selesai: string | null;
    status: string;
    jumlah_baru: number | null;
  } | null;
}

export interface LampiranMlResponse {
  tersedia: boolean;
  alasan?: string;
  data_utama: string;
  siklus?: SiklusMlResponse;
  inflasi?: MetaMlResponse & {
    target: string | null;
    label_target: string | null;
    bukan_inflasi_resmi: boolean;
    wilayah: InflasiWilayahMlResponse[];
  };
  forecast_ringkas?: MetaMlResponse & {
    kabupaten_kode: string | null;
    wilayah_tersedia: string[];
    items: ForecastMlResponse[];
  };
  forecast?: MetaMlResponse & {
    komoditas_kode: string;
    kabupaten_kode: string;
    wilayah_tersedia: string[];
    item: ForecastDetailMlResponse | null;
  };
  model?: MetaMlResponse & {
    status: string;
    target_inflasi: string | null;
    label_target: string | null;
    klasifikasi: {
      model: string | null;
      f1: number | null;
      roc_auc: number | null;
      brier: number | null;
      n_uji: number | null;
    } | null;
    regresi: {
      pemenang: Record<string, number>;
      mape_median: number | null;
      rmse_median: number | null;
      mape_baseline_median: number | null;
      n_seri: number | null;
      mengalahkan_baseline: number | null;
    } | null;
  };
  ews?: MetaMlResponse & {
    ringkasan: {
      aktif: number;
      kritis: number;
      siaga: number;
      waspada: number;
      info: number;
    };
    sinyal: SinyalEwsMlResponse[];
  };
  kesegaran?: MetaMlResponse & {
    dihitung_pada: string;
    ringkasan: {
      segar: number;
      basi: number;
      gagal: number;
      dilewati: number;
      belum_ada_run_terjadwal: number;
    };
    sumber: SumberKesegaranMlResponse[];
  };
}

const namaKomoditas = (kode: string): string =>
  KOMODITAS.find((k) => k.kode === kode)?.nama ?? kode;

const namaKabupaten = (kode: string | null | undefined): string =>
  (kode && KABUPATEN.find((k) => k.kode === kode)?.nama) ||
  (kode ? `Wilayah ${kode}` : "Provinsi Aceh");

/** Sinyal ini inflasi resmi BPS (bukan indeks harga pangan PIHPS). */
function inflasiResmi(w: Pick<InflasiWilayahMlResponse, "target" | "bukan_inflasi_resmi">): boolean {
  return w.target === "RESMI_BPS" && !w.bukan_inflasi_resmi;
}

/** Kelas/siklus klasifikasi hanya boleh dipakai sebagai prediksi bila LAYAK; dokumen lama tanpa `layak` = TIDAK_DIKETAHUI. */
function klasifikasiLayak(w: Pick<InflasiWilayahMlResponse, "kelayakan" | "layak">): boolean {
  return toKelayakan(w.kelayakan, w.layak) === "LAYAK";
}

function mapSiklus(raw?: string | null): Siklus {
  const s = (raw ?? "").toLowerCase();
  if (s === "tekanan" || s === "defisit") return "tekanan";
  if (s === "surplus") return "surplus";
  return "normal";
}

function mapJenisEws(raw: string): JenisEws {
  switch (raw) {
    case "LONJAKAN_HARGA":
    case "PROYEKSI_LONJAKAN":
    case "ANOMALI_HARGA":
      return "LONJAKAN_HARGA";
    case "TEKANAN_INFLASI":
      return "LONJAKAN_HARGA";
    case "SUMBER_DATA":
      return "ANOMALI_DATA";
    default:
      return "ANOMALI_DATA";
  }
}

export function ringkasanEksekutifKosong(): RingkasanEksekutif {
  return {
    demo: false,
    inflasi: null,
    neraca: null,
    fiskal: null,
    ews: { INFO: 0, WASPADA: 0, SIAGA: 0, KRITIS: 0 },
    intervensi: {
      normal: { aktif: 0, selesai: 0 },
      tekanan: { aktif: 0, selesai: 0 },
      surplus: { aktif: 0, selesai: 0 },
    },
    siklusDominan: "normal",
  };
}

export function toRingkasanEksekutifProd(
  pg: RingkasanEksekutifResponse,
  ml?: LampiranMlResponse,
): RingkasanEksekutif {
  const base = toRingkasanEksekutif({ ...pg, demo: false });

  // Saring keras sisa blok DEMO dari PostgreSQL jika ada.
  if (pg.inflasi?.sumber === "DEMO") base.inflasi = null;
  if (pg.neraca?.sumber === "DEMO") base.neraca = null;
  if (pg.fiskal?.sumber === "DEMO") base.fiskal = null;
  base.demo = false;

  if (ml?.tersedia && ml.inflasi?.tersedia && ml.inflasi.wilayah.length > 0) {
    const prov =
      ml.inflasi.wilayah.find((w) => w.wilayah_kode === "11") ??
      ml.inflasi.wilayah[0];
    const periode =
      prov.periode ?? prov.aktual.periode ?? ml.siklus?.cycle_timestamp;
    const yoy = prov.aktual.yoy ?? prov.proyeksi.yoy;
    // Kartu "inflasi" hanya untuk inflasi resmi BPS: indeks harga pangan (INDEKS_PANGAN) tidak pernah tampil sebagai
    // inflasi (docs/PIPELINE-DATA-ML.md §3), dan ditampilkan terpisah di panel tekanan harga.
    if (periode && yoy !== null && inflasiResmi(prov)) {
      const sumber: SumberAngka = prov.target === "RESMI_BPS" ? "BPS" : "BI";
      base.inflasi = {
        periode: tanggalLokal(periode),
        sumber,
        diimporPada: ml.siklus?.cycle_timestamp ?? tanggalLokal(periode),
        yoy,
        mtm: prov.aktual.mtm ?? prov.proyeksi.mtm ?? 0,
        ytd: prov.aktual.ytd ?? prov.proyeksi.ytd ?? 0,
        sasaran: { bawah: 1.5, atas: 3.5 },
        seri: [
          {
            bulan: tanggalLokal(periode).slice(5, 7),
            yoy,
          },
        ],
        penyumbang: prov.penyumbang.map((p) => ({
          nama: p.nama || namaKomoditas(p.komoditas_kode),
          andil: p.andil ?? 0,
        })),
      };
    }
    // Siklus dari klasifikasi hanya dipakai bila modelnya LAYAK; selain itu tetap siklus dari PostgreSQL.
    if (klasifikasiLayak(prov)) base.siklusDominan = mapSiklus(prov.siklus);
  }

  if (ml?.tersedia && ml.ews?.tersedia) {
    base.ews = {
      KRITIS: ml.ews.ringkasan.kritis,
      SIAGA: ml.ews.ringkasan.siaga,
      WASPADA: ml.ews.ringkasan.waspada,
      INFO: ml.ews.ringkasan.info,
    };
  } else if (pg.demo) {
    base.ews = { KRITIS: 0, SIAGA: 0, WASPADA: 0, INFO: 0 };
  }

  return base;
}

export function toSiklusAktifProd(
  pg: SiklusAktifResponse,
  ml?: LampiranMlResponse,
): SiklusAktif {
  const prov =
    ml?.tersedia && ml.inflasi?.tersedia
      ? (ml.inflasi.wilayah.find((w) => w.wilayah_kode === "11") ??
        ml.inflasi.wilayah[0])
      : undefined;
  // Siklus klasifikasi yang belum layak (TIDAK_LAYAK/TIDAK_DIKETAHUI) bukan prediksi: tema tetap dari PostgreSQL.
  if (ml && prov && klasifikasiLayak(prov)) {
    const siklus = mapSiklus(prov.siklus);
    return {
      siklus,
      sejak: (ml.siklus?.cycle_timestamp ?? pg.sejak).slice(0, 10),
      otomatis: true,
      alasan: {
        ringkasan: `Siklus ${siklus.toUpperCase()} dari model ML datasafe (${ml.siklus?.cycle_id ?? "aktif"}).`,
        pemicu: [],
        ditekan: [],
      },
    };
  }
  return toSiklusAktif(pg);
}

function titikTargetHorizon(
  kurva: TitikKurvaMlResponse[],
): TitikKurvaMlResponse | undefined {
  return (
    kurva.find((t) => t.horizon === 14) ??
    kurva.find((t) => t.horizon === 7) ??
    kurva[0]
  );
}

export function toBarisProyeksiProd(ml?: LampiranMlResponse): BarisProyeksi[] {
  if (!ml?.tersedia || !ml.forecast_ringkas?.tersedia) return [];
  return ml.forecast_ringkas.items.map((item) => {
    const t = titikTargetHorizon(item.kurva);
    const hargaKini = item.harga_terakhir ?? 0;
    const p50 = t?.p50 ?? hargaKini;
    const p10 = t?.p10 ?? p50;
    const p90 = t?.p90 ?? p50;
    const pct =
      t?.perubahan_p50_pct ??
      (hargaKini > 0
        ? Math.round(((p50 - hargaKini) / hargaKini) * 1000) / 10
        : 0);
    const bawahPct =
      hargaKini > 0
        ? Math.round(((p10 - hargaKini) / hargaKini) * 1000) / 10
        : 0;
    const atasPct =
      hargaKini > 0
        ? Math.round(((p90 - hargaKini) / hargaKini) * 1000) / 10
        : 0;
    return {
      id: item.komoditas_kode,
      nama: namaKomoditas(item.komoditas_kode),
      kini: Math.round(hargaKini),
      proyeksi: Math.round(p50),
      pct,
      lo: bawahPct,
      hi: atasPct,
      mape: item.metrik.mape ?? 0,
      demo: false,
    };
  });
}

export function toProyeksiXaiProd(
  komoditasKode: string,
  ml?: LampiranMlResponse,
): ProyeksiXai {
  if (!ml?.tersedia || !ml.forecast?.tersedia || !ml.forecast.item) {
    throw new ApiError(
      404,
      `Belum ada hasil training MongoDB (datasafe) untuk ${namaKomoditas(komoditasKode)}.`,
    );
  }
  const f = ml.forecast.item;
  const target = titikTargetHorizon(f.kurva);
  const hargaKini = f.harga_terakhir ?? 0;
  const pct = target?.perubahan_p50_pct ?? 0;
  const p10 = target?.p10 ?? hargaKini;
  const p90 = target?.p90 ?? hargaKini;
  const bawahPct =
    hargaKini > 0 ? Math.round(((p10 - hargaKini) / hargaKini) * 1000) / 10 : 0;
  const atasPct =
    hargaKini > 0 ? Math.round(((p90 - hargaKini) / hargaKini) * 1000) / 10 : 0;

  const seriHist = f.riwayat.map((r) => ({
    tanggal: r.tanggal,
    aktual: r.nilai ?? undefined,
  }));
  const seriKurva = f.kurva.map((t) => ({
    tanggal: t.tanggal,
    proyeksi: t.p50 ?? undefined,
    rentang:
      t.p10 !== null && t.p90 !== null
        ? ([t.p10, t.p90] as [number, number])
        : undefined,
  }));

  const faktor = (f.penjelasan?.faktor ?? []).map((fk) => ({
    nama: fk.label || fk.kelompok,
    kontribusi: fk.kontribusi ?? 0,
    detail: fk.fitur
      .slice(0, 3)
      .map((ft) => ft.nama)
      .join(", "),
  }));

  return {
    komoditasKode: f.komoditas_kode,
    nama: namaKomoditas(f.komoditas_kode),
    horizonHari: target?.horizon ?? 14,
    perubahanPct: pct,
    interval: [bawahPct, atasPct],
    hargaKini,
    seri: [...seriHist, ...seriKurva],
    faktor,
    narasi:
      f.penjelasan?.narasi ||
      `Model ${f.model.nama} memproyeksikan perubahan ${pct.toFixed(1)}% dalam ${target?.horizon ?? 14} hari.`,
    model: {
      // Baseline ditulis jujur: "Persistensi (belum ada model yang mengalahkan baseline)".
      nama: labelModelForecast({ nama: f.model.nama, adalahBaseline: namaBaseline(f.model.nama) }),
      versi: ml.siklus?.cycle_id ?? "datasafe",
      mape: f.metrik.mape ?? 0,
      dilatih: ml.siklus?.cycle_timestamp ?? f.tanggal_terakhir ?? "",
    },
    rekomendasi: [],
    demo: false,
  };
}

export function toBarisModelProd(ml?: LampiranMlResponse): BarisModel[] {
  if (!ml?.tersedia || !ml.model?.tersedia) return [];
  const rows: BarisModel[] = [];
  const dilatih = ml.siklus?.cycle_timestamp ?? "";
  const cycleId = ml.siklus?.cycle_id ?? "datasafe";

  if (ml.forecast_ringkas?.tersedia) {
    for (const f of ml.forecast_ringkas.items) {
      rows.push({
        id: `${cycleId}:${f.komoditas_kode}:${f.kabupaten_kode}`,
        kode: `${f.komoditas_kode}-${cycleId}`,
        komoditasKode: f.komoditas_kode,
        komoditas: namaKomoditas(f.komoditas_kode),
        versi: `${f.model.nama} (${cycleId})`,
        mape: f.metrik.mape ?? 0,
        drift: 0,
        status: "PRODUKSI",
        dilatihPada: dilatih,
        catatan: f.model.alasan ?? `Wilayah ${namaKabupaten(f.kabupaten_kode)}`,
        baselineMape: f.metrik.mape_baseline,
        mapeRealisasi: null,
        nEvaluasi: 0,
        demo: false,
      });
    }
  }
  return rows;
}

export function toStatusMlProd(ml?: LampiranMlResponse): StatusMl {
  const tersedia = Boolean(ml?.tersedia && ml.model?.tersedia);
  return {
    dikonfigurasi: Boolean(ml?.tersedia),
    tersedia,
    versi: ml?.siklus?.cycle_id ?? "",
  };
}

export function toEwsListProd(
  pgRows: EwsAlertResponse[],
  ml?: LampiranMlResponse,
): EwsAlert[] {
  const nyataPg = pgRows
    .filter((r) => r.sumber.toUpperCase() !== "DEMO")
    .map(toEwsAlert);

  if (!ml?.tersedia || !ml.ews?.tersedia) return nyataPg;

  const adaIds = new Set(nyataPg.map((a) => a.id));
  const sinyalMl: EwsAlert[] = ml.ews.sinyal
    // INFO (termasuk sumber yang sengaja dilewati) tidak pernah menjadi alert (§5.4): tidak ditampilkan sebagai alert.
    .filter((s) => s.menjadi_alert !== false)
    .filter((s) => !(s.alert && adaIds.has(s.alert.id)))
    .map((s) => ({
      id: s.alert?.id ?? s.sinyal_kunci,
      kode: s.alert?.kode ?? s.sinyal_kunci,
      severity: s.severity,
      jenis: mapJenisEws(s.jenis),
      judul: s.judul,
      komoditasKode: s.komoditas_kode ?? undefined,
      komoditasNama: s.komoditas_kode
        ? namaKomoditas(s.komoditas_kode)
        : undefined,
      kabupatenKode: s.kabupaten_kode ?? "11",
      wilayah: namaKabupaten(s.kabupaten_kode),
      nilai: s.nilai ?? 0,
      satuanNilai: "%",
      pesan: s.pesan,
      waktu: ml.siklus?.cycle_timestamp ?? new Date().toISOString(),
      status: (s.alert?.status as EwsAlert["status"]) ?? "BARU",
      skorDampak:
        s.severity === "KRITIS" ? 95 : s.severity === "SIAGA" ? 75 : 50,
      sumber: "ML_DATASAFE",
    }));

  return [...nyataPg, ...sinyalMl];
}

export function toSumberPipelineProd(
  pg: PipelineResponse,
  ml?: LampiranMlResponse,
): PipelineSumber {
  if (ml?.tersedia && ml.kesegaran?.tersedia) {
    const r = ml.kesegaran.ringkasan;
    const totalSumber =
      r.segar + r.basi + r.gagal + r.dilewati + r.belum_ada_run_terjadwal;
    const segarPct =
      totalSumber > 0 ? Math.round((r.segar / totalSumber) * 100) : null;
    return {
      bronze: r.segar + r.basi,
      silver: r.segar,
      gold: ml.siklus ? 1 : 0,
      demo: false,
      mutu: {
        kelengkapan: segarPct,
        akurasi: ml.model?.tersedia ? 100 : null,
        ketepatanWaktu: segarPct,
        konsistensi: ml.tersedia ? 100 : null,
      },
    };
  }
  const dasar = toPipeline({ ...pg, demo: false });
  if (pg.demo) {
    return {
      bronze: 0,
      silver: 0,
      gold: 0,
      demo: false,
      mutu: {
        kelengkapan: null,
        akurasi: null,
        ketepatanWaktu: null,
        konsistensi: null,
      },
    };
  }
  return dasar;
}

export function toSumberListProd(
  pgRows: SumberDataResponse[],
  ml?: LampiranMlResponse,
): SumberData[] {
  const mapped = pgRows.map(toSumberData);
  if (!ml?.tersedia || !ml.kesegaran?.tersedia) return mapped;

  const bySumber = new Map<string, SumberKesegaranMlResponse>();
  for (const s of ml.kesegaran.sumber) {
    if (!bySumber.has(s.sumber.toUpperCase())) {
      bySumber.set(s.sumber.toUpperCase(), s);
    }
  }

  return mapped.map((row) => {
    const key = row.kode.replace(/^SRC-/, "").toUpperCase();
    const k = bySumber.get(key) ?? bySumber.get(row.penyedia.toUpperCase());
    if (!k) return row;
    const statusMap: Record<
      SumberKesegaranMlResponse["kategori"],
      SumberData["status"]
    > = {
      SEGAR: "SEHAT",
      BASI: "TERTUNDA",
      GAGAL: "GAGAL",
      DILEWATI: "DIJEDA",
      BELUM_ADA_RUN_TERJADWAL: "BELUM_AKTIF",
    };
    return {
      ...row,
      status: statusMap[k.kategori],
      terakhir:
        k.run_terakhir?.selesai ?? k.last_available_date ?? row.terakhir,
    };
  });
}
