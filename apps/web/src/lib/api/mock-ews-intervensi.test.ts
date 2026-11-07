import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { cocokRujukanAlert } from "./ews";
import { mockEwsDetail, mockEwsRiwayat } from "./ews-mock";
import { riwayatMock } from "./intervensi-mock";
import {
  mockEws,
  mockEwsAturan,
  mockKanban,
  randomEws,
  uuidMock,
} from "./mock";

const DESIMAL = /^[1-9][0-9]*$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/**
 * Data mock harus setia pada kontrak API (docs/SKEMA-DATABASE.md §5): ID BIGINT berupa string desimal, ID UUID
 * berupa UUID kanonik, dan kode tampilan (ID lama) di field `kode`. Mode mock tidak boleh menyembunyikan
 * regresi yang baru terlihat di backend sungguhan.
 */
describe("mock EWS dan intervensi mengikuti kontrak ID baru", () => {
  it("alert: id desimal unik, kode EWS-…, tautan lama berbasis kode menunjuk baris yang sama", () => {
    const alerts = mockEws();
    assert.ok(alerts.length > 0);
    for (const a of alerts) {
      assert.match(a.id, DESIMAL, a.id);
      assert.match(a.kode ?? "", /^EWS-/, a.id);
    }
    assert.equal(new Set(alerts.map((a) => a.id)).size, alerts.length);
    assert.equal(new Set(alerts.map((a) => a.kode)).size, alerts.length);

    const a = alerts.find((x) => x.kode === "EWS-0901");
    assert.ok(a, "alert seed EWS-0901 ada di mock");
    assert.equal(a.id, "901");
    assert.equal(cocokRujukanAlert(a, "EWS-0901"), true);
    assert.equal(cocokRujukanAlert(a, "901"), true);
    assert.equal(
      alerts.filter((x) => cocokRujukanAlert(x, "EWS-0901")).length,
      1,
    );
  });

  it("alert simulasi langsung juga berid desimal dan berkode", () => {
    const a = randomEws();
    assert.match(a.id, DESIMAL);
    assert.match(a.kode ?? "", /^EWS-L/);
  });

  it("aturan: id desimal dan kode ATR-… sebagai alias", () => {
    for (const r of mockEwsAturan()) {
      assert.match(r.id, DESIMAL, r.id);
      assert.match(r.kode ?? "", /^ATR-/, r.id);
    }
  });

  it("detail alert membawa rujukan aturan (id desimal dan kode ATR-…) bila ada aturan untuk jenis dan komoditasnya", () => {
    const aturan = mockEwsAturan().find((r) => r.kode === "ATR-LH-CBM-KRT");
    assert.ok(aturan);
    const a = mockEws().find((x) => x.jenis === "LONJAKAN_HARGA" && x.komoditasKode === "CBM-KRT");
    assert.ok(a, "ada alert lonjakan cabai merah di mock");
    const d = mockEwsDetail(a);
    assert.equal(d.aturanId, aturan.id);
    assert.equal(d.aturanKode, "ATR-LH-CBM-KRT");

    const tanpa = mockEws().find((x) => x.jenis === "ANOMALI_DATA");
    assert.ok(tanpa);
    assert.equal(mockEwsDetail(tanpa).aturanKode, undefined);
  });

  it("riwayat alert: id string desimal unik per alert", () => {
    const a = mockEws().find((x) => x.status === "DIESKALASI");
    assert.ok(a);
    const ids = mockEwsRiwayat(a).map((r) => r.id);
    assert.ok(ids.length > 1);
    assert.equal(new Set(ids).size, ids.length);
    for (const id of ids) assert.match(id, DESIMAL);
  });

  it("kartu: id UUID unik dan stabil, kode INT-… tidak dipakai sebagai id", () => {
    const kartu = mockKanban();
    for (const k of kartu) {
      assert.match(k.id, UUID, k.id);
      assert.match(k.kode ?? "", /^INT-\d+$/, k.id);
    }
    assert.equal(new Set(kartu.map((k) => k.id)).size, kartu.length);
    assert.deepEqual(
      mockKanban().map((k) => k.id),
      kartu.map((k) => k.id),
      "deterministik antar-panggilan",
    );
  });

  it("riwayat kartu: id string desimal", () => {
    const k = mockKanban().find(
      (x) =>
        x.tahap !== "t-ews" &&
        x.tahap !== "n-pantau" &&
        x.tahap !== "s-deteksi",
    );
    assert.ok(k);
    const rows = riwayatMock(k);
    assert.ok(rows.length > 1);
    for (const r of rows) assert.match(r.id, DESIMAL);
  });

  it("uuidMock berbentuk UUID v3 dan berbeda untuk kunci berbeda", () => {
    assert.match(uuidMock("intervensi", "INT-301"), UUID);
    assert.equal(uuidMock("intervensi", "INT-301")[14], "3");
    assert.notEqual(
      uuidMock("intervensi", "INT-301"),
      uuidMock("intervensi", "INT-302"),
    );
    assert.equal(
      uuidMock("intervensi", "INT-301"),
      uuidMock("intervensi", "INT-301"),
    );
  });
});
