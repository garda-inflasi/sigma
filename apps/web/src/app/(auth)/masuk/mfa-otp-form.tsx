"use client";

import { ArrowLeft, ArrowRight, KeyRound, Loader2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { KotakGalatAuth, useSekarang } from "@/components/auth/galat-auth";
import { FormBff, PesanField, idGalatField } from "@/components/form/form-bff";
import { FormField } from "@/components/form/form-field";
import { SpringButton } from "@/components/garda/spring-button";
import { Input } from "@/components/ui/input";
import { bffFetch } from "@/lib/api/client";
import { galatLangkahOtp, rapikanKode, validasiKode } from "@/lib/auth/mfa";
import {
  masihMenunggu,
  masukanDariError,
  pesanTampil,
  type GalatAuth,
} from "@/lib/auth/pesan-login";
import { useFormBff } from "@/lib/form/use-form-bff";

const skemaOtp = z.object({
  code: z
    .string()
    .transform(rapikanKode)
    .superRefine((kode, ctx) => {
      const salah = validasiKode(kode);
      if (salah) ctx.addIssue({ code: "custom", message: salah });
    }),
});

/**
 * Langkah kedua login: kode 6 digit dari aplikasi autentikator atau satu kode cadangan. Token tantangan
 * ada di cookie httpOnly dari langkah kata sandi, jadi form ini hanya mengirim kodenya (JSON, asinkron).
 */
export function MfaOtpForm({ redirect }: { redirect: string }) {
  const [kode, setKode] = useState("");
  const form = useFormBff<
    z.output<typeof skemaOtp>,
    { tujuan: string },
    GalatAuth,
    z.input<typeof skemaOtp>
  >({
    skema: skemaOtp,
    kirim: (n) =>
      bffFetch<{ tujuan: string }>("/api/auth/mfa/validate", {
        method: "POST",
        body: JSON.stringify({ code: n.code, redirect }),
      }),
    sukses: (hasil) => {
      toast.success("Berhasil masuk", {
        description: "Mengalihkan ke ruang kerja Anda…",
      });
      return { ke: hasil.tujuan, segarkan: true };
    },
    petaGalat: (err) => {
      const m = masukanDariError(err);
      return galatLangkahOtp(m.status, m.pesan, {
        kode: m.kode,
        detik: m.detik,
      });
    },
    judulToastGagal: "Verifikasi gagal",
    pesanToast: (g) => pesanTampil(g),
  });
  const galat = form.galat;
  const sekarang = useSekarang(galat?.sampaiMs !== undefined);
  const tertahan = sekarang !== null && masihMenunggu(galat, sekarang);
  const loading = form.pending;

  return (
    <FormBff
      onKirim={form.kirimDari(() => ({ code: kode }))}
      sibuk={loading}
      className="mt-6 flex flex-col gap-4"
    >
      <FormField id="kode" label="Kode verifikasi">
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
            aria-invalid={!!(galat || form.galatField.code)}
            aria-describedby={
              form.galatField.code ? idGalatField("code") : undefined
            }
            className="num h-11 px-10 text-lg tracking-widest"
            required
          />
        </div>
        <PesanField field="code" pesan={form.galatField.code} />
      </FormField>

      {galat && (
        <KotakGalatAuth galat={galat} sekarang={sekarang}>
          {galat.ulangMasuk && (
            <Link
              href="/masuk"
              className="text-data-700 underline-offset-4 hover:underline dark:text-data-300"
            >
              Kembali ke halaman masuk
            </Link>
          )}
        </KotakGalatAuth>
      )}

      <SpringButton
        type="submit"
        variant="primary"
        size="lg"
        className="h-11 w-full text-[15px]"
        disabled={loading || tertahan}
      >
        {loading ? (
          <Loader2 className="size-4 animate-spin" />
        ) : (
          <>
            Verifikasi <ArrowRight className="size-4" />
          </>
        )}
      </SpringButton>

      <Link
        href="/masuk"
        className="flex min-h-11 items-center justify-center gap-1.5 text-sm font-medium text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
      >
        <ArrowLeft className="size-4" /> Gunakan akun lain
      </Link>
    </FormBff>
  );
}
