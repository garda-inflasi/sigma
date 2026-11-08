"use client";

import type { ModeAkun } from "@/components/akun/mode-akun";
import { PanelMfa } from "@/components/akun/panel-mfa";
import { PanelTitle } from "@/components/garda/page-header";

const MODE_TAB: ModeAkun = { jenis: "tab" };

/** Tab "Verifikasi 2 Langkah": panel bersama dengan mode tab (setup, kode cadangan, dan nonaktifkan di tempat). */
export function TabMfa() {
  return (
    <div className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Verifikasi dua langkah"
        description="Selain kata sandi, Anda diminta kode dari aplikasi autentikator setiap kali masuk."
      />
      {/* Tinggi minimum setara tahap tertinggi (QR dan kode): pergantian tahap tidak menggeser tata letak halaman. */}
      <PanelMfa mode={MODE_TAB} className="mt-6 max-w-lg min-h-[28rem]" />
    </div>
  );
}
