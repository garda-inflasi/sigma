"use client";

import { UserCog } from "lucide-react";
import { useMemo, useState } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import { IsiPas } from "@/components/garda/halaman-pas";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { Button } from "@/components/ui/button";
import { kelompokkanPeranTarget } from "@/lib/api/impersonasi";
import { useTargetImpersonasi } from "@/lib/query/impersonasi-hooks";
import { LAPIS_META, ROLES, type Lapis } from "@/lib/rbac/roles";
import { DampakImpersonasi } from "./dampak-impersonasi";
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
    return (
      <div aria-busy="true">
        <Shimmer className="h-48 w-full rounded-lg" />
      </div>
    );
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
    <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_20rem]">
      <DampakImpersonasi className="lg:order-2 lg:self-start">
        <li>
          {data.durasi_maks_menit
            ? `Sesi berakhir otomatis setelah ${data.durasi_maks_menit} menit.`
            : "Tidak ada batas durasi; kembali secara manual bila selesai."}
        </li>
      </DampakImpersonasi>
      <div className="scrollbar-thin grid min-w-0 grid-cols-1 content-start gap-4 lg:order-1 lg:min-h-0 lg:overflow-y-auto">
        {kelompok.map((g) => (
          <section
            key={g.lapis}
            aria-label={LAPIS_META[g.lapis as Lapis]?.label ?? g.lapis}
            className="grid min-w-0 grid-cols-1 gap-2"
          >
            <h3 className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
              {LAPIS_META[g.lapis as Lapis]?.label ?? g.lapis}
            </h3>
            <div className="glass relative overflow-x-auto rounded-lg">
              <table className="w-full text-sm">
                <thead className="text-left text-xs text-muted-foreground">
                  <tr>
                    <th className="px-3 py-2 font-medium">Peran</th>
                    <th className="px-3 py-2 font-medium">Akun</th>
                    <th className="hidden px-3 py-2 font-medium md:table-cell">
                      Lembaga
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      <span className="sr-only">Aksi</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {g.peran.flatMap((p) =>
                    p.akun.map((a, i) => (
                      <tr key={a.id} className="border-t border-border">
                        <td className="px-3 py-2 align-top text-[13px] font-medium">
                          {i === 0
                            ? (ROLES[p.peran as keyof typeof ROLES]?.label ??
                              p.label)
                            : ""}
                        </td>
                        <td className="px-3 py-2">
                          <span className="block">{a.nama}</span>
                          <span className="block text-xs text-muted-foreground">
                            {a.email}
                          </span>
                        </td>
                        <td className="hidden px-3 py-2 text-xs text-muted-foreground md:table-cell">
                          {a.lembaga}
                        </td>
                        <td className="px-3 py-2 text-right">
                          <Button
                            size="sm"
                            variant="outline"
                            aria-label={`Impersonate ${a.nama}`}
                            onClick={() =>
                              setPilihan({ peran: p.peran, penggunaId: a.id })
                            }
                          >
                            <UserCog className="size-3.5" aria-hidden />
                            <span className="max-sm:hidden">Impersonate</span>
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
      </div>
      <DialogAlihPeran
        open={pilihan !== null}
        onOpenChange={(v) => {
          if (!v) setPilihan(null);
        }}
        awal={pilihan ?? undefined}
      />
    </IsiPas>
  );
}
