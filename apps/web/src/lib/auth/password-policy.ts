/**
 * Kebijakan kata sandi, cermin dari `ValidatePassword` di API Go
 * (apps/api/internal/auth/usecase). Dipakai untuk umpan balik langsung di form;
 * keputusan akhir tetap di server.
 */

export const MIN_PASSWORD_LENGTH = 8;

/** Kata sandi yang ditolak server karena terlalu umum (huruf kecil semua). */
const DENY_LIST = new Set([
  "password",
  "12345678",
  "123456789",
  "qwerty123",
  "admin123",
  "letmein",
  "welcome",
  "monkey",
  "dragon",
  "master",
  "acehkukaya",
]);

export interface PasswordRule {
  id: "panjang" | "kapital" | "angka" | "khusus" | "umum";
  label: string;
  terpenuhi: boolean;
}

export function evaluatePassword(password: string): PasswordRule[] {
  return [
    {
      id: "panjang",
      label: `Minimal ${MIN_PASSWORD_LENGTH} karakter`,
      terpenuhi: password.length >= MIN_PASSWORD_LENGTH,
    },
    {
      id: "kapital",
      label: "Mengandung huruf kapital",
      terpenuhi: /[A-Z]/.test(password),
    },
    {
      id: "angka",
      label: "Mengandung angka",
      terpenuhi: /[0-9]/.test(password),
    },
    {
      id: "khusus",
      label: "Mengandung karakter khusus",
      terpenuhi: /[^a-zA-Z0-9]/.test(password),
    },
    {
      id: "umum",
      label: "Bukan kata sandi yang terlalu umum",
      terpenuhi: password !== "" && !DENY_LIST.has(password.toLowerCase()),
    },
  ];
}

export function isPasswordValid(password: string): boolean {
  return evaluatePassword(password).every((r) => r.terpenuhi);
}
