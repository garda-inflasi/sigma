"use client";

import { ShieldX } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { EmptyState } from "@/components/garda/empty-state";
import { HalamanPas } from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { ViewTabs } from "@/components/garda/view-tabs";
import { useCan } from "@/lib/auth/session-context";
import { TabCepat } from "./tab-cepat";
import { TabKebijakan } from "./tab-kebijakan";
import { TabRiwayat } from "./tab-riwayat";

const TAB = ["kebijakan", "cepat", "riwayat"] as const;
type TabImpersonasi = (typeof TAB)[number];

/**
 * Halaman admin impersonasi (khusus superadmin): kebijakan, impersonate cepat, dan riwayat. Penjagaan berlapis:
 * edge (`ROUTE_ACCESS`), pemeriksaan izin sesi di sini, dan API Go (peran ADMIN_SISTEM + izin `impersonasi.kelola`,
 * 403 untuk selain itu; panel galat tab menampilkannya).
 */
export function ImpersonasiPage() {
  const boleh = useCan("impersonasi.kelola");
  const mentah = useSearchParams().get("tab");
  const tab: TabImpersonasi = TAB.includes(mentah as TabImpersonasi)
    ? (mentah as TabImpersonasi)
    : "kebijakan";
  // History API: berpindah tab tidak perlu memuat ulang data server, dan draf kebijakan tidak terganggu navigasi.
  const setTab = (nilai: TabImpersonasi) => {
    const q = new URLSearchParams(window.location.search);
    if (nilai === "kebijakan") q.delete("tab");
    else q.set("tab", nilai);
    const qs = q.toString();
    window.history.replaceState(
      null,
      "",
      `${window.location.pathname}${qs ? `?${qs}` : ""}`,
    );
  };

  if (!boleh) {
    return (
      <EmptyState
        icon={ShieldX}
        title="Di luar kewenangan peran Anda"
        description="Pengaturan impersonasi hanya untuk Administrator Sistem (superadmin)."
      />
    );
  }

  return (
    <HalamanPas>
      <PageHeader
        title="Impersonasi"
        description="Bekerja sementara sebagai akun lain untuk pengecekan. Semua aksi tercatat atas nama akun asli."
        tabs={
          <ViewTabs
            layoutId="tab-impersonasi"
            label="Bagian impersonasi"
            value={tab}
            onChange={setTab}
            tabs={[
              { value: "kebijakan", label: "Kebijakan" },
              { value: "cepat", label: "Impersonate cepat" },
              { value: "riwayat", label: "Riwayat" },
            ]}
          />
        }
      />
      {tab === "kebijakan" && <TabKebijakan />}
      {tab === "cepat" && <TabCepat />}
      {tab === "riwayat" && <TabRiwayat />}
    </HalamanPas>
  );
}
