/**
 * Mode mock: data sintetis deterministik agar UI dapat dijalankan & ditinjau
 * tanpa backend Go. Aktif otomatis bila NEXT_PUBLIC_API_BASE_URL kosong.
 */
import type { Severity } from "@/lib/design/tokens";
import { ROLES } from "@/lib/rbac/roles";
import {
  ANCHOR_DATE, KABUPATEN, KOMODITAS, PASAR, delta7, gauss, hashSeed, kabupatenBerpasar, mulberry32,
  namaWilayah, pick, proyeksi, seriCuaca, seriHarga,
} from "@/lib/data/generators";
import { COLD_STORAGE, USERS, komoditasByKode, pasarById } from "@/lib/data/reference";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";
import type {
  AuditRow, EntriHarga, EwsAlert, FasilitasColdStorage, KartuIntervensi, KpiOperasional, MasterKomoditasRow,
  PenggunaRow, PolicyBrief, ProyeksiXai, RingkasanEksekutif, SelHeatmap, StatusEntri, StokColdStorage, SumberData,
  TugasLapangan,
} from "@/lib/data/types";

const menitLalu = (m: number) => new Date(Date.now() - m * 60_000).toISOString();
const inisial = (nama: string) => nama.split(/\s+/).filter((w) => /^[A-Za-z]/.test(w)).slice(0, 2).map((w) => w[0]).join("").toUpperCase();

/* ── Heatmap & KPI ─────────────────────────────────────────── */
export function mockHeatmap() {
  const kab = kabupatenBerpasar();
  const sel: SelHeatmap[] = [];
  for (const kom of KOMODITAS) {
    for (const k of kab) {
      const seri = seriHarga(kom, k.kode);
      sel.push({ komoditasKode: kom.kode, kabupatenKode: k.kode, delta: delta7(seri), harga: seri[seri.length - 1].konsumen });
    }
  }
  return {
    kabupaten: kab.map((k) => ({ kode: k.kode, nama: k.nama })),
    komoditas: KOMODITAS.map((k) => ({ kode: k.kode, nama: k.nama, pilot: k.pilot })),
    sel,
  };
}

export function mockKpi(): KpiOperasional[] {
  const rng = mulberry32(hashSeed("kpi"));
  const tren = (base: number, amp: number) => Array.from({ length: 14 }, (_, i) => Math.round(base + Math.sin(i / 2) * amp + gauss(rng) * amp * 0.5 + i * amp * 0.15));
  const heat = mockHeatmap();
  const waspada = new Set(heat.sel.filter((s) => s.delta > 7).map((s) => s.komoditasKode)).size;
  const cs = mockColdStorage().fasilitas.filter((f) => f.status === "OPERASIONAL");
  const util = (cs.reduce((s, f) => s + f.terisiTon, 0) / cs.reduce((s, f) => s + f.kapasitasTon, 0)) * 100;
  return [
    { id: "entri", label: "Entri harga hari ini", nilai: 412, format: "angka", delta: 8.2, keteranganDelta: "vs rata-rata 7 hari", tren: tren(360, 30), tone: "data", href: "/data/harga" },
    { id: "validasi", label: "Tingkat validasi", nilai: 87.4, format: "persen", delta: 2.1, keteranganDelta: "poin dari pekan lalu", tren: tren(82, 3), tone: "normal", href: "/data/harga?status=DIAJUKAN" },
    { id: "waspada", label: "Komoditas di atas ambang", nilai: waspada, format: "angka", delta: 2, keteranganDelta: "komoditas sejak kemarin", tren: tren(2, 1), tone: "defisit", href: "/ews" },
    { id: "cs", label: "Utilisasi cold storage", nilai: Math.round(util * 10) / 10, format: "persen", delta: -3.4, keteranganDelta: "poin pekan ini", tren: tren(74, 4), tone: "surplus", href: "/cold-storage" },
  ];
}

export function mockTrenHarga(kode = "CBM-KRT", kab = "1171", hari = 45) {
  const kom = komoditasByKode[kode];
  return seriHarga(kom, kab).slice(-hari).map((p) => ({ tanggal: p.tanggal.toISOString(), produsen: p.produsen, konsumen: p.konsumen }));
}

export function mockMapMarkers() {
  const heat = mockHeatmap();
  return KABUPATEN.map((k) => {
    const sel = heat.sel.filter((s) => s.kabupatenKode === k.kode).sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta));
    const top = sel[0];
    const d = top?.delta ?? gauss(mulberry32(hashSeed(k.kode))) * 3;
    const status = d > 10 ? "defisit" : d > 5 ? "waspada" : d < -6 ? "surplus" : "normal";
    return {
      kode: k.kode, nama: namaWilayah(k.kode), lat: k.lat, lng: k.lng, delta: d,
      komoditas: top ? komoditasByKode[top.komoditasKode].nama : "—",
      status: status as "defisit" | "waspada" | "surplus" | "normal",
      punyaPasar: Boolean(top),
    };
  });
}

