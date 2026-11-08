import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import type { MeUserResponse } from "@/lib/bff/api-client";
import { ROLE_PERMISSIONS } from "@/lib/rbac/permissions";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
  type PermintaanFetch,
} from "@/test/render";
import { TabProfil, TabProfilDemo } from "./tab-profil";

// ── Data ─────────────────────────────────────────────────────────────

const PROFIL: MeUserResponse = {
  id: "u-budi",
  nama: "Budi Santoso",
  email: "budi@garda.test",
  peran: "OPS_KOORDINATOR",
  lapis: "OPERASIONAL",
  lembaga: "FMIPA USK",
  kabupaten_kode: "1106",
  mfa_aktif: false,
  status: "AKTIF",
  dibuat_pada: "2026-10-01T08:00:00Z",
  wajib_ganti_password: false,
  mfa_setup_diwajibkan: false,
  mfa_wajib: false,
  telepon: "+6281234567890",
};

const sesiBudi = (sisip: Parameters<typeof penggunaSesi>[0] = {}) =>
  penggunaSesi({
    id: "u-budi",
    nama: "Budi Santoso",
    email: "budi@garda.test",
    role: "OPS_KOORDINATOR",
    lembaga: "FMIPA USK",
    kabupatenKode: "1106",
    ...sisip,
  });

let fetchPalsu: FetchPalsu;
/** "Basis data" server palsu: GET mengembalikannya, PATCH mengubahnya seperti API sungguhan. */
let server: MeUserResponse;

function pasangApi(awal: MeUserResponse = PROFIL) {
  server = { ...awal };
  fetchPalsu = pasangFetchPalsu({
    "GET /api/auth/me": () => ({ user: server }),
    "PATCH /api/auth/me": (req: PermintaanFetch) => {
      const b = req.body as { nama?: string; telepon?: string };
      if (b.nama !== undefined) server.nama = b.nama;
      if (b.telepon !== undefined) {
        if (b.telepon === "") delete server.telepon;
        else server.telepon = b.telepon;
      }
      return { user: server };
    },
  });
}

beforeEach(() => {
  document.cookie = "garda_csrf=csrf-uji";
  pasangApi();
});
afterEach(() => {
  // Toast Sonner hidup di store global dan lolos dari cleanup render: tanpa dibuang, toast tes sebelumnya
  // muncul lagi (atau toast tes berikutnya tidak tampil) dan timernya menggantung proses tes.
  toast.dismiss();
  fetchPalsu.pulihkan();
});

// ── Pembantu ─────────────────────────────────────────────────────────

const nama = () => screen.getByLabelText("Nama lengkap") as HTMLInputElement;
const telepon = () =>
  screen.getByLabelText("Nomor telepon / WhatsApp") as HTMLInputElement;
const email = () => screen.getByLabelText("Email") as HTMLInputElement;
const simpan = () =>
  screen.getByRole("button", { name: "Simpan perubahan" }) as HTMLButtonElement;
const batalkan = () =>
  screen.getByRole("button", {
    name: "Batalkan perubahan",
  }) as HTMLButtonElement;
const ringkasan = () =>
  within(screen.getByRole("region", { name: "Ringkasan akun" }));

/** Tunggu sampai form terisi dari server. */
const tungguForm = () => screen.findByLabelText("Nama lengkap");

type Pengguna = ReturnType<typeof renderHalaman>["user"];
async function isi(user: Pengguna, el: HTMLElement, teks: string) {
  await user.clear(el);
  if (teks) await user.type(el, teks);
}

function render(sisip: Parameters<typeof sesiBudi>[0] = {}) {
  return renderHalaman(<TabProfil />, {
    url: "/profil",
    sesi: sesiBudi(sisip),
  });
}

const patchTerkirim = () => fetchPalsu.ke("PATCH", "/api/auth/me");

// ── Memuat dan tampilan ──────────────────────────────────────────────

