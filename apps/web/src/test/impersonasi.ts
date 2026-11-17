/**
 * Bahan tes komponen impersonasi: sesi yang sedang impersonate, daftar sasaran, kebijakan, riwayat, dan
 * penangkap navigasi (navigasi sesudah berpindah akun adalah pemuatan penuh, tidak boleh benar-benar terjadi).
 */
import type {
  KebijakanWire,
  RiwayatImpersonasiWire,
  TargetsWire,
} from "@/lib/api/impersonasi";
import type { SessionUser } from "@/lib/auth/resolve";
import { navigasi } from "@/lib/query/impersonasi-hooks";
import { ROLE_PERMISSIONS, type Permission } from "@/lib/rbac/permissions";
import type { Role } from "@/lib/rbac/roles";
import { penggunaSesi } from "./render";

export const ID_PELAKU = "22222222-2222-4222-8222-222222222222";
export const ID_SASARAN = "11111111-1111-4111-8111-111111111111";
export const ID_SESI = "33333333-3333-4333-8333-333333333333";

/** Sesi seorang superadmin yang sedang impersonate akun berperan `peran` (izin = matriks bawaan peran itu). */
export function sesiImpersonate(
  peran: Role = "OPS_KOORDINATOR",
  sisip: Partial<SessionUser> = {},
): SessionUser {
  return penggunaSesi({
    id: ID_SASARAN,
    nama: "Zulkifli, S.P.",
    email: "zul@garda.test",
    role: peran,
    lembaga: "FMIPA USK",
    permissions: [...ROLE_PERMISSIONS[peran]] as Permission[],
    impersonation: {
      active: true,
      originalUser: {
        id: ID_PELAKU,
        name: "Super Admin",
        email: "superadmin@gardainflasi.id",
        role: "ADMIN_SISTEM",
      },
      targetUser: { id: ID_SASARAN, name: "Zulkifli, S.P.", role: peran },
      startedAt: "2026-11-17T10:00:00.000Z",
    },
    ...sisip,
  });
}

/** Menangkap tujuan pemindahan halaman; `pulihkan` mengembalikan fungsi aslinya. */
export function tangkapNavigasi() {
  const asli = navigasi.pindah;
  const tujuan: string[] = [];
  navigasi.pindah = (url) => void tujuan.push(url);
  return { tujuan, pulihkan: () => void (navigasi.pindah = asli) };
}

export function targetsFixture(sisip: Partial<TargetsWire> = {}): TargetsWire {
  return {
    boleh: true,
    aktif: true,
    wajib_alasan: true,
    durasi_maks_menit: 60,
    peran: [
      {
        peran: "STRATEGIS_BI",
        label: "Analis Bank Indonesia",
        lapis: "STRATEGIS",
        akun: [
          { id: "a-bi", nama: "Budi BI", email: "budi.bi@x.id", lembaga: "Bank Indonesia", kabupaten_kode: null },
        ],
      },
      {
        peran: "OPS_KOORDINATOR",
        label: "Koordinator Lapangan",
        lapis: "OPERASIONAL",
        akun: [
          { id: "a-k1", nama: "Zulkifli", email: "zul@x.id", lembaga: "FMIPA USK", kabupaten_kode: "1101" },
          { id: "a-k2", nama: "Siti Koordinator", email: "siti@x.id", lembaga: "FMIPA USK", kabupaten_kode: "1102" },
        ],
      },
      {
        peran: "OPS_MAHASISWA",
        label: "Agen Lapangan Mahasiswa",
        lapis: "OPERASIONAL",
        akun: [
          { id: "a-m1", nama: "Ani Mahasiswa", email: "ani@x.id", lembaga: "FMIPA USK", kabupaten_kode: "1101" },
        ],
      },
    ],
    ...sisip,
  };
}

export function kebijakanFixture(sisip: Partial<KebijakanWire> = {}): KebijakanWire {
  return {
    aktif: true,
    wajib_alasan: true,
    durasi_maks_menit: 60,
    versi: 3,
    diperbarui_pada: "2026-11-16T10:00:00Z",
    diperbarui_oleh: ID_PELAKU,
    diperbarui_oleh_nama: "Super Admin",
    pelaku: [
      {
        tipe: "PERAN",
        peran: "MANAJER_MUTU",
        aktif: true,
        target: [{ tipe: "PERAN", peran: "OPS_MAHASISWA" }],
      },
    ],
    ...sisip,
  };
}

export function riwayatFixture(sisip: Partial<RiwayatImpersonasiWire> = {}): RiwayatImpersonasiWire {
  return {
    id: ID_SESI,
    pelaku: { id: ID_PELAKU, nama: "Super Admin", email: "superadmin@gardainflasi.id", peran: "ADMIN_SISTEM" },
    sasaran: { id: ID_SASARAN, nama: "Zulkifli", email: "zul@x.id", peran: "OPS_KOORDINATOR" },
    alasan: "uji dasbor",
    mulai_pada: "2026-11-17T10:00:00Z",
    kedaluwarsa_pada: "2026-11-17T11:00:00Z",
    berakhir_pada: null,
    alasan_berakhir: null,
    aktif: true,
    ip: "10.0.0.1",
    perangkat: "Chrome di macOS",
    ...sisip,
  };
}

/**
 * Memberi jeda sesudah cleanup supaya timer sisa (Radix menutup dialog dan mengembalikan fokus lewat setTimeout)
 * menyala SEBELUM happy-dom di-unregister di akhir berkas; bila tidak, di mesin yang sibuk timer itu menyala sesudahnya
 * dan melempar "window is not defined". Pasang `afterEach(tuntaskanTimer)` di tes yang membuka dialog.
 */
export const tuntaskanTimer = () => new Promise<void>((selesai) => setTimeout(selesai, 25));
