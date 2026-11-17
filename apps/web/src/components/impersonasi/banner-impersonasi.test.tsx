import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import {
  galatApi,
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { sesiImpersonate, tangkapNavigasi, tuntaskanTimer } from "@/test/impersonasi";
import { BannerImpersonasi } from "./banner-impersonasi";
import { TombolKembaliPeran } from "./tombol-kembali";

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

const STOP = "POST /api/impersonation/stop";
const HASIL_STOP = {
  user: { id: "u-asal", nama: "Super Admin", peran: "ADMIN_SISTEM" },
  redirectTo: "/pengguna",
};

describe("BannerImpersonasi", () => {
  it("tanpa impersonasi tidak merender apa pun", () => {
    api = pasangFetchPalsu();
    renderHalaman(<BannerImpersonasi />, { sesi: penggunaSesi() });
    assert.equal(screen.queryByRole("status"), null);
    assert.equal(screen.queryByTestId("banner-impersonasi"), null);
  });

  it("menampilkan asal -> target dengan nama dan peran, sebagai status, plus tombol kembali", () => {
    api = pasangFetchPalsu();
    renderHalaman(<BannerImpersonasi />, { sesi: sesiImpersonate("OPS_KOORDINATOR") });

    const banner = screen.getByRole("status");
    assert.equal(banner, screen.getByTestId("banner-impersonasi"));
    assert.match(banner.textContent ?? "", /Sedang Impersonate:/);
    assert.match(banner.textContent ?? "", /Super Admin \(Administrator Sistem\)/);
    assert.match(banner.textContent ?? "", /Zulkifli, S\.P\. \(Koordinator Lapangan\)/);
    assert.ok(screen.getByRole("button", { name: "Kembali ke Peran Asal" }));
  });

  it("tertempel di puncak viewport (sticky top-0) dengan kontras tinggi dan tinggi tetap", () => {
    api = pasangFetchPalsu();
    renderHalaman(<BannerImpersonasi />, { sesi: sesiImpersonate() });
    const banner = screen.getByTestId("banner-impersonasi");
    assert.match(banner.className, /\bsticky\b/);
    assert.match(banner.className, /\btop-0\b/);
    assert.match(banner.className, /bg-waspada/);
    assert.equal(banner.style.height, "2.25rem");
  });

  it("klik kembali: memanggil stop sekali, lalu memuat penuh ke tujuan dari BFF (tanpa token di mana pun)", async () => {
    api = pasangFetchPalsu({ [STOP]: HASIL_STOP });
    const { user } = renderHalaman(<BannerImpersonasi />, { sesi: sesiImpersonate() });
    const tombol = screen.getByRole("button", { name: "Kembali ke Peran Asal" });

    await user.click(tombol);
    // Klik ganda sambil berjalan tidak mengirim permintaan kedua.
    await user.click(tombol);

    await waitFor(() => assert.deepEqual(nav.tujuan, ["/pengguna"]));
    assert.equal(api.ke("POST", "/api/impersonation/stop").length, 1);
    assert.equal(
      api.ke("POST", "/api/impersonation/stop")[0].headers["x-csrf-token"] !== undefined,
      true,
      "mutasi menyertakan header CSRF",
    );
  });

  it("gagal: toast galat, tidak pindah halaman, tombol aktif lagi untuk dicoba ulang", async () => {
    api = pasangFetchPalsu({ [STOP]: galatApi(500, "Layanan sesi sedang gangguan") });
    const { user } = renderHalaman(<BannerImpersonasi />, { sesi: sesiImpersonate() });
    const tombol = screen.getByRole("button", { name: "Kembali ke Peran Asal" });

    await user.click(tombol);

    await screen.findByText("Gagal kembali ke peran asal");
    assert.ok(screen.getByText("Layanan sesi sedang gangguan"));
    assert.deepEqual(nav.tujuan, []);
    await waitFor(() => assert.equal((tombol as HTMLButtonElement).disabled, false));
  });
});

describe("TombolKembaliPeran (chip topbar)", () => {
  it("tanpa impersonasi tidak merender apa pun", () => {
    api = pasangFetchPalsu();
    renderHalaman(<TombolKembaliPeran />, { sesi: penggunaSesi() });
    assert.equal(screen.queryByTestId("chip-kembali-peran"), null);
  });

  it("menampilkan 'Kembali ke [Peran Asal]' dan memanggil stop dengan satu klik", async () => {
    api = pasangFetchPalsu({ [STOP]: HASIL_STOP });
    const { user } = renderHalaman(<TombolKembaliPeran />, { sesi: sesiImpersonate() });

    const chip = screen.getByRole("button", { name: "Kembali ke Administrator Sistem" });
    assert.match(chip.textContent ?? "", /Kembali ke Administrator Sistem/);
    await user.click(chip);

    await waitFor(() => assert.deepEqual(nav.tujuan, ["/pengguna"]));
    assert.equal(api.ke("POST", "/api/impersonation/stop").length, 1);
  });

  it("tooltip menjelaskan akun yang dituju dan bahwa aksi tetap tercatat atas akun asli", async () => {
    api = pasangFetchPalsu();
    const { user } = renderHalaman(<TombolKembaliPeran />, { sesi: sesiImpersonate() });
    await user.hover(screen.getByRole("button", { name: "Kembali ke Administrator Sistem" }));
    const tip = await screen.findByRole("tooltip");
    assert.match(tip.textContent ?? "", /kembali ke akun Super Admin \(Administrator Sistem\)/);
    assert.match(tip.textContent ?? "", /tercatat atas nama akun asli/);
  });
});
