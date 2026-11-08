import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { configure, screen, waitFor, within } from "@testing-library/react";
import type { UserEvent } from "@testing-library/user-event";
import { toast } from "sonner";
import {
  PESAN_KONFIRMASI_BEDA,
  PESAN_SANDI_LEMAH,
} from "@/lib/auth/skema-sandi";
import { qk } from "@/lib/query/keys";
import {
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { FormGantiSandi } from "./form-ganti-sandi";
import type { ModeAkun } from "./mode-akun";

// Jaringan palsu dan toast butuh beberapa putaran render; beri kelonggaran saat mesin sibuk (CI).
configure({ asyncUtilTimeout: 3000 });

const MODE_TAB: ModeAkun = { jenis: "tab" };
const MODE_HALAMAN: ModeAkun = { jenis: "halaman", lanjut: "/operasional" };

const SANDI_LAMA = "Lama#2025xx";
const SANDI_BARU = "SandiBaru#2026";
const RUTE = "/api/auth/change-password";

let api: FetchPalsu;
afterEach(async () => {
  // Toast Sonner hidup di store global dan lolos dari cleanup render; tanpa dibuang, timernya menggantung proses tes.
  toast.dismiss();
  // Sisa jadwal React dan animasi diberi waktu selesai selagi DOM masih ada; tanpa jeda, tes terakhir berkas bisa
  // memicu "window is not defined" saat `after` di render.tsx melepas DOM.
  await new Promise((r) => setTimeout(r, 30));
  api?.pulihkan();
});

const kolomLama = () =>
  screen.getByLabelText("Kata sandi saat ini") as HTMLInputElement;
const kolomBaru = () =>
  screen.getByLabelText("Kata sandi baru") as HTMLInputElement;
const kolomUlang = () =>
  screen.getByLabelText("Ulangi kata sandi baru") as HTMLInputElement;

async function isi(
  user: UserEvent,
  nilai: { lama?: string; baru?: string; ulang?: string } = {},
) {
  const baru = nilai.baru ?? SANDI_BARU;
  await user.type(kolomLama(), nilai.lama ?? SANDI_LAMA);
  await user.type(kolomBaru(), baru);
  await user.type(kolomUlang(), nilai.ulang ?? baru);
}

/** Status tiap aturan kebijakan menurut teks tersembunyi di daftar "Aturan kata sandi". */
function statusAturan(): boolean[] {
  const daftar = screen.getByRole("list", { name: "Aturan kata sandi" });
  return within(daftar)
    .getAllByRole("listitem")
    .map((li) => (li.textContent ?? "").startsWith("Terpenuhi"));
}

describe("FormGantiSandi: tampilan", () => {
  it("tiga kolom berlabel, daftar aturan, info perangkat lain, dan tombol sesuai mode", () => {
    api = pasangFetchPalsu();
    const { unmount } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);
    assert.ok(kolomLama());
    assert.ok(kolomBaru());
    assert.ok(kolomUlang());
    assert.equal(kolomLama().type, "password");
    assert.equal(kolomLama().autocomplete, "current-password");
    assert.equal(kolomBaru().autocomplete, "new-password");
    assert.ok(screen.getByRole("list", { name: "Aturan kata sandi" }));
    assert.ok(
      screen.getByText(
        "Perangkat lain akan dikeluarkan setelah kata sandi diganti.",
      ),
    );
    assert.ok(screen.getByRole("button", { name: "Ganti kata sandi" }));
    unmount();

    renderHalaman(<FormGantiSandi mode={MODE_HALAMAN} />);
    assert.ok(screen.getByRole("button", { name: "Simpan dan lanjutkan" }));
    assert.ok(
      screen.getByText(
        "Perangkat lain akan dikeluarkan setelah kata sandi diganti.",
      ),
      "teks info yang sama di halaman alur wajib",
    );
    assert.equal(
      screen.queryByRole("checkbox"),
      null,
      "tanpa checkbox: perangkat lain selalu dikeluarkan",
    );
  });

  it("checklist aturan berubah langsung saat mengetik", async () => {
    api = pasangFetchPalsu();
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);
    const daftar = screen.getByRole("list", { name: "Aturan kata sandi" });
    assert.deepEqual(
      within(daftar)
        .getAllByRole("listitem")
        .map((li) => li.textContent?.replace(/^(Belum t|T)erpenuhi: /, "")),
      [
        "Minimal 8 karakter",
        "Mengandung huruf kapital",
        "Mengandung angka",
        "Mengandung karakter khusus",
        "Bukan kata sandi yang terlalu umum",
      ],
    );
    // [panjang, kapital, angka, khusus, umum]
    assert.deepEqual(statusAturan(), [false, false, false, false, false]);

    await user.type(kolomBaru(), "abc");
    assert.deepEqual(statusAturan(), [false, false, false, false, true]);

    await user.type(kolomBaru(), "DEFG");
    assert.deepEqual(statusAturan(), [false, true, false, false, true]);

    await user.type(kolomBaru(), "1");
    assert.deepEqual(statusAturan(), [true, true, true, false, true]);

    await user.type(kolomBaru(), "!");
    assert.deepEqual(statusAturan(), [true, true, true, true, true]);

    await user.clear(kolomBaru());
    await user.type(kolomBaru(), "password");
    assert.deepEqual(
      statusAturan(),
      [true, false, false, false, false],
      "kata sandi umum menggagalkan aturan terakhir",
    );
  });

  it("meter kekuatan: tersembunyi saat kosong, lalu Lemah, Sedang, Kuat mengikuti isian", async () => {
    api = pasangFetchPalsu();
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);
    const meter = () =>
      screen.queryByRole("meter", { name: "Kekuatan kata sandi" });
    assert.equal(meter(), null, "kosong: meter tidak tampil");

    await user.type(kolomBaru(), "ab");
    assert.equal(meter()?.getAttribute("aria-valuetext"), "Lemah");
    assert.equal(meter()?.getAttribute("aria-valuenow"), "1");

    await user.clear(kolomBaru());
    await user.type(kolomBaru(), "Abcdef1!");
    assert.equal(meter()?.getAttribute("aria-valuetext"), "Sedang");
    assert.equal(meter()?.getAttribute("aria-valuenow"), "2");

    await user.type(kolomBaru(), "xyz!");
    assert.equal(meter()?.getAttribute("aria-valuetext"), "Kuat");
    assert.equal(meter()?.getAttribute("aria-valuenow"), "3");
    assert.ok(screen.getByText("Kuat"));

    await user.clear(kolomBaru());
    assert.equal(meter(), null, "dikosongkan lagi: meter hilang");
  });

  it("meter tidak menggeser tata letak: wadahnya selalu ada dengan tinggi tetap", async () => {
    api = pasangFetchPalsu();
    const { container, user } = renderHalaman(
      <FormGantiSandi mode={MODE_TAB} />,
    );
    const wadah = () => container.querySelector("div.h-5");
    assert.ok(wadah(), "wadah meter ada walau meter belum tampil");
    await user.type(kolomBaru(), "a");
    assert.ok(wadah()?.querySelector("[role='meter']"));
  });
});

