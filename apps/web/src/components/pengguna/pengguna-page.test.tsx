import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import type { UserResponse } from "@/lib/api/pengguna";
import type { PeranWire, IzinWire } from "@/lib/api/rbac";
import { drafMatriks } from "@/lib/rbac/draf-matriks";
import { ROLE_PERMISSIONS } from "@/lib/rbac/permissions";
import { galatApi, pasangFetchPalsu, penggunaSesi, renderHalaman, tangkapUnduhan, type FetchPalsu } from "@/test/render";
import { PenggunaPage } from "./pengguna-page";

// ── Data ─────────────────────────────────────────────────────────────

const MASA_DEPAN = new Date(Date.now() + 30 * 60_000).toISOString();

const dasar = {
  lapis: "OPERASIONAL",
  lembaga: "BPPSDMP",
  dibuat_pada: "2026-10-01T08:00:00Z",
} as const;

/** Lima akun satu halaman: diri sendiri (admin-1), ber-MFA, terkunci, menunggu aktivasi, nonaktif. */
const AKUN: UserResponse[] = [
  {
    ...dasar,
    id: "admin-1",
    nama: "Admin Satu",
    email: "admin@garda.test",
    peran: "ADMIN_SISTEM",
    lapis: "SISTEM",
    mfa_aktif: true,
    status: "AKTIF",
    terakhir_masuk: "2026-11-05T08:00:00Z",
  },
  {
    ...dasar,
    id: "u-ani",
    nama: "Ani Mahasiswa",
    email: "ani@garda.test",
    peran: "OPS_MAHASISWA",
    mfa_aktif: true,
    status: "AKTIF",
    terakhir_masuk: "2026-11-04T08:00:00Z",
  },
  {
    ...dasar,
    id: "u-budi",
    nama: "Budi Terkunci",
    email: "budi@garda.test",
    peran: "MANAJER_DATA",
    lapis: "MANAJERIAL",
    mfa_aktif: false,
    status: "AKTIF",
    terkunci_sampai: MASA_DEPAN,
  },
  {
    ...dasar,
    id: "u-citra",
    nama: "Citra Diundang",
    email: "citra@garda.test",
    peran: "OPS_KOORDINATOR",
    mfa_aktif: false,
    status: "MENUNGGU",
  },
  {
    ...dasar,
    id: "u-dewi",
    nama: "Dewi Nonaktif",
    email: "dewi@garda.test",
    peran: "OPS_AGREGATOR",
    mfa_aktif: false,
    status: "NONAKTIF",
  },
];

function halaman(
  items: UserResponse[] = AKUN,
  total = items.length,
  page = 1,
  limit = 25,
) {
  return { items, total, page, limit };
}

const PERAN: PeranWire[] = [
  {
    id: "ADMIN_SISTEM",
    label: "Administrator Sistem",
    lapis: "SISTEM",
    permissions: ["pengguna.kelola", "harga.lihat"],
    bawaan: ["pengguna.kelola", "harga.lihat"],
    versi: 3,
  },
  {
    id: "OPS_MAHASISWA",
    label: "Mahasiswa",
    lapis: "OPERASIONAL",
    permissions: ["harga.lihat"],
    bawaan: ["harga.lihat"],
    versi: 1,
  },
];
const IZIN: IzinWire[] = [
  { id: "pengguna.kelola", deskripsi: "Kelola pengguna", kelompok: "Pengguna" },
  { id: "harga.lihat", deskripsi: "Lihat harga", kelompok: "Harga" },
  { id: "harga.ekspor", deskripsi: "Ekspor harga", kelompok: "Harga" },
];

let fetchPalsu: FetchPalsu;

function pasangApi(tambahan: Record<string, unknown> = {}) {
  fetchPalsu = pasangFetchPalsu({
    "GET /api/pengguna": () => halaman(),
    "GET /api/pengguna/u-ani": () => AKUN[1],
    "GET /api/pengguna/u-ani/riwayat-peran": () => ({
      items: [
        {
          id: "a-2",
          waktu: "2026-11-03T09:00:00Z",
          aksi: "ASSIGN_ROLE",
          dari: "OPS_AGREGATOR",
          ke: "OPS_MAHASISWA",
          aktor: { id: "admin-1", nama: "Admin Satu" },
        },
        {
          id: "a-1",
          waktu: "2026-10-01T08:00:00Z",
          aksi: "CREATE_USER",
          dari: null,
          ke: "OPS_AGREGATOR",
          aktor: null,
        },
      ],
      terpotong: false,
    }),
    "GET /api/pengguna/u-ani/permissions": () => ({
      peran: "OPS_MAHASISWA",
      izin_peran: ["harga.lihat"],
      langsung: [],
      efektif: ["harga.lihat"],
    }),
    "GET /api/rbac/roles": () => PERAN,
    "GET /api/rbac/permissions": () => IZIN,
    ...tambahan,
  });
}

beforeEach(() => pasangApi());
afterEach(() => {
  fetchPalsu.pulihkan();
  // Draf matriks adalah store global (bertahan antartab di aplikasi); jangan bocor antartes.
  drafMatriks.bersihkan();
});

