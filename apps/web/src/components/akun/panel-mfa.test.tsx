import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { configure, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { qk } from "@/lib/query/keys";
import {
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  tangkapUnduhan,
  type FetchPalsu,
} from "@/test/render";
import type { ModeAkun } from "./mode-akun";
import { PanelMfa } from "./panel-mfa";

// Dialog Radix dan jaringan palsu butuh beberapa putaran render; beri kelonggaran saat mesin sibuk (CI).
configure({ asyncUtilTimeout: 3000 });

const MODE_TAB: ModeAkun = { jenis: "tab" };
const MODE_HALAMAN: ModeAkun = { jenis: "halaman", lanjut: "/operasional" };

const AKUN_BELUM = {
  id: "u-1",
  nama: "Admin Satu",
  email: "admin@garda.test",
  peran: "ADMIN_SISTEM",
  lapis: "STRATEGIS",
  lembaga: "BPPSDMP",
  mfa_aktif: false,
  status: "AKTIF",
  dibuat_pada: "2026-10-01T08:00:00Z",
  wajib_ganti_password: false,
  mfa_setup_diwajibkan: false,
  mfa_wajib: false,
};
const AKUN_AKTIF = { ...AKUN_BELUM, mfa_aktif: true, kode_cadangan_tersisa: 8 };

const KODE = Array.from(
  { length: 10 },
  (_, i) => `AAAA-${String(i).padStart(4, "0")}`,
);
const KODE_BARU = Array.from(
  { length: 10 },
  (_, i) => `BBBB-${String(i).padStart(4, "0")}`,
);
const SETUP = {
  secret: "JBSWY3DPEHPK3PXP",
  provisioning_uri:
    "otpauth://totp/Garda:admin@garda.test?secret=JBSWY3DPEHPK3PXP",
  qr_svg:
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>',
};

let api: FetchPalsu;
afterEach(async () => {
  // Toast Sonner hidup di store global dan lolos dari cleanup render; tanpa dibuang, timernya menggantung proses tes.
  toast.dismiss();
  // Sisa jadwal React dan animasi diberi waktu selesai selagi DOM masih ada; tanpa jeda, tes terakhir berkas bisa
  // memicu "window is not defined" saat `after` di render.tsx melepas DOM.
  await new Promise((r) => setTimeout(r, 30));
  api?.pulihkan();
  delete (navigator as unknown as { clipboard?: unknown }).clipboard;
});

/**
 * Server palsu yang menyimpan keadaan akun: aktivasi dan penonaktifan mengubah jawaban GET /api/auth/me
 * berikutnya seperti API sungguhan.
 */
function pasang(
  awal: Record<string, unknown>,
  tambahan: Record<string, unknown> = {},
) {
  let akun = awal;
  api = pasangFetchPalsu({
    "GET /api/auth/me": () => ({ user: akun }),
    "POST /api/auth/mfa/setup": SETUP,
    "POST /api/auth/mfa/verify": () => {
      akun = {
        ...akun,
        mfa_aktif: true,
        mfa_setup_diwajibkan: false,
        kode_cadangan_tersisa: 10,
      };
      return { kode_cadangan: KODE };
    },
    "POST /api/auth/mfa/backup-codes": () => {
      akun = { ...akun, kode_cadangan_tersisa: 10 };
      return { kode_cadangan: KODE_BARU };
    },
    "POST /api/auth/mfa/disable": () => {
      akun = { ...akun, mfa_aktif: false, kode_cadangan_tersisa: undefined };
      return { success: true };
    },
    ...tambahan,
  });
}

/**
 * Mengganti clipboard peramban. Dipanggil sesudah `renderHalaman`: `userEvent.setup()` memasang stub clipboard-nya
 * sendiri dan akan menimpa yang dipasang lebih dulu. `null` = API clipboard tidak tersedia (mis. konteks tidak aman).
 */
function pasangClipboard(mode: "normal" | "ditolak" | "tidak-ada" = "normal") {
  const tersalin: string[] = [];
  if (mode === "tidak-ada") {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
    return tersalin;
  }
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: {
      writeText: async (teks: string) => {
        if (mode === "ditolak") throw new Error("ditolak");
        tersalin.push(teks);
      },
    },
  });
  return tersalin;
}

