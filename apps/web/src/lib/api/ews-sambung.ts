/**
 * Penyambung aliran EWS (SSE) yang pulih setelah access token habis.
 *
 * EventSource menyambung ulang sendiri hanya untuk putus jaringan. Bila server menjawab bukan-200 (mis. 401
 * karena cookie access token sudah lenyap), koneksi berstatus CLOSED dan tidak pernah dicoba lagi. API Go
 * juga menutup aliran saat access token kedaluwarsa, jadi setelah ±15 menit penyambungan ulang otomatis
 * selalu membentur 401. Di sini: bila CLOSED, tukar refresh token (single-flight bersama permintaan data),
 * lalu buat EventSource baru. Event yang terlewat selama jeda tidak dapat diminta ulang (Last-Event-ID tidak
 * bisa diatur pada EventSource baru), jadi pemanggil menyegarkan daftar alert saat tersambung kembali.
 */

import type { HasilPulih } from "@/lib/auth/sesi-pulih";

const CLOSED = 2;

export type StatusAliran = "menghubungkan" | "terhubung" | "terputus";

/** Bagian EventSource yang dipakai (memudahkan tes dengan sumber palsu). */
export interface SumberAlert {
  readyState: number;
  close(): void;
  onopen: ((ev: Event) => void) | null;
  onerror: ((ev: Event) => void) | null;
  addEventListener(
    type: "alert",
    listener: (ev: MessageEvent<string>) => void,
  ): void;
}

export interface OpsiAliran {
  buat: () => SumberAlert;
  /** Memulihkan sesi (refresh); `sejak` = kapan koneksi yang gagal dibuat. */
  pulih: (sejak: number) => Promise<HasilPulih>;
  saatStatus: (status: StatusAliran) => void;
  saatAlert: (data: string) => void;
  /** Tersambung kembali setelah pernah terputus: segarkan daftar karena event selama jeda hilang. */
  saatTersambungUlang: () => void;
  jadwalkan?: (fn: () => void, ms: number) => () => void;
  sekarang?: () => number;
  /** Jeda awal sebelum mencoba lagi setelah gagal pulih, digandakan sampai `jedaMaksMs`. */
  jedaAwalMs?: number;
  jedaMaksMs?: number;
  /**
   * Pemulihan yang berhasil tetapi koneksi barunya langsung ditolak lagi, berturut-turut, sebelum menyerah
   * (mis. 403: izin hilang, bukan masalah sesi). Kegagalan sementara memulihkan sesi dicoba terus dengan jeda.
   */
  maksPemulihanSiaSia?: number;
}

function jadwalBawaan(fn: () => void, ms: number): () => void {
  const id = setTimeout(fn, ms);
  return () => clearTimeout(id);
}

/** Memulai aliran; mengembalikan fungsi untuk menghentikannya. */
export function jalankanAliranEws(opsi: OpsiAliran): () => void {
  const jadwalkan = opsi.jadwalkan ?? jadwalBawaan;
  const sekarang = opsi.sekarang ?? Date.now;
  const jedaAwal = opsi.jedaAwalMs ?? 2_000;
  const jedaMaks = opsi.jedaMaksMs ?? 30_000;
  const maksSiaSia = opsi.maksPemulihanSiaSia ?? 3;

  let berhenti = false;
  let sumber: SumberAlert | null = null;
  let pernahTerputus = false;
  let siaSia = 0;
  let gagalBeruntun = 0;
  let batalJadwal: (() => void) | null = null;

  const sambung = () => {
    if (berhenti) return;
    const dibuatPada = sekarang();
    const es = opsi.buat();
    sumber = es;
    opsi.saatStatus("menghubungkan");

    es.onopen = () => {
      if (berhenti || sumber !== es) return;
      siaSia = 0;
      gagalBeruntun = 0;
      opsi.saatStatus("terhubung");
      if (pernahTerputus) opsi.saatTersambungUlang();
      pernahTerputus = false;
    };
    es.onerror = () => {
      if (berhenti || sumber !== es) return;
      pernahTerputus = true;
      opsi.saatStatus("terputus");
      // CONNECTING: browser sedang mencoba lagi sendiri. CLOSED: ditolak server, tidak akan dicoba lagi.
      if (es.readyState !== CLOSED) return;
      es.close();
      void pulihkan(dibuatPada);
    };
    es.addEventListener("alert", (e) => {
      if (!berhenti && sumber === es) opsi.saatAlert(e.data);
    });
  };

  const pulihkan = async (dibuatPada: number) => {
    if (siaSia >= maksSiaSia) return;
    const hasil = await opsi.pulih(dibuatPada);
    if (berhenti) return;
    if (hasil === "berakhir") return; // pulih() sudah mengarahkan ke /masuk
    let jeda: number;
    if (hasil === "ok") {
      siaSia += 1;
      jeda = siaSia <= 1 ? 0 : Math.min(jedaAwal * 2 ** (siaSia - 2), jedaMaks);
    } else {
      gagalBeruntun += 1;
      jeda = Math.min(jedaAwal * 2 ** (gagalBeruntun - 1), jedaMaks);
    }
    batalJadwal = jadwalkan(() => {
      batalJadwal = null;
      sambung();
    }, jeda);
  };

  sambung();

  return () => {
    berhenti = true;
    batalJadwal?.();
    sumber?.close();
    sumber = null;
  };
}
