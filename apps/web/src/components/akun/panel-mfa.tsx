"use client";

import { useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import {
  ArrowRight,
  Check,
  Copy,
  Download,
  KeyRound,
  Loader2,
  ShieldAlert,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { FormBff } from "@/components/form/form-bff";
import { Shimmer } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { bffFetch } from "@/lib/api/client";
import { rapikanKode, teksKodeCadangan, validasiKode } from "@/lib/auth/mfa";
import { safeInternalPath } from "@/lib/auth/safe-path";
import type { MfaSetupResponse, UserResponse } from "@/lib/bff/api-client";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { galatFormBawaan } from "@/lib/form/kiriman";
import { useFormBff } from "@/lib/form/use-form-bff";
import { fadeIn } from "@/lib/motion/presets";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";
import { DialogSandi } from "./dialog-sandi";
import type { ModeAkun } from "./mode-akun";
import { kunciAkunKeamanan, useAkunKeamanan } from "./use-akun";

type Tahap = "memuat" | "awal" | "kode" | "cadangan" | "aktif";
/** Dialog konfirmasi kata sandi yang sedang terbuka. */
type DialogAksi = "kode-baru" | "nonaktifkan";

const NAMA_BERKAS_KODE = "kode-cadangan-garda-inflasi.txt";

const sandiWajib = z.string().min(1, "Kata sandi wajib diisi.");

/** Aksi panel MFA; satu kiriman berjalan pada satu waktu dan berbagi status sibuk serta galat. */
const skemaAksiMfa = z.discriminatedUnion("aksi", [
  z.object({ aksi: z.literal("mulai") }),
  z.object({
    aksi: z.literal("verifikasi"),
    code: z
      .string()
      .transform(rapikanKode)
      .superRefine((kode, ctx) => {
        const salah = validasiKode(kode);
        if (salah) ctx.addIssue({ code: "custom", message: salah });
      }),
  }),
  z.object({ aksi: z.literal("kode-baru"), password: sandiWajib }),
  z.object({ aksi: z.literal("nonaktifkan"), password: sandiWajib }),
]);
type AksiMfa = z.output<typeof skemaAksiMfa>;
/** Nilai mentah satu aksi (bentuk datar supaya galat per field dapat dibaca langsung). */
interface MasukanMfa {
  aksi: AksiMfa["aksi"];
  code?: string;
  password?: string;
}
type HasilMfa =
  | { aksi: "mulai"; setup: MfaSetupResponse }
  | { aksi: "verifikasi" | "kode-baru"; kodeCadangan: string[] }
  | { aksi: "nonaktifkan" };

const JUDUL_GAGAL: Record<AksiMfa["aksi"], string> = {
  mulai: "Gagal memulai setup",
  verifikasi: "Kode tidak diterima",
  "kode-baru": "Gagal membuat kode cadangan",
  nonaktifkan: "Gagal menonaktifkan MFA",
};

async function jalankanAksiMfa(n: AksiMfa): Promise<HasilMfa> {
  switch (n.aksi) {
    case "mulai":
      return {
        aksi: n.aksi,
        setup: await bffFetch<MfaSetupResponse>("/api/auth/mfa/setup", {
          method: "POST",
        }),
      };
    case "verifikasi": {
      const r = await bffFetch<{ kode_cadangan: string[] }>(
        "/api/auth/mfa/verify",
        { method: "POST", body: JSON.stringify({ code: n.code }) },
      );
      return { aksi: n.aksi, kodeCadangan: r.kode_cadangan };
    }
    case "kode-baru": {
      const r = await bffFetch<{ kode_cadangan: string[] }>(
        "/api/auth/mfa/backup-codes",
        { method: "POST", body: JSON.stringify({ password: n.password }) },
      );
      return { aksi: n.aksi, kodeCadangan: r.kode_cadangan };
    }
    case "nonaktifkan":
      await bffFetch("/api/auth/mfa/disable", {
        method: "POST",
        body: JSON.stringify({ password: n.password }),
      });
      return { aksi: n.aksi };
  }
}

/** Kotak imbauan/status berwarna (nada dari token status). */
function Imbauan({
  nada,
  ikon: Ikon,
  children,
}: {
  nada: Tone;
  ikon: LucideIcon;
  children: ReactNode;
}) {
  const t = toneClass[nada];
  return (
    <div
      className={cn("flex gap-2.5 rounded-xl p-3.5 text-sm", t.soft, t.text)}
    >
      <Ikon className="mt-px size-4 shrink-0" aria-hidden />
      <div className="min-w-0 space-y-0.5">{children}</div>
    </div>
  );
}

/**
 * Setup MFA (QR, kode pertama, kode cadangan sekali tampil) dan pengelolaannya (kode cadangan baru, nonaktifkan).
 * Tahap awal/aktif mengikuti `mfa_aktif` akun; tahap kode dan cadangan adalah alur lokal yang sedang berjalan.
 * Pada `mode` halaman (alur wajib setup MFA) tombol akhir berpindah ke `lanjut`; pada tab tetap di tempat.
 */
export function PanelMfa({
  mode,
  className,
}: {
  mode: ModeAkun;
  /** Kelas pembungkus tahap (mis. tinggi minimum agar pergantian tahap tidak menggeser tata letak). */
  className?: string;
}) {
  const router = useRouter();
  const qc = useQueryClient();
  const { data: akun, isPending, isError, error, refetch } = useAkunKeamanan();
  const [alur, setAlur] = useState<"kode" | "cadangan" | null>(null);
  const [setup, setSetup] = useState<MfaSetupResponse | null>(null);
  // Diambil saat setup dimulai: sesudah aktivasi server tidak lagi mewajibkannya, tetapi tombol akhir tahap
  // cadangan tetap harus melanjutkan ke tujuan semula.
  const [setupWajib, setSetupWajib] = useState(false);
  const [kode, setKode] = useState("");
  const [cadangan, setCadangan] = useState<string[]>([]);
  const [tersimpan, setTersimpan] = useState(false);
  const [sandi, setSandi] = useState("");
  const [dialog, setDialog] = useState<DialogAksi | null>(null);

  const tahap: Tahap =
    alur ?? (isPending ? "memuat" : akun?.mfa_aktif ? "aktif" : "awal");
  const galatMuat = isError ? galatFormBawaan(error).pesan : null;

  const ubahAkun = (ubah: Partial<UserResponse>) =>
    qc.setQueryData<UserResponse>(kunciAkunKeamanan(), (u) =>
      u ? { ...u, ...ubah } : u,
    );
  // Sesudah aktivasi, sesi lain dicabut server: daftar sesi ikut dimuat ulang.
  const segarkan = (sesiBerubah = false) => {
    void qc.invalidateQueries({ queryKey: qk.profil.saya() });
    if (sesiBerubah) void qc.invalidateQueries({ queryKey: qk.sesi.aktif() });
  };

  const tampilkanKode = (daftar: string[]) => {
    setCadangan(daftar);
    setTersimpan(false);
    setAlur("cadangan");
  };

  const aksi = useFormBff<AksiMfa, HasilMfa, { pesan: string }, MasukanMfa>({
    skema: skemaAksiMfa,
    kirim: jalankanAksiMfa,
    sukses: (h) => {
      switch (h.aksi) {
        case "mulai":
          setSetup(h.setup);
          setSetupWajib(akun?.mfa_setup_diwajibkan === true);
          setKode("");
          setAlur("kode");
          break;
        case "verifikasi":
          tampilkanKode(h.kodeCadangan);
          ubahAkun({
            mfa_aktif: true,
            kode_cadangan_tersisa: h.kodeCadangan.length,
          });
          segarkan(true);
          toast.success("MFA aktif", {
            description: "Simpan kode cadangan Anda sekarang.",
          });
          break;
        case "kode-baru":
          tampilkanKode(h.kodeCadangan);
          ubahAkun({ kode_cadangan_tersisa: h.kodeCadangan.length });
          segarkan();
          tutupDialog();
          toast.success("Kode cadangan baru dibuat", {
            description: "Kode cadangan lama tidak berlaku lagi.",
          });
          break;
        case "nonaktifkan":
          ubahAkun({ mfa_aktif: false, kode_cadangan_tersisa: undefined });
          segarkan();
          tutupDialog();
          toast.success("MFA dinonaktifkan");
          break;
      }
    },
    judulToastGagal: (n) => JUDUL_GAGAL[n.aksi],
  });
  const sibuk = aksi.pending;
  // Galat kiriman milik dialog tampil di dialog; selain itu di tahap yang sedang terbuka.
  const galat =
    aksi.galatField.code ??
    aksi.galatField.password ??
    aksi.galat?.pesan ??
    null;

  const mulai = () => void aksi.kirimNilai({ aksi: "mulai" });

  const bukaDialog = (d: DialogAksi) => {
    aksi.resetGalat();
    setSandi("");
    setDialog(d);
  };
  function tutupDialog() {
    aksi.resetGalat();
    setSandi("");
    setDialog(null);
  }

  // Halaman alur wajib berpindah ke tujuan semula; selain itu kembali ke tahap aktif di tempat.
  const melanjutkan = mode.jenis === "halaman" && setupWajib;
  const selesai = () => {
    if (melanjutkan) {
      // Kode cadangan sudah disimpan: klaim "wajib setup MFA" hilang dari sesi, root layout perlu dirender ulang.
      router.replace(safeInternalPath(mode.lanjut));
      router.refresh();
      return;
    }
    if (setupWajib) router.refresh();
    setCadangan([]);
    setSetup(null);
    setSetupWajib(false);
    setAlur(null);
  };

  async function salin(teks: string, pesan: string) {
    try {
      await navigator.clipboard.writeText(teks);
      toast.success(pesan);
    } catch {
      toast.error("Gagal menyalin", {
        description: "Salin secara manual dari tampilan di layar.",
      });
    }
  }

  function unduh() {
    const url = URL.createObjectURL(
      new Blob([teksKodeCadangan(cadangan, akun?.email ?? "")], {
        type: "text/plain",
      }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = NAMA_BERKAS_KODE;
    a.click();
    URL.revokeObjectURL(url);
  }

  const galatTampil = !dialog && galat && (
    <p role="alert" className="text-sm font-medium text-destructive">
      {galat}
    </p>
  );

  function isiTahap(): ReactNode {
    switch (tahap) {
      case "memuat":
        return (
          <div role="status" aria-label="Memuat" className="space-y-3">
            <Shimmer className="h-16 w-full rounded-xl" />
            <Shimmer className="h-4 w-4/5" />
            <Shimmer className="h-11 w-full rounded-xl" />
          </div>
        );

      case "awal":
        return (
          <>
            {akun &&
              (akun.mfa_setup_diwajibkan ? (
                <Imbauan nada="waspada" ikon={ShieldAlert}>
                  <p>
                    Peran Anda wajib memakai verifikasi dua langkah. Selesaikan
                    setup ini untuk melanjutkan.
                  </p>
                </Imbauan>
              ) : (
                <Imbauan nada="waspada" ikon={ShieldAlert}>
                  <p className="font-medium">
                    Akun Anda belum dilindungi verifikasi dua langkah.
                  </p>
                  <p>
                    Aktifkan agar akun tetap aman walau kata sandi Anda bocor.
                  </p>
                </Imbauan>
              ))}
            <p className="text-sm leading-relaxed text-muted-foreground">
              Anda memerlukan aplikasi autentikator (mis. Google Authenticator,
              Authy, atau 1Password) di ponsel.
            </p>
            {galatMuat && (
              <div
                role="alert"
                className="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm"
              >
                <p className="font-medium text-destructive">{galatMuat}</p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void refetch()}
                >
                  Coba lagi
                </Button>
              </div>
            )}
            {galatTampil}
            <SpringButton
              variant="primary"
              size="lg"
              onClick={mulai}
              disabled={sibuk}
              className="h-11 w-full"
            >
              Mulai setup
              {sibuk ? (
                <Loader2 className="animate-spin" aria-hidden />
              ) : (
                <ArrowRight aria-hidden />
              )}
            </SpringButton>
          </>
        );

      case "kode":
        if (!setup) return null;
        return (
          <FormBff
            onKirim={aksi.kirimDari(() => ({ aksi: "verifikasi", code: kode }))}
            sibuk={sibuk}
            className="space-y-4"
          >
            <ol className="list-decimal space-y-1 pl-5 text-sm text-muted-foreground">
              <li>Pindai kode QR dengan aplikasi autentikator.</li>
              <li>Masukkan kode 6 digit yang tampil di aplikasi.</li>
            </ol>
            {/* SVG dari server dirender sebagai gambar (tajam di semua ukuran layar); bukan HTML mentah. */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={`data:image/svg+xml;utf8,${encodeURIComponent(setup.qr_svg)}`}
              alt="Kode QR untuk aplikasi autentikator"
              className="mx-auto size-48 rounded-xl border border-border bg-white p-2"
            />
            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer font-medium text-foreground">
                Tidak bisa memindai? Masukkan kunci manual
              </summary>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <p className="num min-w-0 flex-1 rounded-lg bg-muted/50 p-2 text-sm break-all text-foreground select-all">
                  {setup.secret}
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void salin(setup.secret, "Kunci disalin")}
                >
                  <Copy /> Salin kunci manual
                </Button>
              </div>
            </details>
            <div className="space-y-2">
              <Label htmlFor="kode">Kode 6 digit</Label>
              <Input
                id="kode"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={7}
                placeholder="123456"
                value={kode}
                onChange={(e) => setKode(e.target.value)}
                disabled={sibuk}
                className="num h-11 text-lg tracking-widest"
                required
              />
            </div>
            {galatTampil}
            <SpringButton
              type="submit"
              variant="primary"
              size="lg"
              disabled={sibuk}
              className="h-11 w-full"
            >
              Aktifkan MFA
              {sibuk ? (
                <Loader2 className="animate-spin" aria-hidden />
              ) : (
                <ArrowRight aria-hidden />
              )}
            </SpringButton>
          </FormBff>
        );

      case "cadangan":
        return (
          <>
            <p className="text-sm leading-relaxed text-muted-foreground">
              Simpan kode cadangan ini. Masing-masing hanya berlaku sekali dan{" "}
              <strong className="text-foreground">
                tidak dapat ditampilkan lagi
              </strong>
              . Pakai salah satunya bila ponsel Anda hilang.
            </p>
            <ul
              aria-label="Kode cadangan"
              className="num grid grid-cols-2 gap-2 rounded-xl bg-muted/50 p-3 text-center text-sm font-medium select-all"
            >
              {cadangan.map((k) => (
                <li key={k}>{k}</li>
              ))}
            </ul>
            <div className="flex gap-2">
              <SpringButton
                variant="outline"
                className="flex-1"
                title="Salin semua kode cadangan"
                onClick={() =>
                  void salin(cadangan.join("\n"), "Kode cadangan disalin")
                }
              >
                <Copy aria-hidden /> Salin
              </SpringButton>
              <SpringButton
                variant="outline"
                className="flex-1"
                title="Unduh berkas (.txt)"
                onClick={unduh}
              >
                <Download aria-hidden /> Unduh
              </SpringButton>
            </div>
            <label className="flex items-start gap-2 text-sm">
              <Checkbox
                checked={tersimpan}
                onCheckedChange={(v) => setTersimpan(v === true)}
                className="mt-0.5"
              />
              Saya sudah menyimpan kode cadangan di tempat aman.
            </label>
            <SpringButton
              variant="primary"
              size="lg"
              disabled={!tersimpan}
              onClick={selesai}
              className="h-11 w-full"
            >
              {melanjutkan ? (
                <>
                  Lanjutkan <ArrowRight aria-hidden />
                </>
              ) : (
                <>
                  Selesai <Check aria-hidden />
                </>
              )}
            </SpringButton>
          </>
        );

      case "aktif":
        return (
          <>
            <Imbauan nada="normal" ikon={ShieldCheck}>
              <p className="font-medium">Verifikasi dua langkah aktif</p>
              {akun?.kode_cadangan_tersisa !== undefined && (
                <p>
                  <span className="num">{akun.kode_cadangan_tersisa}</span> kode
                  cadangan tersisa
                </p>
              )}
            </Imbauan>
            <section className="space-y-2">
              <p className="text-sm font-semibold">Kode cadangan</p>
              <p className="text-xs text-muted-foreground">
                Kode cadangan lama langsung tidak berlaku.
              </p>
              <SpringButton
                variant="outline"
                onClick={() => bukaDialog("kode-baru")}
                className="w-full sm:w-auto"
              >
                <KeyRound aria-hidden /> Buat kode cadangan baru
              </SpringButton>
            </section>
            <section className="space-y-2">
              <p className="text-sm font-semibold">Nonaktifkan</p>
              {akun?.mfa_wajib ? (
                <p className="text-xs text-muted-foreground">
                  Verifikasi dua langkah wajib untuk akun Anda dan tidak dapat
                  dinonaktifkan.
                </p>
              ) : (
                <>
                  <p className="text-xs text-muted-foreground">
                    Akun hanya dilindungi kata sandi sampai Anda mengaktifkannya
                    lagi.
                  </p>
                  <SpringButton
                    variant="outline"
                    onClick={() => bukaDialog("nonaktifkan")}
                    className="w-full text-defisit-ink sm:w-auto dark:text-rose-300"
                  >
                    Nonaktifkan MFA
                  </SpringButton>
                </>
              )}
            </section>
            {mode.jenis === "halaman" && (
              <Link
                href="/"
                className="block text-center text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                Kembali ke ruang kerja
              </Link>
            )}
          </>
        );
    }
  }

  return (
    <div className={className}>
      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={tahap}
          variants={fadeIn}
          initial="hidden"
          animate="show"
          exit="exit"
          className="space-y-4"
        >
          {isiTahap()}
        </motion.div>
      </AnimatePresence>

      <DialogSandi
        open={dialog === "kode-baru"}
        onTutup={tutupDialog}
        judul="Buat ulang kode cadangan?"
        deskripsi="Kode cadangan lama langsung tidak berlaku. Masukkan kata sandi Anda untuk mengonfirmasi."
        tombol="Ya, buat kode baru"
        sandi={sandi}
        onSandi={setSandi}
        onKirim={aksi.kirimDari(() => ({
          aksi: "kode-baru",
          password: sandi,
        }))}
        sibuk={sibuk}
        galatField={aksi.galatField.password}
        galat={dialog === "kode-baru" ? galat : null}
      />
      <DialogSandi
        open={dialog === "nonaktifkan"}
        onTutup={tutupDialog}
        judul="Nonaktifkan verifikasi dua langkah?"
        deskripsi="Akun Anda hanya dilindungi kata sandi dan kode cadangan yang ada dihapus. Masukkan kata sandi Anda untuk mengonfirmasi."
        tombol="Ya, nonaktifkan"
        bahaya
        sandi={sandi}
        onSandi={setSandi}
        onKirim={aksi.kirimDari(() => ({
          aksi: "nonaktifkan",
          password: sandi,
        }))}
        sibuk={sibuk}
        galatField={aksi.galatField.password}
        galat={dialog === "nonaktifkan" ? galat : null}
      />
    </div>
  );
}
