/**
 * Endpoint detail dan riwayat yang dipakai UI: alert EWS (GET /ews/:id, /ews/:id/riwayat),
 * langganan push (DELETE), sumber data (GET /sumber-data/:id), tugas lapangan (GET /lapangan/tugas/:id),
 * dan pengguna (GET /users/:id).
 *
 * Data uji: satu alert berkode `EWS-E2E-1` dibuat lewat SQL (tidak ada endpoint buat; `id` BIGINT dibuat
 * basis data, jadi dibaca kembali lewat `kode`) dan dihapus di akhir (riwayatnya ikut terhapus); status
 * alert seed tidak pernah diubah. Rute alert menerima ID desimal dan kode lama (`EWS-…`) sebagai alias. Langganan push uji memakai
 * endpoint palsu berpenanda "e2e" dan dihapus lewat API. Tidak ada tes yang menyimpan perubahan
 * pengguna atau menjalankan ingestion.
 */
import { randomBytes, randomUUID } from "node:crypto";
import {
  apiSebagai,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  test,
  tungguTenang,
} from "./fixtures";
import { idPenggunaSeed } from "./auth-helpers";
import { cariPengguna } from "./pengguna-helpers";

const ALERT_KODE = "EWS-E2E-1";
/** ID desimal alert uji; diisi di beforeAll karena identity dibuat basis data. */
let alertId = "";
const ALERT_JUDUL = "E2E uji detail peringatan";

// UUID baris sumber SRC-BPS: garda_uuid_stabil('sumber_data', 'SRC-BPS'), sama untuk basis data yang dimigrasi dan yang di-seed.
const UUID_SUMBER_BPS = "e1f49004-97cf-3b83-9c13-15c6eb3112d6";

const unikPath =
  (path: string) => (r: { url(): string; request(): { method(): string } }) =>
    r.request().method() === "GET" && new URL(r.url()).pathname.endsWith(path);

