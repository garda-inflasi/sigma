"use client";

import {
  AlertTriangle,
  CheckCircle2,
  Download,
  Loader2,
  Upload,
} from "lucide-react";
import { useState, type ChangeEvent } from "react";
import { FormAlert, FormField } from "@/components/form/form-field";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  KOLOM_IMPOR,
  LABEL_STATUS_BARIS,
  MAKS_BARIS_IMPOR,
  MAKS_BYTE_IMPOR,
  NAMA_BERKAS_TEMPLATE,
  periksaBerkasImpor,
  ringkasHasil,
  ringkasPratinjau,
  templateImporCsv,
  type BarisImpor,
  type HasilImpor,
  type StatusBarisImpor,
} from "@/lib/pengguna/impor";
import { useImporPengguna } from "@/lib/query/undangan-hooks";
import { ROLE_IDS } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";

const TONE_STATUS: Record<StatusBarisImpor, "normal" | "defisit" | "data"> = {
  valid: "normal",
  galat: "defisit",
  dibuat: "data",
};

function unduhTemplate() {
  const blob = new Blob([templateImporCsv()], {
    type: "text/csv;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const a = Object.assign(document.createElement("a"), {
    href: url,
    download: NAMA_BERKAS_TEMPLATE,
  });
  a.click();
  URL.revokeObjectURL(url);
}

const KELAS_TH =
  "h-9 bg-secondary px-3 text-left text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase";
const KELAS_TD = "border-t border-border px-3 py-2";

