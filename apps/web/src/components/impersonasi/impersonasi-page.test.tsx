import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import type { KebijakanWire } from "@/lib/api/impersonasi";
import {
  kebijakanFixture,
  riwayatFixture,
  sesiImpersonate,
  tangkapNavigasi,
  targetsFixture,
  tuntaskanTimer,
} from "@/test/impersonasi";
import {
  galatApi,
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
  type PermintaanFetch,
} from "@/test/render";
import { ImpersonasiPage } from "./impersonasi-page";

const POLICY = "/api/impersonation/policy";
const RIWAYAT = "/api/impersonation/riwayat";
const TARGETS = "/api/impersonation/targets";

let api: FetchPalsu;
let nav: ReturnType<typeof tangkapNavigasi>;
beforeEach(() => {
  nav = tangkapNavigasi();
});
afterEach(() => {
  api?.pulihkan();
  nav.pulihkan();
});
afterEach(tuntaskanTimer);

const galatBff = (status: number, message: string, kode: string) => ({
  __status: status,
  body: { error: true, message, kode },
});

const renderPage = () =>
  renderHalaman(<ImpersonasiPage />, { sesi: penggunaSesi({ role: "ADMIN_SISTEM" }), url: "/impersonasi" });

// ── Halaman ──────────────────────────────────────────────────────────

describe("ImpersonasiPage", () => {
  it("tanpa izin impersonasi.kelola (mis. sedang impersonate peran lain): keadaan 'di luar kewenangan', tanpa memuat apa pun", () => {
    api = pasangFetchPalsu();
    renderHalaman(<ImpersonasiPage />, { sesi: sesiImpersonate("OPS_KOORDINATOR"), url: "/impersonasi" });
    assert.ok(screen.getByText("Di luar kewenangan peran Anda"));
    assert.equal(api.panggilan.length, 0);
  });

  it("superadmin: tiga tab (Kebijakan, Impersonate cepat, Riwayat); Kebijakan terbuka lebih dulu", async () => {
    api = pasangFetchPalsu({ [`GET ${POLICY}`]: kebijakanFixture() });
    renderPage();
    assert.ok(screen.getByRole("heading", { name: "Impersonasi", level: 1 }));
    const tabs = screen.getAllByRole("tab").map((t) => t.textContent);
    assert.deepEqual(tabs, ["Kebijakan", "Impersonate cepat", "Riwayat"]);
    assert.equal(screen.getByRole("tab", { name: "Kebijakan" }).getAttribute("aria-selected"), "true");
    await screen.findByText("Pengaturan umum");
  });
});

// ── Kebijakan ────────────────────────────────────────────────────────