test.describe("detail alert EWS", () => {
  test.describe.configure({ mode: "serial" });

  test.beforeAll(() => {
    jalankanSql(`
      DELETE FROM ews_alert WHERE kode = '${ALERT_KODE}';
      INSERT INTO ews_alert (kode, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, status, sumber, dibuat_pada)
      VALUES ('${ALERT_KODE}', 'INFO', 'ANOMALI_DATA', '${ALERT_JUDUL}', 'Data uji e2e, dihapus otomatis setelah spec.', '1171', 12.5, '%', 10, 'BARU', 'E2E', now());
      INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, catatan)
      SELECT id, NULL, 'BARU', NULL, 'Dibuat oleh E2E' FROM ews_alert WHERE kode = '${ALERT_KODE}';
    `);
    alertId = jalankanSql(`SELECT id FROM ews_alert WHERE kode = '${ALERT_KODE}';`);
    expect(alertId).toMatch(/^[1-9][0-9]*$/);
  });

  test.afterAll(() => {
    jalankanSql(`DELETE FROM ews_alert WHERE kode = '${ALERT_KODE}';`);
  });

  test("tautan ?id= (ID desimal) membuka detail dengan data segar dan riwayat awal", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    const detail = page.waitForResponse(unikPath(`/v1/ews/${alertId}`));
    const riwayat = page.waitForResponse(
      unikPath(`/v1/ews/${alertId}/riwayat`),
    );
    await buka(page, `/ews?id=${alertId}`);
    expect((await detail).status()).toBe(200);
    expect((await riwayat).status()).toBe(200);

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: ALERT_JUDUL }),
    ).toBeVisible();
    await expect(dialog).toContainText("Kota Banda Aceh");
    await expect(dialog).toContainText(
      "Data uji e2e, dihapus otomatis setelah spec.",
    );
    await expect(dialog.getByText("Skor dampak")).toBeVisible();

    const timeline = dialog.getByRole("list", { name: "Riwayat status" });
    await expect(timeline.getByRole("listitem")).toHaveCount(1);
    await expect(timeline).toContainText("Baru");
    await expect(timeline).toContainText("Sistem");
    await expect(timeline).toContainText("Dibuat oleh E2E");
  });

  test("tautan lama ?id=EWS-… (kode, dari email atau push) tetap membuka detail alert yang sama", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    const detail = page.waitForResponse(unikPath(`/v1/ews/${ALERT_KODE}`));
    await buka(page, `/ews?id=${ALERT_KODE}`);
    expect((await detail).status()).toBe(200);

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: ALERT_JUDUL }),
    ).toBeVisible();
    await expect(
      dialog.getByRole("list", { name: "Riwayat status" }).getByRole("listitem"),
    ).toHaveCount(1);
  });

  test("akui lewat aksi massal menambah riwayat pada detail", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/ews");
    await tungguTenang(page);
    await page
      .getByPlaceholder("Cari peringatan atau wilayah")
      .fill(ALERT_JUDUL);

    const baris = page.getByRole("row", { name: new RegExp(ALERT_JUDUL) });
    await expect(baris).toHaveCount(1);

    // Buka detail lebih dulu: riwayat awal tersimpan di cache dan harus diperbarui setelah akui.
    await baris.getByText(ALERT_JUDUL).click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog
        .getByRole("list", { name: "Riwayat status" })
        .getByRole("listitem"),
    ).toHaveCount(1);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();

    await baris.getByRole("checkbox", { name: "Pilih baris" }).click();
    const bulk = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        new URL(r.url()).pathname.endsWith("/v1/ews/bulk"),
    );
    await page
      .getByRole("toolbar")
      .getByRole("button", { name: "Akui", exact: true })
      .click();
    expect((await bulk).status()).toBe(200);
    await expect(page.getByText("Alert diakui: 1 data")).toBeVisible();

    await baris.getByText(ALERT_JUDUL).click();
    const timeline = dialog.getByRole("list", { name: "Riwayat status" });
    await expect(timeline.getByRole("listitem")).toHaveCount(2);
    await expect(timeline.getByRole("listitem").nth(1)).toContainText("Diakui");

    // Pelaku pada timeline sama dengan yang dicatat server.
    const api = await apiSebagai("manajerData");
    const server = (await (await api.get(`ews/${alertId}/riwayat`)).json())
      .data as {
      ke: string;
      oleh_nama: string | null;
    }[];
    expect(server.map((r) => r.ke)).toEqual(["BARU", "DIAKUI"]);
    expect(server[1].oleh_nama).toBeTruthy();
    await expect(timeline.getByRole("listitem").nth(1)).toContainText(
      server[1].oleh_nama!,
    );
    await expect(dialog).toContainText("Diakui");
    await api.dispose();
  });

  test("API: id tidak dikenal 404 dan peran tanpa ews.lihat 403", async () => {
    const manajer = await apiSebagai("manajerData");
    expect((await manajer.get("ews/TIDAK-ADA")).status()).toBe(404);
    expect((await manajer.get("ews/TIDAK-ADA/riwayat")).status()).toBe(404);
    expect((await manajer.get("ews/999999999999")).status()).toBe(404);
    expect((await manajer.get("ews/0")).status()).toBe(404);
    expect((await manajer.get(`ews/${alertId}`)).status()).toBe(200);
    await manajer.dispose();

    const mahasiswa = await apiSebagai("mahasiswa");
    expect((await mahasiswa.get(`ews/${alertId}`)).status()).toBe(403);
    expect((await mahasiswa.get(`ews/${alertId}/riwayat`)).status()).toBe(403);
    await mahasiswa.dispose();
  });

  test("API: ID desimal dan kode lama menunjuk alert yang sama; id berupa string desimal dan kode ikut dikirim", async () => {
    const api = await apiSebagai("manajerData");
    const lewatId = (await (await api.get(`ews/${alertId}`)).json()).data as {
      id: string;
      kode: string | null;
      judul: string;
    };
    const lewatKode = (await (await api.get(`ews/${ALERT_KODE}`)).json())
      .data as { id: string; kode: string | null };
    expect(lewatId.id).toBe(alertId);
    expect(lewatId.kode).toBe(ALERT_KODE);
    expect(lewatKode.id).toBe(alertId);
    expect(lewatId.judul).toBe(ALERT_JUDUL);

    const riwayatId = (
      await (await api.get(`ews/${alertId}/riwayat`)).json()
    ).data as { id: string }[];
    const riwayatKode = (
      await (await api.get(`ews/${ALERT_KODE}/riwayat`)).json()
    ).data as { id: string }[];
    expect(riwayatKode.map((r) => r.id)).toEqual(riwayatId.map((r) => r.id));
    for (const r of riwayatId) expect(r.id).toMatch(/^[1-9][0-9]*$/);

    // Aksi massal menolak ID bentuk lain (kode lama) dengan 400, bukan mengabaikannya diam-diam.
    const bulk = await api.post("ews/bulk", {
      data: { action: "akui", ids: [ALERT_KODE] },
    });
    expect(bulk.status()).toBe(400);
    await api.dispose();
  });
});

