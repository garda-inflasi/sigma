import { redirect } from "next/navigation";
import { hrefTabProfil } from "@/lib/profil/tab";

/**
 * Keamanan akun kini tab "Perangkat & Sesi Aktif" di /profil. Rute ini bukan tujuan pengalihan wajib
 * (edge-guard), jadi tautan dan penanda buku lama cukup dialihkan di server.
 */
export default function KeamananPage(): never {
  redirect(hrefTabProfil("keamanan"));
}
