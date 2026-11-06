import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  akhirHari,
  balikIzin,
  galatTanggalBerlaku,
  hariIni,
  izinTersedia,
  samaIzin,
  selisihDraf,
  toIzinLangsung,
  toIzinPengguna,
  toMatriksAkses,
  urutkanSesuaiKatalog,
  type IzinPengguna,
  type IzinWire,
  type PeranWire,
} from "./rbac";

const izinWire: IzinWire[] = [
  { id: "harga.lihat", deskripsi: "Melihat harga", kelompok: "Harga" },
  { id: "harga.input", deskripsi: "Input harga", kelompok: "Harga" },
  { id: "kpi.lihat", deskripsi: "Melihat KPI", kelompok: "KPI" },
];

const peranWire = (
  id: string,
  permissions: string[],
  versi = 1,
  bawaan = permissions,
): PeranWire => ({
  id,
  label: `Label ${id}`,
  lapis: "OPERASIONAL",
  permissions,
  bawaan,
  versi,
  diperbarui_pada: "2026-06-01T00:00:00Z",
});

describe("toMatriksAkses", () => {
  it("memetakan peran dan mengelompokkan izin sesuai urutan katalog", () => {
    const m = toMatriksAkses(
      [
        peranWire("OPS_MAHASISWA", ["harga.input"], 3, [
          "harga.input",
          "harga.lihat",
        ]),
      ],
      izinWire,
    );

    assert.equal(m.peran.length, 1);
    assert.equal(m.peran[0].id, "OPS_MAHASISWA");
    assert.equal(m.peran[0].versi, 3);
    assert.deepEqual([...m.peran[0].izin], ["harga.input"]);
    assert.deepEqual([...m.peran[0].bawaan], ["harga.input", "harga.lihat"]);
    assert.deepEqual(
      m.kelompok.map((k) => [k.nama, k.izin.map((i) => i.id)]),
      [
        ["Harga", ["harga.lihat", "harga.input"]],
        ["KPI", ["kpi.lihat"]],
      ],
    );
    assert.equal(m.izin.length, 3);
  });

  it("mengabaikan peran di luar 14 nilai enum", () => {
    const m = toMatriksAkses(
      [peranWire("SUPERADMIN", []), peranWire("ADMIN_SISTEM", ["kpi.lihat"])],
      izinWire,
    );
    assert.deepEqual(
      m.peran.map((p) => p.id),
      ["ADMIN_SISTEM"],
    );
  });
});

describe("draf matriks", () => {
  it("samaIzin membandingkan anggota, bukan urutan", () => {
    assert.equal(samaIzin(new Set(["a", "b"]), new Set(["b", "a"])), true);
    assert.equal(samaIzin(new Set(["a"]), new Set(["a", "b"])), false);
    assert.equal(samaIzin(new Set(["a", "c"]), new Set(["a", "b"])), false);
    assert.equal(samaIzin(new Set(), new Set()), true);
  });

  it("selisihDraf memisahkan izin yang ditambah dan dicabut", () => {
    assert.deepEqual(selisihDraf(new Set(["a", "b"]), new Set(["b", "c"])), {
      ditambah: ["c"],
      dicabut: ["a"],
    });
    assert.deepEqual(selisihDraf(new Set(["a"]), new Set(["a"])), {
      ditambah: [],
      dicabut: [],
    });
  });

  it("balikIzin tidak mengubah himpunan asal", () => {
    const asal = new Set(["a"]);
    const tambah = balikIzin(asal, "b");
    const cabut = balikIzin(asal, "a");
    assert.deepEqual([...tambah].sort(), ["a", "b"]);
    assert.deepEqual([...cabut], []);
    assert.deepEqual([...asal], ["a"]);
  });

  it("urutkanSesuaiKatalog mengikuti urutan katalog dan membuang izin asing", () => {
    const katalog = toMatriksAkses([], izinWire).izin;
    assert.deepEqual(
      urutkanSesuaiKatalog(
        new Set(["kpi.lihat", "harga.lihat", "izin.palsu"]),
        katalog,
      ),
      ["harga.lihat", "kpi.lihat"],
    );
  });
});

describe("izin langsung", () => {
  const wire = {
    izin: "kpi.lihat",
    deskripsi: "Melihat KPI",
    kelompok: "KPI",
    berlaku_sampai: null,
    alasan: "Proyek",
    diberikan_oleh: "admin-1",
    diberikan_oleh_nama: "Admin Satu",
    diberikan_pada: "2026-06-01T00:00:00Z",
    aktif: true,
  };

  it("toIzinLangsung memakai nama pemberi, lalu ID, dan null menjadi tanpa batas", () => {
    assert.equal(toIzinLangsung(wire).diberikanOleh, "Admin Satu");
    assert.equal(
      toIzinLangsung({ ...wire, diberikan_oleh_nama: null }).diberikanOleh,
      "admin-1",
    );
    assert.equal(
      toIzinLangsung({
        ...wire,
        diberikan_oleh_nama: null,
        diberikan_oleh: null,
      }).diberikanOleh,
      undefined,
    );
    assert.equal(toIzinLangsung(wire).berlakuSampai, undefined);
    assert.equal(
      toIzinLangsung({ ...wire, berlaku_sampai: "2030-01-01T00:00:00Z" })
        .berlakuSampai,
      "2030-01-01T00:00:00Z",
    );
  });

  it("izinTersedia membuang izin dari peran dan izin langsung yang masih berlaku, tetapi menyisakan yang kedaluwarsa", () => {
    const katalog = toMatriksAkses([], izinWire).izin;
    const pengguna: IzinPengguna = toIzinPengguna({
      peran: "OPS_MAHASISWA",
      izin_peran: ["harga.input"],
      efektif: ["harga.input", "kpi.lihat"],
      langsung: [wire, { ...wire, izin: "harga.lihat", aktif: false }],
    });
    assert.deepEqual(
      izinTersedia(katalog, pengguna).map((i) => i.id),
      ["harga.lihat"],
    );
  });
});

describe("masa berlaku", () => {
  const sekarang = new Date(2026, 5, 10, 15, 30);

  it("akhirHari menghasilkan akhir hari lokal; tanggal tidak sah null", () => {
    const iso = akhirHari("2026-06-10");
    assert.ok(iso);
    const d = new Date(iso);
    assert.deepEqual(
      [
        d.getFullYear(),
        d.getMonth(),
        d.getDate(),
        d.getHours(),
        d.getMinutes(),
      ],
      [2026, 5, 10, 23, 59],
    );
    for (const salah of [
      "",
      "10-06-2026",
      "2026-13-01",
      "2026-02-30",
      "besok",
    ]) {
      assert.equal(akhirHari(salah), null, salah);
    }
  });

  it("galatTanggalBerlaku: kosong sah, hari ini sah (akhir hari), kemarin ditolak, sampah ditolak", () => {
    assert.equal(galatTanggalBerlaku("", sekarang), undefined);
    assert.equal(galatTanggalBerlaku("2026-06-10", sekarang), undefined);
    assert.equal(galatTanggalBerlaku("2026-07-01", sekarang), undefined);
    assert.match(
      galatTanggalBerlaku("2026-06-09", sekarang) ?? "",
      /masa depan/,
    );
    assert.match(
      galatTanggalBerlaku("2026-02-30", sekarang) ?? "",
      /tidak valid/,
    );
  });

  it("hariIni memformat tanggal lokal dengan nol di depan", () => {
    assert.equal(hariIni(new Date(2026, 0, 5)), "2026-01-05");
    assert.equal(hariIni(sekarang), "2026-06-10");
  });
});
