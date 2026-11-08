"use client";

import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  Check,
  Circle,
  Loader2,
  MonitorSmartphone,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { KolomSandi } from "@/components/auth/kolom-sandi";
import { KotakGalatAuth, useTungguGalat } from "@/components/auth/galat-auth";
import { FormBff } from "@/components/form/form-bff";
import { SpringButton } from "@/components/garda/spring-button";
import { bffFetch } from "@/lib/api/client";
import { evaluatePassword } from "@/lib/auth/password-policy";
import {
  galatAuth,
  masukanDariError,
  pesanTampil,
  type GalatAuth,
} from "@/lib/auth/pesan-login";
import { skemaGantiSandi } from "@/lib/auth/skema-sandi";
import { useFormBff } from "@/lib/form/use-form-bff";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";
import { MeterKekuatanSandi } from "./meter-kekuatan-sandi";
import type { ModeAkun } from "./mode-akun";

/**
 * Form ganti kata sandi: meter kekuatan dan aturan kebijakan tampil langsung saat mengetik. Di halaman auth
 * (alur wajib) berpindah ke `lanjut` hanya sesudah 2xx; di tab /profil tetap di tempat dan form dikosongkan.
 * Server mengganti sesi ini dengan token baru (cookie dipasang BFF) dan mengeluarkan semua perangkat lain.
 */
export function FormGantiSandi({ mode }: { mode: ModeAkun }) {
  const qc = useQueryClient();
  const [lama, setLama] = useState("");
  const [baru, setBaru] = useState("");
  const [konfirmasi, setKonfirmasi] = useState("");
  const form = useFormBff<
    { lama: string; baru: string; konfirmasi: string },
    unknown,
    GalatAuth
  >({
    skema: skemaGantiSandi,
    kirim: (n) =>
      bffFetch("/api/auth/change-password", {
        method: "POST",
        body: JSON.stringify({ old_password: n.lama, new_password: n.baru }),
      }),
    sukses: () => {
      if (mode.jenis === "halaman") {
        toast.success("Kata sandi diganti", {
          description: "Mengalihkan ke ruang kerja Anda…",
        });
        // Token baru tanpa klaim wajib ganti: root layout (sesi) dirender ulang.
        return { ke: mode.lanjut, segarkan: true };
      }
      toast.success("Kata sandi diganti", {
        description: "Perangkat lain telah dikeluarkan.",
      });
      setLama("");
      setBaru("");
      setKonfirmasi("");
      // Sesi lain sudah dicabut server: daftar sesi dan data akun dimuat ulang.
      void qc.invalidateQueries({ queryKey: qk.profil.saya() });
      void qc.invalidateQueries({ queryKey: qk.sesi.aktif() });
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
    <FormBff
      onKirim={form.kirimDari(() => ({ lama, baru, konfirmasi }))}
      sibuk={loading}
      className="space-y-3.5"
    >
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

      {/* Tinggi tetap: meter muncul saat mengetik tanpa menggeser aturan dan kolom di bawahnya. */}
      <div className="flex h-5 items-center">
        <MeterKekuatanSandi sandi={baru} className="w-full" />
      </div>

      <ul className="space-y-1 text-xs" aria-label="Aturan kata sandi">
        {aturan.map((r) => (
          <li
            key={r.id}
            className={cn(
              "flex items-center gap-2 transition-colors",
              r.terpenuhi
                ? "text-normal-ink dark:text-normal-soft"
                : "text-muted-foreground",
            )}
          >
            {r.terpenuhi ? (
              <Check className="size-3.5" aria-hidden />
            ) : (
              <Circle className="size-3.5" aria-hidden />
            )}
            <span className="sr-only">
              {r.terpenuhi ? "Terpenuhi: " : "Belum terpenuhi: "}
            </span>
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

      <p className="flex gap-2.5 rounded-xl bg-data-50/70 p-3 text-xs leading-relaxed text-data-800 dark:bg-data-500/10 dark:text-data-200">
        <MonitorSmartphone className="mt-px size-4 shrink-0" aria-hidden />
        Perangkat lain akan dikeluarkan setelah kata sandi diganti.
      </p>

      <SpringButton
        type="submit"
        variant="primary"
        size="lg"
        disabled={loading || tertahan}
        className="h-11 w-full"
      >
        {mode.jenis === "halaman" ? "Simpan dan lanjutkan" : "Ganti kata sandi"}
        {loading ? (
          <Loader2 className="animate-spin" aria-hidden />
        ) : mode.jenis === "halaman" ? (
          <ArrowRight aria-hidden />
        ) : null}
      </SpringButton>
    </FormBff>
  );
}
