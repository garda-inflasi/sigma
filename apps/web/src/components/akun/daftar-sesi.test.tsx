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
import { DaftarSesi } from "./daftar-sesi";

// Dialog Radix dan jaringan palsu butuh beberapa putaran render; beri kelonggaran saat mesin sibuk (CI).
configure({ asyncUtilTimeout: 3000 });

const sesiWire = (o: Record<string, unknown> = {}) => ({
  id: "s-ini",
  perangkat: "Chrome di macOS",
  user_agent: "Mozilla/5.0 (Macintosh) Chrome/130",
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
  dibuat_pada: "2026-11-07T07:30:00Z",
  terakhir_dipakai_pada: "2026-11-08T09:00:00Z",
});
const IPHONE = sesiWire({
  id: "s-iphone",
  perangkat: "Safari di iOS",
  ip: "10.0.0.3",
  dibuat_pada: "2026-11-06T06:00:00Z",
  terakhir_dipakai_pada: "2026-11-07T06:00:00Z",
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

/**
 * Server palsu yang menyimpan daftar sesi: pencabutan mengubah jawaban GET berikutnya, seperti API sungguhan
 * (sesi ini tidak pernah ikut tercabut oleh "perangkat lain").
 */
function pasang(
  awal: Record<string, unknown>[],
  tambahan: Record<string, unknown> = {},
) {
  let daftar = [...awal];
  api = pasangFetchPalsu({
    "GET /api/auth/sessions": () => daftar,
    ...Object.fromEntries(
      awal.map((s) => [
        `DELETE /api/auth/sessions/${s.id as string}`,
        () => {
          daftar = daftar.filter((x) => x.id !== s.id);
          return { id: s.id };
        },
      ]),
    ),
    "POST /api/auth/sessions/revoke-others": () => {
      const sisa = daftar.filter((s) => s.saat_ini);
      const dicabut = daftar.length - sisa.length;
      daftar = sisa;
      return { dicabut };
    },
    "POST /api/auth/logout-all": { dicabut: awal.length },
    ...tambahan,
  });
}

const daftarSesi = () => screen.getByRole("list", { name: "Sesi aktif" });
const butir = () => within(daftarSesi()).getAllByRole("listitem");
const butirDari = (perangkat: string) => {
  const li = butir().find((el) => el.textContent?.includes(perangkat));
  assert.ok(li, `butir ${perangkat}`);
  return li;
};

describe("DaftarSesi: tampilan", () => {
  it("memuat: kerangka berstatus 'Memuat sesi aktif', lalu daftar sesi", async () => {
    let selesai: (v: unknown) => void = () => {};
    pasang([INI], {
      "GET /api/auth/sessions": () =>
        new Promise((r) => {
          selesai = r;
        }),
    });
    renderHalaman(<DaftarSesi />);

    assert.ok(screen.getByRole("status", { name: "Memuat sesi aktif" }));
    assert.equal(screen.queryByRole("list", { name: "Sesi aktif" }), null);

    selesai([INI]);
    assert.ok(await screen.findByRole("list", { name: "Sesi aktif" }));
    assert.equal(
      screen.queryByRole("status", { name: "Memuat sesi aktif" }),
      null,
    );
  });

  it("kartu sesi: ikon perangkat, browser/OS, IP, waktu masuk, dan aktif terakhir; sesi ini paling atas", async () => {
    pasang([ANDROID, IPHONE, INI]);
    renderHalaman(<DaftarSesi />);

    await screen.findByRole("list", { name: "Sesi aktif" });
    const item = butir();
    assert.equal(item.length, 3);
    assert.match(item[0].textContent ?? "", /Chrome di macOS/);
    assert.match(
      item[1].textContent ?? "",
      /Chrome di Android/,
      "sesi lain: yang terakhir dipakai lebih dulu",
    );
    assert.match(item[2].textContent ?? "", /Safari di iOS/);

    const mac = item[0].textContent ?? "";
    assert.match(mac, /IP 10\.0\.0\.1/);
    assert.match(mac, /Masuk 8 Nov 2026, 08:05/);
    assert.match(mac, /Aktif terakhir .+/);
    assert.ok(
      item[0].querySelector("svg.lucide-laptop"),
      "desktop: ikon Laptop",
    );
    assert.ok(
      item[1].querySelector("svg.lucide-smartphone"),
      "Android: ikon Smartphone",
    );
    assert.ok(
      item[2].querySelector("svg.lucide-smartphone"),
      "iOS: ikon Smartphone",
    );
    assert.equal(
      within(item[0]).getByText("Chrome di macOS").getAttribute("title"),
      "Mozilla/5.0 (Macintosh) Chrome/130",
      "user agent lengkap sebagai petunjuk",
    );
    assert.equal(
      item[0].querySelector("time")?.getAttribute("datetime"),
      "2026-11-08T08:05:00Z",
    );
  });

  it("badge 'Sesi Ini' hanya pada sesi saat ini, dan sesi itu tidak punya tombol cabut", async () => {
    pasang([INI, ANDROID, IPHONE]);
    renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });

    const ini = butirDari("Chrome di macOS");
    assert.ok(within(ini).getByText("Sesi Ini"));
    assert.equal(within(ini).queryAllByRole("button").length, 0);
    assert.equal(screen.getAllByText("Sesi Ini").length, 1);
    for (const perangkat of ["Chrome di Android", "Safari di iOS"]) {
      const lain = butirDari(perangkat);
      assert.equal(within(lain).queryByText("Sesi Ini"), null);
      assert.equal(within(lain).getAllByRole("button").length, 1);
    }
  });

  it("tombol cabut tiap sesi lain bernama 'Cabut sesi <perangkat>, IP <ip>'; tanpa IP: tanpa bagian IP", async () => {
    pasang([
      INI,
      ANDROID,
      sesiWire({ id: "s-x", perangkat: "Perangkat tidak dikenal", ip: "" }),
    ]);
    renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });

    assert.ok(
      screen.getByRole("button", {
        name: "Cabut sesi Chrome di Android, IP 10.0.0.2",
      }),
    );
    assert.ok(
      screen.getByRole("button", {
        name: "Cabut sesi Perangkat tidak dikenal",
      }),
    );
    assert.match(
      butirDari("Perangkat tidak dikenal").textContent ?? "",
      /IP tidak diketahui/,
    );
  });

  it("tanpa perangkat lain: penjelasan ditampilkan dan 'Keluarkan semua perangkat lain' nonaktif", async () => {
    pasang([INI]);
    renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });

    assert.ok(screen.getByText("Tidak ada perangkat lain yang sedang masuk."));
    assert.equal(
      (
        screen.getByRole("button", {
          name: "Keluarkan semua perangkat lain",
        }) as HTMLButtonElement
      ).disabled,
      true,
    );
    assert.equal(
      (
        screen.getByRole("button", {
          name: "Keluar dari semua perangkat",
        }) as HTMLButtonElement
      ).disabled,
      false,
    );
  });

  it("daftar kosong: pesan 'tidak ada sesi aktif', tanpa kartu, dan aksi perangkat lain nonaktif", async () => {
    pasang([]);
    renderHalaman(<DaftarSesi />);

    assert.ok(await screen.findByText("Tidak ada sesi aktif yang tercatat."));
    assert.equal(screen.queryAllByRole("listitem").length, 0);
    assert.equal(
      screen.queryByText("Tidak ada perangkat lain yang sedang masuk."),
      null,
      "satu pesan kosong saja",
    );
    assert.equal(
      (
        screen.getByRole("button", {
          name: "Keluarkan semua perangkat lain",
        }) as HTMLButtonElement
      ).disabled,
      true,
    );
  });

  it("gagal memuat: pesan galat dengan 'Coba lagi' yang memuat ulang", async () => {
    let gagal = true;
    pasang([INI], {
      "GET /api/auth/sessions": () =>
        gagal ? galatApi(500, "Gagal memuat sesi") : [INI],
    });
    const { user } = renderHalaman(<DaftarSesi />);

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Gagal memuat sesi/);
    assert.equal(screen.queryByRole("list", { name: "Sesi aktif" }), null);

    gagal = false;
    await user.click(screen.getByRole("button", { name: "Coba lagi" }));
    assert.ok(await screen.findByRole("list", { name: "Sesi aktif" }));
  });
});

