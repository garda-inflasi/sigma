import { ShieldCheck } from "lucide-react";
import { BrandMark } from "@/components/layout/brand-mark";
import { MFA_SETUP_PATH } from "@/lib/auth/mfa";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";
import { MfaPanel } from "./mfa-panel";

export const metadata = { title: "Verifikasi Dua Langkah" };

/**
 * Setup dan pengelolaan MFA. Wajib dilalui akun berperan strategis, manajerial, koordinator, dan admin
 * (saat penegakan MFA menyala); juga dipakai sukarela dari menu akun.
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
    <main className="grid min-h-dvh place-items-center px-4 py-8">
      <div className="glass-strong w-full max-w-[420px] rounded-xl p-6 sm:p-8">
        <BrandMark className="size-11" />
        <h1 className="mt-5 text-display-md font-bold text-navy-900 dark:text-white">
          Verifikasi dua langkah
        </h1>

        <MfaPanel lanjut={redirect} />

        <div className="mt-5 flex gap-2.5 rounded-xl bg-data-50/70 p-3.5 text-xs leading-relaxed text-data-800 dark:bg-data-500/10 dark:text-data-200">
          <ShieldCheck className="mt-px size-4 shrink-0" />
          <p>
            Selain kata sandi, Anda akan diminta kode dari aplikasi autentikator
            setiap kali masuk.
          </p>
        </div>
      </div>
    </main>
  );
}