test.describe("langganan push", () => {
  const LANGGANAN = () => ({
    endpoint: `https://push.example.test/e2e-${randomBytes(6).toString("hex")}`,
    p256dh: "BE2E-p256dh-uji",
    auth: "e2e-auth-uji",
  });
  const PATH = "ews/notifikasi/langganan-push";

  test("API: DELETE membatalkan langganan sendiri, 404 bila tidak ada, 403 tanpa ews.akui", async () => {
    const api = await apiSebagai("manajerData");
    const langganan = LANGGANAN();
    try {
      // Belum ada: 404 (perilaku terdokumentasi), UI memperlakukannya sebagai sudah terlepas.
      expect(
        (
          await api.delete(PATH, { data: { endpoint: langganan.endpoint } })
        ).status(),
      ).toBe(404);

      expect((await api.put(PATH, { data: langganan })).status()).toBe(200);
      expect(
        (
          await api.delete(PATH, { data: { endpoint: langganan.endpoint } })
        ).status(),
      ).toBe(200);
      expect(
        (
          await api.delete(PATH, { data: { endpoint: langganan.endpoint } })
        ).status(),
      ).toBe(404);

      expect((await api.delete(PATH, { data: {} })).status()).toBe(400);
    } finally {
      await api.delete(PATH, { data: { endpoint: langganan.endpoint } });
      await api.dispose();
    }

    const mahasiswa = await apiSebagai("mahasiswa");
    expect(
      (
        await mahasiswa.delete(PATH, { data: { endpoint: langganan.endpoint } })
      ).status(),
    ).toBe(403);
    await mahasiswa.dispose();
  });

  test("API: langganan milik pengguna lain tidak bisa dibatalkan", async () => {
    const pemilik = await apiSebagai("manajerData");
    const lain = await apiSebagai("manajerProgram");
    const langganan = LANGGANAN();
    try {
      expect((await pemilik.put(PATH, { data: langganan })).status()).toBe(200);
      expect(
        (
          await lain.delete(PATH, { data: { endpoint: langganan.endpoint } })
        ).status(),
      ).toBe(404);
      expect(
        (
          await pemilik.delete(PATH, { data: { endpoint: langganan.endpoint } })
        ).status(),
      ).toBe(200);
    } finally {
      await pemilik.delete(PATH, { data: { endpoint: langganan.endpoint } });
      await pemilik.dispose();
      await lain.dispose();
    }
  });

  test("UI: mematikan kanal push tersimpan di server tanpa galat meski tak ada langganan di browser", async ({
    page,
  }) => {
    const api = await apiSebagai("manajerData");
    const awal = (await (await api.get("ews/notifikasi/preferensi")).json())
      .data.preferensi as {
      kanal: string;
      severity_min: string;
      aktif: boolean;
    }[];
    const pulihkan = () =>
      api.put("ews/notifikasi/preferensi", { data: { preferensi: awal } });

    try {
      // Mulai dari keadaan aktif agar tes deterministik.
      await api.put("ews/notifikasi/preferensi", {
        data: {
          preferensi: awal.map((p) =>
            p.kanal === "PUSH" ? { ...p, aktif: true } : p,
          ),
        },
      });

      await masukSebagai(page, "manajerData");
      await buka(page, "/ews");
      await tungguTenang(page);
      const push = page.getByRole("checkbox", {
        name: "Terima notifikasi Push (perangkat ini)",
      });
      await expect(push).toBeChecked();

      await push.click();
      await expect(
        page.getByText("Preferensi notifikasi disimpan"),
      ).toBeVisible();
      await expect(push).not.toBeChecked();
      const sesudah = (
        await (await api.get("ews/notifikasi/preferensi")).json()
      ).data.preferensi as {
        kanal: string;
        aktif: boolean;
      }[];
      expect(sesudah.find((p) => p.kanal === "PUSH")?.aktif).toBe(false);
    } finally {
      await pulihkan();
      await api.dispose();
    }
  });
});

