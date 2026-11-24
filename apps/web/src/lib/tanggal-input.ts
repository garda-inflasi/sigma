/**
 * Konversi isian tanggal. Nilai form dan API tetap memakai format `<input type="date">` (`YYYY-MM-DD`) dan
 * `datetime-local` (`YYYY-MM-DDTHH:mm:ss`, waktu setempat); pengguna melihat dan mengetik `hh/bb/tttt`.
 */

const dua = (n: number) => String(n).padStart(2, "0");

function buatTanggal(tahun: number, bulan: number, hari: number): Date | null {
  if (tahun < 1900 || tahun > 2999) return null;
  const d = new Date(tahun, bulan - 1, hari);
  return d.getFullYear() === tahun &&
    d.getMonth() === bulan - 1 &&
    d.getDate() === hari
    ? d
    : null;
}

/** Date → `YYYY-MM-DD` menurut waktu setempat. */
export function isoDariDate(d: Date): string {
  return `${d.getFullYear()}-${dua(d.getMonth() + 1)}-${dua(d.getDate())}`;
}

/** Bagian tanggal dari nilai form → Date tengah malam setempat; undefined bila kosong atau tidak valid. */
export function dateDariIso(nilai?: string | null): Date | undefined {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(nilai ?? "");
  if (!m) return undefined;
  return buatTanggal(Number(m[1]), Number(m[2]), Number(m[3])) ?? undefined;
}

/** `YYYY-MM-DD` → `hh/bb/tttt`. Nilai yang tidak dikenali ditampilkan apa adanya. */
export function tampilTanggal(nilai: string): string {
  const d = dateDariIso(nilai);
  if (!d) return nilai;
  return `${dua(d.getDate())}/${dua(d.getMonth() + 1)}/${d.getFullYear()}`;
}

/** Jam dari nilai datetime-local sebagai `HH:mm:ss`; kosong bila tidak ada. */
export function bagianWaktu(nilai: string): string {
  const m = /T(\d{2}):(\d{2})(?::(\d{2}))?/.exec(nilai);
  return m ? `${m[1]}:${m[2]}:${m[3] ?? "00"}` : "";
}

/** `YYYY-MM-DDTHH:mm:ss` → `hh/bb/tttt HH:mm:ss`. */
export function tampilTanggalWaktu(nilai: string): string {
  const jam = bagianWaktu(nilai);
  return dateDariIso(nilai) && jam ? `${tampilTanggal(nilai)} ${jam}` : nilai;
}

/** Ketikan `hh/bb/tttt` (pemisah `/`, `-`, atau `.`) atau `tttt-bb-hh` → `YYYY-MM-DD`; null bila bukan tanggal nyata. */
export function bacaKetikanTanggal(teks: string): string | null {
  const t = teks.trim();
  const lokal = /^(\d{1,2})[/.-](\d{1,2})[/.-](\d{4})$/.exec(t);
  const iso = /^(\d{4})-(\d{1,2})-(\d{1,2})$/.exec(t);
  const d = lokal
    ? buatTanggal(Number(lokal[3]), Number(lokal[2]), Number(lokal[1]))
    : iso
      ? buatTanggal(Number(iso[1]), Number(iso[2]), Number(iso[3]))
      : null;
  return d ? isoDariDate(d) : null;
}

/** Ketikan tanggal dan jam (pemisah jam `:` atau `.`, detik opsional) → `YYYY-MM-DDTHH:mm:ss`. */
export function bacaKetikanTanggalWaktu(teks: string): string | null {
  const m = /^(\S+?)[T\s]+(\d{1,2})[:.](\d{2})(?:[:.](\d{2}))?$/.exec(
    teks.trim(),
  );
  if (!m) return null;
  const tanggal = bacaKetikanTanggal(m[1]);
  const [jam, menit, detik] = [Number(m[2]), Number(m[3]), Number(m[4] ?? 0)];
  if (!tanggal || jam > 23 || menit > 59 || detik > 59) return null;
  return `${tanggal}T${dua(jam)}:${dua(menit)}:${dua(detik)}`;
}

/** Menyisipkan `/` saat angka diketik maju. Menghapus dan teks berformat lain dibiarkan. */
export function rapikanKetikanTanggal(baru: string, lama: string): string {
  if (baru.length <= lama.length) return baru;
  // `2026-` yang sempat tersisip garis miring dikembalikan ke format tahun lebih dulu.
  const tahunDulu = /^(\d{2})\/(\d{2})\/-$/.exec(baru);
  if (tahunDulu && Number(tahunDulu[2]) > 12)
    return `${tahunDulu[1]}${tahunDulu[2]}-`;
  // Pemisah yang diketik tepat setelah garis miring otomatis tidak digandakan.
  if (/\/[/.-]$/.test(baru)) return baru.slice(0, -1);
  if (/^\d{3,8}$/.test(baru))
    return [baru.slice(0, 2), baru.slice(2, 4), baru.slice(4)]
      .filter(Boolean)
      .join("/");
  return /^\d{2}$/.test(baru) || /^\d{1,2}\/\d{2}$/.test(baru)
    ? `${baru}/`
    : baru;
}

/** Pesan bila tanggal di luar batas `min`/`max` (`YYYY-MM-DD`); null bila boleh. */
export function galatRentangTanggal(
  iso: string,
  batas: { min?: string; max?: string },
): string | null {
  const tanggal = iso.slice(0, 10);
  if (batas.min && tanggal < batas.min)
    return `Tanggal paling awal ${tampilTanggal(batas.min)}.`;
  if (batas.max && tanggal > batas.max)
    return `Tanggal paling akhir ${tampilTanggal(batas.max)}.`;
  return null;
}
