import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ApiError } from "./client";
import {
  PERAN_DAPAT_DIPILIH,
  PERAN_OPERASIONAL,
  aturPeranTarget,
  bacaDurasi,
  drafBerubah,
  kelompokkanPeranTarget,
  kodeGalatImpersonasi,
  labelAlasanBerakhir,
  labelPeran,
  queryRiwayat,
  saringAkun,
  setPeranTarget,
  susunMulai,
  toDraf,
  tujuanAman,
  toInput,
  validasiDraf,
  type DrafKebijakan,
  type KebijakanWire,
  type PeranTargetWire,
} from "./impersonasi";

const U1 = "11111111-1111-4111-8111-111111111111";
const U2 = "22222222-2222-4222-8222-222222222222";

function kebijakan(sisip: Partial<KebijakanWire> = {}): KebijakanWire {
  return {
    aktif: true,
    wajib_alasan: true,
    durasi_maks_menit: 60,
    versi: 3,
    diperbarui_pada: "2026-11-17T10:00:00Z",
    pelaku: [
      {
        tipe: "PERAN",
        peran: "MANAJER_MUTU",
        aktif: true,
        target: [
          { tipe: "PERAN", peran: "OPS_MAHASISWA" },
          {
            tipe: "PENGGUNA",
            pengguna_id: U1,
            pengguna_nama: "Zulkifli",
            pengguna_email: "z@x.id",
            pengguna_peran: "OPS_KOORDINATOR",
          },
        ],
      },
      {
        tipe: "PENGGUNA",
        pengguna_id: U2,
        pengguna_nama: "Qa Satu",
        pengguna_email: "qa@x.id",
        pengguna_peran: "MANAJER_DATA",
        aktif: false,
        target: [],
      },
    ],
    ...sisip,
  };
}

describe("peran yang dapat dipilih", () => {
  it("tanpa Administrator Sistem; operasional hanya lapis OPERASIONAL", () => {
    assert.equal(PERAN_DAPAT_DIPILIH.length, 13);
    assert.ok(!PERAN_DAPAT_DIPILIH.includes("ADMIN_SISTEM"));
    assert.deepEqual(
      [...PERAN_OPERASIONAL],
      ["OPS_KOORDINATOR", "OPS_AGREGATOR", "OPS_MAHASISWA"],
    );
  });
});

describe("draf kebijakan", () => {
  it("toDraf lalu toInput mengembalikan masukan PUT tanpa field tampilan, dengan versi kunci optimistis", () => {
    assert.deepEqual(toInput(toDraf(kebijakan()), 3), {
      versi: 3,
      aktif: true,
      wajib_alasan: true,
      durasi_maks_menit: 60,
      pelaku: [
        {
          tipe: "PERAN",
          peran: "MANAJER_MUTU",
          aktif: true,
          target: [
            { tipe: "PERAN", peran: "OPS_MAHASISWA" },
            { tipe: "PENGGUNA", pengguna_id: U1 },
          ],
        },
        { tipe: "PENGGUNA", pengguna_id: U2, aktif: false, target: [] },
      ],
    });
  });

  it("durasi null menjadi isian kosong dan kembali null", () => {
    const d = toDraf(kebijakan({ durasi_maks_menit: null }));
    assert.equal(d.durasiMaks, "");
    assert.equal(toInput(d, 1).durasi_maks_menit, null);
  });

  it("bacaDurasi: kosong null, bilangan bulat apa adanya, selain itu NaN", () => {
    assert.equal(bacaDurasi(""), null);
    assert.equal(bacaDurasi("  "), null);
    assert.equal(bacaDurasi(" 30 "), 30);
    for (const buruk of ["3.5", "-5", "abc", "1e3"]) {
      assert.ok(Number.isNaN(bacaDurasi(buruk)), buruk);
    }
  });

  it("drafBerubah: sama = false; ubah pengaturan, durasi, pelaku, atau target = true; urutan tidak berarti", () => {
    const k = kebijakan();
    const d = toDraf(k);
    assert.equal(drafBerubah(d, k), false);
    assert.equal(drafBerubah({ ...d, aktif: false }, k), true);
    assert.equal(drafBerubah({ ...d, wajibAlasan: false }, k), true);
    assert.equal(drafBerubah({ ...d, durasiMaks: "" }, k), true);
    assert.equal(drafBerubah({ ...d, durasiMaks: " 60 " }, k), false);
    assert.equal(drafBerubah({ ...d, pelaku: [...d.pelaku].reverse() }, k), false);
    assert.equal(drafBerubah({ ...d, pelaku: d.pelaku.slice(1) }, k), true);
    const [p0, p1] = d.pelaku;
    assert.equal(
      drafBerubah({ ...d, pelaku: [{ ...p0, target: [...p0.target].reverse() }, p1] }, k),
      false,
    );
    assert.equal(drafBerubah({ ...d, pelaku: [{ ...p0, target: p0.target.slice(1) }, p1] }, k), true);
    assert.equal(drafBerubah({ ...d, pelaku: [{ ...p0, aktif: false }, p1] }, k), true);
  });
});