test.describe("detail sumber data", () => {
  test("detail memuat data segar dari API (cron, URL dasar, atribusi)", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/sumber-data");
    await tungguTenang(page);
    await page.getByPlaceholder("Cari sumber atau penyedia").fill("BPS WebAPI");

    // Baris daftar membawa UUID sumber; UI memanggil detail lewat UUID itu, bukan kodenya.
    const detail = page.waitForResponse(unikPath(`/v1/sumber-data/${UUID_SUMBER_BPS}`));
    await page.getByRole("row", { name: /BPS WebAPI/ }).click();
    expect((await detail).status()).toBe(200);

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: /BPS WebAPI/ }),
    ).toBeVisible();
    await expect(
      dialog.getByRole("heading", { name: "Informasi sumber" }),
    ).toBeVisible();
    await expect(dialog.getByText("Jadwal cron (WIB)")).toBeVisible();
    // Teks untuk manusia memakai kode (SRC-BPS), bukan UUID.
    await expect(dialog.getByText("SRC-BPS", { exact: true })).toBeVisible();
    await expect(dialog).toContainText("0 6 * * *");
    await expect(dialog).toContainText("https://webapi.bps.go.id/v1/api");
    await expect(dialog).toContainText("Sumber: Badan Pusat Statistik (BPS)");
    await expect(dialog).toContainText("Kepatuhan robots.txt");
    // manajer.data memegang sumber.kelola.
    await expect(
      dialog.getByRole("button", { name: "Jalankan sekarang" }),
    ).toBeVisible();
  });

  test("peran hanya sumber.lihat tidak melihat tombol jalankan", async ({
    page,
  }) => {
    await masukSebagai(page, "strategisFmipa");
    await buka(page, "/sumber-data");
    await tungguTenang(page);
    await page.getByPlaceholder("Cari sumber atau penyedia").fill("BPS WebAPI");
    await page.getByRole("row", { name: /BPS WebAPI/ }).click();

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: "Informasi sumber" }),
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Jalankan sekarang" }),
    ).toHaveCount(0);
  });

  test("API: id tidak dikenal 404 dan tanpa sumber.lihat 403", async () => {
    const manajer = await apiSebagai("manajerData");
    expect((await manajer.get("sumber-data/TIDAK-ADA")).status()).toBe(404);
    // UUID yang sah tetapi tidak ada, dan kode lebih panjang dari 64 karakter, juga 404 (bukan 500).
    expect((await manajer.get("sumber-data/00000000-0000-3000-8000-000000000000")).status()).toBe(404);
    expect((await manajer.get(`sumber-data/${"a".repeat(65)}`)).status()).toBe(404);
    const ok = await manajer.get(`sumber-data/${UUID_SUMBER_BPS}`);
    expect(ok.status()).toBe(200);
    const row = (await ok.json()).data as Record<string, unknown>;
    expect(row).toMatchObject({
      id: UUID_SUMBER_BPS,
      kode: "SRC-BPS",
      jadwal_cron: "0 6 * * *",
      robots_txt_patuh: true,
    });
    // Kode (ID lama) tetap berlaku sebagai alias: tautan lama tidak putus.
    const alias = await manajer.get("sumber-data/SRC-BPS");
    expect(alias.status()).toBe(200);
    expect(((await alias.json()).data as { id: string }).id).toBe(UUID_SUMBER_BPS);
    // Konfigurasi sumber (bisa berisi kunci API) tidak boleh ikut keluar.
    expect(row).not.toHaveProperty("konfigurasi");
    await manajer.dispose();

    const mahasiswa = await apiSebagai("mahasiswa");
    expect((await mahasiswa.get(`sumber-data/${UUID_SUMBER_BPS}`)).status()).toBe(403);
    await mahasiswa.dispose();
  });
});

