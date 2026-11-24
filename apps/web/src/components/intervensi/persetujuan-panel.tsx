"use client";

import { Check, CircleCheck, CircleDashed, X } from "lucide-react";
import { useState } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelPas } from "@/components/garda/halaman-pas";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import {
  bisaMemutus,
  ringkasRantai,
  type Persetujuan,
} from "@/lib/api/intervensi";
import { useSession } from "@/lib/auth/session-context";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";
import { siklusMeta } from "@/lib/design/tokens";
import { formatRelatif } from "@/lib/format";
import {
  useAlurPersetujuan,
  usePersetujuan,
  usePutusPersetujuan,
  useTahapIntervensi,
} from "@/lib/query/hooks";
import { can, type Permission } from "@/lib/rbac/permissions";
import { cn } from "@/lib/utils";
import { z } from "zod";
import { FormDialog } from "@/components/form/form-bff";
import { FormAlert } from "@/components/form/form-field";
import { useFormBff } from "@/lib/form/use-form-bff";

/** Label tahap dari definisi server (cadangan klien selama dimuat); kode tahap bila tak dikenal. */
function useJudulTahap() {
  const { data } = useTahapIntervensi();
  const semua = Object.values(data ?? SIKLUS_TAHAP).flat();
  return (id?: string) => semua.find((t) => t.id === id)?.judul ?? id ?? "";
}

const skemaTolak = z.object({
  catatan: z.string().trim().min(1, "Alasan penolakan wajib diisi."),
});

