import { ArrowRight } from "lucide-react";
import { KartuAuth } from "@/components/auth/kartu-auth";
import { FormAlert } from "@/components/form/form-field";
import { SpringLink } from "@/components/garda/spring-button";
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

  if (!token) {
    return (
      <KartuAuth judul="Buat kata sandi baru">
        <div className="mt-4 space-y-4">
          <FormAlert className="leading-relaxed">
            Tautan tidak lengkap. Buka tautan langsung dari email atur ulang,
            atau minta tautan baru.
          </FormAlert>
          <SpringLink
            href="/lupa-password"
            variant="primary"
            size="lg"
            className="flex h-11 w-full text-[15px]"
          >
            Minta tautan baru <ArrowRight />
          </SpringLink>
        </div>
      </KartuAuth>
    );
  }

  return (
    <KartuAuth
      judul="Buat kata sandi baru"
      deskripsi="Pilih kata sandi baru yang hanya Anda ketahui. Tautan ini hanya dapat dipakai satu kali."
      catatan="Setelah kata sandi diatur ulang, semua sesi di perangkat lain akan diakhiri."
    >
      <ResetPasswordForm token={token} />
    </KartuAuth>
  );
}