/** Tunggu sampai baris pengguna dari server tampil. */
const tungguBaris = () => screen.findByText("Ani Mahasiswa");

/** Baris tabel (tr) yang memuat teks. */
const baris = (teks: string) =>
  screen.getByText(teks).closest("tr") as HTMLElement;

const kueriDaftarTerakhir = () =>
  fetchPalsu.ke("GET", "/api/pengguna").at(-1)!.query;

async function bukaMenuBaris(
  user: ReturnType<typeof renderHalaman>["user"],
  nama: string,
) {
  await user.click(screen.getByRole("button", { name: `Aksi untuk ${nama}` }));
  return screen.findByRole("menu");
}

const itemMenu = (menu: HTMLElement) =>
  within(menu)
    .queryAllByRole("menuitem")
    .map((e) => e.textContent?.trim() ?? "");

// ── Tes ──────────────────────────────────────────────────────────────

describe("PenggunaPage: daftar dari server", () => {
  it("merender baris dari data server dengan peran, status, MFA, dan lencana terkunci", async () => {
    renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const ani = within(baris("Ani Mahasiswa"));
    assert.ok(ani.getByText("ani@garda.test"));
    assert.ok(ani.getByText("Agen Lapangan Mahasiswa"));
    assert.ok(ani.getByText("Aktif"));
    assert.ok(ani.getByLabelText("MFA aktif"));

    assert.ok(within(baris("Citra Diundang")).getByText("Menunggu aktivasi"));
    assert.ok(within(baris("Dewi Nonaktif")).getByText("Nonaktif"));
    // Lencana kunci login membawa sisa waktu yang berjalan (role=timer); akun lain tidak punya.
    assert.ok(
      await within(baris("Budi Terkunci")).findByRole("timer"),
      "lencana kunci login",
    );
    assert.equal(within(baris("Ani Mahasiswa")).queryByRole("timer"), null);
    assert.ok(
      within(baris("Budi Terkunci")).getByText("Belum"),
      "MFA belum aktif",
    );
    assert.ok(screen.getByText("1–5"), "rentang baris yang tampil");
  });

  it("kueri awal ke server membawa halaman dan ukuran bawaan, tanpa filter", async () => {
    renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const q = kueriDaftarTerakhir();
    assert.equal(q.get("page"), "1");
    assert.equal(q.get("limit"), "25");
    for (const k of [
      "search",
      "peran",
      "status",
      "lapis",
      "mfa",
      "terkunci",
      "sort",
      "arah",
    ])
      assert.equal(q.get(k), null, k);
  });

  it("keadaan awal dibaca dari URL (muat ulang memulihkan tabel)", async () => {
    fetchPalsu.atur("GET /api/pengguna", () => halaman(AKUN, 40, 2, 10));
    renderHalaman(<PenggunaPage />, {
      url: "/pengguna?search=ani&status=AKTIF&sort=email&arah=desc&page=2&limit=10",
    });
    await tungguBaris();

    const q = kueriDaftarTerakhir();
    assert.equal(q.get("search"), "ani");
    assert.equal(q.get("status"), "AKTIF");
    assert.equal(q.get("sort"), "email");
    assert.equal(q.get("arah"), "desc");
    assert.equal(q.get("page"), "2");
    assert.equal(q.get("limit"), "10");
    assert.equal(
      (screen.getByPlaceholderText(/Cari nama/) as HTMLInputElement).value,
      "ani",
    );
  });

  it("nilai asing di URL diabaikan, bukan dikirim ke server", async () => {
    renderHalaman(<PenggunaPage />, {
      url: "/pengguna?status=BEKU&sort=password_hash&limit=9999&page=abc",
    });
    await tungguBaris();

    const q = kueriDaftarTerakhir();
    assert.equal(q.get("status"), null);
    assert.equal(q.get("sort"), null);
    assert.equal(q.get("limit"), "25");
    assert.equal(q.get("page"), "1");
  });

  it("galat dari server tidak merender baris dan tidak menggantung", async () => {
    fetchPalsu.atur("GET /api/pengguna", galatApi(500, "Gagal mengambil data"));
    renderHalaman(<PenggunaPage />);

    await waitFor(() =>
      assert.ok(fetchPalsu.ke("GET", "/api/pengguna").length >= 1),
    );
    assert.equal(screen.queryByText("Ani Mahasiswa"), null);
  });
});

