import { ShieldCheck } from "lucide-react";
import Link from "next/link";
import { BrandMark } from "@/components/layout/brand-mark";
import { ambilTokenAktivasi } from "@/lib/auth/aktivasi";
import { AktivasiForm } from "./aktivasi-form";

export const metadata = { title: "Aktivasi Akun" };

/** Tujuan tautan di email undangan: token dari query, kata sandi pertama dari pemilik akun. */
export default async function AktivasiPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string | string[] }>;
}) {
  const token = ambilTokenAktivasi((await searchParams).token);

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-8">
      <div className="glass-strong w-full max-w-[420px] rounded-xl p-6 sm:p-8">
        <BrandMark className="size-11" />
        <h1 className="mt-5 text-display-md font-bold text-navy-900 dark:text-white">
          Aktifkan akun Anda
        </h1>

        {token ? (
          <>
            <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
              Administrator telah membuatkan akun untuk Anda. Tetapkan kata
              sandi yang hanya Anda ketahui. Tautan ini hanya dapat dipakai satu
              kali.
            </p>
            <AktivasiForm token={token} />
          </>
        ) : (
          <div role="alert" className="mt-4 space-y-3 text-sm leading-relaxed">
            <p className="text-muted-foreground">
              Tautan tidak lengkap. Buka tautan langsung dari email undangan,
              atau minta administrator mengirim undangan baru.
            </p>
            <Link
              href="/masuk"
              className="font-medium text-data-700 underline-offset-4 hover:underline"
            >
              Ke halaman masuk
            </Link>
          </div>
        )}

        <div className="mt-5 flex gap-2.5 rounded-xl bg-data-50/70 p-3.5 text-xs leading-relaxed text-data-800 dark:bg-data-500/10 dark:text-data-200">
          <ShieldCheck className="mt-px size-4 shrink-0" />
          <p>
            Setelah akun aktif, masuk dengan email undangan dan kata sandi yang
            baru Anda tetapkan.
          </p>
        </div>
      </div>
    </main>
  );
}