describe("TabProfil: memuat dan tampilan", () => {
  it("menampilkan kerangka 'Memuat profil', lalu form terisi dari server", async () => {
    render();
    assert.ok(screen.getByRole("status", { name: "Memuat profil" }));

    await tungguForm();

    assert.equal(screen.queryByRole("status", { name: "Memuat profil" }), null);
    assert.equal(nama().value, "Budi Santoso");
    assert.equal(telepon().value, "+62 812-3456-7890");
    assert.equal(email().value, "budi@garda.test");
    assert.ok(screen.getByRole("heading", { name: "Data pribadi" }));
    assert.ok(screen.getByRole("heading", { name: "Penugasan & wilayah" }));
  });

  it("ringkasan akun: avatar inisial, nama, peran, status aktif, dan tanggal bergabung", async () => {
    render();
    await tungguForm();

    const r = ringkasan();
    assert.ok(r.getByText("Budi Santoso"));
    assert.ok(r.getByText("Koordinator Lapangan"));
    assert.ok(r.getByText("Aktif"));
    assert.ok(r.getByText("Bergabung 1 Okt 2026"));
    assert.ok(r.getByText("BS"), "inisial avatar");
  });

  it("status selain aktif dan tanggal yang tidak sah tampil wajar tanpa membuat halaman galat", async () => {
    pasangApi({ ...PROFIL, status: "MENUNGGU", dibuat_pada: "bukan-tanggal" });
    render();
    await tungguForm();

    assert.ok(ringkasan().getByText("Menunggu aktivasi"));
    assert.equal(ringkasan().queryByText(/Bergabung/), null);
  });

  it("email hanya-baca dengan ikon gembok dan catatan, tanpa klaim 'terverifikasi'", async () => {
    render();
    await tungguForm();

    assert.equal(email().readOnly, true);
    assert.equal(email().disabled, false, "tetap bisa difokus dan disalin");
    const catatan = screen.getByText(
      "Hubungi administrator untuk perubahan email instansi",
    );
    assert.equal(email().getAttribute("aria-describedby"), catatan.id);
    assert.equal(screen.queryByText(/terverifikasi|verified/i), null);
  });

  it("profil tanpa telepon: kotak kosong dengan contoh format sebagai placeholder", async () => {
    const { telepon: _hapus, ...tanpaTelepon } = PROFIL;
    pasangApi(tanpaTelepon);
    render();
    await tungguForm();

    assert.equal(telepon().value, "");
    assert.equal(telepon().placeholder, "0812-3456-7890");
  });

  it("setiap isian punya label, dan petunjuk telepon terhubung lewat aria-describedby", async () => {
    render();
    await tungguForm();

    for (const el of [nama(), telepon(), email()]) {
      assert.ok(el.labels?.length, `label ${el.id}`);
    }
    const petunjuk = document.getElementById(
      telepon().getAttribute("aria-describedby") ?? "",
    );
    assert.match(petunjuk?.textContent ?? "", /Opsional/);
    assert.equal(telepon().type, "tel");
    assert.equal(telepon().getAttribute("inputmode"), "tel");
  });

  it("galat memuat: pesan dengan tombol 'Coba lagi' yang memuat ulang profil", async () => {
    fetchPalsu.atur("GET /api/auth/me", galatApi(500, "Backend error (500)"));
    const { user } = render();

    const peringatan = await screen.findByRole("alert");
    assert.ok(within(peringatan).getByText("Profil tidak dapat dimuat"));
    assert.ok(
      within(peringatan).getByText(
        "Tidak dapat terhubung ke server. Coba lagi.",
      ),
      "isi galat server tidak ditampilkan mentah",
    );
    assert.equal(screen.queryByLabelText("Nama lengkap"), null);

    fetchPalsu.atur("GET /api/auth/me", () => ({ user: server }));
    await user.click(
      within(peringatan).getByRole("button", { name: "Coba lagi" }),
    );

    assert.equal((await tungguForm()) instanceof HTMLInputElement, true);
    assert.equal(screen.queryByRole("alert"), null);
  });
});

// ── Validasi dan keadaan tombol ──────────────────────────────────────

