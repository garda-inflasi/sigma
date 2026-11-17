"use client";

import { Search, ShieldAlert } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { PanelGalat } from "@/components/garda/panel-galat";
import { SpringButton } from "@/components/garda/spring-button";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  KODE_IMPERSONASI_DIBATASI,
  KODE_TARGET_AMBIGU,
  MAKS_ALASAN,
  kelompokkanPeranTarget,
  kodeGalatImpersonasi,
  labelPeran,
  saringAkun,
  susunMulai,
  type PeranTargetWire,
} from "@/lib/api/impersonasi";
import { useSession } from "@/lib/auth/session-context";
import {
  kunciImpersonasi,
  useAlihAkun,
  useTargetImpersonasi,
} from "@/lib/query/impersonasi-hooks";
import { LAPIS_META, ROLES, type Lapis } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";
import { useQueryClient } from "@tanstack/react-query";
import { useKembaliPeranAsal } from "./tombol-kembali";

/** Daftar akun lebih dari ini diberi kolom cari. */
const AMBANG_CARI = 6;

export interface PilihanAwal {
  peran: string;
  /** Akun yang sudah dipilih (mis. dari tabel di halaman admin). */
  penggunaId?: string;
}

/**
 * Dialog "Beralih / Impersonate Role". Daftar sasaran datang dari API (`targets`): hanya peran yang punya akun
 * aktif dan diizinkan kebijakan bagi pemanggil. Peran beranggota satu akun langsung terpilih; bila lebih dari
 * satu, akunnya dipilih di bawahnya. Selama sudah impersonasi, memilih sasaran baru MENGGANTI (bukan menumpuk;
 * server mengevaluasi hak terhadap akun asli) dan tombol kembali tetap tersedia.
 */
