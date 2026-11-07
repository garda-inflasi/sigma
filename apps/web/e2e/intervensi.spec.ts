/**
 * Kanban intervensi (/intervensi): kolom dari GET /v1/intervensi/tahap, alur persetujuan dari
 * GET /v1/intervensi/alur, dialog "Tambah intervensi" (POST /v1/intervensi) dan lembar detail dengan
 * riwayat (GET /v1/intervensi/:id/riwayat).
 *
 * Asumsi seed demo: siklus TEKANAN punya tahap pertama "EWS terpicu", kartu INT-301 punya riwayat BUAT,
 * alur `pindah-tekanan-fiskal` memiliki dua langkah, dan manajer.kebijakan memegang intervensi.kelola
 * sedangkan strategis.djpb tidak.
 *
 * Pembersihan: kartu uji dihapus lewat SQL (tidak ada endpoint hapus) berdasarkan judulnya; riwayat,
 * penanggung jawab, dan persetujuan ikut terhapus oleh ON DELETE CASCADE. Baris audit_log tidak punya FK
 * ke kartu dan bersifat append-only, jadi tetap tinggal.
 */
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

type Tahap = {
  siklus: string;
  kode_tahap: string;
  urutan: number;
  label: string;
  petunjuk: string;
  izin_dibutuhkan: string;
  sla_hari: number | null;
};

type Alur = {
  kode: string;
  nama: string;
  jenis: string;
  aktif: boolean;
  langkah: { urutan: number; label: string; izin_dibutuhkan: string }[];
};

/** Judul kartu uji; hanya karakter aman untuk SQL. */
const hapusKartuUji = (judul: string) => {
  if (!/^E2E \d+$/.test(judul))
    throw new Error(`judul uji tidak valid: ${judul}`);
  jalankanSql(`DELETE FROM intervensi WHERE judul = '${judul}'`);
};

async function ambilTahapTekanan(akun: keyof typeof AKUN): Promise<Tahap[]> {
  const api = await apiSebagai(akun);
  try {
    const res = await api.get("intervensi/tahap");
    expect(res.status()).toBe(200);
    const semua = ((await res.json()) as { data: Tahap[] }).data;
    return semua
      .filter((t) => t.siklus === "TEKANAN")
      .sort((a, b) => a.urutan - b.urutan);
  } finally {
    await api.dispose();
  }
}

