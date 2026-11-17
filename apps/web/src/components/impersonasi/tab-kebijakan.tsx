"use client";

import { Trash2, User, Users, X } from "lucide-react";
import { useState } from "react";
import { PanelGalat } from "@/components/garda/panel-galat";
import { SpringButton } from "@/components/garda/spring-button";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  PERAN_DAPAT_DIPILIH,
  PERAN_OPERASIONAL,
  aturPeranTarget,
  drafBerubah,
  kunciBaru,
  labelPeran,
  setPeranTarget,
  toDraf,
  toInput,
  validasiDraf,
  type DrafKebijakan,
  type DrafPelaku,
  type DrafTarget,
  type KebijakanWire,
  type PenggunaCari,
} from "@/lib/api/impersonasi";
import { formatRelatif } from "@/lib/format";
import {
  useKebijakanImpersonasi,
  useSimpanKebijakan,
} from "@/lib/query/impersonasi-hooks";
import { ROLES, type Role } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";
import { PemilihPengguna } from "./pemilih-pengguna";

const kelasPilihan =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50";

/** Tab "Kebijakan": pengaturan global, pelaku tambahan, dan target per pelaku. Hanya superadmin (API Go menegakkan). */
export function TabKebijakan() {
  const { data, isLoading, error, refetch } = useKebijakanImpersonasi();

  if (isLoading) {
    return (
      <div className="glass h-64 animate-pulse rounded-xl" aria-busy="true" />
    );
  }
  if (error || !data) {
    return (
      <PanelGalat
        judul="Kebijakan impersonasi gagal dimuat"
        error={error}
        onCoba={() => void refetch()}
      />
    );
  }
  // key = versi: setelah simpan atau konflik versi, data baru membuat draf dimulai ulang dari keadaan server.
  return <EditorKebijakan key={data.versi} kebijakan={data} />;
}

