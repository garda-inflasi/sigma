/**
 * Keluar membersihkan data offline lapangan: antrean entri (IndexedDB) dan halaman berpengguna di Cache
 * Storage, dengan konfirmasi bila masih ada entri yang belum terkirim, serta pembersihan otomatis ketika
 * pengguna lain masuk di perangkat yang sama tanpa menekan Keluar.
 *
 * Asumsi dan aturan suite ini:
 *  - Memerlukan build produksi (stack Docker): service worker, yang menyimpan halaman `/lapangan/*` di
 *    Cache Storage, hanya didaftarkan saat `NODE_ENV=production`. Tanpa itu pra-syarat cache di bawah gagal
 *    dengan pesan yang menyebut hal ini.
 *  - TIDAK ADA entri harga yang pernah dikirim ke server. Kiriman antrean (`POST /api/v1/lapangan/harga`)
 *    dicegat di level konteks: dibatalkan (jaringan gagal) atau dijawab palsu 201 oleh tes. Entri yang
 *    dibuat tes membawa catatan berawalan `PENANDA`, dan `afterAll` memastikan tidak ada baris dengan
 *    penanda itu di `entri_harga` (bila ada, dihapus lalu tes digagalkan).
 *  - Dua akun seed lapangan dipakai: mahasiswa.1 (Aulia Rahman) dan mahasiswa.2. Keduanya masuk lewat API
 *    (`masukSebagai`), jadi anggaran laju login BFF tidak terpakai.
 *  - Tes membuka konteks browser baru, sehingga service worker, IndexedDB, dan cache selalu mulai kosong.
 */
import type { BrowserContext, Page } from "@playwright/test";
import { adalahAsetStatis } from "../src/lib/offline/bersihkan";
import { KUNCI_ANTREAN, KUNCI_PEMILIK } from "../src/lib/offline/kunci";
import {
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  test,
  tungguTenang,
  type PantauGalat,
} from "./fixtures";

test.describe.configure({ timeout: 90_000 });

const PENANDA = "e2e-logout-offline";
const PENGGUNA_KEDUA = "mahasiswa.2@gardainflasi.id";
const NAMA_PENGGUNA_PERTAMA = "Aulia Rahman";
let urutan = 0;
const penandaBaru = () => `${PENANDA} ${Date.now()}-${++urutan}`;

test.afterAll(() => {
  const ditemukan = jalankanSql(
    `SELECT count(*) FROM entri_harga WHERE catatan LIKE '${PENANDA}%'`,
  );
  if (ditemukan !== "0") {
    try {
      jalankanSql(`DELETE FROM entri_harga WHERE catatan LIKE '${PENANDA}%'`);
    } catch {
      // Dilaporkan lewat asersi di bawah; pembersihan paksa hanya upaya terbaik.
    }
  }
  expect(
    ditemukan,
    "entri e2e sampai ke server (kiriman antrean harus dicegat)",
  ).toBe("0");
});

// ── Pembantu: penyimpanan peramban ────────────────────────────────────────

interface BarisIdb {
  kunci: string;
  nilai: unknown;
}
type IsiIdb = Record<string, Record<string, BarisIdb[]>>;

/** Seluruh isi semua basis data IndexedDB origin ini: { basisData: { store: [{kunci, nilai}] } }. */
async function bacaIdb(page: Page): Promise<IsiIdb> {
  return page.evaluate(async () => {
    const minta = <T>(r: IDBRequest<T>) =>
      new Promise<T>((ok, gagal) => {
        r.onsuccess = () => ok(r.result);
        r.onerror = () => gagal(r.error);
      });
    const hasil: Record<
      string,
      Record<string, { kunci: string; nilai: unknown }[]>
    > = {};
    for (const info of await indexedDB.databases()) {
      if (!info.name) continue;
      const db = await minta(indexedDB.open(info.name));
      hasil[info.name] = {};
      for (const nama of Array.from(db.objectStoreNames)) {
        const store = db.transaction(nama, "readonly").objectStore(nama);
        const [kunci, nilai] = await Promise.all([
          minta(store.getAllKeys()),
          minta(store.getAll()),
        ]);
        hasil[info.name][nama] = kunci.map((k, i) => ({
          kunci: String(k),
          nilai: nilai[i],
        }));
      }
      db.close();
    }
    return hasil;
  });
}

