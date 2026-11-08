import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  PESAN_TELEPON_TIDAK_VALID,
  formatTelepon,
  normalisasiTelepon,
} from "./telepon";

const ok = (nilai: string | null) => ({ ok: true, nilai });
const tidakValid = { ok: false, pesan: "Nomor telepon tidak valid" };

describe("normalisasiTelepon: awalan", () => {
  it("08…, 62…, dan +62… menjadi E.164 yang sama", () => {
    assert.deepEqual(normalisasiTelepon("081234567890"), ok("+6281234567890"));
    assert.deepEqual(normalisasiTelepon("6281234567890"), ok("+6281234567890"));
    assert.deepEqual(
      normalisasiTelepon("+6281234567890"),
      ok("+6281234567890"),
    );
  });

  it("awalan lain tidak valid: tanpa awalan, kode negara lain, awalan internasional 00, plus ganda", () => {
    for (const input of [
      "81234567890",
      "+81234567890",
      "+1 650 253 0000",
      "1234567890",
      "+",
      "++6281234567890",
      "00 62 812 3456 7890",
    ]) {
      assert.deepEqual(normalisasiTelepon(input), tidakValid, input);
    }
  });

  it("bagian nasional tidak boleh diawali nol: +620…, 620…", () => {
    assert.deepEqual(normalisasiTelepon("+62081234567890"), tidakValid);
    assert.deepEqual(normalisasiTelepon("6208123456789"), tidakValid);
  });
});

describe("normalisasiTelepon: pemisah", () => {
  it("spasi, tanda hubung, titik, dan kurung dibuang", () => {
    for (const input of [
      "0812-3456-7890",
      "0812 3456 7890",
      "0812.3456.7890",
      "(0812) 3456-7890",
      "+62 (812) 3456 7890",
      "+62-812-3456-7890",
      "  0812 3456 7890  ",
      "0812 - 3456 - 7890",
    ]) {
      assert.deepEqual(normalisasiTelepon(input), ok("+6281234567890"), input);
    }
  });

  it("spasi Unicode dari hasil salin (NBSP, spasi sempit) dan tab juga dibuang", () => {
    assert.deepEqual(
      normalisasiTelepon("+62 812 3456 7890"),
      ok("+6281234567890"),
    );
    assert.deepEqual(
      normalisasiTelepon("0812\t3456\n7890"),
      ok("+6281234567890"),
    );
  });

  it("himpunan spasi sama dengan unicode.IsSpace di Go: U+0085 dibuang, U+FEFF (BOM) tidak", () => {
    assert.deepEqual(
      normalisasiTelepon("\u00850812\u00853456\u00857890\u0085"),
      ok("+6281234567890"),
    );
    assert.deepEqual(normalisasiTelepon("\ufeff081234567890"), tidakValid);
    assert.deepEqual(normalisasiTelepon("0812\ufeff34567890"), tidakValid);
  });

  it("pemisah lain (garis miring, plus di tengah, garis bawah) bukan pemisah", () => {
    assert.deepEqual(normalisasiTelepon("0812/3456/7890"), tidakValid);
    assert.deepEqual(normalisasiTelepon("0812+3456+7890"), tidakValid);
    assert.deepEqual(normalisasiTelepon("0812_3456_7890"), tidakValid);
  });
});

describe("normalisasiTelepon: panjang", () => {
  it("bagian nasional 8 sampai 13 digit", () => {
    assert.deepEqual(normalisasiTelepon("+621234567"), tidakValid, "7 digit");
    assert.deepEqual(
      normalisasiTelepon("+6212345678"),
      ok("+6212345678"),
      "8 digit (batas bawah)",
    );
    assert.deepEqual(
      normalisasiTelepon("+62812345678"),
      ok("+62812345678"),
      "9 digit",
    );
    assert.deepEqual(
      normalisasiTelepon("+621234567890123"),
      ok("+621234567890123"),
      "13 digit (batas atas)",
    );
    assert.deepEqual(
      normalisasiTelepon("+6212345678901234"),
      tidakValid,
      "14 digit",
    );
  });

  it("total digit E.164 10 sampai 15", () => {
    const nomor = (nasional: number) => `+62${"8".repeat(nasional)}`;
    const totalDigit = (s: string) => s.replace("+", "").length;
    assert.equal(totalDigit(nomor(8)), 10);
    assert.equal(totalDigit(nomor(13)), 15);
    assert.equal(normalisasiTelepon(nomor(8)).ok, true);
    assert.equal(normalisasiTelepon(nomor(13)).ok, true);
    assert.equal(normalisasiTelepon(nomor(7)).ok, false);
    assert.equal(normalisasiTelepon(nomor(14)).ok, false);
  });
});

describe("normalisasiTelepon: bukan angka", () => {
  it("huruf, simbol, dan angka non-ASCII ditolak", () => {
    for (const input of [
      "0812abc7890",
      "abcdefghij",
      "0812-3456-78x0",
      "0812 3456 7890 ext",
      "０８１２３４５６７８９０",
      "٠٨١٢٣٤٥٦٧٨٩٠",
      "0812-3456-789O",
      "@@@",
    ]) {
      assert.deepEqual(normalisasiTelepon(input), tidakValid, input);
    }
  });

  it("pemisah tanpa angka tidak dianggap kosong", () => {
    for (const input of ["---", "()", " . ", "( )"]) {
      assert.deepEqual(normalisasiTelepon(input), tidakValid, input);
    }
  });
});

