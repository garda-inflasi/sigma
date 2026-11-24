import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { EwsAlert, StatusEws } from "@/lib/data/types";
import type { Severity } from "@/lib/design/tokens";
import {
  MAKS_TOAST_KRITIS,
  isiFeed,
  kritisAktif,
  kritisBelumDitindaklanjuti,
  kritisDulu,
  pesanTutupMassal,
  pilihToastKritis,
  terapkanPeristiwaKritis,
} from "./kritis";

const alert = (id: string, severity: Severity, status: StatusEws): EwsAlert => ({
  id,
  severity,
  status,
  jenis: "LONJAKAN_HARGA",
  judul: `Alert ${id}`,
  kabupatenKode: "1171",
  wilayah: "Kota Banda Aceh",
  nilai: 20,
  satuanNilai: "%",
  pesan: "",
  waktu: "2026-11-24T00:00:00Z",
  skorDampak: 90,
  sumber: "uji",
});

describe("status peringatan KRITIS", () => {
  it("hanya BARU yang belum ditindaklanjuti; semua selain SELESAI masih aktif", () => {
    assert.equal(kritisBelumDitindaklanjuti(alert("1", "KRITIS", "BARU")), true);
    for (const s of ["DIAKUI", "DIESKALASI", "SELESAI"] as StatusEws[]) {
      assert.equal(kritisBelumDitindaklanjuti(alert("1", "KRITIS", s)), false, s);
    }
    assert.equal(kritisBelumDitindaklanjuti(alert("1", "SIAGA", "BARU")), false);
    assert.equal(kritisAktif(alert("1", "KRITIS", "DIAKUI")), true);
    assert.equal(kritisAktif(alert("1", "KRITIS", "SELESAI")), false);
    assert.equal(kritisAktif(alert("1", "SIAGA", "BARU")), false);
  });

  it("kritisDulu menaruh KRITIS aktif di depan dan menjaga urutan lainnya", () => {
    const hasil = kritisDulu([
      alert("a", "SIAGA", "BARU"),
      alert("b", "KRITIS", "SELESAI"),
      alert("c", "KRITIS", "DIAKUI"),
      alert("d", "INFO", "BARU"),
      alert("e", "KRITIS", "BARU"),
    ]);
    assert.deepEqual(hasil.map((a) => a.id), ["c", "e", "a", "b", "d"]);
  });
});

describe("isiFeed", () => {
  it("KRITIS aktif tidak terpotong batas, walau melebihinya", () => {
    const kritis = Array.from({ length: 40 }, (_, i) => alert(`k${i}`, "KRITIS", "BARU"));
    const lain = [alert("s1", "SIAGA", "BARU")];
    const hasil = isiFeed([...lain, ...kritis], [], 35);
    assert.equal(hasil.length, 40);
    assert.ok(hasil.every((a) => a.severity === "KRITIS"));
  });

  it("KRITIS yang sudah diakui tetap tampil; alert biasa yang diakui tidak", () => {
    const hasil = isiFeed(
      [alert("s", "SIAGA", "DIAKUI"), alert("k", "KRITIS", "DIAKUI"), alert("x", "KRITIS", "SELESAI")],
      [],
      10,
    );
    assert.deepEqual(hasil.map((a) => a.id), ["k"]);
  });

  it("alert lain mengisi sisa batas setelah KRITIS", () => {
    const hasil = isiFeed(
      [alert("a", "SIAGA", "BARU"), alert("b", "WASPADA", "BARU"), alert("k", "KRITIS", "BARU")],
      [],
      2,
    );
    assert.deepEqual(hasil.map((a) => a.id), ["k", "a"]);
  });

  it("alert yang baru masuk tetap tampil sebentar walau sudah diakui", () => {
    const hasil = isiFeed([alert("a", "SIAGA", "DIAKUI")], ["a"], 5);
    assert.deepEqual(hasil.map((a) => a.id), ["a"]);
  });
});

describe("pilihToastKritis", () => {
  const daftar = ["1", "2", "3", "4", "5"].map((id) => ({ id }));

  it("puluhan KRITIS dibatasi jumlah toast", () => {
    assert.deepEqual(pilihToastKritis(daftar, []), ["1", "2", "3"]);
    assert.equal(MAKS_TOAST_KRITIS, 3);
  });

  it("yang dikecilkan digantikan alert berikutnya", () => {
    assert.deepEqual(pilihToastKritis(daftar, ["1", "3"]), ["2", "4", "5"]);
    assert.deepEqual(pilihToastKritis(daftar, ["1", "2", "3", "4", "5"]), []);
  });
});

describe("terapkanPeristiwaKritis", () => {
  it("daftar belum dimuat tetap belum dimuat", () => {
    assert.equal(terapkanPeristiwaKritis(undefined, alert("1", "KRITIS", "BARU")), undefined);
  });

  it("KRITIS BARU masuk ke depan, dan yang sudah ada diganti di tempatnya", () => {
    const awal = [alert("1", "KRITIS", "BARU")];
    const tambah = terapkanPeristiwaKritis(awal, alert("2", "KRITIS", "BARU"))!;
    assert.deepEqual(tambah.map((a) => a.id), ["2", "1"]);
    const ganti = terapkanPeristiwaKritis(tambah, { ...alert("1", "KRITIS", "BARU"), nilai: 33 })!;
    assert.deepEqual(ganti.map((a) => a.id), ["2", "1"]);
    assert.equal(ganti[1].nilai, 33);
  });

  it("tindak lanjut dari tab atau pengguna lain mengeluarkannya", () => {
    const awal = [alert("1", "KRITIS", "BARU"), alert("2", "KRITIS", "BARU")];
    for (const s of ["DIAKUI", "DIESKALASI", "SELESAI"] as StatusEws[]) {
      const hasil = terapkanPeristiwaKritis(awal, alert("1", "KRITIS", s))!;
      assert.deepEqual(hasil.map((a) => a.id), ["2"], s);
    }
  });

  it("peristiwa alert lain tidak mengubah daftar", () => {
    const awal = [alert("1", "KRITIS", "BARU")];
    assert.equal(terapkanPeristiwaKritis(awal, alert("9", "SIAGA", "BARU")), awal);
    assert.equal(terapkanPeristiwaKritis(awal, alert("9", "KRITIS", "SELESAI")), awal);
  });
});

describe("pesanTutupMassal", () => {
  it("menyebut jumlah KRITIS terdampak", () => {
    const pesan = pesanTutupMassal(3, [
      alert("1", "KRITIS", "BARU"),
      alert("2", "KRITIS", "DIAKUI"),
      alert("3", "SIAGA", "BARU"),
    ]);
    assert.match(pesan, /^3 alert/);
    assert.match(pesan, /2 peringatan KRITIS/);
  });

  it("tanpa KRITIS tidak menyebutnya", () => {
    assert.doesNotMatch(pesanTutupMassal(1, [alert("1", "SIAGA", "BARU")]), /KRITIS/);
  });
});
