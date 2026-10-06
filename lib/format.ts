import { format, formatDistanceToNowStrict } from "date-fns";
import { id } from "date-fns/locale";

const rupiah = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });
const angka = new Intl.NumberFormat("id-ID");

export function formatRupiah(n: number, opts: { compact?: boolean } = {}) {
  if (opts.compact) {
    const abs = Math.abs(n);
    if (abs >= 1e12) return `Rp${(n / 1e12).toLocaleString("id-ID", { maximumFractionDigits: 1 })} T`;
    if (abs >= 1e9) return `Rp${(n / 1e9).toLocaleString("id-ID", { maximumFractionDigits: 1 })} M`;
    if (abs >= 1e6) return `Rp${(n / 1e6).toLocaleString("id-ID", { maximumFractionDigits: 1 })} jt`;
  }
  return rupiah.format(Math.round(n)).replace(/\s/g, "");
}

export function formatAngka(n: number, digits = 0) {
  return n.toLocaleString("id-ID", { minimumFractionDigits: digits, maximumFractionDigits: digits });
}

export function formatPersen(n: number, opts: { digits?: number; signed?: boolean } = {}) {
  const { digits = 1, signed = false } = opts;
  const s = Math.abs(n).toLocaleString("id-ID", { minimumFractionDigits: digits, maximumFractionDigits: digits });
  const sign = signed ? (n > 0 ? "+" : n < 0 ? "−" : "") : n < 0 ? "−" : "";
  return `${sign}${s}%`;
}

export function formatTon(n: number) {
  return `${angka.format(Math.round(n))} ton`;
}

export function formatTanggal(d: Date | string, pola = "d MMM yyyy") {
  return format(typeof d === "string" ? new Date(d) : d, pola, { locale: id });
}

export function formatRelatif(d: Date | string) {
  return formatDistanceToNowStrict(typeof d === "string" ? new Date(d) : d, { locale: id, addSuffix: true });
}

/** Parse input "52.500" → 52500 */
export function parseRupiah(s: string) {
  return Number(s.replace(/[^\d]/g, "")) || 0;
}
