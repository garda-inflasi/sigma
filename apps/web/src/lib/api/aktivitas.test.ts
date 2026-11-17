import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  AKSI_FILTER,
  UKURAN_HALAMAN,
  labelAksi,
  queryAktivitas,
  queryRiwayatSesi,
  toAktivitas,
  toRiwayatSesi,
} from "./aktivitas";

describe("labelAksi", () => {
  it("aksi dikenal berbahasa Indonesia, aksi baru jatuh ke kode mentah", () => {
    assert.equal(labelAksi("LOGIN"), "Masuk");
    assert.equal(labelAksi("PUTUS_PERSETUJUAN"), "Memutuskan persetujuan intervensi");
    assert.equal(labelAksi("AKSI_BARU"), "AKSI_BARU");
  });

  it("setiap pilihan filter punya label", () => {
    for (const k of AKSI_FILTER) assert.notEqual(labelAksi(k), "", k);
  });
});

describe("queryAktivitas", () => {
  it("hanya nilai terisi; sebagai=semua dan filter kosong tidak dikirim; limit selalu ada", () => {
    assert.equal(queryAktivitas({ sebagai: "semua" }), `limit=${UKURAN_HALAMAN}`);
    const q = new URLSearchParams(
      queryAktivitas({ aksi: "LOGIN", hasil: "GAGAL", sebagai: "sasaran", dari: "2026-11-01", sampai: "2026-11-08" }, "abc"),
    );
    assert.equal(q.get("aksi"), "LOGIN");
    assert.equal(q.get("hasil"), "GAGAL");
    assert.equal(q.get("sebagai"), "sasaran");
    assert.equal(q.get("dari"), "2026-11-01");
    assert.equal(q.get("sampai"), "2026-11-08");
    assert.equal(q.get("cursor"), "abc");
  });

  it("tidak pernah membawa parameter pemilih akun", () => {
    const q = new URLSearchParams(queryAktivitas({ aksi: "LOGIN" }));
    for (const k of ["aktor", "user_id", "pengguna_id", "q"]) assert.equal(q.has(k), false, k);
  });
});

describe("queryRiwayatSesi", () => {
  it("semua tidak dikirim; aktif dan berakhir dikirim; kursor diteruskan", () => {
    assert.equal(queryRiwayatSesi("semua"), `limit=${UKURAN_HALAMAN}`);
    assert.equal(new URLSearchParams(queryRiwayatSesi("berakhir", "c1")).get("status"), "berakhir");
    assert.equal(new URLSearchParams(queryRiwayatSesi("aktif", "c1")).get("cursor"), "c1");
  });
});

describe("pemeta", () => {
  it("toAktivitas mengisi nilai bawaan untuk bidang opsional", () => {
    const a = toAktivitas({
      id: "7",
      waktu: "2026-11-08T08:00:00Z",
      aksi: "RESET_MFA",
      entitas: "pengguna",
      entitas_id: "u",
      hasil: "BERHASIL",
      sebagai: "sasaran",
      oleh_peran: "ADMIN_SISTEM",
    });
    assert.equal(a.label, "Verifikasi 2 langkah diatur ulang admin");
    assert.equal(a.ip, "");
    assert.equal(a.perangkat, "");
    assert.equal(a.olehPeran, "ADMIN_SISTEM");
  });

  it("toRiwayatSesi memberi nama perangkat bawaan dan menandai sesi ini", () => {
    const s = toRiwayatSesi({
      id: "s",
      perangkat: " ",
      mulai_pada: "2026-11-08T08:00:00Z",
      terakhir_dipakai_pada: "2026-11-08T09:00:00Z",
      status: "DICABUT",
      berakhir_pada: "2026-11-08T09:30:00Z",
      saat_ini: false,
    });
    assert.equal(s.perangkat, "Perangkat tidak dikenal");
    assert.equal(s.saatIni, false);
    assert.equal(s.berakhirPada, "2026-11-08T09:30:00Z");
  });
});
