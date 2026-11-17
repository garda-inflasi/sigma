import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import {
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import {
  sesiImpersonate,
  tangkapNavigasi,
  targetsFixture,
  tuntaskanTimer,
} from "@/test/impersonasi";
import { DialogAlihPeran, type PilihanAwal } from "./dialog-alih-peran";

const TARGETS = "GET /api/impersonation/targets";
const START = "POST /api/impersonation/start";
const STOP = "POST /api/impersonation/stop";
const HASIL = {
  user: { id: "a-bi", nama: "Budi BI", peran: "STRATEGIS_BI" },
  impersonasi: { aktif: true },
  redirectTo: "/eksekutif",
};
const galatBff = (status: number, message: string, kode: string) => ({
  __status: status,
  body: { error: true, message, kode },
});

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

function render(
  opsi: { sesi?: ReturnType<typeof penggunaSesi>; awal?: PilihanAwal } = {},
) {
  return renderHalaman(
    <DialogAlihPeran open onOpenChange={() => {}} awal={opsi.awal} />,
    { sesi: opsi.sesi, url: "/audit?hasil=GAGAL" },
  );
}

const alasan = () => screen.getByRole("textbox", { name: /Alasan/ });
const tombolMulai = () =>
  screen.getByRole("button", { name: /Mulai Impersonate|Ganti sasaran/ }) as HTMLButtonElement;

describe("DialogAlihPeran: daftar sasaran", () => {
  it("memuat sasaran dari API dan mengelompokkan peran per lapis dengan ringkasan akun", async () => {
    api = pasangFetchPalsu({ [TARGETS]: targetsFixture() });
    render();

    const dialog = await screen.findByRole("dialog", { name: "Beralih / Impersonate Role" });
    await within(dialog).findByRole("radio", { name: /Analis Bank Indonesia/ });
    const teks = dialog.textContent ?? "";
    assert.ok(teks.indexOf("Lapis strategis") < teks.indexOf("Lapis operasional"));
    assert.match(
      within(dialog).getByRole("radio", { name: /Analis Bank Indonesia/ }).textContent ?? "",
      /Budi BI/,
      "peran beranggota satu akun menampilkan nama akunnya",
    );
    assert.match(
      within(dialog).getByRole("radio", { name: /Koordinator Lapangan/ }).textContent ?? "",
      /2 akun aktif/,
    );
    assert.ok(within(dialog).getByText(/Dicatat di audit trail/));
    assert.ok(within(dialog).getByText(/berakhir otomatis setelah 60 menit/));
  });

  it("tanpa batas durasi: menjelaskan kembali manual", async () => {
    api = pasangFetchPalsu({ [TARGETS]: targetsFixture({ durasi_maks_menit: null }) });
    render();
    await screen.findByRole("radio", { name: /Analis Bank Indonesia/ });
    assert.ok(screen.getByText(/Tidak ada batas durasi/));
  });

  it("tidak berhak (boleh false), kebijakan nonaktif, dan tanpa akun: pesan yang jelas dan tidak ada tombol mulai aktif", async () => {
    for (const [data, teks] of [
      [targetsFixture({ boleh: false, peran: [] }), /tidak diizinkan memakai impersonasi/],
      [targetsFixture({ aktif: false }), /dinonaktifkan oleh superadmin/],
      [targetsFixture({ peran: [] }), /Belum ada akun aktif/],
    ] as const) {
      api = pasangFetchPalsu({ [TARGETS]: data });
      const { unmount } = render();
      assert.ok(await screen.findByText(teks));
      assert.equal(tombolMulai().disabled, true);
      unmount();
      api.pulihkan();
    }
  });

  it("gagal memuat: panel galat dengan tombol coba lagi yang memuat ulang", async () => {
    let n = 0;
    api = pasangFetchPalsu({
      [TARGETS]: () => (++n === 1 ? { __status: 500, body: { message: "gagal" } } : targetsFixture()),
    });
    const { user } = render();
    await screen.findByText("Daftar sasaran gagal dimuat");
    await user.click(screen.getByRole("button", { name: /Coba lagi/ }));
    await screen.findByRole("radio", { name: /Analis Bank Indonesia/ });
    assert.equal(n, 2);
  });
});

describe("DialogAlihPeran: memulai", () => {
  it("peran beranggota satu akun terpilih otomatis; mulai butuh alasan (wajib) dan mengirim id akun, alasan, dan halaman semula", async () => {
    api = pasangFetchPalsu({ [TARGETS]: targetsFixture(), [START]: HASIL });
    const { user } = render();
    await user.click(await screen.findByRole("radio", { name: /Analis Bank Indonesia/ }));

    // Peran terpilih tetapi alasan wajib belum diisi.
    assert.equal(tombolMulai().disabled, true);
    await user.type(alasan(), "   ");
    assert.equal(tombolMulai().disabled, true, "alasan hanya spasi tidak dihitung");
    await user.type(alasan(), "  periksa dasbor eksekutif  ");
    assert.equal(tombolMulai().disabled, false);
    await user.click(tombolMulai());

    await waitFor(() => assert.deepEqual(nav.tujuan, ["/eksekutif"]));
    const kiriman = api.ke("POST", "/api/impersonation/start");
    assert.equal(kiriman.length, 1);
    assert.deepEqual(kiriman[0].body, {
      target_pengguna_id: "a-bi",
      alasan: "periksa dasbor eksekutif",
      dari: "/audit?hasil=GAGAL",
    });
    assert.ok(kiriman[0].headers["x-csrf-token"] !== undefined);
  });

  it("peran beranggota banyak akun: akun wajib dipilih; mulai mengirim akun yang dipilih", async () => {
    api = pasangFetchPalsu({ [TARGETS]: targetsFixture(), [START]: HASIL });
    const { user } = render();
    await user.click(await screen.findByRole("radio", { name: /Koordinator Lapangan/ }));
    await user.type(alasan(), "uji koordinator");

    // Alasan terisi, tetapi akun belum dipilih.
    assert.equal(tombolMulai().disabled, true);
    const daftar = screen.getByRole("radiogroup", { name: "Akun sasaran" });
    assert.equal(within(daftar).getAllByRole("radio").length, 2);
    // Dua akun: kolom cari tidak perlu.
    assert.equal(screen.queryByRole("textbox", { name: "Cari akun" }), null);

    await user.click(within(daftar).getByRole("radio", { name: /Siti Koordinator/ }));
    assert.equal(tombolMulai().disabled, false);
    await user.click(tombolMulai());

    await waitFor(() => assert.equal(api.ke("POST", "/api/impersonation/start").length, 1));
    assert.equal(
      (api.ke("POST", "/api/impersonation/start")[0].body as { target_pengguna_id: string }).target_pengguna_id,
      "a-k2",
    );
  });

  it("daftar akun panjang diberi kolom cari yang menyaring nama, email, dan lembaga", async () => {
    const akun = Array.from({ length: 7 }, (_, i) => ({
      id: `a-${i}`,
      nama: `Petugas ${i}`,
      email: `petugas${i}@x.id`,
      lembaga: i === 3 ? "Dinas Pertanian" : "FMIPA USK",
      kabupaten_kode: null,
    }));
    api = pasangFetchPalsu({
      [TARGETS]: targetsFixture({
        peran: [{ peran: "OPS_AGREGATOR", label: "Unit Agregator", lapis: "OPERASIONAL", akun }],
      }),
    });
    const { user } = render();
    await user.click(await screen.findByRole("radio", { name: /Unit Agregator/ }));

    const cari = screen.getByRole("textbox", { name: "Cari akun" });
    await user.type(cari, "pertanian");
    const daftar = screen.getByRole("radiogroup", { name: "Akun sasaran" });
    assert.deepEqual(
      within(daftar).getAllByRole("radio").map((r) => r.textContent?.split(" · ")[0]),
      ["Petugas 3petugas3@x.id"],
    );
    await user.clear(cari);
    await user.type(cari, "tidak-ada");
    assert.ok(within(daftar).getByText("Tidak ada akun yang cocok."));
  });

  it("alasan opsional bila kebijakan tidak mewajibkannya: mulai aktif setelah memilih, tanpa alasan terkirim", async () => {
    api = pasangFetchPalsu({
      [TARGETS]: targetsFixture({ wajib_alasan: false }),
      [START]: HASIL,
    });
    const { user } = render();
    await user.click(await screen.findByRole("radio", { name: /Analis Bank Indonesia/ }));
    assert.match(screen.getByText(/\(opsional\)/).textContent ?? "", /opsional/);
    assert.equal(tombolMulai().disabled, false);
    await user.click(tombolMulai());
    await waitFor(() => assert.equal(api.ke("POST", "/api/impersonation/start").length, 1));
    assert.deepEqual(
      (api.ke("POST", "/api/impersonation/start")[0].body as Record<string, unknown>).alasan,
      undefined,
    );
  });

  it("galat dari API tampil di dialog dan dialog tetap terbuka; 409 TARGET_AMBIGU memuat ulang daftar", async () => {
    let jumlahTargets = 0;
    api = pasangFetchPalsu({
      [TARGETS]: () => {
        jumlahTargets++;
        return targetsFixture();
      },
      [START]: galatBff(409, "Peran ini punya lebih dari satu akun.", "TARGET_AMBIGU"),
    });
    const { user } = render();
    await user.click(await screen.findByRole("radio", { name: /Analis Bank Indonesia/ }));
    await user.type(alasan(), "uji");
    await user.click(tombolMulai());

    const galat = await screen.findByRole("alert");
    assert.match(galat.textContent ?? "", /lebih dari satu akun/);
    assert.deepEqual(nav.tujuan, []);
    assert.ok(screen.getByRole("dialog"));
    await waitFor(() => assert.equal(jumlahTargets, 2));
    await waitFor(() => assert.equal(tombolMulai().disabled, false));
  });

  it("403 ditolak ditampilkan apa adanya (pesan server)", async () => {
    api = pasangFetchPalsu({
      [TARGETS]: targetsFixture(),
      [START]: galatBff(403, "Anda tidak berhak meng-impersonate akun ini.", "IMPERSONASI_DITOLAK"),
    });
    const { user } = render();
    await user.click(await screen.findByRole("radio", { name: /Analis Bank Indonesia/ }));
    await user.type(alasan(), "uji");
    await user.click(tombolMulai());
    assert.match((await screen.findByRole("alert")).textContent ?? "", /tidak berhak meng-impersonate/);
  });

  it("pilihan awal dari halaman admin: peran dan akun langsung terpilih", async () => {
    api = pasangFetchPalsu({ [TARGETS]: targetsFixture(), [START]: HASIL });
    const { user } = render({ awal: { peran: "OPS_KOORDINATOR", penggunaId: "a-k2" } });
    const siti = await screen.findByRole("radio", { name: /Siti Koordinator/ });
    assert.equal(siti.getAttribute("aria-checked"), "true");
    assert.equal(
      screen.getByRole("radio", { name: /Koordinator Lapangan/ }).getAttribute("aria-checked"),
      "true",
    );
    await user.type(alasan(), "uji");
    await user.click(tombolMulai());
    await waitFor(() => assert.equal(api.ke("POST", "/api/impersonation/start").length, 1));
    assert.equal(
      (api.ke("POST", "/api/impersonation/start")[0].body as { target_pengguna_id: string }).target_pengguna_id,
      "a-k2",
    );
  });
});

describe("DialogAlihPeran: selama impersonasi", () => {
  it("menjelaskan bahwa memilih sasaran baru MENGGANTI (mengacu akun asli), tombol berlabel 'Ganti sasaran', dan menyediakan kembali", async () => {
    api = pasangFetchPalsu({
      [TARGETS]: targetsFixture(),
      [STOP]: { user: { id: "u", nama: "A", peran: "ADMIN_SISTEM" }, redirectTo: "/pengguna" },
    });
    const { user } = render({ sesi: sesiImpersonate("OPS_KOORDINATOR") });
    await screen.findByRole("radio", { name: /Analis Bank Indonesia/ });

    const catatan = screen.getByText(/Anda sedang impersonate/).parentElement!;
    assert.match(catatan.textContent ?? "", /Zulkifli, S\.P\./);
    assert.match(catatan.textContent ?? "", /mengganti/);
    assert.match(catatan.textContent ?? "", /akun asli Super Admin/);
    assert.equal(tombolMulai().textContent, "Ganti sasaran");

    await user.click(within(catatan).getByRole("button", { name: "Kembali ke Peran Asal" }));
    await waitFor(() => assert.deepEqual(nav.tujuan, ["/pengguna"]));
  });

  it("sesi biasa tidak menampilkan catatan impersonasi", async () => {
    api = pasangFetchPalsu({ [TARGETS]: targetsFixture() });
    render();
    await screen.findByRole("radio", { name: /Analis Bank Indonesia/ });
    assert.equal(screen.queryByText(/Anda sedang impersonate/), null);
    assert.equal(tombolMulai().textContent, "Mulai Impersonate");
  });
});
