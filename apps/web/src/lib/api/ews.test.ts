import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  cocokRujukanAlert,
  satuanAmbang,
  toEwsAlert,
  kunciVapidKeBytes,
  toEwsAturan,
  toBatalPushBody,
  toEwsDetail,
  toEwsRiwayat,
  toLanggananPushBody,
  toPreferensiBody,
  toPreferensiView,
  toUbahAturanBody,
  validasiAmbang,
  type EwsAlertResponse,
} from "./ews";

const alert: EwsAlertResponse = {
  id: "1000",
  kode: "EWS-1000",
  severity: "KRITIS",
  jenis: "LONJAKAN_HARGA",
  judul: "Lonjakan harga cabai merah keriting",
  pesan: "Harga naik.",
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "Cabai merah keriting",
  kabupaten_kode: "1171",
  wilayah: "KOTA BANDA ACEH",
  nilai: 21.4,
  satuan_nilai: "%",
  skor_dampak: 91,
  status: "BARU",
  sumber: "Model EWS v2.3",
  dibuat_pada: "2026-10-16T08:56:00Z",
};

describe("toEwsAlert", () => {
  it("memetakan kolom API ke model UI dengan nama wilayah ringkas", () => {
    assert.deepEqual(toEwsAlert(alert), {
      id: "1000",
      kode: "EWS-1000",
      severity: "KRITIS",
      jenis: "LONJAKAN_HARGA",
      judul: "Lonjakan harga cabai merah keriting",
      komoditasKode: "CBM-KRT",
      komoditasNama: "Cabai merah keriting",
      kabupatenKode: "1171",
      wilayah: "Kota Banda Aceh",
      nilai: 21.4,
      satuanNilai: "%",
      pesan: "Harga naik.",
      waktu: "2026-10-16T08:56:00Z",
      status: "BARU",
      skorDampak: 91,
      sumber: "Model EWS v2.3",
    });
  });

  it("alert tanpa komoditas tidak membawa kunci komoditas", () => {
    const row = toEwsAlert({
      ...alert,
      komoditas_kode: null,
      komoditas_nama: null,
      satuan_nilai: "mm",
    });
    assert.equal(row.komoditasKode, undefined);
    assert.equal(row.komoditasNama, undefined);
    assert.equal(row.satuanNilai, "mm");
  });

  it("satuan tak dikenal jatuh ke persen", () => {
    assert.equal(toEwsAlert({ ...alert, satuan_nilai: "?" }).satuanNilai, "%");
  });

  it("id tetap string desimal dan kode opsional (null atau tidak ada tidak membawa kode)", () => {
    assert.equal(toEwsAlert(alert).id, "1000");
    assert.equal(toEwsAlert({ ...alert, kode: null }).kode, undefined);
    const { kode: _kode, ...tanpaKode } = alert;
    assert.equal(toEwsAlert(tanpaKode).kode, undefined);
  });
});

describe("cocokRujukanAlert", () => {
  const a = { id: "12", kode: "EWS-0901" };

  it("tautan baru memakai id desimal", () => {
    assert.equal(cocokRujukanAlert(a, "12"), true);
    assert.equal(cocokRujukanAlert(a, "13"), false);
  });

  it("tautan lama dari email atau push memakai kode dan tetap menyorot baris yang sama", () => {
    assert.equal(cocokRujukanAlert(a, "EWS-0901"), true);
    assert.equal(cocokRujukanAlert(a, "EWS-0902"), false);
  });

  it("tanpa rujukan tidak ada baris yang cocok, termasuk baris tanpa kode", () => {
    assert.equal(cocokRujukanAlert(a, null), false);
    assert.equal(cocokRujukanAlert(a, undefined), false);
    assert.equal(cocokRujukanAlert(a, ""), false);
    assert.equal(cocokRujukanAlert({ id: "7" }, "undefined"), false);
  });
});

describe("toEwsAturan", () => {
  it("memetakan ambang dan kanal", () => {
    const row = toEwsAturan({
      id: "1",
      kode: "ATR-LH-CBM-KRT",
      komoditas_kode: "CBM-KRT",
      komoditas_nama: "Cabai merah keriting",
      kabupaten_kode: null,
      jenis: "LONJAKAN_HARGA",
      ambang_waspada: 8,
      ambang_siaga: 12,
      ambang_kritis: 18,
      jendela_hari: 7,
      cooldown_jam: 24,
      kanal: ["DASHBOARD", "PUSH"],
      aktif: true,
      keterangan: "Ambang awal",
      versi: 3,
    });
    assert.deepEqual([row.waspada, row.siaga, row.kritis], [8, 12, 18]);
    assert.equal(row.id, "1");
    assert.equal(row.kode, "ATR-LH-CBM-KRT");
    assert.equal(row.versi, 3);
    assert.equal(row.cooldownJam, 24);
    assert.equal(row.kabupatenKode, undefined);
    assert.deepEqual(row.kanal, ["DASHBOARD", "PUSH"]);
    assert.equal(row.komoditasNama, "Cabai merah keriting");
  });
});

describe("satuanAmbang", () => {
  it("cuaca dalam mm, lainnya persen", () => {
    assert.equal(satuanAmbang("ANOMALI_CUACA"), "mm");
    assert.equal(satuanAmbang("LONJAKAN_HARGA"), "%");
    assert.equal(satuanAmbang("KELANGKAAN_STOK"), "%");
  });
});

