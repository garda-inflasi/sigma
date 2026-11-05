import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  buatPemulih,
  fetchDenganPemulihan,
  perluPulih,
  tujuanSetelahBerakhir,
  type CatatanSegar,
} from "./sesi-pulih";

/** Refresh palsu yang dapat diatur kapan selesainya dan apa jawabannya. */
function refreshTerkendali() {
  const panggilan: number[] = [];
  let lepas: (status: number) => void = () => {};
  const refresh = () =>
    new Promise<{ status: number }>((resolve) => {
      panggilan.push(panggilan.length);
      lepas = (status) => resolve({ status });
    });
  return { refresh, panggilan, selesai: (status: number) => lepas(status) };
}

const res = (status: number, body: unknown = {}) =>
  Response.json(body, { status });

describe("perluPulih", () => {
  it("hanya 401 yang memicu pemulihan", () => {
    for (const status of [200, 400, 403, 404, 500, 503])
      assert.equal(perluPulih("/api/v1/x", status, "x"), false);
    assert.equal(perluPulih("/api/v1/x", 401, "Unauthorized"), true);
  });

  it("401 karena token (Go, BFF, edge, sesi dicabut) layak dipulihkan", () => {
    for (const pesan of [
      "Unauthorized",
      "Token tidak valid",
      "Tidak terautentikasi.",
      "Token expired",
      "Sesi telah berakhir, silakan masuk kembali",
      undefined,
    ])
      assert.equal(
        perluPulih("/api/pengguna", 401, pesan),
        true,
        String(pesan),
      );
  });

  it("401 karena kata sandi salah bukan sesi kedaluwarsa: tidak diulang (menghindari percobaan ganda)", () => {
    assert.equal(
      perluPulih("/api/auth/change-password", 401, "Password lama salah"),
      false,
    );
    assert.equal(
      perluPulih("/api/auth/mfa/disable", 401, "Password salah"),
      false,
    );
    assert.equal(perluPulih("/api/x", 401, "Kata sandi tidak cocok"), false);
  });

  it("alur masuk itu sendiri tidak dipulihkan", () => {
    for (const path of [
      "/api/auth/login",
      "/api/auth/refresh",
      "/api/auth/logout",
      "/api/auth/mfa/validate",
      "/api/auth/forgot-password",
      "/api/auth/reset-password",
    ])
      assert.equal(perluPulih(path, 401, "Unauthorized"), false, path);
  });

  it("endpoint auth lain yang butuh sesi tetap dipulihkan; query tidak mengecoh", () => {
    assert.equal(
      perluPulih("/api/auth/me", 401, "Tidak terautentikasi."),
      true,
    );
    assert.equal(perluPulih("/api/auth/sessions", 401, undefined), true);
    assert.equal(perluPulih("/api/auth/refreshx", 401, undefined), true);
    assert.equal(
      perluPulih("/api/v1/x?next=/api/auth/login", 401, undefined),
      true,
    );
    assert.equal(
      perluPulih("https://app.test/api/auth/login?x=1", 401, undefined),
      false,
    );
  });
});

describe("buatPemulih (single-flight)", () => {
  it("banyak pemanggil serentak berbagi satu refresh", async () => {
    const r = refreshTerkendali();
    const pulih = buatPemulih({ refresh: r.refresh });
    const hasil = [pulih(), pulih(), pulih()];
    assert.equal(r.panggilan.length, 1);
    r.selesai(200);
    assert.deepEqual(await Promise.all(hasil), ["ok", "ok", "ok"]);
  });

  it("setelah selesai, pemanggilan berikutnya refresh lagi", async () => {
    const r = refreshTerkendali();
    const pulih = buatPemulih({ refresh: r.refresh });
    const a = pulih();
    r.selesai(200);
    await a;
    const b = pulih();
    assert.equal(r.panggilan.length, 2);
    r.selesai(200);
    assert.equal(await b, "ok");
  });

  it("401 → berakhir, saatBerakhir dipanggil sekali, dan pemanggilan berikutnya tidak menyentuh jaringan", async () => {
    const r = refreshTerkendali();
    let berakhir = 0;
    const pulih = buatPemulih({
      refresh: r.refresh,
      saatBerakhir: () => {
        berakhir += 1;
      },
    });
    const a = pulih();
    r.selesai(401);
    assert.equal(await a, "berakhir");
    assert.equal(await pulih(), "berakhir");
    assert.equal(await pulih(), "berakhir");
    assert.equal(r.panggilan.length, 1);
    assert.equal(berakhir, 1);
    assert.equal(pulih.berakhir(), true);
  });

  it("5xx, 429, dan jaringan putus bersifat sementara: gagal, bukan berakhir", async () => {
    for (const status of [500, 502, 503, 429]) {
      let dipanggil = 0;
      const pulih = buatPemulih({
        refresh: async () => ({ status }),
        saatBerakhir: () => {
          dipanggil += 1;
        },
      });
      assert.equal(await pulih(), "gagal", String(status));
      assert.equal(pulih.berakhir(), false);
      assert.equal(dipanggil, 0);
    }
    const putus = buatPemulih({
      refresh: async () => {
        throw new TypeError("network");
      },
    });
    assert.equal(await putus(), "gagal");
    assert.equal(putus.berakhir(), false);
  });

  it("kegagalan sementara tidak mengunci: percobaan berikutnya boleh berhasil", async () => {
    const status = [503, 200];
    const pulih = buatPemulih({
      refresh: async () => ({ status: status.shift()! }),
    });
    assert.equal(await pulih(), "gagal");
    assert.equal(await pulih(), "ok");
  });
});

