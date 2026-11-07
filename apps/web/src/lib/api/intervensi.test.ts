import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";
import {
  alurUntukTahap,
  bisaMemutus,
  deskripsiRiwayat,
  persetujuanDiDepan,
  ringkasHasilKartu,
  ringkasRantai,
  toAlur,
  toBuatBody,
  toKartu,
  toPersetujuan,
  toRiwayat,
  toTahapPerSiklus,
  validasiBuat,
  type AlurResponse,
  type BuatIntervensiInput,
  type KartuResponse,
  type PersetujuanResponse,
  type RiwayatResponse,
  type TahapResponse,
} from "./intervensi";
import { alurMock, riwayatMock, tahapMock } from "./intervensi-mock";

const UUID_301 = "a7c1e4f0-3b2d-3c55-8e41-5d0f6b7a9c10";
const UUID_304 = "5e9d2b3c-71aa-3f0e-9b64-0c2e8d41f7a3";

const kartu: KartuResponse = {
  id: UUID_301,
  kode: "INT-301",
  siklus: "TEKANAN",
  tahap: "t-ews",
  judul: "Lonjakan cabai merah Banda Aceh",
  deskripsi: "Harga naik 12% sepekan",
  komoditas_kode: "CBM-KRT",
  kabupaten_kode: "1171",
  prioritas: "TINGGI",
  tenggat: "2026-10-17T09:00:00Z",
  rekomendasi_ai: true,
  progres: { selesai: 1, total: 5 },
  penanggung_jawab: [
    { id: "e810ffa6-9117-330a-aee6-f281fa031dfe", nama: "Rina Safitri, S.T., M.T." },
  ],
  ews_alert_id: "12",
  ews_alert_kode: "EWS-1000",
  dibuat_pada: "2026-10-10T08:00:00Z",
  selesai_pada: null,
  versi: 4,
  dieskalasi_pada: null,
};

describe("toKartu", () => {
  it("memetakan kolom API ke model kanban", () => {
    assert.deepEqual(toKartu(kartu), {
      id: UUID_301,
      kode: "INT-301",
      siklus: "tekanan",
      tahap: "t-ews",
      judul: "Lonjakan cabai merah Banda Aceh",
      komoditasKode: "CBM-KRT",
      kabupatenKode: "1171",
      prioritas: "TINGGI",
      penanggungJawab: [{ nama: "Rina Safitri, S.T., M.T.", inisial: "RS" }],
      tenggat: "2026-10-17T09:00:00Z",
      rekomendasiAi: true,
      progres: { selesai: 1, total: 5 },
      versi: 4,
      dieskalasi: false,
      deskripsi: "Harga naik 12% sepekan",
      ewsAlertId: "12",
      ewsAlertKode: "EWS-1000",
      dibuatPada: "2026-10-10T08:00:00Z",
      selesaiPada: undefined,
    });
  });

  it("menandai kartu yang dieskalasi", () => {
    assert.equal(
      toKartu({ ...kartu, dieskalasi_pada: "2026-11-05T08:00:00Z" }).dieskalasi,
      true,
    );
  });

  it("siklus tak dikenal jatuh ke normal", () => {
    assert.equal(toKartu({ ...kartu, siklus: "ACAK" }).siklus, "normal");
  });

  it("id tetap UUID untuk tautan dan kunci; kode tampilan opsional (null atau tidak ada)", () => {
    assert.equal(toKartu(kartu).id, UUID_301);
    assert.equal(toKartu({ ...kartu, kode: null }).kode, undefined);
    const { kode: _kode, ...tanpaKode } = kartu;
    assert.equal(toKartu(tanpaKode).kode, undefined);
  });

  it("kartu tanpa alert pemicu tidak membawa id maupun kode alert", () => {
    const k = toKartu({ ...kartu, ews_alert_id: null, ews_alert_kode: null });
    assert.equal(k.ewsAlertId, undefined);
    assert.equal(k.ewsAlertKode, undefined);
  });
});

/** Cermin seed intervensi.reference di API Go (internal/intervensi/seeder/seeder_test.go). */
const seed = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../../packages/contracts/intervensi-tahap.json"),
    "utf8",
  ),
) as {
  siklus: string;
  kode: string;
  urutan: number;
  label: string;
  petunjuk: string;
  izin_dibutuhkan: string;
}[];

