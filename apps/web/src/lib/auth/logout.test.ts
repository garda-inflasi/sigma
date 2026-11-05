import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { keluar, LOGIN_URL } from "./logout";

describe("keluar", () => {
  it("memanggil logout lebih dulu, lalu memuat ulang halaman masuk", async () => {
    const urutan: string[] = [];
    await keluar({
      panggilLogout: async () => {
        urutan.push("logout");
      },
      pindah: (url) => urutan.push(`pindah:${url}`),
    });
    assert.deepEqual(urutan, ["logout", "pindah:/masuk"]);
    assert.equal(LOGIN_URL, "/masuk");
  });

  it("tetap ke halaman masuk bila logout gagal", async () => {
    let tujuan = "";
    await keluar({
      panggilLogout: async () => {
        throw new Error("jaringan putus");
      },
      pindah: (url) => {
        tujuan = url;
      },
    });
    assert.equal(tujuan, "/masuk");
  });
});
