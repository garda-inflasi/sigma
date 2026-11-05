import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";
import {
  adalahAsetStatis,
  bersihkanCache,
  bersihkanDataLokal,
  dipertahankanDiCache,
  hitungBelumTerkirim,
  hitungBelumTerkirimDariItems,
  POLA_NAMA_CACHE,
} from "./bersihkan";

const ORIGIN = "https://garda.example";

describe("adalahAsetStatis", () => {
  it("mengenali aset ber-hash, ikon, cangkang offline, manifest, dan berkas statis", () => {
    for (const p of [
      "/_next/static/chunks/main-abc123.js",
      "/_next/static/media/font.woff2",
      "/icons/icon-192.png",
      "/offline",
      "/manifest.webmanifest",
      "/logo.svg",
      "/styles/app.css",
    ]) {
      assert.equal(adalahAsetStatis(p), true, p);
    }
  });

  it("menolak halaman berpengguna dan rute lain", () => {
    for (const p of [
      "/lapangan/input",
      "/lapangan/riwayat",
      "/lapangan/tugas",
      "/masuk",
      "/operasional",
      "/",
      "/offline/lain",
      "/api/auth/me",
    ]) {
      assert.equal(adalahAsetStatis(p), false, p);
    }
  });
});

describe("dipertahankanDiCache", () => {
  it("mempertahankan aset statis satu origin, query diabaikan", () => {
    assert.equal(dipertahankanDiCache(`${ORIGIN}/offline`, ORIGIN), true);
    assert.equal(
      dipertahankanDiCache(`${ORIGIN}/_next/static/a.js?v=1`, ORIGIN),
      true,
    );
  });

  it("menghapus HTML navigasi (dengan atau tanpa query) walau satu origin", () => {
    assert.equal(
      dipertahankanDiCache(`${ORIGIN}/lapangan/input`, ORIGIN),
      false,
    );
    assert.equal(
      dipertahankanDiCache(
        `${ORIGIN}/lapangan/input?pasar=p1&komoditas=a,b`,
        ORIGIN,
      ),
      false,
    );
  });

  it("gagal tertutup untuk origin lain dan URL rusak", () => {
    assert.equal(
      dipertahankanDiCache("https://cdn.example/_next/static/a.js", ORIGIN),
      false,
    );
    assert.equal(dipertahankanDiCache("bukan url", ORIGIN), false);
    assert.equal(dipertahankanDiCache("", ORIGIN), false);
  });

  it("selaras dengan public/sw.js: aturan aset statis dan daftar cangkang", () => {
    const sw = readFileSync(path.join(process.cwd(), "public/sw.js"), "utf8");
    // Aturan aset statis di sw.js yang sama dengan yang dipakai pembersih.
    assert.match(sw, /\\\.\(woff2\|png\|svg\|css\|js\)\$/);
    assert.match(sw, /\/_next\/static\//);
    assert.match(sw, /\/icons\//);
    const daftarDari = (nama: string) => {
      const m = new RegExp(`const ${nama} = \\[([^\\]]+)\\]`).exec(sw);
      assert.ok(m, `${nama} tidak ditemukan di sw.js`);
      return [...m[1].matchAll(/"([^"]+)"/g)].map((x) => x[1]);
    };
    // Cangkang berpengguna (HTML dengan sesi di dalamnya) dihapus saat keluar.
    const pribadi = daftarDari("SHELL");
    assert.ok(pribadi.length > 0);
    for (const p of pribadi) {
      assert.equal(
        dipertahankanDiCache(`${ORIGIN}${p}`, ORIGIN),
        false,
        `${p} berisi data pengguna dan harus dihapus`,
      );
    }
    // Cangkang publik dipertahankan, dan sw.js mengambilnya tanpa cookie agar salinannya anonim.
    const publik = daftarDari("SHELL_PUBLIK");
    assert.ok(publik.includes("/offline"));
    for (const p of publik) {
      assert.equal(
        dipertahankanDiCache(`${ORIGIN}${p}`, ORIGIN),
        true,
        `${p} harus dipertahankan`,
      );
    }
    assert.match(sw, /fetch\(u, \{ credentials: "omit" \}\)/);
    // Nama cache sw.js cocok dengan pola pembersih.
    const versi = /const VERSI = "([^"]+)"/.exec(sw);
    assert.ok(versi && POLA_NAMA_CACHE.test(versi[1]));
  });
});

