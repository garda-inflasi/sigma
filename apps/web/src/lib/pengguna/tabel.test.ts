import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  KOLOM_URUT,
  MAKS_PANJANG_CARI,
  STATE_AWAL,
  UKURAN_BAWAAN,
  jumlahFilterAktif,
  jumlahHalaman,
  koreksiHalaman,
  kunciCachePengguna,
  parseStateTabel,
  queryApiPengguna,
  queryUrlPengguna,
  resetFilter,
  ubahCari,
  ubahFilter,
  ubahHalaman,
  ubahUkuran,
  ubahUrut,
  type StateTabelPengguna,
} from "./tabel";

const dariQuery = (qs: string) => parseStateTabel(new URLSearchParams(qs));
const state = (
  patch: Partial<StateTabelPengguna> = {},
): StateTabelPengguna => ({ ...STATE_AWAL, ...patch });

describe("parseStateTabel", () => {
  it("URL kosong menghasilkan keadaan awal", () => {
    assert.deepEqual(dariQuery(""), STATE_AWAL);
  });

  it("membaca semua parameter yang sah", () => {
    const s = dariQuery(
      "search=ani&peran=ADMIN_SISTEM&status=NONAKTIF&lapis=SISTEM&mfa=true&terkunci=false&sort=nama&arah=desc&page=3&limit=50",
    );
    assert.deepEqual(s, {
      search: "ani",
      peran: "ADMIN_SISTEM",
      status: "NONAKTIF",
      lapis: "SISTEM",
      mfa: "true",
      terkunci: "false",
      sort: "nama",
      arah: "desc",
      page: 3,
      limit: 50,
    });
  });

  it("nilai asing dibuang, bukan dianggap galat", () => {
    const s = dariQuery(
      "peran=SUPERADMIN&status=aktif&lapis=OPS&mfa=1&terkunci=ya&sort=password_hash&arah=desc&page=abc&limit=7",
    );
    assert.deepEqual(
      s,
      STATE_AWAL,
      "URL hasil suntingan tangan tidak boleh merusak tabel atau sampai ke API",
    );
  });

  it("arah tanpa sort diabaikan; sort tanpa arah naik", () => {
    assert.equal(dariQuery("arah=desc").arah, undefined);
    const s = dariQuery("sort=email");
    assert.equal(s.sort, "email");
    assert.equal(s.arah, "asc");
    assert.equal(dariQuery("sort=email&arah=salah").arah, "asc");
  });

  it("halaman harus bilangan bulat positif", () => {
    for (const salah of [
      "0",
      "-2",
      "1.5",
      "1e3",
      "",
      " 2",
      "999999999999999999999",
    ]) {
      assert.equal(
        dariQuery(`page=${encodeURIComponent(salah)}`).page,
        1,
        salah,
      );
    }
    assert.equal(dariQuery("page=12").page, 12);
  });

  it("ukuran halaman hanya dari pilihan yang tersedia", () => {
    for (const n of [10, 25, 50, 100])
      assert.equal(dariQuery(`limit=${n}`).limit, n);
    for (const n of ["0", "20", "101", "-10", "x"])
      assert.equal(dariQuery(`limit=${n}`).limit, UKURAN_BAWAAN, n);
  });

  it("kata kunci dipangkas dan dibatasi panjangnya", () => {
    assert.equal(dariQuery("search=++ani++").search, "ani");
    assert.equal(
      dariQuery(`search=${"x".repeat(MAKS_PANJANG_CARI + 50)}`).search.length,
      MAKS_PANJANG_CARI,
    );
  });

  it("menerima semua kolom urutan API", () => {
    for (const kolom of KOLOM_URUT)
      assert.equal(dariQuery(`sort=${kolom}`).sort, kolom);
  });
});