const semuaBaris = (isi: IsiIdb) =>
  Object.values(isi).flatMap((stores) => Object.values(stores).flat());

/** Entri antrean lapangan yang tersimpan di IndexedDB (kosong bila kuncinya belum ada). */
async function antreanIdb(page: Page) {
  const mentah = semuaBaris(await bacaIdb(page)).find(
    (b) => b.kunci === KUNCI_ANTREAN,
  )?.nilai;
  if (typeof mentah !== "string") return [];
  return (
    JSON.parse(mentah) as {
      state: {
        items: {
          localId: string;
          status: string;
          catatan?: string;
          percobaan: number;
        }[];
      };
    }
  ).state.items;
}

async function pemilikIdb(page: Page): Promise<string | undefined> {
  const nilai = semuaBaris(await bacaIdb(page)).find(
    (b) => b.kunci === KUNCI_PEMILIK,
  )?.nilai;
  return typeof nilai === "string" ? nilai : undefined;
}

interface BarisCache {
  cache: string;
  /** Path tanpa query. */
  path: string;
  teks?: string;
}

/** Semua entri Cache Storage; `isi` membaca badan entri non-statis (untuk memeriksa nama pengguna). */
async function bacaCache(page: Page, isi = false): Promise<BarisCache[]> {
  return page.evaluate(async (denganIsi) => {
    const keluaran: { cache: string; path: string; teks?: string }[] = [];
    for (const nama of await caches.keys()) {
      const cache = await caches.open(nama);
      for (const req of await cache.keys()) {
        const { pathname } = new URL(req.url);
        const statis =
          pathname.startsWith("/_next/static/") ||
          /\.(woff2|png|svg|css|js)$/.test(pathname);
        const teks =
          denganIsi && !statis
            ? await (await cache.match(req, { ignoreVary: true }))?.text()
            : undefined;
        keluaran.push({ cache: nama, path: pathname, teks });
      }
    }
    return keluaran;
  }, isi);
}

// ── Pembantu: alur ────────────────────────────────────────────────────────

/** Galat jaringan yang wajar saat offline atau saat kiriman antrean sengaja dibatalkan. */
function izinkanGalatJaringan(galat: PantauGalat): void {
  galat.izinkan(/Failed to load resource/);
  galat.izinkan(
    /Failed to fetch|NetworkError|ERR_INTERNET_DISCONNECTED|net::ERR_/,
  );
}

interface KirimanPalsu {
  /** "gagal": koneksi dibatalkan; "sukses": dijawab 201 palsu. Tidak ada yang diteruskan ke server. */
  mode: "gagal" | "sukses";
  permintaan: {
    idempotensi: string | undefined;
    body: { catatan?: string; harga?: number };
  }[];
}

/** Mencegat kiriman antrean di level konteks (juga untuk permintaan service worker). Tidak pernah meneruskan POST. */
async function cegatKirimHarga(context: BrowserContext): Promise<KirimanPalsu> {
  const kiriman: KirimanPalsu = { mode: "gagal", permintaan: [] };
  await context.route(
    (url) => url.pathname === "/api/v1/lapangan/harga",
    async (route) => {
      const req = route.request();
      if (req.method() !== "POST") return route.fallback();
      kiriman.permintaan.push({
        idempotensi: req.headers()["idempotency-key"],
        body: req.postDataJSON() ?? {},
      });
      if (kiriman.mode === "gagal") return route.abort("failed");
      return route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          code: "201",
          message: "Created",
          data: { id: "e2e-palsu" },
        }),
      });
    },
  );
  return kiriman;
}

/** Masuk, membuka form input, dan menunggu cangkang lapangan siap (antrean dipulihkan, pemilik tercatat). */
async function bukaLapangan(
  page: Page,
  akun: string,
  path = "/lapangan/input",
): Promise<string> {
  await masukSebagai(page, akun);
  await buka(page, path);
  await expect(page.getByRole("button", { name: "Menu akun" })).toBeVisible();
  await expect
    .poll(() => pemilikIdb(page), {
      message: "pemilik data offline belum tercatat di IndexedDB",
    })
    .toBeTruthy();
  await tungguTenang(page);
  return (await pemilikIdb(page))!;
}

