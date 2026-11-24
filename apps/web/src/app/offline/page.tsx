import { CloudOff } from "lucide-react";
import Link from "next/link";

export const metadata = { title: "Offline" };

export default function OfflinePage() {
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="glass w-full max-w-sm rounded-lg p-6 text-center sm:p-8">
        <span className="mx-auto grid size-14 place-items-center rounded-lg bg-secondary text-muted-foreground"><CloudOff className="size-6" /></span>
        <h1 className="mt-5 text-display-sm font-semibold">Halaman ini belum tersimpan offline</h1>
        <p className="mt-2 text-sm text-muted-foreground">Form input harga tetap bisa dipakai tanpa jaringan. Entri akan dikirim otomatis saat koneksi kembali.</p>
        <Link href="/lapangan/input" className="mt-6 inline-flex h-11 items-center rounded-md bg-primary px-5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90">Buka form input harga</Link>
      </div>
    </main>
  );
}
