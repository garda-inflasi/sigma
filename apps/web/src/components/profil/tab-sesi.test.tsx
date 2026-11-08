import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { configure, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import {
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { TabSesi } from "./tab-sesi";

// Dialog Radix dan jaringan palsu butuh beberapa putaran render; beri kelonggaran saat mesin sibuk (CI).
configure({ asyncUtilTimeout: 3000 });

const sesiWire = (o: Record<string, unknown>) => ({
  id: "s-ini",
  perangkat: "Chrome di macOS",
  user_agent: "Mozilla/5.0",
  ip: "10.0.0.1",
  dibuat_pada: "2026-11-08T08:05:00Z",
  terakhir_dipakai_pada: "2026-11-08T09:55:00Z",
  kedaluwarsa_pada: "2026-11-15T08:05:00Z",
  saat_ini: false,
  ...o,
});
const INI = sesiWire({ saat_ini: true });
const ANDROID = sesiWire({
  id: "s-android",
  perangkat: "Chrome di Android",
  ip: "10.0.0.2",
});

let api: FetchPalsu;
afterEach(async () => {
  // Toast Sonner hidup di store global dan lolos dari cleanup render; tanpa dibuang, timernya menggantung proses tes.
  toast.dismiss();
  // Sisa jadwal React dan animasi diberi waktu selesai selagi DOM masih ada; tanpa jeda, tes terakhir berkas bisa
  // memicu "window is not defined" saat `after` di render.tsx melepas DOM.
  await new Promise((r) => setTimeout(r, 30));
  api?.pulihkan();
});

describe("TabSesi", () => {
  it("kartu panel dengan judul tab dan daftar sesi aktif; sesi ini ditandai tanpa tombol cabut", async () => {
    api = pasangFetchPalsu({ "GET /api/auth/sessions": [INI, ANDROID] });
    const { container } = renderHalaman(<TabSesi />, {
      url: "/profil?tab=keamanan",
    });

    const judul = screen.getByRole("heading", {
      name: "Perangkat yang sedang masuk",
    });
    assert.equal(judul.tagName, "H2");
    assert.equal(screen.queryByRole("heading", { level: 1 }), null);
    const daftar = await screen.findByRole("list", { name: "Sesi aktif" });
    const [ini, lain] = within(daftar).getAllByRole("listitem");
    assert.ok(within(ini).getByText("Sesi Ini"));
    assert.equal(within(ini).queryAllByRole("button").length, 0);
    assert.ok(
      within(lain).getByRole("button", {
        name: "Cabut sesi Chrome di Android, IP 10.0.0.2",
      }),
    );
    const kartu = container.firstElementChild as HTMLElement;
    assert.ok(kartu.className.includes("glass"));
    assert.ok(kartu.className.includes("rounded-xl"));
  });

  it("cabut satu sesi dari tab: konfirmasi cepat lalu DELETE", async () => {
    let daftar = [INI, ANDROID];
    api = pasangFetchPalsu({
      "GET /api/auth/sessions": () => daftar,
      "DELETE /api/auth/sessions/s-android": () => {
        daftar = [INI];
        return { id: "s-android" };
      },
    });
    const { user } = renderHalaman(<TabSesi />, {
      url: "/profil?tab=keamanan",
    });

    await user.click(
      await screen.findByRole("button", {
        name: "Cabut sesi Chrome di Android, IP 10.0.0.2",
      }),
    );
    await user.click(screen.getByRole("button", { name: "Konfirmasi cabut" }));

    assert.ok(await screen.findByText("Sesi dicabut"));
    assert.equal(api.ke("DELETE", "/api/auth/sessions/s-android").length, 1);
    await waitFor(() =>
      assert.equal(
        within(screen.getByRole("list", { name: "Sesi aktif" })).getAllByRole(
          "listitem",
        ).length,
        1,
      ),
    );
  });

  it("keluarkan semua perangkat lain dari tab: dialog lalu POST revoke-others", async () => {
    let daftar = [INI, ANDROID];
    api = pasangFetchPalsu({
      "GET /api/auth/sessions": () => daftar,
      "POST /api/auth/sessions/revoke-others": () => {
        daftar = [INI];
        return { dicabut: 1 };
      },
    });
    const { user } = renderHalaman(<TabSesi />, {
      url: "/profil?tab=keamanan",
    });

    await user.click(
      await screen.findByRole("button", {
        name: "Keluarkan semua perangkat lain",
      }),
    );
    const dialog = await screen.findByRole("alertdialog");
    assert.ok(within(dialog).getByText("Keluarkan semua perangkat lain?"));
    await user.click(
      within(dialog).getByRole("button", { name: "Ya, keluarkan" }),
    );

    assert.ok(await screen.findByText("Perangkat lain dikeluarkan"));
    assert.equal(api.ke("POST", "/api/auth/sessions/revoke-others").length, 1);
  });
});
