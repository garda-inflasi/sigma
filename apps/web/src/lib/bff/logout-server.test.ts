import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { cabutSesi, type ApiKeluar } from "./logout-server";

class GalatApi extends Error {
  constructor(
    public status: number,
    message = "x",
  ) {
    super(message);
  }
}

/** API palsu yang mencatat urutan panggilan; jawaban diatur per fungsi. */
function apiPalsu(o: {
  logout?: (at: string, rt?: string) => Promise<unknown>;
  refresh?: (d: { refresh_token: string }) => Promise<{
    access_token: string;
    refresh_token?: string;
  }>;
}) {
  const log: string[] = [];
  const api: ApiKeluar = {
    logout: async (at, rt) => {
      log.push(`logout:${at}:${rt ?? "-"}`);
      return (o.logout ?? (async () => undefined))(at, rt);
    },
    refresh: async (d) => {
      log.push(`refresh:${d.refresh_token}`);
      if (!o.refresh) throw new Error("refresh tidak diharapkan");
      return o.refresh(d);
    },
  };
  return { api, log };
}

describe("cabutSesi", () => {
  it("access token sah: keluar langsung dan meneruskan refresh token cookie", async () => {
    const { api, log } = apiPalsu({});
    const hasil = await cabutSesi(
      { accessToken: "at", refreshToken: "rt" },
      api,
    );
    assert.equal(hasil, "dicabut");
    assert.deepEqual(log, ["logout:at:rt"]);
  });

  it("access token kedaluwarsa (401) tetapi refresh token sah: tukar dulu, lalu keluar dengan access token baru", async () => {
    let n = 0;
    const { api, log } = apiPalsu({
      logout: async () => {
        if (n++ === 0) throw new GalatApi(401);
      },
      refresh: async () => ({ access_token: "at2", refresh_token: "rt2" }),
    });
    const hasil = await cabutSesi(
      { accessToken: "at", refreshToken: "rt" },
      api,
    );
    assert.equal(hasil, "dicabut");
    assert.deepEqual(log, ["logout:at:rt", "refresh:rt", "logout:at2:rt2"]);
  });

  it("tanpa access token sama sekali (cookie sudah lenyap): tetap mencabut lewat refresh token", async () => {
    const { api, log } = apiPalsu({
      refresh: async () => ({ access_token: "at2", refresh_token: "rt2" }),
    });
    const hasil = await cabutSesi({ refreshToken: "rt" }, api);
    assert.equal(hasil, "dicabut");
    assert.deepEqual(log, ["refresh:rt", "logout:at2:rt2"]);
  });

  it("pemutaran ulang (tanpa refresh token baru): memakai refresh token cookie untuk logout", async () => {
    const { api, log } = apiPalsu({
      refresh: async () => ({ access_token: "at2" }),
    });
    await cabutSesi({ refreshToken: "rt" }, api);
    assert.deepEqual(log, ["refresh:rt", "logout:at2:rt"]);
  });

  it("sesi sudah dicabut (refresh 401): dianggap beres tanpa logout kedua", async () => {
    const { api, log } = apiPalsu({
      logout: async () => {
        throw new GalatApi(401);
      },
      refresh: async () => {
        throw new GalatApi(401);
      },
    });
    const hasil = await cabutSesi(
      { accessToken: "at", refreshToken: "rt" },
      api,
    );
    assert.equal(hasil, "sudah-berakhir");
    assert.deepEqual(log, ["logout:at:rt", "refresh:rt"]);
  });

  it("tanpa cookie apa pun tidak ada panggilan jaringan", async () => {
    const { api, log } = apiPalsu({});
    assert.equal(await cabutSesi({}, api), "sudah-berakhir");
    assert.deepEqual(log, []);
  });

  it("access token ditolak 401 dan tidak ada refresh token: sudah berakhir", async () => {
    const { api, log } = apiPalsu({
      logout: async () => {
        throw new GalatApi(401);
      },
    });
    assert.equal(await cabutSesi({ accessToken: "at" }, api), "sudah-berakhir");
    assert.deepEqual(log, ["logout:at:-"]);
  });

  it("galat server (bukan 401) saat logout: gagal, tidak memutar refresh token", async () => {
    const { api, log } = apiPalsu({
      logout: async () => {
        throw new GalatApi(503);
      },
    });
    assert.equal(
      await cabutSesi({ accessToken: "at", refreshToken: "rt" }, api),
      "gagal",
    );
    assert.deepEqual(log, ["logout:at:rt"]);
  });

  it("galat server saat refresh atau logout kedua: gagal, tidak melempar", async () => {
    const a = apiPalsu({
      refresh: async () => {
        throw new GalatApi(500);
      },
    });
    assert.equal(await cabutSesi({ refreshToken: "rt" }, a.api), "gagal");

    const b = apiPalsu({
      logout: async () => {
        throw new TypeError("fetch failed");
      },
      refresh: async () => ({ access_token: "at2", refresh_token: "rt2" }),
    });
    assert.equal(await cabutSesi({ refreshToken: "rt" }, b.api), "gagal");
  });
});
