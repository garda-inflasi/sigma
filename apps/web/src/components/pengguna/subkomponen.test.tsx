import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import type { PenggunaRow } from "@/lib/data/types";
import { drafMatriks } from "@/lib/rbac/draf-matriks";
import {
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  tangkapUnduhan,
  type FetchPalsu,
} from "@/test/render";
import { AksiUndangan } from "./aksi-undangan";
import { IzinLangsungPanel } from "./izin-langsung";
import { MatriksAksesEditor } from "./matriks-akses";
import { ResetMfaDialog } from "./reset-mfa-dialog";
import { RiwayatPeran } from "./riwayat-peran";

const PENGGUNA: PenggunaRow = {
  id: "u-ani",
  nama: "Ani",
  email: "ani@garda.test",
  role: "OPS_MAHASISWA",
  lembaga: "BPPSDMP",
  wilayah: "Seluruh Aceh",
  status: "AKTIF",
  mfa: true,
  terakhirMasuk: "",
  dibuatPada: "2026-10-01T08:00:00Z",
};

let api: FetchPalsu;
afterEach(() => {
  api?.pulihkan();
  drafMatriks.bersihkan();
});

// ── ResetMfaDialog ───────────────────────────────────────────────────

describe("ResetMfaDialog", () => {
  beforeEach(() => {
    api = pasangFetchPalsu({
      "POST /api/pengguna/u-ani/reset-mfa": {
        id: "u-ani",
        mfa_aktif_sebelumnya: true,
        sesi_dicabut: 1,
      },
    });
  });

  it("menjelaskan akibatnya sesuai peran: peran wajib MFA harus mendaftar ulang", () => {
    renderHalaman(
      <ResetMfaDialog
        user={{ ...PENGGUNA, role: "ADMIN_SISTEM" }}
        open
        onOpenChange={() => {}}
      />,
    );
    assert.ok(screen.getByText(/Peran ini wajib MFA/));
    assert.ok(screen.getByText(/semua kode cadangan dihapus/));
  });

  it("peran tanpa MFA wajib: masuk dengan kata sandi saja sampai mendaftar ulang", () => {
    renderHalaman(
      <ResetMfaDialog user={PENGGUNA} open onOpenChange={() => {}} />,
    );
    assert.ok(screen.getByText(/masuk dengan kata sandi saja/));
    assert.equal(screen.queryByText(/Peran ini wajib MFA/), null);
  });

  it("konfirmasi mengirim reset lalu menutup dialog; Batal menutup tanpa mengirim", async () => {
    const ditutup: boolean[] = [];
    const { user } = renderHalaman(
      <ResetMfaDialog
        user={PENGGUNA}
        open
        onOpenChange={(v) => ditutup.push(v)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Batal" }));
    assert.deepEqual(ditutup, [false]);
    assert.equal(api.ke("POST", "/api/pengguna/u-ani/reset-mfa").length, 0);

    await user.click(screen.getByRole("button", { name: "Reset MFA" }));
    await waitFor(() =>
      assert.equal(api.ke("POST", "/api/pengguna/u-ani/reset-mfa").length, 1),
    );
    await waitFor(() => assert.deepEqual(ditutup, [false, false]));
  });

  it("kiriman ganda diabaikan selama menunggu jawaban (tombol nonaktif)", async () => {
    let selesai: (v: unknown) => void = () => {};
    api.atur(
      "POST /api/pengguna/u-ani/reset-mfa",
      () => new Promise((r) => (selesai = r)),
    );
    const { user } = renderHalaman(
      <ResetMfaDialog user={PENGGUNA} open onOpenChange={() => {}} />,
    );

    await user.click(screen.getByRole("button", { name: "Reset MFA" }));
    const tombol = await screen.findByRole("button", { name: "Mereset…" });
    assert.equal((tombol as HTMLButtonElement).disabled, true);
    await user.click(tombol);
    assert.equal(api.ke("POST", "/api/pengguna/u-ani/reset-mfa").length, 1);
    selesai({ id: "u-ani", mfa_aktif_sebelumnya: true, sesi_dicabut: 0 });
  });

  it("server menolak akun sendiri (400): pesannya tampil sebagai alert di dialog", async () => {
    api.atur(
      "POST /api/pengguna/u-ani/reset-mfa",
      galatApi(400, "tidak dapat mereset MFA akun sendiri lewat halaman admin"),
    );
    const { user } = renderHalaman(
      <ResetMfaDialog user={PENGGUNA} open onOpenChange={() => {}} />,
    );

    await user.click(screen.getByRole("button", { name: "Reset MFA" }));

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /akun sendiri/,
    );
  });
});

// ── RiwayatPeran ─────────────────────────────────────────────────────

describe("RiwayatPeran", () => {
  const entri = (o: Record<string, unknown>) => ({
    id: "a-1",
    waktu: "2026-11-03T09:00:00Z",
    aksi: "ASSIGN_ROLE",
    dari: "OPS_AGREGATOR",
    ke: "OPS_MAHASISWA",
    aktor: { id: "admin-1", nama: "Admin Satu" },
    ...o,
  });

  it("riwayat kosong: pesan 'belum ada perubahan', tautan ke audit tetap ada", async () => {
    api = pasangFetchPalsu({
      "GET /api/pengguna/u-ani/riwayat-peran": { items: [], terpotong: false },
    });
    renderHalaman(<RiwayatPeran userId="u-ani" />);

    assert.ok(
      await screen.findByText(/Belum ada perubahan peran yang tercatat/),
    );
    assert.equal(
      screen
        .getByRole("link", { name: "Lihat di audit trail" })
        .getAttribute("href"),
      "/audit?entitas=pengguna&entitas_id=u-ani",
    );
  });

  it("entri: judul aksi, peran dari→ke dengan label, pelaku (akun dihapus / sistem)", async () => {
    api = pasangFetchPalsu({
      "GET /api/pengguna/u-ani/riwayat-peran": {
        items: [
          entri({ id: "a-3", aktor: { id: null, nama: "Mantan Admin" } }),
          entri({
            id: "a-2",
            aksi: "UPDATE_USER",
            dari: null,
            ke: "OPS_AGREGATOR",
            aktor: null,
          }),
          entri({
            id: "a-1",
            aksi: "CREATE_USER",
            dari: null,
            ke: "OPS_AGREGATOR",
            aktor: null,
          }),
        ],
        terpotong: true,
      },
    });
    renderHalaman(<RiwayatPeran userId="u-ani" />);

    const daftar = await screen.findByRole("list", { name: "Riwayat peran" });
    const item = within(daftar)
      .getAllByRole("listitem")
      .map((e) => e.textContent ?? "");
    assert.match(item[0], /Peran diganti/);
    assert.match(item[0], /Mantan Admin \(akun dihapus\)/);
    assert.match(item[1], /Peran awal ditetapkan/);
    assert.match(item[1], /oleh Sistem/);
    assert.match(item[2], /Akun dibuat/);
    assert.ok(
      screen.getByText(/Menampilkan 3 perubahan terbaru/),
      "tanda terpotong",
    );
  });

  it("galat server tampil sebagai alert, bukan daftar kosong", async () => {
    api = pasangFetchPalsu({
      "GET /api/pengguna/u-ani/riwayat-peran": galatApi(
        404,
        "Pengguna tidak ditemukan",
      ),
    });
    renderHalaman(<RiwayatPeran userId="u-ani" />);

    assert.match(
      (await screen.findByRole("alert")).textContent ?? "",
      /Pengguna tidak ditemukan/,
    );
    assert.equal(screen.queryByText(/Belum ada perubahan peran/), null);
  });
});

// ── IzinLangsungPanel ────────────────────────────────────────────────

describe("IzinLangsungPanel", () => {
  const langsung = (o: Record<string, unknown>) => ({
    izin: "harga.ekspor",
    deskripsi: "Ekspor harga",
    kelompok: "Harga",
    berlaku_sampai: null,
    alasan: "Audit triwulan",
    diberikan_oleh: "admin-1",
    diberikan_oleh_nama: "Admin Satu",
    diberikan_pada: "2026-11-01T08:00:00Z",
    aktif: true,
    ...o,
  });
  const matriks = [
    {
      id: "pengguna.kelola",
      deskripsi: "Kelola pengguna",
      kelompok: "Pengguna",
    },
    { id: "harga.lihat", deskripsi: "Lihat harga", kelompok: "Harga" },
    { id: "harga.ekspor", deskripsi: "Ekspor harga", kelompok: "Harga" },
    { id: "harga.validasi", deskripsi: "Validasi harga", kelompok: "Harga" },
  ];

  function pasang(
    izin: Record<string, unknown>,
    tambahan: Record<string, unknown> = {},
  ) {
    api = pasangFetchPalsu({
      "GET /api/pengguna/u-ani/permissions": izin,
      "GET /api/rbac/roles": [
        {
          id: "OPS_MAHASISWA",
          label: "Mahasiswa",
          lapis: "OPERASIONAL",
          permissions: ["harga.lihat"],
          bawaan: ["harga.lihat"],
          versi: 1,
        },
      ],
      "GET /api/rbac/permissions": matriks,
      ...tambahan,
    });
    return renderHalaman(
      <Dialog open>
        <DialogContent>
          <IzinLangsungPanel user={PENGGUNA} onTutup={() => {}} />
        </DialogContent>
      </Dialog>,
    );
  }

  it("daftar izin langsung menampilkan status, masa berlaku, alasan, dan pemberi; izin peran terpisah", async () => {
    pasang({
      peran: "OPS_MAHASISWA",
      izin_peran: ["harga.lihat"],
      efektif: ["harga.ekspor", "harga.lihat"],
      langsung: [
        langsung({ berlaku_sampai: "2099-12-31T16:59:59Z" }),
        langsung({
          izin: "harga.validasi",
          deskripsi: "Validasi harga",
          aktif: false,
          berlaku_sampai: "2026-10-01T00:00:00Z",
          alasan: "Sudah selesai",
        }),
      ],
    });

    const daftar = await screen.findByRole("list", { name: "Izin langsung" });
    const [aktif, lewat] = within(daftar).getAllByRole("listitem");
    assert.match(aktif.textContent ?? "", /harga\.ekspor/);
    assert.match(aktif.textContent ?? "", /Aktif/);
    assert.match(aktif.textContent ?? "", /Berlaku sampai/);
    assert.match(aktif.textContent ?? "", /Audit triwulan/);
    assert.match(aktif.textContent ?? "", /oleh Admin Satu/);
    assert.match(lewat.textContent ?? "", /Kedaluwarsa/);
    assert.match(lewat.textContent ?? "", /Berakhir/);
    assert.ok(screen.getByText("Izin dari peran (1)"));
    assert.ok(
      within(screen.getByRole("list", { name: "Izin dari peran" })).getByText(
        "harga.lihat",
      ),
    );
  });

  it("izin tanpa batas waktu ditulis 'Tanpa batas waktu'", async () => {
    pasang({
      peran: "OPS_MAHASISWA",
      izin_peran: [],
      efektif: ["harga.ekspor"],
      langsung: [langsung({})],
    });
    assert.ok(await screen.findByText("Tanpa batas waktu"));
  });

  it("mencabut izin memanggil DELETE untuk izin itu lalu memuat ulang daftar", async () => {
    let dicabut = false;
    const { user } = pasang(
      { peran: "OPS_MAHASISWA", izin_peran: [], efektif: [], langsung: [] },
      {
        "GET /api/pengguna/u-ani/permissions": () => ({
          peran: "OPS_MAHASISWA",
          izin_peran: [],
          efektif: [],
          langsung: dicabut ? [] : [langsung({})],
        }),
        "DELETE /api/pengguna/u-ani/permissions/harga.ekspor": () => {
          dicabut = true;
          return { id: "u-ani", izin: "harga.ekspor" };
        },
      },
    );
    await user.click(
      await screen.findByRole("button", { name: "Cabut izin harga.ekspor" }),
    );

    await waitFor(() =>
      assert.equal(
        api.ke("DELETE", "/api/pengguna/u-ani/permissions/harga.ekspor").length,
        1,
      ),
    );
    assert.ok(
      await screen.findByText("Belum ada izin langsung untuk pengguna ini."),
    );
  });

  it("formulir: alasan wajib dan tanggal lampau ditolak di klien tanpa memanggil server", async () => {
    const { user } = pasang({
      peran: "OPS_MAHASISWA",
      izin_peran: ["harga.lihat"],
      efektif: ["harga.lihat"],
      langsung: [],
    });
    const form = await screen.findByRole("form", {
      name: "Beri izin langsung",
    });

    await user.click(
      within(form).getByRole("button", { name: "Berikan izin" }),
    );

    assert.ok(await within(form).findByText("Pilih izin yang akan diberikan."));
    assert.ok(within(form).getByText("Alasan wajib diisi."));
    assert.equal(api.ke("POST", "/api/pengguna/u-ani/permissions").length, 0);
    assert.equal(
      within(form).getByLabelText("Alasan").getAttribute("aria-invalid"),
      "true",
    );
  });

  it("pemilih hanya menawarkan izin yang belum dimiliki (peran maupun langsung aktif)", async () => {
    const { user } = pasang({
      peran: "OPS_MAHASISWA",
      izin_peran: ["harga.lihat"],
      efektif: ["harga.ekspor", "harga.lihat"],
      langsung: [langsung({})],
    });
    const form = await screen.findByRole("form", {
      name: "Beri izin langsung",
    });

    await user.click(within(form).getByRole("combobox"));

    const opsi = (await screen.findAllByRole("option")).map(
      (o) => o.textContent ?? "",
    );
    assert.equal(opsi.length, 2);
    assert.ok(opsi.some((t) => t.startsWith("pengguna.kelola")));
    assert.ok(opsi.some((t) => t.startsWith("harga.validasi")));
    assert.ok(
      !opsi.some((t) => t.startsWith("harga.lihat")),
      "sudah dari peran",
    );
    assert.ok(
      !opsi.some((t) => t.startsWith("harga.ekspor")),
      "sudah langsung dan aktif",
    );
  });

  it("memberi izin mengirim izin dan alasan terpangkas (tanpa tanggal = tanpa batas); formulir dikosongkan", async () => {
    const { user } = pasang(
      {
        peran: "OPS_MAHASISWA",
        izin_peran: ["harga.lihat"],
        efektif: ["harga.lihat"],
        langsung: [],
      },
      {
        "POST /api/pengguna/u-ani/permissions": () =>
          langsung({ izin: "harga.validasi", deskripsi: "Validasi harga" }),
      },
    );
    const form = await screen.findByRole("form", {
      name: "Beri izin langsung",
    });

    await user.click(within(form).getByRole("combobox"));
    await user.click(
      await screen.findByRole("option", { name: /harga\.validasi/ }),
    );
    await user.type(
      within(form).getByLabelText("Alasan"),
      "  Tugas validasi  ",
    );
    await user.click(
      within(form).getByRole("button", { name: "Berikan izin" }),
    );

    await waitFor(() =>
      assert.equal(api.ke("POST", "/api/pengguna/u-ani/permissions").length, 1),
    );
    assert.deepEqual(
      api.ke("POST", "/api/pengguna/u-ani/permissions")[0].body,
      { izin: "harga.validasi", alasan: "Tugas validasi" },
    );
    await waitFor(() =>
      assert.equal(
        (within(form).getByLabelText("Alasan") as HTMLTextAreaElement).value,
        "",
      ),
    );
  });

  it("konflik 409 dari server tampil di formulir dan isian dipertahankan", async () => {
    const { user } = pasang(
      {
        peran: "OPS_MAHASISWA",
        izin_peran: ["harga.lihat"],
        efektif: ["harga.lihat"],
        langsung: [],
      },
      {
        "POST /api/pengguna/u-ani/permissions": galatApi(
          409,
          "Pengguna sudah memiliki izin ini lewat perannya.",
        ),
      },
    );
    const form = await screen.findByRole("form", {
      name: "Beri izin langsung",
    });
    await user.click(within(form).getByRole("combobox"));
    await user.click(
      await screen.findByRole("option", { name: /harga\.validasi/ }),
    );
    await user.type(within(form).getByLabelText("Alasan"), "Uji");
    await user.click(
      within(form).getByRole("button", { name: "Berikan izin" }),
    );

    assert.match(
      (await within(form).findByRole("alert")).textContent ?? "",
      /sudah memiliki izin ini/,
    );
    assert.equal(
      (within(form).getByLabelText("Alasan") as HTMLTextAreaElement).value,
      "Uji",
    );
  });

  it("gagal memuat izin pengguna: pesan dengan petunjuk, tanpa formulir", async () => {
    const { user } = pasang(
      {},
      { "GET /api/pengguna/u-ani/permissions": galatApi(500, "rusak") },
    );
    assert.ok(await screen.findByText(/Izin pengguna gagal dimuat/));
    assert.equal(
      screen.queryByRole("form", { name: "Beri izin langsung" }),
      null,
    );
  });
});

// ── MatriksAksesEditor ───────────────────────────────────────────────

describe("MatriksAksesEditor", () => {
  const peran = (versi: number, izin: string[], bawaan = ["harga.lihat"]) => [
    {
      id: "OPS_MAHASISWA",
      label: "Mahasiswa",
      lapis: "OPERASIONAL",
      permissions: izin,
      bawaan,
      versi,
    },
  ];
  const izinKatalog = [
    { id: "harga.lihat", deskripsi: "Lihat harga", kelompok: "Harga" },
    { id: "harga.ekspor", deskripsi: "Ekspor harga", kelompok: "Harga" },
  ];

  it("gagal memuat: pesan dengan tombol 'Muat ulang' yang benar-benar memuat ulang", async () => {
    let gagal = true;
    api = pasangFetchPalsu({
      "GET /api/rbac/roles": () =>
        gagal ? galatApi(500, "rusak") : peran(1, ["harga.lihat"]),
      "GET /api/rbac/permissions": izinKatalog,
    });
    const { user } = renderHalaman(<MatriksAksesEditor />);

    assert.ok(await screen.findByText("Matriks akses gagal dimuat."));
    gagal = false;
    await user.click(screen.getByRole("button", { name: "Muat ulang" }));

    assert.ok(
      await screen.findByRole("table", {
        name: "Matriks akses peran dan izin",
      }),
    );
  });

  it("izin yang berbeda dari bawaan ditandai; 'Pulihkan bawaan' menjadikannya draf", async () => {
    api = pasangFetchPalsu({
      "GET /api/rbac/roles": peran(2, ["harga.lihat", "harga.ekspor"]),
      "GET /api/rbac/permissions": izinKatalog,
    });
    const { user } = renderHalaman(<MatriksAksesEditor />);
    await screen.findByRole("table", { name: "Matriks akses peran dan izin" });

    await user.click(
      screen.getByRole("button", { name: "Pulihkan bawaan Mahasiswa" }),
    );

    assert.ok(screen.getByText("1 peran belum disimpan"));
    assert.equal(
      screen
        .getByRole("button", { name: "harga.ekspor untuk Mahasiswa" })
        .getAttribute("aria-pressed"),
      "false",
    );
    assert.ok(screen.getByText(/Belum disimpan · \+0 −1/));
  });

  it("konflik versi (admin lain menyimpan lebih dulu) membuang draf dan memuat ulang", async () => {
    let versi = 1;
    api = pasangFetchPalsu({
      "GET /api/rbac/roles": () => peran(versi, ["harga.lihat"]),
      "GET /api/rbac/permissions": izinKatalog,
      "PUT /api/rbac/roles/OPS_MAHASISWA/permissions": () => {
        versi = 2;
        return {
          __status: 409,
          body: {
            message: "Matriks peran sudah diubah admin lain.",
            kode: "ROLE_VERSION_CONFLICT",
          },
        };
      },
    });
    const { user } = renderHalaman(<MatriksAksesEditor />);
    await screen.findByRole("table", { name: "Matriks akses peran dan izin" });

    await user.click(
      screen.getByRole("button", { name: "harga.ekspor untuk Mahasiswa" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Simpan matriks Mahasiswa" }),
    );

    await waitFor(() =>
      assert.equal(screen.queryByText(/belum disimpan/), null),
    );
    await waitFor(() => assert.ok(screen.getByText("v2")), { timeout: 2000 });
    assert.equal(
      screen
        .getByRole("button", { name: "harga.ekspor untuk Mahasiswa" })
        .getAttribute("aria-pressed"),
      "false",
    );
  });

  it("draf yang versi dasarnya tertinggal dibuang saat matriks dimuat dengan versi baru", async () => {
    drafMatriks.atur("OPS_MAHASISWA", {
      versi: 1,
      izin: new Set(["harga.lihat", "harga.ekspor"]),
    });
    api = pasangFetchPalsu({
      "GET /api/rbac/roles": peran(5, ["harga.lihat"]),
      "GET /api/rbac/permissions": izinKatalog,
    });
    renderHalaman(<MatriksAksesEditor />);
    await screen.findByRole("table", { name: "Matriks akses peran dan izin" });

    await waitFor(() => assert.equal(drafMatriks.ambil().size, 0));
    assert.equal(screen.queryByText(/belum disimpan/), null);
  });

  it("draf bertahan melewati unmount (pindah tab) dan memasang peringatan sebelum halaman ditutup", async () => {
    api = pasangFetchPalsu({
      "GET /api/rbac/roles": peran(1, ["harga.lihat"]),
      "GET /api/rbac/permissions": izinKatalog,
    });
    const { user, unmount } = renderHalaman(<MatriksAksesEditor />);
    await screen.findByRole("table", { name: "Matriks akses peran dan izin" });
    await user.click(
      screen.getByRole("button", { name: "harga.ekspor untuk Mahasiswa" }),
    );

    const acara = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(acara);
    assert.equal(
      acara.defaultPrevented,
      true,
      "penutupan halaman diperingatkan selama ada draf",
    );

    unmount();
    assert.equal(drafMatriks.ambil().size, 1);
    const sesudah = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(sesudah);
    assert.equal(
      sesudah.defaultPrevented,
      false,
      "pendengar dilepas saat komponen dilepas",
    );
  });
});

// ── AksiUndangan ─────────────────────────────────────────────────────

describe("AksiUndangan", () => {
  let unduhan: ReturnType<typeof tangkapUnduhan>;
  beforeEach(() => {
    unduhan = tangkapUnduhan();
    api = pasangFetchPalsu({
      "GET /api/pengguna/ekspor": () =>
        new Response("id;nama", {
          status: 200,
          headers: {
            "Content-Type": "text/csv",
            "Content-Disposition": 'attachment; filename="pengguna-x.csv"',
            "X-Export-Rows": "1",
          },
        }),
    });
  });
  afterEach(() => unduhan.pulihkan());

  it("tiga aksi toolbar; label ekspor mengikuti ada tidaknya filter", () => {
    const { rerender } = renderHalaman(<AksiUndangan />);
    assert.ok(screen.getByRole("button", { name: /Undang pengguna/ }));
    assert.ok(screen.getByRole("button", { name: /Impor CSV/ }));
    assert.ok(screen.getByRole("button", { name: "Ekspor semua (CSV)" }));
    rerender(<AksiUndangan filter={{ status: "AKTIF" }} />);
    assert.ok(
      screen.getByRole("button", { name: "Ekspor hasil filter (CSV)" }),
    );
    rerender(<AksiUndangan filter={{ search: "   " }} />);
    assert.ok(
      screen.getByRole("button", { name: "Ekspor semua (CSV)" }),
      "pencarian spasi saja bukan filter",
    );
  });

  it("'Buat dengan kata sandi sementara' menutup dialog undangan dan memanggil onBuatSementara", async () => {
    let dibuka = 0;
    const { user } = renderHalaman(
      <AksiUndangan onBuatSementara={() => dibuka++} />,
    );

    await user.click(screen.getByRole("button", { name: /Undang pengguna/ }));
    const dialog = await screen.findByRole("dialog");
    await user.click(
      within(dialog).getByRole("button", { name: /kata sandi sementara/i }),
    );

    assert.equal(dibuka, 1);
    await waitFor(() => assert.equal(screen.queryByRole("dialog"), null));
  });

  it("ekspor menyimpan berkas dengan nama dari server", async () => {
    const { user } = renderHalaman(
      <AksiUndangan filter={{ peran: "ADMIN_SISTEM" }} />,
    );

    await user.click(
      screen.getByRole("button", { name: "Ekspor hasil filter (CSV)" }),
    );

    await waitFor(() => assert.equal(unduhan.daftar.length, 1));
    assert.equal(unduhan.daftar[0].download, "pengguna-x.csv");
    assert.equal(
      api.ke("GET", "/api/pengguna/ekspor")[0].query.get("peran"),
      "ADMIN_SISTEM",
    );
  });
});