/* ── Entri harga (antrean validasi) ───────────────────────── */
export function mockEntri(jumlah = 168): EntriHarga[] {
  const rng = mulberry32(hashSeed("entri"));
  const mhs = USERS.filter((u) => u.role === "OPS_MAHASISWA");
  const statusPool: StatusEntri[] = ["DIAJUKAN", "DIAJUKAN", "DIAJUKAN", "DIAJUKAN", "DIAJUKAN", "DIVALIDASI", "DIVALIDASI", "DIVALIDASI", "DIPUBLIKASI", "DIPUBLIKASI", "DITOLAK", "DRAF"];
  return Array.from({ length: jumlah }, (_, i) => {
    const u = mhs[i % mhs.length];
    const pasar = PASAR.find((p) => p.kabupatenKode === u.kabupatenKode) ?? PASAR[0];
    const kom = pick(rng, KOMODITAS);
    const tipe = rng() > 0.45 ? "KONSUMEN" : "PRODUSEN";
    const seri = seriHarga(kom, pasar.kabupatenKode);
    const acuan = tipe === "KONSUMEN" ? seri[seri.length - 1].konsumen : seri[seri.length - 1].produsen;
    const outlier = rng() < 0.06;
    const dev = outlier ? (rng() > 0.5 ? 1 : -1) * (28 + rng() * 30) : gauss(rng) * 4;
    const harga = Math.round((acuan * (1 + dev / 100)) / 100) * 100;
    return {
      id: `HRG-${String(26100300 + i)}`,
      komoditasKode: kom.kode,
      komoditasNama: kom.nama,
      pasarId: pasar.id,
      pasarNama: pasar.nama,
      kabupatenKode: pasar.kabupatenKode,
      wilayah: namaWilayah(pasar.kabupatenKode),
      tipe,
      harga,
      satuan: kom.satuan,
      kualitas: pick(rng, ["PREMIUM", "MEDIUM", "MEDIUM", "ASALAN"] as const),
      petugasId: u.id,
      petugasNama: u.nama,
      dicatatPada: menitLalu(Math.floor(i * 7 + rng() * 6)),
      status: outlier ? "DIAJUKAN" : pick(rng, statusPool),
      deviasiPct: Math.round(dev * 10) / 10,
      skorAnomali: Math.min(1, Math.abs(dev) / 40 + rng() * 0.05),
      adaFoto: rng() > 0.25,
      dalamGeofence: rng() > 0.07,
    };
  });
}

/* ── EWS ──────────────────────────────────────────────────── */
function sevDariDelta(d: number): Severity {
  if (d >= 18) return "KRITIS";
  if (d >= 12) return "SIAGA";
  if (d >= 8) return "WASPADA";
  return "INFO";
}

