/**
 * Cold storage: ubah fasilitas dan catat suhu manual (izin coldstorage.kelola) lewat panel detail,
 * serta penyembunyian aksi itu bagi peran yang hanya boleh melihat.
 *
 * Memakai fasilitas seed CS-03 (Sentra Hortikultura Takengon, OPERASIONAL, kapasitas 250 ton,
 * batas alarm belum diatur). Sejak migrasi 000113 `cold_storage.id` adalah UUID dan "CS-03" adalah `kode`
 * (alias yang tetap diterima API di path dan body), jadi SQL memilih baris menurut `kode`. Keadaan baris `cold_storage` dipotret lewat SQL sebelum uji dan
 * dikembalikan persis di akhir: batas suhu awalnya NULL (bawaan target ± 2 °C), yang tidak bisa
 * dipulihkan lewat API, dan pembacaan manual memperbarui kolom "pembacaan terakhir" fasilitas.
 * Pembacaan uji tidak punya endpoint hapus, jadi dihapus lewat SQL berdasarkan sumber MANUAL
 * dan suhu uji (5,3 dan 15 °C pada CS-03).
 */
import type { APIRequestContext, Page } from "@playwright/test";
import {
  apiSebagai,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  test,
  tungguTenang,
} from "./fixtures";

const ID = "CS-03";
const NAMA = "Sentra Hortikultura Takengon";
const SUHU_NORMAL = "5,3";
const SUHU_ALARM = "15";
/** Waktu tetap yang ganjil untuk pembacaan lewat API, jauh di masa lalu agar tak mengubah "terakhir". */
const WAKTU_API_1 = "2020-02-02T02:02:37Z";
const WAKTU_API_2 = "2020-02-02T02:03:37Z";

const KOLOM_SNAPSHOT = [
  "kapasitas_ton",
  "suhu_min_c",
  "suhu_maks_c",
  "status",
  "suhu_aktual_c",
  "kelembapan_aktual_pct",
  "telemetri_pada",
];

let snapshot: string[] = [];

/** Memotret kolom fasilitas sebagai literal SQL (quote_nullable) agar NULL ikut terpulihkan. */
function potretFasilitas(): string[] {
  const keluaran = jalankanSql(
    `SELECT concat_ws('|', ${KOLOM_SNAPSHOT.map((k) => `quote_nullable(${k})`).join(", ")}) FROM cold_storage WHERE kode = '${ID}'`,
  );
  const nilai = keluaran.split("|");
  expect(nilai, "kolom snapshot lengkap").toHaveLength(KOLOM_SNAPSHOT.length);
  return nilai;
}

function hapusPembacaanUji(): void {
  jalankanSql(
    `DELETE FROM telemetri_suhu WHERE fasilitas_id = (SELECT id FROM cold_storage WHERE kode = '${ID}') AND sumber = 'MANUAL' AND (abs(suhu_c - 5.3) < 1e-9 OR abs(suhu_c - 15) < 1e-9 OR abs(suhu_c - 5.37) < 1e-9 OR abs(suhu_c - 20) < 1e-9)`,
  );
}

function pulihkanFasilitas(): void {
  jalankanSql(
    `UPDATE cold_storage SET ${KOLOM_SNAPSHOT.map((k, i) => `${k} = ${snapshot[i]}`).join(", ")} WHERE kode = '${ID}'`,
  );
}

interface DetailApi {
  data: {
    fasilitas: {
      id: string;
      kode: string | null;
      kapasitas_ton: number;
      batas_min_c: number;
      batas_maks_c: number;
      status: string;
    };
    telemetri: { suhu_c: number; sumber: string }[];
  };
}

async function bacaDetail(api: APIRequestContext) {
  const res = await api.get(`cold-storage/fasilitas/${ID}`);
  expect(res.status()).toBe(200);
  return ((await res.json()) as DetailApi).data;
}

async function bukaPanel(page: Page) {
  await buka(page, "/cold-storage");
  await page.getByRole("button", { name: `Detail ${NAMA}` }).click();
  const panel = page.getByRole("dialog");
  await expect(
    panel.getByRole("region", { name: "Riwayat suhu" }),
  ).toBeVisible();
  return panel;
}