const tombol = (nama: string) => screen.getByRole("button", { name: nama });
const kolomKode = () =>
  screen.getByLabelText("Kode 6 digit") as HTMLInputElement;

/** Dari tahap awal sampai tahap kode cadangan (kode benar diterima server). */
async function aktifkan(user: ReturnType<typeof renderHalaman>["user"]) {
  await user.click(await screen.findByRole("button", { name: "Mulai setup" }));
  await user.type(await screen.findByLabelText("Kode 6 digit"), "123456");
  await user.click(tombol("Aktifkan MFA"));
  await screen.findByRole("list", { name: "Kode cadangan" });
}

describe("PanelMfa: tahap awal (belum aktif)", () => {
  it("selagi memuat: kerangka berstatus 'Memuat', lalu tahap awal", async () => {
    let selesai: (v: unknown) => void = () => {};
    pasang(AKUN_BELUM, {
      "GET /api/auth/me": () =>
        new Promise((r) => {
          selesai = r;
        }),
    });
    renderHalaman(<PanelMfa mode={MODE_TAB} />);

    assert.ok(screen.getByRole("status", { name: "Memuat" }));
    assert.equal(screen.queryByRole("button", { name: "Mulai setup" }), null);

    selesai({ user: AKUN_BELUM });
    assert.ok(await screen.findByRole("button", { name: "Mulai setup" }));
    assert.equal(screen.queryByRole("status", { name: "Memuat" }), null);
  });

  it("belum aktif: banner imbauan keamanan, petunjuk autentikator, dan tombol 'Mulai setup'", async () => {
    pasang(AKUN_BELUM);
    renderHalaman(<PanelMfa mode={MODE_TAB} />);

    assert.ok(await screen.findByRole("button", { name: "Mulai setup" }));
    assert.ok(
      screen.getByText("Akun Anda belum dilindungi verifikasi dua langkah."),
    );
    assert.ok(
      screen.getByText(
        "Anda memerlukan aplikasi autentikator (mis. Google Authenticator, Authy, atau 1Password) di ponsel.",
      ),
    );
    assert.equal(
      screen.queryByText(/Peran Anda wajib memakai verifikasi dua langkah/),
      null,
    );
    assert.equal(screen.queryByText("Verifikasi dua langkah aktif"), null);
  });

  it("peran wajib MFA yang belum setup: banner kewajiban, bukan imbauan umum", async () => {
    pasang({ ...AKUN_BELUM, mfa_wajib: true, mfa_setup_diwajibkan: true });
    renderHalaman(<PanelMfa mode={MODE_HALAMAN} />);

    assert.ok(
      await screen.findByText(
        "Peran Anda wajib memakai verifikasi dua langkah. Selesaikan setup ini untuk melanjutkan.",
      ),
    );
    assert.equal(
      screen.queryByText("Akun Anda belum dilindungi verifikasi dua langkah."),
      null,
    );
  });

  it("gagal memuat akun: pesan galat dengan 'Coba lagi' yang memuat ulang; setup tetap bisa dimulai", async () => {
    let gagal = true;
    pasang(AKUN_BELUM, {
      "GET /api/auth/me": () =>
        gagal ? galatApi(500, "Kesalahan server") : { user: AKUN_AKTIF },
    });
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Tidak dapat terhubung ke server/);
    assert.ok(tombol("Mulai setup"), "setup tidak diblokir oleh galat muat");
    assert.equal(
      screen.queryByText("Akun Anda belum dilindungi verifikasi dua langkah."),
      null,
      "status tidak diketahui: tanpa imbauan",
    );

    gagal = false;
    await user.click(tombol("Coba lagi"));
    assert.ok(await screen.findByText("Verifikasi dua langkah aktif"));
  });
});