describe("Tab Kebijakan", () => {
  /** Kebijakan di "server": PUT menggantinya, GET membacanya. */
  function pasangKebijakan(awal: KebijakanWire = kebijakanFixture()) {
    const server = { sekarang: awal };
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: () => server.sekarang,
      [`PUT ${POLICY}`]: (req: PermintaanFetch) => {
        const b = req.body as { aktif: boolean; wajib_alasan: boolean; durasi_maks_menit: number | null };
        server.sekarang = {
          ...server.sekarang,
          versi: server.sekarang.versi + 1,
          aktif: b.aktif,
          wajib_alasan: b.wajib_alasan,
          durasi_maks_menit: b.durasi_maks_menit,
        };
        return server.sekarang;
      },
    });
    return server;
  }
  const simpan = () => screen.getByRole("button", { name: /Simpan kebijakan|Menyimpan/ }) as HTMLButtonElement;

  it("menampilkan pengaturan dan pelaku dari server; simpan nonaktif sampai ada perubahan", async () => {
    pasangKebijakan();
    renderPage();
    await screen.findByText("Pengaturan umum");

    assert.equal(screen.getByRole("checkbox", { name: "Impersonasi aktif" }).getAttribute("aria-checked"), "true");
    assert.equal(screen.getByRole("checkbox", { name: "Alasan wajib diisi" }).getAttribute("aria-checked"), "true");
    assert.equal((screen.getByLabelText(/Durasi maksimal sesi/) as HTMLInputElement).value, "60");
    const kartu = screen.getByRole("article", { name: "Pelaku Unit Mutu & Pascapanen" });
    assert.equal(
      within(kartu).getByRole("checkbox", { name: "Agen Lapangan Mahasiswa" }).getAttribute("aria-checked"),
      "true",
    );
    assert.equal(
      within(kartu).getByRole("checkbox", { name: "Koordinator Lapangan" }).getAttribute("aria-checked"),
      "false",
    );
    assert.equal(within(kartu).queryByRole("checkbox", { name: "Administrator Sistem" }), null, "ADMIN_SISTEM bukan target");
    assert.equal(simpan().disabled, true);
    assert.match(document.body.textContent ?? "", /Terakhir disimpan .* oleh Super Admin/);
  });

  it("mengubah pengaturan lalu menyimpan: PUT dengan versi (kunci optimistis), lalu draf mengikuti versi baru", async () => {
    pasangKebijakan();
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");

    await user.click(screen.getByRole("checkbox", { name: "Impersonasi aktif" }));
    const durasi = screen.getByLabelText(/Durasi maksimal sesi/);
    await user.clear(durasi);
    await user.type(durasi, "30");
    assert.equal(simpan().disabled, false);
    await user.click(simpan());

    await screen.findByText("Kebijakan impersonasi disimpan");
    const put = api.ke("PUT", POLICY);
    assert.equal(put.length, 1);
    assert.deepEqual(put[0].body, {
      versi: 3,
      aktif: false,
      wajib_alasan: true,
      durasi_maks_menit: 30,
      pelaku: [
        {
          tipe: "PERAN",
          peran: "MANAJER_MUTU",
          aktif: true,
          target: [{ tipe: "PERAN", peran: "OPS_MAHASISWA" }],
        },
      ],
    });
    // Sesudah simpan draf dimulai ulang dari server (versi 4): tidak ada perubahan tersisa.
    await waitFor(() => assert.equal(simpan().disabled, true));
    assert.equal(screen.getByRole("checkbox", { name: "Impersonasi aktif" }).getAttribute("aria-checked"), "false");
  });

  it("durasi kosong = tanpa batas dikirim sebagai null", async () => {
    pasangKebijakan();
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");
    await user.clear(screen.getByLabelText(/Durasi maksimal sesi/));
    await user.click(simpan());
    await screen.findByText("Kebijakan impersonasi disimpan");
    assert.equal((api.ke("PUT", POLICY)[0].body as { durasi_maks_menit: unknown }).durasi_maks_menit, null);
  });

  it("durasi di luar batas: galat tampil dan simpan nonaktif tanpa mengirim apa pun", async () => {
    pasangKebijakan();
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");
    const durasi = screen.getByLabelText(/Durasi maksimal sesi/);
    await user.clear(durasi);
    await user.type(durasi, "2");
    assert.match((await screen.findByRole("alert")).textContent ?? "", /5 sampai 1440 menit/);
    assert.equal(simpan().disabled, true);
    assert.equal(api.ke("PUT", POLICY).length, 0);
  });

  it("konflik versi (superadmin lain menyimpan lebih dulu): toast, data dimuat ulang, draf dibuang", async () => {
    const server = pasangKebijakan();
    api.atur(`PUT ${POLICY}`, () => {
      // Superadmin lain menyimpan lebih dulu: server berpindah ke versi 5 dengan durasi 15.
      server.sekarang = { ...server.sekarang, versi: 5, durasi_maks_menit: 15 };
      return galatBff(409, "Kebijakan sudah diubah superadmin lain.", "VERSI_KEBIJAKAN_BERUBAH");
    });
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");
    await user.click(screen.getByRole("checkbox", { name: "Alasan wajib diisi" }));
    await user.click(simpan());

    await screen.findByText("Kebijakan sudah diubah superadmin lain");
    await waitFor(() => assert.equal((screen.getByLabelText(/Durasi maksimal sesi/) as HTMLInputElement).value, "15"));
    // Draf lama (alasan tidak wajib) dibuang: kembali ke keadaan server (wajib alasan).
    assert.equal(screen.getByRole("checkbox", { name: "Alasan wajib diisi" }).getAttribute("aria-checked"), "true");
    assert.equal(simpan().disabled, true);
  });

  it("penolakan lain (mis. 400) mempertahankan draf dan menampilkan toast galat", async () => {
    pasangKebijakan();
    api.atur(`PUT ${POLICY}`, galatApi(400, "Pengguna sasaran tidak ditemukan."));
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");
    await user.click(screen.getByRole("checkbox", { name: "Impersonasi aktif" }));
    await user.click(simpan());
    await screen.findByText("Gagal menyimpan kebijakan impersonasi");
    assert.equal(screen.getByRole("checkbox", { name: "Impersonasi aktif" }).getAttribute("aria-checked"), "false");
    assert.equal(simpan().disabled, false);
  });

  it("tambah pelaku berperan: wajib punya target sebelum bisa disimpan; 'semua peran' memilih 13 peran tanpa Administrator Sistem", async () => {
    pasangKebijakan(kebijakanFixture({ pelaku: [] }));
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");
    assert.ok(screen.getByText(/Belum ada pelaku tambahan/));

    const pilih = screen.getByLabelText("Tambah pelaku berdasarkan peran") as HTMLSelectElement;
    assert.ok(![...pilih.options].some((o) => o.textContent === "Administrator Sistem"), "ADMIN_SISTEM tidak ditawarkan");
    await user.selectOptions(pilih, "MANAJER_RISIKO");

    const kartu = await screen.findByRole("article", { name: "Pelaku Unit Monitoring & Risiko" });
    assert.match((await screen.findByRole("alert")).textContent ?? "", /aktif tetapi belum punya target/);
    assert.equal(simpan().disabled, true);

    await user.click(within(kartu).getByRole("button", { name: "Semua peran (tanpa Administrator Sistem)" }));
    const kotak = within(kartu).getAllByRole("checkbox").filter((c) => c.id.includes("-target-"));
    assert.equal(kotak.length, 13);
    assert.ok(kotak.every((c) => c.getAttribute("aria-checked") === "true"));
    assert.equal(screen.queryByRole("alert"), null);
    await user.click(simpan());

    await screen.findByText("Kebijakan impersonasi disimpan");
    const pelaku = (api.ke("PUT", POLICY)[0].body as { pelaku: { tipe: string; peran: string; target: unknown[] }[] }).pelaku;
    assert.equal(pelaku.length, 1);
    assert.equal(pelaku[0].peran, "MANAJER_RISIKO");
    assert.equal(pelaku[0].target.length, 13);
    assert.ok(!JSON.stringify(pelaku).includes("ADMIN_SISTEM"));
    // Peran yang sudah jadi pelaku tidak ditawarkan lagi.
  });

  it("pintasan 'Lapis operasional saja' dan 'Kosongkan peran'", async () => {
    pasangKebijakan();
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");
    const kartu = screen.getByRole("article", { name: "Pelaku Unit Mutu & Pascapanen" });

    await user.click(within(kartu).getByRole("button", { name: "Lapis operasional saja" }));
    const aktif = () =>
      within(kartu)
        .getAllByRole("checkbox")
        .filter((c) => c.id.includes("-target-") && c.getAttribute("aria-checked") === "true")
        .map((c) => c.id.split("-target-")[1]);
    assert.deepEqual(aktif(), ["OPS_KOORDINATOR", "OPS_AGREGATOR", "OPS_MAHASISWA"]);
    await user.click(within(kartu).getByRole("button", { name: "Kosongkan peran" }));
    assert.deepEqual(aktif(), []);
  });

  it("hapus pelaku dan 'Buang perubahan' mengembalikan keadaan server", async () => {
    pasangKebijakan();
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");

    await user.click(screen.getByRole("button", { name: "Hapus pelaku Unit Mutu & Pascapanen" }));
    assert.equal(screen.queryByRole("article", { name: "Pelaku Unit Mutu & Pascapanen" }), null);
    assert.equal(simpan().disabled, false);

    await user.click(screen.getByRole("button", { name: "Buang perubahan" }));
    assert.ok(screen.getByRole("article", { name: "Pelaku Unit Mutu & Pascapanen" }));
    assert.equal(simpan().disabled, true);
  });

  it("tambah pelaku dan target berdasarkan akun lewat pencarian pengguna (Administrator Sistem tidak muncul)", async () => {
    pasangKebijakan(kebijakanFixture({ pelaku: [] }));
    api.atur("GET /api/pengguna", () => ({
      items: [
        { id: "u-qa", nama: "Qa Satu", email: "qa@x.id", peran: "MANAJER_DATA" },
        { id: "u-adm", nama: "Qa Admin", email: "adm@x.id", peran: "ADMIN_SISTEM" },
      ],
      total: 2,
      page: 1,
      limit: 25,
    }));
    const { user } = renderPage();
    await screen.findByText("Pengaturan umum");

    await user.type(screen.getByRole("textbox", { name: "Cari akun pelaku" }), "qa");
    const hasil = screen.getByRole("list", { name: /hasil cari akun pelaku/i });
    assert.ok(within(hasil).getByText("Mencari…"), "selama jeda ketik tidak mengklaim 'tidak ada'");
    const pilihan = await within(hasil).findByRole("button", { name: /Qa Satu/ });
    assert.equal(within(hasil).queryByText("Qa Admin"), null, "Administrator Sistem tidak ditawarkan");
    await user.click(pilihan);

    const kartu = await screen.findByRole("article", { name: "Pelaku Qa Satu" });
    assert.equal(api.ke("GET", "/api/pengguna")[0].query.get("status"), "AKTIF");
    assert.equal(api.ke("GET", "/api/pengguna")[0].query.get("search"), "qa");

    // Target akun pada pelaku itu.
    await user.type(within(kartu).getByRole("textbox", { name: "Cari akun target Qa Satu" }), "qa");
    const hasilTarget = within(kartu).getByRole("list", { name: /hasil cari akun target/i });
    await user.click(await within(hasilTarget).findByRole("button", { name: /Qa Satu/ }));
    assert.ok(within(kartu).getByRole("button", { name: "Hapus target Qa Satu" }));
    await user.click(simpan());

    await screen.findByText("Kebijakan impersonasi disimpan");
    assert.deepEqual((api.ke("PUT", POLICY)[0].body as { pelaku: unknown[] }).pelaku, [
      {
        tipe: "PENGGUNA",
        pengguna_id: "u-qa",
        aktif: true,
        target: [{ tipe: "PENGGUNA", pengguna_id: "u-qa" }],
      },
    ]);
  });

  it("403 dari API (bukan superadmin): panel galat dengan pesan server, tanpa editor", async () => {
    api = pasangFetchPalsu({ [`GET ${POLICY}`]: galatApi(403, "Akses ditolak") });
    renderPage();
    await screen.findByText("Kebijakan impersonasi gagal dimuat");
    assert.ok(screen.getByText("Akses ditolak"));
    assert.equal(screen.queryByText("Pengaturan umum"), null);
  });
});