function EditorKebijakan({ kebijakan }: { kebijakan: KebijakanWire }) {
  const [draf, setDraf] = useState<DrafKebijakan>(() => toDraf(kebijakan));
  const simpan = useSimpanKebijakan();
  const galat = validasiDraf(draf);
  const berubah = drafBerubah(draf, kebijakan);

  const ubahPelaku = (kunci: string, f: (p: DrafPelaku) => DrafPelaku) =>
    setDraf((d) => ({
      ...d,
      pelaku: d.pelaku.map((p) => (p.kunci === kunci ? f(p) : p)),
    }));
  const tambahPelaku = (p: Omit<DrafPelaku, "kunci" | "aktif" | "target">) =>
    setDraf((d) => ({
      ...d,
      pelaku: [
        ...d.pelaku,
        { ...p, kunci: kunciBaru(), aktif: true, target: [] },
      ],
    }));

  const sudahPelakuPeran = new Set(
    draf.pelaku.filter((p) => p.tipe === "PERAN").map((p) => p.peran as string),
  );
  const sudahPelakuAkun = draf.pelaku
    .filter((p) => p.tipe === "PENGGUNA")
    .map((p) => p.penggunaId as string);

  return (
    <div className="grid gap-5">
      <section
        aria-labelledby="judul-umum"
        className="glass grid gap-4 rounded-xl p-4 sm:p-5"
      >
        <div>
          <h2 id="judul-umum" className="text-display-sm font-semibold">
            Pengaturan umum
          </h2>
          <p className="mt-0.5 text-[13px] text-muted-foreground">
            Administrator Sistem (superadmin) selalu boleh meng-impersonate akun
            selain Administrator Sistem selama impersonasi aktif. Akun
            Administrator Sistem tidak pernah dapat menjadi sasaran.
          </p>
        </div>

        <label className="flex items-start gap-2.5 text-sm">
          <Checkbox
            checked={draf.aktif}
            onCheckedChange={(v) =>
              setDraf((d) => ({ ...d, aktif: v === true }))
            }
            aria-label="Impersonasi aktif"
            className="mt-0.5"
          />
          <span>
            <span className="font-medium">Impersonasi aktif</span>
            <span className="block text-xs text-muted-foreground">
              Mematikan ini seketika mengakhiri semua sesi impersonasi yang
              sedang berjalan, termasuk milik superadmin.
            </span>
          </span>
        </label>

        <label className="flex items-start gap-2.5 text-sm">
          <Checkbox
            checked={draf.wajibAlasan}
            onCheckedChange={(v) =>
              setDraf((d) => ({ ...d, wajibAlasan: v === true }))
            }
            aria-label="Alasan wajib diisi"
            className="mt-0.5"
          />
          <span>
            <span className="font-medium">Alasan wajib diisi saat memulai</span>
            <span className="block text-xs text-muted-foreground">
              Alasan dicatat di audit trail bersama setiap sesi impersonasi.
            </span>
          </span>
        </label>

        <div className="grid max-w-xs gap-1.5">
          <Label htmlFor="durasi-maks">Durasi maksimal sesi (menit)</Label>
          <Input
            id="durasi-maks"
            inputMode="numeric"
            value={draf.durasiMaks}
            onChange={(e) =>
              setDraf((d) => ({ ...d, durasiMaks: e.target.value }))
            }
            placeholder="Kosong = tanpa batas"
          />
          <p className="text-xs text-muted-foreground">
            5 sampai 1440 menit. Sesi berakhir otomatis setelah batas ini.
          </p>
        </div>
      </section>

      <section
        aria-labelledby="judul-pelaku"
        className="glass grid gap-4 rounded-xl p-4 sm:p-5"
      >
        <div>
          <h2 id="judul-pelaku" className="text-display-sm font-semibold">
            Pelaku tambahan dan targetnya
          </h2>
          <p className="mt-0.5 text-[13px] text-muted-foreground">
            Peran atau akun selain superadmin (mis. QA, auditor) yang boleh
            memulai impersonasi, beserta peran atau akun yang boleh mereka
            impersonate. Mereka tidak dapat mengubah kebijakan ini.
          </p>
        </div>

        {draf.pelaku.length === 0 ? (
          <p className="rounded-lg bg-secondary px-4 py-6 text-center text-sm text-muted-foreground">
            Belum ada pelaku tambahan. Hanya superadmin yang dapat
            meng-impersonate.
          </p>
        ) : (
          <ul className="grid gap-3">
            {draf.pelaku.map((p) => (
              <li key={p.kunci}>
                <KartuPelaku
                  pelaku={p}
                  onUbah={(f) => ubahPelaku(p.kunci, f)}
                  onHapus={() =>
                    setDraf((d) => ({
                      ...d,
                      pelaku: d.pelaku.filter((x) => x.kunci !== p.kunci),
                    }))
                  }
                />
              </li>
            ))}
          </ul>
        )}

        <div className="grid gap-3 rounded-lg border border-dashed border-border p-3 sm:grid-cols-2">
          <div className="grid gap-1.5">
            <Label htmlFor="tambah-pelaku-peran">
              Tambah pelaku berdasarkan peran
            </Label>
            <select
              id="tambah-pelaku-peran"
              className={kelasPilihan}
              value=""
              onChange={(e) => {
                const peran = e.target.value as Role;
                if (peran) tambahPelaku({ tipe: "PERAN", peran });
              }}
            >
              <option value="">Pilih peran…</option>
              {PERAN_DAPAT_DIPILIH.filter((r) => !sudahPelakuPeran.has(r)).map(
                (r) => (
                  <option key={r} value={r}>
                    {ROLES[r].label}
                  </option>
                ),
              )}
            </select>
          </div>
          <div className="grid gap-1.5">
            <Label>Tambah pelaku berdasarkan akun</Label>
            <PemilihPengguna
              label="Cari akun pelaku"
              kecuali={sudahPelakuAkun}
              onPilih={(u: PenggunaCari) =>
                tambahPelaku({
                  tipe: "PENGGUNA",
                  penggunaId: u.id,
                  nama: u.nama,
                  email: u.email,
                  penggunaPeran: u.peran,
                })
              }
            />
          </div>
        </div>
      </section>

      <div className="sticky bottom-3 z-10 grid gap-2 rounded-xl border border-border bg-card/95 p-3 shadow-ambient-sm backdrop-blur">
        {galat.length > 0 && (
          <ul
            role="alert"
            className="grid list-disc gap-0.5 pl-5 text-sm text-destructive"
          >
            {galat.map((g) => (
              <li key={g}>{g}</li>
            ))}
          </ul>
        )}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-muted-foreground">
            {kebijakan.diperbarui_pada
              ? `Terakhir disimpan ${formatRelatif(kebijakan.diperbarui_pada)}${
                  kebijakan.diperbarui_oleh_nama
                    ? ` oleh ${kebijakan.diperbarui_oleh_nama}`
                    : ""
                }.`
              : "Belum pernah disimpan: kebijakan bawaan berlaku."}
          </p>
          <div className="flex gap-2">
            <Button
              variant="outline"
              disabled={!berubah || simpan.isPending}
              onClick={() => setDraf(toDraf(kebijakan))}
            >
              Buang perubahan
            </Button>
            <SpringButton
              disabled={!berubah || galat.length > 0 || simpan.isPending}
              onClick={() => simpan.mutate(toInput(draf, kebijakan.versi))}
            >
              {simpan.isPending ? "Menyimpan…" : "Simpan kebijakan"}
            </SpringButton>
          </div>
        </div>
      </div>
    </div>
  );
}