describe("FormGantiSandi: validasi di klien", () => {
  it("konfirmasi yang tidak cocok ditolak di kolom konfirmasi tanpa memanggil server", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: { success: true } });
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);

    await isi(user, { ulang: "SandiBaru#2027" });
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    assert.equal(
      (await screen.findByText(PESAN_KONFIRMASI_BEDA)).id,
      "konfirmasi-galat",
    );
    assert.equal(kolomUlang().getAttribute("aria-invalid"), "true");
    assert.equal(api.ke("POST", RUTE).length, 0);
    assert.equal(kolomBaru().value, SANDI_BARU, "isian dipertahankan");
  });

  it("kata sandi baru yang melanggar kebijakan ditolak di kolom baru tanpa memanggil server", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: { success: true } });
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);

    await isi(user, { baru: "lemah", ulang: "lemah" });
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    assert.equal((await screen.findByText(PESAN_SANDI_LEMAH)).id, "baru-galat");
    assert.equal(api.ke("POST", RUTE).length, 0);
  });

  it("kata sandi saat ini wajib diisi", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: { success: true } });
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);

    await user.type(kolomBaru(), SANDI_BARU);
    await user.type(kolomUlang(), SANDI_BARU);
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    assert.ok(await screen.findByText("Kata sandi saat ini wajib diisi."));
    assert.equal(api.ke("POST", RUTE).length, 0);
  });
});