type Entri = { url: string; id: number };

/** Cache Storage palsu: nama → daftar entri; mencatat penghapusan. */
function cachePalsu(
  isi: Record<string, string[]>,
  opsi: { gagalHapus?: (url: string) => boolean } = {},
) {
  const data = new Map(
    Object.entries(isi).map(([n, urls]) => [
      n,
      urls.map((url, id) => ({ url, id }) as Entri),
    ]),
  );
  const dibuka: string[] = [];
  const dihapus: string[] = [];
  return {
    data,
    dibuka,
    dihapus,
    deps: {
      originApp: ORIGIN,
      namaCache: async () => [...data.keys()],
      buka: async (nama: string) => {
        dibuka.push(nama);
        return {
          daftar: async () => [...(data.get(nama) ?? [])],
          hapus: async (e: Entri) => {
            if (opsi.gagalHapus?.(e.url)) throw new Error("hapus gagal");
            dihapus.push(`${nama}|${e.url}`);
            data.set(
              nama,
              (data.get(nama) ?? []).filter((x) => x.id !== e.id),
            );
            return true;
          },
        };
      },
    },
  };
}

describe("bersihkanCache", () => {
  it("menghapus halaman berpengguna dan menyisakan aset statis serta cangkang offline", async () => {
    const c = cachePalsu({
      "garda-v1": [
        `${ORIGIN}/lapangan/input`,
        `${ORIGIN}/lapangan/riwayat`,
        `${ORIGIN}/masuk`,
        `${ORIGIN}/offline`,
        `${ORIGIN}/manifest.webmanifest`,
        `${ORIGIN}/_next/static/chunks/app.js`,
        `${ORIGIN}/icons/icon-192.png`,
      ],
    });

    const hasil = await bersihkanCache(c.deps);

    assert.deepEqual(hasil, { dihapus: 3, dipertahankan: 4 });
    assert.deepEqual(
      c.data.get("garda-v1")!.map((e) => e.url),
      [
        `${ORIGIN}/offline`,
        `${ORIGIN}/manifest.webmanifest`,
        `${ORIGIN}/_next/static/chunks/app.js`,
        `${ORIGIN}/icons/icon-192.png`,
      ],
    );
  });

  it("memproses semua cache garda-* dan tidak menyentuh cache pihak lain", async () => {
    const c = cachePalsu({
      "garda-v1": [`${ORIGIN}/lapangan/tugas`],
      "garda-v0": [`${ORIGIN}/lapangan/input`],
      "mapbox-tiles": ["https://api.mapbox.com/v4/tile/1.pbf"],
    });

    const hasil = await bersihkanCache(c.deps);

    assert.equal(hasil.dihapus, 2);
    assert.deepEqual(c.dibuka.sort(), ["garda-v0", "garda-v1"]);
    assert.equal(c.data.get("mapbox-tiles")!.length, 1);
  });

  it("tanpa cache sama sekali tidak melakukan apa pun", async () => {
    assert.deepEqual(await bersihkanCache(cachePalsu({}).deps), {
      dihapus: 0,
      dipertahankan: 0,
    });
  });

  it("satu entri gagal dihapus tidak menghentikan sisanya, lalu galat dilempar", async () => {
    const c = cachePalsu(
      {
        "garda-v1": [
          `${ORIGIN}/lapangan/input`,
          `${ORIGIN}/lapangan/riwayat`,
          `${ORIGIN}/masuk`,
        ],
      },
      { gagalHapus: (u) => u.endsWith("/riwayat") },
    );

    await assert.rejects(() => bersihkanCache(c.deps), /hapus gagal/);

    assert.deepEqual(
      c.data.get("garda-v1")!.map((e) => e.url),
      [`${ORIGIN}/lapangan/riwayat`],
      "entri lain tetap terhapus",
    );
  });
});