describe("validasiDraf", () => {
  const sah = (): DrafKebijakan => toDraf(kebijakan({ pelaku: [] }));

  it("draf sah tidak punya galat", () => {
    assert.deepEqual(validasiDraf(sah()), []);
    assert.deepEqual(
      validasiDraf(toDraf(kebijakan({ pelaku: [kebijakan().pelaku[0]] }))),
      [],
    );
  });

  it("durasi di luar 5..1440 atau bukan bilangan bulat ditolak", () => {
    for (const durasi of ["4", "1441", "abc", "2.5", "-1"]) {
      assert.equal(validasiDraf({ ...sah(), durasiMaks: durasi }).length, 1, durasi);
    }
    for (const durasi of ["", "5", "1440"]) {
      assert.deepEqual(validasiDraf({ ...sah(), durasiMaks: durasi }), [], durasi);
    }
  });

  it("pelaku aktif tanpa target ditolak; pelaku nonaktif tanpa target boleh", () => {
    const d = toDraf(
      kebijakan({
        pelaku: [
          { tipe: "PERAN", peran: "MANAJER_MUTU", aktif: true, target: [] },
          { tipe: "PERAN", peran: "MANAJER_DATA", aktif: false, target: [] },
        ],
      }),
    );
    const galat = validasiDraf(d);
    assert.equal(galat.length, 1);
    assert.match(galat[0], /Manajer|Mutu/);
  });

  it("Administrator Sistem tidak boleh jadi pelaku maupun target; duplikat ditolak", () => {
    const d: DrafKebijakan = {
      ...sah(),
      pelaku: [
        {
          kunci: "a",
          tipe: "PERAN",
          peran: "ADMIN_SISTEM",
          aktif: false,
          target: [],
        },
        {
          kunci: "b",
          tipe: "PERAN",
          peran: "MANAJER_MUTU",
          aktif: true,
          target: [
            { tipe: "PERAN", peran: "ADMIN_SISTEM" },
            { tipe: "PERAN", peran: "OPS_MAHASISWA" },
            { tipe: "PERAN", peran: "OPS_MAHASISWA" },
          ],
        },
        {
          kunci: "c",
          tipe: "PERAN",
          peran: "MANAJER_MUTU",
          aktif: false,
          target: [],
        },
      ],
    };
    const galat = validasiDraf(d).join("\n");
    assert.match(galat, /Administrator Sistem selalu boleh/);
    assert.match(galat, /tidak boleh menjadi target/);
    assert.match(galat, /Target ganda/);
    assert.match(galat, /terdaftar lebih dari sekali/);
  });

  it("pelaku tanpa pilihan (peran/akun kosong) ditolak", () => {
    const d: DrafKebijakan = {
      ...sah(),
      pelaku: [
        { kunci: "a", tipe: "PERAN", aktif: false, target: [] },
        { kunci: "b", tipe: "PENGGUNA", aktif: false, target: [] },
      ],
    };
    assert.equal(validasiDraf(d).length, 2);
  });

  it("batas jumlah pelaku dan target mengikuti API", () => {
    const banyak = Array.from({ length: 101 }, (_, i) => ({
      kunci: `k${i}`,
      tipe: "PENGGUNA" as const,
      penggunaId: `${String(i).padStart(8, "0")}-1111-4111-8111-111111111111`,
      aktif: false,
      target: [],
    }));
    assert.match(validasiDraf({ ...sah(), pelaku: banyak }).join(), /maksimal 100/);
  });
});

describe("target peran", () => {
  it("aturPeranTarget menambah/membuang peran, urut ROLE_IDS, akun tetap", () => {
    const akun = { tipe: "PENGGUNA" as const, penggunaId: U1 };
    let t = aturPeranTarget([akun], "OPS_MAHASISWA", true);
    t = aturPeranTarget(t, "STRATEGIS_BI", true);
    assert.deepEqual(t, [
      { tipe: "PERAN", peran: "STRATEGIS_BI" },
      { tipe: "PERAN", peran: "OPS_MAHASISWA" },
      akun,
    ]);
    assert.deepEqual(aturPeranTarget(t, "STRATEGIS_BI", false), [
      { tipe: "PERAN", peran: "OPS_MAHASISWA" },
      akun,
    ]);
    // idempoten
    assert.deepEqual(aturPeranTarget(t, "OPS_MAHASISWA", true), t);
  });

  it("setPeranTarget mengganti semua peran dan membuang Administrator Sistem, akun tetap", () => {
    const akun = { tipe: "PENGGUNA" as const, penggunaId: U1 };
    const t = setPeranTarget([akun, { tipe: "PERAN", peran: "MANAJER_DATA" }], [
      "OPS_AGREGATOR",
      "ADMIN_SISTEM",
      "OPS_KOORDINATOR",
      "OPS_AGREGATOR",
    ]);
    assert.deepEqual(t, [
      { tipe: "PERAN", peran: "OPS_KOORDINATOR" },
      { tipe: "PERAN", peran: "OPS_AGREGATOR" },
      akun,
    ]);
    assert.deepEqual(setPeranTarget(t, []), [akun]);
    assert.equal(setPeranTarget([], PERAN_DAPAT_DIPILIH).length, 13);
  });
});

