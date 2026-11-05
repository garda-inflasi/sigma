"use client";

import {
  ArrowRight,
  Check,
  Copy,
  Download,
  Loader2,
  ShieldAlert,
  ShieldCheck,
} from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError, bffFetch } from "@/lib/api/client";
import type { MfaSetupResponse, UserResponse } from "@/lib/bff/api-client";
import { rapikanKode, teksKodeCadangan, validasiKode } from "@/lib/auth/mfa";
import { cn } from "@/lib/utils";

const TOMBOL_UTAMA =
  "flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-b from-navy-700 to-navy-800 text-[15px] font-medium text-white shadow-ambient transition-all hover:from-navy-600 hover:to-navy-700 hover:shadow-lift focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-70";
const TOMBOL_SEKUNDER =
  "flex h-10 items-center justify-center gap-2 rounded-xl border border-input bg-white/80 px-4 text-sm font-medium text-navy-800 transition-all hover:bg-white focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-60 dark:bg-white/[0.06] dark:text-white";

const pesanGalat = (err: unknown) =>
  err instanceof ApiError || err instanceof Error
    ? err.message
    : "Tidak dapat terhubung ke server. Coba lagi.";

type Tahap = "memuat" | "awal" | "kode" | "cadangan" | "aktif";

/** Setup MFA (QR, kode pertama, kode cadangan sekali tampil) dan pengelolaannya (kode cadangan baru, nonaktifkan). */
export function MfaPanel({ lanjut }: { lanjut: string }) {
  const [akun, setAkun] = useState<UserResponse | null>(null);
  const [tahap, setTahap] = useState<Tahap>("memuat");
  const [setup, setSetup] = useState<MfaSetupResponse | null>(null);
  const [kode, setKode] = useState("");
  const [cadangan, setCadangan] = useState<string[]>([]);
  const [tersimpan, setTersimpan] = useState(false);
  const [sibuk, setSibuk] = useState(false);
  const [galat, setGalat] = useState<string | null>(null);
  const [sandi, setSandi] = useState("");

  const muat = useCallback(async () => {
    try {
      const { user } = await bffFetch<{ user: UserResponse }>("/api/auth/me");
      setAkun(user);
      setTahap(user.mfa_aktif ? "aktif" : "awal");
    } catch (err) {
      setGalat(pesanGalat(err));
      setTahap("awal");
    }
  }, []);

  useEffect(() => {
    void muat();
  }, [muat]);

  async function jalankan(aksi: () => Promise<void>, judulGagal: string) {
    if (sibuk) return;
    setSibuk(true);
    setGalat(null);
    try {
      await aksi();
    } catch (err) {
      const pesan = pesanGalat(err);
      setGalat(pesan);
      toast.error(judulGagal, { description: pesan });
    } finally {
      setSibuk(false);
    }
  }

  const mulai = () =>
    jalankan(async () => {
      setSetup(
        await bffFetch<MfaSetupResponse>("/api/auth/mfa/setup", {
          method: "POST",
        }),
      );
      setKode("");
      setTahap("kode");
    }, "Gagal memulai setup");

  const verifikasi = (e: React.FormEvent) => {
    e.preventDefault();
    const salah = validasiKode(kode);
    if (salah) return setGalat(salah);
    void jalankan(async () => {
      const hasil = await bffFetch<{ kode_cadangan: string[] }>(
        "/api/auth/mfa/verify",
        {
          method: "POST",
          body: JSON.stringify({ code: rapikanKode(kode) }),
        },
      );
      setCadangan(hasil.kode_cadangan);
      setTersimpan(false);
      setTahap("cadangan");
      toast.success("MFA aktif", {
        description: "Simpan kode cadangan Anda sekarang.",
      });
    }, "Kode tidak diterima");
  };

  const buatKodeBaru = (e: React.FormEvent) => {
    e.preventDefault();
    void jalankan(async () => {
      const hasil = await bffFetch<{ kode_cadangan: string[] }>(
        "/api/auth/mfa/backup-codes",
        {
          method: "POST",
          body: JSON.stringify({ password: sandi }),
        },
      );
      setCadangan(hasil.kode_cadangan);
      setTersimpan(false);
      setSandi("");
      setTahap("cadangan");
    }, "Gagal membuat kode cadangan");
  };

  const nonaktifkan = () =>
    jalankan(async () => {
      await bffFetch("/api/auth/mfa/disable", {
        method: "POST",
        body: JSON.stringify({ password: sandi }),
      });
      setSandi("");
      toast.success("MFA dinonaktifkan");
      await muat();
    }, "Gagal menonaktifkan MFA");

  async function salin() {
    await navigator.clipboard.writeText(cadangan.join("\n"));
    toast.success("Kode cadangan disalin");
  }

  function unduh() {
    const url = URL.createObjectURL(
      new Blob([teksKodeCadangan(cadangan, akun?.email ?? "")], {
        type: "text/plain",
      }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = "kode-cadangan-garda-inflasi.txt";
    a.click();
    URL.revokeObjectURL(url);
  }

  if (tahap === "memuat") {
    return (
      <Loader2
        className="mx-auto mt-8 size-6 animate-spin text-muted-foreground"
        aria-label="Memuat"
      />
    );
  }

  const galatTampil = galat && (
    <p role="alert" className="text-sm font-medium text-destructive">
      {galat}
    </p>
  );

  if (tahap === "awal") {
    return (
      <div className="mt-6 space-y-4">
        {akun?.mfa_setup_diwajibkan && (
          <p className="flex gap-2.5 rounded-xl bg-waspada-soft p-3.5 text-sm text-waspada-ink">
            <ShieldAlert className="mt-px size-4 shrink-0" />
            Peran Anda wajib memakai verifikasi dua langkah. Selesaikan setup
            ini untuk melanjutkan.
          </p>
        )}
        <p className="text-sm leading-relaxed text-muted-foreground">
          Anda memerlukan aplikasi autentikator (mis. Google Authenticator,
          Authy, atau 1Password) di ponsel.
        </p>
        {galatTampil}
        <button
          type="button"
          onClick={mulai}
          disabled={sibuk}
          className={TOMBOL_UTAMA}
        >
          {sibuk ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <>
              Mulai setup <ArrowRight className="size-4" />
            </>
          )}
        </button>
      </div>
    );
  }

  if (tahap === "kode" && setup) {
    return (
      <form onSubmit={verifikasi} className="mt-6 space-y-4" noValidate>
        <ol className="list-decimal space-y-1 pl-5 text-sm text-muted-foreground">
          <li>Pindai kode QR dengan aplikasi autentikator.</li>
          <li>Masukkan kode 6 digit yang tampil di aplikasi.</li>
        </ol>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={`data:image/svg+xml;utf8,${encodeURIComponent(setup.qr_svg)}`}
          alt="Kode QR untuk aplikasi autentikator"
          className="mx-auto size-48 rounded-xl border border-border bg-white p-2"
        />
        <details className="text-xs text-muted-foreground">
          <summary className="cursor-pointer font-medium text-foreground">
            Tidak bisa memindai? Masukkan kunci manual
          </summary>
          <p className="num mt-2 break-all rounded-lg bg-muted/50 p-2 text-sm text-foreground select-all">
            {setup.secret}
          </p>
        </details>
        <div className="space-y-2">
          <Label htmlFor="kode">Kode 6 digit</Label>
          <Input
            id="kode"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={7}
            placeholder="123456"
            value={kode}
            onChange={(e) => setKode(e.target.value)}
            disabled={sibuk}
            className="num h-11 text-lg tracking-widest"
            required
          />
        </div>
        {galatTampil}
        <button type="submit" disabled={sibuk} className={TOMBOL_UTAMA}>
          {sibuk ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <>
              Aktifkan MFA <ArrowRight className="size-4" />
            </>
          )}
        </button>
      </form>
    );
  }

  if (tahap === "cadangan") {
    return (
      <div className="mt-6 space-y-4">
        <p className="text-sm leading-relaxed text-muted-foreground">
          Simpan kode cadangan ini. Masing-masing hanya berlaku sekali dan{" "}
          <strong className="text-foreground">
            tidak dapat ditampilkan lagi
          </strong>
          . Pakai salah satunya bila ponsel Anda hilang.
        </p>
        <ul className="num grid grid-cols-2 gap-2 rounded-xl bg-muted/50 p-3 text-center text-sm font-medium select-all">
          {cadangan.map((k) => (
            <li key={k}>{k}</li>
          ))}
        </ul>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={salin}
            className={cn(TOMBOL_SEKUNDER, "flex-1")}
          >
            <Copy className="size-4" /> Salin
          </button>
          <button
            type="button"
            onClick={unduh}
            className={cn(TOMBOL_SEKUNDER, "flex-1")}
          >
            <Download className="size-4" /> Unduh
          </button>
        </div>
        <label className="flex items-start gap-2 text-sm">
          <Checkbox
            checked={tersimpan}
            onCheckedChange={(v) => setTersimpan(v === true)}
            className="mt-0.5"
          />
          Saya sudah menyimpan kode cadangan di tempat aman.
        </label>
        <button
          type="button"
          disabled={!tersimpan}
          onClick={() =>
            akun?.mfa_setup_diwajibkan
              ? window.location.assign(lanjut)
              : void muat()
          }
          className={TOMBOL_UTAMA}
        >
          {akun?.mfa_setup_diwajibkan ? (
            <>
              Lanjutkan <ArrowRight className="size-4" />
            </>
          ) : (
            <>
              Selesai <Check className="size-4" />
            </>
          )}
        </button>
      </div>
    );
  }

  return (
    <div className="mt-6 space-y-4">
      <p className="flex gap-2.5 rounded-xl bg-normal-soft p-3.5 text-sm text-normal-ink">
        <ShieldCheck className="mt-px size-4 shrink-0" />
        Verifikasi dua langkah aktif
        {akun?.kode_cadangan_tersisa !== undefined && (
          <>
            {" "}
            · <span className="num">{akun.kode_cadangan_tersisa}</span> kode
            cadangan tersisa
          </>
        )}
        .
      </p>
      <div className="space-y-2">
        <Label htmlFor="sandi">Kata sandi (untuk mengonfirmasi)</Label>
        <Input
          id="sandi"
          type="password"
          autoComplete="current-password"
          value={sandi}
          onChange={(e) => setSandi(e.target.value)}
          disabled={sibuk}
          className="h-11"
        />
      </div>
      {galatTampil}
      <form onSubmit={buatKodeBaru} className="space-y-2">
        <button
          type="submit"
          disabled={sibuk || !sandi}
          className={cn(TOMBOL_SEKUNDER, "w-full")}
        >
          Buat kode cadangan baru
        </button>
        <p className="text-xs text-muted-foreground">
          Kode cadangan lama langsung tidak berlaku.
        </p>
      </form>
      {akun?.mfa_wajib ? (
        <p className="text-xs text-muted-foreground">
          Verifikasi dua langkah wajib untuk akun Anda dan tidak dapat
          dinonaktifkan.
        </p>
      ) : (
        <button
          type="button"
          onClick={nonaktifkan}
          disabled={sibuk || !sandi}
          className={cn(TOMBOL_SEKUNDER, "w-full text-defisit-ink")}
        >
          Nonaktifkan MFA
        </button>
      )}
      <Link
        href="/"
        className="block text-center text-xs font-medium text-muted-foreground hover:text-foreground"
      >
        Kembali ke ruang kerja
      </Link>
    </div>
  );
}
