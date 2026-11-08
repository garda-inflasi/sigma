/**
 * Tab halaman /profil dan tautan ke tiap tab (`?tab=`). Dipakai halaman profil dan menu akun di topbar,
 * sehingga nama tab dan URL-nya hanya didefinisikan di satu tempat.
 */

export const PROFIL_PATH = "/profil";

export const TAB_PROFIL = [
  "profil",
  "ganti-password",
  "mfa",
  "keamanan",
] as const;
export type TabProfil = (typeof TAB_PROFIL)[number];

/** Tab yang dibuka bila `?tab=` kosong atau tidak dikenal. */
export const TAB_BAWAAN: TabProfil = "profil";

export const LABEL_TAB_PROFIL: Record<TabProfil, string> = {
  profil: "Edit Profil",
  "ganti-password": "Ganti Kata Sandi",
  mfa: "Verifikasi 2 Langkah",
  keamanan: "Perangkat & Sesi Aktif",
};

export function adalahTabProfil(nilai: unknown): nilai is TabProfil {
  return (
    typeof nilai === "string" &&
    (TAB_PROFIL as readonly string[]).includes(nilai)
  );
}

/** Nilai `?tab=` menjadi tab yang sah; kosong atau tidak dikenal jatuh ke tab bawaan. */
export function bacaTabProfil(nilai: string | null | undefined): TabProfil {
  return adalahTabProfil(nilai) ? nilai : TAB_BAWAAN;
}

/** URL tab: tab bawaan tanpa query (`/profil`), tab lain `/profil?tab=<id>`. */
export function hrefTabProfil(tab: TabProfil): string {
  return tab === TAB_BAWAAN ? PROFIL_PATH : `${PROFIL_PATH}?tab=${tab}`;
}
