"use client";

import { DaftarSesi } from "@/components/akun/daftar-sesi";
import { PanelTitle } from "@/components/garda/page-header";

/** Tab "Perangkat & Sesi Aktif": perangkat yang sedang masuk beserta pencabutannya. */
export function TabSesi() {
  return (
    <div className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Perangkat yang sedang masuk"
        description="Cabut sesi yang tidak Anda kenali; perangkat itu langsung keluar dan harus memasukkan kata sandi lagi."
      />
      <div className="mt-6">
        <DaftarSesi />
      </div>
    </div>
  );
}