/** Bentuk respons GET /v1/intervensi/tahap yang dibangun dari fixture kontrak. */
const responsDariSeed: TahapResponse[] = seed.map((x) => ({
  siklus: x.siklus.toUpperCase(),
  kode_tahap: x.kode,
  urutan: x.urutan,
  label: x.label,
  petunjuk: x.petunjuk,
  izin_dibutuhkan: x.izin_dibutuhkan,
  sla_hari: null,
}));

const ringkasTahap = (
  tahap: { id: string; judul: string; petunjuk: string; izinMasuk?: string }[],
) =>
  tahap.map((t) => ({
    kode: t.id,
    label: t.judul,
    petunjuk: t.petunjuk,
    izin: t.izinMasuk ?? "intervensi.kelola",
  }));

describe("cadangan klien siklus.ts sama dengan seed intervensi.reference", () => {
  for (const [siklus, tahap] of Object.entries(SIKLUS_TAHAP)) {
    it(`tahap siklus ${siklus}: kode, urutan, label, petunjuk, dan izin masuk sama`, () => {
      const dariSeed = seed
        .filter((s) => s.siklus === siklus)
        .sort((a, b) => a.urutan - b.urutan);
      assert.deepEqual(
        ringkasTahap(tahap),
        dariSeed.map((s) => ({
          kode: s.kode,
          label: s.label,
          petunjuk: s.petunjuk,
          izin: s.izin_dibutuhkan,
        })),
      );
    });
  }

  it("seed tidak punya siklus yang tidak dikenal web", () => {
    assert.deepEqual(
      [...new Set(seed.map((s) => s.siklus))].sort(),
      Object.keys(SIKLUS_TAHAP).sort(),
    );
  });
});

describe("toTahapPerSiklus (GET /v1/intervensi/tahap)", () => {
  it("memetakan respons yang dibangun dari kontrak menjadi kolom yang sama dengan cadangan klien", () => {
    const hasil = toTahapPerSiklus(responsDariSeed);
    for (const siklus of ["normal", "tekanan", "surplus"] as const) {
      assert.deepEqual(
        ringkasTahap(hasil[siklus]),
        ringkasTahap(SIKLUS_TAHAP[siklus]),
      );
    }
  });

  it("mengurutkan menurut urutan, bukan urutan kedatangan, dan mengabaikan siklus tak dikenal", () => {
    const acak = [...responsDariSeed].reverse();
    acak.push({
      siklus: "ACAK",
      kode_tahap: "x-1",
      urutan: 1,
      label: "X",
      petunjuk: "",
      izin_dibutuhkan: "intervensi.kelola",
      sla_hari: null,
    });
    const hasil = toTahapPerSiklus(acak);
    assert.deepEqual(
      hasil.tekanan.map((t) => t.id),
      SIKLUS_TAHAP.tekanan.map((t) => t.id),
    );
    assert.equal(
      Object.values(hasil)
        .flat()
        .some((t) => t.id === "x-1"),
      false,
    );
  });

  it("izin selain kelola menjadi izinMasuk, SLA ikut terbawa, dan data mock memakai jalur yang sama", () => {
    const hasil = toTahapPerSiklus([
      {
        siklus: "TEKANAN",
        kode_tahap: "t-fiskal",
        urutan: 5,
        label: "Dukungan",
        petunjuk: "p",
        izin_dibutuhkan: "intervensi.setujui_fiskal",
        sla_hari: 5,
      },
      {
        siklus: "TEKANAN",
        kode_tahap: "t-ews",
        urutan: 1,
        label: "EWS",
        petunjuk: "p",
        izin_dibutuhkan: "intervensi.kelola",
        sla_hari: 1,
      },
    ]);
    assert.deepEqual(
      hasil.tekanan.map((t) => [t.id, t.izinMasuk, t.slaHari]),
      [
        ["t-ews", undefined, 1],
        ["t-fiskal", "intervensi.setujui_fiskal", 5],
      ],
    );
    assert.deepEqual(
      ringkasTahap(toTahapPerSiklus(tahapMock()).surplus),
      ringkasTahap(SIKLUS_TAHAP.surplus),
    );
  });
});

