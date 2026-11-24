"use client";

import { History } from "lucide-react";
import { useState } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Segmented } from "@/components/garda/segmented";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  labelAlasanBerakhir,
  labelPeran,
  type RiwayatImpersonasiWire,
} from "@/lib/api/impersonasi";
import { formatTanggal } from "@/lib/format";
import {
  useAkhiriImpersonasi,
  useRiwayatImpersonasi,
} from "@/lib/query/impersonasi-hooks";

const OPSI_STATUS = [
  { value: "semua", label: "Semua" },
  { value: "aktif", label: "Aktif" },
] as const;

const waktu = (iso: string) => formatTanggal(iso, "d MMM yyyy HH:mm");

/**
 * Tab "Riwayat": sesi impersonasi (pelaku, sasaran, alasan, waktu, status) dengan paginasi cursor, saringan
 * aktif/semua, dan pengakhiran paksa sesi yang masih berjalan. Jejak lengkap aksi per sesi ada di audit trail
 * (aktor = pelaku asli, metadata.impersonasi memuat id sesi).
 */
export function TabRiwayat() {
  const [status, setStatus] = useState<"semua" | "aktif">("semua");
  const q = useRiwayatImpersonasi(status);
  const akhiri = useAkhiriImpersonasi();
  const [konfirmasi, setKonfirmasi] = useState<RiwayatImpersonasiWire | null>(
    null,
  );

  const baris = q.data?.pages.flatMap((h) => h.items) ?? [];

  return (
    <div className="flex min-w-0 flex-col gap-3 lg:min-h-0 lg:flex-1">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
        <Segmented
          layoutId="seg-riwayat-impersonasi"
          size="sm"
          value={status}
          onChange={setStatus}
          options={[...OPSI_STATUS]}
          className="w-fit"
        />
        {q.hasNextPage && (
          <Button
            variant="outline"
            size="sm"
            disabled={q.isFetchingNextPage}
            onClick={() => void q.fetchNextPage()}
          >
            {q.isFetchingNextPage ? "Memuat…" : "Muat lebih banyak"}
          </Button>
        )}
      </div>

      {q.isLoading ? (
        <div
          className="h-48 animate-pulse rounded-lg bg-secondary"
          aria-busy="true"
        />
      ) : q.error ? (
        <PanelGalat
          judul="Riwayat impersonasi gagal dimuat"
          error={q.error}
          onCoba={() => void q.refetch()}
        />
      ) : baris.length === 0 ? (
        <EmptyState
          icon={History}
          title={
            status === "aktif"
              ? "Tidak ada sesi impersonasi yang aktif"
              : "Belum ada riwayat impersonasi"
          }
          description="Setiap sesi impersonasi yang dimulai akan tercatat di sini dan di audit trail."
        />
      ) : (
        <div className="scrollbar-thin relative max-h-[70dvh] overflow-auto rounded-lg border border-border bg-card lg:max-h-none lg:min-h-0 lg:flex-1">
          <table className="w-full min-w-[820px] text-sm">
            <thead className="sticky top-0 z-10 text-left">
              <tr>
                <th className="h-9 bg-secondary px-3 text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase">
                  Pelaku
                </th>
                <th className="h-9 bg-secondary px-3 text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase">
                  Sasaran
                </th>
                <th className="h-9 bg-secondary px-3 text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase">
                  Alasan
                </th>
                <th className="h-9 bg-secondary px-3 text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase">
                  Mulai
                </th>
                <th className="h-9 bg-secondary px-3 text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase">
                  Berakhir
                </th>
                <th className="h-9 bg-secondary px-3 text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase">
                  Status
                </th>
                <th className="h-9 bg-secondary px-3 text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase text-right">
                  <span className="sr-only">Aksi</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {baris.map((r) => (
                <tr
                  key={r.id}
                  className="border-t border-border align-top text-[13px] hover:bg-secondary/60"
                >
                  <td className="px-3 py-2">
                    <span className="block font-medium">{r.pelaku.nama}</span>
                    <span className="block text-xs text-muted-foreground">
                      {labelPeran(r.pelaku.peran)}
                    </span>
                  </td>
                  <td className="px-3 py-2">
                    <span className="block font-medium">{r.sasaran.nama}</span>
                    <span className="block text-xs text-muted-foreground">
                      {labelPeran(r.sasaran.peran)}
                    </span>
                  </td>
                  <td className="max-w-56 px-3 py-2 text-[13px] break-words text-muted-foreground">
                    {r.alasan || "—"}
                  </td>
                  <td className="num px-3 py-2 text-xs whitespace-nowrap">
                    {waktu(r.mulai_pada)}
                  </td>
                  <td className="num px-3 py-2 text-xs whitespace-nowrap">
                    {r.berakhir_pada
                      ? waktu(r.berakhir_pada)
                      : r.kedaluwarsa_pada
                        ? `maks. ${waktu(r.kedaluwarsa_pada)}`
                        : "—"}
                  </td>
                  <td className="px-3 py-2">
                    {r.aktif ? (
                      <StatusBadge tone="waspada">Aktif</StatusBadge>
                    ) : (
                      <StatusBadge tone="netral">
                        {labelAlasanBerakhir(r.alasan_berakhir) || "Berakhir"}
                      </StatusBadge>
                    )}
                  </td>
                  <td className="px-3 py-2 text-right">
                    {r.aktif && (
                      <Button
                        size="sm"
                        variant="outline"
                        aria-label={`Akhiri sesi ${r.pelaku.nama} sebagai ${r.sasaran.nama}`}
                        onClick={() => setKonfirmasi(r)}
                      >
                        Akhiri
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <AlertDialog
        open={konfirmasi !== null}
        onOpenChange={(v) => {
          if (!v) setKonfirmasi(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Akhiri sesi impersonasi?</AlertDialogTitle>
            <AlertDialogDescription>
              {konfirmasi?.pelaku.nama} akan keluar dari sesi sebagai{" "}
              {konfirmasi?.sasaran.nama} pada request berikutnya dan harus masuk
              kembali. Tindakan ini tercatat di audit trail.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                if (konfirmasi) akhiri.mutate(konfirmasi.id);
                setKonfirmasi(null);
              }}
            >
              Akhiri sesi
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
