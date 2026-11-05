/**
 * /peta: neraca pasokan, pencarian lokasi (geolokasi dan klik peta), riwayat optimasi rute,
 * serta pemeriksaan API spasial (neraca, lokasi, riwayat) menurut izin.
 *
 * Asumsi data seed: seeder `spasial.demo` (neraca periode terbaru untuk CBM-KRT dan BRS-MED di
 * 23 kabupaten/kota, poligon wilayah) dan akun demo: manajer.data (spasial.lihat), manajer.program
 * (spasial.lihat + logistik.kelola), superadmin (ADMIN_SISTEM, tanpa spasial.lihat).
 *
 * Uji simpan optimasi memakai Tomat (TMT) dan menghapus baris yang dibuatnya (optimasi_rute dan
 * rute_distribusi tidak punya endpoint hapus); baris audit OPTIMASI_RUTE bersifat tambah-saja.
 */
import {
  apiSebagai,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  test,
  tungguTenang,
} from "./fixtures";

test.describe.configure({ mode: "serial" });

type Amplop<T> = { data: T };
type Neraca = {
  komoditas_kode: string;
  komoditas_nama: string;
  kabupaten_kode: string;
  wilayah: string;
  periode: string;
  surplus_defisit_ton: number;
  delta_harga_pct: number;
  sumber: string;
};
type Lokasi = {
  kode: string;
  nama: string;
  di_dalam: boolean;
  jarak_pusat_km: number;
};
type Rencana = { id: string; komoditas_kode: string; komoditas_nama: string };

test.describe("neraca pasokan", () => {
  test("tabel terisi dan filter komoditas mengganti barisnya", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/peta");
    const panel = page.getByRole("region", { name: "Neraca pasokan" });
    await expect(panel).toBeVisible();

    // Bawaan: Cabai merah keriting.
    await expect(
      panel.getByRole("row", { name: /Cabai merah keriting/ }).first(),
    ).toBeVisible();
    await expect(panel.getByText("Data demo").first()).toBeVisible();
    await expect(panel.getByText(/^(Surplus|Defisit)$/).first()).toBeVisible();

    await panel.getByRole("combobox", { name: "Komoditas neraca" }).click();
    await page
      .getByRole("option", { name: "Beras medium", exact: true })
      .click();

    await expect(
      panel.getByRole("row", { name: /Beras medium/ }).first(),
    ).toBeVisible();
    await expect(
      panel.getByRole("row", { name: /Cabai merah keriting/ }),
    ).toHaveCount(0);
  });
});

test.describe("lokasi saya", () => {
  test("geolokasi Kota Banda Aceh menampilkan kabupaten/kota yang benar", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await page.context().grantPermissions(["geolocation"]);
    await page
      .context()
      .setGeolocation({ latitude: 5.5483, longitude: 95.3238 });
    await buka(page, "/peta");

    await page.getByRole("button", { name: "Lokasi saya" }).click();

    const hasil = page
      .getByRole("status")
      .filter({ hasText: "Kota Banda Aceh" });
    await expect(hasil).toBeVisible();
    await expect(hasil).toContainText(
      /di dalam batas \(perkiraan\)|terdekat, .* km dari pusat/,
    );
    await expect(hasil).toContainText("5.5483, 95.3238");
  });

  test("izin lokasi ditolak menampilkan pesan, bukan hasil", async ({
    page,
    galat,
  }) => {
    // Peramban dapat mencatat penolakan izin sebagai pesan konsol.
    galat.izinkan(/geolocation/i);
    await masukSebagai(page, "manajerData");
    await page.context().clearPermissions();
    await buka(page, "/peta");

    await page.getByRole("button", { name: "Lokasi saya" }).click();

    await expect(page.getByText(/Izin lokasi ditolak/)).toBeVisible();
  });

  test("klik peta mencari wilayah untuk titik itu", async ({ page }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/peta");
    await tungguTenang(page);
    // Peta skema SVG (tanpa token Mapbox) atau kanvas Mapbox; keduanya meneruskan klik sebagai koordinat.
    const skema = page.getByLabel("Peta skema Aceh");
    const peta = (await skema.count()) > 0 ? skema : page.locator("canvas.mapboxgl-canvas");
    await expect(peta).toBeVisible();

    // Pojok barat laut (laut) agar tidak mengenai marker; wilayah terdekat tetap dikembalikan.
    const box = (await peta.boundingBox())!;
    await peta.click({ position: { x: box.width * 0.08, y: box.height * 0.12 } });

    const koordinat = /\((-?\d+\.\d{4}), (-?\d+\.\d{4})\)/;
    const status = page.getByRole("status").filter({ hasText: koordinat });
    await expect(status).toBeVisible();
    const [, lat, lng] = (await status.innerText()).match(koordinat)!;

    const api = await apiSebagai("manajerData");
    const res = await api.get(`spasial/lokasi?lat=${lat}&lng=${lng}`);
    expect(res.status()).toBe(200);
    const nama = ((await res.json()) as Amplop<Lokasi>).data.nama;
    await api.dispose();

    await expect(status).toContainText(new RegExp(nama.replace(/^(KOTA|KABUPATEN) /i, ""), "i"));
  });
});

