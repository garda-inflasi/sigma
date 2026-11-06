"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useCallback, useEffect, useRef, useState, useTransition } from "react";
import { toast } from "sonner";
import type { ZodType } from "zod";
import {
  galatFormBawaan,
  jalankanKiriman,
  type GalatField,
  type GalatForm,
  type HasilKiriman,
  type Navigasi,
} from "./kiriman";

export interface OpsiFormBff<TNilai, THasil, TGalat, TMasukan = TNilai> {
  /**
   * Skema nilai form; galatnya ditampilkan per field tanpa memanggil server. Boleh mengubah nilai
   * (trim/transform): `kirim` menerima hasil parse-nya, dan `TMasukan` adalah nilai mentah form.
   */
  skema?: ZodType<TNilai, TMasukan>;
  /** Panggilan asinkron ke BFF (JSON lewat `bffFetch`/`apiFetch`, atau `mutateAsync` React Query). */
  kirim: (nilai: TNilai) => Promise<THasil>;
  /** Sesudah 2xx. Kembalikan path (atau `Navigasi`) untuk berpindah halaman lewat router App Router. */
  sukses?: (hasil: THasil, nilai: TNilai) => Navigasi | string | void | Promise<Navigasi | string | void>;
  /** Sesudah kiriman ditolak server (bukan galat skema), mis. untuk mengosongkan kata sandi. */
  gagal?: (galat: TGalat, nilai: TMasukan) => void;
  /** Galat kiriman menjadi galat tingkat form. Bawaan: `galatFormBawaan`. */
  petaGalat?: (err: unknown) => TGalat;
  /** Galat skema tanpa field menjadi galat tingkat form. Bawaan: `{ pesan }` (cocok untuk `GalatForm`). */
  galatDariPesan?: (pesan: string) => TGalat;
  /** Judul toast saat gagal (boleh bergantung pada nilai yang dikirim); tanpa ini galat hanya tampil di form. */
  judulToastGagal?: string | ((nilai: TMasukan) => string | undefined);
  /** Pesan toast dari galat (bawaan: properti `pesan` bila ada). */
  pesanToast?: (galat: TGalat) => string | undefined;
}

export interface FormBff<THasil, TGalat, TMasukan> {
  /** True selama kiriman berjalan (dan selama navigasi sesudah sukses). */
  pending: boolean;
  /** Galat tingkat form dari kiriman terakhir. */
  galat: TGalat | null;
  /** Galat per field (skema klien atau 400/422 dari server). */
  galatField: GalatField<TMasukan>;
  setGalat: (galat: TGalat | null) => void;
  /** Menghapus galat (mis. saat pengguna mulai mengetik ulang). */
  resetGalat: () => void;
  /**
   * Handler `onSubmit` yang selalu memanggil `preventDefault()`: form tidak pernah dikirim browser secara
   * native (tidak ada reload, tidak ada POST di riwayat yang memicu "Confirm Form Resubmission").
   */
  kirimDari: (ambilNilai: () => TMasukan) => (e: FormEvent<HTMLFormElement>) => void;
  /** Kiriman tanpa event form (mis. tombol di luar form). Kiriman ganda saat masih berjalan diabaikan. */
  kirimNilai: (nilai: TMasukan) => Promise<HasilKiriman<THasil, TGalat, TMasukan> | null>;
}

const galatDariPesanBawaan = <TGalat,>(pesan: string) => ({ pesan }) as TGalat;

/**
 * Satu-satunya cara form di aplikasi ini mengirim data: asinkron ke BFF, state validasi/loading/galat di
 * klien, navigasi lewat `useRouter` hanya sesudah 2xx. Lihat docs/FORM.md.
 */
export function useFormBff<TNilai, THasil = unknown, TGalat = GalatForm, TMasukan = TNilai>(
  opsi: OpsiFormBff<TNilai, THasil, TGalat, TMasukan>,
): FormBff<THasil, TGalat, TMasukan> {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [galat, setGalat] = useState<TGalat | null>(null);
  const [galatField, setGalatField] = useState<GalatField<TMasukan>>({});
  // Navigasi sesudah sukses berjalan sebagai transisi: form tetap "pending" sampai halaman tujuan tampil.
  const [menavigasi, mulaiNavigasi] = useTransition();
  // Ref, bukan state: dua klik dalam satu frame tidak boleh lolos sebelum state terbarui.
  const sedangKirim = useRef(false);
  const navigasiBerjalan = useRef(false);
  // Bila komponen masih terpasang sesudah navigasi selesai (mis. tujuan = halaman yang sama), form dibuka lagi.
  useEffect(() => {
    if (!menavigasi && navigasiBerjalan.current) {
      navigasiBerjalan.current = false;
      sedangKirim.current = false;
    }
  }, [menavigasi]);
  // Opsi terbaru dibaca saat kiriman dimulai (selalu sesudah commit), tanpa membuat ulang handler.
  const opsiRef = useRef(opsi);
  useEffect(() => {
    opsiRef.current = opsi;
  });

  const resetGalat = useCallback(() => {
    setGalat(null);
    setGalatField({});
  }, []);

  const kirimNilai = useCallback(
    async (nilai: TMasukan) => {
      if (sedangKirim.current) return null;
      sedangKirim.current = true;
      const o = opsiRef.current;
      setGalat(null);
      setGalatField({});
      setPending(true);

      const hasil = await jalankanKiriman<TNilai, THasil, TGalat, TMasukan>({
        nilai,
        skema: o.skema,
        kirim: o.kirim,
        sukses: o.sukses,
        petaGalat: o.petaGalat ?? (galatFormBawaan as (err: unknown) => TGalat),
      });

      setPending(false);
      if (hasil.jenis === "berhasil" && hasil.navigasi) {
        // Kiriman ulang tetap ditolak sampai transisi navigasi selesai (lihat efek di atas).
        const nav = hasil.navigasi;
        navigasiBerjalan.current = true;
        mulaiNavigasi(() => {
          if (nav.mode === "push") router.push(nav.ke);
          else router.replace(nav.ke);
          if (nav.segarkan) router.refresh();
        });
        return hasil;
      }

      sedangKirim.current = false;
      if (hasil.jenis === "tidak-valid") {
        setGalatField(hasil.galatField);
        if (hasil.pesanForm) setGalat((o.galatDariPesan ?? galatDariPesanBawaan<TGalat>)(hasil.pesanForm));
      } else if (hasil.jenis === "gagal") {
        setGalat(hasil.galat);
        setGalatField(hasil.galatField);
        o.gagal?.(hasil.galat, nilai);
        const judul = typeof o.judulToastGagal === "function" ? o.judulToastGagal(nilai) : o.judulToastGagal;
        if (judul) {
          const pesan = o.pesanToast?.(hasil.galat) ?? (hasil.galat as { pesan?: string } | null)?.pesan;
          toast.error(judul, pesan ? { description: pesan } : undefined);
        }
      }
      return hasil;
    },
    [router],
  );

  const kirimDari = useCallback(
    (ambilNilai: () => TMasukan) => (e: FormEvent<HTMLFormElement>) => {
      e.preventDefault();
      // Form di dalam dialog (portal) tetap bersarang di pohon React induknya: jangan memicu form luar.
      e.stopPropagation();
      void kirimNilai(ambilNilai());
    },
    [kirimNilai],
  );

  return { pending: pending || menavigasi, galat, galatField, setGalat, resetGalat, kirimDari, kirimNilai };
}