describe("buatPemulih (antar-tab)", () => {
  it("memakai kunci untuk menjalankan refresh", async () => {
    const urutan: string[] = [];
    const pulih = buatPemulih({
      refresh: async () => {
        urutan.push("refresh");
        return { status: 200 };
      },
      kunci: async (jalankan) => {
        urutan.push("kunci-masuk");
        const hasil = await jalankan();
        urutan.push("kunci-keluar");
        return hasil;
      },
    });
    assert.equal(await pulih(), "ok");
    assert.deepEqual(urutan, ["kunci-masuk", "refresh", "kunci-keluar"]);
  });

  it("tab lain baru saja berhasil setelah permintaan ini dimulai → lewati refresh", async () => {
    let dipanggil = 0;
    const pulih = buatPemulih({
      refresh: async () => {
        dipanggil += 1;
        return { status: 200 };
      },
      baca: () => ({ t: 1_000 }),
    });
    assert.equal(await pulih(900), "ok");
    assert.equal(dipanggil, 0);
  });

  it("refresh tab lain sudah lebih tua dari permintaan ini → tetap refresh sendiri", async () => {
    let dipanggil = 0;
    const pulih = buatPemulih({
      refresh: async () => {
        dipanggil += 1;
        return { status: 200 };
      },
      baca: () => ({ t: 1_000 }),
    });
    assert.equal(await pulih(1_001), "ok");
    assert.equal(dipanggil, 1);
  });

  it("tanpa waktu mulai permintaan catatan tidak dipakai", async () => {
    let dipanggil = 0;
    const pulih = buatPemulih({
      refresh: async () => {
        dipanggil += 1;
        return { status: 200 };
      },
      baca: () => ({ t: Number.MAX_SAFE_INTEGER }),
    });
    await pulih();
    assert.equal(dipanggil, 1);
  });

  it("refresh berhasil dicatat untuk tab lain; yang gagal tidak", async () => {
    const tercatat: CatatanSegar[] = [];
    const ok = buatPemulih({
      refresh: async () => ({ status: 200 }),
      tulis: (c) => tercatat.push(c),
      sekarang: () => 42,
    });
    await ok();
    assert.deepEqual(tercatat, [{ t: 42 }]);

    const gagal = buatPemulih({
      refresh: async () => ({ status: 401 }),
      tulis: (c) => tercatat.push(c),
    });
    await gagal();
    assert.equal(tercatat.length, 1);
  });

  it("dua tab: yang kedua menunggu kunci lalu melewati refresh karena yang pertama baru selesai", async () => {
    // Simulasi dua tab berbagi satu antrean kunci dan satu catatan.
    let ekor: Promise<unknown> = Promise.resolve();
    const kunci = <T>(jalankan: () => Promise<T>) => {
      const hasil = ekor.then(jalankan);
      ekor = hasil.catch(() => undefined);
      return hasil;
    };
    let catatan: CatatanSegar | null = null;
    let refreshKe = 0;
    const buatTab = () =>
      buatPemulih({
        refresh: async () => {
          refreshKe += 1;
          await new Promise((r) => setTimeout(r, 5));
          return { status: 200 };
        },
        kunci,
        baca: () => catatan,
        tulis: (c) => {
          catatan = c;
        },
        sekarang: Date.now,
      });
    const mulai = Date.now();
    const [tabA, tabB] = [buatTab(), buatTab()];
    const [a, b] = await Promise.all([tabA(mulai), tabB(mulai)]);
    assert.deepEqual([a, b], ["ok", "ok"]);
    assert.equal(refreshKe, 1, "token hanya diputar sekali untuk dua tab");
  });
});