describe("toUbahAturanBody", () => {
  it("menerjemahkan masukan formulir ke badan PUT snake_case dengan versi yang dibaca", () => {
    assert.deepEqual(
      toUbahAturanBody({ waspada: 5, siaga: 10, kritis: 20, jendelaHari: 7, cooldownJam: 12, kanal: ["EMAIL"], aktif: true, keterangan: "x", versi: 2 }),
      { versi: 2, ambang_waspada: 5, ambang_siaga: 10, ambang_kritis: 20, jendela_hari: 7, cooldown_jam: 12, kanal: ["EMAIL"], aktif: true, keterangan: "x" },
    );
  });
});

describe("validasiAmbang", () => {
  it("menerima ambang positif yang menaik", () => {
    assert.equal(validasiAmbang({ waspada: 1, siaga: 2, kritis: 3 }), null);
  });
  it("menolak urutan sama, menurun, nol, dan bukan angka", () => {
    for (const a of [
      { waspada: 5, siaga: 5, kritis: 9 },
      { waspada: 9, siaga: 5, kritis: 1 },
      { waspada: 0, siaga: 5, kritis: 9 },
      { waspada: Number.NaN, siaga: 5, kritis: 9 },
    ]) assert.notEqual(validasiAmbang(a), null);
  });
});

describe("preferensi notifikasi", () => {
  it("memetakan jawaban API ke tampilan dan kembali ke badan PUT", () => {
    const view = toPreferensiView({
      preferensi: [
        { kanal: "EMAIL", severity_min: "SIAGA", aktif: true, tersedia: false },
        { kanal: "PUSH", severity_min: "KRITIS", aktif: false, tersedia: true },
      ],
      vapid_public_key: "BKey",
    });
    assert.equal(view.vapidPublicKey, "BKey");
    assert.deepEqual(view.preferensi[1], { kanal: "PUSH", severityMin: "KRITIS", aktif: false, tersedia: true });
    assert.deepEqual(toPreferensiBody(view.preferensi), {
      preferensi: [
        { kanal: "EMAIL", severity_min: "SIAGA", aktif: true },
        { kanal: "PUSH", severity_min: "KRITIS", aktif: false },
      ],
    });
  });
});

describe("langganan push", () => {
  it("mengambil endpoint dan kunci dari PushSubscription.toJSON()", () => {
    assert.deepEqual(toLanggananPushBody({ endpoint: "https://push.example/a", keys: { p256dh: "k", auth: "a" } }), { endpoint: "https://push.example/a", p256dh: "k", auth: "a" });
  });
  it("menolak langganan yang tidak lengkap", () => {
    assert.equal(toLanggananPushBody({}), null);
    assert.equal(toLanggananPushBody({ endpoint: "https://x", keys: { p256dh: "k" } }), null);
    assert.equal(toLanggananPushBody({ keys: { p256dh: "k", auth: "a" } }), null);
  });
  it("mengubah kunci VAPID base64url menjadi bytes", () => {
    // "AQID_w" = bytes 01 02 03 ff (base64url tanpa padding, memakai '_' )
    assert.deepEqual([...kunciVapidKeBytes("AQID_w")], [1, 2, 3, 255]);
  });
});

describe("detail dan riwayat alert", () => {
  it("toEwsDetail membawa jejak pengakuan, eskalasi, dan penutupan", () => {
    const d = toEwsDetail({
      ...alert,
      aturan_id: "3",
      aturan_kode: "ATR-LH-CBM",
      diakui_pada: "2026-10-16T09:00:00Z",
      dieskalasi_pada: "2026-10-16T10:00:00Z",
      selesai_pada: null,
    });
    assert.equal(d.judul, "Lonjakan harga cabai merah keriting");
    assert.equal(d.aturanId, "3");
    assert.equal(d.aturanKode, "ATR-LH-CBM");
    assert.equal(d.kode, "EWS-1000");
    assert.equal(d.diakuiPada, "2026-10-16T09:00:00Z");
    assert.equal(d.dieskalasiPada, "2026-10-16T10:00:00Z");
    assert.equal(d.selesaiPada, undefined);
  });

  it("toEwsDetail tetap sah untuk alert tanpa jejak (kolom opsional hilang)", () => {
    const d = toEwsDetail(alert);
    assert.equal(d.diakuiPada, undefined);
    assert.equal(d.aturanId, undefined);
    assert.equal(d.aturanKode, undefined);
  });

  it("toEwsRiwayat: perubahan oleh mesin tidak punya pelaku dan status asal", () => {
    assert.deepEqual(
      toEwsRiwayat({ id: "1", dari: null, ke: "BARU", oleh_id: null, oleh_nama: null, waktu: "2026-10-16T08:56:00Z", catatan: "Dibuat oleh rule engine" }),
      { id: "1", dari: undefined, ke: "BARU", olehNama: undefined, waktu: "2026-10-16T08:56:00Z", catatan: "Dibuat oleh rule engine" },
    );
  });

  it("toEwsRiwayat: perubahan oleh pengguna memuat nama dan status asal", () => {
    const r = toEwsRiwayat({ id: "2", dari: "BARU", ke: "DIAKUI", oleh_id: "u1", oleh_nama: "Manajer Data", waktu: "2026-10-16T09:00:00Z", catatan: "" });
    assert.equal(r.dari, "BARU");
    assert.equal(r.olehNama, "Manajer Data");
  });

  it("toBatalPushBody hanya mengirim endpoint", () => {
    assert.deepEqual(toBatalPushBody("https://push.example/a"), { endpoint: "https://push.example/a" });
  });
});
