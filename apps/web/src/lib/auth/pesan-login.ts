/**
 * Satu-satunya tabel pesan galat alur autentikasi (masuk, kode OTP, lupa/atur ulang/ganti kata sandi) beserta
 * kode mesin stabilnya. Dipakai di dua sisi: BFF (membentuk jawaban JSON dan alihan `/masuk?...`) dan form
 * (menampilkan pesan dan hitung mundur), jadi teks tidak pernah ditulis dua kali. Tanpa dependensi server
 * maupun React supaya aman diimpor di mana pun.
 *
 * Kode datang dari API Go (kolom `code` amplop galat, apps/api/internal/auth/dto/kunci.go) dan diteruskan BFF
 * tanpa diubah; satu-satunya kode yang lahir di web adalah RATE_LIMITED (batas laju BFF per IP).
 *
 * Keputusan "password kedaluwarsa": tidak ada kebijakan umur kata sandi yang aktif (PRD §6.1 #11 menyebut
 * PASSWORD_EXPIRY_DAYS dengan bawaan nonaktif). Kata sandi yang harus diganti, apa pun sebabnya, ditangani alur
 * wajib ganti yang sudah ada: API menjawab 403 PASSWORD_CHANGE_REQUIRED dan pengguna diarahkan ke /ganti-password.
 */

export const KODE_GALAT = {
  KREDENSIAL_SALAH: "INVALID_CREDENTIALS",
  AKUN_TERKUNCI: "ACCOUNT_LOCKED",
  AKUN_NONAKTIF: "ACCOUNT_INACTIVE",
  TERLALU_BANYAK: "RATE_LIMITED",
  MFA_KODE_SALAH: "MFA_CODE_INVALID",
  MFA_SESI_BERAKHIR: "MFA_CHALLENGE_EXPIRED",
  GANTI_PASSWORD_WAJIB: "PASSWORD_CHANGE_REQUIRED",
} as const;

export type KodeGalat = (typeof KODE_GALAT)[keyof typeof KODE_GALAT];

/** Pesan tiap kode. Kode yang menunggu (terkunci, batas laju) ditambah "Coba lagi dalam …" oleh `pesanTampil`. */
export const PESAN_GALAT: Record<KodeGalat, string> = {
  INVALID_CREDENTIALS: "Email atau password salah.",
  ACCOUNT_LOCKED: "Akun terkunci karena terlalu banyak percobaan gagal.",
  ACCOUNT_INACTIVE: "Akun Anda tidak aktif. Hubungi administrator.",
  RATE_LIMITED: "Terlalu banyak percobaan dari perangkat ini.",
  MFA_CODE_INVALID:
    "Kode verifikasi tidak valid. Periksa kode atau tunggu kode berikutnya.",
  MFA_CHALLENGE_EXPIRED:
    "Sesi verifikasi berakhir. Masuk ulang dengan kata sandi Anda.",
  PASSWORD_CHANGE_REQUIRED:
    "Anda harus mengganti kata sandi sebelum melanjutkan.",
};

/** Galat yang lahir di form (validasi isian), bukan dari server. */
export function galatLokal(pesan: string): GalatAuth {
  return { pesan, ulangMasuk: false, tautanBaru: false };
}

/** Pesan umum saat jawaban server tidak bisa dijelaskan (galat 5xx, jaringan putus). */
export const PESAN_JARINGAN = "Tidak dapat terhubung ke server. Coba lagi.";
/** Pesan saat hitung mundur selesai. */
export const PESAN_TUNGGU_SELESAI = "Waktu tunggu selesai. Silakan coba lagi.";

/** Kode yang berarti "tunggu": pesannya diikuti sisa waktu. */
const KODE_TUNGGU: ReadonlySet<string> = new Set([
  KODE_GALAT.AKUN_TERKUNCI,
  KODE_GALAT.TERLALU_BANYAK,
]);

const KODE_DIKENAL: ReadonlySet<string> = new Set(Object.values(KODE_GALAT));

/** Kode mesin stabil (HURUF_BESAR_GARIS_BAWAH) dari server, atau undefined untuk kode yang hanya mengulang status ("401"). */
export function kodeStabil(kode: string | undefined): string | undefined {
  return kode !== undefined && /^[A-Z][A-Z0-9_]+$/.test(kode) ? kode : undefined;
}

export function adalahKodeDikenal(kode: string | undefined): kode is KodeGalat {
  return kode !== undefined && KODE_DIKENAL.has(kode);
}

/** Layar tempat galat muncul; status yang sama bisa berarti hal berbeda (401 saat masuk ≠ 401 saat atur ulang). */
export type KonteksAuth = "masuk" | "mfa" | "lupa" | "reset" | "ganti";

/** Apa yang diketahui tentang kegagalan: dari jawaban API Go, BFF, atau parameter URL. */
export interface MasukanGalat {
  status?: number;
  /** Kode dari server, apa adanya (bisa kode stabil, bisa status sebagai teks seperti "401"). */
  kode?: string;
  /** Pesan dari server. */
  pesan?: string;
  /** Sisa waktu tunggu dalam detik (kunci akun, batas laju). */
  detik?: number;
}

