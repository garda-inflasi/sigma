import { ShieldCheck } from "lucide-react";
import { BrandMark } from "@/components/layout/brand-mark";
import { PASSWORD_CHANGE_PATH } from "@/lib/auth/password-change";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";
import { GantiPasswordForm } from "./ganti-password-form";

export const metadata = { title: "Ganti Kata Sandi" };

/**
 * Ganti kata sandi. Wajib dilalui akun yang kata sandinya ditetapkan pihak lain
 * (admin awal, akun baru, reset oleh administrator); juga dipakai sukarela dari menu akun.
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

        <GantiPasswordForm redirect={redirect} />

        <div className="mt-5 flex gap-2.5 rounded-xl bg-data-50/70 p-3.5 text-xs leading-relaxed text-data-800 dark:bg-data-500/10 dark:text-data-200">
          <ShieldCheck className="mt-px size-4 shrink-0" />
          <p>
            Setelah kata sandi diganti, sesi di perangkat lain akan diakhiri.
          </p>
        </div>
      </div>
    </main>
  );
}