describe("queryUrlPengguna dan queryApiPengguna", () => {
  it("URL membuang nilai bawaan; keadaan awal berupa kueri kosong", () => {
    assert.equal(queryUrlPengguna(STATE_AWAL), "");
    assert.equal(
      queryUrlPengguna(state({ page: 1, limit: UKURAN_BAWAAN })),
      "",
    );
    assert.equal(queryUrlPengguna(state({ page: 2 })), "page=2");
    assert.equal(queryUrlPengguna(state({ limit: 50 })), "limit=50");
  });

  it("API selalu membawa page dan limit", () => {
    assert.equal(queryApiPengguna(STATE_AWAL), "page=1&limit=25");
  });

  it("urutan parameter tetap dan sort selalu disertai arah", () => {
    const s = state({
      search: "ani aceh",
      peran: "MANAJER_DATA",
      status: "AKTIF",
      lapis: "MANAJERIAL",
      mfa: "false",
      terkunci: "true",
      sort: "terakhir_masuk",
      arah: "desc",
      page: 4,
      limit: 10,
    });
    assert.equal(
      queryApiPengguna(s),
      "search=ani+aceh&peran=MANAJER_DATA&status=AKTIF&lapis=MANAJERIAL&mfa=false&terkunci=true&sort=terakhir_masuk&arah=desc&page=4&limit=10",
    );
    assert.equal(
      queryUrlPengguna(s),
      "search=ani+aceh&peran=MANAJER_DATA&status=AKTIF&lapis=MANAJERIAL&mfa=false&terkunci=true&sort=terakhir_masuk&arah=desc&page=4&limit=10",
    );
    assert.equal(
      queryApiPengguna(state({ sort: "nama" })),
      "sort=nama&arah=asc&page=1&limit=25",
      "arah bawaan ditulis eksplisit",
    );
  });

  it("karakter khusus di kata kunci di-escape", () => {
    assert.equal(
      queryApiPengguna(state({ search: "100% & a_b" })),
      "search=100%25+%26+a_b&page=1&limit=25",
    );
  });

  it("kata kunci hanya spasi tidak dikirim", () => {
    assert.equal(queryApiPengguna(state({ search: "   " })), "page=1&limit=25");
  });

  it("URL lalu parse memulihkan keadaan yang sama (bolak-balik)", () => {
    const kasus: StateTabelPengguna[] = [
      STATE_AWAL,
      state({ search: "ani" }),
      state({ peran: "ADMIN_SISTEM", status: "AKTIF", page: 7, limit: 100 }),
      state({ mfa: "true", terkunci: "true", sort: "lembaga", arah: "desc" }),
      state({
        search: "100% & a_b",
        lapis: "STRATEGIS",
        sort: "status",
        arah: "asc",
        page: 2,
        limit: 10,
      }),
    ];
    for (const s of kasus) {
      assert.deepEqual(
        parseStateTabel(new URLSearchParams(queryUrlPengguna(s))),
        s,
        queryUrlPengguna(s),
      );
    }
  });

  it("kunci cache sama untuk keadaan yang sama dan beda bila hasil yang diminta beda", () => {
    assert.equal(
      kunciCachePengguna(state({ search: "a" })),
      kunciCachePengguna(state({ search: "a" })),
    );
    assert.notEqual(
      kunciCachePengguna(state({ page: 1 })),
      kunciCachePengguna(state({ page: 2 })),
    );
    assert.notEqual(
      kunciCachePengguna(state({ sort: "nama", arah: "asc" })),
      kunciCachePengguna(state({ sort: "nama", arah: "desc" })),
    );
  });
});

