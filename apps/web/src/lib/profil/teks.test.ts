import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { SPASI_GO, pangkas } from "./teks";

/** Himpunan White_Space menurut unicode.IsSpace di Go. */
const SPASI = [
  "\t",
  "\n",
  "\v",
  "\f",
  "\r",
  " ",
  "\u0085",
  " ",
  " ",
  ...Array.from({ length: 11 }, (_, i) => String.fromCharCode(0x2000 + i)),
  " ",
  " ",
  " ",
  " ",
  "　",
];

describe("pangkas", () => {
  it("membuang setiap karakter spasi Go di awal dan akhir", () => {
    for (const s of SPASI) {
      assert.equal(
        pangkas(`${s}${s}Budi${s}`),
        "Budi",
        `U+${s.charCodeAt(0).toString(16).padStart(4, "0")}`,
      );
    }
  });

  it("spasi di tengah tidak disentuh", () => {
    assert.equal(pangkas("  Budi  Santoso \t"), "Budi  Santoso");
  });

  it("U+FEFF (BOM) bukan spasi menurut Go, jadi tetap ada, berbeda dari trim() JavaScript", () => {
    assert.equal(pangkas("﻿Budi﻿"), "﻿Budi﻿");
    assert.notEqual("﻿Budi".trim(), "﻿Budi");
  });

  it("teks tanpa spasi dan teks kosong tidak berubah", () => {
    assert.equal(pangkas("Budi"), "Budi");
    assert.equal(pangkas(""), "");
    assert.equal(pangkas(" \t "), "");
  });

  it("SPASI_GO memuat tepat himpunan di atas", () => {
    const kelas = new RegExp(`^[${SPASI_GO}]$`);
    for (const s of SPASI)
      assert.ok(kelas.test(s), s.charCodeAt(0).toString(16));
    for (const bukan of ["a", "0", "-", "​", "᠎", "﻿", " x"]) {
      assert.equal(kelas.test(bukan), false, bukan);
    }
  });
});
