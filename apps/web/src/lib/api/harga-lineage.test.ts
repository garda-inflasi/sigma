import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { EntriHarga } from "@/lib/data/types";
import {
  labelKolom,
  ringkasPerubahan,
  tautanAuditEntri,
  toLineageHarga,
  toVersiEntri,
  type LineageResponse,
  type VersiEntriResponse,
} from "./harga-lineage";
import { mockLineage } from "./harga-lineage-mock";
import { mockEntri } from "./mock";

const flag = {
  di_luar_rentang: false,
  outlier: false,
  duplikat: false,
  luar_geofence: true,
  tanpa_lokasi: false,
  bentrok: false,
  median: 52000,
};

const versiApi: VersiEntriResponse[] = [
  {
    versi: 1,
    aksi: "INPUT",
    status: "DIAJUKAN",
    harga: 52500,
    oleh: "e3461775-ad20-374a-931e-cf99dd9b8424",
    oleh_nama: "Cut Nanda",
    waktu: "2026-10-16T05:12:00Z",
    perubahan: [],
    data: { id: 1024, status: "DIAJUKAN" },
  },
  {
    versi: 2,
    aksi: "VALIDASI",
    status: "DIVALIDASI",
    harga: 52500,
    oleh: "51dc3298-0976-3724-b5cc-45c2c6c20714",
    oleh_nama: "Rizki Maulana",
    waktu: "2026-10-16T07:30:00Z",
    perubahan: ["divalidasi_oleh", "status"],
    data: { id: 1024, status: "DIVALIDASI" },
  },
];

const respons: LineageResponse = {
  entri: {
    id: "1024",
    kode: "HRG-D00-00-04-K",
    status: "DIVALIDASI",
    harga: 52500,
  },
  asal: {
    sumber: "LAPANGAN",
    petugas_id: "e3461775-ad20-374a-931e-cf99dd9b8424",
    petugas_nama: "Cut Nanda",
    dicatat_pada: "2026-10-16T05:12:00Z",
    diterima_pada: "2026-10-16T05:14:10Z",
    lat: 5.55,
    lng: 95.32,
    dalam_geofence: false,
    kode_pedagang: "PDG-012",
    tugas: [
      {
        id: "7",
        kode: null,
        judul: "Survei pasar pagi",
        ditautkan_pada: "2026-10-16T05:14:10Z",
      },
    ],
  },
  validasi_otomatis: {
    flag_validasi: flag,
    skor_anomali: 0.12,
    deviasi_pct: 3.2,
  },
  validasi: {
    oleh: "51dc3298-0976-3724-b5cc-45c2c6c20714",
    nama: "Rizki Maulana",
    pada: "2026-10-16T07:30:00Z",
  },
  publikasi: null,
  penolakan: null,
  versi_terkini: 2,
  versi: versiApi,
  riwayat: [],
  audit: [
    {
      id: "9",
      waktu: "2026-10-16T05:14:11Z",
      aksi: "HARGA_INPUT",
      aktor_nama: "Cut Nanda",
      aktor_peran: "OPS_MAHASISWA",
      hasil: "BERHASIL",
    },
    {
      id: "12",
      waktu: "2026-10-16T07:30:01Z",
      aksi: "HARGA_VALIDASI",
      aktor_nama: "Rizki Maulana",
      aktor_peran: "OPS_KOORDINATOR",
      hasil: "BERHASIL",
    },
    {
      id: "13",
      waktu: "2026-10-16T07:31:00Z",
      aksi: "HARGA_VALIDASI",
      aktor_nama: "Rizki Maulana",
      aktor_peran: "OPS_KOORDINATOR",
      hasil: "BERHASIL",
    },
  ],
};

describe("toLineageHarga", () => {
  it("memetakan asal, cek otomatis, tahap, versi, dan aksi audit ke model UI", () => {
    const l = toLineageHarga(respons);
    assert.equal(l.petugasNama, "Cut Nanda");
    assert.equal(l.dicatatPada, "2026-10-16T05:12:00Z");
    assert.equal(l.diterimaPada, "2026-10-16T05:14:10Z");
    assert.equal(l.sumber, "LAPANGAN");
    assert.equal(l.kodePedagang, "PDG-012");
    assert.deepEqual(l.tugas, ["Survei pasar pagi"]);
    assert.deepEqual(l.bendera, ["Di luar geofence pasar"]);
    assert.equal(l.skorAnomali, 0.12);
    assert.equal(l.deviasiPct, 3.2);
    assert.deepEqual(l.validasi, {
      olehNama: "Rizki Maulana",
      pada: "2026-10-16T07:30:00Z",
    });
    assert.equal(l.versiTerkini, 2);
    assert.deepEqual(
      l.versi.map((v) => [v.versi, v.aksi, v.status, v.olehNama]),
      [
        [1, "INPUT", "DIAJUKAN", "Cut Nanda"],
        [2, "VALIDASI", "DIVALIDASI", "Rizki Maulana"],
      ],
    );
    assert.deepEqual(
      l.aksiAudit,
      ["HARGA_INPUT", "HARGA_VALIDASI"],
      "aksi audit tanpa duplikat",
    );
  });

  it("tahap yang belum berlaku menjadi undefined dan penolakan membawa alasan", () => {
    const belum = toLineageHarga(respons);
    assert.equal(belum.publikasi, undefined);
    assert.equal(belum.penolakan, undefined);

    const ditolak = toLineageHarga({
      ...respons,
      validasi: null,
      penolakan: {
        oleh: "u1",
        nama: null,
        pada: "2026-10-16T07:40:00Z",
        alasan: "bukti kurang",
      },
    });
    assert.equal(ditolak.validasi, undefined);
    assert.deepEqual(ditolak.penolakan, {
      pada: "2026-10-16T07:40:00Z",
      alasan: "bukti kurang",
    });
  });

  it("petugas, pedagang, dan tugas yang kosong tidak membawa kunci kosong", () => {
    const l = toLineageHarga({
      ...respons,
      asal: {
        ...respons.asal,
        petugas_nama: null,
        kode_pedagang: null,
        tugas: [],
      },
      validasi_otomatis: {
        flag_validasi: { ...flag, luar_geofence: false },
        skor_anomali: 0,
        deviasi_pct: 0,
      },
      audit: [],
    });
    assert.equal("petugasNama" in l, false);
    assert.equal("kodePedagang" in l, false);
    assert.deepEqual(l.tugas, []);
    assert.deepEqual(l.bendera, []);
    assert.deepEqual(l.aksiAudit, []);
  });
});

