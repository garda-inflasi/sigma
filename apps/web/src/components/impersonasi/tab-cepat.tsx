"use client";

import { UserCog } from "lucide-react";
import { useMemo, useState } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Button } from "@/components/ui/button";
import { kelompokkanPeranTarget } from "@/lib/api/impersonasi";
import { useTargetImpersonasi } from "@/lib/query/impersonasi-hooks";
import { LAPIS_META, ROLES, type Lapis } from "@/lib/rbac/roles";
import { DialogAlihPeran, type PilihanAwal } from "./dialog-alih-peran";

/**
 * Tab "Impersonate cepat": peran dan akun yang dapat di-impersonate pemanggil, masing-masing dengan tombol yang
 * langsung membuka dialog pemilih (alasan, konfirmasi) dengan akun itu sudah terpilih.
 */
export function TabCepat() {
  const { data, isLoading, error, refetch } = useTargetImpersonasi(true);
  const [pilihan, setPilihan] = useState<PilihanAwal | null>(null);
  const kelompok = useMemo(
    () => kelompokkanPeranTarget(data?.peran ?? []),
    [data],
  );

  if (isLoading) {
    return <div className="glass h-48 animate-pulse rounded-xl" aria-busy="true" />;
  }
  if (error || !data) {
    return (
      <PanelGalat
        judul="Daftar sasaran gagal dimuat"
        error={error}
        onCoba={() => void refetch()}
      />
    );
  }
  if (!data.aktif) {
    return (
      <EmptyState
        icon={UserCog}
        title="Impersonasi sedang dinonaktifkan"
        description="Aktifkan kembali di tab Kebijakan untuk memulai impersonasi."
      />
    );
  }
  if (!data.boleh || data.peran.length === 0) {
    return (
      <EmptyState
        icon={UserCog}
        title="Belum ada akun yang dapat di-impersonate"
        description="Hanya akun aktif selain Administrator Sistem yang dapat menjadi sasaran."
      />
    );
  }

  return (
    <div className="grid gap-4">
      <p className="text-[13px] text-muted-foreground">
        Pilih akun untuk bekerja sementara sebagai akun itu. Semua aksi tercatat atas nama akun asli Anda.
        {data.durasi_maks_menit ? ` Sesi berakhir otomatis setelah ${data.durasi_maks_menit} menit.` : ""}
      </p>
      {kelompok.map((g) => (
        <section key={g.lapis} aria-label={LAPIS_META[g.lapis as Lapis]?.label ?? g.lapis} className="grid gap-2">
          <h3 className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
            {LAPIS_META[g.lapis as Lapis]?.label ?? g.lapis}
          </h3>
          <div className="glass overflow-hidden rounded-xl">
            <table className="w-full text-sm">
              <thead className="text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-3 py-2 font-medium">Peran</th>
                  <th className="px-3 py-2 font-medium">Akun</th>
                  <th className="hidden px-3 py-2 font-medium md:table-cell">Lembaga</th>
                  <th className="px-3 py-2 text-right font-medium">
                    <span className="sr-only">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {g.peran.flatMap((p) =>
                  p.akun.map((a, i) => (
                    <tr key={a.id} className="border-t border-border/60">
                      <td className="px-3 py-2 align-top text-[13px] font-medium">
                        {i === 0 ? (ROLES[p.peran as keyof typeof ROLES]?.label ?? p.label) : ""}
                      </td>
                      <td className="px-3 py-2">
                        <span className="block">{a.nama}</span>
                        <span className="block text-xs text-muted-foreground">{a.email}</span>
                      </td>
                      <td className="hidden px-3 py-2 text-xs text-muted-foreground md:table-cell">{a.lembaga}</td>
                      <td className="px-3 py-2 text-right">
                        <Button
                          size="sm"
                          variant="outline"
                          aria-label={`Impersonate ${a.nama}`}
                          onClick={() => setPilihan({ peran: p.peran, penggunaId: a.id })}
                        >
                          <UserCog className="size-3.5" aria-hidden /> Impersonate
                        </Button>
                      </td>
                    </tr>
                  )),
                )}
              </tbody>
            </table>
          </div>
        </section>
      ))}
      <DialogAlihPeran
        open={pilihan !== null}
        onOpenChange={(v) => {
          if (!v) setPilihan(null);
        }}
        awal={pilihan ?? undefined}
      />
    </div>
  );
}