const alurFiskal: AlurResponse = {
  kode: "pindah-tekanan-fiskal",
  nama: "Dukungan instrumen belanja",
  jenis: "PINDAH_TAHAP",
  siklus: "TEKANAN",
  tahap_tujuan: "t-fiskal",
  aktif: true,
  langkah: [
    {
      urutan: 2,
      label: "Persetujuan Kanwil DJPb",
      izin_dibutuhkan: "intervensi.setujui_fiskal",
    },
    {
      urutan: 1,
      label: "Persetujuan Dewan Pengarah",
      izin_dibutuhkan: "intervensi.setujui",
    },
  ],
};

describe("alur persetujuan (GET /v1/intervensi/alur)", () => {
  it("toAlur mengurutkan langkah dan mengosongkan siklus alur eskalasi", () => {
    const a = toAlur(alurFiskal);
    assert.equal(a.siklus, "tekanan");
    assert.deepEqual(
      a.langkah.map((l) => l.izin),
      ["intervensi.setujui", "intervensi.setujui_fiskal"],
    );
    assert.equal(
      ringkasRantai(a),
      "Persetujuan Dewan Pengarah → Persetujuan Kanwil DJPb",
    );
    const eskalasi = toAlur({ ...alurMock[4] });
    assert.equal(eskalasi.siklus, undefined);
    assert.equal(eskalasi.tahapTujuan, undefined);
  });

  it("alurUntukTahap hanya mengembalikan alur pemindahan yang aktif", () => {
    const alur = alurMock.map(toAlur);
    assert.equal(
      alurUntukTahap(alur, "t-fiskal")?.kode,
      "pindah-tekanan-fiskal",
    );
    assert.equal(alurUntukTahap(alur, "t-ews"), undefined);
    assert.equal(
      alurUntukTahap(
        alur.map((a) => ({ ...a, aktif: false })),
        "t-fiskal",
      ),
      undefined,
    );
  });

  it("persetujuanDiDepan mendaftar tahap persetujuan setelah tahap kini, berurutan", () => {
    const alur = alurMock.map(toAlur);
    const dariAwal = persetujuanDiDepan(
      alur,
      SIKLUS_TAHAP.tekanan,
      "t-ews",
    ).map((x) => x.tahap.id);
    assert.deepEqual(dariAwal, ["t-petakan", "t-fiskal", "t-selesai"]);
    assert.deepEqual(
      persetujuanDiDepan(alur, SIKLUS_TAHAP.tekanan, "t-logistik").map(
        (x) => x.tahap.id,
      ),
      ["t-fiskal", "t-selesai"],
    );
    assert.deepEqual(
      persetujuanDiDepan(alur, SIKLUS_TAHAP.tekanan, "t-selesai"),
      [],
    );
    assert.deepEqual(
      persetujuanDiDepan(alur, SIKLUS_TAHAP.normal, "n-pantau"),
      [],
    );
  });
});

const labelTahap = (kode?: string) =>
  ({ "t-ews": "EWS terpicu", "t-verifikasi": "Verifikasi cepat BI/BPS" })[
    kode ?? ""
  ] ??
  kode ??
  "";
const riwayat = (r: Partial<RiwayatResponse>) =>
  toRiwayat({
    id: "1",
    aksi: "BUAT",
    dari: null,
    ke: null,
    oleh_id: null,
    oleh_nama: null,
    waktu: "2026-11-05T01:00:00Z",
    alasan: "",
    ...r,
  });

