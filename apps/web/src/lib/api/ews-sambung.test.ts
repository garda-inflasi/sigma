import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { HasilPulih } from "@/lib/auth/sesi-pulih";
import {
  jalankanAliranEws,
  type SumberAlert,
  type StatusAliran,
} from "./ews-sambung";

class SumberPalsu implements SumberAlert {
  readyState = 0;
  onopen: ((ev: Event) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  tertutup = false;
  private pendengar: ((ev: MessageEvent<string>) => void)[] = [];
  close() {
    this.tertutup = true;
    this.readyState = 2;
  }
  addEventListener(_t: "alert", fn: (ev: MessageEvent<string>) => void) {
    this.pendengar.push(fn);
  }
  buka() {
    this.readyState = 1;
    this.onopen?.(new Event("open"));
  }
  /** Putus jaringan: browser menyambung ulang sendiri (CONNECTING). */
  putusSementara() {
    this.readyState = 0;
    this.onerror?.(new Event("error"));
  }
  /** Ditolak server (mis. 401): koneksi mati permanen (CLOSED). */
  tolak() {
    this.readyState = 2;
    this.onerror?.(new Event("error"));
  }
  kirim(data: string) {
    for (const fn of this.pendengar) fn({ data } as MessageEvent<string>);
  }
}

function siapkan(
  opsi: { pulih?: (sejak: number) => Promise<HasilPulih> } = {},
) {
  const sumber: SumberPalsu[] = [];
  const status: StatusAliran[] = [];
  const alert: string[] = [];
  let tersambungUlang = 0;
  const jadwal: { fn: () => void; ms: number; batal: boolean }[] = [];
  const pulihPanggil: number[] = [];
  const hentikan = jalankanAliranEws({
    buat: () => {
      const s = new SumberPalsu();
      sumber.push(s);
      return s;
    },
    pulih: async (sejak) => {
      pulihPanggil.push(sejak);
      return (opsi.pulih ?? (async () => "ok" as const))(sejak);
    },
    saatStatus: (s) => status.push(s),
    saatAlert: (d) => alert.push(d),
    saatTersambungUlang: () => {
      tersambungUlang += 1;
    },
    jadwalkan: (fn, ms) => {
      const entri = { fn, ms, batal: false };
      jadwal.push(entri);
      return () => {
        entri.batal = true;
      };
    },
    sekarang: () => 1_000,
  });
  /** Menjalankan jadwal yang belum dibatalkan dan menunggu pemulihan async selesai. */
  const jalankanJadwal = async () => {
    await new Promise((r) => setImmediate(r));
    for (const j of jadwal.splice(0)) if (!j.batal) j.fn();
  };
  return {
    sumber,
    status,
    alert,
    jadwal,
    pulihPanggil,
    hentikan,
    jalankanJadwal,
    tersambungUlang: () => tersambungUlang,
  };
}

describe("jalankanAliranEws", () => {
  it("tersambung: status menghubungkan → terhubung, alert diteruskan", () => {
    const t = siapkan();
    assert.deepEqual(t.status, ["menghubungkan"]);
    t.sumber[0].buka();
    t.sumber[0].kirim('{"id":"A"}');
    assert.deepEqual(t.status, ["menghubungkan", "terhubung"]);
    assert.deepEqual(t.alert, ['{"id":"A"}']);
    assert.equal(t.tersambungUlang(), 0);
  });

  it("putus jaringan: browser menyambung sendiri, tidak ada pemulihan; daftar disegarkan saat tersambung lagi", () => {
    const t = siapkan();
    t.sumber[0].buka();
    t.sumber[0].putusSementara();
    assert.equal(t.status.at(-1), "terputus");
    assert.equal(t.sumber.length, 1);
    assert.equal(t.pulihPanggil.length, 0);
    t.sumber[0].buka();
    assert.equal(t.status.at(-1), "terhubung");
    assert.equal(t.tersambungUlang(), 1);
  });

  it("ditolak server (401, CLOSED): pulihkan sesi lalu buat EventSource baru, dan segarkan daftar saat terbuka", async () => {
    const t = siapkan();
    t.sumber[0].buka();
    t.sumber[0].tolak();
    assert.equal(t.sumber[0].tertutup, true);
    await t.jalankanJadwal();

    assert.deepEqual(
      t.pulihPanggil,
      [1_000],
      "pulih diberi waktu koneksi yang gagal dibuat",
    );
    assert.equal(t.sumber.length, 2, "EventSource baru setelah refresh");
    t.sumber[1].buka();
    assert.equal(t.status.at(-1), "terhubung");
    assert.equal(
      t.tersambungUlang(),
      1,
      "event yang terlewat diganti dengan penyegaran daftar",
    );
    t.sumber[1].kirim('{"id":"B"}');
    assert.deepEqual(t.alert, ['{"id":"B"}']);
  });

  it("sesi berakhir: tidak menyambung lagi (pulih() sudah mengarahkan ke /masuk)", async () => {
    const t = siapkan({ pulih: async () => "berakhir" });
    t.sumber[0].tolak();
    await t.jalankanJadwal();
    assert.equal(t.sumber.length, 1);
    assert.equal(t.jadwal.length, 0);
  });

  it("gagal sementara memulihkan sesi: dicoba lagi dengan jeda yang bertambah", async () => {
    const t = siapkan({ pulih: async () => "gagal" });
    t.sumber[0].tolak();
    await new Promise((r) => setImmediate(r));
    assert.deepEqual(
      t.jadwal.map((j) => j.ms),
      [2_000],
    );
    await t.jalankanJadwal();
    assert.equal(t.sumber.length, 2);
    t.sumber[1].tolak();
    await new Promise((r) => setImmediate(r));
    assert.deepEqual(
      t.jadwal.map((j) => j.ms),
      [4_000],
    );
  });

  it("pemulihan berhasil tetapi koneksi baru ditolak terus (mis. 403): berhenti setelah beberapa kali, tidak memutar token tanpa akhir", async () => {
    const t = siapkan();
    t.sumber[0].tolak();
    for (let i = 0; i < 10; i++) {
      await t.jalankanJadwal();
      t.sumber.at(-1)!.tolak();
    }
    await t.jalankanJadwal();
    assert.equal(t.pulihPanggil.length, 3);
    assert.equal(t.sumber.length, 4, "satu awal + tiga percobaan");
  });

  it("koneksi yang sempat terbuka mereset hitungan percobaan sia-sia", async () => {
    const t = siapkan();
    for (let i = 0; i < 5; i++) {
      t.sumber.at(-1)!.buka();
      t.sumber.at(-1)!.tolak();
      await t.jalankanJadwal();
    }
    assert.equal(t.pulihPanggil.length, 5);
  });

  it("dihentikan: sumber ditutup, jadwal dibatalkan, dan pemulihan yang tertunda tidak membuat sumber baru", async () => {
    let selesaikan: (h: HasilPulih) => void = () => {};
    const t = siapkan({ pulih: () => new Promise((r) => (selesaikan = r)) });
    t.sumber[0].tolak();
    t.hentikan();
    selesaikan("ok");
    await t.jalankanJadwal();
    assert.equal(t.sumber.length, 1);
    assert.equal(t.sumber[0].tertutup, true);
  });

  it("event dari sumber lama diabaikan setelah diganti", async () => {
    const t = siapkan();
    const lama = t.sumber[0];
    lama.tolak();
    await t.jalankanJadwal();
    lama.kirim('{"id":"LAMA"}');
    lama.buka();
    assert.deepEqual(t.alert, []);
  });
});