export function mockEws(): EwsAlert[] {
  const heat = mockHeatmap();
  const alerts: EwsAlert[] = [];
  let i = 0;
  for (const s of [...heat.sel].sort((a, b) => b.delta - a.delta)) {
    const kom = komoditasByKode[s.komoditasKode];
    if (s.delta >= 8) {
      alerts.push({
        id: `EWS-${1000 + i}`, severity: sevDariDelta(s.delta), jenis: "LONJAKAN_HARGA",
        judul: `Lonjakan harga ${kom.nama.toLowerCase()}`, komoditasKode: kom.kode, komoditasNama: kom.nama,
        kabupatenKode: s.kabupatenKode, wilayah: namaWilayah(s.kabupatenKode), nilai: Math.round(s.delta * 10) / 10, satuanNilai: "%",
        pesan: `Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp${s.harga.toLocaleString("id-ID")}/${kom.satuan}.`,
        waktu: menitLalu(4 + i * 23), status: i < 4 ? "BARU" : i < 9 ? "DIAKUI" : "DIESKALASI",
        skorDampak: Math.min(98, Math.round(40 + s.delta * 2.4)), sumber: "Model EWS v2.3 + entri lapangan",
      });
      i++;
    } else if (s.delta <= -7) {
      alerts.push({
        id: `EWS-${1000 + i}`, severity: "INFO", jenis: "OVER_SUPPLY",
        judul: `Indikasi surplus ${kom.nama.toLowerCase()}`, komoditasKode: kom.kode, komoditasNama: kom.nama,
        kabupatenKode: s.kabupatenKode, wilayah: namaWilayah(s.kabupatenKode), nilai: Math.round(s.delta * 10) / 10, satuanNilai: "%",
        pesan: "Harga turun di bawah koridor. Pertimbangkan penyerapan ke cold storage.",
        waktu: menitLalu(30 + i * 31), status: "BARU", skorDampak: Math.round(30 + Math.abs(s.delta) * 2), sumber: "Model EWS v2.3",
      });
      i++;
    }
  }
  const cuaca = seriCuaca("1106");
  const last = cuaca[cuaca.length - 1];
  alerts.push(
    {
      id: "EWS-0901", severity: "SIAGA", jenis: "ANOMALI_CUACA", judul: "Curah hujan ekstrem di sentra Gayo",
      kabupatenKode: "1106", wilayah: namaWilayah("1106"), nilai: last.curahHujanMm, satuanNilai: "mm",
      pesan: "Curah hujan 2,1× normal selama 10 hari. Risiko gagal panen cabai & tomat meningkat.",
      waktu: menitLalu(95), status: "DIAKUI", skorDampak: 81, sumber: "BMKG + OpenWeather",
    },
    {
      id: "EWS-0902", severity: "WASPADA", jenis: "KELANGKAAN_STOK", judul: "Stok cabai di Cold Storage Lambaro menipis",
      komoditasKode: "CBM-KRT", komoditasNama: "Cabai merah keriting", kabupatenKode: "1108", wilayah: namaWilayah("1108"),
      nilai: -32, satuanNilai: "%", pesan: "Stok turun 32% dalam 5 hari; cukup untuk ±4 hari permintaan Banda Aceh.",
      waktu: menitLalu(140), status: "BARU", skorDampak: 74, sumber: "Telemetri cold storage",
    },
    {
      id: "EWS-0903", severity: "INFO", jenis: "ANOMALI_DATA", judul: "Entri harga di luar geofence",
      kabupatenKode: "1104", wilayah: namaWilayah("1104"), nilai: 3, satuanNilai: "%",
      pesan: "3 entri tercatat lebih dari 1 km dari Pasar Kutacane. Perlu verifikasi koordinator.",
      waktu: menitLalu(210), status: "SELESAI", skorDampak: 22, sumber: "Validasi otomatis",
    },
  );
  return alerts.sort((a, b) => +new Date(b.waktu) - +new Date(a.waktu));
}

let seqLive = 0;
/** Alert acak untuk simulasi feed real-time di mode mock. */
export function randomEws(): EwsAlert {
  const rng = mulberry32(Date.now() & 0xffffffff);
  const kom = pick(rng, KOMODITAS.filter((k) => k.volatilitas > 0.015));
  const kab = pick(rng, kabupatenBerpasar());
  const naik = rng() > 0.2;
  const nilai = naik ? Math.round((8 + rng() * 16) * 10) / 10 : -Math.round((7 + rng() * 8) * 10) / 10;
  const severity = naik ? sevDariDelta(nilai) : "INFO";
  seqLive++;
  return {
    id: `EWS-L${Date.now()}-${seqLive}`, severity, jenis: naik ? "LONJAKAN_HARGA" : "OVER_SUPPLY",
    judul: naik ? `Lonjakan harga ${kom.nama.toLowerCase()}` : `Indikasi surplus ${kom.nama.toLowerCase()}`,
    komoditasKode: kom.kode, komoditasNama: kom.nama, kabupatenKode: kab.kode, wilayah: namaWilayah(kab.kode),
    nilai, satuanNilai: "%", pesan: naik ? "Kenaikan melampaui ambang dinamis wilayah." : "Harga produsen turun di bawah koridor.",
    waktu: new Date().toISOString(), status: "BARU", skorDampak: Math.round(35 + Math.abs(nilai) * 2.5), sumber: "Model EWS v2.3",
  };
}

