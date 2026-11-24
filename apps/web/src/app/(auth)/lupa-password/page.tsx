import { KartuAuth } from "@/components/auth/kartu-auth";
import { LupaPasswordForm } from "./lupa-password-form";

export const metadata = { title: "Lupa Kata Sandi" };

/** Pemulihan kata sandi: kirim tautan atur ulang ke email institusi (layanan auth Go). */
export default function LupaPasswordPage() {
  return (
    <KartuAuth
      judul="Atur ulang kata sandi"
      deskripsi="Masukkan email institusi Anda. Jika terdaftar, kami kirim tautan untuk mengatur ulang kata sandi."
      catatan="Demi keamanan, kami tidak mengonfirmasi apakah email terdaftar. Tautan atur ulang berlaku 30 menit."
    >
      <LupaPasswordForm />
    </KartuAuth>
  );
}
