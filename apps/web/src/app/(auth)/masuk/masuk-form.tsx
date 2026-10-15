"use client";

import { ArrowRight, Eye, EyeOff, Loader2, Lock, Mail } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { API_BASE, ApiError, MODE_MOCK } from "@/lib/api/client";
import { cn } from "@/lib/utils";

/** Logo "G" Google empat warna — SVG inline agar tidak bergantung aset eksternal. */
function GoogleGlyph({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 48 48" className={className} aria-hidden>
      <path
        fill="#4285F4"
        d="M45.12 24.5c0-1.56-.14-3.06-.4-4.5H24v8.51h11.84c-.51 2.75-2.06 5.08-4.39 6.64v5.52h7.11c4.16-3.83 6.56-9.47 6.56-16.17z"
      />
      <path
        fill="#34A853"
        d="M24 46c5.94 0 10.92-1.97 14.56-5.33l-7.11-5.52c-1.97 1.32-4.49 2.1-7.45 2.1-5.73 0-10.58-3.87-12.31-9.07H4.34v5.7C7.96 41.07 15.4 46 24 46z"
      />
      <path
        fill="#FBBC05"
        d="M11.69 28.18c-.44-1.32-.69-2.73-.69-4.18s.25-2.86.69-4.18v-5.7H4.34A21.97 21.97 0 0 0 2 24c0 3.55.85 6.91 2.34 9.88l7.35-5.7z"
      />
      <path
        fill="#EA4335"
        d="M24 10.75c3.23 0 6.13 1.11 8.41 3.29l6.31-6.31C34.91 4.18 29.93 2 24 2 15.4 2 7.96 6.93 4.34 14.12l7.35 5.7C13.42 14.62 18.27 10.75 24 10.75z"
      />
    </svg>
  );
}

// Akun demo (mode mock, tanpa backend) — satu-satunya kredensial yang diterima.
const DEMO_EMAIL = "khaidirhasan@gmail.com";
const DEMO_SANDI = "acehkukaya";

/**
 * Form masuk institusi: email + kata sandi (lapis utama), dengan alternatif SSO Google.
 * Di mode backend (NEXT_PUBLIC_API_BASE_URL aktif): POST ke layanan auth Go yang
 * menerbitkan cookie httpOnly `garda_session`; di mode mock UI berjalan tanpa backend.
 */
export function MasukForm({
  redirect,
  googleSso,
}: {
  redirect: string;
  googleSso: string;
}) {
  const router = useRouter();
  const [email, setEmail] = useState(MODE_MOCK ? DEMO_EMAIL : "");
  const [sandi, setSandi] = useState(MODE_MOCK ? DEMO_SANDI : "");
  const [lihat, setLihat] = useState(false);
  const [ingat, setIngat] = useState(true);
  const [loading, setLoading] = useState(false);
  const [galat, setGalat] = useState<string | null>(null);

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

      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify({ email: email.trim(), password: sandi }),
      });

      if (!res.ok) {
        let pesan =
          res.status === 401
            ? "Email atau kata sandi salah."
            : `Gagal masuk (${res.status}).`;
        try {
          const data = (await res.json()) as {
            message?: string;
          };
          if (data.message) pesan = data.message;
        } catch {}
        throw new ApiError(res.status, pesan);
      }

      // Cookies are set by the BFF — reload to let SSR read the new session
      toast.success("Berhasil masuk");
      window.location.assign(redirect);
    } catch (err) {
      const pesan =
        err instanceof ApiError
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
    <form onSubmit={onSubmit} className="mt-6 space-y-3.5" noValidate>
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
        disabled={loading}
        className={cn(
          "flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-b from-navy-700 to-navy-800 text-[15px] font-medium text-white shadow-ambient transition-all",
          "hover:from-navy-600 hover:to-navy-700 hover:shadow-lift focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
          "disabled:cursor-not-allowed disabled:opacity-70",
        )}
      >
        {loading ? (
          <>
            <Loader2 className="size-4 animate-spin" /> Memverifikasi…
          </>
        ) : (
          <>
            Masuk <ArrowRight className="size-4" />
          </>
        )}
      </button>

      {/* Pemisah */}
      <div className="flex items-center gap-3 text-xs text-muted-foreground">
        <span className="h-px flex-1 bg-border" />
        atau
        <span className="h-px flex-1 bg-border" />
      </div>

      {/* SSO Google */}
      <a
        href={googleSso}
        className={cn(
          "flex h-11 w-full items-center justify-center gap-3 rounded-xl border border-input bg-white/80 text-[15px] font-medium text-navy-800 shadow-ambient-sm transition-all",
          "hover:bg-white hover:shadow-ambient focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
          "dark:bg-white/[0.06] dark:text-white dark:hover:bg-white/[0.1]",
          loading && "pointer-events-none opacity-70",
        )}
      >
        <GoogleGlyph className="size-5" />
        Masuk dengan Google
      </a>

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
