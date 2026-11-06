"use client";

import { Check, RotateCcw, Save, Undo2 } from "lucide-react";
import { Fragment, useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  balikIzin,
  samaIzin,
  selisihDraf,
  urutkanSesuaiKatalog,
  type PeranAkses,
} from "@/lib/api/rbac";
import { useMatriksAkses, useSimpanPeran } from "@/lib/query/rbac-hooks";
import { drafMatriks, useDrafMatriks } from "@/lib/rbac/draf-matriks";
import { LAPIS_META, ROLES } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";

/**
 * Matriks peran × izin yang dapat diedit (pengguna.kelola, 02-C). Izin yang berlaku dibaca dari basis data. Klik sel
 * mengubah DRAF (belum berlaku); "Simpan" per peran mengirim seluruh izin peran itu dengan versi yang dibaca
 * (kunci optimistis: bila admin lain menyimpan lebih dulu, draf dibuang dan matriks dimuat ulang). Sel yang berbeda
 * dari bawaan di kode ditandai, dan "Pulihkan bawaan" menjadikannya draf. Draf bertahan saat berpindah tab di
 * halaman dan memperingatkan sebelum halaman ditutup.
 */
export function MatriksAksesEditor() {
  const { data: matriks, isLoading, isError, refetch } = useMatriksAkses();
  const draf = useDrafMatriks();
  const simpan = useSimpanPeran();
  const [menyimpan, setMenyimpan] = useState<string | null>(null);

  // Draf yang versi dasarnya sudah tertinggal (admin lain menyimpan lebih dulu) tidak boleh menimpa perubahan mereka.
  useEffect(() => {
    if (!matriks) return;
    const dibuang = drafMatriks.buangUsang(
      new Map(matriks.peran.map((p) => [p.id, p.versi])),
    );
    if (dibuang.length > 0) {
      toast.warning("Draf dibuang karena matriks diubah admin lain", {
        description: dibuang.map((r) => ROLES[r].label).join(", "),
      });
    }
  }, [matriks]);

  const jumlahDraf = draf.size;
  useEffect(() => {
    if (jumlahDraf === 0) return;
    const peringatkan = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", peringatkan);
    return () => window.removeEventListener("beforeunload", peringatkan);
  }, [jumlahDraf]);

  if (isError) {
    return (
      <div
        role="alert"
        className="glass flex items-center justify-between gap-4 rounded-xl p-4 text-sm"
      >
        <span>Matriks akses gagal dimuat.</span>
        <Button variant="outline" size="sm" onClick={() => void refetch()}>
          Muat ulang
        </Button>
      </div>
    );
  }
  if (isLoading || !matriks)
    return (
      <div className="glass h-64 animate-pulse rounded-xl" aria-busy="true" />
    );

  const izinTampil = (p: PeranAkses): ReadonlySet<string> =>
    draf.get(p.id)?.izin ?? p.izin;

  const klikSel = (p: PeranAkses, izin: string) => {
    const baru = balikIzin(izinTampil(p), izin);
    if (samaIzin(baru, p.izin)) drafMatriks.hapus(p.id);
    else drafMatriks.atur(p.id, { versi: p.versi, izin: baru });
  };

  const pulihkanBawaan = (p: PeranAkses) => {
    if (samaIzin(p.bawaan, p.izin)) drafMatriks.hapus(p.id);
    else drafMatriks.atur(p.id, { versi: p.versi, izin: new Set(p.bawaan) });
  };

  const simpanPeran = async (p: PeranAkses) => {
    const d = draf.get(p.id);
    if (!d || menyimpan) return;
    setMenyimpan(p.id);
    try {
      const hasil = await simpan.mutateAsync({
        peran: p,
        izin: urutkanSesuaiKatalog(d.izin, matriks.izin),
      });
      if (hasil !== "ditolak") drafMatriks.hapus(p.id);
    } catch {
      // Toast galat ditampilkan hook; draf dipertahankan agar bisa dicoba lagi.
    } finally {
      setMenyimpan(null);
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">
          Klik sel untuk mengubah izin sebuah peran, lalu simpan per peran.
          Perubahan berlaku pada request berikutnya dan tercatat di audit trail.
          Titik kuning menandai izin yang berbeda dari matriks bawaan.
        </p>
        {jumlahDraf > 0 && (
          <div
            role="status"
            className="flex items-center gap-2 rounded-full bg-waspada-soft px-3 py-1 text-xs font-medium text-waspada-ink"
          >
            {jumlahDraf} peran belum disimpan
            <Button
              variant="ghost"
              size="xs"
              aria-label="Batalkan semua perubahan matriks"
              onClick={() => drafMatriks.bersihkan()}
              disabled={!!menyimpan}
            >
              Batalkan semua
            </Button>
          </div>
        )}
      </div>

      <div className="glass scrollbar-thin overflow-x-auto rounded-xl">
        <table
          aria-label="Matriks akses peran dan izin"
          className="w-full min-w-[1500px] border-separate border-spacing-0 text-xs"
        >
          <thead className="sticky top-0 z-10">
            <tr>
              <th className="sticky left-0 z-20 min-w-56 bg-white/95 p-3 text-left font-medium text-muted-foreground backdrop-blur dark:bg-navy-950/95">
                Izin
              </th>
              {matriks.peran.map((p) => (
                <KepalaPeran
                  key={p.id}
                  peran={p}
                  tampil={izinTampil(p)}
                  punyaDraf={draf.has(p.id)}
                  menyimpan={menyimpan === p.id}
                  terkunci={!!menyimpan}
                  onSimpan={() => void simpanPeran(p)}
                  onBatal={() => drafMatriks.hapus(p.id)}
                  onBawaan={() => pulihkanBawaan(p)}
                />
              ))}
            </tr>
          </thead>
          <tbody>
            {matriks.kelompok.map((g) => (
              <Fragment key={g.nama}>
                <tr>
                  <td
                    colSpan={matriks.peran.length + 1}
                    className="sticky left-0 bg-mist-100/80 px-3 py-1.5 font-semibold dark:bg-white/5"
                  >
                    {g.nama}
                  </td>
                </tr>
                {g.izin.map((izin) => (
                  <tr
                    key={izin.id}
                    className="hover:bg-white/50 dark:hover:bg-white/[0.03]"
                  >
                    <td className="sticky left-0 border-b border-border/50 bg-white/90 px-3 py-2 backdrop-blur dark:bg-navy-950/90">
                      <span className="num block">{izin.id}</span>
                      <span className="block text-[10px] text-muted-foreground">
                        {izin.deskripsi}
                      </span>
                    </td>
                    {matriks.peran.map((p) => (
                      <SelIzin
                        key={p.id}
                        peran={p}
                        izin={izin.id}
                        tampil={izinTampil(p)}
                        onKlik={() => klikSel(p, izin.id)}
                        terkunci={menyimpan === p.id}
                      />
                    ))}
                  </tr>
                ))}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function KepalaPeran({
  peran,
  tampil,
  punyaDraf,
  menyimpan,
  terkunci,
  onSimpan,
  onBatal,
  onBawaan,
}: {
  peran: PeranAkses;
  tampil: ReadonlySet<string>;
  punyaDraf: boolean;
  menyimpan: boolean;
  terkunci: boolean;
  onSimpan: () => void;
  onBatal: () => void;
  onBawaan: () => void;
}) {
  const { ditambah, dicabut } = selisihDraf(peran.izin, tampil);
  const bedaDariBawaan = !samaIzin(tampil, peran.bawaan);
  return (
    <th
      scope="col"
      className={cn(
        "min-w-28 bg-white/90 p-2 align-top font-medium backdrop-blur dark:bg-navy-950/90",
        punyaDraf && "bg-waspada-soft/70 dark:bg-waspada/10",
      )}
    >
      <span className="block text-[11px] leading-tight">{peran.label}</span>
      <span className="mt-1 block text-[10px] font-normal text-muted-foreground">
        {LAPIS_META[ROLES[peran.id].lapis].label.split(" ")[1]}
      </span>
      <span className="mt-0.5 block text-[10px] font-normal text-muted-foreground">
        v{peran.versi}
      </span>
      {punyaDraf && (
        <span className="mt-1 block text-[10px] font-normal text-waspada-ink">
          Belum disimpan · +{ditambah.length} −{dicabut.length}
        </span>
      )}
      <span className="mt-1.5 flex flex-wrap justify-center gap-1">
        {punyaDraf && (
          <>
            <Button
              size="xs"
              aria-label={`Simpan matriks ${peran.label}`}
              onClick={onSimpan}
              disabled={terkunci}
            >
              <Save /> {menyimpan ? "Menyimpan…" : "Simpan"}
            </Button>
            <Button
              variant="outline"
              size="xs"
              aria-label={`Batalkan perubahan ${peran.label}`}
              onClick={onBatal}
              disabled={terkunci}
            >
              <Undo2 /> Batal
            </Button>
          </>
        )}
        {bedaDariBawaan && (
          <Button
            variant="ghost"
            size="xs"
            aria-label={`Pulihkan bawaan ${peran.label}`}
            onClick={onBawaan}
            disabled={terkunci}
          >
            <RotateCcw /> Bawaan
          </Button>
        )}
      </span>
    </th>
  );
}

function SelIzin({
  peran,
  izin,
  tampil,
  onKlik,
  terkunci,
}: {
  peran: PeranAkses;
  izin: string;
  tampil: ReadonlySet<string>;
  onKlik: () => void;
  terkunci: boolean;
}) {
  const aktif = tampil.has(izin);
  const berubah = aktif !== peran.izin.has(izin);
  const bedaBawaan = aktif !== peran.bawaan.has(izin);
  return (
    <td className="border-b border-border/50 p-0 text-center">
      <button
        type="button"
        aria-pressed={aktif}
        aria-label={`${izin} untuk ${peran.label}`}
        data-berubah={berubah || undefined}
        disabled={terkunci}
        onClick={onKlik}
        className="relative inline-grid size-9 place-items-center rounded-md outline-none transition-colors hover:bg-mist-100 focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50 dark:hover:bg-white/10"
      >
        {aktif ? (
          <span
            className={cn(
              "inline-grid size-5 place-items-center rounded-full bg-data-500 text-white",
              berubah && "ring-2 ring-waspada ring-offset-1",
            )}
          >
            <Check className="size-3" />
          </span>
        ) : (
          <span
            className={cn(
              "inline-block rounded-full bg-mist-300 dark:bg-white/15",
              berubah ? "size-3 ring-2 ring-waspada ring-offset-1" : "size-1",
            )}
          />
        )}
        {bedaBawaan && (
          <span
            aria-hidden
            title="Berbeda dari matriks bawaan"
            className="absolute right-1 top-1 size-1.5 rounded-full bg-waspada"
          />
        )}
      </button>
    </td>
  );
}
