import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { configure, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import {
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { TabAktivitas } from "./tab-aktivitas";

configure({ asyncUtilTimeout: 3000 });

const PATH_AKTIVITAS = "/api/auth/me/aktivitas";
const PATH_SESI = "/api/auth/me/riwayat-sesi";

const baris = (o: Record<string, unknown>) => ({
  id: "1",
  waktu: "2026-11-08T08:00:00Z",
  aksi: "LOGIN",
  entitas: "pengguna",
  entitas_id: "u",
  hasil: "BERHASIL",
  sebagai: "pelaku",
  perangkat: "Chrome di macOS",
  ip: "10.0.0.1",
  ...o,
});

const sesi = (o: Record<string, unknown>) => ({
  id: "s1",
  perangkat: "Chrome di macOS",
  ip: "10.0.0.1",
  mulai_pada: "2026-11-08T08:00:00Z",
  terakhir_dipakai_pada: "2026-11-08T09:00:00Z",
  status: "AKTIF",
  saat_ini: true,
  ...o,
});

let api: FetchPalsu;
afterEach(async () => {
  toast.dismiss();
  await new Promise((r) => setTimeout(r, 30));
  api?.pulihkan();
});

const kosong = { items: [] };

describe("TabAktivitas: aktivitas akun", () => {
  it("menampilkan label Indonesia, hasil, dan perangkat; baris orang lain tanpa IP dan diberi peran pelaku", async () => {
    api = pasangFetchPalsu({
      [`GET ${PATH_AKTIVITAS}`]: {
        items: [
          baris({ id: "3", aksi: "RESET_MFA", sebagai: "sasaran", perangkat: undefined, ip: undefined, oleh_peran: "ADMIN_SISTEM" }),
          baris({ id: "2", aksi: "CHANGE_PASSWORD", hasil: "GAGAL" }),
          baris({ id: "1" }),
        ],
      },
      [`GET ${PATH_SESI}`]: kosong,
    });
    renderHalaman(<TabAktivitas />, { url: "/profil?tab=aktivitas" });

    const daftar = await screen.findByRole("list", { name: "Aktivitas akun" });
    const [reset, ganti, masuk] = within(daftar).getAllByRole("listitem");
    assert.ok(within(reset).getByText("Verifikasi 2 langkah diatur ulang admin"));
    assert.ok(within(reset).getByText("Oleh orang lain"));
    assert.ok(within(reset).getByText(/dilakukan ADMIN_SISTEM/));
    assert.equal(within(reset).queryByText(/IP/), null);
    assert.ok(within(ganti).getByText("Mengganti kata sandi"));
    assert.ok(within(ganti).getByText("Gagal"));
    assert.ok(within(masuk).getByText("Masuk"));
    assert.ok(within(masuk).getByText(/Chrome di macOS/));
    assert.ok(within(masuk).getByText(/IP 10\.0\.0\.1/));
  });

  it("menyaring di server: jenis, hasil, pelaku, dan rentang tanggal masuk ke query; hapus saringan mengosongkannya", async () => {
    api = pasangFetchPalsu({
      [`GET ${PATH_AKTIVITAS}`]: kosong,
      [`GET ${PATH_SESI}`]: kosong,
    });
    const { user } = renderHalaman(<TabAktivitas />, { url: "/profil?tab=aktivitas" });
    await screen.findByText("Belum ada aktivitas tercatat.");

    await user.click(screen.getByRole("combobox", { name: "Jenis aktivitas" }));
    await user.click(await screen.findByRole("option", { name: "Masuk" }));
    await user.click(screen.getByRole("combobox", { name: "Hasil" }));
    await user.click(await screen.findByRole("option", { name: "Gagal" }));
    await user.click(screen.getByRole("combobox", { name: "Pelaku" }));
    await user.click(await screen.findByRole("option", { name: "Dilakukan saya" }));
    await user.type(screen.getByLabelText("Dari tanggal"), "2026-11-01");

    await waitFor(() => {
      const terakhir = api.ke("GET", PATH_AKTIVITAS).at(-1)!.query;
      assert.equal(terakhir.get("aksi"), "LOGIN");
      assert.equal(terakhir.get("hasil"), "GAGAL");
      assert.equal(terakhir.get("sebagai"), "pelaku");
      assert.equal(terakhir.get("dari"), "2026-11-01");
    });
    assert.ok(await screen.findByText("Tidak ada aktivitas yang cocok dengan saringan."));

    await user.click(screen.getByRole("button", { name: "Hapus saringan" }));
    await waitFor(() => {
      const terakhir = api.ke("GET", PATH_AKTIVITAS).at(-1)!.query;
      assert.equal(terakhir.has("aksi"), false);
      assert.equal(terakhir.has("hasil"), false);
      assert.equal(terakhir.has("sebagai"), false);
    });
    assert.equal(screen.queryByRole("button", { name: "Hapus saringan" }), null);
  });

  it("paginasi kursor: Muat lebih banyak menambah baris dan kursor halaman berikutnya terkirim", async () => {
    api = pasangFetchPalsu({
      [`GET ${PATH_AKTIVITAS}`]: (req: { query: URLSearchParams }) =>
        req.query.get("cursor") === "k2"
          ? { items: [baris({ id: "1", aksi: "LOGOUT" })] }
          : { items: [baris({ id: "2" })], next_cursor: "k2" },
      [`GET ${PATH_SESI}`]: kosong,
    });
    const { user } = renderHalaman(<TabAktivitas />, { url: "/profil?tab=aktivitas" });

    const daftar = await screen.findByRole("list", { name: "Aktivitas akun" });
    assert.equal(within(daftar).getAllByRole("listitem").length, 1);
    await user.click(screen.getAllByRole("button", { name: "Muat lebih banyak" })[0]);

    await waitFor(() =>
      assert.equal(
        within(screen.getByRole("list", { name: "Aktivitas akun" })).getAllByRole("listitem").length,
        2,
      ),
    );
    assert.equal(api.ke("GET", PATH_AKTIVITAS).at(-1)!.query.get("cursor"), "k2");
    assert.equal(screen.queryByRole("button", { name: "Muat lebih banyak" }), null, "halaman terakhir");
  });

  it("galat server: pesan dan Coba lagi memuat ulang", async () => {
    let gagal = true;
    api = pasangFetchPalsu({
      [`GET ${PATH_AKTIVITAS}`]: () => (gagal ? galatApi(500, "boom") : { items: [baris({})] }),
      [`GET ${PATH_SESI}`]: kosong,
    });
    const { user } = renderHalaman(<TabAktivitas />, { url: "/profil?tab=aktivitas" });

    const alert = await screen.findByRole("alert");
    assert.ok(within(alert).getByText(/Gagal memuat aktivitas akun/));
    gagal = false;
    await user.click(within(alert).getByRole("button", { name: "Coba lagi" }));

    assert.ok(await screen.findByRole("list", { name: "Aktivitas akun" }));
  });
});

describe("TabAktivitas: riwayat masuk", () => {
  it("menampilkan status tiap sesi, penanda sesi ini, dan waktu berakhir", async () => {
    api = pasangFetchPalsu({
      [`GET ${PATH_AKTIVITAS}`]: kosong,
      [`GET ${PATH_SESI}`]: {
        items: [
          sesi({}),
          sesi({ id: "s2", perangkat: "Chrome di Android", status: "DICABUT", saat_ini: false, berakhir_pada: "2026-11-07T10:00:00Z" }),
          sesi({ id: "s3", status: "KEDALUWARSA", saat_ini: false, berakhir_pada: "2026-11-01T10:00:00Z" }),
        ],
      },
    });
    renderHalaman(<TabAktivitas />, { url: "/profil?tab=aktivitas" });

    const daftar = await screen.findByRole("list", { name: "Riwayat sesi" });
    const [ini, dicabut, lewat] = within(daftar).getAllByRole("listitem");
    assert.ok(within(ini).getByText("Aktif"));
    assert.ok(within(ini).getByText("Sesi Ini"));
    assert.ok(within(dicabut).getByText("Dicabut"));
    assert.ok(within(dicabut).getByText(/Berakhir/));
    assert.equal(within(dicabut).queryByText("Sesi Ini"), null);
    assert.ok(within(lewat).getByText("Kedaluwarsa"));
  });

  it("filter status dikirim ke server", async () => {
    api = pasangFetchPalsu({
      [`GET ${PATH_AKTIVITAS}`]: kosong,
      [`GET ${PATH_SESI}`]: kosong,
    });
    const { user } = renderHalaman(<TabAktivitas />, { url: "/profil?tab=aktivitas" });
    await screen.findByText("Belum ada sesi yang cocok.");

    await user.click(screen.getByRole("combobox", { name: "Status sesi" }));
    await user.click(await screen.findByRole("option", { name: "Sudah berakhir" }));

    await waitFor(() => assert.equal(api.ke("GET", PATH_SESI).at(-1)!.query.get("status"), "berakhir"));
  });
});