describe("PenggunaPage: urutan, pencarian, filter, halaman", () => {
  it("klik kepala kolom mengubah URL: naik, turun, lalu kembali ke bawaan", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    const kepala = () => screen.getByRole("columnheader", { name: /Nama/ });

    await user.click(within(kepala()).getByRole("button"));
    assert.equal(window.location.search, "?sort=nama&arah=asc");
    await waitFor(() =>
      assert.equal(kueriDaftarTerakhir().get("sort"), "nama"),
    );
    assert.equal(kepala().getAttribute("aria-sort"), "ascending");

    await user.click(within(kepala()).getByRole("button"));
    assert.equal(window.location.search, "?sort=nama&arah=desc");
    await waitFor(() =>
      assert.equal(kueriDaftarTerakhir().get("arah"), "desc"),
    );
    assert.equal(kepala().getAttribute("aria-sort"), "descending");

    await user.click(within(kepala()).getByRole("button"));
    assert.equal(window.location.search, "");
    await waitFor(() => assert.equal(kueriDaftarTerakhir().get("sort"), null));
  });

  it("kolom yang tidak bisa diurutkan (MFA, cakupan) tidak punya tombol urut", async () => {
    renderHalaman(<PenggunaPage />);
    await tungguBaris();

    assert.equal(
      within(screen.getByRole("columnheader", { name: "MFA" })).queryByRole(
        "button",
      ),
      null,
    );
    assert.equal(
      within(screen.getByRole("columnheader", { name: "Cakupan" })).queryByRole(
        "button",
      ),
      null,
    );
    assert.ok(
      within(
        screen.getByRole("columnheader", { name: /Terakhir masuk/ }),
      ).getByRole("button"),
    );
  });

  it("pencarian: Enter mengirim seketika, URL memakai replace, dan kembali ke halaman 1", async () => {
    const { user } = renderHalaman(<PenggunaPage />, {
      url: "/pengguna?page=3",
    });
    await tungguBaris();
    const riwayatSebelum = window.history.length;

    await user.type(screen.getByPlaceholderText(/Cari nama/), "ani{Enter}");

    assert.equal(window.location.search, "?search=ani");
    assert.equal(
      window.history.length,
      riwayatSebelum,
      "mengetik tidak menambah langkah 'kembali'",
    );
    await waitFor(() => {
      const q = kueriDaftarTerakhir();
      assert.equal(q.get("search"), "ani");
      assert.equal(q.get("page"), "1");
    });
  });

  it("pencarian: tanpa Enter, dikirim setelah jeda ketik", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.type(screen.getByPlaceholderText(/Cari nama/), "budi");
    assert.equal(window.location.search, "", "belum dikirim sebelum jeda");

    await waitFor(() => assert.equal(window.location.search, "?search=budi"), {
      timeout: 2000,
    });
  });

  it("filter: memilih status mengubah URL dan kueri, 'Reset filter' membuangnya", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.click(screen.getByRole("combobox", { name: "Filter status" }));
    await user.click(
      await screen.findByRole("option", { name: "Menunggu aktivasi" }),
    );
    assert.equal(window.location.search, "?status=MENUNGGU");
    await waitFor(() =>
      assert.equal(kueriDaftarTerakhir().get("status"), "MENUNGGU"),
    );

    await user.click(screen.getByRole("button", { name: "Reset filter (1)" }));
    assert.equal(window.location.search, "");
    await waitFor(() =>
      assert.equal(kueriDaftarTerakhir().get("status"), null),
    );
    assert.equal(screen.queryByRole("button", { name: /Reset filter/ }), null);
  });

  it("paginasi: total dari server menentukan halaman; berikutnya mengubah URL; sebelumnya nonaktif di halaman 1", async () => {
    fetchPalsu.atur("GET /api/pengguna", (req) =>
      halaman(
        AKUN,
        60,
        Number(req.query.get("page")),
        Number(req.query.get("limit")),
      ),
    );
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    assert.ok(screen.getByText("1 / 3"));
    assert.equal(
      (
        screen.getByRole("button", {
          name: "Halaman sebelumnya",
        }) as HTMLButtonElement
      ).disabled,
      true,
    );

    await user.click(
      screen.getByRole("button", { name: "Halaman berikutnya" }),
    );
    assert.equal(window.location.search, "?page=2");
    await waitFor(() => assert.equal(kueriDaftarTerakhir().get("page"), "2"));
    await waitFor(() => assert.ok(screen.getByText("2 / 3")));
  });

  it("ukuran halaman dari pemilih ikut ke URL dan kueri", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.click(
      screen.getByRole("combobox", { name: "" }).closest("div")
        ? screen.getAllByRole("combobox").at(-1)!
        : screen.getAllByRole("combobox").at(-1)!,
    );
    await user.click(
      await screen.findByRole("option", { name: "50 / halaman" }),
    );

    assert.equal(window.location.search, "?limit=50");
    await waitFor(() => assert.equal(kueriDaftarTerakhir().get("limit"), "50"));
  });

  it("halaman di luar jangkauan dikoreksi ke halaman terakhir", async () => {
    fetchPalsu.atur("GET /api/pengguna", (req) =>
      halaman(
        Number(req.query.get("page")) > 2 ? [] : AKUN,
        30,
        Number(req.query.get("page")),
        25,
      ),
    );
    renderHalaman(<PenggunaPage />, { url: "/pengguna?page=9" });

    await waitFor(() => assert.equal(window.location.search, "?page=2"));
  });
});

