import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import type { PenggunaRow } from "@/lib/data/types";
import { STATE_AWAL, type StateTabelPengguna } from "@/lib/pengguna/tabel";
import { ROLES } from "@/lib/rbac/roles";
import { penggunaApi, toPenggunaRow, type UserResponse } from "./pengguna";
import { queryMockPengguna } from "./pengguna-mock";

const realFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = realFetch;
});

const dasar: UserResponse = {
  id: "ef45e253-50b5-3808-9710-90ed1484f293",
  nama: "Siti Rahma",
  email: "siti@gardainflasi.id",
  peran: "MANAJER_DATA",
  lapis: "MANAJERIAL",
  lembaga: "BPS Aceh",
  mfa_aktif: true,
  status: "AKTIF",
  terakhir_masuk: "2026-10-16T08:00:00Z",
  dibuat_pada: "2026-01-05T08:00:00Z",
};

function stubFetch(body: unknown, status = 200) {
  const panggilan: { url: string; init: RequestInit }[] = [];
  globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
    panggilan.push({ url: String(url), init: init ?? {} });
    return Response.json(body, { status });
  }) as typeof fetch;
  return panggilan;
}

describe("penggunaApi.daftar", () => {
  it("memanggil BFF dengan kueri keadaan tabel dan memetakan halaman", async () => {
    const panggilan = stubFetch({
      items: [dasar],
      total: 41,
      page: 2,
      limit: 10,
    });
    const state: StateTabelPengguna = {
      ...STATE_AWAL,
      search: "siti",
      status: "AKTIF",
      mfa: "true",
      sort: "nama",
      arah: "desc",
      page: 2,
      limit: 10,
    };

    const halaman = await penggunaApi.daftar(state);

    assert.equal(
      panggilan[0].url,
      "/api/pengguna?search=siti&status=AKTIF&mfa=true&sort=nama&arah=desc&page=2&limit=10",
    );
    assert.equal(halaman.total, 41);
    assert.equal(halaman.page, 2);
    assert.equal(halaman.limit, 10);
    assert.equal(halaman.rows.length, 1);
    assert.equal(halaman.rows[0].id, "ef45e253-50b5-3808-9710-90ed1484f293");
    assert.equal(halaman.rows[0].mfa, true);
    assert.equal(halaman.rows[0].dibuatPada, "2026-01-05T08:00:00Z");
  });

  it("satu permintaan per halaman, bukan memuat semua halaman", async () => {
    const panggilan = stubFetch({
      items: [dasar],
      total: 500,
      page: 1,
      limit: 25,
    });
    await penggunaApi.daftar(STATE_AWAL);
    assert.equal(panggilan.length, 1);
  });
});

describe("penggunaApi.resetMfa dan riwayatPeran", () => {
  it("resetMfa memanggil POST /api/pengguna/{id}/reset-mfa", async () => {
    const panggilan = stubFetch({
      id: "a/b",
      mfa_aktif_sebelumnya: true,
      sesi_dicabut: 1,
    });

    const hasil = await penggunaApi.resetMfa("a/b");

    assert.deepEqual(hasil, {
      id: "a/b",
      mfa_aktif_sebelumnya: true,
      sesi_dicabut: 1,
    });
    assert.equal(panggilan[0].url, "/api/pengguna/a%2Fb/reset-mfa");
    assert.equal(panggilan[0].init.method, "POST");
  });

  it("riwayatPeran memetakan respons ke model tampilan", async () => {
    const panggilan = stubFetch({
      items: [
        {
          id: "a-1",
          waktu: "2026-11-06T09:00:00Z",
          aksi: "ASSIGN_ROLE",
          dari: "OPS_MAHASISWA",
          ke: "MANAJER_DATA",
          aktor: { id: "adm", nama: "Admin" },
        },
      ],
      terpotong: false,
    });

    const riwayat = await penggunaApi.riwayatPeran("USR 1");

    assert.equal(panggilan[0].url, "/api/pengguna/USR%201/riwayat-peran");
    assert.equal(riwayat.entri[0].dari, ROLES.OPS_MAHASISWA.label);
    assert.equal(riwayat.entri[0].ke, ROLES.MANAJER_DATA.label);
    assert.equal(riwayat.entri[0].aktor, "Admin");
  });
});

describe("toPenggunaRow", () => {
  it("membawa waktu dibuat untuk kolom Dibuat", () => {
    assert.equal(toPenggunaRow(dasar).dibuatPada, "2026-01-05T08:00:00Z");
  });
});

// ── Padanan mock dari GET /v1/users ───────────────────────────────────

