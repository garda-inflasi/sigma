"use client";

import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  Loader2,
  Mail,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { MODE_MOCK } from "@/lib/api/client";
import { cn } from "@/lib/utils";

/** Form permintaan tautan atur ulang kata sandi. */
export function LupaPasswordForm({ apiBase }: { apiBase: string }) {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [terkirim, setTerkirim] = useState(false);
  const [galat, setGalat] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (loading) return;
    setGalat(null);

    if (!email.trim()) {
      setGalat("Email wajib diisi.");
      return;
    }

    setLoading(true);
    try {
      if (MODE_MOCK) {
        await new Promise((r) => setTimeout(r, 650));
      } else {
        await fetch(`${apiBase}/v1/auth/password/forgot`, {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
          },
          body: JSON.stringify({ email: email.trim() }),
        });
      }
      // Selalu tampilkan keadaan sukses (tanpa membocorkan keberadaan akun).
      setTerkirim(true);
      toast.success("Tautan dikirim", {
        description: "Periksa kotak masuk email institusi Anda.",
      });
    } catch {
      const pesan = "Tidak dapat terhubung ke server. Coba lagi.";
      setGalat(pesan);
      toast.error("Gagal mengirim", { description: pesan });
    } finally {
      setLoading(false);
    }
  }

  if (terkirim) {
    return (
      <div className="mt-7 space-y-5">
        <div className="flex gap-3 rounded-xl bg-normal-soft/70 p-4 text-sm text-normal-ink dark:bg-normal/10 dark:text-normal-soft">
          <CheckCircle2 className="mt-0.5 size-5 shrink-0" />
          <p>
            Jika <span className="font-medium">{email.trim()}</span> terdaftar,
            tautan atur ulang kata sandi sudah dikirim. Periksa juga folder
            spam.
          </p>
        </div>
        <Link
          href="/masuk"
          className="flex h-11 w-full items-center justify-center gap-2 rounded-xl border border-input bg-white/80 text-[15px] font-medium text-navy-800 shadow-ambient-sm transition-all hover:bg-white hover:shadow-ambient dark:bg-white/[0.06] dark:text-white dark:hover:bg-white/[0.1]"
        >
          <ArrowLeft className="size-4" /> Kembali ke halaman masuk
        </Link>
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit} className="mt-6 space-y-3.5" noValidate>
      <div className="space-y-2">
        <Label htmlFor="email">Email institusi</Label>
        <div className="relative">
          <Mail className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id="email"
            name="email"
            type="email"
            inputMode="email"
            autoComplete="email"
            placeholder="nama@kemenkeu.go.id"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            disabled={loading}
            aria-invalid={!!galat}
            className="h-11 px-10"
            required
          />
        </div>
      </div>

      {galat && (
        <p role="alert" className="text-sm font-medium text-destructive">
          {galat}
        </p>
      )}

      <button
        type="submit"
        disabled={loading}
        className={cn(
          "flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-b from-navy-700 to-navy-800 text-[15px] font-medium text-white shadow-ambient transition-all",
          "hover:from-navy-600 hover:to-navy-700 hover:shadow-lift focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
          "disabled:cursor-not-allowed disabled:opacity-70",
        )}
      >
        {loading ? (
          <>
            <Loader2 className="size-4 animate-spin" /> Mengirim…
          </>
        ) : (
          <>
            Kirim tautan atur ulang <ArrowRight className="size-4" />
          </>
        )}
      </button>

      <Link
        href="/masuk"
        className="flex items-center justify-center gap-1.5 text-sm font-medium text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
      >
        <ArrowLeft className="size-4" /> Kembali ke halaman masuk
      </Link>
    </form>
  );
}
