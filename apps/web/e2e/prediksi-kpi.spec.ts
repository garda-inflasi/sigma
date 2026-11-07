/**
 * Prediksi (ekspor CSV, segarkan proyeksi, evaluasi akurasi) dan riwayat KPI.
 *
 * Penyegaran proyeksi hanya berlaku untuk model produksi hasil pelatihan, sedangkan seed demo
 * hanya punya model produksi berupa data demo. `beforeAll` melatih model untuk CBM-KRT lewat
 * layanan ML dan mempromosikannya; `afterAll` mengembalikan model demo sebagai produksi dan
 * menghapus model uji beserta proyeksinya, sehingga suite bisa diulang tanpa mengubah data demo.
 * Bila CBM-KRT sudah punya model produksi hasil pelatihan, model itu dipakai apa adanya.
 */
import { readFileSync } from "node:fs";
import type { APIRequestContext } from "@playwright/test";
import {
  apiSebagai,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  test,
  tungguTenang,
} from "./fixtures";

const KOMODITAS = "CBM-KRT";
const KOLOM_CSV =
  "komoditas_kode;komoditas_nama;kabupaten_kode;horizon_hari;tanggal;aktual;proyeksi;bawah;atas;model_versi;sumber";

type Model = {
  /** ID BIGINT model sebagai string desimal; `kode` adalah kode tampilan (MDL-<komoditas>-<versi>). */
  id: string;
  kode: string | null;
  komoditas_kode: string;
  versi: string;
  status: string;
  sumber: string;
};
type Amplop<T> = { data: T };

async function registri(api: APIRequestContext): Promise<Model[]> {
  const res = await api.get("prediksi/registri");
  expect(res.status()).toBe(200);
  return ((await res.json()) as Amplop<Model[]>).data;
}

/** Keadaan yang dibuat beforeAll dan harus dibatalkan afterAll. */
let disiapkan: { lama: Model; baru: Model; mulai: string } | null = null;

test.describe.configure({ mode: "serial" });