/* ── Kanban intervensi ───────────────────────────────────── */
export function mockKanban(): KartuIntervensi[] {
  const rng = mulberry32(hashSeed("kanban"));
  const tim = USERS.filter((u) => u.role.startsWith("MANAJER") || u.role === "OPS_KOORDINATOR" || u.role === "OPS_AGREGATOR");
  const def: { siklus: KartuIntervensi["siklus"]; judul: string; kom: string; kab: string; p: KartuIntervensi["prioritas"]; ai?: boolean }[] = [
    { siklus: "tekanan", judul: "Lonjakan cabai merah Banda Aceh", kom: "CBM-KRT", kab: "1171", p: "TINGGI", ai: true },
    { siklus: "tekanan", judul: "Pasokan cabai rawit Lhokseumawe", kom: "CBR-RWT", kab: "1174", p: "TINGGI", ai: true },
    { siklus: "tekanan", judul: "Bawang merah Meulaboh naik 9%", kom: "BWM", kab: "1107", p: "SEDANG", ai: true },
    { siklus: "tekanan", judul: "Kirim cabai Takengon → Banda Aceh", kom: "CBM-KRT", kab: "1106", p: "TINGGI" },
    { siklus: "tekanan", judul: "Subsidi ongkos angkut Simeulue", kom: "CBM-KRT", kab: "1101", p: "SEDANG" },
    { siklus: "tekanan", judul: "Verifikasi harga ikan tongkol Sabang", kom: "IKN-TGK", kab: "1172", p: "RENDAH" },
    { siklus: "tekanan", judul: "Operasi pasar cabai Peunayong", kom: "CBM-KRT", kab: "1171", p: "TINGGI" },
    { siklus: "normal", judul: "Rekap harga beras pekan 40", kom: "BRS-MED", kab: "1171", p: "RENDAH" },
    { siklus: "normal", judul: "Cocokkan IHK BPS September", kom: "BRS-MED", kab: "1171", p: "SEDANG" },
    { siklus: "normal", judul: "Jadwal armada Bireuen–Lhokseumawe", kom: "TLR", kab: "1110", p: "RENDAH" },
    { siklus: "normal", judul: "Agregasi telur peternak Bireuen", kom: "TLR", kab: "1110", p: "SEDANG" },
    { siklus: "normal", judul: "Pantau daging ayam Langsa", kom: "DAG-AYM", kab: "1173", p: "RENDAH" },
    { siklus: "normal", judul: "Verifikasi gula pasir Pidie", kom: "GLA", kab: "1109", p: "RENDAH" },
    { siklus: "surplus", judul: "Kentang Gayo: harga produsen −18%", kom: "KTG", kab: "1106", p: "TINGGI", ai: true },
    { siklus: "surplus", judul: "Tomat Bener Meriah surplus 1.200 ton", kom: "TMT", kab: "1117", p: "TINGGI", ai: true },
    { siklus: "surplus", judul: "Alokasi 80 ton ke CS Takengon", kom: "KTG", kab: "1106", p: "SEDANG" },
    { siklus: "surplus", judul: "Uji olahan saus tomat UMKM", kom: "TMT", kab: "1117", p: "SEDANG" },
    { siklus: "surplus", judul: "Pembeli kentang dari Medan", kom: "KTG", kab: "1106", p: "SEDANG" },
    { siklus: "surplus", judul: "Keripik kentang — mitra koperasi", kom: "KTG", kab: "1106", p: "RENDAH" },
  ];
  return def.map((d, i) => {
    const tahap = SIKLUS_TAHAP[d.siklus];
    const t = tahap[Math.min(tahap.length - 1, Math.floor(rng() * (tahap.length - 0.6)))];
    const total = 3 + Math.floor(rng() * 5);
    const pj = [pick(rng, tim), ...(rng() > 0.5 ? [pick(rng, tim)] : [])];
    return {
      id: `INT-${String(301 + i)}`, siklus: d.siklus, tahap: t.id, judul: d.judul, komoditasKode: d.kom, kabupatenKode: d.kab,
      prioritas: d.p, rekomendasiAi: Boolean(d.ai),
      penanggungJawab: [...new Map(pj.map((u) => [u.id, { nama: u.nama, inisial: inisial(u.nama) }])).values()],
      tenggat: new Date(ANCHOR_DATE.getTime() + (Math.floor(rng() * 9) - 2) * 86_400_000).toISOString(),
      progres: { selesai: Math.floor(rng() * (total + 1)), total },
    };
  });
}

/* ── Cold storage ────────────────────────────────────────── */
export function mockColdStorage(): { fasilitas: FasilitasColdStorage[]; stok: StokColdStorage[] } {
  const rng = mulberry32(hashSeed("cs"));
  const fasilitas = COLD_STORAGE.map((c) => {
    const util = c.status === "RENCANA" ? 0 : c.status === "PERAWATAN" ? 0.22 : 0.58 + rng() * 0.3;
    return {
      id: c.id, nama: c.nama, wilayah: namaWilayah(c.kabupatenKode), kabupatenKode: c.kabupatenKode,
      kapasitasTon: c.kapasitasTon, terisiTon: Math.round(c.kapasitasTon * util), suhuTargetC: c.suhuTargetC,
      suhuAktualC: Math.round((c.suhuTargetC + gauss(rng) * 0.8) * 10) / 10, status: c.status, lat: c.lat, lng: c.lng,
    };
  });
  const agregator = ["Koperasi Tani Gayo Makmur", "CV Lambaro Segar", "KUD Pidie Jaya", "Kelompok Tani Bener Meriah"];
  const stok: StokColdStorage[] = [];
  COLD_STORAGE.filter((c) => c.status !== "RENCANA").forEach((c) => {
    c.fokus.forEach((kode, j) => {
      for (let b = 0; b < 2 + Math.floor(rng() * 2); b++) {
        const kom = komoditasByKode[kode];
        const umur = Math.floor(rng() * kom.shelfLifeHari * 0.9);
        const sisa = kom.shelfLifeHari - umur;
        stok.push({
          id: `STK-${c.id}-${j}${b}`, fasilitasId: c.id, fasilitasNama: c.nama, komoditasKode: kode, komoditasNama: kom.nama,
          volumeTon: Math.round((4 + rng() * 38) * 10) / 10,
          masuk: new Date(ANCHOR_DATE.getTime() - umur * 86_400_000).toISOString(),
          sisaShelfLifeHari: sisa, risikoFoodLoss: Math.max(0.02, Math.min(0.95, 1 - sisa / kom.shelfLifeHari + gauss(rng) * 0.05)),
          pemilik: pick(rng, agregator),
        });
      }
    });
  });
  return { fasilitas, stok: stok.sort((a, b) => a.sisaShelfLifeHari - b.sisaShelfLifeHari) };
}

