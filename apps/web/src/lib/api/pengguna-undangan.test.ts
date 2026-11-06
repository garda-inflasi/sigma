import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { ApiError } from "./client";
import {
  MAKS_ID_EKSPOR,
  namaBerkasEkspor,
  eksporBerfilter,
  queryEksporPengguna,
  toHasilUndangan,
  undanganApi,
  unduhEksporPengguna,
  type UndanganWire,
} from "./pengguna-undangan";

const fetchAsli = globalThis.fetch;
const documentAsli = (globalThis as { document?: unknown }).document;
afterEach(() => {
  globalThis.fetch = fetchAsli;
  (globalThis as { document?: unknown }).document = documentAsli;
});

interface Panggilan {
  url: string;
  init: RequestInit;
}

function fetchMenjawab(buat: () => Response): Panggilan[] {
  const panggilan: Panggilan[] = [];
  globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
    panggilan.push({ url: String(url), init: init ?? {} });
    return buat();
  }) as typeof fetch;
  return panggilan;
}

const undanganWire: UndanganWire = {
  user: {
    id: "ef45e253-50b5-3808-9710-90ed1484f293",
    nama: "Budi",
    email: "budi@garda.test",
    peran: "OPS_MAHASISWA",
    lapis: "OPERASIONAL",
    lembaga: "FMIPA",
    kabupaten_kode: "1171",
    mfa_aktif: false,
    status: "MENUNGGU",
    dibuat_pada: "2026-11-06T00:00:00Z",
  },
  berlaku_sampai: "2026-11-09T00:00:00Z",
  email_terkirim: true,
};

describe("toHasilUndangan", () => {
  it("memetakan akun menjadi baris tabel berstatus MENUNGGU beserta masa berlaku dan status email", () => {
    const h = toHasilUndangan(undanganWire);
    assert.equal(h.pengguna.id, "ef45e253-50b5-3808-9710-90ed1484f293");
    assert.equal(h.pengguna.status, "MENUNGGU");
    assert.equal(h.pengguna.role, "OPS_MAHASISWA");
    assert.equal(h.berlakuSampai, "2026-11-09T00:00:00Z");
    assert.equal(h.emailTerkirim, true);
    assert.equal(
      toHasilUndangan({ ...undanganWire, email_terkirim: false }).emailTerkirim,
      false,
    );
  });
});

describe("queryEksporPengguna", () => {
  it("meneruskan filter daftar (peran, status, lapis, mfa, terkunci) dalam urutan tetap", () => {
    assert.equal(
      queryEksporPengguna({
        search: "ani",
        terkunci: "false",
        peran: "ADMIN_SISTEM",
        mfa: "true",
        status: "AKTIF",
        lapis: "SISTEM",
        ids: ["u1"],
      }),
      "?search=ani&peran=ADMIN_SISTEM&status=AKTIF&lapis=SISTEM&mfa=true&terkunci=false&ids=u1",
    );
    assert.equal(queryEksporPengguna({ peran: undefined, mfa: undefined }), "");
  });

  it("eksporBerfilter: hanya bila pencarian atau filter terisi", () => {
    assert.equal(eksporBerfilter(), false);
    assert.equal(eksporBerfilter({ search: "  " }), false);
    assert.equal(eksporBerfilter({ ids: ["a"] }), false);
    assert.equal(eksporBerfilter({ search: "ani" }), true);
    assert.equal(eksporBerfilter({ mfa: "false" }), true);
    assert.equal(eksporBerfilter({ status: "MENUNGGU" }), true);
  });

  it("tanpa filter: kosong (semua pengguna)", () => {
    assert.equal(queryEksporPengguna(), "");
    assert.equal(queryEksporPengguna({}), "");
    assert.equal(queryEksporPengguna({ search: "  ", ids: [] }), "");
  });

  it("pencarian dirapikan, id digabung dengan koma dan ter-encode", () => {
    assert.equal(
      queryEksporPengguna({ search: " ani rahma " }),
      "?search=ani+rahma",
    );
    assert.equal(
      queryEksporPengguna({ ids: ["a", "b", "c"] }),
      "?ids=a%2Cb%2Cc",
    );
    assert.equal(
      queryEksporPengguna({ search: "x", ids: ["u1"] }),
      "?search=x&ids=u1",
    );
  });
});