describe("bersihkanDataLokal", () => {
  it("berhenti menulis, mengosongkan memori, lalu menghapus penyimpanan", async () => {
    const urutan: string[] = [];
    const hasil = await bersihkanDataLokal({
      hentikanPersistensi: () => urutan.push("henti"),
      kosongkanMemori: () => urutan.push("memori"),
      hapusIdb: async () => {
        urutan.push("idb");
      },
      hapusCache: async () => {
        urutan.push("cache");
      },
    });
    assert.deepEqual(urutan.slice(0, 2), ["henti", "memori"]);
    assert.deepEqual(urutan.slice(2).sort(), ["cache", "idb"]);
    assert.deepEqual(hasil, { galat: [] });
  });

  it("satu langkah gagal dilaporkan tanpa menghentikan yang lain dan tanpa melempar", async () => {
    const jalan: string[] = [];
    const hasil = await bersihkanDataLokal({
      hentikanPersistensi: () => {
        throw new Error("storage rusak");
      },
      kosongkanMemori: () => jalan.push("memori"),
      hapusIdb: async () => {
        throw new Error("idb terkunci");
      },
      hapusCache: async () => {
        jalan.push("cache");
      },
    });
    assert.deepEqual(jalan.sort(), ["cache", "memori"]);
    assert.equal(hasil.galat.length, 2);
    assert.match(hasil.galat[0], /hentikan persistensi: storage rusak/);
    assert.match(hasil.galat[1], /indexeddb: idb terkunci/);
  });

  it("langkah opsional boleh dihilangkan (pembersihan saat ganti pengguna)", async () => {
    const jalan: string[] = [];
    const hasil = await bersihkanDataLokal({
      hapusIdb: async () => {
        jalan.push("idb");
      },
      hapusCache: async () => {
        jalan.push("cache");
      },
    });
    assert.deepEqual(jalan.sort(), ["cache", "idb"]);
    assert.deepEqual(hasil, { galat: [] });
  });

  it("galat yang bukan Error tetap dicatat sebagai teks", async () => {
    const hasil = await bersihkanDataLokal({
      hapusIdb: async () => Promise.reject("ditolak"),
      hapusCache: async () => undefined,
    });
    assert.deepEqual(hasil.galat, ["indexeddb: ditolak"]);
  });
});

const persist = (items: unknown) =>
  JSON.stringify({ state: { items, lastSyncAt: "x" }, version: 0 });

describe("hitungBelumTerkirim", () => {
  it("menghitung semua status selain TERSINKRON", () => {
    const mentah = persist([
      { localId: "1", status: "ANTRE" },
      { localId: "2", status: "GAGAL" },
      { localId: "3", status: "MENYINKRON" },
      { localId: "4", status: "KONFLIK" },
      { localId: "5", status: "DRAF" },
      { localId: "6", status: "TERSINKRON" },
    ]);
    assert.equal(hitungBelumTerkirim(mentah), 5);
  });

  it("antrean kosong, semua tersinkron, atau tanpa nilai bernilai nol", () => {
    assert.equal(hitungBelumTerkirim(persist([])), 0);
    assert.equal(
      hitungBelumTerkirim(
        persist([{ status: "TERSINKRON" }, { status: "TERSINKRON" }]),
      ),
      0,
    );
    assert.equal(hitungBelumTerkirim(undefined), 0);
    assert.equal(hitungBelumTerkirim(null), 0);
  });

  it("nilai rusak tidak melempar", () => {
    assert.equal(hitungBelumTerkirim("{bukan json"), 0);
    assert.equal(hitungBelumTerkirim(JSON.stringify({ state: {} })), 0);
    assert.equal(
      hitungBelumTerkirim(JSON.stringify({ state: { items: "x" } })),
      0,
    );
    assert.equal(hitungBelumTerkirim(JSON.stringify(null)), 0);
    assert.equal(hitungBelumTerkirim(42), 0);
  });

  it("item tanpa status dihitung belum terkirim, item bukan objek diabaikan", () => {
    assert.equal(hitungBelumTerkirim(persist([{ localId: "x" }, null, 7])), 1);
  });
});

describe("hitungBelumTerkirimDariItems", () => {
  it("sama dengan versi mentah untuk item di memori", () => {
    assert.equal(
      hitungBelumTerkirimDariItems([
        { status: "ANTRE" },
        { status: "TERSINKRON" },
        { status: "DRAF" },
      ]),
      2,
    );
    assert.equal(hitungBelumTerkirimDariItems([]), 0);
  });
});
