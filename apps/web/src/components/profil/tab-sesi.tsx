"use client";

import { DaftarSesi } from "@/components/akun/daftar-sesi";
import { PanelPas } from "@/components/garda/halaman-pas";

/** Tab "Perangkat & Sesi Aktif": perangkat yang sedang masuk beserta pencabutannya. Daftar bergulir di panelnya. */
export function TabSesi() {
  return (
    <PanelPas
      judul="Perangkat yang sedang masuk"
      deskripsi={
        <span className="whitespace-normal">
          Cabut sesi yang tidak Anda kenali; perangkat itu langsung keluar dan
          harus memasukkan kata sandi lagi.
        </span>
      }
      tanpaPadding
      isiClassName="flex flex-col lg:overflow-hidden"
      className="rounded-xl lg:max-h-full"
    >
      <DaftarSesi />
    </PanelPas>
  );
}