describe("DaftarSesi: cabut satu sesi (konfirmasi cepat)", () => {
  const CABUT_ANDROID = "Cabut sesi Chrome di Android, IP 10.0.0.2";

  it("Cabut membuka konfirmasi di tempat; 'Konfirmasi cabut' mengirim DELETE, toast, dan sesi hilang dari daftar", async () => {
    pasang([INI, ANDROID, IPHONE]);
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });

    await user.click(screen.getByRole("button", { name: CABUT_ANDROID }));

    const android = butirDari("Chrome di Android");
    assert.ok(
      within(android).getByRole("button", { name: "Konfirmasi cabut" }),
    );
    assert.ok(within(android).getByRole("button", { name: "Batal" }));
    assert.ok(
      within(android).getByRole("group", {
        name: "Konfirmasi pencabutan sesi Chrome di Android",
      }),
    );
    assert.equal(
      screen.queryByRole("button", { name: CABUT_ANDROID }),
      null,
      "tombol Cabut diganti konfirmasi",
    );
    assert.equal(
      api.ke("DELETE", "/api/auth/sessions/s-android").length,
      0,
      "belum dicabut sebelum dikonfirmasi",
    );

    await user.click(
      within(android).getByRole("button", { name: "Konfirmasi cabut" }),
    );

    assert.ok(await screen.findByText("Sesi dicabut"));
    assert.ok(screen.getByText("Perangkat itu harus masuk ulang."));
    assert.equal(api.ke("DELETE", "/api/auth/sessions/s-android").length, 1);
    await waitFor(() => assert.equal(butir().length, 2));
    assert.equal(
      butir().some((el) => el.textContent?.includes("Chrome di Android")),
      false,
    );
    assert.ok(
      butirDari("Safari di iOS"),
      "sesi lain yang tidak dikonfirmasi tetap ada",
    );
  });

  it("Batal menutup konfirmasi tanpa mengirim apa pun, dan fokus kembali ke tombol Cabut", async () => {
    pasang([INI, ANDROID]);
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });
    await user.click(screen.getByRole("button", { name: CABUT_ANDROID }));
    const batal = within(butirDari("Chrome di Android")).getByRole("button", {
      name: "Batal",
    });
    await waitFor(() => assert.equal(document.activeElement, batal));

    await user.click(batal);

    const cabut = await screen.findByRole("button", { name: CABUT_ANDROID });
    await waitFor(() => assert.equal(document.activeElement, cabut));
    assert.equal(
      screen.queryByRole("button", { name: "Konfirmasi cabut" }),
      null,
    );
    assert.equal(api.ke("DELETE", "/api/auth/sessions/s-android").length, 0);
    assert.equal(butir().length, 2);
  });

  it("Esc membatalkan konfirmasi", async () => {
    pasang([INI, ANDROID]);
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });
    await user.click(screen.getByRole("button", { name: CABUT_ANDROID }));
    await screen.findByRole("button", { name: "Konfirmasi cabut" });

    await user.keyboard("{Escape}");

    assert.ok(await screen.findByRole("button", { name: CABUT_ANDROID }));
    assert.equal(
      screen.queryByRole("button", { name: "Konfirmasi cabut" }),
      null,
    );
    assert.equal(api.ke("DELETE", "/api/auth/sessions/s-android").length, 0);
  });

  it("hanya satu konfirmasi pada satu waktu: meminta cabut sesi lain membatalkan yang sebelumnya", async () => {
    pasang([INI, ANDROID, IPHONE]);
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });

    await user.click(screen.getByRole("button", { name: CABUT_ANDROID }));
    await user.click(
      screen.getByRole("button", {
        name: "Cabut sesi Safari di iOS, IP 10.0.0.3",
      }),
    );

    assert.equal(
      screen.getAllByRole("button", { name: "Konfirmasi cabut" }).length,
      1,
    );
    assert.ok(
      within(butirDari("Safari di iOS")).getByRole("button", {
        name: "Konfirmasi cabut",
      }),
    );
    assert.ok(screen.getByRole("button", { name: CABUT_ANDROID }));
  });

  it("server menolak pencabutan: toast galat, konfirmasi ditutup, daftar dimuat ulang", async () => {
    pasang([INI, ANDROID], {
      "DELETE /api/auth/sessions/s-android": galatApi(
        404,
        "Sesi tidak ditemukan",
      ),
    });
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });
    await user.click(screen.getByRole("button", { name: CABUT_ANDROID }));

    await user.click(screen.getByRole("button", { name: "Konfirmasi cabut" }));

    assert.ok(await screen.findByText("Gagal mencabut sesi"));
    assert.ok(screen.getByText("Sesi tidak ditemukan"));
    assert.ok(await screen.findByRole("button", { name: CABUT_ANDROID }));
    assert.equal(
      screen.queryByRole("button", { name: "Konfirmasi cabut" }),
      null,
    );
    assert.ok(
      api.ke("GET", "/api/auth/sessions").length >= 2,
      "daftar dimuat ulang agar sesuai keadaan server",
    );
  });
});

