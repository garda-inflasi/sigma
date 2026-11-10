/**
 * Data mock dashboard operasional (mode tanpa backend). Ringkasan 08-B dibuat dalam bentuk respons API (snake_case)
 * lalu dipetakan oleh pemeta yang sama dengan data nyata. Daftar petugas bersumber dari referensi statis (`USERS`),
 * yang hanya dipakai di sini: komponen membaca petugas dari API (atau dari mock ini), bukan dari referensi.
 */

import { hashSeed, namaWilayah, seriHarga } from "@/lib/data/generators";
import {
  COLD_STORAGE,
  KABUPATEN,
  KOMODITAS,
  USERS,
  kabupatenByKode,
  komoditasByKode,
} from "@/lib/data/reference";
import {
  aktivitasMock,
  type AktivitasLapangan,
  type SaringanOperasional,
} from "./dashboard";
import type {
  AktivitasPetugasItemResponse,
  AktivitasPetugasResponse,
  FasilitasLogistikResponse,
  GerakJenisResponse,
  LogistikResponse,
  PergerakanResponse,
  ProdusenKomoditasResponse,
  ProdusenResponse,
  RuteAktifResponse,
  StokKomoditasResponse,
  StokResponse,
} from "./dashboard-operasional";
import { mockColdStorage, mockEntri, mockRute } from "./mock";

/** Jumlah komoditas aktif: target harian tiap petugas (sama dengan target API). */
const KOMODITAS_AKTIF = 14;
const JENDELA_HARI = 7;
const AMBANG_KEDALUWARSA_HARI = 7;
const AMBANG_RISIKO = 0.6;
const WIB_MS = 7 * 3_600_000;
const HARI_MS = 86_400_000;

const bulat1 = (n: number) => Math.round(n * 10) / 10;

/** Tanggal Aceh (YYYY-MM-DD) `mundur` hari sebelum hari ini. */
const tanggalAceh = (mundur = 0) =>
  new Date(Date.now() + WIB_MS - mundur * HARI_MS).toISOString().slice(0, 10);

/** Nama resmi seperti respons API: "KOTA BANDA ACEH", "KABUPATEN ACEH BESAR". */
function namaResmi(kode: string): string {
  const k = kabupatenByKode[kode];
  return k
    ? `${k.jenis === "Kota" ? "KOTA" : "KABUPATEN"} ${k.nama.toUpperCase()}`
    : kode;
}

// ── Aktivitas per wilayah ─────────────────────────────────────────────

/** Jumlah petugas lapangan mock per kabupaten, untuk target aktivitas harian. */
function petugasPerKabupaten(): Map<string, number> {
  const n = new Map<string, number>();
  for (const u of USERS)
    if (u.role === "OPS_MAHASISWA" && u.kabupatenKode)
      n.set(u.kabupatenKode, (n.get(u.kabupatenKode) ?? 0) + 1);
  return n;
}

/** Capaian entri per kabupaten dari entri mock (per petugas: lihat mockAktivitasPetugas). */
export const mockAktivitasLapangan = (): AktivitasLapangan[] =>
  aktivitasMock(mockEntri(), petugasPerKabupaten(), KOMODITAS_AKTIF);

// ── Aktivitas per petugas ─────────────────────────────────────────────

/**
 * Petugas lapangan mock beserta capaiannya, dari entri mock yang sama dengan daftar entri harga (jadi jumlahnya
 * cocok dengan mockAktivitasLapangan). Terurut seperti API: nama kabupaten, nama petugas.
 */
export function mockAktivitasPetugas(
  kabupatenKode?: string,
): AktivitasPetugasResponse {
  const entri = mockEntri();
  const petugas: AktivitasPetugasItemResponse[] = USERS.filter(
    (u) =>
      u.role === "OPS_MAHASISWA" &&
      u.kabupatenKode &&
      (!kabupatenKode || u.kabupatenKode === kabupatenKode),
  )
    .map((u) => {
      const milik = entri.filter((e) => e.petugasId === u.id);
      const h = hashSeed(u.id);
      return {
        id: u.id,
        nama: u.nama,
        kabupaten_kode: u.kabupatenKode!,
        kabupaten_nama: namaResmi(u.kabupatenKode!),
        entri_hari_ini: milik.filter((e) => e.tipe === "KONSUMEN").length,
        target_harian: KOMODITAS_AKTIF,
        menunggu_validasi: milik.filter((e) => e.status === "DIAJUKAN").length,
        ditolak_jendela: milik.filter((e) => e.status === "DITOLAK").length,
        entri_terakhir_pada:
          milik
            .map((e) => e.dicatatPada)
            .sort()
            .at(-1) ?? null,
        tugas_aktif: 1 + (h % 3),
        tugas_selesai: h % 4,
        tugas_terlambat: h % 5 === 0 ? 1 : 0,
      };
    })
    .sort(
      (a, b) =>
        a.kabupaten_nama.localeCompare(b.kabupaten_nama) ||
        a.nama.localeCompare(b.nama),
    );
  return { tanggal: tanggalAceh(), jendela_hari: JENDELA_HARI, petugas };
}

