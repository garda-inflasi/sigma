import { FormGantiSandi } from "@/components/akun/form-ganti-sandi";
import { BrandMark } from "@/components/layout/brand-mark";
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
    <main className="grid min-h-dvh place-items-center px-4 py-8">
      <div className="glass-strong w-full max-w-[420px] rounded-xl p-6 sm:p-8">
        <BrandMark className="size-11" />
        <h1 className="mt-5 text-display-md font-bold text-navy-900 dark:text-white">
          Ganti kata sandi
        </h1>
        <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
          Kata sandi Anda ditetapkan oleh pihak lain. Buat kata sandi baru yang
          hanya Anda ketahui sebelum melanjutkan.
        </p>

        <div className="mt-6">
          <FormGantiSandi mode={{ jenis: "halaman", lanjut: redirect }} />
        </div>
      </div>
    </main>
  );
}
