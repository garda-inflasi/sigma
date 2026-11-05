"use client";

import { ArrowRight, Eye, EyeOff, Loader2, Lock, Mail } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { API_BASE, MODE_MOCK } from "@/lib/api/client";
import { cn } from "@/lib/utils";

// Akun demo (mode mock, tanpa backend) — satu-satunya kredensial yang diterima.
const DEMO_EMAIL = "khaidirhasan@gmail.com";
const DEMO_SANDI = "acehkukaya";

/**
 * Form masuk institusi: email + kata sandi.
 * Di mode backend (NEXT_PUBLIC_API_BASE_URL aktif): POST ke layanan auth Go yang
 * menerbitkan cookie httpOnly `garda_session`; di mode mock UI berjalan tanpa backend.
 */
export function MasukForm({
  redirect,
  galatAwal,
}: {
  redirect: string;
  /** Pesan gagal dari BFF setelah kiriman form (?error=). */
  galatAwal?: string;
}) {
  const router = useRouter();
  const [email, setEmail] = useState(MODE_MOCK ? DEMO_EMAIL : "");
  const [sandi, setSandi] = useState(MODE_MOCK ? DEMO_SANDI : "");
  const [lihat, setLihat] = useState(false);
  const [ingat, setIngat] = useState(true);
  const [loading, setLoading] = useState(false);
  const [galat, setGalat] = useState<string | null>(galatAwal ?? null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (loading) return;
    setGalat(null);

    if (!email.trim() || !sandi) {
      setGalat("Email dan kata sandi wajib diisi.");
      return;
    }

    setLoading(true);
    try {
      if (MODE_MOCK) {
        // Tanpa backend: verifikasi terhadap akun demo lalu lanjut ke tujuan.
        await new Promise((r) => setTimeout(r, 650));
        if (email.trim().toLowerCase() !== DEMO_EMAIL || sandi !== DEMO_SANDI) {
          const pesan = "Email atau kata sandi salah.";
          setGalat(pesan);
          toast.error("Gagal masuk", { description: pesan });
          setLoading(false);
          return;
        }
        toast.success("Berhasil masuk", {
          description: "Mengalihkan ke ruang kerja Anda…",
        });
        // Set cookie dev role agar resolveSession membaca role yang benar
        const roleBaru = document.cookie
          .split("; ")
          .find((r) => r.startsWith("garda_dev_role="))
          ?.split("=")[1] ?? "OPS_MAHASISWA";
        document.cookie = `garda_dev_role=${roleBaru}; path=/; max-age=2592000; samesite=lax`;
        router.replace(redirect);
        router.refresh();
        return;
      }
    } catch (err) {
      const pesan =
        err instanceof Error
          ? err.message
          : "Tidak dapat terhubung ke server. Coba lagi.";
      setGalat(pesan);
      toast.error("Gagal masuk", { description: pesan });
      setLoading(false);
    }
  }

  // Padding field: `Input` punya `px-3` bawaan, dan paket `cn` proyek ini tidak
  // membuangnya bila di-override dengan `pl-*`/`pr-*`. Hanya `px-*` simetris yang
  // menang, jadi ruang ikon dibuat via px-10 / px-11 (terverifikasi di browser).
  return (
    <form
      action={MODE_MOCK ? undefined : "/api/auth/login"}
      onSubmit={MODE_MOCK ? onSubmit : undefined}
      method="POST"
      className="mt-6 space-y-3.5"
      noValidate
    >
      <input type="hidden" name="redirect" value={redirect} />
      {/* Email */}
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

      {/* Kata sandi */}
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-3">
          <Label htmlFor="sandi">Kata sandi</Label>
          <Link
            href="/lupa-password"
            className="shrink-0 text-xs font-medium text-data-600 underline-offset-4 transition-colors hover:text-data-700 hover:underline dark:text-data-300 dark:hover:text-data-200"
          >
            Lupa kata sandi?
          </Link>
        </div>
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
            disabled={loading}
            aria-invalid={!!galat}
            className="h-11 px-11"
            required
          />
          <button
            type="button"
            onClick={() => setLihat((v) => !v)}
            disabled={loading}
            aria-label={
              lihat ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"
            }
            aria-pressed={lihat}
            className="absolute top-1/2 right-1.5 grid size-8 -translate-y-1/2 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-black/5 hover:text-foreground dark:hover:bg-white/10"
          >
            {lihat ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
          </button>
        </div>
      </div>

      {/* Ingat saya */}
      <div className="flex items-center gap-2">
        <Checkbox
          id="ingat"
          checked={ingat}
          onCheckedChange={(v) => setIngat(v === true)}
          disabled={loading}
        />
        <Label
          htmlFor="ingat"
          className="text-sm font-normal text-muted-foreground"
        >
          Tetap masuk di perangkat ini
        </Label>
      </div>

      {galat && (
        <p role="alert" className="text-sm font-medium text-destructive">
          {galat}
        </p>
      )}

      {/* Tombol masuk — CTA utama bergaya navy sesuai desain Garda */}
      <button
        type="submit"
        className={cn(
          "flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-b from-navy-700 to-navy-800 text-[15px] font-medium text-white shadow-ambient transition-all",
          "hover:from-navy-600 hover:to-navy-700 hover:shadow-lift focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
        )}
      >
        Masuk <ArrowRight className="size-4" />
      </button>

      {MODE_MOCK && (
        <p className="rounded-lg border border-dashed border-border bg-muted/40 px-3 py-2 text-center text-xs leading-relaxed text-muted-foreground">
          <span className="font-medium text-foreground">Akun demo</span>{" "}
          <span className="num">{DEMO_EMAIL}</span> ·{" "}
          <span className="num">{DEMO_SANDI}</span>
        </p>
      )}
    </form>
  );
}
