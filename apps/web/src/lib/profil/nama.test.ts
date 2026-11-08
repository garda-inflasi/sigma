import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  NAMA_MAKS,
  PESAN_NAMA_PANJANG,
  PESAN_NAMA_PENDEK,
  validasiNama,
} from "./nama";

describe("validasiNama", () => {
  it("pesan sama dengan pesan API", () => {
    assert.equal(PESAN_NAMA_PENDEK, "Nama minimal 3 karakter");
    assert.equal(PESAN_NAMA_PANJANG, "Nama maksimal 100 karakter");
  });

  it("nama sah dikembalikan setelah dipangkas", () => {
    assert.deepEqual(validasiNama("  Budi Santoso  "), {
      ok: true,
      nilai: "Budi Santoso",
    });
    assert.deepEqual(validasiNama("Ani"), { ok: true, nilai: "Ani" });
  });

  it("kurang dari 3 karakter ditolak, termasuk kosong dan hanya spasi", () => {
    for (const input of ["", "  ", "Bu", " Bu "]) {
      assert.deepEqual(
        validasiNama(input),
        { ok: false, pesan: PESAN_NAMA_PENDEK },
        JSON.stringify(input),
      );
    }
    // Spasi di tengah ikut terhitung: "A B" tepat 3 karakter.
    assert.equal(validasiNama("A B").ok, true);
  });

  it("batas atas 100 karakter: 100 sah, 101 ditolak", () => {
    assert.equal(validasiNama("a".repeat(NAMA_MAKS)).ok, true);
    assert.deepEqual(validasiNama("a".repeat(NAMA_MAKS + 1)), {
      ok: false,
      pesan: PESAN_NAMA_PANJANG,
    });
  });

  it("spasi di tepi tidak dihitung terhadap batas", () => {
    assert.equal(validasiNama(`  ${"a".repeat(NAMA_MAKS)}  `).ok, true);
    assert.equal(validasiNama("  Bu  ").ok, false);
  });

  it("panjang dihitung dalam karakter Unicode, bukan unit UTF-16", () => {
    // 100 emoji = 200 unit UTF-16 tetapi 100 karakter.
    assert.equal(validasiNama("😀".repeat(NAMA_MAKS)).ok, true);
    assert.equal(validasiNama("😀".repeat(NAMA_MAKS + 1)).ok, false);
    // 2 emoji = 4 unit UTF-16 tetapi hanya 2 karakter.
    assert.equal(validasiNama("😀😀").ok, false);
    assert.equal(validasiNama("😀😀😀").ok, true);
  });

  it("NUL dibuang sebelum dihitung, seperti di API Go", () => {
    assert.deepEqual(validasiNama("Bu\u0000di"), { ok: true, nilai: "Budi" });
    assert.deepEqual(validasiNama("Bu\u0000"), {
      ok: false,
      pesan: PESAN_NAMA_PENDEK,
    });
  });

  it("himpunan spasi sama dengan unicode.IsSpace di Go: U+0085 dipangkas, U+FEFF (BOM) tidak", () => {
    assert.deepEqual(validasiNama("\u0085Budi\u0085"), { ok: true, nilai: "Budi" });
    assert.deepEqual(validasiNama("\ufeffBu"), { ok: true, nilai: "\ufeffBu" });
  });

  it("gelar dan tanda baca pada nama tidak diubah", () => {
    const nama = "Prof. Dr. Ilham Maulana, S.Si., M.Sc.";
    assert.deepEqual(validasiNama(nama), { ok: true, nilai: nama });
  });
});

describe("paritas dengan TestNormalisasiNama di API Go (usecase/profil_test.go)", () => {
  // [masukan, nilai | null untuk galat pendek | "panjang" untuk galat panjang]
  const kasus: [string, string | "pendek" | "panjang"][] = [
    ["Budi Santoso", "Budi Santoso"],
    ["  Budi Santoso \t", "Budi Santoso"],
    ["Ani", "Ani"],
    ["日本語", "日本語"],
    [" a b ", "a b"],
    ["a".repeat(100), "a".repeat(100)],
    ["é".repeat(100), "é".repeat(100)],
    ["😀".repeat(100), "😀".repeat(100)],
    ["Bu\u0000di Santoso", "Budi Santoso"],
    ["", "pendek"],
    ["      ", "pendek"],
    ["\t\n\r", "pendek"],
    ["Al", "pendek"],
    ["   Al   ", "pendek"],
    ["日本", "pendek"],
    ["\u0000\u0000\u0000", "pendek"],
    ["a".repeat(101), "panjang"],
    ["é".repeat(101), "panjang"],
    ["  " + "a".repeat(101) + "  ", "panjang"],
  ];

  for (const [masukan, harapan] of kasus) {
    it(JSON.stringify(masukan.length > 30 ? `${masukan.slice(0, 12)}… (${[...masukan].length} karakter)` : masukan), () => {
      assert.deepEqual(
        validasiNama(masukan),
        harapan === "pendek"
          ? { ok: false, pesan: "Nama minimal 3 karakter" }
          : harapan === "panjang"
            ? { ok: false, pesan: "Nama maksimal 100 karakter" }
            : { ok: true, nilai: harapan },
      );
    });
  }
});

