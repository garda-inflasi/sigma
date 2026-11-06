import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { galatFieldDariIsu } from "@/lib/form/kiriman";
import {
  PESAN_KONFIRMASI_BEDA,
  PESAN_SANDI_LAMA_KOSONG,
  PESAN_SANDI_LEMAH,
  skemaGantiSandi,
  skemaSandiBaru,
} from "./skema-sandi";

const galat = (hasil: { success: boolean; error?: { issues: { path: PropertyKey[]; message: string }[] } }) =>
  hasil.success ? {} : galatFieldDariIsu(hasil.error!.issues);

describe("skemaSandiBaru", () => {
  it("kata sandi yang melanggar kebijakan ditandai di field baru", () => {
    assert.deepEqual(galat(skemaSandiBaru.safeParse({ baru: "lemah", konfirmasi: "lemah" })), { baru: PESAN_SANDI_LEMAH });
  });

  it("konfirmasi berbeda ditandai di field konfirmasi", () => {
    assert.deepEqual(galat(skemaSandiBaru.safeParse({ baru: "SandiBaru#2026", konfirmasi: "SandiBaru#2027" })), {
      konfirmasi: PESAN_KONFIRMASI_BEDA,
    });
  });

  it("kata sandi kuat yang dikonfirmasi diterima", () => {
    assert.equal(skemaSandiBaru.safeParse({ baru: "SandiBaru#2026", konfirmasi: "SandiBaru#2026" }).success, true);
  });
});

describe("skemaGantiSandi", () => {
  it("kata sandi saat ini wajib", () => {
    assert.deepEqual(galat(skemaGantiSandi.safeParse({ lama: "", baru: "SandiBaru#2026", konfirmasi: "SandiBaru#2026" })), {
      lama: PESAN_SANDI_LAMA_KOSONG,
    });
  });

  it("semua aturan lolos", () => {
    assert.equal(
      skemaGantiSandi.safeParse({ lama: "Lama#2025xx", baru: "SandiBaru#2026", konfirmasi: "SandiBaru#2026" }).success,
      true,
    );
  });
});