describe("FormGantiSandi: mode tab (tetap di tempat)", () => {
  it("sukses: kirim kata sandi lama dan baru, toast, form dikosongkan, tanpa pindah halaman, data disegarkan", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: { success: true } });
    const { user, router, qc } = renderHalaman(
      <FormGantiSandi mode={MODE_TAB} />,
      { url: "/profil?tab=ganti-password" },
    );
    qc.setQueryData(qk.profil.saya(), { id: "admin-1" });
    qc.setQueryData(qk.sesi.aktif(), []);

    await isi(user);
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    assert.ok(await screen.findByText("Kata sandi diganti"));
    assert.ok(screen.getByText("Perangkat lain telah dikeluarkan."));
    const [kiriman] = api.ke("POST", RUTE);
    assert.deepEqual(kiriman.body, {
      old_password: SANDI_LAMA,
      new_password: SANDI_BARU,
    });

    await waitFor(() => assert.equal(kolomLama().value, ""));
    assert.equal(kolomBaru().value, "");
    assert.equal(kolomUlang().value, "");
    assert.equal(
      screen.queryByRole("meter"),
      null,
      "meter ikut hilang karena isian dikosongkan",
    );
    assert.deepEqual(router.push, []);
    assert.deepEqual(router.replace, []);
    assert.equal(router.refresh, 0);
    assert.equal(
      window.location.pathname + window.location.search,
      "/profil?tab=ganti-password",
    );
    assert.equal(
      qc.getQueryState(qk.profil.saya())?.isInvalidated,
      true,
      "data akun dimuat ulang",
    );
    assert.equal(
      qc.getQueryState(qk.sesi.aktif())?.isInvalidated,
      true,
      "daftar sesi dimuat ulang: perangkat lain sudah dikeluarkan server",
    );
    // Form siap dipakai lagi.
    assert.equal(
      (
        screen.getByRole("button", {
          name: "Ganti kata sandi",
        }) as HTMLButtonElement
      ).disabled,
      false,
    );
  });

  it("kata sandi saat ini salah (401): pesan server tampil, isian dipertahankan, tidak ada toast sukses", async () => {
    api = pasangFetchPalsu({
      [`POST ${RUTE}`]: galatApi(401, "Password lama salah"),
    });
    const { user, router } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);

    await isi(user);
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Password lama salah/);
    assert.equal(kolomLama().value, SANDI_LAMA);
    assert.equal(kolomBaru().value, SANDI_BARU);
    assert.equal(screen.queryByText("Kata sandi diganti"), null);
    assert.equal(router.refresh, 0);
    // Judul toast gagal tampil bersama pesan servernya.
    assert.ok(await screen.findByText("Gagal mengganti kata sandi"));
  });
});