describe("PanelMfa: setup sampai kode cadangan (mode tab)", () => {
  it("Mulai setup menampilkan QR (SVG sebagai gambar), kunci manual, dan isian kode 6 digit", async () => {
    pasang(AKUN_BELUM);
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);

    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );

    const qr = (await screen.findByRole("img", {
      name: "Kode QR untuk aplikasi autentikator",
    })) as HTMLImageElement;
    assert.equal(
      qr.getAttribute("src"),
      `data:image/svg+xml;utf8,${encodeURIComponent(SETUP.qr_svg)}`,
      "SVG ditanam sebagai gambar (tajam di semua ukuran), bukan HTML mentah",
    );
    assert.ok(screen.getByText("Pindai kode QR dengan aplikasi autentikator."));
    assert.ok(
      screen.getByText("Masukkan kode 6 digit yang tampil di aplikasi."),
    );
    assert.ok(screen.getByText("Tidak bisa memindai? Masukkan kunci manual"));
    assert.ok(screen.getByText(SETUP.secret), "kunci manual ada di DOM");
    assert.equal(kolomKode().getAttribute("autocomplete"), "one-time-code");
    assert.equal(kolomKode().getAttribute("inputmode"), "numeric");
    assert.equal(api.ke("POST", "/api/auth/mfa/setup").length, 1);
    assert.equal(
      screen.queryByRole("button", { name: "Mulai setup" }),
      null,
      "tahap awal sudah diganti tahap kode",
    );
  });

  it("'Salin kunci manual' menyalin kunci; clipboard yang ditolak menampilkan toast galat", async () => {
    pasang(AKUN_BELUM);
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    const tersalin = pasangClipboard();
    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );

    await user.click(
      await screen.findByRole("button", { name: "Salin kunci manual" }),
    );
    assert.deepEqual(tersalin, [SETUP.secret]);
    assert.ok(await screen.findByText("Kunci disalin"));

    pasangClipboard("ditolak");
    await user.click(tombol("Salin kunci manual"));
    assert.ok(await screen.findByText("Gagal menyalin"));
  });

  it("kode 6 digit diaktifkan: spasi dirapikan, toast 'MFA aktif', tampil 10 kode cadangan, tetap di tab", async () => {
    pasang(AKUN_BELUM);
    const { user, router, qc } = renderHalaman(<PanelMfa mode={MODE_TAB} />, {
      url: "/profil?tab=mfa",
    });
    qc.setQueryData(qk.sesi.aktif(), []);
    qc.setQueryData(qk.profil.saya(), { marker: true });
    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );

    await user.type(await screen.findByLabelText("Kode 6 digit"), "123 456");
    await user.click(tombol("Aktifkan MFA"));

    const daftar = await screen.findByRole("list", { name: "Kode cadangan" });
    assert.deepEqual(
      within(daftar)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
      KODE,
    );
    assert.deepEqual(api.ke("POST", "/api/auth/mfa/verify")[0].body, {
      code: "123456",
    });
    assert.ok(await screen.findByText("MFA aktif"));
    assert.ok(screen.getByText("Simpan kode cadangan Anda sekarang."));
    assert.ok(
      screen.getByText(/tidak dapat ditampilkan lagi/),
      "peringatan sekali tampil",
    );
    assert.deepEqual(router.replace, []);
    assert.deepEqual(router.push, []);
    assert.equal(
      qc.getQueryState(qk.sesi.aktif())?.isInvalidated,
      true,
      "aktivasi mencabut sesi lain: daftar sesi dimuat ulang",
    );
    assert.equal(
      qc.getQueryState(qk.profil.saya())?.isInvalidated,
      true,
      "data profil dimuat ulang: status MFA berubah",
    );
    assert.equal(window.location.search, "?tab=mfa");
  });

  it("kode kosong ditolak di klien tanpa memanggil server", async () => {
    pasang(AKUN_BELUM);
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );

    await user.click(
      await screen.findByRole("button", { name: "Aktifkan MFA" }),
    );

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /Kode wajib diisi\./,
    );
    assert.equal(api.ke("POST", "/api/auth/mfa/verify").length, 0);
  });

  it("kode ditolak server (400): pesan tampil, tetap di tahap kode, isian dipertahankan", async () => {
    pasang(AKUN_BELUM, {
      "POST /api/auth/mfa/verify": galatApi(
        400,
        "Kode verifikasi tidak valid. Periksa kode atau tunggu kode berikutnya.",
        "MFA_CODE_INVALID",
      ),
    });
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );

    await user.type(await screen.findByLabelText("Kode 6 digit"), "000000");
    await user.click(tombol("Aktifkan MFA"));

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /Kode verifikasi tidak valid/,
    );
    assert.ok(await screen.findByText("Kode tidak diterima"), "toast gagal");
    assert.equal(kolomKode().value, "000000");
    assert.equal(screen.queryByRole("list", { name: "Kode cadangan" }), null);
  });

  it("gagal memulai setup (server): pesan tampil dan tombol 'Mulai setup' tetap ada", async () => {
    pasang(AKUN_BELUM, {
      "POST /api/auth/mfa/setup": galatApi(409, "MFA sudah aktif"),
    });
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);

    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /MFA sudah aktif/,
    );
    assert.ok(tombol("Mulai setup"));
    assert.equal((tombol("Mulai setup") as HTMLButtonElement).disabled, false);
  });
});