function KartuPelaku({
  pelaku,
  onUbah,
  onHapus,
}: {
  pelaku: DrafPelaku;
  onUbah: (f: (p: DrafPelaku) => DrafPelaku) => void;
  onHapus: () => void;
}) {
  const adalahPeran = pelaku.tipe === "PERAN";
  const nama = adalahPeran
    ? labelPeran(pelaku.peran)
    : pelaku.nama || pelaku.penggunaId || "Akun";
  const Ikon = adalahPeran ? Users : User;
  const targetPeran = new Set(
    pelaku.target.filter((t) => t.tipe === "PERAN").map((t) => t.peran as Role),
  );
  const targetAkun = pelaku.target.filter((t) => t.tipe === "PENGGUNA");
  const setTarget = (f: (t: readonly DrafTarget[]) => DrafTarget[]) =>
    onUbah((p) => ({ ...p, target: f(p.target) }));

  return (
    <article
      aria-label={`Pelaku ${nama}`}
      className={cn(
        "grid gap-3 rounded-lg border border-border bg-card p-3",
        !pelaku.aktif && "opacity-80",
      )}
    >
      <header className="flex flex-wrap items-center gap-2">
        <Ikon className="size-4 text-muted-foreground" aria-hidden />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">{nama}</p>
          <p className="truncate text-xs text-muted-foreground">
            {adalahPeran
              ? "Semua akun berperan ini"
              : `${pelaku.email ?? ""}${pelaku.penggunaPeran ? ` · ${labelPeran(pelaku.penggunaPeran)}` : ""}`}
          </p>
        </div>
        <label className="flex items-center gap-1.5 text-xs">
          <Checkbox
            checked={pelaku.aktif}
            onCheckedChange={(v) =>
              onUbah((p) => ({ ...p, aktif: v === true }))
            }
            aria-label={`Pelaku ${nama} aktif`}
          />
          Aktif
        </label>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onHapus}
          aria-label={`Hapus pelaku ${nama}`}
        >
          <Trash2 className="size-4" />
        </Button>
      </header>

      <fieldset className="grid gap-2">
        <legend className="mb-1 text-xs font-medium text-muted-foreground">
          Peran yang boleh di-impersonate
        </legend>
        <div className="flex flex-wrap gap-1.5">
          <Button
            variant="outline"
            size="xs"
            onClick={() =>
              setTarget((t) => setPeranTarget(t, PERAN_DAPAT_DIPILIH))
            }
          >
            Semua peran (tanpa Administrator Sistem)
          </Button>
          <Button
            variant="outline"
            size="xs"
            onClick={() =>
              setTarget((t) => setPeranTarget(t, PERAN_OPERASIONAL))
            }
          >
            Lapis operasional saja
          </Button>
          <Button
            variant="ghost"
            size="xs"
            onClick={() => setTarget((t) => setPeranTarget(t, []))}
          >
            Kosongkan peran
          </Button>
        </div>
        <div className="grid gap-1.5 sm:grid-cols-2 lg:grid-cols-3">
          {PERAN_DAPAT_DIPILIH.map((r) => {
            const id = `${pelaku.kunci}-target-${r}`;
            return (
              <label
                key={r}
                htmlFor={id}
                className="flex items-center gap-2 text-[13px]"
              >
                <Checkbox
                  id={id}
                  checked={targetPeran.has(r)}
                  onCheckedChange={(v) =>
                    setTarget((t) => aturPeranTarget(t, r, v === true))
                  }
                />
                {ROLES[r].label}
              </label>
            );
          })}
        </div>
      </fieldset>

      <div className="grid gap-2">
        <p className="text-xs font-medium text-muted-foreground">
          Akun tertentu yang boleh di-impersonate
        </p>
        {targetAkun.length > 0 && (
          <ul className="flex flex-wrap gap-1.5">
            {targetAkun.map((t) => (
              <li
                key={t.penggunaId}
                className="inline-flex items-center gap-1 rounded-md bg-secondary py-0.5 pr-0.5 pl-2 text-xs"
              >
                <span>{t.nama || t.penggunaId}</span>
                <button
                  type="button"
                  aria-label={`Hapus target ${t.nama || t.penggunaId}`}
                  onClick={() =>
                    setTarget((x) =>
                      x.filter(
                        (y) =>
                          !(
                            y.tipe === "PENGGUNA" &&
                            y.penggunaId === t.penggunaId
                          ),
                      ),
                    )
                  }
                  className="grid size-5 place-items-center rounded hover:bg-border"
                >
                  <X className="size-3" aria-hidden />
                </button>
              </li>
            ))}
          </ul>
        )}
        <PemilihPengguna
          label={`Cari akun target ${nama}`}
          kecuali={targetAkun.map((t) => t.penggunaId as string)}
          onPilih={(u) =>
            setTarget((t) => [
              ...t,
              {
                tipe: "PENGGUNA",
                penggunaId: u.id,
                nama: u.nama,
                email: u.email,
                penggunaPeran: u.peran,
              },
            ])
          }
        />
      </div>
    </article>
  );
}
