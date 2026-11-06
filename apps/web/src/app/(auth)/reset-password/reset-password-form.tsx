"use client";

import { ArrowRight, Check, CheckCircle2, Circle, Loader2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { KolomSandi } from "@/components/auth/kolom-sandi";
import { KotakGalatAuth, useTungguGalat } from "@/components/auth/galat-auth";
import { FormBff } from "@/components/form/form-bff";
import { bffFetch } from "@/lib/api/client";
import { evaluatePassword } from "@/lib/auth/password-policy";
import { masukanDariError, pesanTampil, type GalatAuth } from "@/lib/auth/pesan-login";
import { galatReset } from "@/lib/auth/reset-password";
import { skemaSandiBaru } from "@/lib/auth/skema-sandi";
import { useFormBff } from "@/lib/form/use-form-bff";
import { cn } from "@/lib/utils";

const TOMBOL =
  "flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-b from-navy-700 to-navy-800 text-[15px] font-medium text-white shadow-ambient transition-all hover:from-navy-600 hover:to-navy-700 hover:shadow-lift focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-70";

/** Form kata sandi baru dari tautan email: aturan kebijakan langsung, lalu arahkan ke halaman masuk. */
export function ResetPasswordForm({ token }: { token: string }) {
  const [baru, setBaru] = useState("");
  const [konfirmasi, setKonfirmasi] = useState("");
  const [selesai, setSelesai] = useState(false);
  const form = useFormBff<{ baru: string; konfirmasi: string }, unknown, GalatAuth>({
    skema: skemaSandiBaru,
    kirim: (n) => bffFetch("/api/auth/reset-password", { method: "POST", body: JSON.stringify({ token, password: n.baru }) }),
    sukses: () => {
      setSelesai(true);
      toast.success("Kata sandi diatur ulang", { description: "Silakan masuk dengan kata sandi baru." });
    },
    petaGalat: (err) => {
      const m = masukanDariError(err);
      return galatReset(m.status, m.pesan, { kode: m.kode, detik: m.detik });
    },
    judulToastGagal: "Gagal mengatur ulang",
    pesanToast: (g) => pesanTampil(g),
  });
  const galat = form.galat;
  const loading = form.pending;
  const { sekarang, tertahan } = useTungguGalat(galat);

  const aturan = evaluatePassword(baru);

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
    <FormBff onKirim={form.kirimDari(() => ({ baru, konfirmasi }))} sibuk={loading} className="mt-6 space-y-3.5">
      <KolomSandi
        id="baru"
        label="Kata sandi baru"
        value={baru}
        onChange={setBaru}
        disabled={loading}
        galat={form.galatField.baru}
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
        galat={form.galatField.konfirmasi}
        autoComplete="new-password"
      />

      {galat && (
        <KotakGalatAuth galat={galat} sekarang={sekarang}>
          {galat.tautanBaru && (
            <Link
              href="/lupa-password"
              className="text-data-700 underline-offset-4 hover:underline"
            >
              Minta tautan baru
            </Link>
          )}
        </KotakGalatAuth>
      )}

      <button type="submit" disabled={loading || tertahan} className={TOMBOL}>
        {loading ? (
          <Loader2 className="size-4 animate-spin" />
        ) : (
          <>
            Atur ulang kata sandi <ArrowRight className="size-4" />
          </>
        )}
      </button>
    </FormBff>
  );
}