describe("PanelMfa: kode cadangan sekali tampil", () => {
  it("'Selesai' dinonaktifkan sampai kode dinyatakan tersimpan, lalu kembali ke status aktif di tab", async () => {
    pasang(AKUN_BELUM);
    const { user, router } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    await aktifkan(user);

    const selesai = tombol("Selesai") as HTMLButtonElement;
    assert.equal(selesai.disabled, true);
    assert.equal(screen.queryByRole("button", { name: "Lanjutkan" }), null);

    await user.click(
      screen.getByRole("checkbox", {
        name: "Saya sudah menyimpan kode cadangan di tempat aman.",
      }),
    );
    assert.equal(selesai.disabled, false);
    await user.click(selesai);

    assert.ok(await screen.findByText("Verifikasi dua langkah aktif"));
    assert.equal(
      screen.getByText(/kode\s+cadangan tersisa/).textContent,
      "10 kode cadangan tersisa",
    );
    assert.equal(
      screen.queryByRole("list", { name: "Kode cadangan" }),
      null,
      "kode dibuang dari layar sesudah selesai",
    );
    assert.deepEqual(router.replace, []);
    assert.equal(router.refresh, 0, "klaim sesi tidak berubah pada tab");
  });

  it("'Salin' menyalin semua kode (satu per baris) dan menampilkan toast", async () => {
    pasang(AKUN_BELUM);
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    const tersalin = pasangClipboard();
    await aktifkan(user);

    assert.equal(
      tombol("Salin").getAttribute("title"),
      "Salin semua kode cadangan",
      "petunjuk ruang lingkup aksi",
    );
    await user.click(tombol("Salin"));

    assert.deepEqual(tersalin, [KODE.join("\n")]);
    assert.ok(await screen.findByText("Kode cadangan disalin"));
  });

  it("clipboard tidak tersedia: toast galat, kode tetap bisa disalin manual", async () => {
    pasang(AKUN_BELUM);
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    pasangClipboard("tidak-ada");
    await aktifkan(user);

    await user.click(tombol("Salin"));

    assert.ok(await screen.findByText("Gagal menyalin"));
    assert.ok(screen.getByRole("list", { name: "Kode cadangan" }));
  });

  it("'Unduh' menyimpan berkas .txt berisi akun dan kesepuluh kode", async () => {
    pasang(AKUN_BELUM);
    const unduhan = tangkapUnduhan();
    const blobs: Blob[] = [];
    const buat = URL.createObjectURL;
    URL.createObjectURL = (b: Blob | MediaSource) => {
      blobs.push(b as Blob);
      return buat(b);
    };
    try {
      const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
      await aktifkan(user);

      assert.equal(tombol("Unduh").getAttribute("title"), "Unduh berkas (.txt)");
      await user.click(tombol("Unduh"));

      assert.equal(unduhan.daftar.length, 1);
      assert.equal(
        unduhan.daftar[0].download,
        "kode-cadangan-garda-inflasi.txt",
      );
      const isi = await blobs[0].text();
      assert.match(isi, /admin@garda\.test/);
      for (const k of KODE) assert.ok(isi.includes(k), k);
      assert.equal(blobs[0].type, "text/plain");
    } finally {
      unduhan.pulihkan();
    }
  });
});