export interface GalatAuth {
  kode?: string;
  status?: number;
  /** Pesan tanpa bagian hitung mundur; tampilkan lewat `pesanTampil`. */
  pesan: string;
  /** Akhir waktu tunggu (epoch milidetik) untuk kode yang menunggu. */
  sampaiMs?: number;
  /** True bila sesi verifikasi tak bisa dilanjutkan dan pengguna harus mengulang dari kata sandi. */
  ulangMasuk: boolean;
  /** True bila tautan atur ulang tak bisa dipakai lagi: tawarkan minta tautan baru. */
  tautanBaru: boolean;
}

// ── Waktu ─────────────────────────────────────────────────────────────

/** Sisa detik (dibulatkan ke atas, minimal 0) sampai `sampaiMs`. */
export function sisaDetik(sampaiMs: number, sekarangMs: number): number {
  return Math.max(0, Math.ceil((sampaiMs - sekarangMs) / 1000));
}

/** "45 detik", "14 menit", "14 menit 52 detik", "1 jam 5 menit". */
export function formatSisaWaktu(detik: number): string {
  const total = Math.max(0, Math.ceil(detik));
  if (total < 60) return `${total} detik`;
  const jam = Math.floor(total / 3600);
  const menit = Math.floor((total % 3600) / 60);
  const sisa = total % 60;
  if (jam > 0) return menit === 0 ? `${jam} jam` : `${jam} jam ${menit} menit`;
  return sisa === 0 ? `${menit} menit` : `${menit} menit ${sisa} detik`;
}

/** Bentuk ringkas untuk lencana tabel: "45 dtk", "14 mnt", "1 jam 5 mnt". */
export function formatSisaSingkat(detik: number): string {
  const total = Math.max(0, Math.ceil(detik));
  if (total < 60) return `${total} dtk`;
  const jam = Math.floor(total / 3600);
  const menit = Math.ceil((total % 3600) / 60);
  if (jam > 0)
    return menit === 0 || menit === 60
      ? `${jam + (menit === 60 ? 1 : 0)} jam`
      : `${jam} jam ${menit} mnt`;
  return `${Math.ceil(total / 60)} mnt`;
}

// ── Pemetaan ──────────────────────────────────────────────────────────

/** Menebak kode dari status dan konteks bila server tidak mengirim kode stabil (API lama). */
function simpulkanKode(
  m: MasukanGalat,
  konteks: KonteksAuth,
): KodeGalat | undefined {
  switch (m.status) {
    case 429:
      return KODE_GALAT.TERLALU_BANYAK;
    case 423:
      return KODE_GALAT.AKUN_TERKUNCI;
    case 401:
      if (konteks === "masuk") return KODE_GALAT.KREDENSIAL_SALAH;
      if (konteks === "mfa") {
        return /masuk ulang/i.test(m.pesan ?? "")
          ? KODE_GALAT.MFA_SESI_BERAKHIR
          : KODE_GALAT.MFA_KODE_SALAH;
      }
      return undefined;
    case 403:
      return konteks === "masuk" ? KODE_GALAT.AKUN_NONAKTIF : undefined;
    default:
      return undefined;
  }
}

/**
 * Menerjemahkan kegagalan menjadi galat yang siap ditampilkan. Kode stabil dari server menang; tanpa kode,
 * status dan konteks dipakai; selebihnya pesan server (bila galat klien) atau pesan jaringan.
 */
export function galatAuth(
  masukan: MasukanGalat,
  konteks: KonteksAuth,
  sekarangMs: number = Date.now(),
): GalatAuth {
  const kode: string | undefined = adalahKodeDikenal(masukan.kode)
    ? masukan.kode
    : simpulkanKode(masukan, konteks);

  if (adalahKodeDikenal(kode)) {
    const menunggu =
      KODE_TUNGGU.has(kode) && masukan.detik !== undefined && masukan.detik > 0;
    return {
      kode,
      status: masukan.status,
      pesan: PESAN_GALAT[kode],
      sampaiMs: menunggu ? sekarangMs + masukan.detik! * 1000 : undefined,
      ulangMasuk: kode === KODE_GALAT.MFA_SESI_BERAKHIR,
      tautanBaru: false,
    };
  }

  const dasar = {
    kode: masukan.kode,
    status: masukan.status,
    ulangMasuk: false,
  };
  if (konteks === "reset" && masukan.status === 401) {
    return {
      ...dasar,
      pesan:
        masukan.pesan ||
        "Tautan atur ulang tidak valid atau sudah tidak berlaku.",
      tautanBaru: true,
    };
  }
  if (masukan.status !== undefined && masukan.status < 500 && masukan.pesan) {
    return { ...dasar, pesan: masukan.pesan, tautanBaru: false };
  }
  if (masukan.status !== undefined && masukan.status < 500) {
    return {
      ...dasar,
      pesan: "Permintaan tidak dapat diproses.",
      tautanBaru: false,
    };
  }
  return { ...dasar, pesan: PESAN_JARINGAN, tautanBaru: false };
}

