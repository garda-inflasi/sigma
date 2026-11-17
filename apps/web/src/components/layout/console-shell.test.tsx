import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen } from "@testing-library/react";
import { PathnameContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import { sesiImpersonate, tuntaskanTimer } from "@/test/impersonasi";
import {
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { ConsoleShell } from "./console-shell";
import { RoleSwitcher } from "./role-switcher";
import { NavList } from "./sidebar";

let api: FetchPalsu;
beforeEach(() => {
  api = pasangFetchPalsu();
});
afterEach(() => api.pulihkan());
afterEach(tuntaskanTimer);

function renderShell(sesi: ReturnType<typeof penggunaSesi>) {
  return renderHalaman(
    <PathnameContext.Provider value="/profil">
      <ConsoleShell>
        <p>isi halaman</p>
      </ConsoleShell>
    </PathnameContext.Provider>,
    { sesi, url: "/profil" },
  );
}

describe("ConsoleShell: banner impersonasi", () => {
  it("sesi biasa: tanpa banner dan tanpa offset sticky; isi halaman dirender", () => {
    const { container } = renderShell(penggunaSesi({ role: "OPS_MAHASISWA" }));
    assert.equal(screen.queryByTestId("banner-impersonasi"), null);
    assert.ok(screen.getByText("isi halaman"));
    const akar = container.firstElementChild as HTMLElement;
    assert.equal(akar.style.getPropertyValue("--bar-atas"), "");
  });

  it("selama impersonasi: banner jadi anak PERTAMA wadah (paling atas, di atas sidebar dan topbar) dan membawa offset --bar-atas", () => {
    const { container } = renderShell(sesiImpersonate("OPS_MAHASISWA"));

    const akar = container.firstElementChild as HTMLElement;
    const banner = screen.getByTestId("banner-impersonasi");
    assert.equal(akar.firstElementChild, banner, "banner di puncak, mendahului sidebar dan topbar");
    assert.equal(akar.style.getPropertyValue("--bar-atas"), "2.25rem");
    // Sidebar dan topbar bergeser sebesar banner lewat variabel yang sama (tidak tertutup).
    const topbar = screen.getByRole("banner");
    assert.match(topbar.className, /top-\[var\(--bar-atas,0px\)\]/);
    const sidebar = container.querySelector("aside") as HTMLElement;
    assert.match(sidebar.className, /top-\[var\(--bar-atas,0px\)\]/);
    assert.match(sidebar.className, /h-\[calc\(100dvh-var\(--bar-atas,0px\)\)\]/);
    // Chip kembali ada di topbar dan banner di atasnya: dua jalan kembali.
    assert.ok(topbar.querySelector('[data-testid="chip-kembali-peran"]'));
    assert.ok(screen.getByText("isi halaman"));
  });
});

describe("RoleSwitcher (pengalih peran dev)", () => {
  it("tampil di lingkungan non-produksi untuk sesi biasa, disembunyikan selama impersonasi", () => {
    const biasa = renderHalaman(<RoleSwitcher />, { sesi: penggunaSesi({ role: "OPS_MAHASISWA" }) });
    assert.ok(screen.getByText(/Lihat sebagai/));
    biasa.unmount();

    renderHalaman(<RoleSwitcher />, { sesi: sesiImpersonate("OPS_MAHASISWA") });
    assert.equal(screen.queryByText(/Lihat sebagai/), null);
  });
});

describe("Navigasi: item Impersonasi", () => {
  function renderNav(sesi: ReturnType<typeof penggunaSesi>) {
    return renderHalaman(
      <PathnameContext.Provider value="/impersonasi">
        <NavList />
      </PathnameContext.Provider>,
      { sesi, url: "/impersonasi" },
    );
  }

  it("superadmin melihat 'Impersonasi' di grup Tata kelola dan menyala di /impersonasi", () => {
    renderNav(penggunaSesi({ role: "ADMIN_SISTEM" }));
    const tautan = screen.getByRole("link", { name: "Impersonasi" });
    assert.equal(tautan.getAttribute("href"), "/impersonasi");
    assert.equal(tautan.getAttribute("aria-current"), "page");
    assert.ok(screen.getByRole("link", { name: "Pengguna & peran" }).getAttribute("aria-current") === null);
  });

  it("peran lain tidak melihatnya; sesi impersonasi (izin sasaran) juga tidak", () => {
    const lain = renderNav(penggunaSesi({ role: "MANAJER_PROGRAM" }));
    assert.equal(screen.queryByRole("link", { name: "Impersonasi" }), null);
    lain.unmount();

    renderNav(sesiImpersonate("OPS_KOORDINATOR"));
    assert.equal(screen.queryByRole("link", { name: "Impersonasi" }), null);
  });
});