/** Mengisi form harga dengan harga acuan (placeholder) dan catatan penanda, hingga tombol simpan aktif. */
async function isiForm(page: Page, penanda: string): Promise<void> {
  const harga = page.getByLabel(/Harga per/);
  await harga.fill((await harga.getAttribute("placeholder"))!);
  // Catatan 10+ karakter memenuhi syarat bukti walau harga acuan server mengubah rentang wajar.
  await page.getByPlaceholder(/^Catatan: kondisi pasokan/).fill(penanda);
  const etik = page.getByRole("checkbox", { name: /Kode etik agen lapangan/ });
  await etik.click();
  await expect(etik).toBeChecked();
}

async function pilihKeluar(page: Page): Promise<void> {
  await page.getByRole("button", { name: "Menu akun" }).click();
  await page.getByRole("menuitem", { name: "Keluar" }).click();
}

/** Pra-syarat: service worker sudah menyimpan halaman lapangan milik pengguna (membuktikan pembersihannya berarti). */
async function tungguHalamanTersimpan(page: Page): Promise<void> {
  await expect
    .poll(
      async () =>
        (await bacaCache(page)).some((e) => e.path.startsWith("/lapangan/")),
      {
        timeout: 20_000,
        message:
          "service worker tidak menyimpan halaman /lapangan (spec ini butuh build produksi)",
      },
    )
    .toBe(true);
}

async function pastikanPerangkatBersih(
  page: Page,
  statisSebelum: string[],
): Promise<void> {
  const isi = await bacaIdb(page);
  expect(
    semuaBaris(isi).map((b) => b.kunci),
    "kunci IndexedDB yang tersisa sesudah keluar",
  ).toEqual([]);

  const cache = await bacaCache(page, true);
  expect(
    cache.filter((e) => e.path.startsWith("/lapangan")).map((e) => e.path),
    "halaman lapangan di Cache Storage sesudah keluar",
  ).toEqual([]);
  expect(
    cache
      .filter((e) => e.teks?.includes(NAMA_PENGGUNA_PERTAMA))
      .map((e) => e.path),
    "entri cache yang masih memuat nama pengguna sebelumnya",
  ).toEqual([]);
  // Cangkang statis tidak ikut dibuang.
  const statisSesudah = cache
    .filter((e) => adalahAsetStatis(e.path))
    .map((e) => e.path);
  for (const path of statisSebelum) expect(statisSesudah).toContain(path);
}

// ── Tes ───────────────────────────────────────────────────────────────────

test("Keluar tanpa entri tertahan langsung ke /masuk dan membersihkan perangkat", async ({
  page,
}) => {
  await bukaLapangan(page, "mahasiswa");
  await tungguHalamanTersimpan(page);
  const statisSebelum = (await bacaCache(page))
    .filter((e) => adalahAsetStatis(e.path))
    .map((e) => e.path);

  await pilihKeluar(page);

  // Dialog konfirmasi akan menahan pengalihan; tiba di /masuk berarti tidak ada entri tertahan.
  await expect(page).toHaveURL(/\/masuk$/);
  expect(
    (await page.context().cookies()).find((c) => c.name === "garda_access"),
    "cookie sesi sesudah keluar",
  ).toBeUndefined();
  await pastikanPerangkatBersih(page, statisSebelum);
});