// ── Stok ──────────────────────────────────────────────────────────────

/** Stok berlaku dari lot cold storage mock: lot di fasilitas OPERASIONAL; fasilitas perawatan masuk `di_luar_operasional_ton`. */
export function mockStok(saringan: SaringanOperasional = {}): StokResponse {
  const { fasilitas, stok } = mockColdStorage();
  const dalam = fasilitas.filter(
    (f) =>
      !saringan.kabupatenKode || f.kabupatenKode === saringan.kabupatenKode,
  );
  const idDalam = new Set(dalam.map((f) => f.id));
  const operasional = dalam.filter((f) => f.status === "OPERASIONAL");
  const idOperasional = new Set(operasional.map((f) => f.id));
  const lot = stok.filter(
    (s) =>
      idDalam.has(s.fasilitasId) &&
      (!saringan.komoditasKode || s.komoditasKode === saringan.komoditasKode),
  );
  const berlaku = lot.filter((s) => idOperasional.has(s.fasilitasId));
  const jumlah = (l: typeof lot) => l.reduce((t, s) => t + s.volumeTon, 0);
  const mendekati = (l: typeof lot) =>
    jumlah(l.filter((s) => s.sisaShelfLifeHari <= AMBANG_KEDALUWARSA_HARI));
  const berisiko = (l: typeof lot) =>
    jumlah(l.filter((s) => s.risikoFoodLoss > AMBANG_RISIKO));

  const kapasitas = operasional.reduce((t, f) => t + f.kapasitasTon, 0);
  const perKomoditas = new Map<string, typeof lot>();
  for (const s of berlaku)
    perKomoditas.set(s.komoditasKode, [
      ...(perKomoditas.get(s.komoditasKode) ?? []),
      s,
    ]);
  const komoditas: StokKomoditasResponse[] = [...perKomoditas]
    .map(([kode, l]) => ({
      komoditas_kode: kode,
      komoditas_nama: l[0].komoditasNama,
      total_ton: bulat1(jumlah(l)),
      jumlah_lot: l.length,
      jumlah_fasilitas: new Set(l.map((s) => s.fasilitasId)).size,
      mendekati_kedaluwarsa_ton: bulat1(mendekati(l)),
      risiko_tinggi_ton: bulat1(berisiko(l)),
    }))
    .sort((a, b) => b.total_ton - a.total_ton);

  return {
    tanggal: tanggalAceh(),
    ambang_kedaluwarsa_hari: AMBANG_KEDALUWARSA_HARI,
    ambang_risiko: AMBANG_RISIKO,
    ringkasan: {
      total_ton: bulat1(jumlah(berlaku)),
      jumlah_lot: berlaku.length,
      jumlah_fasilitas: new Set(berlaku.map((s) => s.fasilitasId)).size,
      kapasitas_ton: kapasitas,
      fasilitas_operasional: operasional.length,
      utilisasi_pct:
        kapasitas > 0 ? bulat1((100 * jumlah(berlaku)) / kapasitas) : 0,
      mendekati_kedaluwarsa_ton: bulat1(mendekati(berlaku)),
      risiko_tinggi_ton: bulat1(berisiko(berlaku)),
      kedaluwarsa_lot: 0,
      kedaluwarsa_ton: 0,
      di_luar_operasional_ton: bulat1(
        jumlah(lot.filter((s) => !idOperasional.has(s.fasilitasId))),
      ),
    },
    komoditas,
  };
}

// ── Produsen ──────────────────────────────────────────────────────────

/** Komoditas di luar pilot yang punya harga produsen pada data mock; sisanya hanya punya produksi. */
const PUNYA_HARGA_PRODUSEN = new Set(["KTG", "TMT", "CBR-RWT"]);

