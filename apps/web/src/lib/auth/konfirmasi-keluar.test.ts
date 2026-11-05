import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  kirimLaluPeriksa,
  pesanHasilKirim,
  tentukanAksiKeluar,
  type DepsKirimDulu,
} from "./konfirmasi-keluar";

describe("tentukanAksiKeluar", () => {
  it("keluar langsung bila tidak ada entri tertahan", async () => {
    assert.deepEqual(await tentukanAksiKeluar(async () => 0), {
      jenis: "langsung",
    });
  });

  it("minta konfirmasi dengan jumlah entri bila ada yang tertahan", async () => {
    assert.deepEqual(await tentukanAksiKeluar(async () => 3), {
      jenis: "konfirmasi",
      sisa: 3,
    });
  });

  it("gagal membaca antrean tidak menahan pengguna", async () => {
    assert.deepEqual(
      await tentukanAksiKeluar(async () => {
        throw new Error("idb rusak");
      }),
      { jenis: "langsung" },
    );
  });
});

function kirimDeps(ubah: Partial<DepsKirimDulu> & { sisa?: number[] } = {}) {
  const log: string[] = [];
  const sisa = [...(ubah.sisa ?? [0])];
  const d: DepsKirimDulu = {
    online: () => true,
    siapkan: async () => {
      log.push("siapkan");
    },
    kirim: async () => {
      log.push("kirim");
    },
    hitungSisa: () => (sisa.length > 1 ? sisa.shift()! : sisa[0]),
    ...ubah,
  };
  return { d, log };
}

describe("kirimLaluPeriksa", () => {
  it("selesai bila semua entri terkirim, setelah menyiapkan lalu mengirim", async () => {
    const { d, log } = kirimDeps({ sisa: [0] });
    assert.deepEqual(await kirimLaluPeriksa(d), { jenis: "selesai" });
    assert.deepEqual(log, ["siapkan", "kirim"]);
  });

  it("offline: tidak mengirim, melaporkan sisa", async () => {
    const { d, log } = kirimDeps({ online: () => false, sisa: [2] });
    assert.deepEqual(await kirimLaluPeriksa(d), { jenis: "offline", sisa: 2 });
    assert.deepEqual(log, ["siapkan"]);
  });

  it("sebagian gagal atau konflik: melaporkan sisa yang tertinggal", async () => {
    const { d } = kirimDeps({ sisa: [1] });
    assert.deepEqual(await kirimLaluPeriksa(d), { jenis: "sisa", sisa: 1 });
  });

  it("galat tak terduga saat mengirim dilaporkan dengan pesan dan sisa", async () => {
    const { d } = kirimDeps({
      kirim: async () => {
        throw new Error("server 500");
      },
      sisa: [2],
    });
    assert.deepEqual(await kirimLaluPeriksa(d), {
      jenis: "galat",
      sisa: 2,
      pesan: "server 500",
    });
  });

  it("galat yang bukan Error dan sisa yang tak terbaca tetap menghasilkan laporan", async () => {
    const { d } = kirimDeps({
      siapkan: async () => Promise.reject("ditolak"),
      hitungSisa: () => {
        throw new Error("x");
      },
    });
    assert.deepEqual(await kirimLaluPeriksa(d), {
      jenis: "galat",
      sisa: 0,
      pesan: "Gagal mengirim",
    });
  });
});

describe("pesanHasilKirim", () => {
  it("tidak ada pesan saat selesai", () => {
    assert.equal(pesanHasilKirim({ jenis: "selesai" }), null);
  });

  it("menyebut jumlah sisa dan jalan keluar pada setiap kasus", () => {
    assert.match(pesanHasilKirim({ jenis: "offline", sisa: 2 })!, /offline/);
    const sisa = pesanHasilKirim({ jenis: "sisa", sisa: 3 })!;
    assert.match(sisa, /3 entri/);
    assert.match(sisa, /Riwayat/);
    const galat = pesanHasilKirim({
      jenis: "galat",
      sisa: 1,
      pesan: "server 500",
    })!;
    assert.match(galat, /server 500/);
    assert.match(galat, /1 entri/);
  });
});
