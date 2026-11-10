import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import { muatFixture } from "@/lib/api/__fixtures__/muat";
import type { AktivitasLapanganResponse } from "@/lib/api/dashboard";
import type {
  AktivitasPetugasItemResponse,
  AktivitasPetugasResponse,
} from "@/lib/api/dashboard-operasional";
import { USERS } from "@/lib/data/reference";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { BATAS_PETUGAS, FieldActivity } from "./field-activity";

const WILAYAH = "/v1/dashboard/operasional/aktivitas-lapangan";
const PETUGAS = "/v1/dashboard/operasional/aktivitas-petugas";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

/** Data nyata peran provinsi (fixture hasil tangkapan API Go): sepuluh kabupaten, satu petugas tiap kabupaten. */
const wilayahNyata = () =>
  muatFixture<AktivitasLapanganResponse[]>("provinsi", "aktivitas-lapangan");
const petugasNyata = () =>
  muatFixture<AktivitasPetugasResponse>("provinsi", "aktivitas-petugas");

const labelBar = () =>
  screen.getAllByRole("progressbar").map((b) => b.getAttribute("aria-label"));
const jumlahShimmer = () => document.querySelectorAll(".shimmer").length;

async function bukaPerPetugas(user: ReturnType<typeof renderHalaman>["user"]) {
  await user.click(screen.getByRole("tab", { name: "Per petugas" }));
}

/** `n` petugas sintetis (nama dan ID unik), capaian naik menurut urutan. */
function petugasBanyak(n: number): AktivitasPetugasResponse {
  const petugas: AktivitasPetugasItemResponse[] = Array.from(
    { length: n },
    (_, i) => ({
      id: `00000000-0000-4000-8000-${String(i).padStart(12, "0")}`,
      nama: `Petugas ${String(i + 1).padStart(3, "0")}`,
      kabupaten_kode: "1171",
      kabupaten_nama: "KOTA BANDA ACEH",
      entri_hari_ini: i % 15,
      target_harian: 14,
      menunggu_validasi: 0,
      ditolak_jendela: 0,
      entri_terakhir_pada: null,
      tugas_aktif: 0,
      tugas_selesai: 0,
      tugas_terlambat: 0,
    }),
  );
  return { tanggal: "2026-11-10", jendela_hari: 7, petugas };
}

describe("FieldActivity: per wilayah (bawaan)", () => {
  it("memuat dengan shimmer lalu menampilkan capaian tiap kabupaten, yang tertinggi di atas; petugas belum diminta", async () => {
    api = pasangFetchPalsu({ [`GET ${WILAYAH}`]: amplop(wilayahNyata()) });
    renderHalaman(<FieldActivity />, { url: "/operasional" });

    assert.ok(jumlahShimmer() > 0, "kerangka selama memuat");
    assert.ok(await screen.findByText("Langsa"));
    assert.equal(screen.getAllByRole("progressbar").length, 10);
    assert.deepEqual(
      labelBar().slice(0, 2),
      ["Capaian Langsa", "Capaian Pidie"],
      "11/14 lalu 10/14 (urutan server dipertahankan pada capaian sama)",
    );
    assert.equal(
      screen.getByText("Aceh Timur").parentElement?.textContent,
      "Aceh Timur2/14",
    );
    assert.equal(
      screen
        .getByRole("tab", { name: "Per wilayah" })
        .getAttribute("aria-selected"),
      "true",
    );
    assert.equal(
      api.ke("GET", PETUGAS).length,
      0,
      "rincian per petugas tidak diminta sebelum dipilih",
    );
    assert.match(
      screen.getByText("Capaian entri hari ini terhadap target").textContent ??
        "",
      /terhadap target/,
    );
  });

  it("tanpa petugas: keterangan kosong", async () => {
    api = pasangFetchPalsu({ [`GET ${WILAYAH}`]: amplop([]) });
    renderHalaman(<FieldActivity />, { url: "/operasional" });
    assert.ok(await screen.findByText("Belum ada petugas lapangan aktif."));
    assert.equal(screen.queryAllByRole("progressbar").length, 0);
  });

  it("galat: panel galat dengan pesan server dan tombol coba lagi yang memuat ulang", async () => {
    let panggilan = 0;
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: () =>
        ++panggilan === 1
          ? galatApi(500, "Gagal mengambil aktivitas lapangan")
          : amplop(wilayahNyata()),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Aktivitas per wilayah gagal dimuat/);
    assert.match(alert.textContent ?? "", /Gagal mengambil aktivitas lapangan/);
    assert.equal(
      screen.queryByText("Belum ada petugas lapangan aktif."),
      null,
      "galat bukan keadaan kosong",
    );

    await user.click(within(alert).getByRole("button", { name: /Coba lagi/ }));
    assert.ok(await screen.findByText("Langsa"));
    assert.equal(screen.queryByRole("alert"), null);
    assert.equal(panggilan, 2);
  });
});