describe("transisi keadaan", () => {
  it("mengubah kata kunci, filter, ukuran, dan urutan kembali ke halaman 1", () => {
    const di3 = state({ page: 3, search: "x", peran: "ADMIN_SISTEM" });
    assert.equal(ubahCari(di3, "ani").page, 1);
    assert.equal(ubahFilter(di3, "status", "AKTIF").page, 1);
    assert.equal(ubahUkuran(di3, 50).page, 1);
    assert.equal(ubahUrut(di3, "nama").page, 1);
    assert.equal(resetFilter(di3).page, 1);
  });

  it("mengganti halaman tidak mengubah yang lain dan tidak pernah di bawah 1", () => {
    const s = state({
      search: "ani",
      peran: "ADMIN_SISTEM",
      sort: "nama",
      arah: "asc",
    });
    assert.deepEqual(ubahHalaman(s, 4), { ...s, page: 4 });
    assert.equal(ubahHalaman(s, 0).page, 1);
    assert.equal(ubahHalaman(s, -3).page, 1);
    assert.equal(ubahHalaman(s, Number.NaN).page, 1);
    assert.equal(ubahHalaman(s, 2.9).page, 2);
  });

  it("ubahFilter memasang dan membuang filter tanpa menyentuh yang lain", () => {
    let s = ubahFilter(STATE_AWAL, "peran", "MANAJER_DATA");
    s = ubahFilter(s, "mfa", "true");
    assert.equal(s.peran, "MANAJER_DATA");
    assert.equal(s.mfa, "true");
    s = ubahFilter(s, "peran", undefined);
    assert.equal(
      "peran" in s,
      false,
      "dibuang, bukan undefined: URL dan kunci cache bersih",
    );
    assert.equal(s.mfa, "true");
    assert.equal("mfa" in ubahFilter(s, "mfa", ""), false);
  });

  it("ubahCari memangkas dan membatasi", () => {
    assert.equal(ubahCari(STATE_AWAL, "  ani ").search, "ani");
    assert.equal(
      ubahCari(STATE_AWAL, "x".repeat(500)).search.length,
      MAKS_PANJANG_CARI,
    );
  });

  it("resetFilter membuang filter saja; kata kunci dan urutan tetap", () => {
    const s = state({
      search: "ani",
      sort: "nama",
      arah: "desc",
      peran: "ADMIN_SISTEM",
      status: "AKTIF",
      lapis: "SISTEM",
      mfa: "true",
      terkunci: "false",
      page: 5,
    });
    assert.deepEqual(resetFilter(s), {
      search: "ani",
      sort: "nama",
      arah: "desc",
      page: 1,
      limit: 25,
    });
  });

  it("ubahUkuran menolak ukuran di luar pilihan", () => {
    assert.equal(ubahUkuran(STATE_AWAL, 100).limit, 100);
    assert.equal(ubahUkuran(STATE_AWAL, 33).limit, UKURAN_BAWAAN);
  });

  it("ubahUrut berputar: kolom baru naik, naik jadi turun, turun kembali ke bawaan", () => {
    let s = ubahUrut(STATE_AWAL, "nama");
    assert.deepEqual([s.sort, s.arah], ["nama", "asc"]);
    s = ubahUrut(s, "nama");
    assert.deepEqual([s.sort, s.arah], ["nama", "desc"]);
    s = ubahUrut(s, "nama");
    assert.deepEqual([s.sort, s.arah], [undefined, undefined]);
    assert.equal("sort" in s, false);
    s = ubahUrut(ubahUrut(STATE_AWAL, "nama"), "peran");
    assert.deepEqual(
      [s.sort, s.arah],
      ["peran", "asc"],
      "pindah kolom selalu mulai dari naik",
    );
  });

  it("jumlahFilterAktif hanya menghitung filter", () => {
    assert.equal(jumlahFilterAktif(STATE_AWAL), 0);
    assert.equal(
      jumlahFilterAktif(
        state({ search: "ani", sort: "nama", arah: "asc", page: 3 }),
      ),
      0,
    );
    assert.equal(
      jumlahFilterAktif(state({ peran: "ADMIN_SISTEM", mfa: "false" })),
      2,
    );
  });
});

describe("halaman", () => {
  it("jumlahHalaman membulatkan ke atas dan minimal 1", () => {
    assert.equal(jumlahHalaman(0, 25), 1);
    assert.equal(jumlahHalaman(25, 25), 1);
    assert.equal(jumlahHalaman(26, 25), 2);
    assert.equal(jumlahHalaman(100, 10), 10);
    assert.equal(jumlahHalaman(-5, 25), 1);
    assert.equal(jumlahHalaman(10, 0), 10, "limit tak sah diperlakukan 1");
  });

  it("koreksiHalaman mengarahkan ke halaman terakhir bila terlewat", () => {
    assert.equal(
      koreksiHalaman(state({ page: 3 }), 60),
      null,
      "halaman 3 dari 3 sah",
    );
    assert.equal(koreksiHalaman(state({ page: 4 }), 60), 3);
    assert.equal(koreksiHalaman(state({ page: 9, limit: 10 }), 41), 5);
    assert.equal(
      koreksiHalaman(state({ page: 2 }), 0),
      null,
      "tanpa baris: tampilkan keadaan kosong, jangan berputar",
    );
    assert.equal(koreksiHalaman(state({ page: 1 }), 0), null);
  });
});
