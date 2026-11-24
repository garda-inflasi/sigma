"use client";

import { KeyRound, ShieldCheck, Sparkles } from "lucide-react";
import { FormGantiSandi } from "@/components/akun/form-ganti-sandi";
import type { ModeAkun } from "@/components/akun/mode-akun";
import { PanelPas } from "@/components/garda/halaman-pas";

const MODE_TAB: ModeAkun = { jenis: "tab" };

/** Tab "Ganti Kata Sandi": form bersama dengan mode tab dan kartu panduan keamanan di sisi kanan. */
export function TabGantiSandi() {
  return (
    <PanelPas
      judul="Ganti kata sandi"
      deskripsi={
        <span className="whitespace-normal">
          Pakai kata sandi yang kuat dan belum pernah dipakai di layanan lain.
        </span>
      }
      className="rounded-xl lg:max-h-full"
    >
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,28rem)_minmax(0,1fr)]">
        <div className="min-w-0">
          <FormGantiSandi mode={MODE_TAB} />
        </div>

        <aside
          aria-label="Panduan keamanan kata sandi"
          className="rounded-xl border border-border/80 bg-secondary/45 p-4 dark:bg-white/[0.03]"
        >
          <div className="flex items-center gap-2 text-sm font-semibold text-navy-900 dark:text-white">
            <KeyRound className="size-4 text-data-600 dark:text-data-400" aria-hidden />
            <h3>Praktik kata sandi yang aman</h3>
          </div>
          <ul className="mt-3 space-y-2.5 text-xs leading-relaxed text-muted-foreground">
            <li className="flex items-start gap-2">
              <Sparkles className="mt-0.5 size-3.5 shrink-0 text-data-600 dark:text-data-400" aria-hidden />
              <span>
                Gunakan frasa sandi minimal 12 karakter agar kekuatan mencapai tingkat <strong className="text-foreground">Kuat</strong>.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-normal-ink dark:text-emerald-300" aria-hidden />
              <span>
                Hindari memakai kata sandi yang sama dengan email dinas, perbankan, atau akun pribadi lainnya.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-normal-ink dark:text-emerald-300" aria-hidden />
              <span>
                Setelah berhasil disimpan, hanya sesi di perangkat ini yang tetap masuk untuk mencegah akses tidak sah.
              </span>
            </li>
          </ul>
        </aside>
      </div>
    </PanelPas>
  );
}
