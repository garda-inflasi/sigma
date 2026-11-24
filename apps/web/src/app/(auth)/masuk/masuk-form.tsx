"use client";

import { ArrowRight, Eye, EyeOff, Loader2, Lock, Mail } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { KotakGalatAuth, useSekarang } from "@/components/auth/galat-auth";
import { FormBff, PesanField, idGalatField } from "@/components/form/form-bff";
import { FormField } from "@/components/form/form-field";
import { SpringButton } from "@/components/garda/spring-button";
import { Input } from "@/components/ui/input";
import { ApiError, bffFetch, MODE_MOCK } from "@/lib/api/client";
import {
  galatAuth,
  galatLokal,
  masihMenunggu,
  masukanDariError,
  pesanTampil,
  type GalatAuth,
} from "@/lib/auth/pesan-login";
import { useFormBff } from "@/lib/form/use-form-bff";

// Akun demo (mode mock, tanpa backend) — satu-satunya kredensial yang diterima.
const DEMO_EMAIL = "khaidirhasan@gmail.com";
const DEMO_SANDI = "acehkukaya";

const skemaMasuk = z.object({
  email: z.string().trim().min(1, "Email wajib diisi."),
  password: z.string().min(1, "Kata sandi wajib diisi."),
});
type NilaiMasuk = z.infer<typeof skemaMasuk>;

/** Jawaban `POST /api/auth/login` (app/api/auth/login/route.ts). */
interface JawabanMasuk {
  tujuan: string;
  mfa_required?: boolean;
}

/** Mode mock (tanpa backend): hanya akun demo; peran diambil dari pengalih peran pengembangan. */
async function masukMock(
  n: NilaiMasuk,
  redirect: string,
): Promise<JawabanMasuk> {
  await new Promise((r) => setTimeout(r, 650));
  if (n.email.toLowerCase() !== DEMO_EMAIL || n.password !== DEMO_SANDI) {
    throw new ApiError(401, "Email atau kata sandi salah.");
  }
  const peran =
    document.cookie
      .split("; ")
      .find((r) => r.startsWith("garda_dev_role="))
      ?.split("=")[1] ?? "OPS_MAHASISWA";
  document.cookie = `garda_dev_role=${peran}; path=/; max-age=2592000; samesite=lax`;
  return { tujuan: redirect };
}

/**
 * Form masuk institusi: email + kata sandi, dikirim asinkron (JSON) ke BFF `/api/auth/login`. Tidak memakai
 * kiriman form native: galat tampil di form tanpa reload, dan halaman berpindah lewat router hanya sesudah
 * 2xx (ke beranda, atau ke langkah OTP untuk akun ber-MFA).
 */