test.describe("API intervensi", () => {
  test("GET /intervensi/tahap mengembalikan definisi tahap tiga siklus", async () => {
    const api = await apiSebagai(AKUN.strategisDjpb);
    const res = await api.get("intervensi/tahap");
    expect(res.status()).toBe(200);
    const data = ((await res.json()) as { data: Tahap[] }).data;
    expect(data.length).toBeGreaterThanOrEqual(16);
    expect([...new Set(data.map((t) => t.siklus))].sort()).toEqual([
      "NORMAL",
      "SURPLUS",
      "TEKANAN",
    ]);
    for (const t of data) {
      expect(t).toEqual(
        expect.objectContaining({
          kode_tahap: expect.any(String),
          label: expect.any(String),
          urutan: expect.any(Number),
          izin_dibutuhkan: expect.stringMatching(/^intervensi\./),
        }),
      );
    }
    const fiskal = data.find((t) => t.kode_tahap === "t-fiskal");
    expect(fiskal?.izin_dibutuhkan).toBe("intervensi.setujui_fiskal");
    await api.dispose();
  });

  test("GET /intervensi/alur mengembalikan rantai persetujuan berlangkah", async () => {
    const api = await apiSebagai(AKUN.strategisDjpb);
    const res = await api.get("intervensi/alur");
    expect(res.status()).toBe(200);
    const data = ((await res.json()) as { data: Alur[] }).data;
    expect(data.length).toBeGreaterThanOrEqual(5);
    for (const a of data) {
      expect(a).toEqual(
        expect.objectContaining({
          kode: expect.any(String),
          nama: expect.any(String),
          aktif: expect.any(Boolean),
        }),
      );
      expect(a.langkah.length).toBeGreaterThan(0);
    }
    const fiskal = data.find((a) => a.kode === "pindah-tekanan-fiskal");
    expect(fiskal?.langkah.map((l) => l.izin_dibutuhkan)).toEqual([
      "intervensi.setujui",
      "intervensi.setujui_fiskal",
    ]);
    await api.dispose();
  });

  test("POST /intervensi: 403 tanpa intervensi.kelola, 400 untuk badan tidak valid", async () => {
    const valid = {
      judul: "E2E tidak boleh tersimpan",
      siklus: "TEKANAN",
      prioritas: "SEDANG",
      komoditas_kode: "BRS-MED",
      kabupaten_kode: "1171",
      tenggat: "2030-01-01T00:00:00Z",
    };
    const djpb = await apiSebagai(AKUN.strategisDjpb);
    expect((await djpb.post("intervensi", { data: valid })).status()).toBe(403);
    await djpb.dispose();

    const kebijakan = await apiSebagai(AKUN.manajerKebijakan);
    expect((await kebijakan.post("intervensi", { data: {} })).status()).toBe(
      400,
    );
    expect(
      (
        await kebijakan.post("intervensi", {
          data: { ...valid, siklus: "ACAK" },
        })
      ).status(),
    ).toBe(400);
    expect(
      (
        await kebijakan.post("intervensi", { data: { ...valid, judul: "   " } })
      ).status(),
    ).toBe(400);
    expect(
      (
        await kebijakan.post("intervensi", {
          data: { ...valid, komoditas_kode: "TIDAK-ADA" },
        })
      ).status(),
    ).toBe(400);
    await kebijakan.dispose();
  });

  test("GET /intervensi/:id/riwayat: 200 untuk kartu seed (alias kode INT-301 dan UUID), 404 untuk id tak dikenal", async () => {
    const api = await apiSebagai(AKUN.strategisDjpb);
    // Alias kode lama tetap berlaku di URL.
    const ada = await api.get("intervensi/INT-301/riwayat");
    expect(ada.status()).toBe(200);
    const riwayat = (
      (await ada.json()) as {
        data: { id: string; aksi: string; ke: string | null }[];
      }
    ).data;
    expect(riwayat[0].aksi).toBe("BUAT");
    expect(riwayat[0].ke).toBe("t-ews");
    expect(riwayat[0].id).toMatch(/^[1-9][0-9]*$/);

    // id kartu adalah UUID dan kode tampilan dikirim di field `kode`; keduanya menunjuk kartu yang sama.
    const daftar = (
      (await (await api.get("intervensi")).json()) as {
        data: { id: string; kode: string | null }[];
      }
    ).data;
    const kartu = daftar.find((k) => k.kode === "INT-301");
    expect(kartu, "kartu seed INT-301").toBeDefined();
    expect(kartu!.id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
    );
    const lewatUuid = await api.get(`intervensi/${kartu!.id}/riwayat`);
    expect(lewatUuid.status()).toBe(200);
    expect(
      ((await lewatUuid.json()) as { data: { id: string }[] }).data.map(
        (r) => r.id,
      ),
    ).toEqual(riwayat.map((r) => r.id));

    expect((await api.get("intervensi/INT-0/riwayat")).status()).toBe(404);
    expect((await api.get("intervensi/bukan-uuid/riwayat")).status()).toBe(404);
    expect(
      (
        await api.get("intervensi/00000000-0000-0000-0000-000000000000/riwayat")
      ).status(),
    ).toBe(404);
    await api.dispose();
  });

  test("POST /intervensi/persetujuan/:pid/keputusan: pid bukan ID desimal dijawab 404", async () => {
    // strategis.djpb memegang intervensi.setujui_fiskal, jadi lolos pemeriksaan izin rute.
    const api = await apiSebagai(AKUN.strategisDjpb);
    expect(
      (
        await api.post("intervensi/persetujuan/bukan-angka/keputusan", {
          data: { keputusan: "SETUJU" },
        })
      ).status(),
    ).toBe(404);
    await api.dispose();
  });
});

