import assert from "node:assert/strict";
import { beforeEach, describe, it } from "node:test";
import type { KartuIntervensi } from "@/lib/data/types";
import { useKanban } from "./kanban-store";

function kartu(id: string, tahap: string, versi = 1): KartuIntervensi {
  return {
    id,
    siklus: "tekanan",
    tahap,
    judul: id,
    komoditasKode: "CBM-KRT",
    kabupatenKode: "1171",
    prioritas: "TINGGI",
    penanggungJawab: [],
    tenggat: "2026-11-05T00:00:00Z",
    rekomendasiAi: false,
    progres: { selesai: 0, total: 1 },
    versi,
    dieskalasi: false,
  };
}

describe("kanban-store", () => {
  beforeEach(() =>
    useKanban.setState({ ready: false, cards: {}, order: {}, selected: [] }),
  );

  it("sync menyusun kolom menurut tahap dan mempertahankan pilihan yang masih ada", () => {
    useKanban
      .getState()
      .sync([
        kartu("a", "t-ews"),
        kartu("b", "t-ews"),
        kartu("c", "t-verifikasi"),
      ]);
    useKanban.getState().selectMany(["a", "x"]);
    useKanban
      .getState()
      .sync([kartu("a", "t-ews"), kartu("c", "t-verifikasi")]);

    const s = useKanban.getState();
    assert.deepEqual(s.order["t-ews"], ["a"]);
    assert.deepEqual(s.order["t-verifikasi"], ["c"]);
    assert.deepEqual(s.selected, ["a"]);
  });

  it("move memindahkan kartu antar kolom pada posisi yang diminta", () => {
    useKanban
      .getState()
      .sync([
        kartu("a", "t-ews"),
        kartu("b", "t-verifikasi"),
        kartu("c", "t-verifikasi"),
      ]);
    useKanban.getState().move("a", "t-verifikasi", 1);

    const s = useKanban.getState();
    assert.deepEqual(s.order["t-verifikasi"], ["b", "a", "c"]);
    assert.deepEqual(s.order["t-ews"], []);
    assert.equal(s.cards.a.tahap, "t-verifikasi");
  });

  it("setVersi menyimpan versi baru dari server tanpa mengubah kartu lain", () => {
    useKanban.getState().sync([kartu("a", "t-ews", 1), kartu("b", "t-ews", 1)]);
    useKanban.getState().setVersi("a", 5);

    assert.equal(useKanban.getState().cards.a.versi, 5);
    assert.equal(useKanban.getState().cards.b.versi, 1);
  });

  it("setVersi pada kartu yang tidak ada diabaikan", () => {
    useKanban.getState().sync([kartu("a", "t-ews")]);
    useKanban.getState().setVersi("zzz", 9);
    assert.deepEqual(Object.keys(useKanban.getState().cards), ["a"]);
  });

  it("snapshot dan restore mengembalikan papan setelah pemindahan yang gagal", () => {
    useKanban
      .getState()
      .sync([kartu("a", "t-ews"), kartu("b", "t-verifikasi")]);
    const snap = useKanban.getState().snapshot();
    useKanban.getState().move("a", "t-verifikasi", 0);
    useKanban.getState().restore(snap);

    assert.deepEqual(useKanban.getState().order["t-ews"], ["a"]);
    assert.equal(useKanban.getState().cards.a.tahap, "t-ews");
  });
});