describe("riwayat kartu (GET /v1/intervensi/{id}/riwayat)", () => {
  it("toRiwayat mengosongkan nilai null; pelaku kosong berarti sistem", () => {
    const r = riwayat({ oleh_nama: "Rina" });
    assert.equal(r.olehNama, "Rina");
    assert.equal(r.dari, undefined);
    assert.equal(riwayat({}).olehNama, undefined);
  });

  it("deskripsi tiap aksi menyebut label tahap", () => {
    assert.equal(
      deskripsiRiwayat(riwayat({ aksi: "BUAT", ke: "t-ews" }), labelTahap),
      "Kartu dibuat di tahap “EWS terpicu”",
    );
    assert.equal(
      deskripsiRiwayat(
        riwayat({ aksi: "PINDAH_TAHAP", dari: "t-ews", ke: "t-verifikasi" }),
        labelTahap,
      ),
      "Dipindahkan dari “EWS terpicu” ke “Verifikasi cepat BI/BPS”",
    );
    assert.equal(
      deskripsiRiwayat(
        riwayat({ aksi: "TUGASKAN", oleh_nama: "Rina" }),
        labelTahap,
      ),
      "Rina menjadi penanggung jawab",
    );
    assert.equal(
      deskripsiRiwayat(riwayat({ aksi: "ESKALASI" }), labelTahap),
      "Dieskalasi ke Dewan Pengarah",
    );
    assert.equal(
      deskripsiRiwayat(
        riwayat({ aksi: "AJUKAN", ke: "t-verifikasi" }),
        labelTahap,
      ),
      "Persetujuan diajukan untuk pindah ke “Verifikasi cepat BI/BPS”",
    );
    assert.equal(
      deskripsiRiwayat(riwayat({ aksi: "AJUKAN" }), labelTahap),
      "Telaah eskalasi diajukan",
    );
    assert.equal(
      deskripsiRiwayat(riwayat({ aksi: "SETUJU", ke: "t-ews" }), labelTahap),
      "Persetujuan pindah ke “EWS terpicu” disetujui",
    );
    assert.equal(
      deskripsiRiwayat(riwayat({ aksi: "TOLAK" }), labelTahap),
      "Telaah eskalasi ditolak",
    );
  });

  it("riwayat mock dimulai dengan BUAT di tahap pertama dan berakhir di tahap kartu", () => {
    const kartuMock = { ...toKartu(kartu), tahap: "t-logistik" };
    const rows = riwayatMock(kartuMock);
    assert.deepEqual(
      rows.map((r) => r.aksi),
      ["BUAT", "PINDAH_TAHAP", "PINDAH_TAHAP", "PINDAH_TAHAP"],
    );
    assert.equal(rows[0].ke, "t-ews");
    assert.equal(rows.at(-1)?.ke, "t-logistik");
  });
});

const masukan: BuatIntervensiInput = {
  judul: "  Operasi pasar beras Banda Aceh ",
  deskripsi: " Gelontorkan 20 ton ",
  siklus: "tekanan",
  prioritas: "TINGGI",
  komoditasKode: "BRS-MED",
  kabupatenKode: "1171",
  tenggat: "2026-12-01",
  penanggungJawabIds: ["4a5595b3-31f9-3231-97cc-91bb5aa27394"],
};

describe("membuat intervensi (POST /v1/intervensi)", () => {
  it("validasiBuat menerima masukan lengkap", () => {
    assert.deepEqual(validasiBuat(masukan), {});
  });

  it("validasiBuat menolak judul kosong atau terlalu panjang, rujukan kosong, dan tenggat tak valid", () => {
    assert.equal(
      validasiBuat({ ...masukan, judul: "   " }).judul,
      "Judul wajib diisi",
    );
    assert.equal(
      validasiBuat({ ...masukan, judul: "x".repeat(256) }).judul,
      "Judul maksimal 255 karakter",
    );
    assert.equal(
      validasiBuat({ ...masukan, judul: "x".repeat(255) }).judul,
      undefined,
    );
    assert.equal(
      validasiBuat({ ...masukan, komoditasKode: "" }).komoditasKode,
      "Pilih komoditas",
    );
    assert.equal(
      validasiBuat({ ...masukan, kabupatenKode: "" }).kabupatenKode,
      "Pilih wilayah",
    );
    assert.equal(
      validasiBuat({ ...masukan, tenggat: "" }).tenggat,
      "Tenggat wajib diisi",
    );
    assert.equal(
      validasiBuat({ ...masukan, tenggat: "2026-13-45" }).tenggat,
      "Tenggat wajib diisi",
    );
  });

  it("toBuatBody memotong spasi, memakai enum basis data, dan menaruh tenggat di akhir hari WIB", () => {
    assert.deepEqual(toBuatBody(masukan), {
      judul: "Operasi pasar beras Banda Aceh",
      deskripsi: "Gelontorkan 20 ton",
      siklus: "TEKANAN",
      prioritas: "TINGGI",
      komoditas_kode: "BRS-MED",
      kabupaten_kode: "1171",
      tenggat: "2026-12-01T16:59:00.000Z",
      penanggung_jawab_ids: ["4a5595b3-31f9-3231-97cc-91bb5aa27394"],
    });
  });
});

