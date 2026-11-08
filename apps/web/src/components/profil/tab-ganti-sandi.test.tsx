import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { configure, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { qk } from "@/lib/query/keys";
import {
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { TabGantiSandi } from "./tab-ganti-sandi";

// Jaringan palsu dan toast butuh beberapa putaran render; beri kelonggaran saat mesin sibuk (CI).
configure({ asyncUtilTimeout: 3000 });

const RUTE = "/api/auth/change-password";
const URL_TAB = "/profil?tab=ganti-password";

let api: FetchPalsu;
afterEach(async () => {
  // Toast Sonner hidup di store global dan lolos dari cleanup render; tanpa dibuang, timernya menggantung proses tes.
  toast.dismiss();
  // Sisa jadwal React dan animasi diberi waktu selesai selagi DOM masih ada; tanpa jeda, tes terakhir berkas bisa
  // memicu "window is not defined" saat `after` di render.tsx melepas DOM.
  await new Promise((r) => setTimeout(r, 30));
  api?.pulihkan();
});

describe("TabGantiSandi", () => {
  it("kartu panel dengan judul tab (bukan heading halaman auth) dan form ganti kata sandi", () => {
    api = pasangFetchPalsu();
    const { container } = renderHalaman(<TabGantiSandi />, { url: URL_TAB });

    const judul = screen.getByRole("heading", { name: "Ganti kata sandi" });
    assert.equal(judul.tagName, "H2", "judul panel, bukan h1 halaman auth");
    assert.equal(screen.queryByRole("heading", { level: 1 }), null);
    assert.equal(
      screen.queryByRole("img"),
      null,
      "tanpa BrandMark halaman auth",
    );
    assert.ok(screen.getByLabelText("Kata sandi saat ini"));
    assert.ok(screen.getByLabelText("Kata sandi baru"));
    assert.ok(screen.getByLabelText("Ulangi kata sandi baru"));
    assert.ok(screen.getByRole("list", { name: "Aturan kata sandi" }));
    assert.ok(
      screen.getByText(
        "Perangkat lain akan dikeluarkan setelah kata sandi diganti.",
      ),
    );
    assert.equal(
      screen.queryByRole("button", { name: /Simpan dan lanjutkan/ }),
      null,
      "teks tombol alur wajib tidak dipakai di tab",
    );
    const kartu = container.firstElementChild as HTMLElement;
    assert.ok(kartu.className.includes("glass"));
    assert.ok(kartu.className.includes("rounded-xl"));
  });

  it("sukses: tetap di /profil?tab=ganti-password, toast, form direset, tanpa router.replace/refresh", async () => {
    api = pasangFetchPalsu({ [`POST ${RUTE}`]: { success: true } });
    const { user, router, qc } = renderHalaman(<TabGantiSandi />, {
      url: URL_TAB,
    });
    qc.setQueryData(qk.profil.saya(), { marker: true });
    qc.setQueryData(qk.sesi.aktif(), []);

    await user.type(
      screen.getByLabelText("Kata sandi saat ini"),
      "Lama#2025xx",
    );
    await user.type(screen.getByLabelText("Kata sandi baru"), "SandiBaru#2026");
    await user.type(
      screen.getByLabelText("Ulangi kata sandi baru"),
      "SandiBaru#2026",
    );
    assert.ok(screen.getByRole("meter", { name: "Kekuatan kata sandi" }));
    await user.click(screen.getByRole("button", { name: "Ganti kata sandi" }));

    assert.ok(await screen.findByText("Kata sandi diganti"));
    assert.ok(screen.getByText("Perangkat lain telah dikeluarkan."));
    await waitFor(() =>
      assert.equal(
        (screen.getByLabelText("Kata sandi baru") as HTMLInputElement).value,
        "",
      ),
    );
    assert.equal(
      (screen.getByLabelText("Kata sandi saat ini") as HTMLInputElement).value,
      "",
    );
    assert.deepEqual(router.replace, []);
    assert.deepEqual(router.push, []);
    assert.equal(router.refresh, 0);
    assert.equal(window.location.pathname, "/profil");
    assert.equal(window.location.search, "?tab=ganti-password");
    assert.equal(qc.getQueryState(qk.profil.saya())?.isInvalidated, true);
    assert.equal(qc.getQueryState(qk.sesi.aktif())?.isInvalidated, true);
  });
});
