import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { AuditRow } from "@/lib/data/types";
import {
  formatNilaiAudit,
  halamanObjekAudit,
  queryMockAudit,
  toAuditDetail,
  toAuditQuery,
  toExportQuery,
  type AuditDetailResponse,
} from "./audit";

const detail: AuditDetailResponse = {
  id: "a1",
  waktu: "2026-10-16T09:00:00Z",
  aktor_nama: "Rina Safitri",
  aktor_email: "rina@gardainflasi.id",
  aktor_peran: "MANAJER_PROGRAM",
  aksi: "ASSIGN_ROLE",
  entitas: "pengguna",
  entitas_id: "u2",
  hasil: "BERHASIL",
  user_agent: "uji",
  correlation_id: "req-1",
  perubahan: [
    { path: "peran", jenis: "DIUBAH", sebelum: "A", sesudah: "B" },
    { path: "aktif", jenis: "DITAMBAH", sesudah: true },
  ],
};

describe("toAuditDetail", () => {
  it("memetakan baris, metadata permintaan, dan perubahan", () => {
    const d = toAuditDetail(detail);

    assert.equal(d.aktor, "Rina Safitri");
    assert.equal(d.email, "rina@gardainflasi.id");
    assert.equal(d.userAgent, "uji");
    assert.equal(d.correlationId, "req-1");
    assert.equal(d.perubahan.length, 2);
  });

  it("bidang opsional yang kosong menjadi string kosong", () => {
    const d = toAuditDetail({
      ...detail,
      user_agent: undefined,
      correlation_id: undefined,
      aktor_email: undefined,
    });

    assert.equal(d.userAgent, "");
    assert.equal(d.correlationId, "");
    assert.equal(d.email, "");
  });
});

describe("toExportQuery", () => {
  it("memuat hanya filter terisi, tanpa limit/cursor", () => {
    assert.equal(
      toExportQuery({ aksi: "LOGIN", q: "rina", dari: "" }),
      "?aksi=LOGIN&q=rina",
    );
  });

  it("tanpa filter menghasilkan string kosong", () => {
    assert.equal(toExportQuery({}), "");
  });
});

describe("filter lanjutan", () => {
  it("dikirim dengan nama parameter API", () => {
    assert.equal(
      toAuditQuery({ entitas_id: "ENT-01", correlation_id: "req-1", q: "x" }),
      "?limit=50&entitas_id=ENT-01&correlation_id=req-1&q=x",
    );
  });

  const row = (id: string, patch: Partial<AuditRow>): AuditRow => ({
    id,
    waktu: "2026-10-16T09:00:00Z",
    aktor: "Budi",
    peran: "",
    aksi: "LOGIN",
    entitas: "pengguna",
    entitasId: id,
    hasil: "BERHASIL",
    ip: "",
    ...patch,
  });
  const all = [
    row("u1", {}),
    row("u2", { aksi: "ASSIGN_ROLE", aktor: "Rina" }),
    row("u3", { entitas: "EntriHarga" }),
  ];

  it("mock: entitas_id persis dan pencarian bebas", () => {
    assert.deepEqual(
      queryMockAudit(all, { entitas_id: "u2" }).rows.map((r) => r.id),
      ["u2"],
    );
    assert.deepEqual(
      queryMockAudit(all, { q: "assign" }).rows.map((r) => r.id),
      ["u2"],
    );
    assert.deepEqual(
      queryMockAudit(all, { q: "ENTRIHARGA" }).rows.map((r) => r.id),
      ["u3"],
    );
  });
});

describe("halamanObjekAudit", () => {
  it("mengenali nama entitas backend dan mock", () => {
    assert.equal(halamanObjekAudit("entri_harga"), "/data/harga");
    assert.equal(halamanObjekAudit("EntriHarga"), "/data/harga");
    assert.equal(halamanObjekAudit("stok_cold_storage"), "/cold-storage");
    assert.equal(halamanObjekAudit("pengguna"), "/pengguna");
  });

  it("entitas tak dikenal tanpa tautan", () => {
    assert.equal(halamanObjekAudit("audit_log"), undefined);
  });
});

describe("formatNilaiAudit", () => {
  it("string apa adanya, lainnya JSON, undefined kosong", () => {
    assert.equal(formatNilaiAudit("A"), "A");
    assert.equal(formatNilaiAudit(true), "true");
    assert.equal(formatNilaiAudit([1, 2]), "[1,2]");
    assert.equal(formatNilaiAudit(null), "null");
    assert.equal(formatNilaiAudit(undefined), "");
  });
});