// ── Impersonate cepat ────────────────────────────────────────────────

describe("Tab Impersonate cepat", () => {
  it("daftar peran dan akun per lapis; tombol Impersonate membuka dialog dengan akun itu terpilih lalu memulai", async () => {
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${TARGETS}`]: targetsFixture(),
      [`POST /api/impersonation/start`]: {
        user: { id: "a-k2", nama: "Siti", peran: "OPS_KOORDINATOR" },
        redirectTo: "/",
      },
    });
    const { user } = renderPage();
    await user.click(screen.getByRole("tab", { name: "Impersonate cepat" }));

    await screen.findByRole("button", { name: "Impersonate Siti Koordinator" });
    assert.ok(screen.getByRole("region", { name: /Lapis strategis/ }));
    assert.ok(screen.getByRole("region", { name: /Lapis operasional/ }));
    await user.click(screen.getByRole("button", { name: "Impersonate Siti Koordinator" }));

    const dialog = await screen.findByRole("dialog", { name: "Beralih / Impersonate Role" });
    const siti = await within(dialog).findByRole("radio", { name: /Siti Koordinator/ });
    assert.equal(siti.getAttribute("aria-checked"), "true");
    await user.type(within(dialog).getByRole("textbox", { name: /Alasan/ }), "uji koordinator");
    await user.click(within(dialog).getByRole("button", { name: "Mulai Impersonate" }));

    await waitFor(() => assert.deepEqual(nav.tujuan, ["/"]));
    assert.equal(
      (api.ke("POST", "/api/impersonation/start")[0].body as { target_pengguna_id: string }).target_pengguna_id,
      "a-k2",
    );
  });

  it("kebijakan nonaktif dan tanpa akun: keadaan kosong yang menjelaskan", async () => {
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${TARGETS}`]: targetsFixture({ aktif: false }),
    });
    const { user } = renderPage();
    await user.click(screen.getByRole("tab", { name: "Impersonate cepat" }));
    assert.ok(await screen.findByText("Impersonasi sedang dinonaktifkan"));
  });
});

