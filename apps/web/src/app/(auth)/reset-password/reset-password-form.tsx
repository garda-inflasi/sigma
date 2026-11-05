"use client";

import { ArrowRight, Check, CheckCircle2, Circle, Loader2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { KolomSandi } from "@/components/auth/kolom-sandi";
import { ApiError, bffFetch } from "@/lib/api/client";
import { evaluatePassword } from "@/lib/auth/password-policy";
import {
  galatReset,
  validasiFormReset,
  type GalatReset,
} from "@/lib/auth/reset-password";
import { cn } from "@/lib/utils";

const TOMBOL =
  "flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-b from-navy-700 to-navy-800 text-[15px] font-medium text-white shadow-ambient transition-all hover:from-navy-600 hover:to-navy-700 hover:shadow-lift focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-70";

/** Form kata sandi baru dari tautan email: aturan kebijakan langsung, lalu arahkan ke halaman masuk. */
export function ResetPasswordForm({ token }: { token: string }) {
  const [baru, setBaru] = useState("");
  const [konfirmasi, setKonfirmasi] = useState("");
  const [loading, setLoading] = useState(false);
  const [galat, setGalat] = useState<GalatReset | null>(null);
  const [selesai, setSelesai] = useState(false);

  const aturan = evaluatePassword(baru);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (loading) return;

    const salah = validasiFormReset(baru, konfirmasi);
    if (salah) return setGalat({ pesan: salah, tautanBaru: false });

    setGalat(null);
    setLoading(true);
    try {
      await bffFetch("/api/auth/reset-password", {
        method: "POST",
        body: JSON.stringify({ token, password: baru }),
      });
      setSelesai(true);
      toast.success("Kata sandi diatur ulang", {
        description: "Silakan masuk dengan kata sandi baru.",
      });
    } catch (err) {
      const status = err instanceof ApiError ? err.status : undefined;
      const hasil = galatReset(
        status,
        err instanceof Error ? err.message : undefined,
      );
      setGalat(hasil);
      toast.error("Gagal mengatur ulang", { description: hasil.pesan });
    } finally {
      setLoading(false);
    }
  }

  if (selesai) {
    return (
      <div className="mt-6 space-y-4 text-center" role="status">
        <CheckCircle2 className="mx-auto size-10 text-normal" />
        <p className="text-sm leading-relaxed text-muted-foreground">
          Kata sandi berhasil diatur ulang. Semua sesi lama telah diakhiri.
        </p>
        <Link href="/masuk" className={TOMBOL}>
          Masuk <ArrowRight className="size-4" />
        </Link>
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit} className="mt-6 space-y-3.5" noValidate>
      <KolomSandi
        id="baru"
        label="Kata sandi baru"
        value={baru}
        onChange={setBaru}
        disabled={loading}
        autoComplete="new-password"
      />

      <ul className="space-y-1 text-xs" aria-label="Aturan kata sandi">
        {aturan.map((r) => (
          <li
            key={r.id}
            className={cn(
              "flex items-center gap-2",
              r.terpenuhi
                ? "text-normal-ink dark:text-normal-soft"
                : "text-muted-foreground",
            )}
          >
            {r.terpenuhi ? (
              <Check className="size-3.5" />
            ) : (
              <Circle className="size-3.5" />
            )}
            {r.label}
          </li>
        ))}
      </ul>

      <KolomSandi
        id="konfirmasi"
        label="Ulangi kata sandi baru"
        value={konfirmasi}
        onChange={setKonfirmasi}
        disabled={loading}
        autoComplete="new-password"
      />

      {galat && (
        <div role="alert" className="space-y-1 text-sm">
          <p className="font-medium text-destructive">{galat.pesan}</p>
          {galat.tautanBaru && (
            <Link
              href="/lupa-password"
              className="text-data-700 underline-offset-4 hover:underline"
            >
              Minta tautan baru
            </Link>
          )}
        </div>
      )}

      <button type="submit" disabled={loading} className={TOMBOL}>
        {loading ? (
          <Loader2 className="size-4 animate-spin" />
        ) : (
          <>
            Atur ulang kata sandi <ArrowRight className="size-4" />
          </>
        )}
      </button>
    </form>
  );
}
