"use client";

import { ShieldCheck, Smartphone } from "lucide-react";
import type { ModeAkun } from "@/components/akun/mode-akun";
import { PanelMfa } from "@/components/akun/panel-mfa";
import { PanelPas } from "@/components/garda/halaman-pas";

const MODE_TAB: ModeAkun = { jenis: "tab" };

/** Tab "Verifikasi 2 Langkah": panel bersama dengan mode tab dan kartu ringkasan cara kerja di sisi kanan. */
export function TabMfa() {
  return (
    <PanelPas
      judul="Verifikasi dua langkah"
      deskripsi={
        <span className="whitespace-normal">
          Selain kata sandi, Anda diminta kode dari aplikasi autentikator setiap
          kali masuk.
        </span>
      }
      className="rounded-xl lg:max-h-full"
    >
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,32rem)_minmax(0,1fr)]">
        {/* Tinggi minimum setara tahap tertinggi (QR dan kode): pergantian tahap tidak menggeser tata letak panel. */}
        <PanelMfa mode={MODE_TAB} className="min-w-0 max-w-lg min-h-[28rem]" />

        <aside
          aria-label="Informasi autentikator dua langkah"
          className="rounded-xl border border-border/80 bg-secondary/45 p-4 dark:bg-white/[0.03]"
        >
          <div className="flex items-center gap-2 text-sm font-semibold text-navy-900 dark:text-white">
            <ShieldCheck className="size-4 text-data-600 dark:text-data-400" aria-hidden />
            <h3>Mengapa mengaktifkan 2FA?</h3>
          </div>
          <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
            Verifikasi dua langkah melindungi akses data harga dan sistem kontrol
            wilayah meskipun kata sandi Anda diketahui pihak lain.
          </p>
          <div className="mt-3.5 border-t border-border/80 pt-3">
            <p className="flex items-center gap-1.5 text-xs font-medium text-foreground">
              <Smartphone className="size-3.5 text-muted-foreground" aria-hidden />
              Aplikasi autentikator yang didukung
            </p>
            <ul className="mt-2 flex flex-wrap gap-1.5 text-[11px] text-muted-foreground">
              {["Google Authenticator", "Microsoft Authenticator", "Authy", "1Password"].map((app) => (
                <li
                  key={app}
                  className="rounded-md border border-border bg-card px-2 py-1 font-medium text-foreground/85"
                >
                  {app}
                </li>
              ))}
            </ul>
          </div>
        </aside>
      </div>
    </PanelPas>
  );
}
