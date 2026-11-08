import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";
import { PERMISSIONS, ROLE_PERMISSIONS } from "@/lib/rbac/permissions";
import { ROLE_IDS } from "@/lib/rbac/roles";
import { INFO_IZIN, KELOMPOK_LAINNYA, kelompokkanIzin } from "./izin";

const contract = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../../packages/contracts/permissions.json"),
    "utf8",
  ),
) as { permissions: string[] };

describe("INFO_IZIN", () => {
  it("memuat tepat izin yang ada di kontrak API (katalog tidak melenceng)", () => {
    assert.deepEqual(
      Object.keys(INFO_IZIN).sort(),
      [...contract.permissions].sort(),
    );
    assert.deepEqual([...PERMISSIONS].sort(), Object.keys(INFO_IZIN).sort());
  });

  it("setiap izin punya label dan kelompok tak kosong", () => {
    for (const [id, info] of Object.entries(INFO_IZIN)) {
      assert.ok(info.label.trim(), `label ${id}`);
      assert.ok(info.kelompok.trim(), `kelompok ${id}`);
    }
  });
});

describe("kelompokkanIzin", () => {
  it("mengelompokkan menurut katalog dan mempertahankan urutannya", () => {
    const hasil = kelompokkanIzin([
      "audit.lihat",
      "harga.input",
      "harga.lihat",
      "dashboard.operasional",
    ]);
    assert.deepEqual(
      hasil.map((k) => [k.nama, k.izin.map((i) => i.id)]),
      [
        ["Dasbor", ["dashboard.operasional"]],
        ["Harga", ["harga.lihat", "harga.input"]],
        ["Pengguna dan audit", ["audit.lihat"]],
      ],
    );
  });

  it("item membawa label dari katalog", () => {
    const [kelompok] = kelompokkanIzin(["harga.lihat"]);
    assert.deepEqual(kelompok.izin, [
      { id: "harga.lihat", label: "Melihat data harga pangan" },
    ]);
  });

  it("urutan masukan tidak memengaruhi hasil dan izin ganda dibuang", () => {
    const a = kelompokkanIzin(["harga.lihat", "audit.lihat", "harga.lihat"]);
    const b = kelompokkanIzin(["audit.lihat", "harga.lihat"]);
    assert.deepEqual(a, b);
    assert.equal(a.flatMap((k) => k.izin).length, 2);
  });

  it("izin di luar katalog masuk 'Lainnya' di akhir dengan kodenya sebagai label", () => {
    const hasil = kelompokkanIzin(["izin.baru", "audit.lihat", "izin.lain"]);
    assert.deepEqual(hasil.at(-1), {
      nama: KELOMPOK_LAINNYA,
      izin: [
        { id: "izin.baru", label: "izin.baru" },
        { id: "izin.lain", label: "izin.lain" },
      ],
    });
    assert.equal(hasil[0].nama, "Pengguna dan audit");
  });

  it("nama bawaan Object seperti 'constructor' diperlakukan sebagai izin tak dikenal", () => {
    const hasil = kelompokkanIzin(["constructor", "toString"]);
    assert.deepEqual(hasil, [
      {
        nama: KELOMPOK_LAINNYA,
        izin: [
          { id: "constructor", label: "constructor" },
          { id: "toString", label: "toString" },
        ],
      },
    ]);
  });

  it("daftar kosong menghasilkan tanpa kelompok", () => {
    assert.deepEqual(kelompokkanIzin([]), []);
  });

  it("seluruh izin bawaan setiap peran terkelompok tanpa sisa", () => {
    for (const role of ROLE_IDS) {
      const hasil = kelompokkanIzin(ROLE_PERMISSIONS[role]);
      assert.equal(
        hasil.flatMap((k) => k.izin).length,
        ROLE_PERMISSIONS[role].length,
        role,
      );
      assert.equal(
        hasil.some((k) => k.nama === KELOMPOK_LAINNYA),
        false,
        role,
      );
    }
  });
});
