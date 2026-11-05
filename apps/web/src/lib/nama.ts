/**
 * Inisial untuk avatar dari nama lengkap bergelar, mis. "Prof. Dr.rer.nat. Ilham Maulana, S.Si." → "IM".
 * Gelar setelah koma dibuang, begitu pula kata bertitik di depan nama (Prof., Dr., Ir., Drs., H.).
 * Bila yang tersisa hanya gelar, dipakai huruf pertama nama.
 */
export function inisialNama(nama: string): string {
  const kata = nama
    .split(",")[0]
    .split(/\s+/)
    .filter((k) => /^\p{L}/u.test(k) && !k.includes("."));
  const hasil = kata
    .slice(0, 2)
    .map((k) => k[0])
    .join("");
  return (hasil || (nama.match(/\p{L}/u)?.[0] ?? "")).toUpperCase();
}
