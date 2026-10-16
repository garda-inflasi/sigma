export type PageItem = number | "…";

const range = (from: number, to: number) => Array.from({ length: to - from + 1 }, (_, i) => from + i);

/**
 * Daftar tombol paginasi: halaman pertama dan terakhir selalu tampil, dengan
 * jendela di sekitar halaman aktif dan elipsis di celah. Jumlah item tetap
 * (`siblings * 2 + 5`) sehingga lebar paginasi tidak melompat saat pindah halaman.
 */
export function paginationItems(current: number, total: number, siblings = 1): PageItem[] {
  if (total <= 0) return [];

  const slots = siblings * 2 + 5;
  if (total <= slots) return range(1, total);

  const page = Math.min(Math.max(current, 1), total);
  const left = Math.max(page - siblings, 1);
  const right = Math.min(page + siblings, total);
  const dotsLeft = left > 2;
  const dotsRight = right < total - 1;
  const edge = siblings * 2 + 3;

  if (!dotsLeft) return [...range(1, edge), "…", total];
  if (!dotsRight) return [1, "…", ...range(total - edge + 1, total)];
  return [1, "…", ...range(left, right), "…", total];
}