describe("PenggunaPage: pilih baris", () => {
  it("'pilih semua' hanya baris di halaman ini, dengan catatan untuk halaman lain", async () => {
    fetchPalsu.atur("GET /api/pengguna", () => halaman(AKUN, 12, 1, 5));
    const { user } = renderHalaman(<PenggunaPage />, {
      url: "/pengguna?limit=10",
    });
    await tungguBaris();

    await user.click(
      screen.getByRole("checkbox", { name: "Pilih semua di halaman ini" }),
    );

    assert.ok(
      screen.getByRole("toolbar", { name: "Aksi untuk 5 pengguna terpilih" }),
    );
    assert.ok(
      screen.getByText(
        /Seluruh 5 pengguna di halaman ini terpilih\. 7 pengguna di halaman lain tidak ikut terpilih\./,
      ),
    );
    assert.equal(
      screen.queryByText(/Pilih seluruh/),
      null,
      "tidak ada pilihan lintas halaman",
    );
  });

  it("pilihan dibersihkan begitu halaman, urutan, atau filter berubah", async () => {
    fetchPalsu.atur("GET /api/pengguna", () => halaman(AKUN, 12, 1, 5));
    const { user } = renderHalaman(<PenggunaPage />, {
      url: "/pengguna?limit=10",
    });
    await tungguBaris();

    await user.click(
      screen.getByRole("checkbox", { name: "Pilih semua di halaman ini" }),
    );
    assert.ok(screen.getByRole("toolbar"));

    await user.click(
      within(screen.getByRole("columnheader", { name: /Nama/ })).getByRole(
        "button",
      ),
    );
    await waitFor(() => assert.equal(screen.queryByRole("toolbar"), null));
  });

  it("aksi massal Nonaktifkan hanya aktif bila semua terpilih AKTIF; konfirmasi mengirim id terpilih", async () => {
    fetchPalsu.atur("POST /api/pengguna/bulk", { berhasil: 2, gagal: 0 });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.click(within(baris("Ani Mahasiswa")).getByRole("checkbox"));
    await user.click(within(baris("Budi Terkunci")).getByRole("checkbox"));
    const nonaktifkan = screen.getByRole("button", {
      name: /Nonaktifkan/,
    }) as HTMLButtonElement;
    assert.equal(nonaktifkan.disabled, false);

    await user.click(nonaktifkan);
    const dialog = await screen.findByRole("alertdialog");
    assert.ok(within(dialog).getByText("Nonaktifkan akun terpilih?"));
    await user.click(
      within(dialog).getByRole("button", { name: "Nonaktifkan" }),
    );

    await waitFor(() =>
      assert.equal(fetchPalsu.ke("POST", "/api/pengguna/bulk").length, 1),
    );
    assert.deepEqual(fetchPalsu.ke("POST", "/api/pengguna/bulk")[0].body, {
      action: "nonaktifkan",
      ids: ["u-ani", "u-budi"],
    });

    // Menambahkan akun MENUNGGU ke pilihan mematikan tombol: aksi massal tidak berlaku untuknya.
    await waitFor(() => assert.equal(screen.queryByRole("toolbar"), null));
    await user.click(within(baris("Ani Mahasiswa")).getByRole("checkbox"));
    await user.click(within(baris("Citra Diundang")).getByRole("checkbox"));
    assert.equal(
      (screen.getByRole("button", { name: /Nonaktifkan/ }) as HTMLButtonElement)
        .disabled,
      true,
    );
    assert.equal(
      (screen.getByRole("button", { name: /Aktifkan/ }) as HTMLButtonElement)
        .disabled,
      true,
    );
  });
});

