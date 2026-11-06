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
import { z } from "zod";
import { FormBff, PesanField, idGalatField } from "@/components/form/form-bff";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { KotakGalatAuth, useTungguGalat } from "@/components/auth/galat-auth";
import { bffFetch, MODE_MOCK } from "@/lib/api/client";
import {
  galatAuth,
  masukanDariError,
  pesanTampil,
  type GalatAuth,
} from "@/lib/auth/pesan-login";
import { useFormBff } from "@/lib/form/use-form-bff";
import { cn } from "@/lib/utils";

const skemaLupa = z.object({ email: z.string().trim().min(1, "Email wajib diisi.") });

/** Form permintaan tautan atur ulang kata sandi (JSON ke BFF, tanpa reload). */
export function LupaPasswordForm() {
  const [email, setEmail] = useState("");
  const [terkirim, setTerkirim] = useState(false);
  const form = useFormBff<z.infer<typeof skemaLupa>, unknown, GalatAuth>({
    skema: skemaLupa,
    kirim: (n) =>
      MODE_MOCK
        ? new Promise((r) => setTimeout(r, 650))
        : bffFetch("/api/auth/forgot-password", { method: "POST", body: JSON.stringify(n) }),
    sukses: () => {
      // Selalu tampilkan keadaan sukses (tanpa membocorkan keberadaan akun).
      setTerkirim(true);
      toast.success("Tautan dikirim", { description: "Periksa kotak masuk email institusi Anda." });
    },
    // 429 dari BFF membawa sisa waktu; selebihnya pesan jaringan umum (tidak membocorkan keberadaan akun).
    petaGalat: (err) => galatAuth(masukanDariError(err), "lupa"),
    judulToastGagal: "Gagal mengirim",
    pesanToast: (g) => pesanTampil(g),
  });
  const galat = form.galat;
  const loading = form.pending;
  const { sekarang, tertahan } = useTungguGalat(galat);

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
    <FormBff onKirim={form.kirimDari(() => ({ email }))} sibuk={loading} className="mt-6 space-y-3.5">
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
            aria-invalid={!!(galat || form.galatField.email)}
            aria-describedby={form.galatField.email ? idGalatField("email") : undefined}
            className="h-11 px-10"
            required
          />
        </div>
        <PesanField field="email" pesan={form.galatField.email} />
      </div>

      {galat && <KotakGalatAuth galat={galat} sekarang={sekarang} />}

      <button
        type="submit"
        disabled={loading || tertahan}
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
    </FormBff>
  );
}