test("Keluar dengan entri offline: dialog menyebut jumlahnya; Batal dan Kirim dulu menahan; Tetap keluar membersihkan perangkat; pengguna berikutnya tidak melihat apa pun", async ({
  page,
  context,
  galat,
}) => {
  izinkanGalatJaringan(galat);
  const kiriman = await cegatKirimHarga(context);
  const pemilikPertama = await bukaLapangan(page, "mahasiswa");
  await tungguHalamanTersimpan(page);
  const statisSebelum = (await bacaCache(page))
    .filter((e) => adalahAsetStatis(e.path))
    .map((e) => e.path);

  // 1. Entri disimpan saat offline sehingga masuk antrean (tidak ada permintaan yang keluar).
  const penanda = penandaBaru();
  await context.setOffline(true);
  await isiForm(page, penanda);
  await page
    .getByRole("button", { name: "Simpan offline", exact: true })
    .click();
  await expect(page.getByText("Entri disimpan offline")).toBeVisible();
  await expect
    .poll(async () =>
      (await antreanIdb(page)).map((e) => [e.status, e.catatan]),
    )
    .toEqual([["ANTRE", penanda]]);
  expect(kiriman.permintaan, "tidak ada kiriman selagi offline").toHaveLength(
    0,
  );

  // 2. Online lagi: mesin sinkron mencoba mengirim, tetapi kiriman dibatalkan, jadi entri tertahan (GAGAL).
  await context.setOffline(false);
  await expect.poll(() => kiriman.permintaan.length).toBeGreaterThan(0);
  await expect
    .poll(async () => (await antreanIdb(page)).map((e) => e.status))
    .toEqual(["GAGAL"]);

  // 3. Keluar: dialog menyebut jumlah entri.
  await pilihKeluar(page);
  const dialog = page.getByRole("alertdialog");
  await expect(
    dialog.getByRole("heading", { name: "Ada entri yang belum terkirim" }),
  ).toBeVisible();
  await expect(dialog).toContainText("1 entri harga belum terkirim");
  await expect(
    dialog.getByRole("button", { name: "Kirim dulu" }),
  ).toBeEnabled();
  await expect(
    dialog.getByRole("button", { name: "Tetap keluar" }),
  ).toBeEnabled();

  // 4. Batal: tetap di halaman, antrean utuh.
  await dialog.getByRole("button", { name: "Batal" }).click();
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(/\/lapangan\/input/);
  expect(await antreanIdb(page)).toHaveLength(1);

  // 5. Kirim dulu: kiriman masih gagal, dialog tetap terbuka dan menyebut sisa.
  await pilihKeluar(page);
  const sebelumKirim = kiriman.permintaan.length;
  await dialog.getByRole("button", { name: "Kirim dulu" }).click();
  await expect(
    dialog.getByText(/Masih ada 1 entri yang belum terkirim/),
  ).toBeVisible();
  expect(kiriman.permintaan.length).toBeGreaterThan(sebelumKirim);
  await expect(page).toHaveURL(/\/lapangan\/input/);
  expect(await antreanIdb(page)).toHaveLength(1);

  // 6. Tetap keluar: sesi dicabut, perangkat bersih, tiba di /masuk.
  await dialog.getByRole("button", { name: "Tetap keluar" }).click();
  await expect(page).toHaveURL(/\/masuk$/);
  expect(
    (await page.context().cookies()).find((c) => c.name === "garda_access"),
    "cookie sesi sesudah keluar",
  ).toBeUndefined();
  await pastikanPerangkatBersih(page, statisSebelum);

  // 7. Pengguna lain masuk di perangkat yang sama: tidak ada jejak pengguna pertama.
  await bukaLapangan(page, PENGGUNA_KEDUA, "/lapangan/riwayat");
  expect(await pemilikIdb(page)).not.toBe(pemilikPertama);
  expect(await antreanIdb(page)).toEqual([]);
  await expect(page.getByText("Menunggu kirim")).toHaveCount(0);
  await expect(page.getByText("Gagal kirim")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Status sinkronisasi: Tersinkron" }),
  ).toBeVisible();
  expect(
    (await bacaCache(page, true))
      .filter((e) => e.teks?.includes(NAMA_PENGGUNA_PERTAMA))
      .map((e) => e.path),
    "cache pengguna kedua masih memuat nama pengguna pertama",
  ).toEqual([]);
  expect(kiriman.permintaan.every((p) => p.body.catatan === penanda)).toBe(
    true,
  );
});