describe("toVersiEntri", () => {
  it("pelaku yang tidak diketahui (versi migrasi) tidak membawa nama", () => {
    const v = toVersiEntri({
      ...versiApi[0],
      aksi: "MIGRASI",
      oleh: null,
      oleh_nama: null,
    });
    assert.equal(v.aksi, "MIGRASI");
    assert.equal("olehNama" in v, false);
  });

  it("perubahan yang kosong dari server tetap larik", () => {
    const v = toVersiEntri({
      ...versiApi[0],
      perubahan: undefined as unknown as string[],
    });
    assert.deepEqual(v.perubahan, []);
  });
});

describe("label kolom dan tautan audit", () => {
  it("kolom dikenal diberi label, yang tak dikenal diubah garis bawahnya", () => {
    assert.equal(labelKolom("divalidasi_oleh"), "validator");
    assert.equal(labelKolom("kolom_baru_lain"), "kolom baru lain");
  });

  it("ringkasan perubahan menggabungkan label yang sama (lat dan lng menjadi lokasi)", () => {
    assert.equal(
      ringkasPerubahan(["divalidasi_oleh", "status"]),
      "validator, status",
    );
    assert.equal(ringkasPerubahan(["lat", "lng"]), "lokasi");
    assert.equal(ringkasPerubahan([]), "");
  });

  it("tautan audit menyaring entitas entri_harga pada ID entri", () => {
    assert.equal(
      tautanAuditEntri("1024"),
      "/audit?entitas=entri_harga&entitas_id=1024",
    );
  });
});

describe("mockLineage", () => {
  const entri = (status: EntriHarga["status"]) =>
    mockEntri().find((e) => e.status === status)!;

  it("tiap status menghasilkan jalur versi yang sama dengan pemicu basis data", () => {
    const jalur = (s: EntriHarga["status"]) =>
      mockLineage(entri(s)).versi.map((v) => `${v.aksi}/${v.status}`);
    assert.deepEqual(jalur("DIAJUKAN"), ["INPUT/DIAJUKAN"]);
    assert.deepEqual(jalur("DIVALIDASI"), [
      "INPUT/DIAJUKAN",
      "VALIDASI/DIVALIDASI",
    ]);
    assert.deepEqual(jalur("DIPUBLIKASI"), [
      "INPUT/DIAJUKAN",
      "VALIDASI/DIVALIDASI",
      "PUBLIKASI/DIPUBLIKASI",
    ]);
    assert.deepEqual(jalur("DITOLAK"), ["INPUT/DIAJUKAN", "TOLAK/DITOLAK"]);
    assert.deepEqual(jalur("DRAF"), ["INPUT/DIAJUKAN", "KEMBALIKAN/DRAF"]);
  });

  it("blok tahap mengikuti status dan versi terkini sama dengan jumlah versi", () => {
    const terbit = mockLineage(entri("DIPUBLIKASI"));
    assert.ok(terbit.validasi && terbit.publikasi);
    assert.equal(terbit.penolakan, undefined);
    assert.equal(terbit.versiTerkini, terbit.versi.length);

    const tolak = mockLineage(entri("DITOLAK"));
    assert.equal(tolak.publikasi, undefined);
    assert.ok(tolak.penolakan?.alasan);

    const baru = mockLineage(entri("DIAJUKAN"));
    assert.equal(baru.validasi, undefined);
  });

  it("waktu versi tidak mendahului pencatatan dan tidak melampaui sekarang", () => {
    const sekarang = Date.now() + 1000;
    for (const s of ["DIVALIDASI", "DIPUBLIKASI"] as const) {
      const e = entri(s);
      for (const v of mockLineage(e).versi) {
        const t = new Date(v.waktu).getTime();
        assert.ok(t >= new Date(e.dicatatPada).getTime(), `${s} v${v.versi}`);
        assert.ok(t <= sekarang, `${s} v${v.versi}`);
      }
    }
  });
});
