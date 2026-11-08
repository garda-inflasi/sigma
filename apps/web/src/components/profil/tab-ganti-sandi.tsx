"use client";

import { FormGantiSandi } from "@/components/akun/form-ganti-sandi";
import type { ModeAkun } from "@/components/akun/mode-akun";
import { PanelTitle } from "@/components/garda/page-header";

const MODE_TAB: ModeAkun = { jenis: "tab" };

/** Tab "Ganti Kata Sandi": form bersama dengan mode tab, jadi pengguna tetap di /profil sesudah berhasil. */
export function TabGantiSandi() {
  return (
    <div className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Ganti kata sandi"
        description="Pakai kata sandi yang kuat dan belum pernah dipakai di layanan lain."
      />
      <div className="mt-6 max-w-md">
        <FormGantiSandi mode={MODE_TAB} />
      </div>
    </div>
  );
}
