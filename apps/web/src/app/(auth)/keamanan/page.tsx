import { BrandMark } from "@/components/layout/brand-mark";
import { KeamananPanel } from "./keamanan-panel";

export const metadata = { title: "Keamanan Akun" };

/**
 * Keamanan akun untuk semua peran: perangkat yang sedang masuk (sesi aktif), mengeluarkan perangkat lain,
 * dan keluar dari semua perangkat. Tanpa izin khusus; hanya butuh sesi yang sah.
 */
export default function KeamananPage() {
  return (
    <main className="grid min-h-dvh place-items-center px-4 py-8">
      <div className="glass-strong w-full max-w-[560px] rounded-xl p-6 sm:p-8">
        <BrandMark className="size-11" />
        <h1 className="mt-5 text-display-md font-bold text-navy-900 dark:text-white">
          Keamanan akun
        </h1>
        <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
          Perangkat yang sedang masuk ke akun Anda. Cabut sesi yang tidak Anda
          kenali; perangkat itu langsung keluar dan harus memasukkan kata sandi
          lagi.
        </p>

        <KeamananPanel />
      </div>
    </main>
  );
}
