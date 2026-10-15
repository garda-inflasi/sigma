/* eslint-disable no-console */
/**
 * Seeder AI Garda Inflasi — data sintetis deterministik, konsisten dengan mode mock UI.
 *   • Master: 1 provinsi, 23 kab/kota (kode BPS), 12 pasar, 14 komoditas (3 pilot)
 *   • Pengguna 3 lapis + admin (27 akun, domain .test)
 *   • Harga 90 hari: 12 pasar × 14 komoditas × 2 tipe (≈30 ribu baris) + pembanding PIHPS mingguan
 *   • Cuaca 90 hari × 23 kab/kota, aturan & alert EWS, model registry, prediksi 14 hari + faktor XAI
 *   • Cold storage + stok, ruang fiskal, policy brief, kanban intervensi 3 siklus, tugas, audit
 * Jalankan: npm run db:push && npm run db:seed
 */
import { PrismaClient } from "@prisma/client";
import bcrypt from "bcryptjs";
import { randomBytes } from "node:crypto";
import { mockAudit, mockBriefs, mockColdStorage, mockEksekutif, mockEws, mockKanban, mockSumber, mockTugas, mockXai } from "../lib/api/mock";
import { HARI_SERI, hashSeed, kabupatenBerpasar, mulberry32, proyeksi, seriCuaca, seriHarga } from "../lib/data/generators";
import { COLD_STORAGE, KABUPATEN, KOMODITAS, PASAR, PROVINSI, USERS } from "../lib/data/reference";
import { SIKLUS_TAHAP } from "../lib/data/siklus";
import { ROLES } from "../lib/rbac/roles";

const prisma = new PrismaClient();
const CHUNK = 4000;
const rng = mulberry32(hashSeed("seed-garda"));

async function chunked<T>(rows: T[], fn: (batch: T[]) => Promise<unknown>) {
  for (let i = 0; i < rows.length; i += CHUNK) await fn(rows.slice(i, i + CHUNK));
}
const userBy = (pred: (u: (typeof USERS)[number]) => boolean) => USERS.find(pred);
const idByNama = (nama: string) => USERS.find((u) => u.nama === nama)?.id;

async function bersihkan() {
  // urutan mengikuti dependensi FK
  await prisma.auditLog.deleteMany();
  await prisma.perubahanMaster.deleteMany();
  await prisma.intervensi.deleteMany();
  await prisma.tugasLapangan.deleteMany();
  await prisma.policyBrief.deleteMany();
  await prisma.ruangFiskal.deleteMany();
  await prisma.stokColdStorage.deleteMany();
  await prisma.coldStorage.deleteMany();
  await prisma.prediksiHarga.deleteMany();
  await prisma.modelRegistry.deleteMany();
  await prisma.ewsAlert.deleteMany();
  await prisma.aturanEws.deleteMany();
  await prisma.sumberData.deleteMany();
  await prisma.cuacaHarian.deleteMany();
  await prisma.entriHarga.deleteMany();
  await prisma.pengguna.deleteMany();
  await prisma.pasar.deleteMany();
  await prisma.komoditas.deleteMany();
  await prisma.kabupaten.deleteMany();
  await prisma.provinsi.deleteMany();
}

