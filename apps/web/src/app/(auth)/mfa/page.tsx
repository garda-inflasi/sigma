import { PanelMfa } from "@/components/akun/panel-mfa";
import { KartuAuth } from "@/components/auth/kartu-auth";
import { MFA_SETUP_PATH } from "@/lib/auth/mfa";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";

export const metadata = { title: "Verifikasi Dua Langkah" };

/**
 * Setup MFA untuk akun berperan strategis, manajerial, koordinator, dan admin (saat penegakan MFA menyala),
 * yang belum boleh membuka konsol termasuk /profil. Menu akun memakai tab di /profil; panelnya komponen
 * yang sama (components/akun).
 */
export default async function MfaPage({
  searchParams,
}: {
  searchParams: Promise<{ lanjut?: string }>;
}) {
  const { lanjut } = await searchParams;
  const tujuan = validateRedirectUrl(lanjut ?? null);
  const redirect = tujuan && !tujuan.startsWith(MFA_SETUP_PATH) ? tujuan : "/";

  return (
    <KartuAuth
      judul="Verifikasi dua langkah"
      catatan="Selain kata sandi, Anda akan diminta kode dari aplikasi autentikator setiap kali masuk."
    >
      <PanelMfa
        mode={{ jenis: "halaman", lanjut: redirect }}
        className="mt-5"
      />
    </KartuAuth>
  );
}