/**
 * Harga produsen dan konsumen sepekan terakhir dari seri harga mock. Margin dan produksi dibuat di sini seperti
 * server (web tidak menghitungnya): margin hanya bila kedua sisi harga ada.
 */
export function mockProdusen(
  saringan: SaringanOperasional = {},
): ProdusenResponse {
  const wilayah = saringan.kabupatenKode ?? "1171";
  const bulan = tanggalAceh().slice(0, 7);
  const komoditas = KOMODITAS.filter(
    (k) => !saringan.komoditasKode || k.kode === saringan.komoditasKode,
  )
    .map((k): ProdusenKomoditasResponse => {
      let produsen: ProdusenKomoditasResponse["produsen"] = null;
      let konsumen: ProdusenKomoditasResponse["konsumen"] = null;
      let margin: ProdusenKomoditasResponse["margin"] = null;
      if (k.pilot || PUNYA_HARGA_PRODUSEN.has(k.kode)) {
        const pekan = seriHarga(k, wilayah).slice(-JENDELA_HARI);
        const rata = (ambil: (p: (typeof pekan)[number]) => number) =>
          Math.round(pekan.reduce((t, p) => t + ambil(p), 0) / pekan.length);
        produsen = {
          harga: rata((p) => p.produsen),
          jumlah_entri: 3 * JENDELA_HARI,
          jumlah_kabupaten: 1,
        };
        konsumen = {
          harga: rata((p) => p.konsumen),
          jumlah_entri: 3 * JENDELA_HARI,
          jumlah_kabupaten: 1,
        };
        const selisih = konsumen.harga - produsen.harga;
        margin = {
          selisih,
          persen_produsen: bulat1((selisih / produsen.harga) * 100),
          persen_konsumen: bulat1((selisih / konsumen.harga) * 100),
        };
      }
      const resmi = k.kategori === "Pangan pokok";
      return {
        komoditas_kode: k.kode,
        komoditas_nama: k.nama,
        satuan: k.satuan,
        produsen,
        konsumen,
        margin,
        produksi: {
          ton: 400 * (1 + (hashSeed(k.kode) % 20)),
          periode: `${bulan}-01`,
          sumber: resmi ? "BPS" : "DEMO",
          demo: !resmi,
        },
      };
    })
    .sort(
      (a, b) =>
        Number(komoditasByKode[b.komoditas_kode].pilot) -
          Number(komoditasByKode[a.komoditas_kode].pilot) ||
        a.komoditas_nama.localeCompare(b.komoditas_nama),
    );
  return {
    dari: tanggalAceh(JENDELA_HARI - 1),
    sampai: tanggalAceh(),
    komoditas,
  };
}

// ── Logistik ──────────────────────────────────────────────────────────

const fasilitasLogistik = (id: string): FasilitasLogistikResponse => {
  const f = COLD_STORAGE.find((c) => c.id === id)!;
  return {
    id: f.id,
    kode: f.id,
    nama: f.nama,
    kabupaten_kode: f.kabupatenKode,
    kabupaten_nama: namaResmi(f.kabupatenKode),
  };
};

const nolJenis = (): GerakJenisResponse => ({ jumlah: 0, volume_ton: 0 });

/**
 * Pergerakan stok sepekan terakhir dan rute aktif dari lot dan rute mock: sebagian besar MASUK, satu PINDAH, dan satu
 * KELUAR. Filter kabupaten menyaring pergerakan yang salah satu ujungnya di sana; komoditas menyaring pergerakan dan rute.
 */