async function main() {
  console.time("seed");
  await bersihkan();

  /* 1 ─ Master data */
  await prisma.provinsi.create({ data: { kode: PROVINSI.kode, nama: PROVINSI.nama } });
  await prisma.kabupaten.createMany({ data: KABUPATEN.map((k) => ({ ...k, provinsiKode: PROVINSI.kode })) });
  await prisma.komoditas.createMany({
    data: KOMODITAS.map((k) => ({ kode: k.kode, nama: k.nama, satuan: k.satuan, kategori: k.kategori, hargaAcuan: k.hargaAcuan, pilot: k.pilot, shelfLifeHari: k.shelfLifeHari })),
  });
  await prisma.pasar.createMany({ data: PASAR.map((p) => ({ ...p })) });

  /* 2 ─ Pengguna 3 lapis */
  const password = process.env.SEED_PASSWORD || randomBytes(9).toString("base64url");
  const hash = await bcrypt.hash(password, 10);
  await prisma.pengguna.createMany({
    data: USERS.map((u, i) => ({
      id: u.id, nama: u.nama, email: u.email, passwordHash: hash, peran: u.role, lapis: ROLES[u.role].lapis, lembaga: u.lembaga,
      kabupatenKode: u.kabupatenKode ?? null, mfaAktif: ROLES[u.role].mfaWajib, status: i === 7 ? "MENUNGGU" : "AKTIF",
      terakhirMasuk: new Date(Date.now() - Math.floor(rng() * 72) * 3_600_000),
    })),
  });

  /* 3 ─ Harga 90 hari (lapangan) + pembanding PIHPS mingguan */
  const risiko = userBy((u) => u.role === "MANAJER_RISIKO")!;
  const mhsSemua = USERS.filter((u) => u.role === "OPS_MAHASISWA");
  const rows: Record<string, unknown>[] = [];
  for (const pasar of PASAR) {
    const mhs = mhsSemua.filter((u) => u.kabupatenKode === pasar.kabupatenKode);
    const koord = userBy((u) => u.role === "OPS_KOORDINATOR" && u.kabupatenKode === pasar.kabupatenKode) ?? userBy((u) => u.role === "OPS_KOORDINATOR")!;
    for (const kom of KOMODITAS) {
      const seri = seriHarga(kom, pasar.kabupatenKode, pasar.id);
      seri.forEach((p, t) => {
        for (const tipe of ["PRODUSEN", "KONSUMEN"] as const) {
          const harga = tipe === "KONSUMEN" ? p.konsumen : p.produsen;
          const prev = seri.slice(Math.max(0, t - 7), t).map((x) => (tipe === "KONSUMEN" ? x.konsumen : x.produsen));
          const mean = prev.length ? prev.reduce((a, b) => a + b, 0) / prev.length : harga;
          const dev = ((harga - mean) / mean) * 100;
          const terakhir = t === HARI_SERI - 1;
          const r = rng();
          const status = t < HARI_SERI - 2 ? "DIPUBLIKASI" : t === HARI_SERI - 2 ? "DIVALIDASI" : r < 0.7 ? "DIAJUKAN" : r < 0.9 ? "DIVALIDASI" : r < 0.95 ? "DITOLAK" : "DRAF";
          const petugas = (mhs.length ? mhs : mhsSemua)[(t + kom.kode.length) % (mhs.length || mhsSemua.length)];
          const waktu = new Date(p.tanggal.getTime() + Math.floor(rng() * 150) * 60_000);
          rows.push({
            komoditasKode: kom.kode, pasarId: pasar.id, tipe, harga, satuan: kom.satuan, kualitas: r < 0.2 ? "PREMIUM" : r > 0.9 ? "ASALAN" : "MEDIUM",
            sumber: "LAPANGAN", status, deviasiPct: Math.round(dev * 10) / 10, skorAnomali: Math.min(1, Math.abs(dev) / 40),
            lat: pasar.lat + (rng() - 0.5) * 0.002, lng: pasar.lng + (rng() - 0.5) * 0.002, dalamGeofence: rng() > 0.03,
            dicatatPada: waktu, petugasId: petugas.id,
            divalidasiOleh: status === "DIVALIDASI" || status === "DIPUBLIKASI" ? koord.id : null,
            divalidasiPada: status === "DIVALIDASI" || status === "DIPUBLIKASI" ? new Date(waktu.getTime() + 3_600_000) : null,
            dipublikasiOleh: status === "DIPUBLIKASI" ? risiko.id : null,
            catatan: terakhir && Math.abs(dev) > 10 ? "Pasokan dari sentra berkurang, pedagang menaikkan harga." : null,
          });
        }
        if (t % 7 === 0) {
          rows.push({
            komoditasKode: kom.kode, pasarId: pasar.id, tipe: "KONSUMEN", harga: Math.round((p.konsumen * (1 + (rng() - 0.5) * 0.04)) / 100) * 100,
            satuan: kom.satuan, sumber: "PIHPS", status: "DIPUBLIKASI", dicatatPada: new Date(p.tanggal.getTime() + 2 * 3_600_000), petugasId: null,
          });
        }
      });
    }
  }
  await chunked(rows, (batch) => prisma.entriHarga.createMany({ data: batch as never }));

  /* 4 ─ Cuaca (BMKG) */
  await chunked(
    KABUPATEN.flatMap((k) => seriCuaca(k.kode).map((c) => ({ kabupatenKode: k.kode, tanggal: c.tanggal, curahHujanMm: c.curahHujanMm, suhuC: c.suhuC, kelembapan: c.kelembapan, anomali: c.anomali, sumber: "BMKG" }))),
    (batch) => prisma.cuacaHarian.createMany({ data: batch }),
  );

  /* 5 ─ Sumber data API & scraper */
  const CRON: Record<string, string> = { "Harian 06.00": "0 6 * * *", "Harian 10.00": "0 10 * * *", "Harian 11.00": "0 11 * * *", "Harian 12.00": "0 12 * * *", "Mingguan Senin": "0 7 * * 1", "Tiap 3 jam": "0 */3 * * *", "Tiap jam": "0 * * * *", "Tiap 5 menit": "*/5 * * * *" };
  await prisma.sumberData.createMany({
    data: mockSumber().map((s) => ({
      id: s.id, nama: s.nama, penyedia: s.penyedia, jenis: s.jenis, cakupan: s.cakupan, jadwalCron: CRON[s.jadwal] ?? "0 6 * * *", status: s.status,
      terakhirJalan: new Date(s.terakhir), keberhasilan7h: s.keberhasilan7h, latensiMs: s.latensiMs, robotsTxtPatuh: true, atribusi: `Sumber: ${s.penyedia}`,
    })),
  });

  /* 6 ─ Aturan EWS per kategori (ambang Δ% vs rata-rata 7 hari) */
  const AMBANG: Record<string, [number, number, number]> = { "Pangan pokok": [3, 5, 8], Hortikultura: [8, 12, 18], Peternakan: [5, 8, 12], Perikanan: [6, 10, 15], "Pangan olahan": [3, 5, 8] };
  await prisma.aturanEws.createMany({
    data: [
      ...KOMODITAS.map((k) => ({ komoditasKode: k.kode, jenis: "LONJAKAN_HARGA" as const, ambangWaspada: AMBANG[k.kategori][0], ambangSiaga: AMBANG[k.kategori][1], ambangKritis: AMBANG[k.kategori][2], kanal: ["DASHBOARD", "PUSH", "EMAIL", "WHATSAPP"] })),
      { komoditasKode: null, jenis: "OVER_SUPPLY" as const, ambangWaspada: -7, ambangSiaga: -12, ambangKritis: -18, kanal: ["DASHBOARD", "PUSH"] },
      { komoditasKode: null, jenis: "ANOMALI_CUACA" as const, ambangWaspada: 1.5, ambangSiaga: 2, ambangKritis: 2.5, jendelaHari: 10, kanal: ["DASHBOARD", "PUSH", "EMAIL"] },
      { komoditasKode: null, jenis: "KELANGKAAN_STOK" as const, ambangWaspada: -20, ambangSiaga: -30, ambangKritis: -45, jendelaHari: 5, kanal: ["DASHBOARD", "PUSH", "WHATSAPP"] },
    ],
  });

  /* 7 ─ Alert EWS */
  const pjData = userBy((u) => u.role === "MANAJER_DATA")!;
  const ews = mockEws();
  await prisma.ewsAlert.createMany({
    data: ews.map((a) => ({
      id: a.id, severity: a.severity, jenis: a.jenis, judul: a.judul, pesan: a.pesan, komoditasKode: a.komoditasKode ?? null, kabupatenKode: a.kabupatenKode,
      nilai: a.nilai, satuanNilai: a.satuanNilai, skorDampak: a.skorDampak, status: a.status, sumber: a.sumber, dibuatPada: new Date(a.waktu),
      diakuiOlehId: a.status !== "BARU" ? pjData.id : null, diakuiPada: a.status !== "BARU" ? new Date(new Date(a.waktu).getTime() + 20 * 60_000) : null,
    })),
  });

  /* 8 ─ Model registry (MLOps) */
  await prisma.modelRegistry.createMany({
    data: KOMODITAS.flatMap((k) => {
      const base = k.volatilitas * 180 + 1.5;
      return [
        { komoditasKode: k.kode, nama: "Ensemble LightGBM + SARIMAX", versi: "v2.3.1", mape: +(base).toFixed(2), drift: +(rng() * 0.12).toFixed(3), status: "PRODUKSI" as const, dilatihPada: new Date("2026-09-28") },
        { komoditasKode: k.kode, nama: "Ensemble LightGBM + SARIMAX", versi: "v2.4.0-rc1", mape: +(base * 0.93).toFixed(2), drift: 0, status: "KANDIDAT" as const, dilatihPada: new Date("2026-10-02") },
        { komoditasKode: k.kode, nama: "Gradient boosting", versi: "v2.2.0", mape: +(base * 1.15).toFixed(2), drift: +(0.1 + rng() * 0.15).toFixed(3), status: "ARSIP" as const, dilatihPada: new Date("2026-08-15") },
      ];
    }),
  });

  /* 9 ─ Prediksi 14 hari komoditas pilot + faktor XAI */
  const prediksi = KOMODITAS.filter((k) => k.pilot).flatMap((k) => {
    const faktor = JSON.parse(JSON.stringify(mockXai(k.kode).faktor)); // → nilai Json murni
    return kabupatenBerpasar().flatMap((kab) => {
      const seri = seriHarga(k, kab.kode);
      const akhir = seri[seri.length - 1].tanggal;
      return proyeksi(seri.slice(-60).map((p) => p.konsumen), 14).map((p) => ({
        komoditasKode: k.kode, kabupatenKode: kab.kode, tanggalTarget: new Date(akhir.getTime() + p.h * 86_400_000), horizonHari: p.h,
        nilai: p.nilai, bawah: p.bawah, atas: p.atas, modelVersi: "v2.3.1", faktor,
      }));
    });
  });
  await chunked(prediksi, (batch) => prisma.prediksiHarga.createMany({ data: batch }));

  /* 10 ─ Cold storage & stok */
  await prisma.coldStorage.createMany({
    data: COLD_STORAGE.map((c) => ({ id: c.id, nama: c.nama, kabupatenKode: c.kabupatenKode, lat: c.lat, lng: c.lng, kapasitasTon: c.kapasitasTon, suhuTargetC: c.suhuTargetC, status: c.status })),
  });
  await prisma.stokColdStorage.createMany({
    data: mockColdStorage().stok.map((s) => ({ id: s.id, coldStorageId: s.fasilitasId, komoditasKode: s.komoditasKode, volumeTon: s.volumeTon, masukPada: new Date(s.masuk), pemilik: s.pemilik, risikoFoodLoss: s.risikoFoodLoss })),
  });

  /* 11 ─ Ruang fiskal (Kanwil DJPb) */
  const fiskal = mockEksekutif().fiskal;
  await prisma.ruangFiskal.createMany({
    data: fiskal.instrumen.map((i) => ({ tahun: 2026, instrumen: i.nama, pagu: BigInt(Math.round(i.pagu)), realisasi: BigInt(Math.round(i.realisasi)), perTanggal: new Date(fiskal.perTanggal) })),
  });

  /* 12 ─ Policy brief */
  for (const b of mockBriefs()) {
    await prisma.policyBrief.create({
      data: {
        id: b.id, periode: b.periode, judul: b.judul, status: b.status, halaman: b.halaman,
        disusunOlehId: idByNama(b.disusunOleh) ?? USERS[6].id,
        ringkasan: `Ringkasan ${b.periode}: ${b.judul}.`,
        rekomendasi: b.id === "PB-2026-10" ? mockXai("CBM-KRT").rekomendasi : [`${b.jumlahRekomendasi} rekomendasi tercantum di dokumen.`],
      },
    });
  }

  /* 13 ─ Kanban intervensi 3 siklus */
  const program = userBy((u) => u.role === "MANAJER_PROGRAM")!;
  const urutan: Record<string, number> = {};
  for (const c of mockKanban()) {
    const ewsTerkait = c.rekomendasiAi ? ews.find((a) => a.komoditasKode === c.komoditasKode && a.kabupatenKode === c.kabupatenKode) : undefined;
    urutan[c.tahap] = (urutan[c.tahap] ?? -1) + 1;
    await prisma.intervensi.create({
      data: {
        id: c.id, siklus: c.siklus.toUpperCase() as "NORMAL" | "TEKANAN" | "SURPLUS", tahap: c.tahap, urutan: urutan[c.tahap], judul: c.judul,
        komoditasKode: c.komoditasKode, kabupatenKode: c.kabupatenKode, prioritas: c.prioritas, tenggat: new Date(c.tenggat), rekomendasiAi: c.rekomendasiAi,
        checklistSelesai: c.progres.selesai, checklistTotal: c.progres.total, ewsAlertId: ewsTerkait?.id ?? null, dibuatOlehId: program.id,
        penanggungJawab: { connect: c.penanggungJawab.map((p) => ({ id: idByNama(p.nama)! })).filter((x) => x.id) },
      },
    });
  }
  const tahapValid = new Set(Object.values(SIKLUS_TAHAP).flat().map((t) => t.id));
  if (Object.keys(urutan).some((t) => !tahapValid.has(t))) throw new Error("Tahap intervensi tidak dikenal");

  /* 14 ─ Tugas lapangan */
  const tugasUtama = mockTugas().map((t) => ({ ...t, petugasId: "usr-mhs-01" }));
  const tugasLain = mhsSemua.slice(1).flatMap((u, i) => {
    const pasar = PASAR.find((p) => p.kabupatenKode === u.kabupatenKode) ?? PASAR[0];
    return [0, 1].map((j) => ({
      id: `TGS-${900 + i * 2 + j}`, judul: j === 0 ? "Survei harga konsumen pagi" : "Harga produsen kelompok tani", pasarId: pasar.id, petugasId: u.id,
      komoditas: j === 0 ? ["CBM-KRT", "BWM", "BRS-MED", "TLR"] : ["CBM-KRT", "BWM", "TMT"], tipe: j === 0 ? ("KONSUMEN" as const) : ("PRODUSEN" as const),
      tenggat: new Date(Date.now() + (j ? 26 : 3) * 3_600_000).toISOString(), status: "BELUM" as const, target: j === 0 ? 4 : 3,
    }));
  });
  await prisma.tugasLapangan.createMany({
    data: [...tugasUtama, ...tugasLain].map((t) => ({ id: t.id, judul: t.judul, pasarId: t.pasarId, petugasId: t.petugasId, komoditas: t.komoditas, tipe: t.tipe, tenggat: new Date(t.tenggat), status: t.status, target: t.target })),
  });

  /* 15 ─ Audit trail & versi master */
  await prisma.auditLog.createMany({
    data: mockAudit().map((a) => ({ aktorId: idByNama(a.aktor) ?? null, aksi: a.aksi, entitas: a.entitas, entitasId: a.entitasId, hasil: a.hasil, ip: a.ip, waktu: new Date(a.waktu), metadata: { peran: a.peran } })),
  });
  await prisma.perubahanMaster.create({
    data: { entitas: "Komoditas", entitasId: "CBR-RWT", versi: 2, perubahan: { hargaAcuan: { dari: 58000, ke: 60000 }, alasan: "Penyesuaian rata-rata 3 bulan" }, diajukanOlehId: pjData.id },
  });
  await prisma.komoditas.update({ where: { kode: "CBR-RWT" }, data: { status: "MENUNGGU_PERSETUJUAN" } });

  const ringkas = {
    kabupaten: await prisma.kabupaten.count(), pasar: await prisma.pasar.count(), komoditas: await prisma.komoditas.count(),
    pengguna: await prisma.pengguna.count(), entriHarga: await prisma.entriHarga.count(), cuaca: await prisma.cuacaHarian.count(),
    ews: await prisma.ewsAlert.count(), prediksi: await prisma.prediksiHarga.count(), intervensi: await prisma.intervensi.count(),
  };
  console.table(ringkas);
  console.log(process.env.SEED_PASSWORD ? "Password akun: dari SEED_PASSWORD" : `Password awal semua akun (hanya dev): ${password}`);
  console.timeEnd("seed");
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  })
  .finally(() => prisma.$disconnect());