/** Dialog catatan penolakan; penolakan wajib disertai alasan (ditegakkan juga di server). */
function DialogTolak({
  permintaan,
  onClose,
}: {
  permintaan: Persetujuan | null;
  onClose: () => void;
}) {
  const putus = usePutusPersetujuan();
  const [catatan, setCatatan] = useState("");
  // Toast hasil/galat dari hook mutasi; galat juga tampil di dialog.
  const kiriman = useFormBff<z.output<typeof skemaTolak>>({
    skema: skemaTolak,
    kirim: (n) =>
      putus.mutateAsync({
        id: permintaan!.id,
        keputusan: "TOLAK",
        catatan: n.catatan,
      }),
    sukses: () => {
      setCatatan("");
      onClose();
    },
  });
  return (
    <Dialog
      open={permintaan !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Tolak permintaan?</DialogTitle>
          <DialogDescription>
            {permintaan?.jenis === "ESKALASI"
              ? `Telaah eskalasi “${permintaan.kartuJudul}” ditutup dan tanda eskalasinya dicabut.`
              : `Permintaan memindahkan “${permintaan?.kartuJudul}” ditutup dan kartu tetap di tahapnya.`}{" "}
            Alasan penolakan dicatat di riwayat kartu dan audit trail.
          </DialogDescription>
        </DialogHeader>
        <FormDialog
          onKirim={kiriman.kirimDari(() => ({ catatan }))}
          sibuk={kiriman.pending}
        >
          <DialogBody>
            <Textarea
              value={catatan}
              onChange={(e) => setCatatan(e.target.value)}
              placeholder="Alasan penolakan"
              maxLength={500}
              aria-label="Alasan penolakan"
            />
            <FormAlert>{kiriman.galat?.pesan}</FormAlert>
          </DialogBody>
          <DialogFooter>
            <SpringButton variant="soft" onClick={onClose}>
              Batal
            </SpringButton>
            <SpringButton
              type="submit"
              disabled={catatan.trim() === "" || kiriman.pending}
            >
              Tolak
            </SpringButton>
          </DialogFooter>
        </FormDialog>
      </DialogContent>
    </Dialog>
  );
}

function Rantai({ p }: { p: Persetujuan }) {
  return (
    <ol className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs">
      {p.langkah.map((l) => {
        const selesai = l.keputusan === "SETUJU";
        const kini = p.status === "MENUNGGU" && l.urutan === p.langkahKini;
        return (
          <li
            key={l.urutan}
            className={cn(
              "inline-flex items-center gap-1",
              kini ? "font-medium text-foreground" : "text-muted-foreground",
            )}
          >
            {selesai ? (
              <CircleCheck className="size-3.5 text-normal-ink" />
            ) : (
              <CircleDashed className="size-3.5" />
            )}
            {l.label}
            {l.olehNama && (
              <span className="text-muted-foreground">· {l.olehNama}</span>
            )}
          </li>
        );
      })}
    </ol>
  );
}

/** Antrean persetujuan berjenjang: pemindahan kartu ke tahap persetujuan dan telaah eskalasi. */
export function PersetujuanPanel() {
  const sesi = useSession();
  const userId = sesi.id;
  const { data, isLoading, isError, error, refetch } = usePersetujuan(
    can(sesi, "intervensi.lihat"),
  );
  const putus = usePutusPersetujuan();
  const judulTahap = useJudulTahap();
  const [ditolak, setDitolak] = useState<Persetujuan | null>(null);

  return (
    <PanelPas
      judul="Menunggu persetujuan"
      deskripsi={
        data?.length
          ? `${data.length} permintaan, diputuskan berjenjang`
          : "Pemindahan tahap dan telaah eskalasi"
      }
      aria-label="Antrean persetujuan"
      tanpaPadding
    >
      {isLoading ? (
        <div
          role="status"
          aria-label="Memuat antrean"
          className="space-y-2 p-3.5"
        >
          <Shimmer className="h-16 w-full" />
          <Shimmer className="h-16 w-full" />
        </div>
      ) : isError ? (
        <PanelGalat
          judul="Antrean persetujuan belum dapat dimuat"
          error={error}
          onCoba={() => void refetch()}
          className="m-3.5"
        />
      ) : !data || data.length === 0 ? (
        <EmptyState
          title="Tidak ada permintaan menunggu"
          description="Permintaan muncul di sini saat kartu diajukan ke tahap bergembok atau dieskalasi."
          className="py-10"
        />
      ) : (
        <>
          <p className="border-b border-border px-3.5 py-2 text-xs text-muted-foreground">
            Satu orang tidak memutus dua langkah, dan pengaju tidak menyetujui
            sendiri.
          </p>
          <ul className="divide-y divide-border">
            {data.map((p) => {
              const boleh = bisaMemutus(
                p,
                userId,
                can(sesi, p.izinLangkahKini as Permission),
              );
              const langkahKini = p.langkah.find(
                (l) => l.urutan === p.langkahKini,
              );
              return (
                <li
                  key={p.id}
                  className="flex flex-wrap items-start justify-between gap-3 px-3.5 py-3"
                >
                  <div className="min-w-0 flex-1 basis-56">
                    <p className="text-[13px] font-medium">{p.kartuJudul}</p>
                    <p className="text-xs text-muted-foreground">
                      {p.jenis === "ESKALASI"
                        ? "Telaah eskalasi"
                        : `Pindah ke “${judulTahap(p.tahapTujuan)}”`}{" "}
                      · diajukan {p.diajukanOleh ?? "sistem"}{" "}
                      {formatRelatif(p.diajukanPada)}
                    </p>
                    <Rantai p={p} />
                  </div>
                  {boleh ? (
                    <div className="flex shrink-0 gap-2">
                      <SpringButton
                        variant="soft"
                        size="sm"
                        disabled={putus.isPending}
                        onClick={() => setDitolak(p)}
                      >
                        <X /> Tolak
                      </SpringButton>
                      <SpringButton
                        size="sm"
                        disabled={putus.isPending}
                        onClick={() =>
                          putus.mutate({ id: p.id, keputusan: "SETUJU" })
                        }
                      >
                        <Check /> Setujui
                      </SpringButton>
                    </div>
                  ) : (
                    langkahKini && (
                      <p className="shrink-0 text-xs text-muted-foreground">
                        Menunggu {langkahKini.label}
                      </p>
                    )
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}
      <DialogTolak permintaan={ditolak} onClose={() => setDitolak(null)} />
    </PanelPas>
  );
}

/**
 * Referensi alur persetujuan dari GET /v1/intervensi/alur: tahap mana yang butuh persetujuan, siapa
 * (izin apa) yang memutus tiap langkah, dan urutannya. Membantu pengaju memperkirakan jalur sebelum meminta.
 */
export function AlurPersetujuanPanel() {
  const sesi = useSession();
  const { data, isLoading, isError, error, refetch } = useAlurPersetujuan(
    can(sesi, "intervensi.lihat"),
  );
  const judulTahap = useJudulTahap();
  const aktif = (data ?? []).filter((a) => a.aktif);
  return (
    <PanelPas
      judul="Alur persetujuan"
      deskripsi="Urutan keputusan untuk tahap bergembok"
      aria-label="Alur persetujuan"
    >
      {isLoading ? (
        <div role="status" aria-label="Memuat alur" className="space-y-2">
          <Shimmer className="h-20 w-full" />
          <Shimmer className="h-20 w-full" />
        </div>
      ) : isError ? (
        <PanelGalat
          judul="Alur persetujuan belum dapat dimuat."
          error={error}
          onCoba={() => void refetch()}
        />
      ) : aktif.length === 0 ? (
        <EmptyState
          title="Belum ada alur aktif"
          description="Bila alur persetujuan diaktifkan, rantai keputusannya tampil di sini."
          className="py-10"
        />
      ) : (
        <ul className="grid grid-cols-1 gap-2">
          {aktif.map((a) => (
            <li
              key={a.kode}
              className="rounded-lg bg-secondary/50 px-3 py-2.5 text-[13px]"
            >
              <p className="font-medium">{a.nama}</p>
              <p className="text-xs text-muted-foreground">
                {a.jenis === "ESKALASI"
                  ? "Berlaku untuk semua siklus"
                  : `${a.siklus ? siklusMeta[a.siklus].label : ""} · menuju “${judulTahap(a.tahapTujuan)}”`}
              </p>
              <ol className="mt-2 space-y-1 text-xs">
                {a.langkah.map((l) => (
                  <li
                    key={l.urutan}
                    className="flex items-baseline gap-2"
                    title={`Izin: ${l.izin}`}
                  >
                    <span className="num grid size-4 shrink-0 place-items-center rounded-full bg-navy-800 text-[10px] text-white dark:bg-white/20">
                      {l.urutan}
                    </span>
                    <span>{l.label}</span>
                  </li>
                ))}
              </ol>
              <span className="sr-only">{ringkasRantai(a)}</span>
            </li>
          ))}
        </ul>
      )}
    </PanelPas>
  );
}
