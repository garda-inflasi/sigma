import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { AUDIT_AKSI, filterAuditDariQuery, toAuditQuery } from "./audit";

const dariQuery = (qs: string) => filterAuditDariQuery(new URLSearchParams(qs));

describe("filterAuditDariQuery", () => {
  it("tautan riwayat peran menyaring audit pada satu pengguna", () => {
    assert.deepEqual(dariQuery("entitas=pengguna&entitas_id=e2e_1"), {
      entitas: "pengguna",
      entitas_id: "e2e_1",
    });
  });

  it("tanpa kueri tidak ada filter", () => {
    assert.deepEqual(dariQuery(""), {});
  });

  it("membaca semua kunci yang dikenal", () => {
    const f = dariQuery(
      "aktor=Ani&aksi=ASSIGN_ROLE&entitas=pengguna&entitas_id=u1&correlation_id=abc&q=cari&dari=2026-01-01&sampai=2026-02-01&hasil=GAGAL",
    );
    assert.deepEqual(f, {
      aktor: "Ani",
      aksi: "ASSIGN_ROLE",
      entitas: "pengguna",
      entitas_id: "u1",
      correlation_id: "abc",
      q: "cari",
      dari: "2026-01-01",
      sampai: "2026-02-01",
      hasil: "GAGAL",
    });
  });

  it("kunci asing, tanggal rusak, dan hasil tak dikenal diabaikan", () => {
    const f = dariQuery(
      "foo=bar&dari=kemarin&sampai=2026-13&hasil=sukses&limit=500&cursor=x",
    );
    assert.deepEqual(f, {});
  });

  it("nilai kosong atau spasi dibuang dan nilai panjang dipotong", () => {
    assert.deepEqual(dariQuery("aktor=++&entitas_id="), {});
    assert.equal(dariQuery(`q=${"x".repeat(500)}`).q?.length, 200);
  });

  it("hasilnya menjadi kueri API yang sama dengan filter yang dipasang manual", () => {
    const f = dariQuery("entitas=pengguna&entitas_id=e2e_1");
    assert.equal(
      toAuditQuery(f),
      toAuditQuery({ entitas: "pengguna", entitas_id: "e2e_1" }),
    );
    assert.match(toAuditQuery(f), /entitas=pengguna/);
    assert.match(toAuditQuery(f), /entitas_id=e2e_1/);
  });
});

describe("AUDIT_AKSI", () => {
  it("memuat aksi reset MFA", () => {
    assert.ok((AUDIT_AKSI as readonly string[]).includes("RESET_MFA"));
  });
});
