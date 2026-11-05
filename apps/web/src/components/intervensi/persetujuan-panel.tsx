"use client";

import { Check, CircleCheck, CircleDashed, X } from "lucide-react";
import { useState } from "react";
import { PanelTitle } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { bisaMemutus, type Persetujuan } from "@/lib/api/intervensi";
import { useSession } from "@/lib/auth/session-context";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";
import { formatRelatif } from "@/lib/format";
import { usePersetujuan, usePutusPersetujuan } from "@/lib/query/hooks";
import { can, type Permission } from "@/lib/rbac/permissions";
import { cn } from "@/lib/utils";

const SEMUA_TAHAP = Object.values(SIKLUS_TAHAP).flat();
const judulTahap = (id?: string) =>
  SEMUA_TAHAP.find((t) => t.id === id)?.judul ?? id ?? "";

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
  const tolak = () => {
    if (!permintaan) return;
    putus.mutate(
      { id: permintaan.id, keputusan: "TOLAK", catatan: catatan.trim() },
      {
        onSuccess: () => {
          setCatatan("");
          onClose();
        },
      },
    );
  };
  return (
    <Dialog
      open={permintaan !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="glass-strong rounded-xl border-0 sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Tolak permintaan?</DialogTitle>
          <DialogDescription>
            {permintaan?.kartuJudul}. Alasan penolakan dicatat di riwayat kartu
            dan audit trail.
          </DialogDescription>
        </DialogHeader>
        <Textarea
          value={catatan}
          onChange={(e) => setCatatan(e.target.value)}
          placeholder="Alasan penolakan"
          maxLength={500}
          aria-label="Alasan penolakan"
        />
        <div className="flex justify-end gap-2">
          <SpringButton variant="soft" onClick={onClose}>
            Batal
          </SpringButton>
          <SpringButton
            disabled={catatan.trim() === "" || putus.isPending}
            onClick={tolak}
          >
            Tolak
          </SpringButton>
        </div>
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
  const { id: userId, role } = useSession();
  const { data, isLoading } = usePersetujuan(can(role, "intervensi.lihat"));
  const putus = usePutusPersetujuan();
  const [ditolak, setDitolak] = useState<Persetujuan | null>(null);

  if (isLoading || !data || data.length === 0) return null;
  return (
    <section
      className="glass rounded-xl p-5 sm:p-6"
      aria-label="Antrean persetujuan"
    >
      <PanelTitle
        title="Menunggu persetujuan"
        description="Pemindahan ke tahap persetujuan dan telaah eskalasi diputuskan berjenjang; satu orang tidak memutus dua langkah, dan pengaju tidak menyetujui sendiri."
      />
      <ul className="mt-4 divide-y divide-border/60">
        {data.map((p) => {
          const boleh = bisaMemutus(
            p,
            userId,
            can(role, p.izinLangkahKini as Permission),
          );
          return (
            <li
              key={p.id}
              className="flex flex-wrap items-start justify-between gap-3 py-3"
            >
              <div className="min-w-0">
                <p className="text-sm font-medium">{p.kartuJudul}</p>
                <p className="text-xs text-muted-foreground">
                  {p.jenis === "ESKALASI"
                    ? "Telaah eskalasi"
                    : `Pindah ke “${judulTahap(p.tahapTujuan)}”`}{" "}
                  · diajukan {p.diajukanOleh ?? "sistem"}{" "}
                  {formatRelatif(p.diajukanPada)}
                </p>
                <Rantai p={p} />
              </div>
              {boleh && (
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
              )}
            </li>
          );
        })}
      </ul>
      <DialogTolak permintaan={ditolak} onClose={() => setDitolak(null)} />
    </section>
  );
}
