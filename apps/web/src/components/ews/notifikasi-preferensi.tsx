"use client";

import { useState } from "react";
import { toast } from "sonner";
import { PanelPas } from "@/components/garda/halaman-pas";
import { SpringButton } from "@/components/garda/spring-button";
import { Checkbox } from "@/components/ui/checkbox";
import { kelasKontrol } from "@/components/ui/kontrol";
import type { KanalNotifikasi, PreferensiNotifikasi } from "@/lib/api/ews";
import { severityMeta, type Severity } from "@/lib/design/tokens";
import { useCan } from "@/lib/auth/session-context";
import {
  usePreferensiNotifikasi,
  useSimpanPreferensi,
} from "@/lib/query/hooks";
import { aktifkanPush, nonaktifkanPush, pushDidukung } from "@/lib/push";
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
    simpan.mutate(
      data.preferensi.map((p) => (p.kanal === kanal ? { ...p, ...patch } : p)),
    );
    // Push dimatikan: langganan perangkat ini ikut dilepas agar server berhenti mengirim ke sini.
    if (kanal === "PUSH" && patch.aktif === false) void lepasPush();
  };

  async function lepasPush() {
    try {
      const hasil = await nonaktifkanPush();
      if (hasil === "dilepas")
        toast.success("Push dinonaktifkan di perangkat ini");
    } catch (err) {
      toast.error("Langganan push belum terlepas", {
        description:
          err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    }
  }

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
      className="lg:flex-1"
    >
      <div className="space-y-3">
        <p className="text-xs leading-snug text-muted-foreground">
          Hanya mengatur alert yang dikirim kepada Anda. Halaman ini tetap
          menampilkan semuanya.
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
              <select
                aria-label={`Tingkat minimum ${KANAL_LABEL[p.kanal]}`}
                value={p.severityMin}
                disabled={!p.aktif || simpan.isPending}
                onChange={(e) =>
                  ubah(p.kanal, { severityMin: e.target.value as Severity })
                }
                className={cn(
                  kelasKontrol,
                  "h-9 w-full px-2.5 text-base md:text-[13px] dark:bg-card dark:scheme-dark",
                )}
              >
                {TINGKAT.map((s) => (
                  <option key={s} value={s}>
                    ≥ {severityMeta[s].label}
                  </option>
                ))}
              </select>
            </div>
            {!p.tersedia && (
              <p className="text-xs text-muted-foreground">
                Pengiriman {KANAL_LABEL[p.kanal].split(" ")[0].toLowerCase()}{" "}
                belum dikonfigurasi di server; notifikasi hanya dicatat.
              </p>
            )}
            {p.kanal === "PUSH" && p.tersedia && p.aktif && pushDidukung() && (
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
