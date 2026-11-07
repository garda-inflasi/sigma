/**
 * Entitas master yang punya halaman `/master/[entitas]`. Antrean persetujuan `/master/perubahan` sengaja
 * BUKAN entitas: ia rute statis tersendiri (menang atas rute dinamis) dan tidak boleh masuk `generateStaticParams`.
 */
export const ENTITAS = {
  komoditas: "Komoditas",
  wilayah: "Wilayah (kode BPS)",
  pasar: "Pasar",
  kecamatan: "Kecamatan",
  satuan: "Satuan",
  pedagang: "Pedagang",
} as const;
export type Entitas = keyof typeof ENTITAS;

/** Segmen URL yang sah sebagai entitas; `Object.hasOwn` agar "constructor" atau "toString" tidak lolos. */
export const adalahEntitas = (nilai: string): nilai is Entitas =>
  Object.hasOwn(ENTITAS, nilai);
