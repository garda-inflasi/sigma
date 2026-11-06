/**
 * Pembantu e2e alur autentikasi (dipakai auth.spec.ts, boleh dipakai spec lain):
 *  - TOTP RFC 6238 untuk akun ber-MFA,
 *  - klien HTTP Mailpit (kotak masuk email demo) untuk mengambil tautan atur ulang kata sandi,
 *  - anggaran laju BFF, dan
 *  - pembuatan/penghapusan pengguna uji sementara lewat API.
 */
import { createHmac, randomBytes } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { request, type APIRequestContext } from "@playwright/test";
import { apiSebagai, BASE_URL } from "./fixtures";

// ── TOTP (RFC 6238, HMAC-SHA1, 6 digit, langkah 30 detik) ────────────────────

const ABJAD_BASE32 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
export const PERIODE_TOTP_DETIK = 30;

/** Rahasia TOTP akun demo `mfa@gardainflasi.id` (apps/api/internal/auth/seeder/demo.go). */
export const RAHASIA_MFA_DEMO = "JBSWY3DPEHPK3PXP";

/** Mendekode base32 RFC 4648 (huruf besar/kecil, padding `=` dan spasi diabaikan). */
export function dekodeBase32(teks: string): Buffer {
  let bit = 0;
  let nilai = 0;
  const keluar: number[] = [];
  for (const ch of teks.toUpperCase().replace(/[=\s]/g, "")) {
    const idx = ABJAD_BASE32.indexOf(ch);
    if (idx < 0) throw new Error(`Karakter base32 tidak sah: ${ch}`);
    nilai = (nilai << 5) | idx;
    bit += 5;
    if (bit >= 8) {
      keluar.push((nilai >>> (bit - 8)) & 0xff);
      bit -= 8;
    }
  }
  return Buffer.from(keluar);
}

/** Kode TOTP untuk `rahasia` (base32) pada waktu `waktuMs` (epoch milidetik, bawaan sekarang). */
export function kodeTotp(
  rahasia: string,
  waktuMs: number = Date.now(),
  digit = 6,
): string {
  const langkah = Math.floor(waktuMs / 1000 / PERIODE_TOTP_DETIK);
  const penghitung = Buffer.alloc(8);
  penghitung.writeBigUInt64BE(BigInt(langkah));
  const hmac = createHmac("sha1", dekodeBase32(rahasia))
    .update(penghitung)
    .digest();
  const offset = hmac[hmac.length - 1] & 0x0f;
  const biner =
    ((hmac[offset] & 0x7f) << 24) |
    (hmac[offset + 1] << 16) |
    (hmac[offset + 2] << 8) |
    hmac[offset + 3];
  return String(biner % 10 ** digit).padStart(digit, "0");
}

/** Milidetik sampai langkah TOTP berikutnya dimulai. */
export function msSampaiLangkahBerikut(waktuMs: number = Date.now()): number {
  const periode = PERIODE_TOTP_DETIK * 1000;
  return periode - (waktuMs % periode);
}

// ── Mailpit ────────────────────────────────────────────────────────────────

/** Alamat UI/API Mailpit yang dinyalakan `make docker/demo/up` (hanya loopback host). */
export const MAILPIT_URL = (
  process.env.E2E_MAILPIT_URL ?? "http://localhost:8025"
).replace(/\/+$/, "");

