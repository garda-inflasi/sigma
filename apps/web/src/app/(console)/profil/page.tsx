import { Suspense } from "react";
import { ProfilPage } from "@/components/profil/profil-page";

export const metadata = { title: "Profil Saya" };

/**
 * Profil terpadu: data diri, ganti kata sandi, verifikasi dua langkah, dan sesi aktif dalam satu halaman
 * konsol. Tanpa izin khusus; cukup sesi yang sah. Tab dipilih lewat `?tab=` (lihat lib/profil/tab.ts).
 */
export default function Page() {
  return (
    <Suspense>
      <ProfilPage />
    </Suspense>
  );
}
