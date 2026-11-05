import { ShieldCheck } from "lucide-react";
import Link from "next/link";
import { BrandMark } from "@/components/layout/brand-mark";
import { ambilToken } from "@/lib/auth/reset-password";
import { ResetPasswordForm } from "./reset-password-form";

export const metadata = { title: "Atur Ulang Kata Sandi" };

/** Tujuan tautan di email atur ulang: token dari query, kata sandi baru dari pengguna. */
export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string | string[] }>;
}) {
  const token = ambilToken((await searchParams).token);

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-8">
      <div className="glass-strong w-full max-w-[420px] rounded-xl p-6 sm:p-8">
        <BrandMark className="size-11" />
        <h1 className="mt-5 text-display-md font-bold text-navy-900 dark:text-white">
          Buat kata sandi baru
        </h1>

        {token ? (
          <>
            <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
              Pilih kata sandi baru yang hanya Anda ketahui. Tautan ini hanya
              dapat dipakai satu kali.
            </p>
            <ResetPasswordForm token={token} />
          </>
        ) : (
          <div role="alert" className="mt-4 space-y-3 text-sm leading-relaxed">
            <p className="text-muted-foreground">
              Tautan tidak lengkap. Buka tautan langsung dari email atur ulang,
              atau minta tautan baru.
            </p>
            <Link
              href="/lupa-password"
              className="font-medium text-data-700 underline-offset-4 hover:underline"
            >
              Minta tautan baru
            </Link>
          </div>
        )}

        <div className="mt-5 flex gap-2.5 rounded-xl bg-data-50/70 p-3.5 text-xs leading-relaxed text-data-800 dark:bg-data-500/10 dark:text-data-200">
          <ShieldCheck className="mt-px size-4 shrink-0" />
          <p>
            Setelah kata sandi diatur ulang, semua sesi di perangkat lain akan
            diakhiri.
          </p>
        </div>
      </div>
    </main>
  );
}
