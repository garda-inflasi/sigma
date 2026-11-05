import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { AuditRow } from "@/lib/data/types";
import {
  queryMockAudit,
  toAuditPage,
  toAuditQuery,
  toAuditRow,
  type AuditLogResponse,
} from "./audit";

const response: AuditLogResponse = {
  id: "a1",
  waktu: "2026-10-16T09:00:00Z",
  aktor_id: "u1",
  aktor_nama: "Rina Safitri",
  aktor_email: "rina@gardainflasi.id",
  aktor_peran: "MANAJER_PROGRAM",
  aksi: "ASSIGN_ROLE",
  entitas: "pengguna",
  entitas_id: "u2",
  hasil: "BERHASIL",
  ip: "10.0.0.7",
};

describe("toAuditRow", () => {
  it("memetakan aktor, peran (label), dan entitas", () => {
    const row = toAuditRow(response);

    assert.equal(row.aktor, "Rina Safitri");
    assert.equal(row.peran, "Manajer Program");
    assert.equal(row.entitasId, "u2");
    assert.equal(row.hasil, "BERHASIL");
    assert.equal(row.ip, "10.0.0.7");
  });

  it("aktor tanpa nama memakai email, tanpa aktor menjadi Sistem", () => {
    assert.equal(
      toAuditRow({ ...response, aktor_nama: undefined }).aktor,
      "rina@gardainflasi.id",
    );
    assert.equal(
      toAuditRow({
        ...response,
        aktor_nama: undefined,
        aktor_email: undefined,
        aktor_peran: undefined,
      }).aktor,
      "Sistem",
    );
    assert.equal(toAuditRow({ ...response, aktor_peran: undefined }).peran, "");
  });

  it("mempertahankan hasil GAGAL dan DITOLAK", () => {
    assert.equal(toAuditRow({ ...response, hasil: "GAGAL" }).hasil, "GAGAL");
    assert.equal(
      toAuditRow({ ...response, hasil: "DITOLAK" }).hasil,
      "DITOLAK",
    );
  });
});

describe("toAuditPage", () => {
  it("meneruskan cursor halaman berikutnya", () => {
    assert.equal(
      toAuditPage({ items: [response], next_cursor: "abc" }).nextCursor,
      "abc",
    );
    assert.equal(toAuditPage({ items: [response] }).nextCursor, undefined);
  });
});

describe("toAuditQuery", () => {
  it("memuat limit dan hanya filter yang terisi", () => {
    assert.equal(
      toAuditQuery({ aksi: "LOGIN", hasil: "GAGAL", aktor: "" }, "kursor"),
      "?limit=50&aksi=LOGIN&hasil=GAGAL&cursor=kursor",
    );
  });

  it("tanpa filter hanya limit", () => {
    assert.equal(toAuditQuery({}, undefined, 10), "?limit=10");
  });
});

function row(i: number, patch: Partial<AuditRow> = {}): AuditRow {
  return {
    id: `r${i}`,
    waktu: new Date(Date.UTC(2026, 9, 16, 12, 0) - i * 3_600_000).toISOString(),
    aktor: i % 2 ? "Rina Safitri" : "Budi",
    peran: "",
    aksi: i % 3 ? "LOGIN" : "ASSIGN_ROLE",
    entitas: "pengguna",
    entitasId: `u${i}`,
    hasil: i % 5 === 0 ? "DITOLAK" : "BERHASIL",
    ip: "",
    ...patch,
  };
}

describe("queryMockAudit", () => {
  const all = Array.from({ length: 30 }, (_, i) => row(i));

  it("mengurutkan terbaru dulu dan memaginasi dengan cursor sampai habis", () => {
    const seen: string[] = [];
    let cursor: string | undefined;
    do {
      const page = queryMockAudit(all, {}, cursor, 12);
      seen.push(...page.rows.map((r) => r.id));
      cursor = page.nextCursor;
    } while (cursor);

    assert.equal(seen.length, 30);
    assert.equal(new Set(seen).size, 30);
    assert.equal(seen[0], "r0");
  });

  it("menyaring aksi, hasil, dan aktor", () => {
    assert.ok(
      queryMockAudit(all, { aksi: "ASSIGN_ROLE" }, undefined, 100).rows.every(
        (r) => r.aksi === "ASSIGN_ROLE",
      ),
    );
    assert.ok(
      queryMockAudit(all, { hasil: "DITOLAK" }, undefined, 100).rows.every(
        (r) => r.hasil === "DITOLAK",
      ),
    );
    assert.ok(
      queryMockAudit(all, { aktor: "rina" }, undefined, 100).rows.every(
        (r) => r.aktor === "Rina Safitri",
      ),
    );
  });

  it("sampai bertanggal mencakup seluruh hari itu", () => {
    const hariItu = queryMockAudit(
      all,
      { dari: "2026-10-16", sampai: "2026-10-16" },
      undefined,
      100,
    ).rows;

    assert.ok(hariItu.length > 0);
    assert.ok(hariItu.every((r) => new Date(r.waktu).getDate() === 16));
  });
});