test.describe("detail tugas lapangan", () => {
  type Tugas = {
    id: string;
    kode?: string | null;
    judul: string;
    pasar_id: string;
    komoditas: string[];
    tipe: string;
    target: number;
    terisi: number;
    rincian_komoditas?: { kode: string; nama: string; terisi: boolean }[];
  };
  const JUDUL = "Verifikasi ulang harga cabai";

  test("sheet detail menampilkan komoditas dan tautan Mulai isi dengan parameter yang sama", async ({
    page,
  }) => {
    const api = await apiSebagai("mahasiswa");
    const saya = (await (await api.get("lapangan/tugas/saya")).json())
      .data as Tugas[];
    const tugas = saya.find((t) => t.judul === JUDUL);
    expect(tugas, `tugas "${JUDUL}" ada di seed demo`).toBeTruthy();
    const detail = (await (await api.get(`lapangan/tugas/${tugas!.id}`)).json())
      .data as Tugas;
    await api.dispose();
    expect(detail.rincian_komoditas?.map((r) => r.kode)).toEqual(
      tugas!.komoditas,
    );

    await masukSebagai(page, "mahasiswa");
    await buka(page, "/lapangan/tugas");
    await tungguTenang(page);

    const respons = page.waitForResponse(
      unikPath(`/v1/lapangan/tugas/${tugas!.id}`),
    );
    await page
      .getByRole("button", { name: new RegExp(`^Detail tugas ${JUDUL}`) })
      .click();
    expect((await respons).status()).toBe(200);

    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: JUDUL })).toBeVisible();
    await expect(
      dialog.getByRole("progressbar", { name: "Progres tugas" }),
    ).toBeVisible();
    await expect(dialog.getByText("Tenggat")).toBeVisible();

    const daftar = dialog.getByRole("list", { name: "Komoditas tugas" });
    await expect(daftar.getByRole("listitem")).toHaveCount(
      detail.rincian_komoditas!.length,
    );
    for (const r of detail.rincian_komoditas!) {
      const item = daftar.getByRole("listitem").filter({ hasText: r.nama });
      await expect(item).toContainText(
        r.terisi ? "Sudah diisi" : "Belum diisi",
      );
    }

    const tautan = dialog.getByRole("link", { name: "Mulai isi" });
    await expect(tautan).toHaveAttribute(
      "href",
      `/lapangan/input?pasar=${tugas!.pasar_id}&komoditas=${tugas!.komoditas.join(",")}&tipe=${tugas!.tipe}&tugas=${tugas!.kode ?? tugas!.id}`,
    );
    await tautan.click();
    await expect(page).toHaveURL(/\/lapangan\/input\?pasar=/);
  });

  test("API: ID BIGINT berupa string angka, kode lama tetap menjadi alias tautan", async () => {
    const api = await apiSebagai("mahasiswa");
    const saya = (await (await api.get("lapangan/tugas/saya")).json())
      .data as Tugas[];
    const tugas = saya.find((t) => t.judul === JUDUL);
    expect(tugas, `tugas "${JUDUL}" ada di seed demo`).toBeTruthy();
    expect(tugas!.id).toMatch(/^[1-9][0-9]*$/);
    expect(tugas!.kode, "tugas seed menyimpan ID lama sebagai kode").toMatch(
      /^TGS-/,
    );

    const lewatKode = await api.get(`lapangan/tugas/${tugas!.kode}`);
    expect(lewatKode.status()).toBe(200);
    expect(((await lewatKode.json()).data as Tugas).id).toBe(tugas!.id);
    for (const salah of ["0", "-1", "007", "9223372036854775808"]) {
      expect((await api.get(`lapangan/tugas/${salah}`)).status(), salah).toBe(
        404,
      );
    }
    await api.dispose();
  });

  test("API: tugas petugas lain 404 untuk mahasiswa, terbuka untuk tugas.kelola", async () => {
    const lain = await apiSebagai("mahasiswa.2@gardainflasi.id");
    const tugasLain = (
      (await (await lain.get("lapangan/tugas/saya")).json()).data as Tugas[]
    )[0];
    await lain.dispose();
    expect(tugasLain, "mahasiswa.2 punya tugas seed").toBeTruthy();

    const mahasiswa = await apiSebagai("mahasiswa");
    expect(
      (await mahasiswa.get(`lapangan/tugas/${tugasLain.id}`)).status(),
    ).toBe(404);
    expect((await mahasiswa.get("lapangan/tugas/TIDAK-ADA")).status()).toBe(
      404,
    );
    await mahasiswa.dispose();

    const koordinator = await apiSebagai("koordinator");
    const res = await koordinator.get(`lapangan/tugas/${tugasLain.id}`);
    expect(res.status()).toBe(200);
    expect(
      ((await res.json()).data as Tugas).rincian_komoditas?.length,
    ).toBeGreaterThan(0);
    await koordinator.dispose();

    const tanpaIzin = await apiSebagai("manajerKebijakan");
    expect(
      (await tanpaIzin.get(`lapangan/tugas/${tugasLain.id}`)).status(),
    ).toBe(403);
    await tanpaIzin.dispose();
  });
});

