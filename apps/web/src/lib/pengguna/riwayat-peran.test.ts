import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ROLES } from "@/lib/rbac/roles";
import {
  judulAksiPeran,
  labelPeran,
  namaAktor,
  tautanAuditPengguna,
  toRiwayatPeran,
  type RiwayatPeranResponse,
} from "./riwayat-peran";

const respons: RiwayatPeranResponse = {
  terpotong: false,
  items: [
    {
      id: "a-3",
      waktu: "2026-11-06T09:00:00Z",
      aksi: "UPDATE_USER",
      dari: "MANAJER_DATA",
      ke: "OPS_KOORDINATOR",
      aktor: { id: "adm-2", nama: "Admin Kedua" },
    },
    {
      id: "a-2",
      waktu: "2026-11-05T09:00:00Z",
      aksi: "ASSIGN_ROLE",
      dari: "OPS_MAHASISWA",
      ke: "MANAJER_DATA",
      aktor: { id: null, nama: "Admin Lama" },
    },
    {
      id: "a-1",
      waktu: "2026-11-04T09:00:00Z",
      aksi: "CREATE_USER",
      dari: null,
      ke: "OPS_MAHASISWA",
      aktor: { id: "2dd0fc3b-a8e1-3708-907e-35aa3d7363ac", nama: "Test User" },
    },
  ],
};

describe("toRiwayatPeran", () => {
  it("memetakan peran ke label dan mempertahankan urutan server", () => {
    const { entri, terpotong } = toRiwayatPeran(respons);
    assert.equal(terpotong, false);
    assert.deepEqual(
      entri.map((e) => e.id),
      ["a-3", "a-2", "a-1"],
    );
    assert.equal(entri[0].dari, ROLES.MANAJER_DATA.label);
    assert.equal(entri[0].ke, ROLES.OPS_KOORDINATOR.label);
    assert.equal(entri[0].judul, "Data pengguna diubah");
    assert.equal(entri[1].judul, "Peran diganti");
    assert.equal(entri[2].judul, "Akun dibuat");
  });

  it("peran awal tidak punya sebelumnya", () => {
    const { entri } = toRiwayatPeran(respons);
    assert.equal(entri[2].dari, null);
    assert.equal(entri[2].ke, ROLES.OPS_MAHASISWA.label);
  });

  it("menampilkan pelaku, termasuk akun yang sudah dihapus", () => {
    const { entri } = toRiwayatPeran(respons);
    assert.equal(entri[0].aktor, "Admin Kedua");
    assert.equal(entri[1].aktor, "Admin Lama (akun dihapus)");
    assert.equal(entri[2].aktor, "Test User");
  });

  it("meneruskan penanda terpotong dan menerima riwayat kosong", () => {
    assert.deepEqual(toRiwayatPeran({ items: [], terpotong: false }), {
      entri: [],
      terpotong: false,
    });
    assert.equal(
      toRiwayatPeran({ items: [], terpotong: true }).terpotong,
      true,
    );
  });
});

describe("labelPeran", () => {
  it("peran yang tak dikenal ditampilkan apa adanya, termasuk nama bawaan objek", () => {
    assert.equal(labelPeran("PERAN_LAMA"), "PERAN_LAMA");
    assert.equal(
      labelPeran("constructor"),
      "constructor",
      "bukan properti prototipe objek",
    );
    assert.equal(labelPeran("ADMIN_SISTEM"), ROLES.ADMIN_SISTEM.label);
  });
});

describe("namaAktor", () => {
  it("tanpa pelaku = Sistem", () => {
    assert.equal(namaAktor(null), "Sistem");
    assert.equal(namaAktor({ id: null, nama: null }), "Sistem");
  });
  it("akun ada tetapi nama kosong", () => {
    assert.equal(
      namaAktor({ id: "u-1", nama: null }),
      "Pengguna tidak dikenal",
    );
  });
  it("akun dihapus tetap menyebut nama snapshot", () => {
    assert.equal(namaAktor({ id: null, nama: "Budi" }), "Budi (akun dihapus)");
  });
});

describe("judulAksiPeran", () => {
  it("aksi yang belum dikenal tampil apa adanya agar tetap terlacak", () => {
    assert.equal(
      judulAksiPeran("INVITE_ACCEPT", "OPS_MAHASISWA"),
      "INVITE_ACCEPT",
    );
  });
  it("aksi apa pun yang menetapkan peran dari kosong disebut peran awal bila bukan pembuatan akun", () => {
    assert.equal(judulAksiPeran("UPDATE_USER", null), "Peran awal ditetapkan");
    assert.equal(judulAksiPeran("CREATE_USER", null), "Akun dibuat");
  });
});

describe("tautanAuditPengguna", () => {
  it("menyaring audit pada entitas pengguna dengan id yang di-encode", () => {
    assert.equal(
      tautanAuditPengguna("841c8e22-0f6f-35ed-a0d8-7ca42d78188e"),
      "/audit?entitas=pengguna&entitas_id=841c8e22-0f6f-35ed-a0d8-7ca42d78188e",
    );
    assert.equal(
      tautanAuditPengguna("a b&c"),
      "/audit?entitas=pengguna&entitas_id=a+b%26c",
    );
  });
});