export function MasukForm({
  redirect,
  galatAwal,
}: {
  redirect: string;
  /** Pesan gagal dari alur yang kembali ke halaman ini lewat URL (?error=, mis. SSO Google). */
  galatAwal?: string;
}) {
  const [email, setEmail] = useState(MODE_MOCK ? DEMO_EMAIL : "");
  const [sandi, setSandi] = useState(MODE_MOCK ? DEMO_SANDI : "");
  const [lihat, setLihat] = useState(false);
  // Galat dari URL hanya berlaku sampai kiriman pertama.
  const [galatUrl, setGalatUrl] = useState<GalatAuth | null>(() =>
    galatAwal ? galatLokal(galatAwal) : null,
  );

  const form = useFormBff<NilaiMasuk, JawabanMasuk, GalatAuth>({
    skema: skemaMasuk,
    kirim: (n) =>
      MODE_MOCK
        ? masukMock(n, redirect)
        : bffFetch<JawabanMasuk>("/api/auth/login", {
            method: "POST",
            body: JSON.stringify({ ...n, redirect }),
          }),
    sukses: (hasil) => {
      if (!hasil.mfa_required)
        toast.success("Berhasil masuk", {
          description: "Mengalihkan ke ruang kerja Anda…",
        });
      // Cookie sesi berubah: root layout (yang membaca sesi) harus dirender ulang.
      return { ke: hasil.tujuan, segarkan: true };
    },
    // Kata sandi yang ditolak tidak dibiarkan terisi; email tetap supaya mudah dicoba lagi.
    gagal: () => setSandi(""),
    petaGalat: (err) => galatAuth(masukanDariError(err), "masuk"),
    judulToastGagal: "Gagal masuk",
    pesanToast: (g) => pesanTampil(g),
  });

  const galat = form.galat ?? galatUrl;
  const sekarang = useSekarang(galat?.sampaiMs !== undefined);
  const tertahan = sekarang !== null && masihMenunggu(galat, sekarang);
  const { galatField } = form;
  const kirim = form.kirimDari(() => ({ email, password: sandi }));

  // Padding field: `Input` punya `px-3` bawaan, dan paket `cn` proyek ini tidak
  // membuangnya bila di-override dengan `pl-*`/`pr-*`. Hanya `px-*` simetris yang
  // menang, jadi ruang ikon dibuat via px-10 / px-11 (terverifikasi di browser).
  return (
    <FormBff
      onKirim={(e) => {
        setGalatUrl(null);
        kirim(e);
      }}
      sibuk={form.pending}
      className="mt-6 flex flex-col gap-4"
    >
      {/* Email */}
      <FormField id="email" label="Email institusi" galat={galatField.email}>
        <div className="relative">
          <Mail className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id="email"
            name="email"
            type="email"
            inputMode="email"
            autoComplete="email"
            placeholder="nama@gardainflasi.id"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            disabled={form.pending}
            aria-invalid={!!(galatField.email || galat)}
            aria-describedby={
              galatField.email ? idGalatField("email") : undefined
            }
            className="h-11 px-10"
            required
          />
        </div>
      </FormField>

      {/* Kata sandi */}
      <div className="relative">
        {/* Di luar <label> agar nama aksesibel isian tetap "Kata sandi"; sebelum isian agar urutan Tab tidak berubah. */}
        <Link
          href="/lupa-password"
          className="absolute top-0 right-0 text-xs font-medium text-data-600 underline-offset-4 transition-colors hover:text-data-700 hover:underline dark:text-data-300 dark:hover:text-data-200"
        >
          Lupa kata sandi?
        </Link>
        <FormField id="sandi" label="Kata sandi">
          <div className="relative">
            <Lock className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="sandi"
              name="password"
              type={lihat ? "text" : "password"}
              autoComplete="current-password"
              placeholder="••••••••"
              value={sandi}
              onChange={(e) => setSandi(e.target.value)}
              disabled={form.pending}
              aria-invalid={!!(galatField.password || galat)}
              aria-describedby={
                galatField.password ? idGalatField("password") : undefined
              }
              className="h-11 px-11"
              required
            />
            <button
              type="button"
              onClick={() => setLihat((v) => !v)}
              disabled={form.pending}
              aria-label={
                lihat ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"
              }
              aria-pressed={lihat}
              className="absolute top-1/2 right-1.5 grid size-8 -translate-y-1/2 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            >
              {lihat ? (
                <EyeOff className="size-4" />
              ) : (
                <Eye className="size-4" />
              )}
            </button>
          </div>
          <PesanField field="password" pesan={galatField.password} />
        </FormField>
      </div>

      {galat && <KotakGalatAuth galat={galat} sekarang={sekarang} />}

      <SpringButton
        type="submit"
        variant="primary"
        size="lg"
        className="h-11 w-full text-[15px]"
        disabled={form.pending || tertahan}
      >
        {form.pending ? (
          <>
            <Loader2 className="size-4 animate-spin" /> Memproses…
          </>
        ) : (
          <>
            Masuk <ArrowRight className="size-4" />
          </>
        )}
      </SpringButton>

      {MODE_MOCK && (
        <p className="rounded-lg border border-dashed border-border bg-muted/40 px-3 py-2 text-center text-xs leading-relaxed text-muted-foreground">
          <span className="font-medium text-foreground">Akun demo</span>{" "}
          <span className="num">{DEMO_EMAIL}</span> ·{" "}
          <span className="num">{DEMO_SANDI}</span>
        </p>
      )}
    </FormBff>
  );
}