describe("PanelMfa: status aktif dan pengelolaan", () => {
  it("aktif: status dengan sisa kode cadangan dan dua aksi; belum ada isian kata sandi", async () => {
    pasang(AKUN_AKTIF);
    renderHalaman(<PanelMfa mode={MODE_TAB} />);

    assert.ok(await screen.findByText("Verifikasi dua langkah aktif"));
    assert.equal(
      screen.getByText(/kode\s+cadangan tersisa/).textContent,
      "8 kode cadangan tersisa",
    );
    assert.ok(tombol("Buat kode cadangan baru"));
    assert.ok(tombol("Nonaktifkan MFA"));
    assert.ok(screen.getByText("Kode cadangan lama langsung tidak berlaku."));
    assert.equal(
      screen.queryByLabelText("Kata sandi (untuk mengonfirmasi)"),
      null,
    );
    assert.equal(
      screen.queryByRole("link", { name: "Kembali ke ruang kerja" }),
      null,
      "tab sudah berada di ruang kerja",
    );
  });

  it("buat kode cadangan baru: dialog meminta kata sandi, lalu 10 kode baru tampil dan dialog tertutup", async () => {
    pasang(AKUN_AKTIF);
    const { user, qc, router } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    qc.setQueryData(qk.profil.saya(), { marker: true });
    qc.setQueryData(qk.sesi.aktif(), []);
    await user.click(
      await screen.findByRole("button", { name: "Buat kode cadangan baru" }),
    );

    const dialog = await screen.findByRole("dialog");
    assert.ok(within(dialog).getByText("Buat ulang kode cadangan?"));
    const konfirmasi = within(dialog).getByRole("button", {
      name: "Ya, buat kode baru",
    }) as HTMLButtonElement;
    assert.equal(konfirmasi.disabled, true, "kata sandi wajib diisi dulu");

    await user.type(
      within(dialog).getByLabelText("Kata sandi (untuk mengonfirmasi)"),
      "Rahasia#2026",
    );
    await user.click(konfirmasi);

    const daftar = await screen.findByRole("list", { name: "Kode cadangan" });
    assert.deepEqual(
      within(daftar)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
      KODE_BARU,
    );
    assert.deepEqual(api.ke("POST", "/api/auth/mfa/backup-codes")[0].body, {
      password: "Rahasia#2026",
    });
    await waitFor(() => assert.equal(screen.queryByRole("dialog"), null));
    assert.ok(await screen.findByText("Kode cadangan baru dibuat"));
    assert.equal(
      qc.getQueryState(qk.profil.saya())?.isInvalidated,
      true,
      "sisa kode cadangan di data profil dimuat ulang",
    );
    assert.notEqual(
      qc.getQueryState(qk.sesi.aktif())?.isInvalidated,
      true,
      "kode baru tidak mencabut sesi: daftar sesi tidak perlu dimuat ulang",
    );
    assert.equal(router.refresh, 0);

    await user.click(
      screen.getByRole("checkbox", {
        name: "Saya sudah menyimpan kode cadangan di tempat aman.",
      }),
    );
    await user.click(tombol("Selesai"));
    assert.ok(await screen.findByText("Verifikasi dua langkah aktif"));
    assert.equal(
      screen.getByText(/kode\s+cadangan tersisa/).textContent,
      "10 kode cadangan tersisa",
    );
  });

  it("kata sandi salah saat membuat kode baru: pesan di dialog, dialog tetap terbuka, kode lama tidak berubah", async () => {
    pasang(AKUN_AKTIF, {
      "POST /api/auth/mfa/backup-codes": galatApi(401, "Password salah"),
    });
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    await user.click(
      await screen.findByRole("button", { name: "Buat kode cadangan baru" }),
    );

    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText("Kata sandi (untuk mengonfirmasi)"),
      "Salah#2026",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Ya, buat kode baru" }),
    );

    assert.match(
      (await within(dialog).findByRole("alert")).textContent ?? "",
      /Password salah/,
    );
    assert.ok(screen.getByRole("dialog"));
    assert.equal(screen.queryByRole("list", { name: "Kode cadangan" }), null);
  });

  it("Batal menutup dialog tanpa mengirim apa pun dan mengosongkan kata sandi", async () => {
    pasang(AKUN_AKTIF);
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    await user.click(
      await screen.findByRole("button", { name: "Nonaktifkan MFA" }),
    );
    let dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText("Kata sandi (untuk mengonfirmasi)"),
      "Rahasia#2026",
    );

    await user.click(within(dialog).getByRole("button", { name: "Batal" }));

    await waitFor(() => assert.equal(screen.queryByRole("dialog"), null));
    assert.equal(api.ke("POST", "/api/auth/mfa/disable").length, 0);
    await user.click(tombol("Nonaktifkan MFA"));
    dialog = await screen.findByRole("dialog");
    assert.equal(
      (
        within(dialog).getByLabelText(
          "Kata sandi (untuk mengonfirmasi)",
        ) as HTMLInputElement
      ).value,
      "",
    );
  });

  it("nonaktifkan: dialog kata sandi, POST disable, toast, lalu kembali ke tahap awal", async () => {
    pasang(AKUN_AKTIF);
    const { user, qc } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    qc.setQueryData(qk.profil.saya(), { marker: true });
    await user.click(
      await screen.findByRole("button", { name: "Nonaktifkan MFA" }),
    );

    const dialog = await screen.findByRole("dialog");
    assert.ok(within(dialog).getByText("Nonaktifkan verifikasi dua langkah?"));
    await user.type(
      within(dialog).getByLabelText("Kata sandi (untuk mengonfirmasi)"),
      "Rahasia#2026",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Ya, nonaktifkan" }),
    );

    assert.ok(await screen.findByText("MFA dinonaktifkan"));
    assert.deepEqual(api.ke("POST", "/api/auth/mfa/disable")[0].body, {
      password: "Rahasia#2026",
    });
    assert.ok(await screen.findByRole("button", { name: "Mulai setup" }));
    assert.equal(screen.queryByRole("dialog"), null);
    assert.equal(screen.queryByText("Verifikasi dua langkah aktif"), null);
    assert.equal(
      qc.getQueryState(qk.profil.saya())?.isInvalidated,
      true,
      "data profil dimuat ulang: status MFA berubah",
    );
    assert.ok(
      api.ke("GET", "/api/auth/me").length >= 2,
      "akun dimuat ulang dari server sesudah dinonaktifkan",
    );
  });

  it("kata sandi salah saat menonaktifkan: MFA tetap aktif dan pesan tampil di dialog", async () => {
    pasang(AKUN_AKTIF, {
      "POST /api/auth/mfa/disable": galatApi(401, "Password salah"),
    });
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    await user.click(
      await screen.findByRole("button", { name: "Nonaktifkan MFA" }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText("Kata sandi (untuk mengonfirmasi)"),
      "Salah#2026",
    );

    await user.click(
      within(dialog).getByRole("button", { name: "Ya, nonaktifkan" }),
    );

    assert.match(
      (await within(dialog).findByRole("alert")).textContent ?? "",
      /Password salah/,
    );
    assert.ok(screen.getByRole("dialog"));
  });

  it("MFA wajib untuk peran: tombol Nonaktifkan tidak ada dan alasannya dijelaskan; kode cadangan tetap bisa dibuat ulang", async () => {
    pasang({ ...AKUN_AKTIF, mfa_wajib: true });
    renderHalaman(<PanelMfa mode={MODE_TAB} />);

    assert.ok(await screen.findByText("Verifikasi dua langkah aktif"));
    assert.equal(
      screen.queryByRole("button", { name: "Nonaktifkan MFA" }),
      null,
    );
    assert.ok(
      screen.getByText(
        "Verifikasi dua langkah wajib untuk akun Anda dan tidak dapat dinonaktifkan.",
      ),
    );
    assert.ok(tombol("Buat kode cadangan baru"));
  });
});