describe("normalisasiTelepon: kosong berarti hapus nomor", () => {
  it("string kosong, spasi, null, dan undefined menghasilkan null", () => {
    for (const input of ["", "   ", "\t\n", null, undefined]) {
      assert.deepEqual(normalisasiTelepon(input), ok(null), String(input));
    }
  });
});

describe("normalisasiTelepon: pesan dan idempotensi", () => {
  it("pesan galat sama dengan pesan API", () => {
    assert.equal(PESAN_TELEPON_TIDAK_VALID, "Nomor telepon tidak valid");
    assert.deepEqual(normalisasiTelepon("123"), tidakValid);
  });

  it("hasil normalisasi diterima lagi tanpa berubah", () => {
    const sekali = normalisasiTelepon("(0812) 3456-7890");
    assert.ok(sekali.ok && sekali.nilai);
    assert.deepEqual(normalisasiTelepon(sekali.nilai), sekali);
  });
});

describe("formatTelepon", () => {
  it("memberi spasi setelah +62 dan tanda hubung antarkelompok", () => {
    assert.equal(formatTelepon("+6281234567890"), "+62 812-3456-7890");
    assert.equal(formatTelepon("+628111234567"), "+62 811-1234-567");
  });

  it("nomor pendek dan panjang tetap terbagi tiga kelompok", () => {
    assert.equal(formatTelepon("+6212345678"), "+62 123-4567-8");
    assert.equal(formatTelepon("+62812345678901"), "+62 812-3456-78901");
  });

  it("bentuk tampilan dibaca kembali menjadi nilai semula", () => {
    for (const nilai of [
      "+6281234567890",
      "+628111234567",
      "+6212345678",
      "+621234567890123",
    ]) {
      assert.deepEqual(
        normalisasiTelepon(formatTelepon(nilai)),
        ok(nilai),
        nilai,
      );
    }
  });

  it("nilai yang bukan nomor Indonesia sah dikembalikan apa adanya", () => {
    assert.equal(formatTelepon("+1 650 253 0000"), "+1 650 253 0000");
    assert.equal(formatTelepon("abc"), "abc");
    assert.equal(formatTelepon(""), "");
  });
});

describe("paritas dengan TestNormalisasiTelepon di API Go (usecase/profil_test.go)", () => {
  // [masukan, hasil E.164 | null untuk hapus nomor | "galat"]
  const kasus: [string, string | null | "galat"][] = [
    ["+6281234567890", "+6281234567890"],
    ["081234567890", "+6281234567890"],
    ["6281234567890", "+6281234567890"],
    ["0812-3456-7890", "+6281234567890"],
    ["0812.3456.7890", "+6281234567890"],
    ["0812 3456 7890", "+6281234567890"],
    ["(0812) 3456 7890", "+6281234567890"],
    ["+62 812-3456-7890", "+6281234567890"],
    ["+62(812)3456-7890", "+6281234567890"],
    ["62 812 3456 7890", "+6281234567890"],
    ["  0812 3456 7890\t\n", "+6281234567890"],
    ["0812\u00a03456\u00a07890", "+6281234567890"],
    ["(0651) 123-456", "+62651123456"],
    ["012345678", "+6212345678"],
    ["01234567890123", "+621234567890123"],
    ["", null],
    ["   ", null],
    ["\t\n ", null],
    ["+1 415 555 2671", "galat"],
    ["+44 20 7946 0958", "galat"],
    ["81234567890", "galat"],
    ["8123456789", "galat"],
    ["+6208123456789", "galat"],
    ["6208123456789", "galat"],
    ["0062812345678", "galat"],
    ["0", "galat"],
    ["+62", "galat"],
    ["62", "galat"],
    ["01234567", "galat"],
    ["0812345", "galat"],
    ["012345678901234", "galat"],
    ["+6281234567890123", "galat"],
    ["0812abc34567", "galat"],
    ["08123456789a", "galat"],
    ["-", "galat"],
    ["()", "galat"],
    ["0812+3456789", "galat"],
    ["0812/3456789", "galat"],
    ["０８１２３４５６７８９０", "galat"],
    ["+0812345678", "galat"],
    ["0812345\u0000678", "galat"],
  ];

  for (const [masukan, harapan] of kasus) {
    it(JSON.stringify(masukan), () => {
      assert.deepEqual(
        normalisasiTelepon(masukan),
        harapan === "galat"
          ? { ok: false, pesan: "Nomor telepon tidak valid" }
          : { ok: true, nilai: harapan },
      );
    });
  }

  it("menormalkan ulang hasil normalisasi tidak mengubahnya (idempoten)", () => {
    for (const masukan of ["0812-3456-7890", "62 812 3456 7890", "(0651) 123-456", "+62 81234567890"]) {
      const sekali = normalisasiTelepon(masukan);
      assert.ok(sekali.ok && sekali.nilai, masukan);
      assert.deepEqual(normalisasiTelepon(sekali.nilai), sekali, masukan);
    }
  });
});