test.describe("prediksi dan KPI", () => {
  test.beforeAll(async () => {
    const api = await apiSebagai("manajerData");
    try {
      const status = await api.get("prediksi/ml");
      const ml = ((await status.json()) as Amplop<{ tersedia: boolean }>).data;
      expect(
        ml.tersedia,
        "layanan ML harus berjalan di stack demo (service ml)",
      ).toBe(true);

      const produksi = (await registri(api)).find(
        (m) => m.komoditas_kode === KOMODITAS && m.status === "PRODUKSI",
      );
      expect(produksi, `${KOMODITAS} punya model produksi`).toBeTruthy();
      if (produksi!.sumber === "MODEL") return; // sudah hasil pelatihan

      const mulai = jalankanSql("SELECT now()");
      const latih = await api.post("prediksi/latih", {
        data: { komoditas_kode: KOMODITAS },
      });
      expect(latih.status(), await latih.text()).toBe(201);
      const baru = ((await latih.json()) as Amplop<Model>).data;
      disiapkan = { lama: produksi!, baru, mulai };

      const promosi = await api.post("prediksi/registri/bulk", {
        data: { action: "promosi", ids: [baru.id] },
      });
      expect(promosi.status()).toBe(200);
      const hasil = ((await promosi.json()) as Amplop<{ berhasil: number }>)
        .data;
      if (hasil.berhasil !== 1) {
        // Gerbang baseline menolak (model uji lebih buruk dari baseline naif pada data demo):
        // itu aturan produk, bukan yang diuji di sini, jadi jadikan produksi langsung.
        jalankanSql(`
          UPDATE model_registry SET status = 'ARSIP' WHERE id = '${produksi!.id}';
          UPDATE model_registry SET status = 'PRODUKSI', deployed_at = now() WHERE id = '${baru.id}';`);
      }
    } finally {
      await api.dispose();
    }
  });

  test.afterAll(async () => {
    if (!disiapkan) return;
    const { lama, baru, mulai } = disiapkan;
    const api = await apiSebagai("manajerData");
    try {
      const res = await api.post("prediksi/registri/bulk", {
        data: { action: "promosi", ids: [lama.id] },
      });
      const berhasil =
        res.status() === 200
          ? ((await res.json()) as Amplop<{ berhasil: number }>).data.berhasil
          : 0;
      if (berhasil !== 1) {
        jalankanSql(`
          UPDATE model_registry SET status = 'ARSIP' WHERE id = '${baru.id}';
          UPDATE model_registry SET status = 'PRODUKSI', deployed_at = now() WHERE id = '${lama.id}';`);
      }
    } finally {
      await api.dispose();
    }
    // Model uji tidak punya endpoint hapus; proyeksi dan evaluasinya ikut dihapus (FK cascade).
    jalankanSql(`
      DELETE FROM model_promosi
       WHERE model_id = '${baru.id}' OR model_digantikan_id = '${baru.id}'
          OR (model_id = '${lama.id}' AND waktu >= '${mulai}');
      DELETE FROM prediksi_harga WHERE model_id = '${baru.id}';
      DELETE FROM model_registry WHERE id = '${baru.id}';`);
    disiapkan = null;
  });

  test.describe("API", () => {
    test("export proyeksi: 200 text/csv dengan baris header; kode tak dikenal 404", async () => {
      const api = await apiSebagai("strategisFmipa");
      const res = await api.get(`prediksi/${KOMODITAS}/export`);
      expect(res.status()).toBe(200);
      expect(res.headers()["content-type"]).toContain("text/csv");
      expect(res.headers()["content-disposition"]).toContain(
        `proyeksi_${KOMODITAS}.csv`,
      );
      const baris = (await res.text())
        .replace(/^\uFEFF/, "")
        .trim()
        .split("\n");
      expect(baris[0]).toBe(KOLOM_CSV);
      expect(baris.length).toBeGreaterThan(1);

      expect((await api.get("prediksi/XXX-TAK-ADA/export")).status()).toBe(404);
      await api.dispose();
    });

    test("segarkan dan evaluasi: 403 tanpa model.kelola, 404 komoditas tanpa model", async () => {
      const fmipa = await apiSebagai("strategisFmipa");
      const segarkan = await fmipa.post("prediksi/segarkan", {
        data: { komoditas_kode: KOMODITAS },
      });
      expect(segarkan.status()).toBe(403);
      expect((await fmipa.post("prediksi/evaluasi")).status()).toBe(403);
      await fmipa.dispose();

      const data = await apiSebagai("manajerData");
      const tanpaModel = await data.post("prediksi/segarkan", {
        data: { komoditas_kode: "XXX-TAK-ADA" },
      });
      expect(tanpaModel.status()).toBe(404);
      await data.dispose();
    });

    test("segarkan dan evaluasi idempoten bila diulang", async () => {
      const api = await apiSebagai("manajerData");
      const jalankan = async () => {
        const res = await api.post("prediksi/segarkan", {
          data: { komoditas_kode: KOMODITAS },
        });
        expect(res.status(), await res.text()).toBe(200);
        return ((await res.json()) as Amplop<{ proyeksi: number }>).data
          .proyeksi;
      };
      const pertama = await jalankan();
      expect(pertama).toBeGreaterThan(0);
      expect(await jalankan()).toBe(pertama);

      const evaluasi = async () => {
        const res = await api.post("prediksi/evaluasi");
        expect(res.status()).toBe(200);
        return ((await res.json()) as Amplop<{ dievaluasi: number }>).data
          .dievaluasi;
      };
      await evaluasi();
      expect(await evaluasi(), "evaluasi kedua tak punya yang baru").toBe(0);
      await api.dispose();
    });

    test("riwayat KPI: 200 dengan kpi dan riwayat; kode tak dikenal 404", async () => {
      const api = await apiSebagai("strategisFmipa");
      const daftar = await api.get("kpi");
      expect(daftar.status()).toBe(200);
      const kode = ((await daftar.json()) as Amplop<{ kode: string }[]>).data[0]
        .kode;

      const res = await api.get(`kpi/${kode}/riwayat`);
      expect(res.status()).toBe(200);
      const isi = (
        (await res.json()) as Amplop<{
          kpi: { kode: string };
          riwayat: unknown[];
        }>
      ).data;
      expect(isi.kpi.kode).toBe(kode);
      expect(Array.isArray(isi.riwayat)).toBe(true);

      expect((await api.get("kpi/TAK-ADA/riwayat")).status()).toBe(404);
      await api.dispose();
    });
  });

  test.describe("UI manajer.data (model.kelola)", () => {
    test("Unduh CSV mengunduh proyeksi komoditas terpilih", async ({
      page,
    }) => {
      await masukSebagai(page, "manajerData");
      await buka(page, `/prediksi?komoditas=${KOMODITAS}`);
      const unduh = page.getByRole("button", {
        name: "Unduh CSV",
        exact: true,
      });
      await expect(unduh).toBeVisible();

      const [berkas] = await Promise.all([
        page.waitForEvent("download"),
        unduh.click(),
      ]);

      expect(berkas.suggestedFilename()).toBe(`proyeksi_${KOMODITAS}.csv`);
      const isi = readFileSync((await berkas.path())!, "utf8");
      const baris = isi
        .replace(/^\uFEFF/, "")
        .trim()
        .split("\n");
      expect(baris[0]).toBe(KOLOM_CSV);
      expect(baris.length).toBeGreaterThan(1);
    });

    test("Segarkan proyeksi menampilkan ringkasan hasil, dua kali berturut-turut", async ({
      page,
    }) => {
      await masukSebagai(page, "manajerData");
      await buka(page, `/prediksi?komoditas=${KOMODITAS}`);
      const tombol = page.getByRole("button", { name: "Segarkan proyeksi" });
      await expect(tombol).toBeEnabled();

      await tombol.click();
      const hasil = page.getByText(
        /Hasil penyegaran: \d.* proyeksi ditulis ulang/,
      );
      await expect(hasil).toBeVisible();
      const pertama = await hasil.innerText();

      await expect(tombol).toBeEnabled();
      await tombol.click();
      await expect(tombol).toBeEnabled();
      await expect(hasil).toHaveText(pertama);
      await tungguTenang(page);
    });

    test("Evaluasi akurasi menampilkan hasil yang terbaca", async ({
      page,
    }) => {
      await masukSebagai(page, "manajerData");
      await buka(page, `/prediksi?komoditas=${KOMODITAS}`);
      const tombol = page.getByRole("button", { name: "Evaluasi akurasi" });
      await expect(tombol).toBeEnabled();

      await tombol.click();

      await expect(
        page.getByText(
          /Hasil evaluasi: (\d.* proyeksi jatuh tempo dievaluasi|Tidak ada proyeksi baru yang bisa dievaluasi)/,
        ),
      ).toBeVisible();
      await tungguTenang(page);
    });

    test("klik indikator di /kpi membuka riwayat bernilai periode, realisasi, target, status", async ({
      page,
    }) => {
      const api = await apiSebagai("manajerData");
      const daftar = (
        (await (await api.get("kpi")).json()) as Amplop<
          { nama: string; nilai: unknown }[]
        >
      ).data;
      await api.dispose();
      const berNilai = daftar.find((k) => k.nilai !== null);
      expect(berNilai, "demo punya KPI yang sudah terhitung").toBeTruthy();

      await masukSebagai(page, "manajerData");
      await buka(page, "/kpi");
      await page
        .getByRole("button", {
          name: `Riwayat ${berNilai!.nama}`,
          exact: true,
        })
        .click();

      const lembar = page.getByRole("dialog");
      await expect(
        lembar.getByRole("heading", { name: berNilai!.nama }),
      ).toBeVisible();
      for (const kolom of ["Periode", "Realisasi", "Target", "Status"]) {
        await expect(
          lembar.getByRole("columnheader", { name: kolom }),
        ).toBeVisible();
      }
      // header + sedikitnya satu periode
      await expect
        .poll(() => lembar.getByRole("row").count())
        .toBeGreaterThan(1);
    });

    test("indikator yang menunggu data menampilkan keadaan kosong, bukan galat", async ({
      page,
    }) => {
      const api = await apiSebagai("manajerData");
      const daftar = (
        (await (await api.get("kpi")).json()) as Amplop<
          { nama: string; nilai: unknown }[]
        >
      ).data;
      await api.dispose();
      const menunggu = daftar.find((k) => k.nilai === null);
      test.skip(!menunggu, "semua KPI demo sudah punya nilai");

      await masukSebagai(page, "manajerData");
      await buka(page, "/kpi");
      await page
        .getByRole("button", {
          name: `Riwayat ${menunggu!.nama}`,
          exact: true,
        })
        .click();

      await expect(
        page.getByRole("dialog").getByText("Belum ada nilai tersimpan"),
      ).toBeVisible();
    });
  });

  test.describe("UI strategis.fmipa (prediksi.lihat tanpa model.kelola)", () => {
    test("/prediksi: tombol segarkan dan evaluasi tersembunyi, CSV tetap tersedia", async ({
      page,
    }) => {
      await masukSebagai(page, "strategisFmipa");
      await buka(page, `/prediksi?komoditas=${KOMODITAS}`);
      const unduh = page.getByRole("button", {
        name: "Unduh CSV",
        exact: true,
      });
      await expect(unduh).toBeVisible();
      await tungguTenang(page);

      await expect(
        page.getByRole("button", { name: "Segarkan proyeksi" }),
      ).toHaveCount(0);
      await expect(
        page.getByRole("button", { name: "Evaluasi akurasi" }),
      ).toHaveCount(0);

      const [berkas] = await Promise.all([
        page.waitForEvent("download"),
        unduh.click(),
      ]);
      expect(berkas.suggestedFilename()).toBe(`proyeksi_${KOMODITAS}.csv`);
    });

    test("/eksekutif: panel XAI menawarkan Unduh CSV", async ({ page }) => {
      await masukSebagai(page, "strategisFmipa");
      await buka(page, "/eksekutif");
      const unduh = page.getByRole("button", {
        name: "Unduh CSV",
        exact: true,
      });
      await expect(unduh).toBeVisible();

      const [berkas] = await Promise.all([
        page.waitForEvent("download"),
        unduh.click(),
      ]);

      expect(berkas.suggestedFilename()).toBe(`proyeksi_${KOMODITAS}.csv`);
      const isi = readFileSync((await berkas.path())!, "utf8");
      expect(isi.replace(/^\uFEFF/, "")).toContain(KOLOM_CSV);
    });

    test("/kpi: riwayat dapat dibuka tanpa izin tambahan", async ({ page }) => {
      const api = await apiSebagai("strategisFmipa");
      const res = await api.get("kpi");
      expect(res.status()).toBe(200);
      const daftar = (
        (await res.json()) as Amplop<{ nama: string; nilai: unknown }[]>
      ).data;
      await api.dispose();
      const berNilai = daftar.find((k) => k.nilai !== null);
      test.skip(!berNilai, "belum ada KPI bernilai");

      await masukSebagai(page, "strategisFmipa");
      await buka(page, "/kpi");
      await page
        .getByRole("button", {
          name: `Riwayat ${berNilai!.nama}`,
          exact: true,
        })
        .click();
      await expect(
        page.getByRole("dialog").getByRole("row").nth(1),
      ).toBeVisible();
    });
  });
});