describe("PanelMfa: mode halaman (alur wajib setup MFA)", () => {
  it("peran wajib: sesudah menyimpan kode, 'Lanjutkan' berpindah ke tujuan semula dan menyegarkan sesi", async () => {
    pasang({ ...AKUN_BELUM, mfa_wajib: true, mfa_setup_diwajibkan: true });
    const { user, router } = renderHalaman(<PanelMfa mode={MODE_HALAMAN} />, {
      url: "/mfa?lanjut=%2Foperasional",
    });
    await aktifkan(user);

    // Server tidak lagi mewajibkan setup, tetapi tombol tetap melanjutkan ke tujuan semula.
    const lanjutkan = tombol("Lanjutkan") as HTMLButtonElement;
    assert.equal(screen.queryByRole("button", { name: "Selesai" }), null);
    assert.equal(lanjutkan.disabled, true);
    assert.deepEqual(router.replace, [], "belum pindah sebelum kode disimpan");

    await user.click(
      screen.getByRole("checkbox", {
        name: "Saya sudah menyimpan kode cadangan di tempat aman.",
      }),
    );
    await user.click(lanjutkan);

    assert.deepEqual(router.replace, ["/operasional"]);
    assert.equal(
      router.refresh,
      1,
      "klaim wajib setup MFA hilang: root layout dirender ulang",
    );
    assert.deepEqual(router.push, []);
  });

  it("tujuan yang bukan path internal disaring menjadi beranda", async () => {
    pasang({ ...AKUN_BELUM, mfa_setup_diwajibkan: true });
    const { user, router } = renderHalaman(
      <PanelMfa mode={{ jenis: "halaman", lanjut: "https://evil.example" }} />,
    );
    await aktifkan(user);
    await user.click(
      screen.getByRole("checkbox", {
        name: "Saya sudah menyimpan kode cadangan di tempat aman.",
      }),
    );

    await user.click(tombol("Lanjutkan"));

    assert.deepEqual(router.replace, ["/"]);
  });

  it("setup sukarela di halaman auth: 'Selesai' tetap di tempat, tautan kembali ke ruang kerja tersedia", async () => {
    pasang(AKUN_BELUM);
    const { user, router } = renderHalaman(<PanelMfa mode={MODE_HALAMAN} />);
    await aktifkan(user);
    assert.ok(tombol("Selesai"));
    assert.equal(screen.queryByRole("button", { name: "Lanjutkan" }), null);

    await user.click(
      screen.getByRole("checkbox", {
        name: "Saya sudah menyimpan kode cadangan di tempat aman.",
      }),
    );
    await user.click(tombol("Selesai"));

    assert.ok(await screen.findByText("Verifikasi dua langkah aktif"));
    assert.deepEqual(router.replace, []);
    assert.equal(router.refresh, 0);
    assert.equal(
      screen
        .getByRole("link", { name: "Kembali ke ruang kerja" })
        .getAttribute("href"),
      "/",
    );
  });
});

