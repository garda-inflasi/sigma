import Link from "next/link";

export default function NotFound() {
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="glass w-full max-w-sm rounded-lg p-6 text-center sm:p-8">
        <p className="num text-display-lg font-semibold text-navy-300">404</p>
        <h1 className="mt-2 text-display-sm font-semibold">Halaman tidak ditemukan</h1>
        <p className="mt-2 text-sm text-muted-foreground">Periksa alamat atau buka palet perintah dengan ⌘K untuk mencari halaman.</p>
        <Link href="/" className="mt-6 inline-flex h-9 items-center rounded-md bg-primary px-4 text-[13px] font-medium text-primary-foreground transition-colors hover:bg-primary/90">Ke beranda</Link>
      </div>
    </main>
  );
}