describe("FieldActivity: per petugas", () => {
  it("dibuka lewat sakelar: nama dari API, capaian terendah di atas, penanda menunggu validasi, terlambat, dan ditolak", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop(wilayahNyata()),
      [`GET ${PETUGAS}`]: amplop(petugasNyata()),
    });
    const { user, container } = renderHalaman(<FieldActivity />, {
      url: "/operasional",
    });
    await screen.findByText("Langsa");

    await bukaPerPetugas(user);
    assert.equal(
      screen
        .getByRole("tab", { name: "Per petugas" })
        .getAttribute("aria-selected"),
      "true",
    );
    assert.ok(await screen.findByText("Nurul Fajri"));
    assert.equal(screen.getAllByRole("progressbar").length, 10);
    assert.deepEqual(
      labelBar(),
      [
        "Nurul Fajri",
        "Dewi Sartika",
        "Rahmat Hidayat",
        "Teuku Irfan",
        "Muhammad Rizki",
        "Cut Nanda",
        "Siti Rahmah",
        "Aulia Rahman",
        "Rizky Pratama",
        "Fitriani",
      ].map((n) => `Capaian ${n}`),
      "terendah dulu; capaian sama menurut urutan server (kabupaten, nama)",
    );

    const nurul = screen.getByText("Nurul Fajri").closest("li")!;
    assert.equal(
      within(nurul).getByText("2").parentElement?.textContent,
      "2/14",
    );
    assert.ok(within(nurul).getByText("Aceh Timur"), "nama kabupaten ringkas");
    // Angka pada penanda memakai font angka (.num); teksnya tetap terbaca utuh.
    const penanda = (kata: RegExp) => within(nurul).getByText(kata);
    assert.equal(
      penanda(/menunggu validasi/).textContent,
      "9 menunggu validasi",
    );
    assert.equal(penanda(/tugas terlambat/).textContent, "1 tugas terlambat");
    const ditolak = penanda(/ditolak/);
    assert.equal(ditolak.textContent, "14 ditolak");
    assert.equal(ditolak.querySelector(".num")?.textContent, "14");
    assert.equal(
      ditolak.getAttribute("title"),
      "Entri ditolak dalam 7 hari terakhir",
    );
    assert.equal(
      within(nurul).getByRole("progressbar").getAttribute("aria-valuenow"),
      "14",
    );
    assert.match(
      within(nurul).getByText("2").parentElement?.getAttribute("title") ?? "",
      /^Entri terakhir \d{1,2} Nov 2026 \d\d\.\d\d$/,
    );

    assert.equal(api.ke("GET", PETUGAS).length, 1);
    assert.equal(
      api.ke("GET", PETUGAS)[0].query.size,
      0,
      "tanpa filter kabupaten",
    );
    // Hanya data yang ada di respons: tidak ada email atau telepon, dan tidak ada nama dari referensi statis.
    assert.doesNotMatch(container.textContent ?? "", /@|\+62/);
    const teks = container.textContent ?? "";
    assert.ok(
      USERS.filter((u) => u.role === "OPS_MAHASISWA").every(
        (u) => !teks.includes(u.nama),
      ),
    );
  });

  it("petugas baru yang belum pernah mencatat (jawaban nyata): 0/14 di urutan teratas, tanpa penanda", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: amplop(
        muatFixture("provinsi", "aktivitas-petugas-tanpa-entri"),
      ),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    const baru = (await screen.findByText("Petugas Baru Contoh")).closest(
      "li",
    )!;
    assert.equal(labelBar()[0], "Capaian Petugas Baru Contoh");
    assert.equal(
      within(baru).getByText("0").parentElement?.textContent,
      "0/14",
    );
    assert.equal(
      within(baru).getByText("0").parentElement?.getAttribute("title"),
      "Belum pernah mencatat entri",
    );
    assert.equal(within(baru).getByRole("progressbar").getAttribute("aria-valuenow"), "0");
    assert.ok(within(baru).getByText("Simeulue"));
    assert.equal(
      within(baru).queryByText(/menunggu validasi|tugas terlambat|ditolak/),
      null,
    );
  });

  it("penanda hanya muncul bila ada nilainya", async () => {
    const data = petugasNyata();
    data.petugas = [
      {
        ...data.petugas[0],
        menunggu_validasi: 0,
        tugas_terlambat: 0,
        ditolak_jendela: 0,
        entri_terakhir_pada: null,
      },
    ];
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: amplop(data),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    const baris = (await screen.findByText(data.petugas[0].nama)).closest(
      "li",
    )!;
    assert.equal(
      within(baris).queryByText(/menunggu validasi|tugas terlambat|ditolak/),
      null,
    );
    assert.equal(
      within(baris)
        .getByText(String(data.petugas[0].entri_hari_ini))
        .parentElement?.getAttribute("title"),
      "Belum pernah mencatat entri",
    );
  });

  it("memuat: kerangka shimmer sampai data tiba", async () => {
    let lepas = () => {};
    const tertahan = new Promise<void>((r) => (lepas = r));
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: async () => {
        await tertahan;
        return amplop(petugasNyata());
      },
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    await waitFor(() => assert.ok(jumlahShimmer() > 0));
    assert.equal(screen.queryAllByRole("progressbar").length, 0);
    lepas();
    assert.ok(await screen.findByText("Nurul Fajri"));
    assert.equal(jumlahShimmer(), 0);
  });

  it("tanpa petugas: keterangan kosong", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: amplop(
        muatFixture("koordinator", "aktivitas-petugas-kosong"),
      ),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    assert.ok(await screen.findByText("Belum ada petugas lapangan aktif."));
    assert.equal(screen.queryAllByRole("progressbar").length, 0);
    assert.equal(
      screen.queryByRole("button", { name: /Tampilkan semua/ }),
      null,
    );
  });

  it("Koordinator: satu petugas (kabupatennya saja) tampil seperti biasa", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: amplop(
        muatFixture("koordinator", "aktivitas-petugas"),
      ),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    const baris = (await screen.findByText("Aulia Rahman")).closest("li")!;
    assert.ok(within(baris).getByText("Banda Aceh"));
    assert.equal(screen.getAllByRole("progressbar").length, 1);
  });

  it("galat umum: panel galat dengan tombol coba lagi (bukan keterangan 403, bukan daftar kosong)", async () => {
    let panggilan = 0;
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop(wilayahNyata()),
      [`GET ${PETUGAS}`]: () =>
        ++panggilan === 1
          ? galatApi(500, "Gagal mengambil aktivitas petugas")
          : amplop(petugasNyata()),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Aktivitas per petugas gagal dimuat/);
    assert.match(alert.textContent ?? "", /Gagal mengambil aktivitas petugas/);
    assert.equal(screen.queryByText(/tidak boleh melihat rincian/), null);
    assert.equal(screen.queryByText("Belum ada petugas lapangan aktif."), null);

    await user.click(within(alert).getByRole("button", { name: /Coba lagi/ }));
    assert.ok(await screen.findByText("Nurul Fajri"));
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("403: keterangan peran tidak boleh melihat rincian, tanpa ulang permintaan; per wilayah tetap berfungsi", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop(wilayahNyata()),
      [`GET ${PETUGAS}`]: galatApi(403, "Akses ditolak"),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await screen.findByText("Langsa");
    await bukaPerPetugas(user);

    const status = await screen.findByRole("status");
    assert.match(
      status.textContent ?? "",
      /Peran Anda tidak boleh melihat rincian per petugas/,
    );
    assert.match(status.textContent ?? "", /per wilayah tetap dapat dilihat/);
    assert.equal(
      screen.queryByRole("alert"),
      null,
      "403 bukan galat yang memecah panel",
    );
    assert.equal(screen.queryByRole("button", { name: /Coba lagi/ }), null);
    assert.equal(api.ke("GET", PETUGAS).length, 1, "403 tidak diulang");

    await user.click(screen.getByRole("tab", { name: "Per wilayah" }));
    assert.ok(await screen.findByText("Langsa"));
    assert.equal(screen.getAllByRole("progressbar").length, 10);
    assert.equal(screen.queryByRole("status"), null);
  });

  it("daftar panjang: hanya sebagian yang dirender sampai 'Tampilkan semua', dan bergulir di dalam panel", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: amplop(petugasBanyak(250)),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    await screen.findByText("Petugas 001");
    assert.equal(
      screen.getAllByRole("progressbar").length,
      BATAS_PETUGAS,
      "tidak merender ratusan baris sekaligus",
    );
    const wilayah = screen.getByRole("region", {
      name: "Daftar petugas lapangan",
    });
    assert.match(wilayah.className, /overflow-y-auto/);
    assert.match(wilayah.className, /max-h-/);
    assert.equal(
      wilayah.getAttribute("tabindex"),
      "0",
      "dapat digulir dengan papan ketik",
    );

    await user.click(
      screen.getByRole("button", { name: "Tampilkan semua 250 petugas" }),
    );
    assert.equal(screen.getAllByRole("progressbar").length, 250);
    await user.click(screen.getByRole("button", { name: "Ringkas daftar" }));
    assert.equal(screen.getAllByRole("progressbar").length, BATAS_PETUGAS);
  });

  it("daftar di bawah batas tidak memunculkan tombol; tepat di batas pun tidak", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: amplop(petugasBanyak(BATAS_PETUGAS)),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    await screen.findByText("Petugas 001");
    assert.equal(screen.getAllByRole("progressbar").length, BATAS_PETUGAS);
    assert.equal(
      screen.queryByRole("button", { name: /Tampilkan semua/ }),
      null,
    );
  });

  it("capaian terendah di atas dan diurutkan sebelum dipotong: yang tertinggal selalu tampil di daftar awal", async () => {
    const data = petugasBanyak(60);
    // Petugas 060 (indeks 59) capaian 59 % 15 = 14, tertinggi; yang terendah (0 entri) harus ikut di 20 pertama.
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop([]),
      [`GET ${PETUGAS}`]: amplop(data),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await bukaPerPetugas(user);

    await screen.findByText("Petugas 001");
    const nama = labelBar();
    assert.equal(nama.length, BATAS_PETUGAS);
    const nol = data.petugas.filter((p) => p.entri_hari_ini === 0).length;
    assert.equal(nol, 4, "indeks 0, 15, 30, 45");
    assert.ok(
      nama
        .slice(0, nol)
        .every((n) =>
          [
            "Capaian Petugas 001",
            "Capaian Petugas 016",
            "Capaian Petugas 031",
            "Capaian Petugas 046",
          ].includes(n ?? ""),
        ),
    );
    assert.equal(
      screen.queryByText("Petugas 060"),
      null,
      "yang tertinggi baru tampil setelah 'Tampilkan semua'",
    );
  });

  it("kembali ke per wilayah dan membuka lagi: data tersimpan, sakelar tetap benar", async () => {
    api = pasangFetchPalsu({
      [`GET ${WILAYAH}`]: amplop(wilayahNyata()),
      [`GET ${PETUGAS}`]: amplop(petugasNyata()),
    });
    const { user } = renderHalaman(<FieldActivity />, { url: "/operasional" });
    await screen.findByText("Langsa");
    await bukaPerPetugas(user);
    await screen.findByText("Nurul Fajri");
    await user.click(screen.getByRole("tab", { name: "Per wilayah" }));
    assert.ok(await screen.findByText("Langsa"));
    assert.equal(screen.queryByText("Nurul Fajri"), null);
    await bukaPerPetugas(user);
    assert.ok(await screen.findByText("Nurul Fajri"));
    assert.match(
      screen.getByText(/per petugas, terendah di atas/).textContent ?? "",
      /terendah di atas/,
    );
  });
});