describe("PenggunaPage: aksi baris menurut status dan sesi", () => {
  it("akun MENUNGGU: 'Kirim ulang undangan' menggantikan 'Reset kata sandi' dan memanggil undang-ulang", async () => {
    fetchPalsu.atur("POST /api/pengguna/u-citra/undang-ulang", {
      user: { ...AKUN[3] },
      berlaku_sampai: MASA_DEPAN,
      email_terkirim: true,
    });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const menu = await bukaMenuBaris(user, "Citra Diundang");
    const item = itemMenu(menu);
    assert.ok(item.some((t) => /Kirim ulang undangan/.test(t)));
    assert.ok(!item.some((t) => /Reset kata sandi/.test(t)));

    await user.click(
      within(menu).getByRole("menuitem", { name: /Kirim ulang undangan/ }),
    );
    await waitFor(() =>
      assert.equal(
        fetchPalsu.ke("POST", "/api/pengguna/u-citra/undang-ulang").length,
        1,
      ),
    );
  });

  it("akun AKTIF/NONAKTIF: 'Reset kata sandi' ada, 'Kirim ulang undangan' tidak", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const item = itemMenu(await bukaMenuBaris(user, "Dewi Nonaktif"));
    assert.ok(item.some((t) => /Reset kata sandi/.test(t)));
    assert.ok(!item.some((t) => /Kirim ulang undangan/.test(t)));
  });

  it("Reset MFA hanya untuk akun ber-MFA selain akun sendiri", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    // Ani: ber-MFA dan bukan diri sendiri.
    let menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    assert.ok(itemMenu(menu).some((t) => /Reset MFA/.test(t)));
    await user.keyboard("{Escape}");

    // Admin Satu = akun sendiri (sesi admin-1): ber-MFA tetapi dilarang (400 di server).
    menu = await bukaMenuBaris(user, "Admin Satu");
    assert.ok(!itemMenu(menu).some((t) => /Reset MFA/.test(t)));
    await user.keyboard("{Escape}");

    // Budi: tanpa MFA, tidak ada yang direset.
    menu = await bukaMenuBaris(user, "Budi Terkunci");
    assert.ok(!itemMenu(menu).some((t) => /Reset MFA/.test(t)));
  });

  it("Buka kunci hanya untuk akun yang sedang terkunci, dan memanggil buka-kunci", async () => {
    fetchPalsu.atur("POST /api/pengguna/u-budi/buka-kunci", {
      id: "u-budi",
      terkunci_sebelumnya: true,
    });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    let menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    assert.ok(!itemMenu(menu).some((t) => /Buka kunci/.test(t)));
    await user.keyboard("{Escape}");

    menu = await bukaMenuBaris(user, "Budi Terkunci");
    await user.click(
      within(menu).getByRole("menuitem", { name: /Buka kunci/ }),
    );
    await waitFor(() =>
      assert.equal(
        fetchPalsu.ke("POST", "/api/pengguna/u-budi/buka-kunci").length,
        1,
      ),
    );
  });

  it("kunci yang sudah berakhir tidak menawarkan 'Buka kunci'", async () => {
    const lampau = new Date(Date.now() - 60_000).toISOString();
    fetchPalsu.atur("GET /api/pengguna", () =>
      halaman([{ ...AKUN[2], terkunci_sampai: lampau }, AKUN[1]]),
    );
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const menu = await bukaMenuBaris(user, "Budi Terkunci");
    assert.ok(!itemMenu(menu).some((t) => /Buka kunci/.test(t)));
  });

  it("tanpa izin pengguna.kelola: tidak ada Reset MFA, Buka kunci, maupun matriks yang dapat diedit", async () => {
    const tanpaKelola = penggunaSesi({
      id: "viewer-1",
      role: "OPS_KOORDINATOR",
      permissions: ROLE_PERMISSIONS.OPS_KOORDINATOR.filter(
        (p) => p !== "pengguna.kelola",
      ),
    });
    const { user } = renderHalaman(<PenggunaPage />, { sesi: tanpaKelola });
    await tungguBaris();

    let item = itemMenu(await bukaMenuBaris(user, "Ani Mahasiswa"));
    assert.ok(!item.some((t) => /Reset MFA/.test(t)));
    await user.keyboard("{Escape}");
    item = itemMenu(await bukaMenuBaris(user, "Budi Terkunci"));
    assert.ok(!item.some((t) => /Buka kunci/.test(t)));
  });

  it("Reset MFA: dialog menjelaskan akibat, konfirmasi memanggil reset-mfa dan menutup dialog", async () => {
    fetchPalsu.atur("POST /api/pengguna/u-ani/reset-mfa", {
      id: "u-ani",
      mfa_aktif_sebelumnya: true,
      sesi_dicabut: 2,
    });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    await user.click(within(menu).getByRole("menuitem", { name: /Reset MFA/ }));

    const dialog = await screen.findByRole("dialog", { name: "Reset MFA" });
    assert.ok(within(dialog).getByText(/semua kode cadangan dihapus/));
    assert.ok(within(dialog).getByText(/Tindakan ini tercatat di audit trail/));
    await user.click(within(dialog).getByRole("button", { name: "Reset MFA" }));

    await waitFor(() =>
      assert.equal(
        fetchPalsu.ke("POST", "/api/pengguna/u-ani/reset-mfa").length,
        1,
      ),
    );
    await waitFor(() =>
      assert.equal(screen.queryByRole("dialog", { name: "Reset MFA" }), null),
    );
    // Daftar disegarkan supaya kolom MFA mengikuti server.
    await waitFor(() =>
      assert.ok(fetchPalsu.ke("GET", "/api/pengguna").length >= 2),
    );
  });

  it("Reset MFA ditolak server: galat tampil di dialog dan dialog tetap terbuka", async () => {
    fetchPalsu.atur(
      "POST /api/pengguna/u-ani/reset-mfa",
      galatApi(500, "Gagal mereset MFA"),
    );
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    await user.click(within(menu).getByRole("menuitem", { name: /Reset MFA/ }));
    const dialog = await screen.findByRole("dialog", { name: "Reset MFA" });
    await user.click(within(dialog).getByRole("button", { name: "Reset MFA" }));

    await waitFor(() => assert.ok(within(dialog).getByRole("alert")));
    assert.ok(screen.getByRole("dialog", { name: "Reset MFA" }));
  });

  it("Hapus pengguna memanggil DELETE lalu menyegarkan daftar", async () => {
    fetchPalsu.atur("DELETE /api/pengguna/u-dewi", { id: "u-dewi" });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    const awal = fetchPalsu.ke("GET", "/api/pengguna").length;

    const menu = await bukaMenuBaris(user, "Dewi Nonaktif");
    await user.click(
      within(menu).getByRole("menuitem", { name: /Hapus pengguna/ }),
    );

    await waitFor(() =>
      assert.equal(fetchPalsu.ke("DELETE", "/api/pengguna/u-dewi").length, 1),
    );
    await waitFor(() =>
      assert.ok(fetchPalsu.ke("GET", "/api/pengguna").length > awal),
    );
  });
});

