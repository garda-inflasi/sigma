"use client";

import { ArrowRight, Check, CheckCircle2, Circle, Loader2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { KolomSandi } from "@/components/auth/kolom-sandi";
import { KotakGalatAuth, useTungguGalat } from "@/components/auth/galat-auth";
import { FormBff } from "@/components/form/form-bff";
import { SpringButton, SpringLink } from "@/components/garda/spring-button";
import { bffFetch } from "@/lib/api/client";
import { galatAktivasi, skemaAktivasi } from "@/lib/auth/aktivasi";
import { evaluatePassword } from "@/lib/auth/password-policy";
import {
  masukanDariError,
  pesanTampil,
  type GalatAuth,
} from "@/lib/auth/pesan-login";
import { useFormBff } from "@/lib/form/use-form-bff";
import { cn } from "@/lib/utils";

/**
 * Form kata sandi pertama dari tautan undangan: aturan kebijakan langsung, lalu arahkan ke halaman masuk.
 * Aktivasi tidak menerbitkan sesi: pengguna masuk seperti biasa (lockout dan MFA tetap berlaku di satu tempat).
 */
export function AktivasiForm({ token }: { token: string }) {
  const [baru, setBaru] = useState("");
  const [konfirmasi, setKonfirmasi] = useState("");
  const [selesai, setSelesai] = useState(false);
  const form = useFormBff<
    { baru: string; konfirmasi: string },
    unknown,
    GalatAuth
  >({
    skema: skemaAktivasi,
    kirim: (n) =>
      bffFetch("/api/auth/aktivasi", {
        method: "POST",
        body: JSON.stringify({ token, password: n.baru }),
      }),
    sukses: () => {
      setSelesai(true);
      toast.success("Akun diaktifkan", {
        description: "Silakan masuk dengan kata sandi yang baru ditetapkan.",
      });
    },
    petaGalat: (err) => {
      const m = masukanDariError(err);
      return galatAktivasi(m.status, m.pesan, { kode: m.kode, detik: m.detik });
    },
    judulToastGagal: "Gagal mengaktifkan akun",
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
          Akun Anda sudah aktif. Masuk dengan email undangan dan kata sandi yang
          baru Anda tetapkan.
        </p>
        <SpringLink
          href="/masuk"
          variant="primary"
          size="lg"
          className="flex h-11 w-full text-[15px]"
        >
          Masuk <ArrowRight />
        </SpringLink>
      </div>
    );
  }

  return (
    <FormBff
      onKirim={form.kirimDari(() => ({ baru, konfirmasi }))}
      sibuk={loading}
      className="mt-6 flex flex-col gap-4"
    >
      <KolomSandi
        id="baru"
        label="Kata sandi baru"
        value={baru}
        onChange={setBaru}
        disabled={loading}
        galat={form.galatField.baru}
        autoComplete="new-password"
      />

      <ul
        className="grid gap-x-3 gap-y-1 text-xs sm:grid-cols-2"
        aria-label="Aturan kata sandi"
      >
        {aturan.map((r) => (
          <li
            key={r.id}
            className={cn(
              "flex items-center gap-2 sm:last:col-span-2",
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
            <p className="text-muted-foreground">
              Minta administrator mengirim ulang undangan dari halaman Pengguna,
              lalu buka tautan di email terbaru.
            </p>
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
            Aktifkan akun <ArrowRight className="size-4" />
          </>
        )}
      </SpringButton>
    </FormBff>
  );
}