/** Gagal dengan pesan yang jelas bila Mailpit tidak terjangkau (mis. stack lama tanpa profil `mailpit`). */
export async function pastikanMailpit(): Promise<void> {
  try {
    const res = await fetch(`${MAILPIT_URL}/api/v1/info`, {
      signal: AbortSignal.timeout(5_000),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
  } catch (err) {
    throw new Error(
      `Mailpit tidak terjangkau di ${MAILPIT_URL} (${(err as Error).message}). ` +
        `Nyalakan demo dengan "make docker/demo/up" (menyalakan Mailpit dan mengarahkan SMTP API ke sana) ` +
        `atau set E2E_MAILPIT_URL ke alamat Mailpit yang menerima email API.`,
    );
  }
}

export interface EmailDiterima {
  id: string;
  subjek: string;
  /** HTML dan teks pesan digabung, untuk dicari polanya. */
  isi: string;
}

/** Menunggu email terbaru untuk `penerima` muncul di Mailpit. */
export async function tungguEmail(
  penerima: string,
  batasMs = 15_000,
): Promise<EmailDiterima> {
  const akhir = Date.now() + batasMs;
  const query = encodeURIComponent(`to:${penerima}`);
  for (;;) {
    const cari = await fetch(`${MAILPIT_URL}/api/v1/search?query=${query}`);
    if (!cari.ok) throw new Error(`Mailpit search gagal (HTTP ${cari.status})`);
    const { messages } = (await cari.json()) as {
      messages?: { ID: string; Subject: string }[];
    };
    const pesan = messages?.[0];
    if (pesan) {
      const detail = await fetch(`${MAILPIT_URL}/api/v1/message/${pesan.ID}`);
      if (!detail.ok)
        throw new Error(`Mailpit message gagal (HTTP ${detail.status})`);
      const { HTML, Text } = (await detail.json()) as {
        HTML?: string;
        Text?: string;
      };
      return {
        id: pesan.ID,
        subjek: pesan.Subject,
        isi: `${HTML ?? ""}\n${Text ?? ""}`,
      };
    }
    if (Date.now() > akhir)
      throw new Error(
        `Tidak ada email untuk ${penerima} di Mailpit (${MAILPIT_URL}) dalam ${batasMs} ms. ` +
          `Pastikan API/worker memakai SMTP_HOST=mailpit (make docker/demo/up).`,
      );
    await new Promise((r) => setTimeout(r, 500));
  }
}

/** Mengambil token dari tautan `/reset-password?token=…` di isi email; path relatif agar lepas dari APP_PUBLIC_URL. */
export function tautanReset(isi: string): { path: string; token: string } {
  const cocok = isi.match(/\/reset-password\?token=([A-Za-z0-9%._~-]+)/);
  if (!cocok)
    throw new Error(
      `Tautan /reset-password?token=… tidak ada di email: ${isi.slice(0, 300)}`,
    );
  return {
    path: `/reset-password?token=${cocok[1]}`,
    token: decodeURIComponent(cocok[1]),
  };
}

/** Mengambil token dari tautan `/aktivasi?token=…` di isi email undangan; path relatif agar lepas dari APP_PUBLIC_URL. */
export function tautanAktivasi(isi: string): { path: string; token: string } {
  const cocok = isi.match(/\/aktivasi\?token=([A-Za-z0-9%._~-]+)/);
  if (!cocok)
    throw new Error(
      `Tautan /aktivasi?token=… tidak ada di email: ${isi.slice(0, 300)}`,
    );
  return {
    path: `/aktivasi?token=${cocok[1]}`,
    token: decodeURIComponent(cocok[1]),
  };
}

/** Menghapus semua email untuk `penerima` (upaya terbaik; kegagalan diabaikan). */
export async function hapusEmail(penerima: string): Promise<void> {
  await fetch(
    `${MAILPIT_URL}/api/v1/search?query=${encodeURIComponent(`to:${penerima}`)}`,
    { method: "DELETE" },
  ).catch(() => undefined);
}

// ── Anggaran laju BFF ──────────────────────────────────────────────────────
//
// Route BFF memakai satu jendela geser 60 detik per IP (lib/bff/rate-limit.ts, kunci `bff:<ip>`):
//   - POST /api/auth/login dan /api/auth/mfa/validate diizinkan bila isi jendela < 5,
//   - POST /api/auth/forgot-password diizinkan bila isi jendela < 3 (jendela yang sama, tercampur
//     dengan percobaan login),
//   - POST /api/auth/aktivasi (kata sandi pertama dari tautan undangan) memakai batas login (< 5).
// Setiap permintaan yang lolos menambah satu entri. Semua tes dari mesin ini datang dari IP yang sama,
// jadi `pakai` mencatat tiap percobaan dan menunggu deterministik sampai jendela cukup longgar.
// Catatan disimpan di berkas sementara agar dua kali jalan berurutan (< 1 menit) saling mengetahui.

export type JenisBff = "login" | "mfa" | "lupa" | "aktivasi";

const BERKAS_ANGGARAN = join(tmpdir(), "garda-e2e-bff-laju.json");
const JENDELA_MS = 60_000;
/** Cadangan atas selisih jam dan latensi antara pencatatan di sini dan di server. */
const MARGIN_MS = 3_000;

function muatCatatan(): number[] {
  try {
    const nilai = JSON.parse(readFileSync(BERKAS_ANGGARAN, "utf8")) as unknown;
    return Array.isArray(nilai)
      ? nilai.filter((t): t is number => typeof t === "number")
      : [];
  } catch {
    return [];
  }
}

export const anggaranBff = {
  /**
   * Dipanggil tepat sebelum aksi yang mengenai route BFF berbatas laju: menunggu bila perlu, lalu
   * mencatat satu pemakaian. Panggil sekali per permintaan (termasuk percobaan OTP ulang).
   */
  async pakai(jenis: JenisBff): Promise<void> {
    const maksSebelumnya = jenis === "lupa" ? 2 : 4;
    for (;;) {
      const sekarang = Date.now();
      const aktif = muatCatatan()
        .filter((t) => sekarang - t < JENDELA_MS + MARGIN_MS)
        .sort((a, b) => a - b);
      if (aktif.length <= maksSebelumnya) {
        writeFileSync(BERKAS_ANGGARAN, JSON.stringify([...aktif, sekarang]));
        return;
      }
      // Entri yang harus kedaluwarsa dulu: yang ke-(jumlah - maksSebelumnya) dari yang tertua.
      const penahan = aktif[aktif.length - maksSebelumnya - 1];
      const tunggu = penahan + JENDELA_MS + MARGIN_MS - sekarang + 50;
      console.log(
        `[anggaran-bff] ${jenis}: jendela laju penuh (${aktif.length}/${maksSebelumnya + 1}), menunggu ${Math.ceil(tunggu / 1000)} dtk`,
      );
      await new Promise((r) => setTimeout(r, tunggu));
    }
  },

  /**
   * Mencatat `n` pemakaian jendela laju tanpa menunggu. Untuk tes yang sengaja menghabiskan jendela dengan
   * permintaan langsung ke route BFF (bukan lewat `pakai`): setiap permintaan yang DITERIMA server (bukan
   * 429) dicatat di sini supaya tes sesudahnya menunggu dengan benar lewat `pakai`.
   */
  catat(n = 1): void {
    const sekarang = Date.now();
    const aktif = muatCatatan().filter((t) => sekarang - t < JENDELA_MS + MARGIN_MS);
    for (let i = 0; i < n; i++) aktif.push(sekarang);
    writeFileSync(BERKAS_ANGGARAN, JSON.stringify(aktif));
  },
};

// ── Pengguna uji sementara ─────────────────────────────────────────────────

export interface PenggunaUji {
  id: string;
  email: string;
  /** Kata sandi saat ini (diperbarui pemanggil bila tes mengubahnya). */
  sandi: string;
}

let urutan = 0;

/** Kata sandi acak yang memenuhi kebijakan (kapital, angka, karakter khusus). */
export function sandiAcak(): string {
  return `Uji#${randomBytes(6).toString("hex")}Zq9`;
}

/**
 * Membuat pengguna uji `e2e.<timestamp>.<n>@gardainflasi.id` lewat POST /v1/users sebagai superadmin.
 * Peran OPS_AGREGATOR (bukan wajib MFA, beranda /cold-storage). Server menandainya wajib ganti kata sandi.
 */
export async function buatPenggunaUji(): Promise<PenggunaUji> {
  const penanda = `${Date.now()}.${++urutan}`;
  const pengguna: PenggunaUji = {
    id: `e2e_${penanda.replace(".", "_")}`,
    email: `e2e.${penanda}@gardainflasi.id`,
    sandi: sandiAcak(),
  };
  const api = await apiSebagai("superadmin");
  try {
    const res = await api.post("users", {
      data: {
        id: pengguna.id,
        nama: `Pengguna Uji ${penanda}`,
        email: pengguna.email,
        password: pengguna.sandi,
        peran: "OPS_AGREGATOR",
        lapis: "OPERASIONAL",
        lembaga: "FMIPA USK",
      },
    });
    if (res.status() !== 201)
      throw new Error(
        `Membuat pengguna uji gagal (${res.status()}): ${await res.text()}`,
      );
  } finally {
    await api.dispose();
  }
  return pengguna;
}

/** Menghapus pengguna uji lewat DELETE /v1/users/:id; 404 (sudah hilang) dianggap beres. */
export async function hapusPenggunaUji(pengguna: PenggunaUji): Promise<void> {
  const api = await apiSebagai("superadmin");
  try {
    const res = await api.delete(`users/${pengguna.id}`);
    if (res.status() !== 200 && res.status() !== 404)
      throw new Error(
        `Menghapus pengguna uji ${pengguna.email} gagal (${res.status()}): ${await res.text()}`,
      );
  } finally {
    await api.dispose();
  }
  await hapusEmail(pengguna.email);
}

/** Klien API Go tanpa sesi, untuk endpoint publik (forgot/reset-password). */
export async function apiPublik(): Promise<APIRequestContext> {
  return request.newContext({
    baseURL: `${BASE_URL}/api/v1/`,
    extraHTTPHeaders: { Origin: BASE_URL },
  });
}
