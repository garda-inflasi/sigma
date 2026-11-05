"use client";

import { ArrowRight, Check, Circle, Loader2, Lock } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { KotakGalatAuth, useGalatAuth } from "@/components/auth/galat-auth";
import { bffFetch } from "@/lib/api/client";
import {
  galatAuth,
  galatLokal,
  masukanDariError,
  pesanTampil,
} from "@/lib/auth/pesan-login";
import { evaluatePassword, isPasswordValid } from "@/lib/auth/password-policy";
import { cn } from "@/lib/utils";

/** Form ganti kata sandi: menampilkan aturan kebijakan secara langsung. */
export function GantiPasswordForm({ redirect }: { redirect: string }) {
  const [lama, setLama] = useState("");
  const [baru, setBaru] = useState("");
  const [konfirmasi, setKonfirmasi] = useState("");
  const [loading, setLoading] = useState(false);
  const { galat, setGalat, sekarang, tertahan } = useGalatAuth();

  const aturan = evaluatePassword(baru);
  const cocok = baru !== "" && baru === konfirmasi;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (loading) return;
    setGalat(null);

    if (!lama) return setGalat(galatLokal("Kata sandi saat ini wajib diisi."));
    if (!isPasswordValid(baru))
      return setGalat(
        galatLokal("Kata sandi baru belum memenuhi semua aturan."),
      );
    if (!cocok) return setGalat(galatLokal("Konfirmasi kata sandi tidak sama."));

    setLoading(true);
    try {
      await bffFetch("/api/auth/change-password", {
        method: "POST",
        body: JSON.stringify({ old_password: lama, new_password: baru }),
      });
      toast.success("Kata sandi diganti", {
        description: "Mengalihkan ke ruang kerja Anda…",
      });
      window.location.assign(redirect);
    } catch (err) {
      // Kata sandi lama yang salah dijawab server apa adanya; kunci akun (423) membawa hitung mundur.
      const hasil = galatAuth(masukanDariError(err), "ganti");
      setGalat(hasil);
      toast.error("Gagal mengganti kata sandi", {
        description: pesanTampil(hasil),
      });
      setLoading(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="mt-6 space-y-3.5" noValidate>
      <KolomSandi
        id="lama"
        label="Kata sandi saat ini"
        value={lama}
        onChange={setLama}
        disabled={loading}
        autoComplete="current-password"
      />
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

      {galat && <KotakGalatAuth galat={galat} sekarang={sekarang} />}

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
            Simpan dan lanjutkan <ArrowRight className="size-4" />
          </>
        )}
      </button>
    </form>
  );
}

function KolomSandi({
  id,
  label,
  value,
  onChange,
  disabled,
  autoComplete,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (v: string) => void;
  disabled: boolean;
  autoComplete: string;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <Lock className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          id={id}
          type="password"
          autoComplete={autoComplete}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          className="h-11 px-10"
          required
        />
      </div>
    </div>
  );
}
