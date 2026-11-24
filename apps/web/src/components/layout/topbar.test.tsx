import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import { PathnameContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import type { ReactNode } from "react";
import { muatFixture } from "@/lib/api/__fixtures__/muat";
import type { SiklusAktifResponse } from "@/lib/api/siklus";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import {
  sesiImpersonate,
  targetsFixture,
  tangkapNavigasi,
  tuntaskanTimer,
} from "@/test/impersonasi";
import { Topbar, itemMenuProfil } from "./topbar";

let fetchPalsu: FetchPalsu;
beforeEach(() => {
  fetchPalsu = pasangFetchPalsu();
});
afterEach(() => fetchPalsu.pulihkan());
afterEach(tuntaskanTimer);

// Peran tanpa dashboard.operasional dan ews.lihat: topbar tidak memuat siklus dan tidak menampilkan lonceng EWS.
const sesi = penggunaSesi({
  role: "OPS_MAHASISWA",
  nama: "Ani Mahasiswa",
  lembaga: "FMIPA USK",
});

function renderTopbar(path = "/profil") {
  const bungkus = (isi: ReactNode) => (
    <PathnameContext.Provider value={path}>{isi}</PathnameContext.Provider>
  );
  return renderHalaman(bungkus(<Topbar />), { url: path, sesi });
}

async function bukaMenuAkun(user: ReturnType<typeof renderHalaman>["user"]) {
  await user.click(screen.getByRole("button", { name: "Menu akun" }));
  return screen.findByRole("menu");
}

/** Urutan isi menu: item (teks + href bila tautan) dan pemisah. */
function urutanMenu(menu: HTMLElement) {
  return [
    ...menu.querySelectorAll<HTMLElement>(
      '[role="menuitem"],[role="separator"]',
    ),
  ].map((e) =>
    e.getAttribute("role") === "separator"
      ? "---"
      : `${e.textContent?.trim()}${e.getAttribute("href") ? ` -> ${e.getAttribute("href")}` : ""}`,
  );
}

describe("Topbar: menu akun", () => {
  it("urutan: pemisah, Profil Saya, tiga bagian keamanan, pemisah, tema, pemisah, Keluar", async () => {
    const { user } = renderTopbar();
    const menu = await bukaMenuAkun(user);

    // Header (nama, peran, lembaga) bukan menuitem; pemisah pertama memisahkannya dari tautan profil.
    assert.deepEqual(urutanMenu(menu), [
      "---",
      "Profil Saya -> /profil",
      "Ganti Kata Sandi -> /profil?tab=ganti-password",
      "Verifikasi 2 Langkah -> /profil?tab=mfa",
      "Perangkat & Sesi Aktif -> /profil?tab=keamanan",
      "---",
      "Mode gelap",
      "---",
      "Keluar",
    ]);
  });

  it("header menu tetap memuat nama, peran, dan lembaga, lalu pemisah sebelum Profil Saya", async () => {
    const { user } = renderTopbar();
    const menu = await bukaMenuAkun(user);

    const anak = [...menu.children];
    assert.match(anak[0].textContent ?? "", /Ani Mahasiswa/);
    assert.match(anak[0].textContent ?? "", /Agen Lapangan Mahasiswa/);
    assert.match(anak[0].textContent ?? "", /FMIPA USK/);
    assert.equal(anak[1].getAttribute("role"), "separator");
    assert.match(anak[2].textContent ?? "", /Profil Saya/);
  });

  it("setiap tautan adalah menuitem ber-href yang membuka tab yang benar (deep link ?tab=)", async () => {
    const { user } = renderTopbar();
    const menu = await bukaMenuAkun(user);

    const tautan = Object.fromEntries(
      within(menu)
        .getAllByRole("menuitem")
        .filter((e) => e.getAttribute("href"))
        .map((e) => [e.textContent?.trim(), e.getAttribute("href")]),
    );
    assert.deepEqual(tautan, {
      "Profil Saya": "/profil",
      "Ganti Kata Sandi": "/profil?tab=ganti-password",
      "Verifikasi 2 Langkah": "/profil?tab=mfa",
      "Perangkat & Sesi Aktif": "/profil?tab=keamanan",
    });
  });

  it("tidak ada lagi tautan ke halaman auth lama dari menu akun", async () => {
    const { user } = renderTopbar();
    const menu = await bukaMenuAkun(user);

    const href = [...menu.querySelectorAll("a")].map((a) =>
      a.getAttribute("href"),
    );
    for (const lama of ["/ganti-password", "/mfa", "/keamanan"]) {
      assert.equal(href.includes(lama), false, lama);
    }
  });

  it("tombol menu akun menampilkan inisial nama", () => {
    renderTopbar();
    assert.equal(
      screen.getByRole("button", { name: "Menu akun" }).textContent,
      "AM",
    );
  });
});

describe("Topbar: judul halaman", () => {
  it("di /profil judulnya 'Profil saya', bukan nama aplikasi", () => {
    renderTopbar("/profil");
    assert.ok(screen.getByText("Profil saya"));
    assert.equal(screen.queryByText("Garda Inflasi", { selector: "p" }), null);
  });

  it("halaman lain tetap berjudul dari item nav aktif", () => {
    renderTopbar("/audit");
    assert.ok(screen.getByText("Audit trail"));
    assert.equal(screen.queryByText("Profil saya"), null);
  });

  it("halaman tanpa entri nav dan bukan profil memakai nama aplikasi", () => {
    renderTopbar("/tidak/ada");
    assert.ok(screen.getByText("Garda Inflasi", { selector: "p" }));
  });
});

describe("itemMenuProfil", () => {
  it("mode normal: empat item dalam urutan menu akun", () => {
    assert.deepEqual(
      itemMenuProfil(false).map((i) => [i.tab, i.label]),
      [
        ["profil", "Profil Saya"],
        ["ganti-password", "Ganti Kata Sandi"],
        ["mfa", "Verifikasi 2 Langkah"],
        ["keamanan", "Perangkat & Sesi Aktif"],
      ],
    );
  });

  it("mode demo: hanya Profil Saya, tiga tautan keamanan disembunyikan", () => {
    assert.deepEqual(
      itemMenuProfil(true).map((i) => i.tab),
      ["profil"],
    );
  });
});

describe("Topbar: siklus ekonomi aktif", () => {
  const SIKLUS = "GET /v1/siklus/aktif";
  // Peran dengan dashboard.operasional: tajuk memuat siklus aktif dari API.
  const strategis = penggunaSesi({ role: "STRATEGIS_FMIPA", nama: "Ani Strategis", lembaga: "FMIPA USK" });
  const pil = () => screen.findByRole("link", { name: /Tekanan harga \/ defisit|Siklus normal|Surplus pangan/ });

  function renderDenganSiklus(jawaban: unknown) {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu({ [SIKLUS]: jawaban });
    return renderHalaman(
      <PathnameContext.Provider value="/operasional">
        <Topbar />
      </PathnameContext.Provider>,
      { url: "/operasional", sesi: strategis },
    );
  }

  it("siklus dari seed (tidak otomatis, alasan null): label siklus dan keterangan sejak kapan saja", async () => {
    renderDenganSiklus(amplop(muatFixture<SiklusAktifResponse>("provinsi", "siklus-aktif")));
    const pill = await pil();
    assert.equal(pill.textContent?.trim(), "Tekanan harga / defisit");
    assert.equal(pill.getAttribute("title"), "Sejak 4 Nov 2026");
    assert.equal(pill.getAttribute("href"), "/intervensi?siklus=tekanan");
  });

  it("siklus otomatis: keterangan menyebut penentuan otomatis dan ringkasan alasan dari mesin", async () => {
    const nyata = muatFixture<SiklusAktifResponse>("provinsi", "siklus-aktif-otomatis-tekanan");
    renderDenganSiklus(amplop(nyata));
    const pill = await pil();
    assert.equal(pill.getAttribute("title"), `Ditentukan otomatis sejak 10 Nov 2026. ${nyata.alasan!.ringkasan}`);
    assert.match(pill.getAttribute("title") ?? "", /^Ditentukan otomatis sejak 10 Nov 2026\. Siklus awal TEKANAN: 6 alert LONJAKAN_HARGA/);
  });

  it("siklus otomatis NORMAL dan SURPLUS: label mengikuti siklus, alasan tetap dipakai", async () => {
    renderDenganSiklus(amplop(muatFixture<SiklusAktifResponse>("provinsi", "siklus-aktif-otomatis-normal")));
    let pill = await pil();
    assert.equal(pill.textContent?.trim(), "Siklus normal");
    assert.match(pill.getAttribute("title") ?? "", /Siklus awal NORMAL: tidak ada aturan yang memicu\./);
    document.body.innerHTML = "";

    renderDenganSiklus(amplop(muatFixture<SiklusAktifResponse>("provinsi", "siklus-aktif-otomatis-surplus")));
    pill = await pil();
    assert.equal(pill.textContent?.trim(), "Surplus pangan");
    assert.match(pill.getAttribute("title") ?? "", /utilisasi stok 82,1% ≥ 50%/);
  });

  it("siklus belum ada (404 pada basis data yang belum pernah dievaluasi): tajuk tetap tampil tanpa label siklus", async () => {
    renderDenganSiklus(galatApi(404, "Siklus aktif tidak ditemukan untuk wilayah ini"));
    assert.ok(await screen.findByRole("button", { name: "Menu akun" }));
    await waitFor(() => assert.equal(fetchPalsu.ke("GET", "/v1/siklus/aktif").length >= 1, true));
    assert.equal(screen.queryByRole("link", { name: /Tekanan harga|Siklus normal|Surplus pangan/ }), null);
  });

  it("peran tanpa dashboard.operasional tidak memanggil siklus sama sekali", async () => {
    renderTopbar();
    assert.ok(screen.getByRole("button", { name: "Menu akun" }));
    assert.equal(fetchPalsu.ke("GET", "/v1/siklus/aktif").length, 0);
  });
});

describe("Topbar: impersonasi", () => {
  const TARGETS = "GET /api/impersonation/targets";
  const STOP = "POST /api/impersonation/stop";

  function renderDengan(sesiUji: ReturnType<typeof penggunaSesi>, path = "/profil") {
    return renderHalaman(
      <PathnameContext.Provider value={path}>
        <Topbar />
      </PathnameContext.Provider>,
      { url: path, sesi: sesiUji },
    );
  }

  it("sesi biasa: tidak ada chip kembali dan tombol RoleSwitcher tidak berubah", () => {
    renderTopbar();
    assert.equal(screen.queryByTestId("chip-kembali-peran"), null);
  });

  it("selama impersonasi: chip 'Kembali ke [Peran Asal]' tepat di sebelah lonceng dan avatar (lonceng, chip, avatar)", async () => {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu();
    // Peran dengan lonceng EWS tetapi tanpa dasbor operasional (tidak memuat siklus).
    renderDengan(
      sesiImpersonate("OPS_MAHASISWA", { permissions: ["ews.lihat", "harga.input"] }),
    );

    const topbar = screen.getByRole("banner");
    const lonceng = await screen.findByRole("link", { name: /Peringatan dini/ });
    const chip = screen.getByRole("button", { name: "Kembali ke Administrator Sistem" });
    const avatar = screen.getByRole("button", { name: "Menu akun" });
    assert.match(topbar.className, /\bshrink-0\b/);
    assert.match(chip.className, /\bshrink-0\b/);
    const urutan = (a: Node, b: Node) =>
      Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);
    assert.ok(urutan(lonceng, chip), "chip sesudah lonceng");
    assert.ok(urutan(chip, avatar), "chip sebelum avatar");
    assert.equal(chip.parentElement, avatar.parentElement, "satu baris dengan avatar");
    assert.equal(chip.nextElementSibling, avatar, "tepat bersebelahan dengan avatar");
  });

  it("chip memanggil stop dan memuat penuh ke tujuan dari BFF", async () => {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu({
      [STOP]: { user: { id: "u", nama: "A", peran: "ADMIN_SISTEM" }, redirectTo: "/" },
    });
    const nav = tangkapNavigasi();
    try {
      const { user } = renderDengan(sesiImpersonate("OPS_MAHASISWA"));
      await user.click(screen.getByRole("button", { name: "Kembali ke Administrator Sistem" }));
      await waitFor(() => assert.deepEqual(nav.tujuan, ["/"]));
    } finally {
      nav.pulihkan();
    }
  });

  it("menu akun selama impersonasi: label menyebut pelaku asli; 'Beralih' dan 'Kembali ke Administrator Sistem' sebelum tema", async () => {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu();
    const { user } = renderDengan(sesiImpersonate("OPS_MAHASISWA"));
    const menu = await bukaMenuAkun(user);

    assert.match(menu.children[0].textContent ?? "", /Impersonate oleh Super Admin/);
    assert.deepEqual(urutanMenu(menu), [
      "---",
      "Profil Saya -> /profil",
      "Ganti Kata Sandi -> /profil?tab=ganti-password",
      "Verifikasi 2 Langkah -> /profil?tab=mfa",
      "Perangkat & Sesi Aktif -> /profil?tab=keamanan",
      "---",
      "Beralih / Impersonate Role",
      "Kembali ke Administrator Sistem",
      "---",
      "Mode gelap",
      "---",
      "Keluar",
    ]);
  });

  it("superadmin (ADMIN_SISTEM): item 'Beralih / Impersonate Role' tampil tanpa bertanya ke API, tanpa item kembali", async () => {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu();
    const { user } = renderDengan(penggunaSesi({ role: "ADMIN_SISTEM" }));
    const menu = await bukaMenuAkun(user);

    const urutan = urutanMenu(menu);
    assert.ok(urutan.includes("Beralih / Impersonate Role"));
    assert.ok(!urutan.includes("Kembali ke Administrator Sistem"));
    assert.equal(fetchPalsu.ke("GET", "/api/impersonation/targets").length, 0);
  });

  it("peran lain yang di-allowlist (targets.boleh true): item tampil; yang tidak berhak: menu tidak berubah", async () => {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu({ [TARGETS]: targetsFixture({ boleh: true }) });
    const { user, unmount } = renderDengan(penggunaSesi({ role: "MANAJER_MUTU" }));
    await waitFor(() => assert.equal(fetchPalsu.ke("GET", "/api/impersonation/targets").length, 1));
    await waitFor(async () => {
      const menu = await bukaMenuAkun(user);
      assert.ok(urutanMenu(menu).includes("Beralih / Impersonate Role"));
    });
    unmount();

    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu({ [TARGETS]: targetsFixture({ boleh: false, peran: [] }) });
    const kedua = renderDengan(penggunaSesi({ role: "MANAJER_MUTU" }));
    await waitFor(() => assert.equal(fetchPalsu.ke("GET", "/api/impersonation/targets").length, 1));
    const menu = await bukaMenuAkun(kedua.user);
    assert.ok(!urutanMenu(menu).includes("Beralih / Impersonate Role"));
  });

  it("targets gagal (API lama, jaringan): menu tidak berubah dan tidak diulang", async () => {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu({ [TARGETS]: galatApi(500, "gagal") });
    const { user } = renderDengan(penggunaSesi({ role: "MANAJER_MUTU" }));
    const menu = await bukaMenuAkun(user);
    assert.ok(!urutanMenu(menu).includes("Beralih / Impersonate Role"));
    assert.equal(fetchPalsu.ke("GET", "/api/impersonation/targets").length, 1);
  });

  it("memilih 'Beralih / Impersonate Role' membuka dialog pemilih", async () => {
    fetchPalsu.pulihkan();
    fetchPalsu = pasangFetchPalsu({ [TARGETS]: targetsFixture() });
    const { user } = renderDengan(penggunaSesi({ role: "ADMIN_SISTEM" }));
    await bukaMenuAkun(user);
    await user.click(screen.getByRole("menuitem", { name: /Beralih \/ Impersonate Role/ }));

    const dialog = await screen.findByRole("dialog", { name: "Beralih / Impersonate Role" });
    assert.ok(within(dialog).getByText("Peran sasaran"));
  });
});
