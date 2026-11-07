import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import type { MasterKomoditasRow, MasterPasarRow } from "@/lib/data/types";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { UsulanDialog, type TargetUsulan } from "./usulan-dialog";

const komoditas: MasterKomoditasRow = {
  id: "BWM",
  kode: "BWM",
  nama: "Bawang merah",
  kategori: "Hortikultura",
  satuan: "kg",
  hargaAcuan: 41000,
  pilot: true,
  shelfLifeHari: 60,
  versi: 2,
  status: "AKTIF",
};

const pasar: MasterPasarRow = {
  id: "PSR-1171-01",
  nama: "Pasar Peunayong",
  kabupaten: "Kota Banda Aceh",
  kabupatenKode: "1171",
  lat: 5.5603,
  lng: 95.3237,
  radiusMeter: 400,
  aktif: true,
  kecamatanKode: "1171060",
  kecamatanNama: "Kuta Alam",
  versi: 1,
};

const SATUAN = [
  { kode: "kg", nama: "Kilogram", jumlah_komoditas: 9 },
  { kode: "liter", nama: "Liter", jumlah_komoditas: 1 },
];

const KECAMATAN = [
  {
    kode: "1171040",
    nama: "Baiturrahman",
    kabupaten_kode: "1171",
    kabupaten_nama: "KOTA BANDA ACEH",
  },
  {
    kode: "1171060",
    nama: "Kuta Alam",
    kabupaten_kode: "1171",
    kabupaten_nama: "KOTA BANDA ACEH",
  },
];

const POST = "POST /v1/master/perubahan";
const kirimanPost = () => api.ke("POST", "/v1/master/perubahan");

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

function pasangApi(
  post: unknown = (req: { body: unknown }) =>
    amplop({ id: "55", ...(req.body as object), status: "MENUNGGU" }, "201"),
) {
  api = pasangFetchPalsu({
    "GET /v1/master/satuan": amplop(SATUAN),
    "GET /v1/master/kecamatan": amplop(KECAMATAN),
    [POST]: post,
  });
}

function tampilkan(target: TargetUsulan) {
  const ditutup: boolean[] = [];
  const r = renderHalaman(
    <UsulanDialog target={target} open onOpenChange={(v) => ditutup.push(v)} />,
  );
  return { ...r, ditutup };
}

const input = (label: string) =>
  screen.getByLabelText(label) as HTMLInputElement;
const kirim = (user: ReturnType<typeof renderHalaman>["user"]) =>
  user.click(screen.getByRole("button", { name: "Ajukan perubahan" }));