describe("namaBerkasEkspor", () => {
  it("memakai nama dari Content-Disposition server; bawaan bertanggal bila tidak ada atau tidak sah", () => {
    assert.equal(
      namaBerkasEkspor('attachment; filename="pengguna-2026-11-06.csv"'),
      "pengguna-2026-11-06.csv",
    );
    const sekarang = new Date("2026-11-06T10:00:00Z");
    assert.equal(namaBerkasEkspor(null, sekarang), "pengguna-2026-11-06.csv");
    assert.equal(
      namaBerkasEkspor("attachment", sekarang),
      "pengguna-2026-11-06.csv",
    );
    assert.equal(
      namaBerkasEkspor('attachment; filename="../../etc/passwd"', sekarang),
      "pengguna-2026-11-06.csv",
      "tanpa path",
    );
  });
});

describe("undanganApi", () => {
  it("undang: POST /api/pengguna/undang dengan JSON isian, hasil dipetakan", async () => {
    const p = fetchMenjawab(() => Response.json(undanganWire, { status: 201 }));

    const h = await undanganApi.undang({
      nama: "Budi",
      email: "budi@garda.test",
      peran: "OPS_MAHASISWA",
      lembaga: "FMIPA",
      kabupaten_kode: "1171",
    });

    assert.equal(p.length, 1);
    assert.equal(p[0].url, "/api/pengguna/undang");
    assert.equal(p[0].init.method, "POST");
    assert.deepEqual(JSON.parse(String(p[0].init.body)), {
      nama: "Budi",
      email: "budi@garda.test",
      peran: "OPS_MAHASISWA",
      lembaga: "FMIPA",
      kabupaten_kode: "1171",
    });
    assert.equal(h.pengguna.status, "MENUNGGU");
    assert.equal(h.emailTerkirim, true);
  });

  it("undang: galat server (409) menjadi ApiError berpesan server", async () => {
    fetchMenjawab(() =>
      Response.json(
        { error: true, message: "Email sudah terdaftar" },
        { status: 409 },
      ),
    );

    await assert.rejects(
      undanganApi.undang({
        nama: "B",
        email: "b@garda.test",
        peran: "OPS_MAHASISWA",
        lembaga: "X",
      }),
      (err: unknown) =>
        err instanceof ApiError &&
        err.status === 409 &&
        err.message === "Email sudah terdaftar",
    );
  });

  it("undangUlang: POST /api/pengguna/{id}/undang-ulang dengan id ter-encode", async () => {
    const p = fetchMenjawab(() => Response.json(undanganWire));

    const h = await undanganApi.undangUlang("USR/1 ?x");

    assert.equal(p[0].url, "/api/pengguna/USR%2F1%20%3Fx/undang-ulang");
    assert.equal(p[0].init.method, "POST");
    assert.equal(h.pengguna.id, "ef45e253-50b5-3808-9710-90ed1484f293");
  });

  it("impor: POST /api/pengguna/impor dengan csv dan dry_run, hasil dipetakan", async () => {
    const p = fetchMenjawab(() =>
      Response.json({
        dry_run: true,
        total: 1,
        valid: 1,
        galat: 0,
        dibuat: 0,
        email_aktif: true,
        baris: [
          {
            baris: 2,
            nama: "Budi",
            email: "budi@garda.test",
            peran: "OPS_MAHASISWA",
            lembaga: "FMIPA",
            kabupaten_kode: "",
            status: "valid",
            galat: [],
          },
        ],
      }),
    );

    const periksa = await undanganApi.impor("nama;email\r\n", true);
    await undanganApi.impor("nama;email\r\n", false);

    assert.equal(p[0].url, "/api/pengguna/impor");
    assert.equal(p[0].init.method, "POST");
    assert.deepEqual(JSON.parse(String(p[0].init.body)), {
      csv: "nama;email\r\n",
      dry_run: true,
    });
    assert.deepEqual(JSON.parse(String(p[1].init.body)), {
      csv: "nama;email\r\n",
      dry_run: false,
    });
    assert.equal(periksa.baris[0].status, "valid");
    assert.equal(periksa.baris[0].kabupatenKode, "");
  });

  it("impor: galat berkas dari server (400) diteruskan sebagai ApiError", async () => {
    fetchMenjawab(() =>
      Response.json(
        { error: true, message: "Kolom wajib tidak ada di header: peran." },
        { status: 400 },
      ),
    );

    await assert.rejects(
      undanganApi.impor("x", true),
      (err: unknown) =>
        err instanceof ApiError &&
        err.status === 400 &&
        /Kolom wajib/.test(err.message),
    );
  });
});