function TabelBaris({ baris, label }: { baris: BarisImpor[]; label: string }) {
  return (
    <div className="scrollbar-thin max-h-72 min-w-0 overflow-auto rounded-lg border border-border">
      <table
        aria-label={label}
        className="w-full min-w-[720px] border-separate border-spacing-0 text-xs"
      >
        <thead className="sticky top-0 z-10">
          <tr>
            <th scope="col" className={KELAS_TH}>
              Baris
            </th>
            <th scope="col" className={KELAS_TH}>
              Nama
            </th>
            <th scope="col" className={KELAS_TH}>
              Email
            </th>
            <th scope="col" className={KELAS_TH}>
              Peran
            </th>
            <th scope="col" className={KELAS_TH}>
              Lembaga
            </th>
            <th scope="col" className={KELAS_TH}>
              Kab.
            </th>
            <th scope="col" className={KELAS_TH}>
              Status
            </th>
          </tr>
        </thead>
        <tbody>
          {baris.map((b) => (
            <tr
              key={b.baris}
              data-status={b.status}
              className={cn(
                "align-top",
                b.status === "galat" && "bg-defisit/5",
              )}
            >
              <td className={cn(KELAS_TD, "num")}>{b.baris}</td>
              <td className={cn(KELAS_TD, "max-w-40 break-words")}>
                {b.nama || <span className="text-muted-foreground">—</span>}
              </td>
              <td className={cn(KELAS_TD, "max-w-48 break-all")}>
                {b.email || <span className="text-muted-foreground">—</span>}
              </td>
              <td className={cn(KELAS_TD, "num whitespace-nowrap")}>
                {b.peran || <span className="text-muted-foreground">—</span>}
              </td>
              <td className={cn(KELAS_TD, "max-w-40 break-words")}>
                {b.lembaga || <span className="text-muted-foreground">—</span>}
              </td>
              <td className={cn(KELAS_TD, "num")}>{b.kabupatenKode || "—"}</td>
              <td className={cn(KELAS_TD, "min-w-44")}>
                <StatusBadge tone={TONE_STATUS[b.status]}>
                  {LABEL_STATUS_BARIS[b.status]}
                </StatusBadge>
                {b.galat.length > 0 && (
                  <ul className="mt-1 list-disc space-y-0.5 pl-4 break-words text-defisit-ink dark:text-defisit-soft">
                    {b.galat.map((g) => (
                      <li key={g}>{g}</li>
                    ))}
                  </ul>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function IsiImpor({ onOpenChange }: { onOpenChange: (v: boolean) => void }) {
  const impor = useImporPengguna();
  const [teks, setTeks] = useState<string | null>(null);
  const [namaBerkas, setNamaBerkas] = useState("");
  const [pratinjau, setPratinjau] = useState<HasilImpor | null>(null);
  const [hasil, setHasil] = useState<HasilImpor | null>(null);
  const [galat, setGalat] = useState<string | null>(null);
  const sibuk = impor.isPending;

  const ulang = () => {
    setTeks(null);
    setNamaBerkas("");
    setPratinjau(null);
    setHasil(null);
    setGalat(null);
  };

  async function pilihBerkas(e: ChangeEvent<HTMLInputElement>) {
    const berkas = e.target.files?.[0];
    e.target.value = ""; // memilih berkas yang sama sekali lagi tetap memicu onChange
    if (!berkas) return;
    ulang();
    const masalah = periksaBerkasImpor(berkas);
    if (masalah) {
      setGalat(masalah);
      return;
    }
    try {
      const isi = await berkas.text();
      const periksa = await impor.mutateAsync({ csv: isi, dryRun: true });
      setTeks(isi);
      setNamaBerkas(berkas.name);
      setPratinjau(periksa);
    } catch (err) {
      setGalat(
        err instanceof Error ? err.message : "Berkas tidak dapat diperiksa.",
      );
    }
  }

  async function komit() {
    if (teks === null) return;
    setGalat(null);
    try {
      setHasil(await impor.mutateAsync({ csv: teks, dryRun: false }));
    } catch (err) {
      setGalat(err instanceof Error ? err.message : "Impor gagal.");
    }
  }

  const tahapHasil = hasil !== null;
  return (
    <>
      <DialogHeader>
        <DialogTitle>Impor pengguna dari CSV</DialogTitle>
        <DialogDescription>
          Akun dibuat sebagai undangan: tiap pengguna menerima email untuk
          menetapkan kata sandinya sendiri. Berkas diperiksa dulu; baris yang
          salah tidak diimpor.
        </DialogDescription>
      </DialogHeader>

      <DialogBody>
        {!tahapHasil && (
          <FormField id="impor-berkas" label="Berkas CSV">
            <Input
              id="impor-berkas"
              type="file"
              accept=".csv,text/csv"
              disabled={sibuk}
              onChange={pilihBerkas}
            />
            <p className="text-xs leading-relaxed break-words text-muted-foreground">
              Kolom: <span className="num">{KOLOM_IMPOR.join(", ")}</span>{" "}
              (kabupaten_kode boleh dikosongkan). UTF-8, pemisah koma atau titik
              koma, maksimal {MAKS_BARIS_IMPOR} baris dan{" "}
              {MAKS_BYTE_IMPOR / 1024} KB. Peran yang diterima:{" "}
              <span className="num">{ROLE_IDS.join(", ")}</span>.
            </p>
            <div>
              <Button variant="outline" size="sm" onClick={unduhTemplate}>
                <Download /> Unduh templat CSV
              </Button>
            </div>
          </FormField>
        )}

        {sibuk && (
          <p
            role="status"
            className="flex items-center gap-2 text-sm text-muted-foreground"
          >
            <Loader2 className="size-4 animate-spin" />{" "}
            {pratinjau ? "Mengimpor…" : "Memeriksa berkas…"}
          </p>
        )}
        {galat && (
          <FormAlert className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            <span className="min-w-0">{galat}</span>
          </FormAlert>
        )}

        {pratinjau && !tahapHasil && (
          <div className="flex min-w-0 flex-col gap-3">
            <p role="status" className="text-sm break-words">
              <span className="font-medium break-all">{namaBerkas}</span>:{" "}
              {ringkasPratinjau(pratinjau)}
            </p>
            <TabelBaris baris={pratinjau.baris} label="Pratinjau impor" />
          </div>
        )}

        {hasil && (
          <div className="flex min-w-0 flex-col gap-3">
            <p role="status" className="flex items-start gap-2 text-sm">
              <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-normal" />
              <span className="min-w-0">{ringkasHasil(hasil)}</span>
            </p>
            <TabelBaris baris={hasil.baris} label="Hasil impor" />
          </div>
        )}
      </DialogBody>

      <DialogFooter>
        {tahapHasil ? (
          <SpringButton
            variant="primary"
            size="md"
            onClick={() => onOpenChange(false)}
          >
            Selesai
          </SpringButton>
        ) : (
          <>
            <Button
              variant="outline"
              onClick={() => (pratinjau ? ulang() : onOpenChange(false))}
              disabled={sibuk}
            >
              {pratinjau ? "Pilih berkas lain" : "Batal"}
            </Button>
            {pratinjau && (
              <SpringButton
                variant="primary"
                size="md"
                onClick={komit}
                disabled={sibuk || pratinjau.valid === 0}
              >
                <Upload />{" "}
                {sibuk
                  ? "Mengimpor…"
                  : `Impor ${pratinjau.valid.toLocaleString("id-ID")} pengguna valid`}
              </SpringButton>
            )}
          </>
        )}
      </DialogFooter>
    </>
  );
}

/** Dialog "Impor CSV": pilih berkas → pratinjau dengan galat per baris → konfirmasi → ringkasan hasil. */
export function ImporCsvDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-4xl">
        <IsiImpor onOpenChange={onOpenChange} />
      </DialogContent>
    </Dialog>
  );
}