export function mockLogistik(
  saringan: SaringanOperasional = {},
): LogistikResponse {
  const { stok } = mockColdStorage();
  const sekarang = Date.now();
  const lot = stok.filter((s) => s.fasilitasId !== "CS-05").slice(0, 8);
  const semua: PergerakanResponse[] = lot.map((s, i) => {
    const jenis = i === 5 ? "PINDAH" : i === 7 ? "KELUAR" : "MASUK";
    const lainnya = COLD_STORAGE.find(
      (c) => c.id !== s.fasilitasId && c.status === "OPERASIONAL",
    )!.id;
    return {
      id: String(i + 1),
      waktu: new Date(sekarang - i * 0.7 * HARI_MS).toISOString(),
      jenis,
      komoditas_kode: s.komoditasKode,
      komoditas_nama: s.komoditasNama,
      volume_ton: s.volumeTon,
      asal: jenis === "MASUK" ? null : fasilitasLogistik(s.fasilitasId),
      tujuan:
        jenis === "KELUAR"
          ? null
          : fasilitasLogistik(jenis === "PINDAH" ? lainnya : s.fasilitasId),
    };
  });
  const diKabupaten = (p: PergerakanResponse) =>
    !saringan.kabupatenKode ||
    p.asal?.kabupaten_kode === saringan.kabupatenKode ||
    p.tujuan?.kabupaten_kode === saringan.kabupatenKode;
  const gerakan = semua.filter(
    (p) =>
      diKabupaten(p) &&
      (!saringan.komoditasKode || p.komoditas_kode === saringan.komoditasKode),
  );

  const tambah = (g: GerakJenisResponse, p: PergerakanResponse) => ({
    jumlah: g.jumlah + 1,
    volume_ton: g.volume_ton + p.volume_ton,
  });
  const ringkasan = {
    masuk: nolJenis(),
    keluar: nolJenis(),
    pindah: nolJenis(),
  };
  const perKomoditas = new Map<string, LogistikResponse["komoditas"][number]>();
  for (const p of gerakan) {
    const kunci =
      p.jenis === "MASUK"
        ? "masuk"
        : p.jenis === "KELUAR"
          ? "keluar"
          : "pindah";
    ringkasan[kunci] = tambah(ringkasan[kunci], p);
    const k = perKomoditas.get(p.komoditas_kode) ?? {
      komoditas_kode: p.komoditas_kode,
      komoditas_nama: p.komoditas_nama,
      masuk: nolJenis(),
      keluar: nolJenis(),
      pindah: nolJenis(),
    };
    k[kunci] = tambah(k[kunci], p);
    perKomoditas.set(p.komoditas_kode, k);
  }
  const bulatJenis = (g: GerakJenisResponse): GerakJenisResponse => ({
    jumlah: g.jumlah,
    volume_ton: bulat1(g.volume_ton),
  });
  const totalK = (k: LogistikResponse["komoditas"][number]) =>
    k.masuk.volume_ton + k.keluar.volume_ton + k.pindah.volume_ton;

  const rute: RuteAktifResponse[] = mockRute()
    .map((r): RuteAktifResponse => {
      const asal =
        KABUPATEN.find((k) => namaWilayah(k.kode) === r.asal)?.kode ?? "";
      const tujuan =
        KABUPATEN.find((k) => namaWilayah(k.kode) === r.tujuan)?.kode ?? "";
      const kom = KOMODITAS.find((k) => k.nama === r.komoditas)?.kode ?? "";
      return {
        id: r.id,
        kode: `RT-${asal}-${tujuan}`,
        asal_kode: asal,
        asal: namaResmi(asal),
        tujuan_kode: tujuan,
        tujuan: namaResmi(tujuan),
        komoditas_kode: kom,
        komoditas_nama: r.komoditas,
        volume_ton: r.volumeTon,
        jarak_km: r.jarakKm,
        jam_tempuh: r.jamTempuh,
        biaya: r.biaya,
        status: "USULAN",
        sumber: "DEMO",
        dibuat_pada: new Date(sekarang).toISOString(),
      };
    })
    .filter(
      (r) =>
        (!saringan.kabupatenKode ||
          r.asal_kode === saringan.kabupatenKode ||
          r.tujuan_kode === saringan.kabupatenKode) &&
        (!saringan.komoditasKode ||
          r.komoditas_kode === saringan.komoditasKode),
    )
    .sort((a, b) => b.biaya - a.biaya);

  return {
    dari: tanggalAceh(JENDELA_HARI - 1),
    sampai: tanggalAceh(),
    ringkasan: {
      masuk: bulatJenis(ringkasan.masuk),
      keluar: bulatJenis(ringkasan.keluar),
      pindah: bulatJenis(ringkasan.pindah),
    },
    komoditas: [...perKomoditas.values()]
      .sort((a, b) => totalK(b) - totalK(a))
      .map((k) => ({
        ...k,
        masuk: bulatJenis(k.masuk),
        keluar: bulatJenis(k.keluar),
        pindah: bulatJenis(k.pindah),
      })),
    terbaru: gerakan.slice(0, 10),
    rute: {
      jumlah: rute.length,
      volume_ton: bulat1(rute.reduce((t, r) => t + r.volume_ton, 0)),
      biaya: rute.reduce((t, r) => t + r.biaya, 0),
      items: rute.slice(0, 10),
    },
  };
}