describe("PanelMfa: tata letak", () => {
  it("kelas pembungkus diteruskan (tab memakai tinggi minimum agar pergantian tahap tidak menggeser halaman)", async () => {
    pasang(AKUN_BELUM);
    const { container } = renderHalaman(
      <PanelMfa mode={MODE_TAB} className="min-h-[28rem] max-w-lg" />,
    );
    await screen.findByRole("button", { name: "Mulai setup" });
    const akar = container.firstElementChild as HTMLElement;
    assert.ok(akar.className.includes("min-h-[28rem]"));
  });

  it("sepanjang alur hanya satu tahap yang tampil pada satu waktu (tanpa tumpang tindih)", async () => {
    pasang(AKUN_BELUM);
    const { user } = renderHalaman(<PanelMfa mode={MODE_TAB} />);
    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );
    await screen.findByLabelText("Kode 6 digit");
    assert.equal(screen.queryByRole("button", { name: "Mulai setup" }), null);

    await user.type(kolomKode(), "123456");
    await user.click(tombol("Aktifkan MFA"));
    await screen.findByRole("list", { name: "Kode cadangan" });
    assert.equal(screen.queryByLabelText("Kode 6 digit"), null);
    assert.equal(screen.queryByRole("button", { name: "Aktifkan MFA" }), null);
  });
});