describe("DaftarSesi: keluarkan semua perangkat lain", () => {
  const TOMBOL = "Keluarkan semua perangkat lain";

  it("dialog konfirmasi menyebut jumlah perangkat; 'Ya, keluarkan' memanggil revoke-others, toast, dan menyisakan sesi ini", async () => {
    pasang([INI, ANDROID, IPHONE]);
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });

    await user.click(screen.getByRole("button", { name: TOMBOL }));

    const dialog = await screen.findByRole("alertdialog");
    assert.ok(within(dialog).getByText("Keluarkan semua perangkat lain?"));
    assert.match(
      dialog.textContent ?? "",
      /2 perangkat lain akan diakhiri sesinya/,
    );
    assert.equal(api.ke("POST", "/api/auth/sessions/revoke-others").length, 0);

    await user.click(
      within(dialog).getByRole("button", { name: "Ya, keluarkan" }),
    );

    assert.ok(await screen.findByText("Perangkat lain dikeluarkan"));
    assert.ok(screen.getByText("2 sesi diakhiri."));
    assert.equal(api.ke("POST", "/api/auth/sessions/revoke-others").length, 1);
    await waitFor(() => assert.equal(screen.queryByRole("alertdialog"), null));
    await waitFor(() => assert.equal(butir().length, 1));
    assert.ok(within(butir()[0]).getByText("Sesi Ini"));
    assert.ok(screen.getByText("Tidak ada perangkat lain yang sedang masuk."));
    await waitFor(() =>
      assert.equal(
        (screen.getByRole("button", { name: TOMBOL }) as HTMLButtonElement)
          .disabled,
        true,
        "tidak ada perangkat lain lagi",
      ),
    );
  });

  it("Batal menutup dialog tanpa memanggil server", async () => {
    pasang([INI, ANDROID]);
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });
    await user.click(screen.getByRole("button", { name: TOMBOL }));

    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Batal" }));

    await waitFor(() => assert.equal(screen.queryByRole("alertdialog"), null));
    assert.equal(api.ke("POST", "/api/auth/sessions/revoke-others").length, 0);
    assert.equal(butir().length, 2);
  });

  it("server gagal: toast galat dan dialog tertutup, sesi tetap tampil", async () => {
    pasang([INI, ANDROID], {
      "POST /api/auth/sessions/revoke-others": galatApi(
        500,
        "Kesalahan server",
      ),
    });
    const { user } = renderHalaman(<DaftarSesi />);
    await screen.findByRole("list", { name: "Sesi aktif" });
    await user.click(screen.getByRole("button", { name: TOMBOL }));
    const dialog = await screen.findByRole("alertdialog");

    await user.click(
      within(dialog).getByRole("button", { name: "Ya, keluarkan" }),
    );

    assert.ok(await screen.findByText("Gagal mengeluarkan perangkat lain"));
    await waitFor(() => assert.equal(screen.queryByRole("alertdialog"), null));
    assert.equal(butir().length, 2);
  });
});

