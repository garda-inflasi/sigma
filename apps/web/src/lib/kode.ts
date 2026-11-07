/**
 * Kode tampilan entitas yang PK-nya bukan lagi teks bermakna (alert EWS, aturan ambang, kartu intervensi).
 * API mengirim `id` (angka desimal atau UUID) untuk tautan dan kunci, serta `kode` (mis. EWS-0901, INT-301) untuk
 * manusia. Teks yang dibaca pengguna memakai `kode ?? id` supaya UI tidak tiba-tiba menampilkan UUID atau angka
 * mentah di tempat yang dulu menampilkan kode.
 */
export function labelKode(x: { id: string; kode?: string | null }): string {
  return x.kode ?? x.id;
}