describe("PenggunaPage: toolbar undang, impor, ekspor", () => {
  let unduhan: ReturnType<typeof tangkapUnduhan>;
  beforeEach(() => {
    unduhan = tangkapUnduhan();
  });
  afterEach(() => unduhan.pulihkan());

  const csvBalasan = () =>
    new Response("﻿id;nama\r\nu-ani;Ani", {
      status: 200,
      headers: {
        "Content-Type": "text/csv",
        "Content-Disposition": 'attachment; filename="pengguna-2026-11-06.csv"',
        "X-Export-Rows": "1",
      },
    });

  it("label ekspor 'Ekspor semua (CSV)' tanpa filter", async () => {
    renderHalaman(<PenggunaPage />);
    await tungguBaris();
    assert.ok(screen.getByRole("button", { name: /Ekspor semua \(CSV\)/ }));
  });

  it("label ekspor menjadi 'Ekspor hasil filter (CSV)' saat filter atau pencarian aktif, dan mengirim filter itu", async () => {
    fetchPalsu.atur("GET /api/pengguna/ekspor", csvBalasan);
    const { user } = renderHalaman(<PenggunaPage />, {
      url: "/pengguna?status=AKTIF&mfa=true&search=ani",
    });
    await tungguBaris();

    await user.click(
      screen.getByRole("button", { name: /Ekspor hasil filter \(CSV\)/ }),
    );

    await waitFor(() =>
      assert.equal(fetchPalsu.ke("GET", "/api/pengguna/ekspor").length, 1),
    );
    const q = fetchPalsu.ke("GET", "/api/pengguna/ekspor")[0].query;
    assert.equal(q.get("status"), "AKTIF");
    assert.equal(q.get("mfa"), "true");
    assert.equal(q.get("search"), "ani");
    assert.equal(
      q.get("page"),
      null,
      "halaman dan ukuran tidak ikut: ekspor mencakup seluruh hasil filter",
    );
    assert.equal(q.get("ids"), null);
    // Berkas disimpan dengan nama dari header Content-Disposition server.
    await waitFor(() => assert.equal(unduhan.daftar.length, 1));
    assert.equal(unduhan.daftar[0].download, "pengguna-2026-11-06.csv");
  });

  it("ekspor hanya karena pencarian pun berlabel 'hasil filter'", async () => {
    renderHalaman(<PenggunaPage />, { url: "/pengguna?search=ani" });
    await tungguBaris();
    assert.ok(
      screen.getByRole("button", { name: /Ekspor hasil filter \(CSV\)/ }),
    );
  });

  it("aksi massal 'Ekspor CSV' mengirim id terpilih, bukan filter", async () => {
    fetchPalsu.atur("GET /api/pengguna/ekspor", csvBalasan);
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.click(within(baris("Ani Mahasiswa")).getByRole("checkbox"));
    await user.click(within(baris("Dewi Nonaktif")).getByRole("checkbox"));
    // "Ekspor CSV" ada di menu "Aksi lainnya" karena bilah hanya menampilkan tiga aksi.
    await user.click(screen.getByRole("button", { name: "Aksi lainnya" }));
    await user.click(
      await screen.findByRole("menuitem", { name: /Ekspor CSV/ }),
    );

    await waitFor(() =>
      assert.equal(fetchPalsu.ke("GET", "/api/pengguna/ekspor").length, 1),
    );
    assert.equal(
      fetchPalsu.ke("GET", "/api/pengguna/ekspor")[0].query.get("ids"),
      "u-ani,u-dewi",
    );
  });

  it("ekspor yang gagal tidak melempar ke halaman (toast), tombol kembali aktif", async () => {
    fetchPalsu.atur(
      "GET /api/pengguna/ekspor",
      galatApi(500, "Gagal mengekspor pengguna"),
    );
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.click(
      screen.getByRole("button", { name: /Ekspor semua \(CSV\)/ }),
    );

    await waitFor(() =>
      assert.equal(fetchPalsu.ke("GET", "/api/pengguna/ekspor").length, 1),
    );
    await waitFor(() =>
      assert.equal(
        (
          screen.getByRole("button", {
            name: /Ekspor semua \(CSV\)/,
          }) as HTMLButtonElement
        ).disabled,
        false,
      ),
    );
  });

  it("'Undang pengguna' membuka dialog undangan", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.click(screen.getByRole("button", { name: /Undang pengguna/ }));

    assert.ok(await screen.findByRole("dialog"));
  });
});

