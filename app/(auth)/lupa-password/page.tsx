import { ShieldCheck } from "lucide-react";
import { BrandMark } from "@/components/layout/brand-mark";
import { LupaPasswordForm } from "./lupa-password-form";

export const metadata = { title: "Lupa Kata Sandi" };

/** Pemulihan kata sandi: kirim tautan atur ulang ke email institusi (layanan auth Go). */
export default function LupaPasswordPage() {
  const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  return (
    <main className="grid min-h-dvh place-items-center px-4 py-8">
      <div className="glass-strong w-full max-w-[420px] rounded-xl p-6 sm:p-8">
        <BrandMark className="size-11" />
        <h1 className="mt-5 text-display-md font-bold text-navy-900 dark:text-white">
          Atur ulang kata sandi
        </h1>
        <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
          Masukkan email institusi Anda. Jika terdaftar, kami kirim tautan untuk
          mengatur ulang kata sandi.
        </p>

        <LupaPasswordForm apiBase={apiBase} />

        <div className="mt-5 flex gap-2.5 rounded-xl bg-data-50/70 p-3.5 text-xs leading-relaxed text-data-800 dark:bg-data-500/10 dark:text-data-200">
          <ShieldCheck className="mt-px size-4 shrink-0" />
          <p>
            Demi keamanan, kami tidak mengonfirmasi apakah email terdaftar.
            Tautan atur ulang berlaku 30 menit.
          </p>
        </div>
      </div>
    </main>
  );
}
