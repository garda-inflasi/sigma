"use client";

import { useState } from "react";
import { toast } from "sonner";
import { PanelPas } from "@/components/garda/halaman-pas";
import { SpringButton } from "@/components/garda/spring-button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { KanalNotifikasi, PreferensiNotifikasi } from "@/lib/api/ews";
import { severityMeta, type Severity } from "@/lib/design/tokens";
import { useCan } from "@/lib/auth/session-context";
import {
  usePreferensiNotifikasi,
  useSimpanPreferensi,
} from "@/lib/query/hooks";
import { aktifkanPush, pushDidukung } from "@/lib/push";
import { cn } from "@/lib/utils";

const KANAL_LABEL: Record<KanalNotifikasi, string> = {
  EMAIL: "Email",
  PUSH: "Push (perangkat ini)",
};
const TINGKAT: Severity[] = ["INFO", "WASPADA", "SIAGA", "KRITIS"];

/** Preferensi notifikasi EWS milik pengguna: kanal, tingkat minimum, dan perangkat Web Push. */
export function NotifikasiPreferensi() {
  const bisa = useCan("ews.akui");
  const { data, isLoading } = usePreferensiNotifikasi(bisa);
  const simpan = useSimpanPreferensi();
  const [sibukPush, setSibukPush] = useState(false);
  if (!bisa) return null;

  const ubah = (
    kanal: KanalNotifikasi,
    patch: Partial<PreferensiNotifikasi>,
  ) => {
    if (!data) return;
    // Langganan perangkat sengaja tidak dilepas saat kanal dimatikan: peringatan KRITIS tetap harus sampai ke sini.
    simpan.mutate(
      data.preferensi.map((p) => (p.kanal === kanal ? { ...p, ...patch } : p)),
    );
  };

  async function pasangPush() {
    if (!data) return;
    setSibukPush(true);
    try {
      const hasil = await aktifkanPush(data.vapidPublicKey);
      if (hasil === "aktif") toast.success("Push aktif di perangkat ini");
      else
        toast.error(
          hasil === "ditolak"
            ? "Izin notifikasi ditolak di browser"
            : "Browser ini tidak mendukung push",
        );
    } catch (err) {
      toast.error("Push gagal diaktifkan", {
        description:
          err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    } finally {
      setSibukPush(false);
    }
  }

  return (
    <PanelPas
      judul="Notifikasi saya"
      deskripsi="Kanal dan tingkat minimum"
    >
      <div className="space-y-3">
        <p className="text-xs leading-snug text-muted-foreground">
          Hanya mengatur alert yang dikirim kepada Anda. Halaman ini tetap
          menampilkan semuanya.
        </p>
        <p className="text-xs leading-snug font-medium text-kritis">
          Peringatan kritis selalu dikirim, apa pun tingkat minimum atau kanal
          yang dimatikan di sini.
        </p>
        {isLoading && (
          <p className="text-xs text-muted-foreground">Memuat preferensi…</p>
        )}
        {data?.preferensi.map((p) => (
          <div key={p.kanal} className="space-y-1.5">
            <div className="grid gap-1.5">
              <label className="flex min-w-0 items-center gap-2 text-sm">
                <Checkbox
                  checked={p.aktif}
                  disabled={simpan.isPending}
                  onCheckedChange={(on) =>
                    ubah(p.kanal, { aktif: on === true })
                  }
                  aria-label={`Terima notifikasi ${KANAL_LABEL[p.kanal]}`}
                />
                {KANAL_LABEL[p.kanal]}
              </label>
              <Select
                value={p.severityMin}
                disabled={!p.aktif || simpan.isPending}
                onValueChange={(v) =>
                  ubah(p.kanal, { severityMin: v as Severity })
                }
              >
                <SelectTrigger
                  aria-label={`Tingkat minimum ${KANAL_LABEL[p.kanal]}`}
                  className="md:text-[13px]"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TINGKAT.map((s) => (
                    <SelectItem key={s} value={s}>
                      ≥ {severityMeta[s].label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {!p.tersedia && (
              <p className="text-xs text-muted-foreground">
                Pengiriman {KANAL_LABEL[p.kanal].split(" ")[0].toLowerCase()}{" "}
                belum dikonfigurasi di server; notifikasi hanya dicatat.
              </p>
            )}
            {p.kanal === "PUSH" && p.tersedia && pushDidukung() && (
              <SpringButton
                variant="soft"
                size="sm"
                disabled={sibukPush}
                onClick={pasangPush}
              >
                {sibukPush ? "Mengaktifkan…" : "Aktifkan di perangkat ini"}
              </SpringButton>
            )}
          </div>
        ))}
      </div>
    </PanelPas>
  );
}