/* ── Sumber data, pengguna, audit, brief, tugas, master ─── */
export function mockSumber(): SumberData[] {
  return [
    { id: "SRC-BPS", nama: "BPS WebAPI — IHK & inflasi", penyedia: "BPS", jenis: "API", cakupan: "Provinsi & 3 kota IHK", jadwal: "Harian 06.00", terakhir: menitLalu(142), status: "SEHAT", keberhasilan7h: 100, latensiMs: 640, rekamTerakhir: 1284 },
    { id: "SRC-BI", nama: "PIHPS Nasional — harga pangan", penyedia: "Bank Indonesia", jenis: "API", cakupan: "Pasar tradisional & modern", jadwal: "Harian 10.00", terakhir: menitLalu(38), status: "SEHAT", keberhasilan7h: 98.6, latensiMs: 910, rekamTerakhir: 3360 },
    { id: "SRC-DJPB", nama: "Kanwil DJPb — realisasi belanja", penyedia: "Kanwil DJPb Aceh", jenis: "API", cakupan: "APBD & BTT 23 kab/kota", jadwal: "Mingguan Senin", terakhir: menitLalu(2880), status: "SEHAT", keberhasilan7h: 100, latensiMs: 1220, rekamTerakhir: 276 },
    { id: "SRC-BMKG", nama: "BMKG — prakiraan & curah hujan", penyedia: "BMKG", jenis: "API", cakupan: "23 kab/kota", jadwal: "Tiap 3 jam", terakhir: menitLalu(51), status: "SEHAT", keberhasilan7h: 99.1, latensiMs: 480, rekamTerakhir: 552 },
    { id: "SRC-OWM", nama: "OpenWeather — cuaca aktual", penyedia: "OpenWeather", jenis: "API", cakupan: "Titik sentra produksi", jadwal: "Tiap jam", terakhir: menitLalu(12), status: "SEHAT", keberhasilan7h: 99.8, latensiMs: 210, rekamTerakhir: 96 },
    { id: "SRC-BAPANAS", nama: "Panel Harga Bapanas", penyedia: "Badan Pangan Nasional", jenis: "SCRAPER", cakupan: "Harga produsen & konsumen", jadwal: "Harian 11.00", terakhir: menitLalu(325), status: "TERTUNDA", keberhasilan7h: 86.4, latensiMs: 3400, rekamTerakhir: 0 },
    { id: "SRC-PORTAL", nama: "Portal harga Disperindag Aceh", penyedia: "Pemerintah Aceh", jenis: "SCRAPER", cakupan: "Pasar induk provinsi", jadwal: "Harian 12.00", terakhir: menitLalu(1460), status: "GAGAL", keberhasilan7h: 57.1, latensiMs: 0, rekamTerakhir: 0 },
    { id: "SRC-CS", nama: "Telemetri cold storage", penyedia: "Sensor IoT fasilitas", jenis: "API", cakupan: "3 fasilitas operasional", jadwal: "Tiap 5 menit", terakhir: menitLalu(3), status: "SEHAT", keberhasilan7h: 99.97, latensiMs: 95, rekamTerakhir: 12 },
  ];
}

export function mockPengguna(): PenggunaRow[] {
  const rng = mulberry32(hashSeed("pengguna"));
  return USERS.map((u, i) => ({
    id: u.id, nama: u.nama, email: u.email, role: u.role, lembaga: u.lembaga,
    wilayah: u.kabupatenKode ? namaWilayah(u.kabupatenKode) : "Seluruh Aceh",
    status: i === 7 ? "MENUNGGU" : i === 22 ? "NONAKTIF" : "AKTIF",
    mfa: ROLES[u.role].mfaWajib || rng() > 0.4,
    terakhirMasuk: menitLalu(Math.floor(rng() * 4000)),
  }));
}

