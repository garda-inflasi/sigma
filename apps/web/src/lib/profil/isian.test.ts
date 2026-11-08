import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { MeUserResponse } from "@/lib/bff/api-client";
import {
  isianDari,
  periksaDraf,
  terapkanPerubahan,
  type DataDiri,
} from "./isian";

const TERSIMPAN: DataDiri = { nama: "Budi Santoso", telepon: "+6281234567890" };
const TANPA_TELEPON: DataDiri = { nama: "Budi Santoso" };

describe("isianDari", () => {
  it("nama apa adanya dan telepon dalam format yang mudah dibaca", () => {
    assert.deepEqual(isianDari(TERSIMPAN), {
      nama: "Budi Santoso",
      telepon: "+62 812-3456-7890",
    });
  });

  it("tanpa telepon: kotak kosong", () => {
    assert.deepEqual(isianDari(TANPA_TELEPON), {
      nama: "Budi Santoso",
      telepon: "",
    });
  });
});

describe("periksaDraf: tanpa perubahan", () => {
  it("draf kosong: tidak ada perubahan dan tidak ada galat", () => {
    assert.deepEqual(periksaDraf(TERSIMPAN, {}), { perubahan: {}, galat: {} });
  });

  it("nilai yang diketik sama dengan tersimpan (spasi di tepi, format telepon lain) bukan perubahan", () => {
    for (const telepon of [
      "0812-3456-7890",
      "+62 812-3456-7890",
      "6281234567890",
      "(0812) 3456 7890",
    ]) {
      assert.deepEqual(
        periksaDraf(TERSIMPAN, { nama: "  Budi Santoso ", telepon }),
        { perubahan: {}, galat: {} },
        telepon,
      );
    }
  });

  it("mengosongkan telepon yang memang kosong bukan perubahan", () => {
    assert.deepEqual(periksaDraf(TANPA_TELEPON, { telepon: "  " }), {
      perubahan: {},
      galat: {},
    });
  });

  it("bentuk tampilan hasil isianDari tidak dihitung perubahan (bolak-balik stabil)", () => {
    assert.deepEqual(periksaDraf(TERSIMPAN, isianDari(TERSIMPAN)), {
      perubahan: {},
      galat: {},
    });
    assert.deepEqual(periksaDraf(TANPA_TELEPON, isianDari(TANPA_TELEPON)), {
      perubahan: {},
      galat: {},
    });
  });
});

describe("periksaDraf: perubahan", () => {
  it("hanya field yang berbeda yang masuk isian PATCH", () => {
    assert.deepEqual(periksaDraf(TERSIMPAN, { nama: "Budi Baru" }).perubahan, {
      nama: "Budi Baru",
    });
    assert.deepEqual(
      periksaDraf(TERSIMPAN, { telepon: "0877 1111 2222" }).perubahan,
      {
        telepon: "+6287711112222",
      },
    );
    assert.deepEqual(
      periksaDraf(TERSIMPAN, { nama: "Budi Baru", telepon: "081111111111" })
        .perubahan,
      { nama: "Budi Baru", telepon: "+6281111111111" },
    );
  });

  it("nama dipangkas sebelum dikirim", () => {
    assert.deepEqual(
      periksaDraf(TERSIMPAN, { nama: "  Budi Baru  " }).perubahan,
      { nama: "Budi Baru" },
    );
  });

  it("telepon dikirim dalam E.164, bukan seperti yang diketik", () => {
    assert.deepEqual(
      periksaDraf(TANPA_TELEPON, { telepon: "0812-9876-5432" }).perubahan,
      {
        telepon: "+6281298765432",
      },
    );
  });

  it("mengosongkan telepon yang tersimpan menghapus nomor (string kosong)", () => {
    assert.deepEqual(periksaDraf(TERSIMPAN, { telepon: "" }).perubahan, {
      telepon: "",
    });
    assert.deepEqual(periksaDraf(TERSIMPAN, { telepon: "   " }).perubahan, {
      telepon: "",
    });
  });

  it("telepon tersimpan yang tidak baku (data warisan) dibandingkan setelah dibakukan", () => {
    const warisan: DataDiri = {
      nama: "Budi Santoso",
      telepon: "+62 812-3456-7890",
    };
    assert.deepEqual(periksaDraf(warisan, { telepon: "081234567890" }), {
      perubahan: {},
      galat: {},
    });
    const rusak: DataDiri = { nama: "Budi Santoso", telepon: "tidak-valid" };
    assert.deepEqual(
      periksaDraf(rusak, { telepon: "081234567890" }).perubahan,
      { telepon: "+6281234567890" },
    );
  });
});