describe("TabProfil: validasi dan tombol", () => {
  it("tanpa perubahan: simpan dan batalkan nonaktif", async () => {
    render();
    await tungguForm();

    assert.equal(simpan().disabled, true);
    assert.equal(batalkan().disabled, true);
  });

  it("nama terlalu pendek: pesan inline, aria-invalid, aria-describedby menunjuk pesan, simpan nonaktif", async () => {
    const { user } = render();
    await tungguForm();

    await isi(user, nama(), "Bu");

    const pesan = screen.getByText("Nama minimal 3 karakter");
    assert.equal(nama().getAttribute("aria-invalid"), "true");
    assert.equal(nama().getAttribute("aria-describedby"), pesan.id);
    assert.equal(simpan().disabled, true);
    assert.equal(batalkan().disabled, false);
    assert.equal(telepon().getAttribute("aria-invalid"), null);
  });

  it("nama kosong atau lebih dari 100 karakter ditolak dengan pesan yang sama seperti API", async () => {
    const { user } = render();
    await tungguForm();

    await isi(user, nama(), "");
    assert.ok(screen.getByText("Nama minimal 3 karakter"));

    await user.click(nama());
    await user.paste("a".repeat(101));
    assert.ok(screen.getByText("Nama maksimal 100 karakter"));
    assert.equal(simpan().disabled, true);
  });

  it("telepon tidak valid: pesan inline menggantikan petunjuk; diperbaiki, pesan hilang", async () => {
    const { user } = render();
    await tungguForm();

    await isi(user, telepon(), "12345");
    const pesan = screen.getByText("Nomor telepon tidak valid");
    assert.equal(telepon().getAttribute("aria-invalid"), "true");
    assert.equal(telepon().getAttribute("aria-describedby"), pesan.id);
    assert.equal(screen.queryByText(/Opsional/), null);
    assert.equal(simpan().disabled, true);

    await isi(user, telepon(), "0877 1111 2222");
    assert.equal(screen.queryByText("Nomor telepon tidak valid"), null);
    assert.equal(telepon().getAttribute("aria-invalid"), null);
    assert.equal(simpan().disabled, false);
  });

  it("nilai yang sama dengan tersimpan, hanya beda format atau spasi, bukan perubahan", async () => {
    const { user } = render();
    await tungguForm();

    await isi(user, telepon(), "0812 3456 7890");
    await isi(user, nama(), "  Budi Santoso ");

    assert.equal(simpan().disabled, true);
    assert.equal(
      screen.queryByText("Ada perubahan yang belum disimpan."),
      null,
    );
  });

  it("perubahan sah mengaktifkan simpan dan menampilkan penanda belum tersimpan", async () => {
    const { user } = render();
    await tungguForm();

    await isi(user, nama(), "Budi Baru");

    assert.equal(simpan().disabled, false);
    assert.ok(screen.getByText("Ada perubahan yang belum disimpan."));
  });

  it("satu isian salah tidak membuka simpan walau isian lain sah", async () => {
    const { user } = render();
    await tungguForm();

    await isi(user, nama(), "Budi Baru");
    await isi(user, telepon(), "abc");

    assert.equal(simpan().disabled, true);
  });

  it("batalkan mengembalikan nilai tersimpan, menghapus galat, dan menonaktifkan kedua tombol", async () => {
    const { user } = render();
    await tungguForm();
    await isi(user, nama(), "Bu");
    await isi(user, telepon(), "0877 1111 2222");

    await user.click(batalkan());

    assert.equal(nama().value, "Budi Santoso");
    assert.equal(telepon().value, "+62 812-3456-7890");
    assert.equal(screen.queryByText("Nama minimal 3 karakter"), null);
    assert.equal(simpan().disabled, true);
    assert.equal(batalkan().disabled, true);
    assert.equal(patchTerkirim().length, 0, "tidak ada yang dikirim");
  });
});

// ── Menyimpan ────────────────────────────────────────────────────────