export function mockAudit(): AuditRow[] {
  const rng = mulberry32(hashSeed("audit"));
  const aksi = [
    ["VALIDASI_HARGA", "EntriHarga"], ["PUBLIKASI_HARGA", "EntriHarga"], ["AKUI_EWS", "EwsAlert"], ["PINDAH_TAHAP", "Intervensi"],
    ["SETUJUI_INTERVENSI", "Intervensi"], ["UBAH_AMBANG_EWS", "AturanEws"], ["MASUK", "Sesi"], ["EKSPOR_DATA", "EntriHarga"],
    ["AJUKAN_MASTER", "Komoditas"], ["NONAKTIFKAN_PENGGUNA", "Pengguna"],
  ] as const;
  return Array.from({ length: 120 }, (_, i) => {
    const u = pick(rng, USERS);
    const [a, e] = pick(rng, aksi);
    return {
      id: `AUD-${90000 + i}`, waktu: menitLalu(i * 13 + Math.floor(rng() * 10)), aktor: u.nama, peran: ROLES[u.role].label,
      aksi: a, entitas: e, entitasId: `${e.slice(0, 3).toUpperCase()}-${Math.floor(rng() * 9000 + 1000)}`,
      hasil: rng() > 0.06 ? "BERHASIL" : "DITOLAK", ip: `10.20.${Math.floor(rng() * 40)}.${Math.floor(rng() * 250)}`,
    };
  });
}

export function mockBriefs(): PolicyBrief[] {
  return [
    { id: "PB-2026-10", periode: "Oktober 2026", judul: "Meredam tekanan harga cabai pascacurah hujan ekstrem di dataran tinggi Gayo", status: "DRAF", disusunOleh: "Siti Humaira", diperbarui: menitLalu(45), jumlahRekomendasi: 5, halaman: 8 },
    { id: "PB-2026-09", periode: "September 2026", judul: "Optimalisasi cold storage untuk serapan surplus kentang dan tomat", status: "DISETUJUI", disusunOleh: "Siti Humaira", diperbarui: menitLalu(9800), jumlahRekomendasi: 4, halaman: 7 },
    { id: "PB-2026-08", periode: "Agustus 2026", judul: "Stabilisasi harga beras menjelang panen gadu", status: "DIDISTRIBUSIKAN", disusunOleh: "Siti Humaira", diperbarui: menitLalu(52000), jumlahRekomendasi: 3, halaman: 6 },
    { id: "PB-2026-07", periode: "Juli 2026", judul: "Pola distribusi ikan tongkol di wilayah pesisir timur", status: "DIDISTRIBUSIKAN", disusunOleh: "Fajar Ramadhan", diperbarui: menitLalu(95000), jumlahRekomendasi: 4, halaman: 9 },
    { id: "PB-2026-06", periode: "Juni 2026", judul: "Kesiapan pasokan pangan Iduladha di 23 kabupaten/kota", status: "DIDISTRIBUSIKAN", disusunOleh: "Siti Humaira", diperbarui: menitLalu(138000), jumlahRekomendasi: 6, halaman: 10 },
  ];
}

export function mockTugas(): TugasLapangan[] {
  const now = Date.now();
  const jam = (h: number) => new Date(now + h * 3_600_000).toISOString();
  return [
    { id: "TGS-881", judul: "Survei harga konsumen pagi", pasarId: "PSR-1171-01", pasarNama: "Pasar Peunayong", komoditas: ["CBM-KRT", "CBR-RWT", "BWM", "BRS-MED", "TLR", "DAG-AYM"], tenggat: jam(2), status: "BERJALAN", target: 6, terisi: 4, tipe: "KONSUMEN" },
    { id: "TGS-882", judul: "Verifikasi ulang harga cabai", pasarId: "PSR-1171-02", pasarNama: "Pasar Al-Mahirah Lamdingin", komoditas: ["CBM-KRT", "CBR-RWT"], tenggat: jam(5), status: "BELUM", target: 2, terisi: 0, tipe: "KONSUMEN" },
    { id: "TGS-883", judul: "Harga produsen kelompok tani", pasarId: "PSR-1108-01", pasarNama: "Pasar Induk Lambaro", komoditas: ["CBM-KRT", "BWM", "TMT", "KTG"], tenggat: jam(26), status: "BELUM", target: 4, terisi: 0, tipe: "PRODUSEN" },
    { id: "TGS-879", judul: "Survei harga konsumen sore", pasarId: "PSR-1171-01", pasarNama: "Pasar Peunayong", komoditas: ["IKN-TGK", "TLR", "MYK", "GLA"], tenggat: jam(-3), status: "TERLAMBAT", target: 4, terisi: 1, tipe: "KONSUMEN" },
    { id: "TGS-875", judul: "Survei harga konsumen pagi", pasarId: "PSR-1171-01", pasarNama: "Pasar Peunayong", komoditas: ["CBM-KRT", "BWM", "BRS-MED", "TLR"], tenggat: jam(-22), status: "SELESAI", target: 4, terisi: 4, tipe: "KONSUMEN" },
  ];
}

