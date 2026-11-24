import { KartuAuth } from "@/components/auth/kartu-auth";
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
    <KartuAuth
      judul={langkahOtp ? "Verifikasi dua langkah" : "Masuk ke Garda Inflasi"}
      deskripsi={
        langkahOtp
          ? "Masukkan kode 6 digit dari aplikasi autentikator Anda, atau salah satu kode cadangan."
          : "Gunakan email dan kata sandi dari institusi Anda."
      }
      catatan={
        langkahOtp
          ? undefined
          : "Verifikasi dua langkah wajib untuk lapis strategis & manajerial. Seluruh aktivitas tercatat sesuai UU PDP."
      }
    >
      {langkahOtp ? (
        <MfaOtpForm redirect={redirect} />
      ) : (
        <MasukForm
          redirect={redirect}
          galatAwal={pesanGalatSso(error) ?? error}
        />
      )}

      {tampilkanSso && <SsoGoogle lanjut={redirect} />}
    </KartuAuth>
  );
}
