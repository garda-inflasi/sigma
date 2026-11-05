import { ShieldCheck } from "lucide-react";
import { BrandMark } from "@/components/layout/brand-mark";
import { safeInternalPath } from "@/lib/auth/safe-path";
import { pesanGalatSso } from "@/lib/auth/sso";
import { ssoGoogleAktif } from "@/lib/bff/sso-api";
import { MasukForm } from "./masuk-form";
import { MfaOtpForm } from "./mfa-otp-form";
import { SsoGoogle } from "./sso-google";

export const metadata = { title: "Masuk" };

/**
 * Masuk ke Garda Inflasi — email + kata sandi, lalu kode OTP bila akun ber-MFA.
 * Lapis auth sesungguhnya dijalankan layanan Go (kata sandi + MFA wajib untuk
 * lapis strategis & manajerial); halaman ini hanya gerbang masuk.
 */
export default async function MasukPage({
  searchParams,
}: {
  searchParams: Promise<{ lanjut?: string; mfa?: string; error?: string }>;
}) {
  const { lanjut, mfa, error } = await searchParams;
  const langkahOtp = mfa === "1";
  const redirect = safeInternalPath(lanjut);
  // Tombol Google hanya bila API melaporkan SSO terkonfigurasi (dicek di server pada tiap render).
  const tampilkanSso = !langkahOtp && (await ssoGoogleAktif());

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-8">
      <div className="glass-strong w-full max-w-[420px] rounded-xl p-6 sm:p-8">
        <BrandMark className="size-11" />
        <h1 className="mt-5 text-display-md font-bold text-navy-900 dark:text-white">
          {langkahOtp ? "Verifikasi dua langkah" : "Masuk ke Garda Inflasi"}
        </h1>
        <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
          {langkahOtp
            ? "Masukkan kode 6 digit dari aplikasi autentikator Anda, atau salah satu kode cadangan."
            : "Masuk dengan akun institusi Anda. Akses mengikuti peran yang ditetapkan oleh Administrator Sistem."}
        </p>

        {langkahOtp ? <MfaOtpForm redirect={redirect} /> : <MasukForm redirect={redirect} galatAwal={pesanGalatSso(error) ?? error} />}

        {tampilkanSso && <SsoGoogle lanjut={redirect} />}

        <div className="mt-5 flex gap-2.5 rounded-xl bg-data-50/70 p-3.5 text-xs leading-relaxed text-data-800 dark:bg-data-500/10 dark:text-data-200">
          <ShieldCheck className="mt-px size-4 shrink-0" />
          <p>
            Verifikasi dua langkah wajib untuk lapis strategis &amp; manajerial.
            Seluruh aktivitas tercatat sesuai UU PDP.
          </p>
        </div>
      </div>
    </main>
  );
}
