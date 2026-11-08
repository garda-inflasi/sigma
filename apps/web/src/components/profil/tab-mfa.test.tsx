import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { configure, screen } from "@testing-library/react";
import { toast } from "sonner";
import {
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { TabMfa } from "./tab-mfa";

// Dialog Radix dan jaringan palsu butuh beberapa putaran render; beri kelonggaran saat mesin sibuk (CI).
configure({ asyncUtilTimeout: 3000 });

const URL_TAB = "/profil?tab=mfa";

const AKUN = {
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
const KODE = Array.from({ length: 10 }, (_, i) => `KODE-${i}`);

let api: FetchPalsu;
afterEach(async () => {
  // Toast Sonner hidup di store global dan lolos dari cleanup render; tanpa dibuang, timernya menggantung proses tes.
  toast.dismiss();
  // Sisa jadwal React dan animasi diberi waktu selesai selagi DOM masih ada; tanpa jeda, tes terakhir berkas bisa
  // memicu "window is not defined" saat `after` di render.tsx melepas DOM.
  await new Promise((r) => setTimeout(r, 30));
  api?.pulihkan();
});

describe("TabMfa", () => {
  it("kartu panel dengan judul tab (bukan heading halaman auth), tinggi minimum stabil, tanpa tautan 'Kembali'", async () => {
    api = pasangFetchPalsu({ "GET /api/auth/me": { user: AKUN } });
    const { container } = renderHalaman(<TabMfa />, { url: URL_TAB });

    const judul = screen.getByRole("heading", {
      name: "Verifikasi dua langkah",
    });
    assert.equal(judul.tagName, "H2");
    assert.equal(screen.queryByRole("heading", { level: 1 }), null);
    assert.ok(await screen.findByRole("button", { name: "Mulai setup" }));
    assert.equal(
      screen.queryByRole("link", { name: "Kembali ke ruang kerja" }),
      null,
    );
    const kartu = container.firstElementChild as HTMLElement;
    assert.ok(kartu.className.includes("glass"));
    assert.ok(kartu.className.includes("rounded-xl"));
    assert.ok(
      container.querySelector("[class*='min-h-[28rem]']"),
      "pembungkus tahap memakai tinggi minimum",
    );
  });

  it("aktivasi dari tab: kode cadangan tampil lalu status aktif, tetap di /profil?tab=mfa", async () => {
    let akun: Record<string, unknown> = AKUN;
    api = pasangFetchPalsu({
      "GET /api/auth/me": () => ({ user: akun }),
      "POST /api/auth/mfa/setup": {
        secret: "JBSWY3DPEHPK3PXP",
        provisioning_uri: "otpauth://totp/Garda?secret=JBSWY3DPEHPK3PXP",
        qr_svg: '<svg xmlns="http://www.w3.org/2000/svg"></svg>',
      },
      "POST /api/auth/mfa/verify": () => {
        akun = { ...akun, mfa_aktif: true, kode_cadangan_tersisa: 10 };
        return { kode_cadangan: KODE };
      },
    });
    const { user, router } = renderHalaman(<TabMfa />, { url: URL_TAB });

    await user.click(
      await screen.findByRole("button", { name: "Mulai setup" }),
    );
    await user.type(await screen.findByLabelText("Kode 6 digit"), "123456");
    await user.click(screen.getByRole("button", { name: "Aktifkan MFA" }));
    await screen.findByRole("list", { name: "Kode cadangan" });
    await user.click(
      screen.getByRole("checkbox", {
        name: "Saya sudah menyimpan kode cadangan di tempat aman.",
      }),
    );
    await user.click(screen.getByRole("button", { name: "Selesai" }));

    assert.ok(await screen.findByText("Verifikasi dua langkah aktif"));
    assert.deepEqual(router.replace, []);
    assert.deepEqual(router.push, []);
    assert.equal(window.location.pathname + window.location.search, URL_TAB);
  });

  it("MFA wajib: status aktif tanpa tombol Nonaktifkan, dengan penjelasan", async () => {
    api = pasangFetchPalsu({
      "GET /api/auth/me": {
        user: {
          ...AKUN,
          mfa_aktif: true,
          mfa_wajib: true,
          kode_cadangan_tersisa: 7,
        },
      },
    });
    renderHalaman(<TabMfa />, { url: URL_TAB });

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
  });
});
