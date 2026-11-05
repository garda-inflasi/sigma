/**
 * Master komoditas: ajukan perubahan (master.ajukan) lalu setujui (master.setujui) lewat UI,
 * ditambah pemeriksaan izin dan validasi transisi lewat API.
 *
 * Memakai komoditas seed GLA (gula pasir) yang berstatus AKTIF. Arsip tidak diuji di sini karena
 * tidak ada cara memulihkannya; arsipkan dicakup tes integrasi Go. Versi naik setiap persetujuan
 * dan tidak ada endpoint untuk menurunkannya, jadi `afterAll` mengembalikannya lewat SQL.
 */
import type { APIRequestContext } from "@playwright/test";
import {
  AKUN,
  apiSebagai,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  test,
  tungguTenang,
} from "./fixtures";

const KODE = "GLA";
const ENDPOINT_BULK = "master/komoditas/bulk";

interface Komoditas {
  kode: string;
  status: string;
  versi: number;
}

async function bacaKomoditas(api: APIRequestContext): Promise<Komoditas> {
  const res = await api.get(`master/komoditas?q=${KODE}`);
  expect(res.status()).toBe(200);
  const { data } = (await res.json()) as { data: Komoditas[] };
  const ditemukan = data.find((k) => k.kode === KODE);
  expect(ditemukan, `komoditas ${KODE} ada di seed`).toBeTruthy();
  return ditemukan as Komoditas;
}

async function bulk(api: APIRequestContext, action: string, kode: string[]) {
  const res = await api.post(ENDPOINT_BULK, { data: { action, kode } });
  return {
    status: res.status(),
    body: (await res.json()) as {
      data?: {
        berhasil: number;
        gagal: number;
        rincian: { kode: string; berhasil: boolean; pesan?: string }[];
      };
    },
  };
}