describe("fetchDenganPemulihan", () => {
  const siap = async () => "ok" as const;

  it("jawaban bukan 401 diteruskan tanpa pemulihan", async () => {
    let pulihDipanggil = false;
    const hasil = await fetchDenganPemulihan(
      async () => res(200, { a: 1 }),
      async () => {
        pulihDipanggil = true;
        return "ok";
      },
      "/api/v1/x",
    );
    assert.equal(hasil.status, 200);
    assert.equal(pulihDipanggil, false);
  });

  it("401 → pulih → ulangi sekali dan kembalikan jawaban ulangan", async () => {
    const jawaban = [
      res(401, { message: "Unauthorized" }),
      res(200, { ok: true }),
    ];
    const url: string[] = [];
    const hasil = await fetchDenganPemulihan(
      async (u) => {
        url.push(u);
        return jawaban.shift()!;
      },
      siap,
      "/api/v1/harga",
    );
    assert.equal(hasil.status, 200);
    assert.deepEqual(url, ["/api/v1/harga", "/api/v1/harga"]);
  });

  it("hanya mengulang sekali: 401 kedua dikembalikan apa adanya (tanpa loop)", async () => {
    let n = 0;
    let pulihN = 0;
    const hasil = await fetchDenganPemulihan(
      async () => {
        n += 1;
        return res(401, { message: "Unauthorized" });
      },
      async () => {
        pulihN += 1;
        return "ok";
      },
      "/api/v1/x",
    );
    assert.equal(hasil.status, 401);
    assert.equal(n, 2);
    assert.equal(pulihN, 1);
  });

  it("pemulihan berakhir atau gagal → jawaban 401 asli dikembalikan, permintaan tidak diulang", async () => {
    for (const hasilPulih of ["berakhir", "gagal"] as const) {
      let n = 0;
      const hasil = await fetchDenganPemulihan(
        async () => {
          n += 1;
          return res(401, { message: "Unauthorized" });
        },
        async () => hasilPulih,
        "/api/v1/x",
      );
      assert.equal(hasil.status, 401);
      assert.equal(n, 1, hasilPulih);
      // Badan asli tetap dapat dibaca pemanggil (pesanGalat memakai clone).
      assert.deepEqual(await hasil.json(), { message: "Unauthorized" });
    }
  });

  it("401 karena kata sandi salah atau alur masuk tidak memicu pemulihan", async () => {
    let pulihN = 0;
    const pulih = async () => {
      pulihN += 1;
      return "ok" as const;
    };
    const a = await fetchDenganPemulihan(
      async () => res(401, { message: "Password lama salah" }),
      pulih,
      "/api/auth/change-password",
    );
    const b = await fetchDenganPemulihan(
      async () => res(401, { message: "Email atau password salah" }),
      pulih,
      "/api/auth/login",
    );
    assert.equal(a.status, 401);
    assert.equal(b.status, 401);
    assert.equal(pulihN, 0);
  });

  it("init berupa fungsi dihitung ulang saat mengulang (token CSRF baru)", async () => {
    let csrf = "lama";
    const dikirim: (string | null)[] = [];
    const jawaban = [res(401, {}), res(200, {})];
    await fetchDenganPemulihan(
      async (_u, init) => {
        dikirim.push(new Headers(init?.headers).get("x-csrf-token"));
        return jawaban.shift()!;
      },
      async () => {
        csrf = "baru"; // refresh menerbitkan cookie CSRF baru
        return "ok";
      },
      "/api/pengguna",
      () => ({ headers: { "x-csrf-token": csrf } }),
    );
    assert.deepEqual(dikirim, ["lama", "baru"]);
  });

  it("memberi waktu mulai permintaan ke pemulih", async () => {
    let sejak: number | undefined;
    const sebelum = Date.now();
    await fetchDenganPemulihan(
      async () => res(401, {}),
      async (s) => {
        sejak = s;
        return "gagal";
      },
      "/api/v1/x",
    );
    assert.ok(sejak !== undefined && sejak >= sebelum && sejak <= Date.now());
  });
});

describe("tujuanSetelahBerakhir", () => {
  it("membawa halaman semula beserta query di lanjut", () => {
    assert.equal(
      tujuanSetelahBerakhir("/ews", "?id=EWS-1"),
      "/masuk?lanjut=%2Fews%3Fid%3DEWS-1",
    );
    assert.equal(
      tujuanSetelahBerakhir("/pengguna"),
      "/masuk?lanjut=%2Fpengguna",
    );
  });

  it("tidak mengalihkan bila sudah di halaman auth publik", () => {
    for (const p of [
      "/masuk",
      "/lupa-password",
      "/reset-password",
      "/masuk/mfa",
    ])
      assert.equal(tujuanSetelahBerakhir(p), null, p);
  });

  it("halaman terproteksi di bawah auth (mfa, ganti-password, keamanan) tetap dialihkan", () => {
    assert.equal(
      tujuanSetelahBerakhir("/keamanan"),
      "/masuk?lanjut=%2Fkeamanan",
    );
    assert.equal(tujuanSetelahBerakhir("/mfa"), "/masuk?lanjut=%2Fmfa");
  });
});
