import { ArrowRight } from "lucide-react";
import { KartuAuth } from "@/components/auth/kartu-auth";
import { FormAlert } from "@/components/form/form-field";
import { SpringLink } from "@/components/garda/spring-button";
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

  if (!token) {
    return (
      <KartuAuth judul="Aktifkan akun Anda">
        <div className="mt-4 space-y-4">
          <FormAlert className="leading-relaxed">
            Tautan tidak lengkap. Buka tautan langsung dari email undangan, atau
            minta administrator mengirim undangan baru.
          </FormAlert>
          <SpringLink
            href="/masuk"
            variant="primary"
            size="lg"
            className="flex h-11 w-full text-[15px]"
          >
            Ke halaman masuk <ArrowRight />
          </SpringLink>
        </div>
      </KartuAuth>
    );
  }

  return (
    <KartuAuth
      judul="Aktifkan akun Anda"
      deskripsi="Tetapkan kata sandi yang hanya Anda ketahui. Tautan ini hanya dapat dipakai satu kali."
      catatan="Setelah akun aktif, masuk dengan email undangan dan kata sandi yang baru Anda tetapkan."
    >
      <AktivasiForm token={token} />
    </KartuAuth>
  );
}