// ── Riwayat ──────────────────────────────────────────────────────────

describe("Tab Riwayat", () => {
  const aktif = riwayatFixture();
  const selesai = riwayatFixture({
    id: "44444444-4444-4444-8444-444444444444",
    sasaran: { id: "x", nama: "Ani", email: "ani@x.id", peran: "OPS_MAHASISWA" },
    aktif: false,
    berakhir_pada: "2026-11-17T10:30:00Z",
    alasan_berakhir: "STOP",
    alasan: null,
  });

  async function bukaRiwayat() {
    const { user } = renderPage();
    await user.click(screen.getByRole("tab", { name: "Riwayat" }));
    return { user };
  }

  it("menampilkan pelaku, sasaran, alasan, waktu, dan status; 'Akhiri' hanya pada sesi aktif", async () => {
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${RIWAYAT}`]: { items: [aktif, selesai] },
    });
    await bukaRiwayat();

    await screen.findByText("uji dasbor");
    const baris = screen.getAllByRole("row").slice(1);
    assert.equal(baris.length, 2);
    assert.match(baris[0].textContent ?? "", /Super Admin/);
    assert.match(baris[0].textContent ?? "", /Zulkifli/);
    assert.match(baris[0].textContent ?? "", /Aktif/);
    assert.match(baris[1].textContent ?? "", /Dihentikan pelaku/);
    assert.ok(within(baris[0]).getByRole("button", { name: /Akhiri sesi Super Admin sebagai Zulkifli/ }));
    assert.equal(within(baris[1]).queryByRole("button"), null);
    assert.equal(api.ke("GET", RIWAYAT)[0].query.get("status"), "semua");
  });

  it("saringan 'Aktif' meminta status=aktif ke server", async () => {
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${RIWAYAT}`]: (req: PermintaanFetch) => ({ items: req.query.get("status") === "aktif" ? [aktif] : [aktif, selesai] }),
    });
    const { user } = await bukaRiwayat();
    await screen.findByText("uji dasbor");
    await user.click(screen.getByRole("tab", { name: "Aktif" }));
    await waitFor(() => assert.equal(screen.getAllByRole("row").length, 2));
    assert.ok(api.ke("GET", RIWAYAT).some((r) => r.query.get("status") === "aktif"));
  });

  it("paginasi cursor: 'Muat lebih banyak' meminta halaman berikutnya dan menambahkannya", async () => {
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${RIWAYAT}`]: (req: PermintaanFetch) =>
        req.query.get("cursor") === "c2" ? { items: [selesai] } : { items: [aktif], next_cursor: "c2" },
    });
    const { user } = await bukaRiwayat();
    await screen.findByText("uji dasbor");
    await user.click(screen.getByRole("button", { name: "Muat lebih banyak" }));
    await waitFor(() => assert.equal(screen.getAllByRole("row").length, 3));
    assert.equal(screen.queryByRole("button", { name: "Muat lebih banyak" }), null);
  });

  it("Akhiri meminta konfirmasi; membatalkan tidak mengirim; mengonfirmasi mengirim POST /akhiri", async () => {
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${RIWAYAT}`]: { items: [aktif] },
      [`POST ${RIWAYAT}/${aktif.id}/akhiri`]: { berakhir: true },
    });
    const { user } = await bukaRiwayat();
    await user.click(await screen.findByRole("button", { name: /Akhiri sesi Super Admin/ }));

    let dialog = await screen.findByRole("alertdialog");
    assert.match(dialog.textContent ?? "", /Super Admin akan keluar dari sesi sebagai Zulkifli/);
    await user.click(within(dialog).getByRole("button", { name: "Batal" }));
    assert.equal(api.panggilan.filter((p) => p.method === "POST").length, 0);

    await user.click(await screen.findByRole("button", { name: /Akhiri sesi Super Admin/ }));
    dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Akhiri sesi" }));
    await screen.findByText("Sesi impersonasi diakhiri");
    assert.equal(api.ke("POST", `${RIWAYAT}/${aktif.id}/akhiri`).length, 1);
  });

  it("kosong dan galat", async () => {
    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${RIWAYAT}`]: { items: [] },
    });
    await bukaRiwayat();
    assert.ok(await screen.findByText("Belum ada riwayat impersonasi"));
    api.pulihkan();

    api = pasangFetchPalsu({
      [`GET ${POLICY}`]: kebijakanFixture(),
      [`GET ${RIWAYAT}`]: galatApi(403, "Akses ditolak"),
    });
    document.body.innerHTML = "";
    await bukaRiwayat();
    assert.ok(await screen.findByText("Riwayat impersonasi gagal dimuat"));
  });
});