describe("UsulanDialog komoditas (ubah)", () => {
  it("bidang terisi dari data sekarang", async () => {
    pasangApi();
    tampilkan({ entitas: "komoditas", awal: komoditas });

    await screen.findByRole("dialog");
    assert.equal(input("Nama").value, "Bawang merah");
    assert.equal(input("Kategori").value, "Hortikultura");
    assert.equal(input("Harga acuan (Rp)").value, "41000");
    assert.equal(input("Umur simpan (hari)").value, "60");
    await waitFor(() =>
      assert.match(
        screen.getByRole("combobox", { name: "Satuan" }).textContent ?? "",
        /Kilogram \(kg\)/,
      ),
    );
  });

  it("tanpa perubahan tidak dikirim: pesan 'Tidak ada perubahan'", async () => {
    pasangApi();
    const { user } = tampilkan({ entitas: "komoditas", awal: komoditas });
    await screen.findByRole("dialog");

    await user.type(screen.getByLabelText("Alasan pengajuan"), "Hanya mencoba");
    await kirim(user);

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /Tidak ada perubahan/,
    );
    assert.equal(kirimanPost().length, 0);
  });

  it("alasan wajib: galat di bidang alasan dan tidak dikirim", async () => {
    pasangApi();
    const { user } = tampilkan({ entitas: "komoditas", awal: komoditas });
    await screen.findByRole("dialog");

    await user.clear(input("Harga acuan (Rp)"));
    await user.type(input("Harga acuan (Rp)"), "43500");
    await kirim(user);

    assert.match(
      (await screen.findByText(/Alasan wajib diisi, minimal 3 karakter/))
        .textContent ?? "",
      /Alasan/,
    );
    assert.equal(kirimanPost().length, 0);
  });

  it("aturan bidang mencerminkan API: harga harus bilangan bulat lebih dari 0, umur simpan 1–3650", async () => {
    pasangApi();
    const { user } = tampilkan({ entitas: "komoditas", awal: komoditas });
    await screen.findByRole("dialog");

    await user.clear(input("Harga acuan (Rp)"));
    await user.type(input("Harga acuan (Rp)"), "0");
    await user.clear(input("Umur simpan (hari)"));
    await user.type(input("Umur simpan (hari)"), "4000");
    await user.clear(input("Nama"));
    await user.type(screen.getByLabelText("Alasan pengajuan"), "Uji aturan");
    await kirim(user);

    assert.ok(
      await screen.findByText(/Harga acuan harus bilangan bulat lebih dari 0/),
    );
    assert.ok(screen.getByText(/Umur simpan 1–3650 hari/));
    assert.ok(screen.getByText(/Nama wajib diisi/));
    assert.equal(kirimanPost().length, 0);
  });

  it("hanya bidang yang berubah dikirim, lengkap dengan entitas, jenis, ID, dan alasan terpangkas", async () => {
    pasangApi();
    const { user, ditutup } = tampilkan({
      entitas: "komoditas",
      awal: komoditas,
    });
    await screen.findByRole("dialog");

    await user.clear(input("Harga acuan (Rp)"));
    await user.type(input("Harga acuan (Rp)"), "43.500");
    await user.type(
      screen.getByLabelText("Alasan pengajuan"),
      "  Naik mengikuti rilis BPS  ",
    );
    await kirim(user);

    await waitFor(() => assert.equal(kirimanPost().length, 1));
    assert.deepEqual(kirimanPost()[0].body, {
      entitas: "komoditas",
      jenis: "UBAH",
      entitas_id: "BWM",
      perubahan: { harga_acuan: 43500 },
      alasan: "Naik mengikuti rilis BPS",
    });
    await waitFor(() => assert.deepEqual(ditutup, [false]));
  });

  it("galat server 409 USULAN_MENUNGGU tampil di dialog dan dialog tetap terbuka", async () => {
    pasangApi(
      galatApi(
        409,
        "Sudah ada usulan yang menunggu keputusan untuk data ini",
        "USULAN_MENUNGGU",
      ),
    );
    const { user, ditutup } = tampilkan({
      entitas: "komoditas",
      awal: komoditas,
    });
    await screen.findByRole("dialog");

    await user.clear(input("Harga acuan (Rp)"));
    await user.type(input("Harga acuan (Rp)"), "43500");
    await user.type(
      screen.getByLabelText("Alasan pengajuan"),
      "Naik mengikuti BPS",
    );
    await kirim(user);

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /Sudah ada usulan yang menunggu/,
    );
    assert.ok(screen.getByRole("dialog"));
    assert.deepEqual(ditutup, []);
  });

  it("galat 400 per bidang dari server tampil di bidangnya", async () => {
    pasangApi({
      __status: 400,
      body: {
        code: "400",
        message: "data tidak valid",
        data: { errors: { harga_acuan: "harga di luar batas wajar" } },
      },
    });
    const { user } = tampilkan({ entitas: "komoditas", awal: komoditas });
    await screen.findByRole("dialog");

    await user.clear(input("Harga acuan (Rp)"));
    await user.type(input("Harga acuan (Rp)"), "43500");
    await user.type(
      screen.getByLabelText("Alasan pengajuan"),
      "Naik mengikuti BPS",
    );
    await kirim(user);

    assert.ok(await screen.findByText("harga di luar batas wajar"));
  });
});

describe("UsulanDialog komoditas (tambah)", () => {
  it("semua bidang wajib, termasuk kode, sebelum dikirim", async () => {
    pasangApi();
    const { user } = tampilkan({ entitas: "komoditas" });
    await screen.findByRole("dialog");

    await user.click(screen.getByRole("button", { name: "Ajukan data baru" }));

    for (const pesan of [
      /Kode 2–30 karakter/,
      /Nama wajib diisi/,
      /Kategori wajib diisi/,
      /^Pilih satuan\.$/,
      /Harga acuan harus bilangan bulat/,
      /Umur simpan 1–3650/,
      /Alasan wajib diisi/,
    ]) {
      assert.ok(await screen.findByText(pesan), String(pesan));
    }
    assert.equal(kirimanPost().length, 0);
  });

  it("mengirim keenam bidang dengan kode sebagai entitas_id (kode otomatis kapital)", async () => {
    pasangApi();
    const { user, ditutup } = tampilkan({ entitas: "komoditas" });
    await screen.findByRole("dialog");

    await user.type(input("Kode"), "kdl-hju");
    assert.equal(input("Kode").value, "KDL-HJU");
    await user.type(input("Nama"), "Kedelai hijau");
    await user.type(input("Kategori"), "Pangan pokok");
    await user.click(screen.getByRole("combobox", { name: "Satuan" }));
    await user.click(
      await screen.findByRole("option", { name: "Kilogram (kg)" }),
    );
    await user.type(input("Harga acuan (Rp)"), "14000");
    await user.type(input("Umur simpan (hari)"), "180");
    await user.click(screen.getByRole("checkbox", { name: "Komoditas pilot" }));
    await user.type(
      screen.getByLabelText("Alasan pengajuan"),
      "Komoditas baru dipantau",
    );
    await user.click(screen.getByRole("button", { name: "Ajukan data baru" }));

    await waitFor(() => assert.equal(kirimanPost().length, 1));
    assert.deepEqual(kirimanPost()[0].body, {
      entitas: "komoditas",
      jenis: "TAMBAH",
      entitas_id: "KDL-HJU",
      perubahan: {
        nama: "Kedelai hijau",
        kategori: "Pangan pokok",
        satuan: "kg",
        harga_acuan: 14000,
        shelf_life_hari: 180,
        pilot: true,
      },
      alasan: "Komoditas baru dipantau",
    });
    await waitFor(() => assert.deepEqual(ditutup, [false]));
  });
});