describe("unduhEksporPengguna", () => {
  /** Memasang document palsu yang merekam unduhan; mengembalikan perekamnya. */
  function pasangDocument() {
    const unduhan: { href: string; download: string; klik: number }[] = [];
    (globalThis as { document?: unknown }).document = {
      createElement: () => {
        const a = {
          href: "",
          download: "",
          click: () => (a as unknown as { klik: number }).klik++,
          klik: 0,
        };
        unduhan.push(a as never);
        return a;
      },
    };
    return unduhan;
  }

  it("mengunduh dari BFF, memicu simpan berkas, dan melaporkan jumlah baris", async () => {
    const unduhan = pasangDocument();
    const p = fetchMenjawab(
      () =>
        new Response("\ufeffid;nama\r\nef45e253-50b5-3808-9710-90ed1484f293;Budi\r\n", {
          headers: {
            "content-type": "text/csv; charset=utf-8",
            "content-disposition":
              'attachment; filename="pengguna-2026-11-06.csv"',
            "x-export-rows": "1",
          },
        }),
    );

    const hasil = await unduhEksporPengguna({ ids: ["ef45e253-50b5-3808-9710-90ed1484f293"] });

    assert.equal(p[0].url, "/api/pengguna/ekspor?ids=ef45e253-50b5-3808-9710-90ed1484f293");
    assert.deepEqual(hasil, { baris: 1, terpotong: false });
    assert.equal(unduhan.length, 1);
    assert.equal(unduhan[0].download, "pengguna-2026-11-06.csv");
    assert.equal(unduhan[0].klik, 1);
  });

  it("hasil terpotong dilaporkan; tanpa filter memanggil tanpa query", async () => {
    pasangDocument();
    const p = fetchMenjawab(
      () =>
        new Response("id\r\n", {
          headers: { "x-export-rows": "20000", "x-export-truncated": "true" },
        }),
    );

    const hasil = await unduhEksporPengguna();

    assert.equal(p[0].url, "/api/pengguna/ekspor");
    assert.deepEqual(hasil, { baris: 20000, terpotong: true });
  });

  it("galat server (403/500) menjadi ApiError berpesan server dan tidak memicu unduhan", async () => {
    const unduhan = pasangDocument();
    fetchMenjawab(() =>
      Response.json(
        { error: true, message: "Gagal mengekspor pengguna" },
        { status: 500 },
      ),
    );

    await assert.rejects(
      unduhEksporPengguna(),
      (err: unknown) =>
        err instanceof ApiError &&
        err.status === 500 &&
        err.message === "Gagal mengekspor pengguna",
    );
    assert.equal(unduhan.length, 0);
  });

  it("pilihan lebih dari batas ditolak sebelum menghubungi server", async () => {
    const p = fetchMenjawab(() => new Response("x"));
    const ids = Array.from({ length: MAKS_ID_EKSPOR + 1 }, (_, i) => `u${i}`);

    await assert.rejects(unduhEksporPengguna({ ids }), /paling banyak 500/);
    assert.equal(p.length, 0);
    await assert.doesNotReject(async () => {
      pasangDocument();
      fetchMenjawab(
        () => new Response("id\r\n", { headers: { "x-export-rows": "0" } }),
      );
      await unduhEksporPengguna({ ids: ids.slice(0, MAKS_ID_EKSPOR) });
    });
  });
});
