import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { z } from "zod";
import {
  adalahGalatValidasi,
  galatFieldDariIsu,
  galatFieldDariPayload,
  galatFormBawaan,
  jalankanKiriman,
  navigasiAman,
  PESAN_GALAT_SERVER,
} from "./kiriman";

class ApiErrorUji extends Error {
  constructor(public status: number, message: string, public payload?: unknown) {
    super(message);
  }
}

const skema = z.object({
  email: z.string().trim().min(1, "Email wajib diisi."),
  sandi: z.string().min(8, "Minimal 8 karakter."),
});
type Nilai = z.infer<typeof skema>;

describe("jalankanKiriman", () => {
  it("galat skema dipetakan per field dan server tidak dipanggil", async () => {
    let dipanggil = 0;
    const hasil = await jalankanKiriman<Nilai, unknown, { pesan: string }>({
      nilai: { email: " ", sandi: "pendek" },
      skema,
      kirim: async () => void dipanggil++,
      petaGalat: galatFormBawaan,
    });
    assert.equal(dipanggil, 0);
    assert.deepEqual(hasil, {
      jenis: "tidak-valid",
      galatField: { email: "Email wajib diisi.", sandi: "Minimal 8 karakter." },
      pesanForm: undefined,
    });
  });

  it("nilai hasil parse skema (mis. trim) yang dikirim", async () => {
    let terkirim: Nilai | undefined;
    await jalankanKiriman<Nilai, unknown, { pesan: string }>({
      nilai: { email: "  a@b.id ", sandi: "rahasia123" },
      skema,
      kirim: async (n) => void (terkirim = n),
      petaGalat: galatFormBawaan,
    });
    assert.deepEqual(terkirim, { email: "a@b.id", sandi: "rahasia123" });
  });

  it("2xx: navigasi dari `sukses` disaring menjadi path internal", async () => {
    const hasil = await jalankanKiriman({
      nilai: {},
      kirim: async () => ({ tujuan: "/operasional" }),
      sukses: (h) => ({ ke: h.tujuan, segarkan: true }),
      petaGalat: galatFormBawaan,
    });
    assert.deepEqual(hasil, {
      jenis: "berhasil",
      hasil: { tujuan: "/operasional" },
      navigasi: { ke: "/operasional", mode: "replace", segarkan: true },
    });
  });

  it("tujuan dari server yang bukan path internal tidak pernah dipakai", async () => {
    for (const jahat of ["https://evil.example", "//evil.example", "javascript:alert(1)", "/\\evil.example"]) {
      const hasil = await jalankanKiriman({ nilai: {}, kirim: async () => jahat, sukses: (t) => t, petaGalat: galatFormBawaan });
      assert.equal(hasil.jenis === "berhasil" && hasil.navigasi?.ke, "/", jahat);
    }
  });

  it("2xx tanpa navigasi", async () => {
    const hasil = await jalankanKiriman({ nilai: {}, kirim: async () => 1, petaGalat: galatFormBawaan });
    assert.deepEqual(hasil, { jenis: "berhasil", hasil: 1, navigasi: null });
  });

  it("422 dengan galat per field: field dan pesan form keduanya terisi, tidak melempar", async () => {
    const err = new ApiErrorUji(422, "Validasi gagal.", { errors: { email: ["Email sudah dipakai."] } });
    const hasil = await jalankanKiriman<Nilai, unknown, { pesan: string }>({
      nilai: { email: "a@b.id", sandi: "rahasia123" },
      kirim: async () => {
        throw err;
      },
      petaGalat: galatFormBawaan,
    });
    assert.equal(hasil.jenis, "gagal");
    if (hasil.jenis !== "gagal") return;
    assert.deepEqual(hasil.galatField, { email: "Email sudah dipakai." });
    assert.deepEqual(hasil.galat, { pesan: "Validasi gagal." });
    assert.equal(hasil.error, err);
  });

  it("galat selain 400/422 tidak menghasilkan galat field", async () => {
    const hasil = await jalankanKiriman({
      nilai: {},
      kirim: async () => {
        throw new ApiErrorUji(403, "Akses ditolak", { errors: { x: "y" } });
      },
      petaGalat: galatFormBawaan,
    });
    assert.ok(hasil.jenis === "gagal");
    if (hasil.jenis === "gagal") assert.deepEqual(hasil.galatField, {});
  });

  it("`sukses` yang melempar menjadi hasil gagal, bukan exception", async () => {
    const hasil = await jalankanKiriman({
      nilai: {},
      kirim: async () => 1,
      sukses: () => {
        throw new Error("rusak");
      },
      petaGalat: () => "galat",
    });
    assert.equal(hasil.jenis, "gagal");
  });
});

describe("pembantu galat form", () => {
  it("400 dan 422 adalah galat validasi", () => {
    assert.equal(adalahGalatValidasi(400), true);
    assert.equal(adalahGalatValidasi(422), true);
    assert.equal(adalahGalatValidasi(401), false);
    assert.equal(adalahGalatValidasi(undefined), false);
  });

  it("isu skema: pesan pertama per field tingkat atas", () => {
    assert.deepEqual(
      galatFieldDariIsu([
        { path: ["email"], message: "pertama" },
        { path: ["email"], message: "kedua" },
        { path: ["alamat", "kota"], message: "bersarang" },
        { path: [], message: "tanpa field" },
      ]),
      { email: "pertama", alamat: "bersarang" },
    );
  });

  it("payload galat field: errors, fields, dan amplop data API Go", () => {
    assert.deepEqual(galatFieldDariPayload({ errors: { a: "x", b: ["y", "z"], c: 3, d: "" } }), { a: "x", b: "y" });
    assert.deepEqual(galatFieldDariPayload({ fields: { a: "x" } }), { a: "x" });
    assert.deepEqual(galatFieldDariPayload({ code: 422, data: { errors: { a: "x" } } }), { a: "x" });
    assert.deepEqual(galatFieldDariPayload({ errors: ["bukan", "peta"] }), {});
    assert.deepEqual(galatFieldDariPayload("teks"), {});
    assert.deepEqual(galatFieldDariPayload(undefined), {});
  });

  it("pesan bawaan: pesan server untuk 4xx, pesan umum untuk 5xx dan jaringan", () => {
    assert.deepEqual(galatFormBawaan(new ApiErrorUji(400, "Kode tidak valid")), { pesan: "Kode tidak valid" });
    assert.deepEqual(galatFormBawaan(new ApiErrorUji(500, "pq: duplicate key")), { pesan: PESAN_GALAT_SERVER });
    assert.deepEqual(galatFormBawaan(new TypeError("Failed to fetch")), { pesan: PESAN_GALAT_SERVER });
    assert.deepEqual(galatFormBawaan("aneh"), { pesan: PESAN_GALAT_SERVER });
  });

  it("navigasiAman: string, objek, dan kosong", () => {
    assert.deepEqual(navigasiAman("/a?b=1"), { ke: "/a?b=1", mode: "replace", segarkan: false });
    assert.deepEqual(navigasiAman({ ke: "/b", mode: "push" }), { ke: "/b", mode: "push", segarkan: false });
    assert.equal(navigasiAman(undefined), null);
    assert.equal(navigasiAman(""), null);
  });
});