test.describe("riwayat optimasi", () => {
  test("manajer data melihat riwayat tetapi tidak dapat menyimpan", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/peta");
    const riwayat = page.getByLabel("Riwayat optimasi");
    await expect(riwayat).toBeVisible();
    await expect(
      riwayat.getByRole("heading", { name: "Riwayat optimasi" }),
    ).toBeVisible();
    await tungguTenang(page);
    await expect(
      riwayat
        .getByRole("listitem")
        .first()
        .or(riwayat.getByText(/Belum ada rencana yang disimpan/)),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Simpan sebagai usulan" }),
    ).toHaveCount(0);
  });

  test("menyimpan rencana menambah satu entri riwayat", async ({ page }) => {
    const api = await apiSebagai("manajerProgram");
    const idRiwayat = async () =>
      (
        (await (await api.get("spasial/optimasi-rute")).json()) as Amplop<
          Rencana[]
        >
      ).data.map((r) => r.id);
    const sebelum = new Set(await idRiwayat());

    try {
      await masukSebagai(page, "manajerProgram");
      await buka(page, "/peta");
      const panel = page.getByRole("region", { name: "Optimasi rute" });
      await panel.getByRole("combobox", { name: "Komoditas optimasi" }).click();
      await page.getByRole("option", { name: "Tomat", exact: true }).click();

      await panel.getByRole("button", { name: "Hitung rute optimal" }).click();
      await expect(panel.getByText(/tersalur lewat/)).toBeVisible();

      await panel
        .getByRole("button", { name: "Simpan sebagai usulan" })
        .click();
      await expect(panel.getByText(/Disimpan sebagai/)).toBeVisible();

      const riwayat = page.getByLabel("Riwayat optimasi");
      const entri = riwayat
        .getByRole("listitem")
        .filter({ hasText: "Tomat" })
        .first();
      await expect(entri).toBeVisible();
      await expect(entri).toContainText(/oleh .+,/);

      const baru = (await idRiwayat()).filter((id) => !sebelum.has(id));
      expect(baru).toHaveLength(1);
    } finally {
      // Bersihkan hanya baris yang dibuat tes ini.
      const baru = (await idRiwayat()).filter((id) => !sebelum.has(id));
      for (const id of baru) {
        if (!/^[\w-]+$/.test(id)) throw new Error(`id tak terduga: ${id}`);
        jalankanSql(
          `DELETE FROM rute_distribusi WHERE optimasi_id = '${id}'; DELETE FROM optimasi_rute WHERE id = '${id}';`,
        );
      }
      await api.dispose();
    }
  });
});

test.describe("API spasial", () => {
  test("neraca 200 dengan kolom yang diharapkan dan filter komoditas", async () => {
    const api = await apiSebagai("manajerData");
    const res = await api.get("spasial/neraca?komoditas=CBM-KRT");
    expect(res.status()).toBe(200);
    const rows = ((await res.json()) as Amplop<Neraca[]>).data;
    expect(rows.length).toBeGreaterThan(0);
    for (const n of rows) {
      expect(n.komoditas_kode).toBe("CBM-KRT");
      expect(typeof n.surplus_defisit_ton).toBe("number");
      expect(typeof n.delta_harga_pct).toBe("number");
      expect(n.wilayah).toBeTruthy();
      expect(n.kabupaten_kode).toBeTruthy();
      expect(n.periode).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(n.sumber).toBeTruthy();
    }
    expect((await api.get("spasial/neraca?periode=kemarin")).status()).toBe(
      400,
    );
    await api.dispose();
  });

  test("lokasi: 400 tanpa lat, 200 untuk Kota Banda Aceh", async () => {
    const api = await apiSebagai("manajerData");
    expect((await api.get("spasial/lokasi?lng=95.3")).status()).toBe(400);
    expect((await api.get("spasial/lokasi")).status()).toBe(400);
    const res = await api.get("spasial/lokasi?lat=5.5483&lng=95.3238");
    expect(res.status()).toBe(200);
    const l = ((await res.json()) as Amplop<Lokasi>).data;
    expect(l.kode).toBe("1171");
    expect(l.nama).toMatch(/BANDA ACEH/i);
    expect(typeof l.di_dalam).toBe("boolean");
    expect(l.jarak_pusat_km).toBeGreaterThanOrEqual(0);
    await api.dispose();
  });

  test("riwayat optimasi 200 berupa daftar", async () => {
    const api = await apiSebagai("manajerData");
    const res = await api.get("spasial/optimasi-rute");
    expect(res.status()).toBe(200);
    expect(Array.isArray(((await res.json()) as Amplop<unknown>).data)).toBe(
      true,
    );
    await api.dispose();
  });

  test("superadmin (tanpa spasial.lihat) mendapat 403", async () => {
    const api = await apiSebagai("superadmin");
    for (const path of [
      "spasial/neraca",
      "spasial/lokasi?lat=5.5&lng=95.3",
      "spasial/optimasi-rute",
    ]) {
      expect((await api.get(path)).status(), path).toBe(403);
    }
    await api.dispose();
  });
});
