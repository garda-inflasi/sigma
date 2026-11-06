"use client";

import { ArrowRight, Check, Circle, Loader2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { KolomSandi } from "@/components/auth/kolom-sandi";
import { KotakGalatAuth, useTungguGalat } from "@/components/auth/galat-auth";
import { FormBff } from "@/components/form/form-bff";
import { bffFetch } from "@/lib/api/client";
import { evaluatePassword } from "@/lib/auth/password-policy";
import { galatAuth, masukanDariError, pesanTampil, type GalatAuth } from "@/lib/auth/pesan-login";
import { skemaGantiSandi } from "@/lib/auth/skema-sandi";
import { useFormBff } from "@/lib/form/use-form-bff";
import { cn } from "@/lib/utils";

/** Form ganti kata sandi: menampilkan aturan kebijakan secara langsung; berpindah halaman hanya sesudah 2xx. */
export function GantiPasswordForm({ redirect }: { redirect: string }) {
  const [lama, setLama] = useState("");
  const [baru, setBaru] = useState("");
  const [konfirmasi, setKonfirmasi] = useState("");
  const form = useFormBff<{ lama: string; baru: string; konfirmasi: string }, unknown, GalatAuth>({
    skema: skemaGantiSandi,
    kirim: (n) =>
      bffFetch("/api/auth/change-password", {
        method: "POST",
        body: JSON.stringify({ old_password: n.lama, new_password: n.baru }),
      }),
    sukses: () => {
      toast.success("Kata sandi diganti", { description: "Mengalihkan ke ruang kerja Anda…" });
      // Token baru tanpa klaim wajib ganti: root layout (sesi) dirender ulang.
      return { ke: redirect, segarkan: true };
    },
    // Kata sandi lama yang salah dijawab server apa adanya; kunci akun (423) membawa hitung mundur.
    petaGalat: (err) => galatAuth(masukanDariError(err), "ganti"),
    judulToastGagal: "Gagal mengganti kata sandi",
    pesanToast: (g) => pesanTampil(g),
  });
  const galat = form.galat;
  const loading = form.pending;
  const { sekarang, tertahan } = useTungguGalat(galat);

  const aturan = evaluatePassword(baru);

  return (
    <FormBff onKirim={form.kirimDari(() => ({ lama, baru, konfirmasi }))} sibuk={loading} className="mt-6 space-y-3.5">
      <KolomSandi
        id="lama"
        label="Kata sandi saat ini"
        value={lama}
        onChange={setLama}
        disabled={loading}
        galat={form.galatField.lama}
        autoComplete="current-password"
      />
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
    </FormBff>
  );
}
