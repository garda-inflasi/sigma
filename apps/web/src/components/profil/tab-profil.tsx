"use client";

import { MODE_MOCK } from "@/lib/api/client";
import { useSession } from "@/lib/auth/session-context";
import type { MeUserResponse } from "@/lib/bff/api-client";
import { galatFormBawaan } from "@/lib/form/kiriman";
import { useNamaKabupaten, useProfil } from "@/lib/query/profil-hooks";
import { ROLE_PERMISSIONS } from "@/lib/rbac/permissions";
import { ROLES } from "@/lib/rbac/roles";
import { FormDataPribadi } from "./data-diri-form";
import { PenugasanWilayah } from "./data-diri-penugasan";
import { RingkasanAkun } from "./data-diri-ringkasan";
import { DataDiriGalat, DataDiriMemuat } from "./data-diri-status";

/** Bagian profil yang ditampilkan tab ini; `dibuat_pada` dan `telepon` tidak ada di mode demo. */
type ProfilTampil = Pick<
  MeUserResponse,
  | "nama"
  | "email"
  | "peran"
  | "lapis"
  | "lembaga"
  | "kabupaten_kode"
  | "status"
  | "telepon"
> &
  Partial<Pick<MeUserResponse, "dibuat_pada">>;

function IsiDataDiri({
  profil,
  bacaSaja = false,
}: {
  profil: ProfilTampil;
  bacaSaja?: boolean;
}) {
  const sesi = useSession();
  const namaKabupaten = useNamaKabupaten(profil.kabupaten_kode);
  // Izin yang dipakai UI: klaim token sesi, atau matriks bawaan peran bila token tidak membawanya.
  const izin = sesi.permissions ?? ROLE_PERMISSIONS[sesi.role];

  return (
    <div className="space-y-5">
      <RingkasanAkun
        nama={profil.nama}
        peran={profil.peran}
        status={profil.status}
        dibuatPada={profil.dibuat_pada}
      />
      <FormDataPribadi
        tersimpan={profil}
        email={profil.email}
        bacaSaja={bacaSaja}
      />
      <PenugasanWilayah
        lembaga={profil.lembaga}
        peran={profil.peran}
        lapis={profil.lapis}
        kabupatenKode={profil.kabupaten_kode}
        namaKabupaten={namaKabupaten}
        izin={izin}
      />
    </div>
  );
}

/** Profil dari server (BFF auth); memuat, galat dengan "Coba lagi", lalu isinya. */
function TabProfilServer() {
  const profil = useProfil();
  if (profil.isPending) return <DataDiriMemuat />;
  if (!profil.data) {
    return (
      <DataDiriGalat
        pesan={galatFormBawaan(profil.error).pesan}
        mengulang={profil.isFetching}
        onUlangi={() => void profil.refetch()}
      />
    );
  }
  return <IsiDataDiri profil={profil.data} />;
}

/**
 * Mode demo (tanpa API): mode ini tidak punya BFF auth, jadi yang tampil adalah data sesi, hanya-baca. Sesi
 * tidak membawa telepon, status, dan tanggal bergabung; akun yang masuk dianggap aktif.
 */
export function TabProfilDemo() {
  const sesi = useSession();
  return (
    <IsiDataDiri
      bacaSaja
      profil={{
        nama: sesi.nama,
        email: sesi.email,
        peran: sesi.role,
        lapis: ROLES[sesi.role].lapis,
        lembaga: sesi.lembaga,
        kabupaten_kode: sesi.kabupatenKode,
        status: "AKTIF",
        telepon: undefined,
      }}
    />
  );
}

/**
 * Tab "Edit Profil": ringkasan akun, form data pribadi (nama dan telepon dapat diubah, email tidak), dan
 * penugasan & wilayah yang hanya-baca beserta hak akses efektif.
 */
export function TabProfil() {
  return MODE_MOCK ? <TabProfilDemo /> : <TabProfilServer />;
}