describe("PenggunaPage: dialog ubah pengguna", () => {
  it("memuat data segar, menampilkan rincian akun dan riwayat peran, tab Izin langsung untuk pengelola", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    const menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    await user.click(within(menu).getByRole("menuitem", { name: /Ubah data/ }));

    const dialog = await screen.findByRole("dialog", { name: "Ubah pengguna" });
    await waitFor(() =>
      assert.equal(
        (within(dialog).getByLabelText("Nama lengkap") as HTMLInputElement)
          .value,
        "Ani Mahasiswa",
      ),
    );
    assert.equal(
      (within(dialog).getByLabelText("Email") as HTMLInputElement).value,
      "ani@garda.test",
    );
    assert.ok(
      fetchPalsu.ke("GET", "/api/pengguna/u-ani").length >= 1,
      "data segar dari GET /api/pengguna/:id",
    );
    assert.ok(within(dialog).getByLabelText("Rincian akun"));

    // Riwayat peran: terbaru dulu, peran awal tanpa 'dari', pelaku sistem bila tanpa aktor.
    const riwayat = await within(dialog).findByRole("list", {
      name: "Riwayat peran",
    });
    const entri = within(riwayat).getAllByRole("listitem");
    assert.equal(entri.length, 2);
    assert.match(entri[0].textContent ?? "", /Peran diganti/);
    assert.match(entri[0].textContent ?? "", /Admin Satu/);
    assert.match(entri[1].textContent ?? "", /Akun dibuat/);
    assert.match(entri[1].textContent ?? "", /Sistem/);
    assert.equal(
      within(dialog)
        .getByRole("link", { name: "Lihat di audit trail" })
        .getAttribute("href"),
      "/audit?entitas=pengguna&entitas_id=u-ani",
    );

    assert.ok(within(dialog).getByRole("tab", { name: "Izin langsung" }), "tab hanya untuk pengelola");
  });

  it("tab Izin langsung memuat izin pengguna dan menampilkan formulir pemberian", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    const menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    await user.click(within(menu).getByRole("menuitem", { name: /Ubah data/ }));
    const dialog = await screen.findByRole("dialog", { name: "Ubah pengguna" });
    await waitFor(() => within(dialog).getByLabelText("Nama lengkap"));

    await user.click(within(dialog).getByRole("tab", { name: "Izin langsung" }));

    await within(dialog).findByText(/Izin langsung — Ani Mahasiswa/);
    assert.ok(
      await within(dialog).findByText(
        "Belum ada izin langsung untuk pengguna ini.",
      ),
    );
    assert.ok(within(dialog).getByRole("form", { name: "Beri izin langsung" }));
    // Formulir akun tetap terpasang (disembunyikan): isiannya tidak hilang saat berpindah tab.
    assert.ok(dialog.querySelector("#pg-nama"));
  });

  it("menyimpan perubahan mengirim PATCH berisi kolom formulir dan menutup dialog", async () => {
    fetchPalsu.atur("PATCH /api/pengguna/u-ani", {
      ...AKUN[1],
      nama: "Ani Baru",
    });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    const menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    await user.click(within(menu).getByRole("menuitem", { name: /Ubah data/ }));
    const dialog = await screen.findByRole("dialog", { name: "Ubah pengguna" });
    const nama = (await within(dialog).findByLabelText(
      "Nama lengkap",
    )) as HTMLInputElement;
    await waitFor(() => assert.equal(nama.value, "Ani Mahasiswa"));

    await user.clear(nama);
    await user.type(nama, "Ani Baru");
    await user.click(
      within(dialog).getByRole("button", { name: "Simpan perubahan" }),
    );

    await waitFor(() =>
      assert.equal(fetchPalsu.ke("PATCH", "/api/pengguna/u-ani").length, 1),
    );
    assert.deepEqual(fetchPalsu.ke("PATCH", "/api/pengguna/u-ani")[0].body, {
      nama: "Ani Baru",
      email: "ani@garda.test",
      peran: "OPS_MAHASISWA",
      lembaga: "BPPSDMP",
      lapis: "OPERASIONAL",
    });
    await waitFor(() =>
      assert.equal(
        screen.queryByRole("dialog", { name: "Ubah pengguna" }),
        null,
      ),
    );
  });

  it("validasi klien: nama kosong menahan kiriman dan menampilkan pesan di field", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    const menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    await user.click(within(menu).getByRole("menuitem", { name: /Ubah data/ }));
    const dialog = await screen.findByRole("dialog", { name: "Ubah pengguna" });
    const nama = (await within(dialog).findByLabelText(
      "Nama lengkap",
    )) as HTMLInputElement;
    await waitFor(() => assert.equal(nama.value, "Ani Mahasiswa"));

    await user.clear(nama);
    await user.click(
      within(dialog).getByRole("button", { name: "Simpan perubahan" }),
    );

    assert.ok(await within(dialog).findByText("Nama wajib diisi."));
    assert.equal(nama.getAttribute("aria-invalid"), "true");
    assert.equal(fetchPalsu.ke("PATCH", "/api/pengguna/u-ani").length, 0);
  });

  it("gagal memuat detail: formulir memakai baris daftar dan memberi tahu", async () => {
    fetchPalsu.atur("GET /api/pengguna/u-ani", galatApi(500, "rusak"));
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    const menu = await bukaMenuBaris(user, "Ani Mahasiswa");
    await user.click(within(menu).getByRole("menuitem", { name: /Ubah data/ }));

    const dialog = await screen.findByRole("dialog", { name: "Ubah pengguna" });
    assert.ok(
      await within(dialog).findByText(/formulir memakai data dari daftar/),
    );
    assert.equal(
      (within(dialog).getByLabelText("Nama lengkap") as HTMLInputElement).value,
      "Ani Mahasiswa",
    );
  });
});

