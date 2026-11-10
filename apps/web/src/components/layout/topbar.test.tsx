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
import { Topbar, itemMenuProfil } from "./topbar";

let fetchPalsu: FetchPalsu;
beforeEach(() => {
  fetchPalsu = pasangFetchPalsu();
});
afterEach(() => fetchPalsu.pulihkan());

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