describe("ringkasHasilKartu", () => {
  it("semua berhasil", () => {
    assert.deepEqual(
      ringkasHasilKartu("Ditugaskan", {
        berhasil: 2,
        gagal: 0,
        hasil: [
          { id: "a", berhasil: true },
          { id: "b", berhasil: true },
        ],
      }),
      {
        judul: "Ditugaskan: 2 kartu",
        deskripsi: "Tercatat di audit trail.",
      },
    );
  });
  it("sebagian dilewati dengan alasan unik", () => {
    const r = ringkasHasilKartu("Dieskalasi", {
      berhasil: 1,
      gagal: 2,
      hasil: [
        { id: "a", berhasil: true },
        { id: "b", berhasil: false, alasan: "Kartu sudah selesai" },
        { id: "c", berhasil: false, alasan: "Kartu sudah selesai" },
      ],
    });
    assert.equal(r.deskripsi, "2 kartu dilewati (Kartu sudah selesai).");
  });
});

const permintaan: PersetujuanResponse = {
  id: "7",
  intervensi_id: UUID_304,
  kartu_judul: "Kirim cabai Takengon → Banda Aceh",
  kartu_tahap: "t-logistik",
  jenis: "PINDAH_TAHAP",
  tahap_tujuan: "t-fiskal",
  status: "MENUNGGU",
  langkah_kini: 2,
  izin_langkah_kini: "intervensi.setujui_fiskal",
  diajukan_oleh_id: "e810ffa6-9117-330a-aee6-f281fa031dfe",
  diajukan_oleh_nama: "Rina Safitri, S.T., M.T.",
  diajukan_pada: "2026-11-05T05:00:00Z",
  catatan: "",
  langkah: [
    {
      urutan: 1,
      label: "Persetujuan Dewan Pengarah",
      izin_dibutuhkan: "intervensi.setujui",
      keputusan: "SETUJU",
      oleh_id: "96815d37-8076-3986-aae4-c350a1bb14cd",
      oleh_nama: "Dr. Muhammad Rizal",
      catatan: "",
    },
    {
      urutan: 2,
      label: "Persetujuan Kanwil DJPb",
      izin_dibutuhkan: "intervensi.setujui_fiskal",
      keputusan: null,
      oleh_id: null,
      oleh_nama: null,
      catatan: "",
    },
  ],
};

describe("toPersetujuan", () => {
  it("memetakan rantai langkah dan mengosongkan nilai null", () => {
    const p = toPersetujuan(permintaan);
    assert.equal(p.id, "7");
    assert.equal(p.kartuId, UUID_304);
    assert.equal(p.tahapTujuan, "t-fiskal");
    assert.equal(p.langkah[0].keputusan, "SETUJU");
    assert.equal(p.langkah[0].olehNama, "Dr. Muhammad Rizal");
    assert.equal(p.langkah[1].keputusan, undefined);
    assert.equal(
      toPersetujuan({
        ...permintaan,
        tahap_tujuan: null,
        diajukan_oleh_id: null,
        diajukan_oleh_nama: null,
      }).diajukanOlehId,
      undefined,
    );
  });
});

describe("bisaMemutus", () => {
  const p = toPersetujuan(permintaan);
  it("pemegang izin langkah yang belum terlibat boleh memutus", () => {
    assert.equal(bisaMemutus(p, "9e93356e-ae55-3f97-8aab-8dd459c17f0d", true), true);
  });
  it("tanpa izin langkah tidak boleh", () => {
    assert.equal(bisaMemutus(p, "96815d37-8076-3986-aae4-c350a1bb14cd", false), false);
  });
  it("pengaju tidak menyetujui sendiri, dan pemutus langkah lain tidak memutus lagi", () => {
    assert.equal(bisaMemutus(p, "e810ffa6-9117-330a-aee6-f281fa031dfe", true), false);
    assert.equal(bisaMemutus(p, "96815d37-8076-3986-aae4-c350a1bb14cd", true), false);
  });
  it("permintaan yang sudah diputuskan tidak dapat diputus", () => {
    assert.equal(
      bisaMemutus({ ...p, status: "DISETUJUI" }, "9e93356e-ae55-3f97-8aab-8dd459c17f0d", true),
      false,
    );
  });
  it("permintaan buatan sistem dapat diputus siapa pun yang berizin", () => {
    assert.equal(
      bisaMemutus({ ...p, diajukanOlehId: undefined }, "9e93356e-ae55-3f97-8aab-8dd459c17f0d", true),
      true,
    );
  });
});