describe("PenggunaPage: matriks akses", () => {
  it("pengelola melihat matriks yang dapat diedit (sel berupa tombol bertoggle)", async () => {
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();

    await user.click(screen.getByText("Matriks akses"));

    const tabel = await screen.findByRole("table", {
      name: "Matriks akses peran dan izin",
    });
    const sel = within(tabel).getByRole("button", {
      name: "harga.lihat untuk Mahasiswa",
    });
    assert.equal(sel.getAttribute("aria-pressed"), "true");
    assert.equal(
      within(tabel)
        .getByRole("button", { name: "harga.ekspor untuk Mahasiswa" })
        .getAttribute("aria-pressed"),
      "false",
    );
  });

  it("klik sel membuat draf; Simpan mengirim seluruh izin peran dengan versi; Batal membuangnya", async () => {
    fetchPalsu.atur("PUT /api/rbac/roles/OPS_MAHASISWA/permissions", {
      ...PERAN[1],
      permissions: ["harga.ekspor", "harga.lihat"],
      versi: 2,
    });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    await user.click(screen.getByText("Matriks akses"));
    const tabel = await screen.findByRole("table", {
      name: "Matriks akses peran dan izin",
    });

    await user.click(
      within(tabel).getByRole("button", {
        name: "harga.ekspor untuk Mahasiswa",
      }),
    );
    assert.ok(screen.getByText("1 peran belum disimpan"));
    assert.equal(
      within(tabel)
        .getByRole("button", { name: "harga.ekspor untuk Mahasiswa" })
        .getAttribute("aria-pressed"),
      "true",
    );

    await user.click(
      within(tabel).getByRole("button", {
        name: "Batalkan perubahan Mahasiswa",
      }),
    );
    assert.equal(screen.queryByText(/belum disimpan/), null);

    await user.click(
      within(tabel).getByRole("button", {
        name: "harga.ekspor untuk Mahasiswa",
      }),
    );
    await user.click(
      within(tabel).getByRole("button", { name: "Simpan matriks Mahasiswa" }),
    );

    await waitFor(() =>
      assert.equal(
        fetchPalsu.ke("PUT", "/api/rbac/roles/OPS_MAHASISWA/permissions")
          .length,
        1,
      ),
    );
    assert.deepEqual(
      fetchPalsu.ke("PUT", "/api/rbac/roles/OPS_MAHASISWA/permissions")[0].body,
      {
        permissions: ["harga.lihat", "harga.ekspor"],
        versi: 1,
      },
    );
    await waitFor(() =>
      assert.equal(screen.queryByText(/belum disimpan/), null),
    );
  });

  it("penolakan pemegang terakhir mempertahankan draf agar dapat diperbaiki", async () => {
    fetchPalsu.atur("PUT /api/rbac/roles/ADMIN_SISTEM/permissions", {
      __status: 409,
      body: { message: "Perubahan ditolak", kode: "LAST_PERMISSION_HOLDER" },
    });
    const { user } = renderHalaman(<PenggunaPage />);
    await tungguBaris();
    await user.click(screen.getByText("Matriks akses"));
    const tabel = await screen.findByRole("table", {
      name: "Matriks akses peran dan izin",
    });

    await user.click(
      within(tabel).getByRole("button", {
        name: "pengguna.kelola untuk Administrator Sistem",
      }),
    );
    await user.click(
      within(tabel).getByRole("button", {
        name: "Simpan matriks Administrator Sistem",
      }),
    );

    await waitFor(() =>
      assert.equal(
        fetchPalsu.ke("PUT", "/api/rbac/roles/ADMIN_SISTEM/permissions").length,
        1,
      ),
    );
    await waitFor(() => assert.ok(screen.getByText("1 peran belum disimpan")), {
      timeout: 2000,
    });
  });

  it("tanpa pengguna.kelola hanya tampilan baca-saja (tidak ada tombol sel)", async () => {
    const viewer = penggunaSesi({
      id: "viewer-1",
      role: "OPS_KOORDINATOR",
      permissions: ROLE_PERMISSIONS.OPS_KOORDINATOR.filter(
        (p) => p !== "pengguna.kelola",
      ),
    });
    const { user } = renderHalaman(<PenggunaPage />, { sesi: viewer });
    await tungguBaris();
    fetchPalsu.atur("GET /api/rbac/roles", PERAN);

    await user.click(screen.getByText("Matriks akses"));

    await waitFor(() => assert.ok(screen.getByText("Izin")));
    assert.equal(
      screen.queryByRole("table", { name: "Matriks akses peran dan izin" }),
      null,
    );
    assert.equal(
      screen.queryByRole("button", { name: /untuk Mahasiswa/ }),
      null,
    );
  });
});