/** Pesan terbagi supaya sisa waktu yang berjalan bisa dirender di elemen tersendiri (role="timer"). */
export interface BagianPesan {
  /** Kalimat tetap. */
  teks: string;
  /** Sisa waktu yang sedang berjalan ("14 menit 52 detik"); null bila tidak ada hitung mundur. */
  sisa: string | null;
}

/**
 * Pesan pada `sekarangMs`. `null` berarti "belum diketahui" (render server dan hidrasi pertama): hitung mundur
 * ditunda supaya HTML server dan klien sama persis.
 */
export function bagianPesan(
  galat: GalatAuth,
  sekarangMs: number | null = Date.now(),
): BagianPesan {
  if (!galat.kode || !KODE_TUNGGU.has(galat.kode))
    return { teks: galat.pesan, sisa: null };
  if (galat.sampaiMs === undefined)
    return { teks: `${galat.pesan} Coba lagi nanti.`, sisa: null };
  if (sekarangMs === null) return { teks: galat.pesan, sisa: null };
  const sisa = sisaDetik(galat.sampaiMs, sekarangMs);
  return sisa > 0
    ? { teks: galat.pesan, sisa: formatSisaWaktu(sisa) }
    : { teks: PESAN_TUNGGU_SELESAI, sisa: null };
}

/** Teks yang ditampilkan pada `sekarangMs`: menambahkan sisa waktu untuk galat yang menunggu. */
export function pesanTampil(
  galat: GalatAuth,
  sekarangMs: number | null = Date.now(),
): string {
  const { teks, sisa } = bagianPesan(galat, sekarangMs);
  return sisa ? `${teks} Coba lagi dalam ${sisa}.` : teks;
}

/** True selama galat masih menahan pengguna (hitung mundur belum selesai). */
export function masihMenunggu(
  galat: GalatAuth | null,
  sekarangMs: number = Date.now(),
): boolean {
  return (
    !!galat &&
    galat.sampaiMs !== undefined &&
    sisaDetik(galat.sampaiMs, sekarangMs) > 0
  );
}

// ── Pembacaan dari galat klien dan parameter URL ───────────────────────

/**
 * Mengambil masukan galat dari exception `bffFetch` (ApiError: status, message, payload dari BFF).
 * Struktural, supaya modul ini tidak bergantung pada lib/api/client.
 */
export function masukanDariError(err: unknown): MasukanGalat {
  if (!(err instanceof Error)) return {};
  const status = (err as { status?: unknown }).status;
  const payload = (err as { payload?: unknown }).payload as
    { kode?: unknown; sisa_detik?: unknown } | undefined;
  return {
    status: typeof status === "number" ? status : undefined,
    pesan: err.message,
    kode: typeof payload?.kode === "string" ? payload.kode : undefined,
    detik:
      typeof payload?.sisa_detik === "number" ? payload.sisa_detik : undefined,
  };
}

/** Parameter URL halaman masuk setelah gagal kirim form: `?error=…&kode=…&sampai=<epoch detik>`. */
export interface ParamGalat {
  error?: string;
  kode?: string;
  sampai?: string;
}

/** Batas wajar `sampai` terhadap sekarang, supaya parameter ngawur tidak menahan form berhari-hari. */
const BATAS_SAMPAI_DETIK = 24 * 3600;

/** Galat awal halaman masuk dari parameter URL, atau null bila tidak ada galat. */
export function galatDariParam(
  param: ParamGalat,
  sekarangMs: number = Date.now(),
): GalatAuth | null {
  let detik: number | undefined;
  if (param.sampai !== undefined && /^\d{1,12}$/.test(param.sampai)) {
    const sisa = Number(param.sampai) - sekarangMs / 1000;
    if (sisa <= BATAS_SAMPAI_DETIK) detik = Math.max(0, Math.ceil(sisa));
  }
  if (adalahKodeDikenal(param.kode)) {
    const galat = galatAuth({ kode: param.kode, detik }, "masuk", sekarangMs);
    // Penunjuk waktu absolut dipakai apa adanya, supaya muat ulang halaman tidak mengulang hitungan.
    if (KODE_TUNGGU.has(param.kode) && detik !== undefined) {
      return { ...galat, sampaiMs: Number(param.sampai) * 1000 };
    }
    return galat;
  }
  if (param.error)
    return { pesan: param.error, ulangMasuk: false, tautanBaru: false };
  return null;
}

/** Query halaman masuk untuk galat ini (alihan BFF setelah kiriman form gagal). */
export function paramDariGalat(
  galat: GalatAuth,
  sekarangMs: number = Date.now(),
): string {
  const q = new URLSearchParams({ error: pesanTampil(galat, sekarangMs) });
  if (galat.kode) q.set("kode", galat.kode);
  if (galat.sampaiMs !== undefined)
    q.set("sampai", String(Math.ceil(galat.sampaiMs / 1000)));
  return q.toString();
}