function baris(id: string, patch: Partial<PenggunaRow> = {}): PenggunaRow {
  return {
    id,
    nama: `Pengguna ${id}`,
    email: `${id}@garda.test`,
    role: "OPS_MAHASISWA",
    lembaga: "BPPSDMP",
    wilayah: "Seluruh Aceh",
    status: "AKTIF",
    mfa: false,
    terakhirMasuk: "",
    dibuatPada: "2026-01-01T00:00:00Z",
    ...patch,
  };
}
const SEKARANG = Date.parse("2026-11-06T10:00:00Z");
const data: PenggunaRow[] = [
  baris("a", {
    nama: "Ani",
    role: "ADMIN_SISTEM",
    mfa: true,
    terakhirMasuk: "2026-10-01T00:00:00Z",
    dibuatPada: "2026-01-03T00:00:00Z",
  }),
  baris("b", {
    nama: "budi",
    role: "MANAJER_DATA",
    status: "NONAKTIF",
    lembaga: "BPS Aceh",
    dibuatPada: "2026-01-02T00:00:00Z",
  }),
  baris("c", {
    nama: "Citra",
    role: "STRATEGIS_BI",
    terakhirMasuk: "2026-10-01T00:00:00Z",
    terkunciSampai: "2026-11-06T10:15:00Z",
    dibuatPada: "2026-01-02T00:00:00Z",
  }),
  baris("d", {
    nama: "Dewi",
    role: "OPS_MAHASISWA",
    status: "MENUNGGU",
    terkunciSampai: "2026-11-06T09:00:00Z",
    dibuatPada: "2026-01-01T00:00:00Z",
  }),
];
const ids = (h: { rows: PenggunaRow[] }) => h.rows.map((r) => r.id);
const q = (patch: Partial<StateTabelPengguna>) =>
  queryMockPengguna(data, { ...STATE_AWAL, ...patch }, SEKARANG);

describe("queryMockPengguna", () => {
  it("urutan bawaan: terbaru dibuat dulu, kembar diputus id menurun", () => {
    assert.deepEqual(ids(q({})), ["a", "c", "b", "d"]);
  });

  it("urut kolom dua arah dengan tiebreaker id pada arah yang sama", () => {
    assert.deepEqual(
      ids(q({ sort: "nama", arah: "asc" })),
      ["a", "b", "c", "d"],
      "nama tak peka huruf besar-kecil",
    );
    assert.deepEqual(ids(q({ sort: "nama", arah: "desc" })), [
      "d",
      "c",
      "b",
      "a",
    ]);
    assert.deepEqual(ids(q({ sort: "dibuat_pada", arah: "asc" })), [
      "d",
      "b",
      "c",
      "a",
    ]);
    assert.deepEqual(
      ids(q({ sort: "peran", arah: "asc" })),
      ["c", "b", "d", "a"],
      "urutan enum peran",
    );
  });

  it("belum pernah masuk selalu di akhir pada kedua arah", () => {
    assert.deepEqual(ids(q({ sort: "terakhir_masuk", arah: "asc" })), [
      "a",
      "c",
      "b",
      "d",
    ]);
    // Yang belum pernah masuk tetap di akhir; di antaranya id mengikuti arah (menurun pada desc).
    assert.deepEqual(ids(q({ sort: "terakhir_masuk", arah: "desc" })), [
      "c",
      "a",
      "d",
      "b",
    ]);
  });

  it("kata kunci mencari nama, email, peran (juga dengan spasi), dan lembaga", () => {
    assert.deepEqual(ids(q({ search: "ANI" })), ["a"]);
    assert.deepEqual(ids(q({ search: "garda.test" })).length, 4);
    assert.deepEqual(ids(q({ search: "manajer data" })), ["b"]);
    assert.deepEqual(ids(q({ search: "bps aceh" })), ["b"]);
  });

  it("filter peran, status, lapis, mfa, dan terkunci digabung dengan AND", () => {
    assert.deepEqual(ids(q({ peran: "MANAJER_DATA" })), ["b"]);
    assert.deepEqual(ids(q({ status: "NONAKTIF" })), ["b"]);
    assert.deepEqual(ids(q({ lapis: "STRATEGIS" })), ["c"]);
    assert.deepEqual(ids(q({ mfa: "true" })), ["a"]);
    assert.deepEqual(ids(q({ mfa: "false" })), ["c", "b", "d"]);
    assert.deepEqual(
      ids(q({ terkunci: "true" })),
      ["c"],
      "kunci yang sudah lewat (d) tidak dihitung",
    );
    assert.deepEqual(ids(q({ terkunci: "false" })), ["a", "b", "d"]);
    assert.deepEqual(ids(q({ mfa: "false", terkunci: "false" })), ["b", "d"]);
  });

  it("halaman: total tetap seluruh yang cocok, baris hanya milik halaman itu", () => {
    const h1 = q({ limit: 10, page: 1 });
    assert.equal(h1.total, 4);
    const kecil = queryMockPengguna(data, { ...STATE_AWAL, page: 2 }, SEKARANG);
    assert.equal(kecil.rows.length, 0, "halaman di luar jangkauan kosong");
    assert.equal(kecil.total, 4);
    const dua = queryMockPengguna(
      Array.from({ length: 30 }, (_, i) =>
        baris(`u${String(i).padStart(2, "0")}`),
      ),
      { ...STATE_AWAL, page: 2, limit: 25 },
      SEKARANG,
    );
    assert.equal(dua.rows.length, 5);
    assert.equal(dua.total, 30);
  });
});