describe("pemilih sasaran", () => {
  it("susunMulai: akun didahulukan atas peran; alasan dipangkas dan dibuang bila kosong", () => {
    assert.deepEqual(susunMulai({ peran: "OPS_KOORDINATOR", penggunaId: U1 }, "  uji dasbor "), {
      target_pengguna_id: U1,
      alasan: "uji dasbor",
    });
    assert.deepEqual(susunMulai({ peran: "OPS_KOORDINATOR" }, "   "), {
      target_peran: "OPS_KOORDINATOR",
    });
  });

  const peran = (p: string, lapis: string, n = 1): PeranTargetWire => ({
    peran: p,
    label: p,
    lapis,
    akun: Array.from({ length: n }, (_, i) => ({
      id: `${p}-${i}`,
      nama: `Akun ${p} ${i}`,
      email: `${p.toLowerCase()}${i}@x.id`,
      lembaga: "FMIPA",
    })),
  });

  it("kelompokkanPeranTarget: urutan lapis dan peran baku, lapis tak dikenal di akhir", () => {
    const hasil = kelompokkanPeranTarget([
      peran("OPS_MAHASISWA", "OPERASIONAL"),
      peran("ANEH", "LAIN"),
      peran("MANAJER_DATA", "MANAJERIAL"),
      peran("STRATEGIS_BI", "STRATEGIS"),
      peran("MANAJER_PROGRAM", "MANAJERIAL"),
    ]);
    assert.deepEqual(
      hasil.map((g) => [g.lapis, g.peran.map((p) => p.peran)]),
      [
        ["STRATEGIS", ["STRATEGIS_BI"]],
        ["MANAJERIAL", ["MANAJER_PROGRAM", "MANAJER_DATA"]],
        ["OPERASIONAL", ["OPS_MAHASISWA"]],
        ["LAIN", ["ANEH"]],
      ],
    );
  });

  it("saringAkun mencari nama, email, dan lembaga tanpa peka huruf besar-kecil", () => {
    const akun = peran("OPS_AGREGATOR", "OPERASIONAL", 3).akun;
    assert.equal(saringAkun(akun, "").length, 3);
    assert.deepEqual(saringAkun(akun, "AKUN OPS_AGREGATOR 1").map((a) => a.id), ["OPS_AGREGATOR-1"]);
    assert.equal(saringAkun(akun, "fmipa").length, 3);
    assert.equal(saringAkun(akun, "tidak ada").length, 0);
  });
});

describe("label dan query", () => {
  it("label peran dan alasan berakhir; nilai tak dikenal tampil apa adanya", () => {
    assert.equal(labelPeran("ADMIN_SISTEM"), "Administrator Sistem");
    assert.equal(labelPeran("PERAN_BARU"), "PERAN_BARU");
    assert.equal(labelPeran(null), "");
    assert.equal(labelAlasanBerakhir("STOP"), "Dihentikan pelaku");
    assert.equal(labelAlasanBerakhir("DIAKHIRI"), "Diakhiri administrator");
    assert.equal(labelAlasanBerakhir("BARU"), "BARU");
    assert.equal(labelAlasanBerakhir(null), "");
  });

  it("queryRiwayat hanya memuat parameter yang diisi", () => {
    assert.equal(queryRiwayat({}), "");
    assert.equal(
      queryRiwayat({ status: "aktif", cursor: "abc", limit: 20 }),
      "?status=aktif&cursor=abc&limit=20",
    );
  });

  it("kodeGalatImpersonasi membaca kode mesin dari ApiError BFF", () => {
    assert.equal(
      kodeGalatImpersonasi(new ApiError(409, "x", { kode: "TARGET_AMBIGU" })),
      "TARGET_AMBIGU",
    );
    assert.equal(kodeGalatImpersonasi(new ApiError(500, "x")), undefined);
    assert.equal(kodeGalatImpersonasi(new Error("x")), undefined);
  });
});

describe("tujuanAman", () => {
  it("path internal (dengan query) diterima; selain itu /", () => {
    assert.equal(tujuanAman("/kpi?bulan=2026-11"), "/kpi?bulan=2026-11");
    assert.equal(tujuanAman("/"), "/");
    for (const buruk of [
      undefined,
      null,
      5,
      "",
      "kpi",
      "//evil.example",
      "/\\evil.example",
      "https://evil.example",
      "javascript:alert(1)",
      "/kpi\n/x",
      "/\u0000",
    ]) {
      assert.equal(tujuanAman(buruk), "/", String(buruk));
    }
  });
});
