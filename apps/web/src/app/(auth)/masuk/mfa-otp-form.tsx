"use client";

import { ArrowLeft, ArrowRight, KeyRound, Loader2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { KotakGalatAuth, useGalatAuth } from "@/components/auth/galat-auth";
import { bffFetch } from "@/lib/api/client";
import { galatLangkahOtp, rapikanKode, validasiKode } from "@/lib/auth/mfa";
import { galatLokal, masukanDariError, pesanTampil } from "@/lib/auth/pesan-login";
import { cn } from "@/lib/utils";

/**
 * Langkah kedua login: kode 6 digit dari aplikasi autentikator atau satu kode cadangan. Token tantangan
 * ada di cookie httpOnly dari langkah kata sandi, jadi form ini hanya mengirim kodenya.
 */
export function MfaOtpForm({ redirect }: { redirect: string }) {
  const [kode, setKode] = useState("");
  const [loading, setLoading] = useState(false);
  const { galat, setGalat, sekarang, tertahan } = useGalatAuth();

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (loading) return;

    const salah = validasiKode(kode);
    if (salah) return setGalat(galatLokal(salah));

    setGalat(null);
    setLoading(true);
    try {
      const hasil = await bffFetch<{ tujuan: string }>(
        "/api/auth/mfa/validate",
        {
          method: "POST",
          body: JSON.stringify({ code: rapikanKode(kode), redirect }),
        },
      );
      toast.success("Berhasil masuk", {
        description: "Mengalihkan ke ruang kerja Anda…",
      });
      window.location.assign(hasil.tujuan);
    } catch (err) {
      const m = masukanDariError(err);
      const hasil = galatLangkahOtp(m.status, m.pesan, {
        kode: m.kode,
        detik: m.detik,
      });
      setGalat(hasil);
      toast.error("Verifikasi gagal", { description: pesanTampil(hasil) });
      setLoading(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="mt-6 space-y-3.5" noValidate>
      <div className="space-y-2">
        <Label htmlFor="kode">Kode verifikasi</Label>
        <div className="relative">
          <KeyRound className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id="kode"
            name="kode"
            inputMode="numeric"
            autoComplete="one-time-code"
            autoFocus
            placeholder="123456"
            value={kode}
            onChange={(e) => setKode(e.target.value)}
            disabled={loading}
            aria-invalid={!!galat}
            className="num h-11 px-10 text-lg tracking-widest"
            required
          />
        </div>
      </div>

      {galat && (
        <KotakGalatAuth galat={galat} sekarang={sekarang}>
          {galat.ulangMasuk && (
            <Link
              href="/masuk"
              className="text-data-700 underline-offset-4 hover:underline"
            >
              Kembali ke halaman masuk
            </Link>
          )}
        </KotakGalatAuth>
      )}

      <button
        type="submit"
        disabled={loading || tertahan}
        className={cn(
          "flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-b from-navy-700 to-navy-800 text-[15px] font-medium text-white shadow-ambient transition-all",
          "hover:from-navy-600 hover:to-navy-700 hover:shadow-lift focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-70",
        )}
      >
        {loading ? (
          <Loader2 className="size-4 animate-spin" />
        ) : (
          <>
            Verifikasi <ArrowRight className="size-4" />
          </>
        )}
      </button>

      <Link
        href="/masuk"
        className="flex items-center justify-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-3.5" /> Gunakan akun lain
      </Link>
    </form>
  );
}
