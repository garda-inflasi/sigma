/** Ekspor CSV di sisi klien (UTF-8 BOM agar terbaca benar di Excel berlokal Indonesia). */
export function exportCsv<T>(rows: T[], kolom: { header: string; value: (r: T) => string | number | null | undefined }[], namaFile: string) {
  const esc = (v: unknown) => {
    const s = v == null ? "" : String(v);
    return /[;"\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  const lines = [kolom.map((k) => esc(k.header)).join(";"), ...rows.map((r) => kolom.map((k) => esc(k.value(r))).join(";"))];
  const blob = new Blob(["\ufeff" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = Object.assign(document.createElement("a"), { href: url, download: `${namaFile}-${new Date().toISOString().slice(0, 10)}.csv` });
  a.click();
  URL.revokeObjectURL(url);
}