test.describe("kanban intervensi", () => {
  test.describe.configure({ mode: "serial" });

  test("kolom papan sama dengan tahap dari API, dan alur persetujuan tampil", async ({
    page,
  }) => {
    const tahap = await ambilTahapTekanan("manajerKebijakan");
    await masukSebagai(page, "manajerKebijakan");
    await buka(page, "/intervensi?siklus=tekanan");
    const kolom = page.locator("[data-tahap-id]");
    await expect(kolom).toHaveCount(tahap.length);
    await tungguTenang(page);

    expect(
      await kolom.evaluateAll((els) =>
        els.map((e) => e.getAttribute("data-tahap-id")),
      ),
    ).toEqual(tahap.map((t) => t.kode_tahap));
    expect(await page.locator("[data-tahap-id] h3").allTextContents()).toEqual(
      tahap.map((t) => t.label),
    );

    const alur = page.getByRole("region", { name: "Alur persetujuan" });
    await expect(alur).toBeVisible();
    await expect(alur.getByText("Dukungan instrumen belanja", { exact: true })).toBeVisible();
    await expect(alur.getByText("Persetujuan Kanwil DJPb", { exact: true }).first()).toBeVisible();
  });

  test("tambah intervensi: kartu muncul di tahap pertama dan riwayat mencatat pembuatannya", async ({
    page,
  }) => {
    const judul = `E2E ${Date.now()}`;
    const tahap = await ambilTahapTekanan("manajerKebijakan");
    const pertama = tahap[0];
    // Sisa dari lari yang gagal sebelumnya; pola judul sempit agar data lain tak tersentuh.
    jalankanSql(`DELETE FROM intervensi WHERE judul ~ '^E2E [0-9]+$'`);

    try {
      await masukSebagai(page, "manajerKebijakan");
      await buka(page, "/intervensi?siklus=tekanan");
      await expect(page.locator("[data-tahap-id]")).toHaveCount(tahap.length);

      await page.getByRole("button", { name: "Tambah intervensi" }).click();
      const dialog = page.getByRole("dialog", { name: "Tambah intervensi" });
      await expect(dialog).toBeVisible();

      // Validasi sisi klien: judul kosong tidak dikirim ke server.
      await dialog.getByRole("button", { name: "Simpan intervensi" }).click();
      await expect(dialog.getByText("Judul wajib diisi")).toBeVisible();

      await dialog.getByLabel("Judul").fill(judul);
      await dialog
        .getByLabel("Deskripsi (opsional)")
        .fill("Dibuat oleh uji e2e");
      await dialog.getByLabel("Tenggat").fill("2030-01-01");
      await dialog.getByRole("button", { name: "Simpan intervensi" }).click();
      await expect(dialog).toBeHidden();

      const kolomPertama = page.getByRole("group", {
        name: `Tahap ${pertama.label}`,
        exact: true,
      });
      await expect(kolomPertama.getByText(judul)).toBeVisible();

      await page
        .getByRole("button", { name: `Buka detail ${judul}`, exact: true })
        .click();
      const detail = page.getByRole("dialog", { name: judul });
      await expect(detail).toBeVisible();
      await expect(detail.getByText(/^INT-\d+ · /)).toBeVisible();
      await expect(detail.getByText("Dibuat oleh uji e2e")).toBeVisible();

      const riwayat = detail.getByRole("list", { name: "Riwayat kartu" });
      await expect(
        riwayat.getByText(`Kartu dibuat di tahap “${pertama.label}”`),
      ).toBeVisible();
      // Penanggung jawab awal disimpan bersama kartu tanpa peristiwa TUGASKAN, jadi hanya BUAT.
      await expect(riwayat.getByRole("listitem")).toHaveCount(1);
    } finally {
      hapusKartuUji(judul);
    }
  });

  test("strategis.djpb tidak melihat tombol Tambah intervensi", async ({
    page,
  }) => {
    await masukSebagai(page, "strategisDjpb");
    await buka(page, "/intervensi?siklus=tekanan");
    await expect(page.locator("[data-tahap-id]").first()).toBeVisible();
    await tungguTenang(page);
    await expect(
      page.getByRole("button", { name: "Tambah intervensi" }),
    ).toHaveCount(0);
    // Detail tetap dapat dibuka (intervensi.lihat) dan menampilkan riwayat.
    await page
      .getByRole("button", {
        name: /^Buka detail Lonjakan cabai merah Banda Aceh$/,
      })
      .click();
    const detail = page.getByRole("dialog", {
      name: "Lonjakan cabai merah Banda Aceh",
    });
    await expect(
      detail.getByRole("list", { name: "Riwayat kartu" }),
    ).toBeVisible();
    await expect(detail.getByText(/Kartu dibuat di tahap/)).toBeVisible();
  });
});