describe("TabProfil: menyimpan", () => {
  it("ubah nama: PATCH hanya memuat nama dengan token CSRF; toast, ringkasan, dan form diperbarui; sesi dimuat ulang", async () => {
    const { user, router } = render();
    await tungguForm();
    await isi(user, nama(), "  Budi Baru ");

    await user.click(simpan());

    assert.ok(await screen.findByText("Profil diperbarui"));
    const kirim = patchTerkirim();
    assert.equal(kirim.length, 1);
    assert.deepEqual(kirim[0].body, { nama: "Budi Baru" });
    assert.equal(kirim[0].headers["x-csrf-token"], "csrf-uji");
    assert.ok(ringkasan().getByText("Budi Baru"));
    assert.equal(nama().value, "Budi Baru");
    assert.equal(simpan().disabled, true, "tidak ada perubahan tersisa");
    assert.equal(router.refresh, 1, "nama di sesi (topbar) ikut berubah");
    await waitFor(() =>
      assert.ok(
        fetchPalsu.ke("GET", "/api/auth/me").length >= 2,
        "dimuat ulang dari server",
      ),
    );
  });

  it("ubah telepon saja: PATCH memuat nomor E.164, tampilan memakai format baca, sesi tidak dimuat ulang", async () => {
    const { user, router } = render();
    await tungguForm();
    await isi(user, telepon(), "(0877) 1111-2222");

    await user.click(simpan());

    assert.ok(await screen.findByText("Profil diperbarui"));
    assert.deepEqual(patchTerkirim()[0].body, { telepon: "+6287711112222" });
    assert.equal(telepon().value, "+62 877-1111-2222");
    assert.equal(nama().value, "Budi Santoso");
    assert.equal(router.refresh, 0, "nama tidak berubah");
  });

  it("nama dan telepon sekaligus dikirim dalam satu permintaan", async () => {
    const { user } = render();
    await tungguForm();
    await isi(user, nama(), "Budi Baru");
    await isi(user, telepon(), "081111111111");

    await user.click(simpan());

    assert.ok(await screen.findByText("Profil diperbarui"));
    assert.equal(patchTerkirim().length, 1);
    assert.deepEqual(patchTerkirim()[0].body, {
      nama: "Budi Baru",
      telepon: "+6281111111111",
    });
  });

  it("mengosongkan telepon menghapus nomor: PATCH membawa string kosong", async () => {
    const { user } = render();
    await tungguForm();
    await isi(user, telepon(), "");

    await user.click(simpan());

    assert.ok(await screen.findByText("Profil diperbarui"));
    assert.deepEqual(patchTerkirim()[0].body, { telepon: "" });
    assert.equal(telepon().value, "");
  });

  it("Enter di kotak isian mengirim form seperti tombol simpan", async () => {
    const { user } = render();
    await tungguForm();
    await isi(user, nama(), "Budi Baru");

    await user.type(nama(), "{Enter}");

    assert.ok(await screen.findByText("Profil diperbarui"));
    assert.equal(patchTerkirim().length, 1);
  });

  it("tanpa perubahan, Enter tidak mengirim apa pun", async () => {
    const { user } = render();
    await tungguForm();

    await user.type(nama(), "{Enter}");

    assert.equal(patchTerkirim().length, 0);
  });

  it("nama di sesi sudah sama dengan nama baru: sesi tidak perlu dimuat ulang", async () => {
    const { user, router } = renderHalaman(<TabProfil />, {
      url: "/profil",
      sesi: sesiBudi({ nama: "Budi Baru" }),
    });
    await tungguForm();
    await isi(user, nama(), "Budi Baru");

    await user.click(simpan());

    assert.ok(await screen.findByText("Profil diperbarui"));
    assert.equal(router.refresh, 0);
  });

  it("pembaruan optimistis: ringkasan langsung berubah selagi server belum menjawab, isian dikunci", async () => {
    let lepas!: () => void;
    const tertahan = new Promise<void>((r) => (lepas = r));
    fetchPalsu.atur("PATCH /api/auth/me", async (req) => {
      await tertahan;
      server.nama = (req.body as { nama: string }).nama;
      return { user: server };
    });
    const { user } = render();
    await tungguForm();
    await isi(user, nama(), "Budi Baru");

    await user.click(simpan());

    await waitFor(() => assert.ok(ringkasan().getByText("Budi Baru")));
    assert.equal(simpan().disabled, true);
    assert.equal(
      nama().readOnly,
      true,
      "fokus tetap di kotak, tetapi tidak bisa diubah",
    );
    assert.equal(screen.queryByText("Profil diperbarui"), null);

    lepas();
    assert.ok(await screen.findByText("Profil diperbarui"));
    assert.equal(nama().readOnly, false);
  });

  it("simpan tidak menunggu muat ulang profil: toast tampil dan isian terbuka walau GET lanjutan lambat", async () => {
    const { user } = render();
    await tungguForm();
    fetchPalsu.atur("GET /api/auth/me", () => new Promise(() => {}));
    await isi(user, nama(), "Budi Baru");

    await user.click(simpan());

    assert.ok(await screen.findByText("Profil diperbarui"));
    assert.equal(nama().readOnly, false, "isian tidak tertahan oleh muat ulang");
    assert.equal(nama().value, "Budi Baru");
  });

  it("galat 400 dari API: pesan inline dan toast, ringkasan kembali, ketikan dipertahankan, bisa dicoba lagi", async () => {
    fetchPalsu.atur(
      "PATCH /api/auth/me",
      galatApi(400, "Nomor telepon tidak valid"),
    );
    const { user, router } = render();
    await tungguForm();
    // Muat ulang sesudah galat sengaja ditahan: yang mengembalikan ringkasan adalah pembatalan pembaruan
    // optimistis, bukan data baru dari server.
    fetchPalsu.atur("GET /api/auth/me", () => new Promise(() => {}));
    await isi(user, nama(), "Budi Baru");
    await isi(user, telepon(), "0877 1111 2222");

    await user.click(simpan());

    const peringatan = await screen.findByRole("alert");
    assert.equal(peringatan.textContent, "Nomor telepon tidak valid");
    assert.ok(await screen.findByText("Gagal menyimpan profil"), "toast galat");
    assert.ok(
      ringkasan().getByText("Budi Santoso"),
      "pembaruan optimistis dibatalkan",
    );
    assert.equal(nama().value, "Budi Baru", "ketikan tidak hilang");
    assert.equal(telepon().value, "0877 1111 2222");
    assert.equal(simpan().disabled, false, "bisa dicoba lagi");
    assert.equal(router.refresh, 0);

    // Mengetik lagi menghapus pesan galat server.
    await user.type(nama(), "x");
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("galat server (500): pesan umum, bukan isi galat mentah", async () => {
    fetchPalsu.atur(
      "PATCH /api/auth/me",
      galatApi(500, "pq: connection refused 10.0.0.5"),
    );
    const { user } = render();
    await tungguForm();
    await isi(user, nama(), "Budi Baru");

    await user.click(simpan());

    const peringatan = await screen.findByRole("alert");
    assert.equal(
      peringatan.textContent,
      "Tidak dapat terhubung ke server. Coba lagi.",
    );
    assert.equal(screen.queryByText(/pq:/), null);
  });
});

// ── Penugasan dan wilayah ────────────────────────────────────────────

describe("TabProfil: penugasan dan wilayah", () => {
  it("menampilkan lembaga, peran utama, lapis, dan cakupan kabupaten (hanya-baca)", async () => {
    render();
    await tungguForm();

    const bagian = within(
      screen
        .getByRole("heading", { name: "Penugasan & wilayah" })
        .closest("section")!,
    );
    assert.ok(bagian.getByText("FMIPA USK"));
    assert.ok(bagian.getByText("Koordinator Lapangan"));
    assert.ok(bagian.getByText("Lapis operasional wilayah"));
    assert.ok(bagian.getByText("Kab. Aceh Tengah"));
    assert.ok(bagian.getByText(/Cakupan kabupaten\/kota/));
    assert.equal(bagian.queryAllByRole("textbox").length, 0, "tidak ada isian");
  });

  it("nama kabupaten diambil dari master wilayah bila peran boleh membacanya", async () => {
    fetchPalsu.atur("GET /v1/master/wilayah", () =>
      amplop([
        {
          kode: "1106",
          nama: "KABUPATEN ACEH TENGAH RAYA",
          jenis: "Kabupaten",
          ibukota: "Takengon",
          lat: 4.62,
          lng: 96.85,
          provinsi_kode: "11",
        },
      ]),
    );
    render();
    await tungguForm();

    assert.ok(await screen.findByText("Kab. Aceh Tengah Raya"));
    assert.equal(fetchPalsu.ke("GET", "/v1/master/wilayah").length, 1);
  });

  it("peran tanpa izin master.lihat memakai data acuan dan tidak memanggil master wilayah", async () => {
    pasangApi({ ...PROFIL, peran: "OPS_MAHASISWA" });
    render({ role: "OPS_MAHASISWA" });
    await tungguForm();

    assert.ok(await screen.findByText("Kab. Aceh Tengah"));
    assert.equal(fetchPalsu.ke("GET", "/v1/master/wilayah").length, 0);
  });

  it("master wilayah gagal dimuat: tetap tampil nama dari data acuan", async () => {
    fetchPalsu.atur("GET /v1/master/wilayah", galatApi(403, "Akses ditolak"));
    render();
    await tungguForm();

    assert.ok(await screen.findByText("Kab. Aceh Tengah"));
  });

  it("tanpa kode kabupaten: cakupan provinsi", async () => {
    const { kabupaten_kode: _hapus, ...provinsi } = PROFIL;
    pasangApi({ ...provinsi, peran: "STRATEGIS_FMIPA", lapis: "STRATEGIS" });
    render({ role: "STRATEGIS_FMIPA", kabupatenKode: undefined });
    await tungguForm();

    assert.ok(await screen.findByText("Seluruh Aceh"));
    assert.ok(screen.getByText(/Cakupan provinsi/));
    assert.equal(fetchPalsu.ke("GET", "/v1/master/wilayah").length, 0);
  });

  it("hak akses: ringkas dan tertutup, dibuka lewat tombol dengan aria-expanded", async () => {
    const { user } = render({
      permissions: ["harga.lihat", "harga.input", "audit.lihat"],
    });
    await tungguForm();

    const tombol = screen.getByRole("button", { name: "Lihat hak akses (3)" });
    assert.equal(tombol.getAttribute("aria-expanded"), "false");
    assert.equal(screen.queryByText("Melihat data harga pangan"), null);

    await user.click(tombol);

    assert.equal(tombol.getAttribute("aria-expanded"), "true");
    const panel = document.getElementById(
      tombol.getAttribute("aria-controls") ?? "",
    );
    assert.ok(panel, "aria-controls menunjuk panel yang ada");
    assert.ok(within(panel).getByText("Melihat data harga pangan"));
    assert.ok(screen.getByText("Melihat data harga pangan"));
    assert.ok(screen.getByText("Menginput harga dari lapangan"));
    assert.ok(screen.getByText("Melihat jejak audit"));
    assert.equal(
      screen.getByText("Melihat jejak audit").getAttribute("title"),
      "audit.lihat",
    );
    assert.ok(
      screen.getByRole("list", { name: "Hak akses Pengguna dan audit" }),
    );

    await user.click(tombol);
    assert.equal(tombol.getAttribute("aria-expanded"), "false");
    assert.equal(screen.queryByText("Melihat data harga pangan"), null);
  });

  it("hak akses mengikuti klaim izin token sesi; tanpa klaim memakai matriks bawaan peran", async () => {
    const { unmount } = render({ permissions: ["kpi.lihat"] });
    await tungguForm();
    assert.ok(screen.getByRole("button", { name: "Lihat hak akses (1)" }));
    unmount();

    const sesi = sesiBudi();
    delete (sesi as { permissions?: unknown }).permissions;
    renderHalaman(<TabProfil />, { url: "/profil", sesi });
    await tungguForm();
    assert.ok(
      screen.getByRole("button", {
        name: `Lihat hak akses (${ROLE_PERMISSIONS.OPS_KOORDINATOR.length})`,
      }),
    );
  });

  it("izin kosong: keterangan tanpa tombol", async () => {
    render({ permissions: [] });
    await tungguForm();

    assert.ok(screen.getByText("Belum ada hak akses yang diberikan."));
    assert.equal(
      screen.queryByRole("button", { name: /Lihat hak akses/ }),
      null,
    );
  });
});

// ── Mode demo ────────────────────────────────────────────────────────

describe("TabProfilDemo (mode mock)", () => {
  it("menampilkan data sesi hanya-baca dengan catatan, tanpa memanggil API profil", async () => {
    const { user } = renderHalaman(<TabProfilDemo />, {
      url: "/profil",
      sesi: sesiBudi({ kabupatenKode: undefined }),
    });

    assert.ok(
      screen.getByText("Perubahan profil tidak tersedia dalam mode demo"),
    );
    assert.equal(nama().value, "Budi Santoso");
    assert.equal(email().value, "budi@garda.test");
    assert.equal(telepon().value, "");
    assert.equal(nama().disabled, true);
    assert.equal(telepon().disabled, true);
    assert.equal(simpan().disabled, true);
    assert.equal(batalkan().disabled, true);
    assert.ok(ringkasan().getByText("Koordinator Lapangan"));
    assert.ok(ringkasan().getByText("Aktif"));
    assert.equal(ringkasan().queryByText(/Bergabung/), null);

    await user.type(nama(), "x");
    assert.equal(nama().value, "Budi Santoso", "isian nonaktif tidak berubah");
    assert.equal(fetchPalsu.ke("GET", "/api/auth/me").length, 0);
    assert.equal(patchTerkirim().length, 0);
  });
});