describe("UsulanDialog pasar", () => {
  it("ubah: kecamatan dikosongkan dikirim sebagai null dan hanya bidang itu yang dikirim", async () => {
    pasangApi();
    const { user } = tampilkan({ entitas: "pasar", awal: pasar });
    await screen.findByRole("dialog");
    await waitFor(() =>
      assert.match(
        screen.getByRole("combobox", { name: "Kecamatan" }).textContent ?? "",
        /Kuta Alam/,
      ),
    );
    assert.equal(
      api.ke("GET", "/v1/master/kecamatan")[0].query.get("kabupaten_kode"),
      "1171",
      "kecamatan disaring menurut kabupaten pasar",
    );

    await user.click(screen.getByRole("combobox", { name: "Kecamatan" }));
    await user.click(
      await screen.findByRole("option", { name: "Tanpa kecamatan" }),
    );
    await user.type(
      screen.getByLabelText("Alasan pengajuan"),
      "Kecamatan belum pasti",
    );
    await kirim(user);

    await waitFor(() => assert.equal(kirimanPost().length, 1));
    assert.deepEqual(kirimanPost()[0].body, {
      entitas: "pasar",
      jenis: "UBAH",
      entitas_id: "PSR-1171-01",
      perubahan: { kecamatan_kode: null },
      alasan: "Kecamatan belum pasti",
    });
  });

  it("ubah: lintang di luar rentang ditolak di klien", async () => {
    pasangApi();
    const { user } = tampilkan({ entitas: "pasar", awal: pasar });
    await screen.findByRole("dialog");

    await user.clear(input("Lintang"));
    await user.type(input("Lintang"), "95");
    await user.type(
      screen.getByLabelText("Alasan pengajuan"),
      "Koreksi koordinat",
    );
    await kirim(user);

    assert.ok(await screen.findByText(/Lintang harus antara -90 dan 90/));
    assert.equal(kirimanPost().length, 0);
  });

  it("tambah: memilih kabupaten mengisi awalan ID dan memuat kecamatan kabupaten itu", async () => {
    api = pasangFetchPalsu({
      "GET /v1/master/wilayah": amplop([
        {
          kode: "1171",
          nama: "KOTA BANDA ACEH",
          jenis: "Kota",
          ibukota: "Banda Aceh",
          lat: 5.55,
          lng: 95.32,
          provinsi_kode: "11",
        },
      ]),
      "GET /v1/master/kecamatan": amplop(KECAMATAN),
      [POST]: amplop({ id: "56" }, "201"),
    });
    const { user } = tampilkan({ entitas: "pasar" });
    await screen.findByRole("dialog");

    await user.click(screen.getByRole("combobox", { name: "Kabupaten/kota" }));
    await user.click(
      await screen.findByRole("option", { name: "Kota Banda Aceh" }),
    );

    assert.equal(input("ID pasar").value, "PSR-1171-");
    await waitFor(() =>
      assert.equal(
        api.ke("GET", "/v1/master/kecamatan")[0]?.query.get("kabupaten_kode"),
        "1171",
      ),
    );

    await user.type(input("ID pasar"), "05");
    await user.type(input("Nama"), "Pasar Baru");
    await user.type(input("Lintang"), "5.55");
    await user.type(input("Bujur"), "95.32");
    await user.type(
      screen.getByLabelText("Alasan pengajuan"),
      "Pasar baru dibuka",
    );
    await user.click(screen.getByRole("button", { name: "Ajukan data baru" }));

    await waitFor(() => assert.equal(kirimanPost().length, 1));
    assert.deepEqual(kirimanPost()[0].body, {
      entitas: "pasar",
      jenis: "TAMBAH",
      entitas_id: "PSR-1171-05",
      perubahan: {
        nama: "Pasar Baru",
        kabupaten_kode: "1171",
        lat: 5.55,
        lng: 95.32,
        radius_meter: 400,
      },
      alasan: "Pasar baru dibuka",
    });
  });
});

describe("UsulanDialog tanpa sasaran", () => {
  it("tidak menampilkan dialog", () => {
    pasangApi();
    renderHalaman(<UsulanDialog target={null} open onOpenChange={() => {}} />);
    assert.equal(within(document.body).queryByRole("dialog"), null);
  });
});
