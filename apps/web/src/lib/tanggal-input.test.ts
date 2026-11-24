import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  bacaKetikanTanggal,
  bacaKetikanTanggalWaktu,
  bagianWaktu,
  dateDariIso,
  galatRentangTanggal,
  isoDariDate,
  rapikanKetikanTanggal,
  tampilTanggal,
  tampilTanggalWaktu,
} from "./tanggal-input";

describe("tanggal-input", () => {
  it("menjaga tanggal setempat tanpa bergeser zona waktu", () => {
    const d = dateDariIso("2026-01-01");
    assert.ok(d);
    assert.equal(d.getDate(), 1);
    assert.equal(isoDariDate(d), "2026-01-01");
  });

  it("menampilkan nilai form sebagai hh/bb/tttt", () => {
    assert.equal(tampilTanggal("2026-11-24"), "24/11/2026");
    assert.equal(tampilTanggal(""), "");
    assert.equal(
      tampilTanggalWaktu("2026-11-24T07:05:09"),
      "24/11/2026 07:05:09",
    );
    assert.equal(bagianWaktu("2026-11-24T07:05"), "07:05:00");
  });

  it("membaca ketikan lokal dan ISO", () => {
    for (const teks of ["24/11/2026", "24-11-2026", "24.11.2026", "2026-11-24"])
      assert.equal(bacaKetikanTanggal(teks), "2026-11-24");
    assert.equal(bacaKetikanTanggal("1/2/2026"), "2026-02-01");
  });

  it("menolak tanggal yang tidak ada atau belum lengkap", () => {
    for (const teks of [
      "31/02/2026",
      "29/02/2026",
      "24/13/2026",
      "24/11/26",
      "24/11",
      "abc",
      "01/01/0026",
    ])
      assert.equal(bacaKetikanTanggal(teks), null, teks);
    assert.equal(bacaKetikanTanggal("29/02/2028"), "2028-02-29");
  });

  it("membaca tanggal dan jam", () => {
    assert.equal(
      bacaKetikanTanggalWaktu("2026-11-24T07:05:09"),
      "2026-11-24T07:05:09",
    );
    assert.equal(
      bacaKetikanTanggalWaktu("24/11/2026 7.05"),
      "2026-11-24T07:05:00",
    );
    for (const teks of [
      "24/11/2026",
      "24/11/2026 24:00",
      "24/11/2026 10:60",
      "31/02/2026 10:00",
    ])
      assert.equal(bacaKetikanTanggalWaktu(teks), null, teks);
  });

  it("menyisipkan garis miring hanya saat mengetik maju", () => {
    assert.equal(rapikanKetikanTanggal("24", "2"), "24/");
    assert.equal(rapikanKetikanTanggal("24/11", "24/1"), "24/11/");
    assert.equal(rapikanKetikanTanggal("24112026", ""), "24/11/2026");
    assert.equal(rapikanKetikanTanggal("24", "24/"), "24");
    assert.equal(rapikanKetikanTanggal("2030-01-01", ""), "2030-01-01");
  });

  it("tetap menerima ketikan per huruf dengan pemisah apa pun", () => {
    const ketik = (teks: string) =>
      [...teks].reduce((isi, h) => rapikanKetikanTanggal(isi + h, isi), "");
    assert.equal(ketik("2026-11-01"), "2026-11-01");
    assert.equal(ketik("24-11-2026"), "24/11/2026");
    assert.equal(ketik("24.11.2026"), "24/11/2026");
    assert.equal(ketik("24/11/2026"), "24/11/2026");
    assert.equal(ketik("1/2/2026"), "1/2/2026");
  });

  it("memeriksa batas rentang secara inklusif", () => {
    const batas = { min: "2026-01-10", max: "2026-01-20" };
    assert.equal(galatRentangTanggal("2026-01-10", batas), null);
    assert.equal(galatRentangTanggal("2026-01-20", batas), null);
    assert.match(
      galatRentangTanggal("2026-01-09", batas) ?? "",
      /10\/01\/2026/,
    );
    assert.match(
      galatRentangTanggal("2026-01-21", batas) ?? "",
      /20\/01\/2026/,
    );
  });
});
