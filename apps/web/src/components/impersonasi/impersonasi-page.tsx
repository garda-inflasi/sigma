"use client";

import { ShieldX } from "lucide-react";
import { EmptyState } from "@/components/garda/empty-state";
import { PageHeader } from "@/components/garda/page-header";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useCan } from "@/lib/auth/session-context";
import { TabCepat } from "./tab-cepat";
import { TabKebijakan } from "./tab-kebijakan";
import { TabRiwayat } from "./tab-riwayat";

/**
 * Halaman admin impersonasi (khusus superadmin): kebijakan, impersonate cepat, dan riwayat. Penjagaan berlapis:
 * edge (`ROUTE_ACCESS`), pemeriksaan izin sesi di sini, dan API Go (peran ADMIN_SISTEM + izin `impersonasi.kelola`,
 * 403 untuk selain itu; panel galat tab menampilkannya).
 */
export function ImpersonasiPage() {
  const boleh = useCan("impersonasi.kelola");

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
    <div className="grid gap-5">
      <PageHeader
        title="Impersonasi"
        description="Atur siapa yang boleh bekerja sementara sebagai peran atau akun lain untuk pengecekan, lihat riwayatnya, dan mulai impersonasi. Semua aksi tercatat atas nama akun asli di audit trail."
      />
      <Tabs defaultValue="kebijakan">
        <TabsList>
          <TabsTrigger value="kebijakan">Kebijakan</TabsTrigger>
          <TabsTrigger value="cepat">Impersonate cepat</TabsTrigger>
          <TabsTrigger value="riwayat">Riwayat</TabsTrigger>
        </TabsList>
        <TabsContent value="kebijakan" className="pt-2">
          <TabKebijakan />
        </TabsContent>
        <TabsContent value="cepat" className="pt-2">
          <TabCepat />
        </TabsContent>
        <TabsContent value="riwayat" className="pt-2">
          <TabRiwayat />
        </TabsContent>
      </Tabs>
    </div>
  );
}
