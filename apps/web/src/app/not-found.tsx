import Link from "next/link";

export default function NotFound() {
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="glass-strong max-w-sm rounded-xl p-8 text-center">
        <p className="num text-display-lg font-semibold text-navy-300">404</p>
        <h1 className="mt-2 text-display-sm font-semibold">Halaman tidak ditemukan</h1>
        <p className="mt-2 text-sm text-muted-foreground">Periksa alamat atau buka palet perintah dengan ⌘K untuk mencari halaman.</p>
        <Link href="/" className="mt-6 inline-flex h-10 items-center rounded-xl bg-navy-800 px-5 text-sm font-medium text-white">Ke beranda</Link>
      </div>
    </main>
  );
}
