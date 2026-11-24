import { ShieldAlert } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Akibat beralih ke akun lain, sesuai penegakan di API (`impersonasiDilarang` dan audit): ditampilkan sebelum pengguna
 * memulai impersonasi.
 */
export function DampakImpersonasi({
  sasaran,
  children,
  className,
}: {
  /** Nama akun sasaran bila sudah dipilih. */
  sasaran?: string;
  /** Butir tambahan, mis. batas durasi dari kebijakan. */
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex shrink-0 items-start gap-2 rounded-lg border border-waspada/50 bg-waspada-soft p-3 text-[13px] text-navy-900 dark:bg-waspada/15 dark:text-amber-100",
        className,
      )}
    >
      <ShieldAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
      <div className="min-w-0">
        <p className="font-medium">
          Yang terjadi saat Anda beralih{sasaran ? ` ke ${sasaran}` : ""}
        </p>
        <ul className="mt-1 grid list-disc gap-1 pl-4 text-xs leading-snug">
          <li>
            Izin dan cakupan data mengikuti akun sasaran, bukan akun Anda.
          </li>
          <li>
            Perubahan data benar-benar tersimpan dan tercatat atas nama akun
            asli Anda.
          </li>
          <li>
            Kata sandi, MFA, profil, sesi, pengguna, peran, dan kebijakan
            impersonasi tidak dapat diubah selama beralih.
          </li>
          {children}
        </ul>
      </div>
    </div>
  );
}