test("Kirim dulu berhasil: entri terkirim (server dipalsukan) lalu keluar tanpa kehilangan data", async ({
  page,
  context,
  galat,
}) => {
  izinkanGalatJaringan(galat);
  const kiriman = await cegatKirimHarga(context);
  await bukaLapangan(page, "mahasiswa");

  const penanda = penandaBaru();
  await context.setOffline(true);
  await isiForm(page, penanda);
  await page
    .getByRole("button", { name: "Simpan offline", exact: true })
    .click();
  await expect
    .poll(async () => (await antreanIdb(page)).map((e) => e.status))
    .toEqual(["ANTRE"]);
  await context.setOffline(false);
  await expect
    .poll(async () => (await antreanIdb(page)).map((e) => e.status))
    .toEqual(["GAGAL"]);
  const idLokal = (await antreanIdb(page))[0].localId;

  // Dari sini server "menerima" kiriman (jawaban palsu, tidak ada yang sampai ke server sungguhan).
  kiriman.mode = "sukses";
  await pilihKeluar(page);
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toContainText("1 entri harga belum terkirim");
  await dialog.getByRole("button", { name: "Kirim dulu" }).click();

  await expect(page).toHaveURL(/\/masuk$/);
  const diterima = kiriman.permintaan.at(-1)!;
  expect(diterima.idempotensi, "kunci idempotensi = id lokal entri").toBe(
    idLokal,
  );
  expect(diterima.body.catatan).toBe(penanda);
  expect(diterima.body.harga).toBeGreaterThan(0);
  expect(semuaBaris(await bacaIdb(page)).map((b) => b.kunci)).toEqual([]);
});

test("Pengguna lain masuk tanpa menekan Keluar: data offline pengguna sebelumnya dihapus", async ({
  page,
}) => {
  const pemilikPertama = await bukaLapangan(page, "mahasiswa");
  await tungguHalamanTersimpan(page);
  // Pra-syarat: halaman pengguna pertama memang tersimpan di cache (berisi namanya).
  await expect
    .poll(
      async () =>
        (await bacaCache(page, true)).some((e) =>
          e.teks?.includes(NAMA_PENGGUNA_PERTAMA),
        ),
      {
        timeout: 20_000,
        message: "cache belum memuat halaman pengguna pertama",
      },
    )
    .toBe(true);

  // Draf tersimpan di perangkat (tidak pernah dikirim sehingga tidak menyentuh server).
  const penanda = penandaBaru();
  await isiForm(page, penanda);
  await page.getByRole("button", { name: "Simpan draf" }).click();
  await expect(page.getByText("Draf disimpan di perangkat")).toBeVisible();
  await expect
    .poll(async () =>
      (await antreanIdb(page)).map((e) => [e.status, e.catatan]),
    )
    .toEqual([["DRAF", penanda]]);

  // Sesi berganti tanpa Keluar (mis. token habis lalu pengguna lain masuk): cookie ditimpa, halaman dimuat ulang.
  await masukSebagai(page, PENGGUNA_KEDUA);
  await buka(page, "/lapangan/riwayat");

  await expect(
    page.getByText("Data offline pengguna sebelumnya dihapus"),
  ).toBeVisible();
  await expect(
    page.getByText(/1 entri yang belum terkirim milik akun lain/),
  ).toBeVisible();
  await expect
    .poll(async () => await pemilikIdb(page))
    .not.toBe(pemilikPertama);
  expect(await pemilikIdb(page)).toBeTruthy();
  expect(await antreanIdb(page)).toEqual([]);
  await expect(page.getByText("Draf", { exact: true })).toHaveCount(0);
  await expect
    .poll(
      async () =>
        (await bacaCache(page, true))
          .filter((e) => e.teks?.includes(NAMA_PENGGUNA_PERTAMA))
          .map((e) => e.path),
      { message: "cache masih memuat halaman pengguna pertama" },
    )
    .toEqual([]);
});

test("Pengguna yang sama masuk lagi: antrean offline miliknya tetap ada", async ({
  page,
}) => {
  const pemilik = await bukaLapangan(page, "mahasiswa");
  const penanda = penandaBaru();
  await isiForm(page, penanda);
  await page.getByRole("button", { name: "Simpan draf" }).click();
  await expect
    .poll(async () =>
      (await antreanIdb(page)).map((e) => [e.status, e.catatan]),
    )
    .toEqual([["DRAF", penanda]]);

  // Sesi habis lalu akun yang sama masuk lagi (tanpa Keluar): draf tidak boleh hilang.
  await masukSebagai(page, "mahasiswa");
  await buka(page, "/lapangan/riwayat");
  await expect(page.getByText("Draf", { exact: true }).first()).toBeVisible();
  expect(await pemilikIdb(page)).toBe(pemilik);
  expect((await antreanIdb(page)).map((e) => e.catatan)).toEqual([penanda]);
  await expect(
    page.getByText("Data offline pengguna sebelumnya dihapus"),
  ).toHaveCount(0);
});