describe("periksaDraf: galat", () => {
  it("nama terlalu pendek atau panjang membawa pesan API dan tidak menjadi perubahan", () => {
    assert.deepEqual(periksaDraf(TERSIMPAN, { nama: "Bu" }), {
      perubahan: {},
      galat: { nama: "Nama minimal 3 karakter" },
    });
    assert.deepEqual(periksaDraf(TERSIMPAN, { nama: "a".repeat(101) }), {
      perubahan: {},
      galat: { nama: "Nama maksimal 100 karakter" },
    });
  });

  it("telepon tidak valid membawa pesan API", () => {
    assert.deepEqual(periksaDraf(TERSIMPAN, { telepon: "12345" }), {
      perubahan: {},
      galat: { telepon: "Nomor telepon tidak valid" },
    });
  });

  it("galat satu field tidak menghapus perubahan field lain yang sah", () => {
    const hasil = periksaDraf(TERSIMPAN, {
      nama: "Bu",
      telepon: "081111111111",
    });
    assert.deepEqual(hasil.galat, { nama: "Nama minimal 3 karakter" });
    assert.deepEqual(hasil.perubahan, { telepon: "+6281111111111" });
  });

  it("field yang belum disentuh tidak dinilai: nama warisan yang terlalu pendek tidak menghalangi telepon", () => {
    const warisan: DataDiri = { nama: "Bu", telepon: undefined };
    assert.deepEqual(periksaDraf(warisan, { telepon: "081111111111" }), {
      perubahan: { telepon: "+6281111111111" },
      galat: {},
    });
  });
});

describe("terapkanPerubahan", () => {
  const profil = {
    id: "u1",
    nama: "Budi Santoso",
    email: "budi@garda.test",
    peran: "OPS_KOORDINATOR",
    lapis: "OPERASIONAL",
    lembaga: "FMIPA USK",
    mfa_aktif: false,
    status: "AKTIF",
    dibuat_pada: "2026-10-01T08:00:00Z",
    wajib_ganti_password: false,
    mfa_setup_diwajibkan: false,
    mfa_wajib: false,
    telepon: "+6281234567890",
  } satisfies MeUserResponse;

  it("mengganti nama dan telepon tanpa mengubah masukan", () => {
    const hasil = terapkanPerubahan(profil, {
      nama: "Budi Baru",
      telepon: "0811 1111 1111",
    });
    assert.equal(hasil.nama, "Budi Baru");
    assert.equal(hasil.telepon, "+6281111111111");
    assert.equal(profil.nama, "Budi Santoso", "masukan tidak diubah");
    assert.equal(profil.telepon, "+6281234567890");
  });

  it("telepon kosong menghapus field telepon", () => {
    const hasil = terapkanPerubahan(profil, { telepon: "" });
    assert.equal("telepon" in hasil, false);
    assert.equal(hasil.nama, "Budi Santoso");
  });

  it("field yang tidak ada di perubahan dibiarkan; telepon yang tidak valid diabaikan", () => {
    assert.deepEqual(terapkanPerubahan(profil, {}), profil);
    assert.deepEqual(terapkanPerubahan(profil, { telepon: "123" }), profil);
  });
});
