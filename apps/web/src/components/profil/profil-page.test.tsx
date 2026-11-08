/**
 * Tes kerangka /profil: pemilihan tab dari ?tab=, penggantian tab, dan pola tablist. Isi tab keamanan
 * (ganti sandi, MFA, sesi) dikerjakan dan diuji terpisah; di sini hanya kerangkanya, ditambah isi Edit Profil.
 */
import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import {
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { ProfilPage } from "./profil-page";

const PROFIL = {
  id: "admin-1",
  nama: "Admin Satu",
  email: "admin@garda.test",
  peran: "ADMIN_SISTEM",
  lapis: "SISTEM",
  lembaga: "BPPSDMP",
  mfa_aktif: false,
  status: "AKTIF",
  dibuat_pada: "2026-10-01T08:00:00Z",
  wajib_ganti_password: false,
  mfa_setup_diwajibkan: false,
  mfa_wajib: false,
};

let fetchPalsu: FetchPalsu;
beforeEach(() => {
  fetchPalsu = pasangFetchPalsu({
    "GET /api/auth/me": () => ({ user: PROFIL }),
    "GET /api/auth/sessions": () => [],
  });
});
afterEach(() => {
  toast.dismiss();
  fetchPalsu.pulihkan();
});

const NAMA_TAB = {
  profil: "Edit Profil",
  "ganti-password": "Ganti Kata Sandi",
  mfa: "Verifikasi 2 Langkah",
  keamanan: "Perangkat & Sesi Aktif",
} as const;
const URUTAN = Object.values(NAMA_TAB);

const tab = (nama: string) => screen.getByRole("tab", { name: nama });
const terpilih = () =>
  screen
    .getAllByRole("tab")
    .filter((t) => t.getAttribute("aria-selected") === "true")
    .map((t) => t.textContent?.trim());

function render(url = "/profil") {
  return renderHalaman(<ProfilPage />, { url, sesi: penggunaSesi() });
}

describe("ProfilPage: kerangka", () => {
  it("judul halaman, tablist 'Bagian profil' dengan empat tab berurutan, dan satu tabpanel", () => {
    render();

    assert.ok(screen.getByRole("heading", { level: 1, name: "Profil saya" }));
    const daftar = screen.getByRole("tablist", { name: "Bagian profil" });
    assert.deepEqual(
      [...daftar.querySelectorAll('[role="tab"]')].map((t) =>
        t.textContent?.trim(),
      ),
      URUTAN,
    );
    assert.equal(screen.getAllByRole("tabpanel").length, 1);
  });

  it("tabpanel dilabeli tab yang terpilih dan tab menunjuk panelnya (aria-controls)", () => {
    render("/profil?tab=mfa");

    const panel = screen.getByRole("tabpanel");
    assert.equal(
      panel.getAttribute("aria-labelledby"),
      tab("Verifikasi 2 Langkah").id,
    );
    assert.equal(
      tab("Verifikasi 2 Langkah").getAttribute("aria-controls"),
      panel.id,
    );
    assert.equal(
      screen.getByRole("tabpanel", { name: "Verifikasi 2 Langkah" }),
      panel,
    );
  });
});

describe("ProfilPage: tata letak", () => {
  it("nav tab boleh menyusut (min-w-0) dan pil bergulir di dalamnya: tanpa itu item grid melebarkan halaman di mobile", () => {
    render();

    const nav = screen.getByRole("navigation", { name: "Menu profil" });
    assert.ok(nav.classList.contains("min-w-0"));
    const daftar = screen.getByRole("tablist", { name: "Bagian profil" });
    assert.ok(daftar.classList.contains("overflow-x-auto"));
  });
});

describe("ProfilPage: tab dari ?tab=", () => {
  it("tanpa ?tab= tab Edit Profil terpilih dan isinya form data pribadi", async () => {
    render("/profil");

    assert.deepEqual(terpilih(), ["Edit Profil"]);
    assert.ok(await screen.findByLabelText("Nama lengkap"));
  });

  it("setiap nilai ?tab= yang dikenal memilih tab yang sama", () => {
    for (const [id, nama] of Object.entries(NAMA_TAB)) {
      const { unmount } = render(`/profil?tab=${id}`);
      assert.deepEqual(terpilih(), [nama], id);
      unmount();
    }
  });

  it("nilai kosong, tidak dikenal, atau beda huruf jatuh ke Edit Profil", () => {
    for (const nilai of ["", "sesi", "MFA", "keamanan/", "%20mfa", "profil"]) {
      const { unmount } = render(`/profil?tab=${nilai}`);
      assert.deepEqual(terpilih(), ["Edit Profil"], nilai);
      unmount();
    }
  });

  it("hanya tab terpilih yang masuk urutan fokus (roving tabindex)", () => {
    render("/profil?tab=mfa");

    for (const [id, nama] of Object.entries(NAMA_TAB)) {
      assert.equal(tab(nama).tabIndex, id === "mfa" ? 0 : -1, nama);
    }
  });

  it("URL yang berubah dari luar (tombol kembali peramban, tautan menu akun) mengganti tab", async () => {
    render("/profil");
    assert.deepEqual(terpilih(), ["Edit Profil"]);

    window.history.pushState(null, "", "/profil?tab=keamanan");

    await waitFor(() =>
      assert.deepEqual(terpilih(), ["Perangkat & Sesi Aktif"]),
    );
  });
});

describe("ProfilPage: mengganti tab", () => {
  it("klik tab mengganti URL lewat replaceState (riwayat tidak menumpuk) dan aria-selected", async () => {
    const { user } = render("/profil");
    const panjangRiwayat = window.history.length;

    await user.click(tab("Verifikasi 2 Langkah"));
    assert.equal(
      window.location.pathname + window.location.search,
      "/profil?tab=mfa",
    );
    await waitFor(() => assert.deepEqual(terpilih(), ["Verifikasi 2 Langkah"]));

    await user.click(tab("Perangkat & Sesi Aktif"));
    assert.equal(window.location.search, "?tab=keamanan");
    await waitFor(() =>
      assert.deepEqual(terpilih(), ["Perangkat & Sesi Aktif"]),
    );

    assert.equal(
      window.history.length,
      panjangRiwayat,
      "tidak menambah entri riwayat",
    );
  });

  it("kembali ke Edit Profil menghapus ?tab= dari URL, dan isi tab berganti", async () => {
    const { user } = render("/profil?tab=mfa");

    await user.click(tab("Edit Profil"));

    assert.equal(window.location.pathname + window.location.search, "/profil");
    await waitFor(() => assert.deepEqual(terpilih(), ["Edit Profil"]));
    assert.ok(await screen.findByLabelText("Nama lengkap"));
    assert.equal(screen.getAllByRole("tabpanel").length, 1);
  });

  it("klik tab yang sedang terpilih tidak mengubah URL", async () => {
    const { user } = render("/profil?tab=mfa");
    const riwayat = window.history.length;

    await user.click(tab("Verifikasi 2 Langkah"));

    assert.equal(window.location.search, "?tab=mfa");
    assert.equal(window.history.length, riwayat);
  });
});

describe("ProfilPage: papan ketik", () => {
  it("panah bawah/kanan pindah ke tab berikutnya (berputar), panah atas/kiri ke sebelumnya, fokus ikut", async () => {
    const { user } = render("/profil");
    tab("Edit Profil").focus();

    await user.keyboard("{ArrowDown}");
    await waitFor(() => assert.deepEqual(terpilih(), ["Ganti Kata Sandi"]));
    assert.equal(document.activeElement, tab("Ganti Kata Sandi"));
    assert.equal(window.location.search, "?tab=ganti-password");

    await user.keyboard("{ArrowRight}");
    await waitFor(() => assert.deepEqual(terpilih(), ["Verifikasi 2 Langkah"]));

    await user.keyboard("{ArrowUp}");
    await waitFor(() => assert.deepEqual(terpilih(), ["Ganti Kata Sandi"]));

    await user.keyboard("{ArrowLeft}");
    await waitFor(() => assert.deepEqual(terpilih(), ["Edit Profil"]));

    // Berputar dari tab pertama ke terakhir.
    await user.keyboard("{ArrowUp}");
    await waitFor(() =>
      assert.deepEqual(terpilih(), ["Perangkat & Sesi Aktif"]),
    );
    assert.equal(document.activeElement, tab("Perangkat & Sesi Aktif"));

    // Dan dari yang terakhir kembali ke yang pertama.
    await user.keyboard("{ArrowDown}");
    await waitFor(() => assert.deepEqual(terpilih(), ["Edit Profil"]));
    assert.equal(window.location.search, "");
  });

  it("Home ke tab pertama dan End ke tab terakhir", async () => {
    const { user } = render("/profil?tab=mfa");
    tab("Verifikasi 2 Langkah").focus();

    await user.keyboard("{End}");
    await waitFor(() =>
      assert.deepEqual(terpilih(), ["Perangkat & Sesi Aktif"]),
    );
    assert.equal(document.activeElement, tab("Perangkat & Sesi Aktif"));

    await user.keyboard("{Home}");
    await waitFor(() => assert.deepEqual(terpilih(), ["Edit Profil"]));
    assert.equal(document.activeElement, tab("Edit Profil"));
  });

  it("tombol lain tidak mengganti tab", async () => {
    const { user } = render("/profil?tab=mfa");
    tab("Verifikasi 2 Langkah").focus();

    await user.keyboard("a{Enter}{Escape}");

    assert.deepEqual(terpilih(), ["Verifikasi 2 Langkah"]);
    assert.equal(window.location.search, "?tab=mfa");
  });
});