test.describe.serial("cold storage: kelola fasilitas", () => {
  test.beforeAll(() => {
    hapusPembacaanUji();
    snapshot = potretFasilitas();
  });

  test.afterAll(() => {
    hapusPembacaanUji();
    pulihkanFasilitas();
  });

  test("manajer.mutu mengubah kapasitas dan batas suhu; nilai bertahan setelah muat ulang", async ({
    page,
  }) => {
    const api = await apiSebagai("manajerMutu");
    const awal = (await bacaDetail(api)).fasilitas;
    // ID fasilitas adalah UUID; kode CS-03 tetap menjadi kode tampilan dan alias path.
    expect(awal.id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
    );
    expect(awal.kode).toBe(ID);
    const lewatUuid = await api.get(`cold-storage/fasilitas/${awal.id}`);
    expect(lewatUuid.status()).toBe(200);

    await masukSebagai(page, "manajerMutu");
    let panel = await bukaPanel(page);
    await panel.getByRole("button", { name: "Ubah fasilitas" }).click();
    let form = panel.getByRole("form", { name: "Ubah fasilitas" });
    await expect(form.getByLabel("Kapasitas (ton)")).toHaveValue(
      String(awal.kapasitas_ton),
    );

    // Batas terbalik ditolak di klien sebelum dikirim.
    await form.getByLabel("Suhu minimum (°C)").fill("10");
    await expect(form.getByRole("alert")).toContainText(
      "Suhu minimum harus lebih kecil",
    );
    await expect(
      form.getByRole("button", { name: "Simpan perubahan" }),
    ).toBeDisabled();

    await form
      .getByLabel("Kapasitas (ton)")
      .fill(String(awal.kapasitas_ton + 10));
    await form.getByLabel("Suhu minimum (°C)").fill("3");
    await form.getByLabel("Suhu maksimum (°C)").fill("9");
    await expect(form.getByRole("alert")).toHaveCount(0);
    await form.getByRole("button", { name: "Simpan perubahan" }).click();
    await expect(
      panel.getByRole("form", { name: "Ubah fasilitas" }),
    ).toHaveCount(0);

    await page.reload();
    await tungguTenang(page);
    panel = await bukaPanel(page);
    await panel.getByRole("button", { name: "Ubah fasilitas" }).click();
    form = panel.getByRole("form", { name: "Ubah fasilitas" });
    await expect(form.getByLabel("Kapasitas (ton)")).toHaveValue(
      String(awal.kapasitas_ton + 10),
    );
    await expect(form.getByLabel("Suhu minimum (°C)")).toHaveValue("3");
    await expect(form.getByLabel("Suhu maksimum (°C)")).toHaveValue("9");

    const sesudah = (await bacaDetail(api)).fasilitas;
    expect(sesudah.kapasitas_ton).toBe(awal.kapasitas_ton + 10);
    expect([sesudah.batas_min_c, sesudah.batas_maks_c]).toEqual([3, 9]);

    // Pulihkan lewat API; batas NULL asli dipulihkan dari snapshot di afterAll.
    const pulih = await api.patch(`cold-storage/fasilitas/${ID}`, {
      data: {
        kapasitas_ton: awal.kapasitas_ton,
        suhu_min_c: awal.batas_min_c,
        suhu_maks_c: awal.batas_maks_c,
      },
    });
    expect(pulih.status()).toBe(200);
    const akhir = (await bacaDetail(api)).fasilitas;
    expect(akhir.kapasitas_ton).toBe(awal.kapasitas_ton);
    expect([akhir.batas_min_c, akhir.batas_maks_c]).toEqual([
      awal.batas_min_c,
      awal.batas_maks_c,
    ]);
    await api.dispose();
  });

  test("manajer.mutu mencatat suhu manual: tampil di pembacaan terbaru dan alarm dilaporkan", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerMutu");
    const panel = await bukaPanel(page);
    await panel.getByRole("button", { name: "Catat suhu manual" }).click();
    const form = panel.getByRole("form", { name: "Catat suhu manual" });

    // Waktu pembacaan terisi bawaan (sekarang).
    await expect(form.getByLabel("Waktu pembacaan")).not.toHaveValue("");
    await expect(
      form.getByRole("button", { name: "Simpan pembacaan" }),
    ).toBeDisabled();

    await form.getByLabel("Suhu (°C)", { exact: true }).fill(SUHU_NORMAL);
    await form.getByRole("button", { name: "Simpan pembacaan" }).click();
    await expect(form.getByRole("status")).toContainText(
      "1 pembacaan tersimpan.",
    );

    const terbaru = panel.getByRole("region", { name: "Pembacaan terbaru" });
    await expect(
      terbaru
        .getByRole("listitem")
        .filter({ hasText: "Manual" })
        .filter({ hasText: `${SUHU_NORMAL}°C` }),
    ).toHaveCount(1);

    // Pembacaan di atas batas alarm (4 sampai 8 °C) dilaporkan di hasil, dengan waktu 45 detik lalu.
    const waktu = await page.evaluate(() => {
      const d = new Date(Date.now() - 45_000);
      const p = (n: number) => String(n).padStart(2, "0");
      return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
    });
    await form.getByLabel("Waktu pembacaan").fill(waktu);
    await form.getByLabel("Suhu (°C)", { exact: true }).fill(SUHU_ALARM);
    await form.getByRole("button", { name: "Simpan pembacaan" }).click();
    await expect(form.getByRole("status")).toContainText("di luar batas");
    await expect(
      terbaru
        .getByRole("listitem")
        .filter({ hasText: "Manual" })
        .filter({ hasText: `${SUHU_ALARM},0°C` })
        .getByText("Di luar batas"),
    ).toBeVisible();

    // Muat ulang: kedua pembacaan tetap ada.
    await page.reload();
    await tungguTenang(page);
    const setelah = await bukaPanel(page);
    await expect(
      setelah
        .getByRole("region", { name: "Pembacaan terbaru" })
        .getByRole("listitem")
        .filter({ hasText: "Manual" }),
    ).not.toHaveCount(0);

    const api = await apiSebagai("manajerMutu");
    const manual = (await bacaDetail(api)).telemetri.filter(
      (t) => t.sumber === "MANUAL",
    );
    expect(manual.map((t) => t.suhu_c)).toEqual(
      expect.arrayContaining([5.3, 15]),
    );
    await api.dispose();
  });

  test("API telemetri: pembacaan ganda dilaporkan duplikat dan suhu di luar batas dilaporkan sebagai alarm", async () => {
    const api = await apiSebagai("manajerMutu");
    const kirim = (waktu: string, suhu: number) =>
      api.post("cold-storage/telemetri", {
        data: {
          pembacaan: [
            { fasilitas_id: ID, waktu, suhu_c: suhu, sumber: "MANUAL" },
          ],
        },
      });

    const pertama = await kirim(WAKTU_API_1, 5.37);
    expect(pertama.status()).toBe(200);
    expect((await pertama.json()).data).toMatchObject({
      diterima: 1,
      duplikat: 0,
    });
    const ulang = await kirim(WAKTU_API_1, 5.37);
    expect((await ulang.json()).data).toMatchObject({
      diterima: 0,
      duplikat: 1,
    });

    const alarm = await kirim(WAKTU_API_2, 20);
    const hasil = (await alarm.json()).data as {
      di_luar_rentang: { suhu_c: number }[];
    };
    expect(hasil.di_luar_rentang.map((a) => a.suhu_c)).toEqual([20]);
    await api.dispose();
  });

  test("strategis.djpb (hanya melihat) tidak melihat tombol ubah dan catat suhu; API menolak 403", async ({
    page,
  }) => {
    await masukSebagai(page, "strategisDjpb");
    const panel = await bukaPanel(page);
    await expect(
      panel.getByRole("region", { name: "Pembacaan terbaru" }),
    ).toBeVisible();
    await expect(
      panel.getByRole("button", { name: "Ubah fasilitas" }),
    ).toHaveCount(0);
    await expect(
      panel.getByRole("button", { name: "Catat suhu manual" }),
    ).toHaveCount(0);

    const api = await apiSebagai("strategisDjpb");
    const ubah = await api.patch(`cold-storage/fasilitas/${ID}`, {
      data: { kapasitas_ton: 1 },
    });
    expect(ubah.status()).toBe(403);
    const catat = await api.post("cold-storage/telemetri", {
      data: {
        pembacaan: [
          {
            fasilitas_id: ID,
            waktu: "2020-02-02T02:04:37Z",
            suhu_c: 5,
            sumber: "MANUAL",
          },
        ],
      },
    });
    expect(catat.status()).toBe(403);
    await api.dispose();
  });
});