export function DialogAlihPeran({
  open,
  onOpenChange,
  awal,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  /** Pilihan awal saat dialog dibuka dari tabel di halaman admin. */
  awal?: PilihanAwal;
}) {
  const imp = useSession().impersonation;
  const qc = useQueryClient();
  const { data, isLoading, error, refetch } = useTargetImpersonasi(open);
  const { mulai, sibuk } = useAlihAkun();
  const { kembali, sibuk: sibukKembali } = useKembaliPeranAsal();

  const [peran, setPeran] = useState<string | null>(null);
  const [penggunaId, setPenggunaId] = useState<string | null>(null);
  const [cari, setCari] = useState("");
  const [alasan, setAlasan] = useState("");
  const [galat, setGalat] = useState<string | null>(null);

  // Setiap pembukaan dimulai bersih (atau dari pilihan awal) supaya sisa pilihan lama tidak terbawa.
  useEffect(() => {
    if (!open) return;
    setPeran(awal?.peran ?? null);
    setPenggunaId(awal?.penggunaId ?? null);
    setCari("");
    setAlasan("");
    setGalat(null);
  }, [open, awal?.peran, awal?.penggunaId]);

  const kelompok = useMemo(
    () => kelompokkanPeranTarget(data?.peran ?? []),
    [data],
  );
  const terpilih: PeranTargetWire | undefined = data?.peran.find(
    (p) => p.peran === peran,
  );
  // Peran beranggota satu akun: akun itu otomatis dipakai (dikirim sebagai id, bukan hanya peran, supaya tidak
  // ambigu bila ada akun baru pada peran itu di sela-sela).
  const akunEfektif =
    penggunaId ?? (terpilih?.akun.length === 1 ? terpilih.akun[0].id : null);
  const wajibAlasan = data?.wajib_alasan ?? true;
  const alasanKosong = alasan.trim() === "";
  const bolehMulai =
    !!data?.boleh &&
    !!data.aktif &&
    !!terpilih &&
    !!akunEfektif &&
    !(wajibAlasan && alasanKosong) &&
    !sibuk;

  const kirim = async () => {
    if (!bolehMulai || !terpilih || !akunEfektif) return;
    setGalat(null);
    try {
      await mulai.mutateAsync(
        susunMulai({ peran: terpilih.peran, penggunaId: akunEfektif }, alasan),
      );
    } catch (err) {
      const kode = kodeGalatImpersonasi(err);
      if (kode === KODE_TARGET_AMBIGU) {
        // Akun baru muncul pada peran itu: muat ulang daftar supaya pemilih akun tampil.
        qc.invalidateQueries({ queryKey: kunciImpersonasi.targets });
        setPenggunaId(null);
      }
      setGalat(
        kode === KODE_IMPERSONASI_DIBATASI
          ? "Aksi ini tidak tersedia selama impersonasi."
          : err instanceof Error
            ? err.message
            : "Gagal memulai impersonasi. Coba lagi.",
      );
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => (sibuk ? undefined : onOpenChange(v))}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Beralih / Impersonate Role</DialogTitle>
          <DialogDescription>
            Bekerja sementara sebagai akun lain untuk memeriksa tampilan dan hak
            aksesnya. Semua aksi tetap tercatat atas nama akun asli Anda di
            audit trail.
          </DialogDescription>
        </DialogHeader>

        {imp?.active && (
          <div className="flex items-start gap-2 rounded-lg border border-waspada/50 bg-waspada-soft p-3 text-[13px] text-navy-900 dark:bg-waspada/15 dark:text-amber-100">
            <ShieldAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
            <div className="min-w-0 flex-1">
              <p>
                Anda sedang impersonate <b>{imp.targetUser.name}</b> (
                {labelPeran(imp.targetUser.role)}). Memilih sasaran baru akan{" "}
                <b>mengganti</b> sesi ini dan tetap mengacu pada akun asli{" "}
                <b>{imp.originalUser.name}</b>.
              </p>
              <button
                type="button"
                onClick={kembali}
                disabled={sibukKembali}
                className="mt-1.5 text-xs font-medium underline underline-offset-2 disabled:opacity-60"
              >
                Kembali ke Peran Asal
              </button>
            </div>
          </div>
        )}

        {isLoading ? (
          <div
            className="h-48 animate-pulse rounded-lg bg-secondary"
            aria-busy="true"
          />
        ) : error || !data ? (
          <PanelGalat
            judul="Daftar sasaran gagal dimuat"
            error={error}
            onCoba={() => void refetch()}
          />
        ) : !data.boleh ? (
          <p
            role="status"
            className="rounded-lg bg-secondary p-4 text-sm text-muted-foreground"
          >
            Akun Anda tidak diizinkan memakai impersonasi. Minta superadmin
            menambahkan Anda pada kebijakan impersonasi bila dibutuhkan.
          </p>
        ) : !data.aktif ? (
          <p
            role="status"
            className="rounded-lg bg-secondary p-4 text-sm text-muted-foreground"
          >
            Impersonasi sedang dinonaktifkan oleh superadmin.
          </p>
        ) : data.peran.length === 0 ? (
          <p
            role="status"
            className="rounded-lg bg-secondary p-4 text-sm text-muted-foreground"
          >
            Belum ada akun aktif yang boleh Anda impersonate.
          </p>
        ) : (
          <div className="grid gap-4">
            <fieldset className="grid gap-3">
              <legend className="mb-1 text-[13px] font-medium">
                Peran sasaran
              </legend>
              {kelompok.map((g) => (
                <div key={g.lapis} className="grid gap-1.5">
                  <p className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                    {LAPIS_META[g.lapis as Lapis]?.label ?? g.lapis}
                  </p>
                  <div
                    role="radiogroup"
                    aria-label={`Peran ${g.lapis}`}
                    className="grid gap-1.5 sm:grid-cols-2"
                  >
                    {g.peran.map((p) => {
                      const aktif = p.peran === peran;
                      return (
                        <button
                          key={p.peran}
                          type="button"
                          role="radio"
                          aria-checked={aktif}
                          onClick={() => {
                            setPeran(p.peran);
                            setPenggunaId(null);
                            setCari("");
                            setGalat(null);
                          }}
                          className={cn(
                            "flex flex-col rounded-lg border bg-card px-3 py-2 text-left transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                            aktif
                              ? "border-navy-800 shadow-ambient-sm dark:border-white"
                              : "border-border hover:border-navy-300",
                          )}
                        >
                          <span className="text-[13px] font-medium">
                            {ROLES[p.peran as keyof typeof ROLES]?.label ??
                              p.label}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            {p.akun.length === 1
                              ? p.akun[0].nama
                              : `${p.akun.length} akun aktif`}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </div>
              ))}
            </fieldset>

            {terpilih && terpilih.akun.length > 1 && (
              <AkunPicker
                peran={terpilih}
                cari={cari}
                onCari={setCari}
                value={penggunaId}
                onChange={setPenggunaId}
              />
            )}

            <div className="grid gap-1.5">
              <Label htmlFor="alasan-impersonasi">
                Alasan{" "}
                <span className="font-normal text-muted-foreground">
                  {wajibAlasan ? "(wajib)" : "(opsional)"}
                </span>
              </Label>
              <Textarea
                id="alasan-impersonasi"
                value={alasan}
                maxLength={MAKS_ALASAN}
                rows={2}
                onChange={(e) => setAlasan(e.target.value)}
                placeholder="Mis. memeriksa tampilan dasbor Koordinator Lapangan untuk laporan bug #123"
                aria-required={wajibAlasan}
              />
              <p className="text-xs text-muted-foreground">
                Dicatat di audit trail.{" "}
                {data.durasi_maks_menit
                  ? `Sesi impersonasi berakhir otomatis setelah ${data.durasi_maks_menit} menit.`
                  : "Tidak ada batas durasi; kembali secara manual bila selesai."}
              </p>
            </div>

            {galat && (
              <p role="alert" className="text-sm text-destructive">
                {galat}
              </p>
            )}
          </div>
        )}

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={sibuk}
          >
            Batal
          </Button>
          <SpringButton
            variant="primary"
            onClick={() => void kirim()}
            disabled={!bolehMulai}
          >
            {sibuk
              ? "Memulai…"
              : imp?.active
                ? "Ganti sasaran"
                : "Mulai Impersonate"}
          </SpringButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function AkunPicker({
  peran,
  cari,
  onCari,
  value,
  onChange,
}: {
  peran: PeranTargetWire;
  cari: string;
  onCari: (v: string) => void;
  value: string | null;
  onChange: (id: string) => void;
}) {
  const akun = saringAkun(peran.akun, cari);
  return (
    <fieldset className="grid gap-2">
      <legend className="mb-1 text-[13px] font-medium">
        Akun {ROLES[peran.peran as keyof typeof ROLES]?.label ?? peran.label}
      </legend>
      {peran.akun.length > AMBANG_CARI && (
        <div className="relative">
          <Search
            className="pointer-events-none absolute top-2.5 left-2.5 size-4 text-muted-foreground"
            aria-hidden
          />
          <Input
            value={cari}
            onChange={(e) => onCari(e.target.value)}
            placeholder="Cari nama, email, atau lembaga"
            aria-label="Cari akun"
            className="pl-8"
          />
        </div>
      )}
      <div
        role="radiogroup"
        aria-label="Akun sasaran"
        className="scrollbar-thin grid max-h-48 gap-1 overflow-y-auto"
      >
        {akun.length === 0 ? (
          <p className="px-1 py-2 text-sm text-muted-foreground">
            Tidak ada akun yang cocok.
          </p>
        ) : (
          akun.map((a) => (
            <button
              key={a.id}
              type="button"
              role="radio"
              aria-checked={a.id === value}
              onClick={() => onChange(a.id)}
              className={cn(
                "flex flex-col rounded-md border px-3 py-1.5 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                a.id === value
                  ? "border-navy-800 bg-card dark:border-white"
                  : "border-transparent hover:bg-secondary",
              )}
            >
              <span className="text-[13px] font-medium">{a.nama}</span>
              <span className="text-xs text-muted-foreground">
                {a.email} · {a.lembaga}
              </span>
            </button>
          ))
        )}
      </div>
    </fieldset>
  );
}