describe("DaftarSesi: keluar dari semua perangkat", () => {
  /** Mencegat pemuatan ulang penuh ke /masuk (happy-dom akan benar-benar menavigasi window). */
  function cegatPindah() {
    const tujuan: string[] = [];
    Object.defineProperty(window.location, "assign", {
      configurable: true,
      value: (url: string) => void tujuan.push(url),
    });
    return tujuan;
  }

  afterEach(() => {
    delete (window.location as { assign?: unknown }).assign;
  });

  it("dialog konfirmasi lalu logout-all: semua sesi berakhir dan pengguna dimuat ulang ke /masuk", async () => {
    pasang([INI, ANDROID]);
    const { user } = renderHalaman(<DaftarSesi />);
    const tujuan = cegatPindah();
    await screen.findByRole("list", { name: "Sesi aktif" });

    await user.click(
      screen.getByRole("button", { name: "Keluar dari semua perangkat" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    assert.ok(within(dialog).getByText("Keluar dari semua perangkat?"));
    assert.match(
      dialog.textContent ?? "",
      /Semua sesi diakhiri, termasuk perangkat ini\./,
    );
    assert.equal(api.ke("POST", "/api/auth/logout-all").length, 0);

    await user.click(
      within(dialog).getByRole("button", { name: "Ya, keluar semua" }),
    );

    await waitFor(() => assert.deepEqual(tujuan, ["/masuk"]));
    assert.equal(api.ke("POST", "/api/auth/logout-all").length, 1);
  });

  it("Batal menutup dialog tanpa keluar", async () => {
    pasang([INI, ANDROID]);
    const { user } = renderHalaman(<DaftarSesi />);
    const tujuan = cegatPindah();
    await screen.findByRole("list", { name: "Sesi aktif" });
    await user.click(
      screen.getByRole("button", { name: "Keluar dari semua perangkat" }),
    );

    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Batal" }));

    await waitFor(() => assert.equal(screen.queryByRole("alertdialog"), null));
    assert.equal(api.ke("POST", "/api/auth/logout-all").length, 0);
    assert.deepEqual(tujuan, []);
  });

  it("server gagal: toast galat, dialog tertutup, pengguna tetap di halaman dan tombol aktif lagi", async () => {
    pasang([INI, ANDROID], {
      "POST /api/auth/logout-all": galatApi(500, "Kesalahan server"),
    });
    const { user } = renderHalaman(<DaftarSesi />);
    const tujuan = cegatPindah();
    await screen.findByRole("list", { name: "Sesi aktif" });
    await user.click(
      screen.getByRole("button", { name: "Keluar dari semua perangkat" }),
    );
    const dialog = await screen.findByRole("alertdialog");

    await user.click(
      within(dialog).getByRole("button", { name: "Ya, keluar semua" }),
    );

    assert.ok(await screen.findByText("Gagal keluar dari semua perangkat"));
    await waitFor(() => assert.equal(screen.queryByRole("alertdialog"), null));
    assert.deepEqual(tujuan, []);
    assert.equal(
      (
        screen.getByRole("button", {
          name: "Keluar dari semua perangkat",
        }) as HTMLButtonElement
      ).disabled,
      false,
    );
  });
});
