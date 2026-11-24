import { FormGantiSandi } from "@/components/akun/form-ganti-sandi";
import { KartuAuth } from "@/components/auth/kartu-auth";
import { PASSWORD_CHANGE_PATH } from "@/lib/auth/password-change";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";

export const metadata = { title: "Ganti Kata Sandi" };

/**
 * Ganti kata sandi. Wajib dilalui akun yang kata sandinya ditetapkan pihak lain
 * (admin awal, akun baru, reset oleh administrator), yang belum boleh membuka konsol termasuk /profil.
 * Menu akun memakai tab di /profil; formnya komponen yang sama (components/akun).
 */
export default async function GantiPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ lanjut?: string }>;
}) {
  const { lanjut } = await searchParams;
  const tujuan = validateRedirectUrl(lanjut ?? null);
  const redirect =
    tujuan && !tujuan.startsWith(PASSWORD_CHANGE_PATH) ? tujuan : "/";

  return (
    <KartuAuth
      judul="Ganti kata sandi"
      deskripsi="Kata sandi Anda ditetapkan oleh pihak lain. Buat kata sandi baru yang hanya Anda ketahui sebelum melanjutkan."
    >
      <div className="mt-5">
        <FormGantiSandi mode={{ jenis: "halaman", lanjut: redirect }} />
      </div>
    </KartuAuth>
  );
}
