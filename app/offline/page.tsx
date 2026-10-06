import { CloudOff } from "lucide-react";
import Link from "next/link";

export const metadata = { title: "Offline" };

export default function OfflinePage() {
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="glass-strong max-w-sm rounded-xl p-8 text-center">
        <span className="mx-auto grid size-14 place-items-center rounded-xl bg-mist-100 text-navy-700 dark:bg-white/10"><CloudOff className="size-6" /></span>
        <h1 className="mt-5 text-display-sm font-semibold">Halaman ini belum tersimpan offline</h1>
        <p className="mt-2 text-sm text-muted-foreground">Form input harga tetap bisa dipakai tanpa jaringan. Entri akan dikirim otomatis saat koneksi kembali.</p>
        <Link href="/lapangan/input" className="mt-6 inline-flex h-11 items-center rounded-xl bg-navy-800 px-5 text-sm font-medium text-white">Buka form input harga</Link>
      </div>
    </main>
  );
}
