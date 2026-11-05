import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { tujuanPemulihan, tukarRefreshToken } from "./refresh-session";

class GalatApi extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

describe("tukarRefreshToken", () => {
  it("sukses: access token dan refresh token pengganti", async () => {
    const hasil = await tukarRefreshToken("rt-lama", {
      refresh: async (d) => {
        assert.deepEqual(d, { refresh_token: "rt-lama" });
        return { access_token: "at", refresh_token: "rt-baru" };
      },
    });
    assert.deepEqual(hasil, {
      ok: true,
      accessToken: "at",
      refreshToken: "rt-baru",
    });
  });

  it("pemutaran ulang dalam jendela toleransi: tanpa refresh token (cookie yang ada dipertahankan)", async () => {
    for (const kosong of [undefined, ""]) {
      const hasil = await tukarRefreshToken("rt", {
        refresh: async () => ({ access_token: "at", refresh_token: kosong }),
      });
      assert.deepEqual(hasil, {
        ok: true,
        accessToken: "at",
        refreshToken: undefined,
      });
    }
  });

  it("401 dari API berarti sesi berakhir", async () => {
    const hasil = await tukarRefreshToken("rt", {
      refresh: async () => {
        throw new GalatApi(401, "Token tidak valid");
      },
    });
    assert.deepEqual(hasil, {
      ok: false,
      status: 401,
      message: "Token tidak valid",
      berakhir: true,
    });
  });

  it("galat sementara (5xx, 400) tidak dianggap sesi berakhir sehingga cookie tidak dihapus", async () => {
    for (const status of [400, 429, 500, 503]) {
      const hasil = await tukarRefreshToken("rt", {
        refresh: async () => {
          throw new GalatApi(status, "x");
        },
      });
      assert.equal(hasil.ok, false);
      assert.equal(!hasil.ok && hasil.berakhir, false, String(status));
      assert.equal(!hasil.ok && hasil.status, status);
    }
  });

  it("galat jaringan tanpa status → 500 sementara", async () => {
    const hasil = await tukarRefreshToken("rt", {
      refresh: async () => {
        throw new TypeError("fetch failed");
      },
    });
    assert.deepEqual(hasil, {
      ok: false,
      status: 500,
      message: "Kesalahan server internal.",
      berakhir: false,
    });
  });

  it("jawaban tanpa access token ditolak sebagai sementara", async () => {
    const hasil = await tukarRefreshToken("rt", {
      refresh: async () => ({ access_token: "" }),
    });
    assert.equal(hasil.ok, false);
    assert.equal(!hasil.ok && hasil.berakhir, false);
  });
});

describe("tujuanPemulihan", () => {
  it("menerima path internal beserta query", () => {
    assert.equal(tujuanPemulihan("/ews?id=EWS-1"), "/ews?id=EWS-1");
    assert.equal(tujuanPemulihan("/pengguna"), "/pengguna");
  });

  it("menolak open redirect: URL absolut, //host, /\\host, karakter kontrol", () => {
    for (const jahat of [
      "https://evil.test",
      "//evil.test",
      "/\\evil.test",
      "javascript:alert(1)",
      "/ok\r\nSet-Cookie: x=1",
    ])
      assert.equal(tujuanPemulihan(jahat), "/", jahat);
  });

  it("kosong atau tidak ada → beranda", () => {
    assert.equal(tujuanPemulihan(null), "/");
    assert.equal(tujuanPemulihan(undefined), "/");
    assert.equal(tujuanPemulihan(""), "/");
  });

  it("tidak pernah kembali ke rute pemulihan itu sendiri (loop)", () => {
    assert.equal(tujuanPemulihan("/api/auth/refresh"), "/");
    assert.equal(tujuanPemulihan("/api/auth/refresh?lanjut=/x"), "/");
    assert.equal(tujuanPemulihan("/api/auth/refreshx"), "/api/auth/refreshx");
  });
});
