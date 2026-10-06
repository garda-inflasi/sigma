import { ShieldX } from "lucide-react";
import Link from "next/link";
import { getSession } from "@/lib/auth/session";
import { ROLES } from "@/lib/rbac/roles";

export const metadata = { title: "Akses ditolak" };

export default async function AksesDitolak({ searchParams }: { searchParams: Promise<{ dari?: string }> }) {
  const [{ dari }, user] = await Promise.all([searchParams, getSession()]);
  const beranda = user ? ROLES[user.role].beranda : "/masuk";
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="glass-strong max-w-md rounded-xl p-8 text-center">
        <span className="mx-auto grid size-14 place-items-center rounded-xl bg-defisit-soft text-defisit-ink"><ShieldX className="size-6" /></span>
        <h1 className="mt-5 text-display-sm font-semibold">Halaman ini di luar kewenangan peran Anda</h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
          {user ? <>Peran <b className="text-foreground">{ROLES[user.role].label}</b> tidak memiliki izin untuk membuka </> : "Anda perlu masuk untuk membuka "}
          <code className="num rounded bg-mist-100 px-1.5 py-0.5 text-xs dark:bg-white/10">{dari ?? "halaman tersebut"}</code>. Minta penambahan akses ke Administrator Sistem bila dibutuhkan untuk tugas Anda.
        </p>
        <Link href={beranda} className="mt-6 inline-flex h-10 items-center rounded-xl bg-navy-800 px-5 text-sm font-medium text-white">Kembali ke beranda</Link>
      </div>
    </main>
  );
}
