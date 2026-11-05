import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { keluar } from "./logout";

describe("keluar dengan pembersihan data lokal", () => {
  it("urutan: logout server, bersihkan data perangkat, lalu muat ulang /masuk", async () => {
    const urutan: string[] = [];
    await keluar({
      panggilLogout: async () => {
        urutan.push("logout");
      },
      bersihkanLokal: async () => {
        urutan.push("bersih");
      },
      pindah: (url) => urutan.push(`pindah:${url}`),
    });
    assert.deepEqual(urutan, ["logout", "bersih", "pindah:/masuk"]);
  });

  it("data lokal tetap dibersihkan walau logout server gagal (offline atau sesi sudah habis)", async () => {
    const urutan: string[] = [];
    await keluar({
      panggilLogout: async () => {
        throw new Error("jaringan putus");
      },
      bersihkanLokal: async () => {
        urutan.push("bersih");
      },
      pindah: (url) => urutan.push(`pindah:${url}`),
    });
    assert.deepEqual(urutan, ["bersih", "pindah:/masuk"]);
  });

  it("logout yang melempar secara sinkron diperlakukan sama dengan yang ditolak", async () => {
    const urutan: string[] = [];
    await keluar({
      panggilLogout: () => {
        throw new Error("fetch tidak tersedia");
      },
      bersihkanLokal: async () => {
        urutan.push("bersih");
      },
      pindah: (url) => urutan.push(`pindah:${url}`),
    });
    assert.deepEqual(urutan, ["bersih", "pindah:/masuk"]);
  });

  it("pembersihan yang gagal tidak menahan pengguna di halaman", async () => {
    let tujuan = "";
    await keluar({
      panggilLogout: async () => undefined,
      bersihkanLokal: async () => {
        throw new Error("indexeddb terkunci");
      },
      pindah: (url) => {
        tujuan = url;
      },
    });
    assert.equal(tujuan, "/masuk");
  });

  it("logout yang menggantung dilepas setelah batas waktu, data lokal tetap dibersihkan", async () => {
    const urutan: string[] = [];
    const mulai = Date.now();
    await keluar({
      panggilLogout: () => new Promise(() => undefined),
      bersihkanLokal: async () => {
        urutan.push("bersih");
      },
      pindah: (url) => urutan.push(`pindah:${url}`),
      batasWaktuMs: 20,
    });
    assert.deepEqual(urutan, ["bersih", "pindah:/masuk"]);
    assert.ok(
      Date.now() - mulai < 2_000,
      "tidak menunggu batas bawaan 8 detik",
    );
  });

  it("logout yang ditolak sesudah batas waktu tidak menjadi penolakan tak tertangani", async () => {
    let tolak: (e: Error) => void = () => undefined;
    const tunda = new Promise<never>((_, reject) => {
      tolak = reject;
    });
    let tujuan = "";
    await keluar({
      panggilLogout: () => tunda,
      pindah: (url) => {
        tujuan = url;
      },
      batasWaktuMs: 5,
    });
    tolak(new Error("terlambat"));
    await new Promise((r) => setTimeout(r, 10));
    assert.equal(tujuan, "/masuk");
  });

  it("tanpa bersihkanLokal perilakunya sama seperti sebelumnya", async () => {
    const urutan: string[] = [];
    await keluar({
      panggilLogout: async () => {
        urutan.push("logout");
      },
      pindah: (url) => urutan.push(`pindah:${url}`),
    });
    assert.deepEqual(urutan, ["logout", "pindah:/masuk"]);
  });

  it("logout yang cepat selesai segera, tidak menunggu batas waktu", async () => {
    const mulai = Date.now();
    await keluar({
      panggilLogout: async () => undefined,
      pindah: () => undefined,
    });
    assert.ok(Date.now() - mulai < 500);
  });
});