test.describe("detail pengguna", () => {
  const ID = idPenggunaSeed("mahasiswa_1"); // pengguna.id adalah UUID stabil dari slug seed

  test("dialog ubah memuat data segar dari GET /users/:id", async ({
    page,
  }) => {
    const api = await apiSebagai("superadmin");
    const server = (await (await api.get(`users/${ID}`)).json()).data as {
      nama: string;
      email: string;
      lembaga: string;
    };
    await api.dispose();

    await masukSebagai(page, "superadmin");
    await buka(page, "/pengguna");
    await tungguTenang(page);
    await cariPengguna(page, server.nama);

    await page
      .getByRole("button", { name: `Aksi untuk ${server.nama}`, exact: true })
      .click();
    const detail = page.waitForResponse(unikPath(`/api/pengguna/${ID}`));
    await page.getByRole("menuitem", { name: "Ubah data" }).click();
    expect((await detail).status()).toBe(200);

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: "Ubah pengguna" }),
    ).toBeVisible();
    await expect(dialog.getByLabel("Nama lengkap")).toHaveValue(server.nama);
    await expect(dialog.getByLabel("Email")).toHaveValue(server.email);
    await expect(dialog.getByLabel("Lembaga")).toHaveValue(server.lembaga);

    const rincian = dialog.locator("dl[aria-label='Rincian akun']");
    await expect(rincian).toContainText(ID);
    await expect(rincian).toContainText("Dibuat");
    await expect(rincian).toContainText("Terakhir masuk");

    // Tidak menyimpan apa pun: dialog ditutup dengan Batal.
    await dialog.getByRole("button", { name: "Batal" }).click();
    await expect(dialog).toBeHidden();
  });

  test("API: id tidak dikenal 404 dan tanpa pengguna.kelola 403", async () => {
    const admin = await apiSebagai("superadmin");
    // ID yang bukan UUID dijawab 404 tanpa menyentuh basis data, sama dengan UUID yang sah tetapi tidak ada.
    expect((await admin.get("users/TIDAK-ADA")).status()).toBe(404);
    expect((await admin.get(`users/${randomUUID()}`)).status()).toBe(404);
    const ok = await admin.get(`users/${ID}`);
    expect(ok.status()).toBe(200);
    expect(JSON.stringify(await ok.json())).not.toContain("password_hash");
    await admin.dispose();

    for (const akun of ["mahasiswa", "manajerData"] as const) {
      const api = await apiSebagai(akun);
      expect((await api.get(`users/${ID}`)).status(), akun).toBe(403);
      await api.dispose();
    }
  });
});