describe("FormGantiSandi: mode halaman (alur wajib)", () => {
  it("sukses: toast pengalihan, berpindah ke tujuan semula dan menyegarkan sesi (router.refresh)", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: { success: true } });
    const { user, router } = renderHalaman(
      <FormGantiSandi mode={MODE_HALAMAN} />,
      { url: "/ganti-password?lanjut=%2Foperasional" },
    );

    await isi(user);
    await user.click(
      screen.getByRole("button", { name: "Simpan dan lanjutkan" }),
    );

    assert.ok(await screen.findByText("Kata sandi diganti"));
    assert.ok(screen.getByText("Mengalihkan ke ruang kerja Anda…"));
    await waitFor(() => assert.deepEqual(router.replace, ["/operasional"]));
    assert.equal(
      router.refresh,
      1,
      "klaim wajib ganti hilang: root layout dirender ulang",
    );
    assert.deepEqual(router.push, []);
    assert.equal(api.ke("POST", RUTE).length, 1);
    assert.equal(
      screen.queryByText("Perangkat lain telah dikeluarkan."),
      null,
      "deskripsi toast mode tab tidak dipakai di alur wajib",
    );
  });

  it("tujuan yang bukan path internal disaring menjadi beranda", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: { success: true } });
    const { user, router } = renderHalaman(
      <FormGantiSandi mode={{ jenis: "halaman", lanjut: "//evil.example" }} />,
    );

    await isi(user);
    await user.click(
      screen.getByRole("button", { name: "Simpan dan lanjutkan" }),
    );

    await waitFor(() => assert.deepEqual(router.replace, ["/"]));
  });

  it("gagal: tetap di halaman, tanpa pindah dan tanpa penyegaran sesi", async () => {
    api = pasangFetchPalsu({
      [`POST ${RUTE}`]: galatApi(
        400,
        "Password baru tidak boleh sama dengan password lama",
      ),
    });
    const { user, router } = renderHalaman(
      <FormGantiSandi mode={MODE_HALAMAN} />,
    );

    await isi(user);
    await user.click(
      screen.getByRole("button", { name: "Simpan dan lanjutkan" }),
    );

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /tidak boleh sama dengan password lama/,
    );
    assert.deepEqual(router.replace, []);
    assert.equal(router.refresh, 0);
  });
});

describe("FormGantiSandi: akun terkunci dan batas laju", () => {
  /** Jawaban BFF untuk akun terkunci (lib/bff/galat-auth): kode mesin stabil dan sisa waktu tunggu. */
  const terkunci = (detik: number) => ({
    __status: 423,
    body: {
      error: true,
      message: "Akun terkunci karena terlalu banyak percobaan gagal.",
      kode: "ACCOUNT_LOCKED",
      sisa_detik: detik,
    },
  });

  it("423: pesan dengan hitung mundur (role timer) dan tombol dinonaktifkan selama menunggu", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: terkunci(90) });
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);

    await isi(user);
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    const alert = await screen.findByRole("alert");
    assert.match(
      alert.textContent ?? "",
      /Akun terkunci karena terlalu banyak percobaan gagal\. Coba lagi dalam 1 menit (30|29) detik\./,
    );
    const timer = within(alert).getByRole("timer");
    assert.match(timer.textContent ?? "", /^1 menit (30|29) detik$/);
    await waitFor(() =>
      assert.equal(
        (
          screen.getByRole("button", {
            name: "Ganti kata sandi",
          }) as HTMLButtonElement
        ).disabled,
        true,
      ),
    );
    assert.equal(api.ke("POST", RUTE).length, 1);
  });

  it("hitung mundur berjalan sampai habis: pesan 'waktu tunggu selesai' dan tombol aktif lagi", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: terkunci(2) });
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);

    await isi(user);
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));
    await screen.findByRole("timer");

    await waitFor(
      () =>
        assert.match(
          screen.getByRole("alert").textContent ?? "",
          /Waktu tunggu selesai\. Silakan coba lagi\./,
        ),
      { timeout: 5000 },
    );
    assert.equal(screen.queryByRole("timer"), null);
    assert.equal(
      (
        screen.getByRole("button", {
          name: "Ganti kata sandi",
        }) as HTMLButtonElement
      ).disabled,
      false,
    );
  });

  it("429 dari batas laju BFF: pesan batas laju dengan hitung mundur", async () => {
    api = pasangFetchPalsu({
      [`POST ${RUTE}`]: {
        __status: 429,
        body: {
          error: true,
          message: "Terlalu banyak percobaan dari perangkat ini.",
          kode: "RATE_LIMITED",
          sisa_detik: 45,
        },
      },
    });
    const { user } = renderHalaman(<FormGantiSandi mode={MODE_TAB} />);

    await isi(user);
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    const alert = await screen.findByRole("alert");
    assert.match(
      alert.textContent ?? "",
      /Terlalu banyak percobaan dari perangkat ini\. Coba lagi dalam (45|44) detik\./,
    );
    assert.match(
      within(alert).getByRole("timer").textContent ?? "",
      /^(45|44) detik$/,
    );
  });
});