test.describe.serial("master komoditas", () => {
  let versiAwal = 1;

  test.beforeAll(async () => {
    const penyetuju = await apiSebagai("strategisBps");
    const sekarang = await bacaKomoditas(penyetuju);
    // Sisa uji yang terhenti di tengah: kembalikan ke AKTIF supaya uji ini deterministik.
    if (sekarang.status === "MENUNGGU_PERSETUJUAN")
      await bulk(penyetuju, "setujui", [KODE]);
    const aktif = await bacaKomoditas(penyetuju);
    expect(aktif.status).toBe("AKTIF");
    versiAwal = aktif.versi;
    await penyetuju.dispose();
  });

  test.afterAll(async () => {
    const penyetuju = await apiSebagai("strategisBps");
    if ((await bacaKomoditas(penyetuju)).status === "MENUNGGU_PERSETUJUAN")
      await bulk(penyetuju, "setujui", [KODE]);
    await penyetuju.dispose();
    jalankanSql(
      `UPDATE komoditas SET versi = ${versiAwal}, status = 'AKTIF' WHERE kode = '${KODE}'`,
    );
  });

  test("manajer.data mengajukan perubahan dan statusnya bertahan setelah muat ulang", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/master/komoditas");
    await page.getByPlaceholder("Cari komoditas").fill(KODE);
    const baris = page.getByRole("row").filter({ hasText: KODE });
    await expect(baris).toHaveCount(1);
    await expect(baris.getByText("Aktif", { exact: true })).toBeVisible();

    await baris.getByRole("checkbox").click();
    const bilah = page.getByRole("toolbar");
    await expect(
      bilah.getByRole("button", { name: "Ajukan perubahan" }),
    ).toBeEnabled();
    // Persetujuan bukan hak peran ini.
    await expect(bilah.getByRole("button", { name: "Setujui" })).toHaveCount(0);

    await bilah.getByRole("button", { name: "Ajukan perubahan" }).click();
    await expect(
      baris.getByText("Menunggu persetujuan", { exact: true }),
    ).toBeVisible();

    await page.reload();
    await tungguTenang(page);
    await page.getByPlaceholder("Cari komoditas").fill(KODE);
    await expect(
      page
        .getByRole("row")
        .filter({ hasText: KODE })
        .getByText("Menunggu persetujuan", { exact: true }),
    ).toBeVisible();

    const penyetuju = await apiSebagai("strategisBps");
    const sesudah = await bacaKomoditas(penyetuju);
    expect(sesudah.status).toBe("MENUNGGU_PERSETUJUAN");
    expect(sesudah.versi, "versi belum naik sebelum disetujui").toBe(versiAwal);
    await penyetuju.dispose();
  });

  test("strategis.bps menyetujui: status kembali AKTIF dan versi naik satu", async ({
    page,
  }) => {
    await masukSebagai(page, "strategisBps");
    await buka(page, "/master/komoditas");
    await page.getByPlaceholder("Cari komoditas").fill(KODE);
    const baris = page.getByRole("row").filter({ hasText: KODE });
    await expect(
      baris.getByText("Menunggu persetujuan", { exact: true }),
    ).toBeVisible();

    await baris.getByRole("checkbox").click();
    const bilah = page.getByRole("toolbar");
    // Pemegang master.setujui tidak mengajukan atau mengarsipkan.
    await expect(
      bilah.getByRole("button", { name: "Ajukan perubahan" }),
    ).toHaveCount(0);
    await expect(bilah.getByRole("button", { name: "Arsipkan" })).toHaveCount(
      0,
    );
    await bilah.getByRole("button", { name: "Setujui" }).click();

    await expect(baris.getByText("Aktif", { exact: true })).toBeVisible();
    await expect(
      baris.getByText(`v${versiAwal + 1}`, { exact: true }),
    ).toBeVisible();

    await page.reload();
    await tungguTenang(page);
    await page.getByPlaceholder("Cari komoditas").fill(KODE);
    const sesudahMuat = page.getByRole("row").filter({ hasText: KODE });
    await expect(sesudahMuat.getByText("Aktif", { exact: true })).toBeVisible();
    await expect(
      sesudahMuat.getByText(`v${versiAwal + 1}`, { exact: true }),
    ).toBeVisible();

    const penyetuju = await apiSebagai("strategisBps");
    const sesudah = await bacaKomoditas(penyetuju);
    expect(sesudah.status).toBe("AKTIF");
    expect(sesudah.versi).toBe(versiAwal + 1);
    await penyetuju.dispose();
  });

  test("wilayah dan pasar tidak punya aksi status, hanya ekspor", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    for (const entitas of ["wilayah", "pasar"]) {
      await buka(page, `/master/${entitas}`);
      await page.getByRole("checkbox", { name: "Pilih baris" }).first().click();
      const bilah = page.getByRole("toolbar");
      await expect(
        bilah.getByRole("button", { name: "Ekspor CSV" }),
      ).toBeVisible();
      await expect(
        bilah.getByRole("button", { name: "Ajukan perubahan" }),
      ).toHaveCount(0);
      await expect(bilah.getByRole("button", { name: "Arsipkan" })).toHaveCount(
        0,
      );
    }
  });

  test("API: izin per aksi, aksi tidak valid, dan transisi tidak valid", async () => {
    const manajer = await apiSebagai("manajerData");
    const fmipa = await apiSebagai("strategisFmipa");
    const bps = await apiSebagai("strategisBps");

    // Pengaju tidak boleh menyetujui; penyetuju tidak boleh mengajukan atau mengarsipkan.
    expect((await bulk(manajer, "setujui", [KODE])).status).toBe(403);
    expect((await bulk(fmipa, "ajukan_perubahan", [KODE])).status).toBe(403);
    expect((await bulk(fmipa, "arsipkan", [KODE])).status).toBe(403);
    expect((await bulk(manajer, "hapus", [KODE])).status).toBe(400);
    expect((await bulk(bps, "hapus", [KODE])).status).toBe(400);

    // GLA berstatus AKTIF: persetujuan ditolak per item, begitu pula kode yang tak dikenal.
    const hasil = await bulk(bps, "setujui", [KODE, "TIDAK-ADA-E2E"]);
    expect(hasil.status).toBe(200);
    expect(hasil.body.data?.berhasil).toBe(0);
    expect(hasil.body.data?.gagal).toBe(2);
    const rincian = Object.fromEntries(
      (hasil.body.data?.rincian ?? []).map((r) => [r.kode, r]),
    );
    expect(rincian[KODE].berhasil).toBe(false);
    expect(rincian[KODE].pesan).toContain("AKTIF");
    expect(rincian["TIDAK-ADA-E2E"].pesan).toContain("tidak ditemukan");

    const sesudah = await bacaKomoditas(bps);
    expect(sesudah.status, "item yang ditolak tidak berubah").toBe("AKTIF");

    await Promise.all([manajer.dispose(), fmipa.dispose(), bps.dispose()]);
  });
});