export function mockMasterKomoditas(): MasterKomoditasRow[] {
  return KOMODITAS.map((k, i) => ({
    id: k.kode, kode: k.kode, nama: k.nama, kategori: k.kategori, satuan: k.satuan, pilot: k.pilot,
    versi: 1 + (i % 3), status: i === 3 ? "MENUNGGU_PERSETUJUAN" : "AKTIF",
  }));
}

/* ── Eksekutif & XAI ─────────────────────────────────────── */
export function mockEksekutif(): RingkasanEksekutif {
  const bulan = ["Okt", "Nov", "Des", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep"];
  const yoy = [2.12, 1.94, 2.31, 2.58, 2.41, 2.2, 2.47, 2.69, 2.33, 2.51, 2.63, 2.84];
  const ews = mockEws();
  const hitung = (s: Severity) => ews.filter((e) => e.severity === s && e.status !== "SELESAI").length;
  const k = mockKanban();
  const intv = (s: KartuIntervensi["siklus"]) => ({
    aktif: k.filter((c) => c.siklus === s && !c.tahap.endsWith("selesai")).length,
    selesai: k.filter((c) => c.siklus === s && c.tahap.endsWith("selesai")).length,
  });
  return {
    inflasi: {
      yoy: 2.84, mtm: 0.41, ytd: 1.92, sasaran: { bawah: 1.5, atas: 3.5 }, rilis: "2026-10-01",
      seri: bulan.map((b, i) => ({ bulan: b, yoy: yoy[i] })),
      penyumbang: [
        { nama: "Cabai merah", andil: 0.18 }, { nama: "Bawang merah", andil: 0.06 }, { nama: "Ikan tongkol", andil: 0.04 },
        { nama: "Beras", andil: 0.03 }, { nama: "Tomat", andil: -0.02 }, { nama: "Kentang", andil: -0.01 },
      ],
    },
    neraca: [
      { komoditasKode: "BRS-MED", nama: "Beras", produksi: 146200, kebutuhan: 52400 },
      { komoditasKode: "KTG", nama: "Kentang", produksi: 9800, kebutuhan: 3200 },
      { komoditasKode: "TMT", nama: "Tomat", produksi: 4100, kebutuhan: 2900 },
      { komoditasKode: "IKN-TGK", nama: "Ikan tongkol", produksi: 7400, kebutuhan: 6100 },
      { komoditasKode: "DAG-AYM", nama: "Daging ayam", produksi: 6200, kebutuhan: 6000 },
      { komoditasKode: "TLR", nama: "Telur ayam", produksi: 4800, kebutuhan: 5600 },
      { komoditasKode: "CBM-KRT", nama: "Cabai merah", produksi: 3100, kebutuhan: 3900 },
      { komoditasKode: "BWM", nama: "Bawang merah", produksi: 900, kebutuhan: 2600 },
    ],
    fiskal: {
      pagu: 186e9, realisasi: 54.2e9, skor: 72, status: "MEMADAI", perTanggal: "2026-09-30",
      instrumen: [
        { nama: "Belanja tidak terduga (BTT)", pagu: 120e9, realisasi: 31.2e9 },
        { nama: "Operasi pasar murah", pagu: 24e9, realisasi: 11.2e9 },
        { nama: "Subsidi ongkos angkut", pagu: 18e9, realisasi: 9.4e9 },
        { nama: "Gerakan tanam pangan", pagu: 24e9, realisasi: 2.4e9 },
      ],
    },
    ews: { INFO: hitung("INFO"), WASPADA: hitung("WASPADA"), SIAGA: hitung("SIAGA"), KRITIS: hitung("KRITIS") },
    intervensi: { normal: intv("normal"), tekanan: intv("tekanan"), surplus: intv("surplus") },
    siklusDominan: "tekanan",
  };
}

const FAKTOR: Record<string, { nama: string; bobot: number; detail: string }[]> = {
  "CBM-KRT": [
    { nama: "Curah hujan sentra Gayo", bobot: 0.42, detail: "2,1× normal selama 10 hari di Aceh Tengah & Bener Meriah" },
    { nama: "Stok cold storage", bobot: 0.24, detail: "Stok Lambaro turun 32% dalam 5 hari" },
    { nama: "Momentum harga 7 hari", bobot: 0.21, detail: "Kenaikan beruntun di 7 dari 11 pasar" },
    { nama: "Permintaan musiman", bobot: 0.13, detail: "Kenduri Maulid mulai pekan depan" },
    { nama: "Pasokan dari Sumatera Utara", bobot: -0.08, detail: "Kiriman via Langsa naik 12%" },
  ],
  BWM: [
    { nama: "Pasokan antarpulau", bobot: 0.46, detail: "Kiriman dari Brebes tertunda cuaca" },
    { nama: "Momentum harga 7 hari", bobot: 0.28, detail: "Naik moderat di pesisir barat" },
    { nama: "Panen lokal Pidie", bobot: -0.18, detail: "Panen kecil mulai masuk pasar Sigli" },
    { nama: "Ongkos angkut", bobot: 0.12, detail: "Tarif logistik Medan–Banda Aceh naik 4%" },
  ],
  "BRS-MED": [
    { nama: "Panen gadu", bobot: -0.44, detail: "Panen Pidie & Aceh Utara 60% selesai" },
    { nama: "Stok gudang Bulog", bobot: -0.3, detail: "Cukup untuk 4,2 bulan konsumsi" },
    { nama: "Harga gabah kering panen", bobot: 0.2, detail: "Naik 2% di tingkat petani" },
    { nama: "Momentum harga 7 hari", bobot: 0.06, detail: "Stabil" },
  ],
};

export function mockXai(kode = "CBM-KRT"): ProyeksiXai {
  const kom = komoditasByKode[kode];
  const seri = seriHarga(kom, "1171");
  const kons = seri.map((p) => p.konsumen);
  const proj = proyeksi(kons.slice(-60), 14);
  const kini = kons[kons.length - 1];
  const pct = ((proj[13].nilai - kini) / kini) * 100;
  const lo = ((proj[13].bawah - kini) / kini) * 100;
  const hi = ((proj[13].atas - kini) / kini) * 100;
  const faktor = (FAKTOR[kode] ?? FAKTOR["CBM-KRT"]).map((f) => ({ nama: f.nama, detail: f.detail, kontribusi: Math.round(f.bobot * Math.abs(pct) * (pct < 0 ? -1 : 1) * 10) / 10 }));
  const sorted = [...faktor].sort((a, b) => Math.abs(b.kontribusi) - Math.abs(a.kontribusi));
  const hist = seri.slice(-30).map((p) => ({ tanggal: p.tanggal.toISOString(), aktual: p.konsumen }));
  const lastDate = seri[seri.length - 1].tanggal;
  const fut = proj.map((p) => ({
    tanggal: new Date(lastDate.getTime() + p.h * 86_400_000).toISOString(),
    proyeksi: p.nilai,
    rentang: [p.bawah, p.atas] as [number, number],
  }));
  // sambungkan titik terakhir aktual ke awal proyeksi agar garis kontinu
  const join = { ...hist[hist.length - 1], proyeksi: kini, rentang: [kini, kini] as [number, number] };
  return {
    komoditasKode: kode, nama: kom.nama, horizonHari: 14, perubahanPct: Math.round(pct * 10) / 10,
    interval: [Math.round(lo * 10) / 10, Math.round(hi * 10) / 10], hargaKini: kini,
    seri: [...hist.slice(0, -1), join, ...fut],
    faktor,
    narasi: `${kom.nama} di Kota Banda Aceh diproyeksi ${pct >= 0 ? "naik" : "turun"} ${Math.abs(pct).toFixed(1).replace(".", ",")}% dalam 14 hari. Faktor paling berpengaruh: ${sorted[0].nama.toLowerCase()} (${sorted[0].detail.toLowerCase()}).`,
    model: { nama: "Ensemble LightGBM + SARIMAX", versi: "v2.3.1", mape: kode === "BRS-MED" ? 2.1 : kode === "BWM" ? 5.4 : 6.8, dilatih: "2026-09-28" },
    rekomendasi: pct > 5
      ? ["Siapkan operasi pasar di Pasar Peunayong & Lamdingin", "Petakan pasokan dari sentra Bireuen dan Sumatera Utara", "Ajukan dukungan subsidi ongkos angkut ke Kanwil DJPb"]
      : pct < -3
        ? ["Arahkan surplus ke cold storage terdekat", "Hubungkan agregator dengan pembeli antardaerah"]
        : ["Lanjutkan pemantauan rutin", "Verifikasi silang dengan rilis BPS mingguan"],
  };
}

export const komoditasPilot = KOMODITAS.filter((k) => k.pilot);
export { pasarById };
