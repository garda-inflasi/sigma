"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Check, Download, KeyRound, LogOut, ShieldCheck, UserCheck, UserX } from "lucide-react";
import { Fragment, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { Segmented } from "@/components/garda/segmented";
import { StatusBadge } from "@/components/garda/status-badge";
import { exportCsv } from "@/lib/csv";
import type { PenggunaRow } from "@/lib/data/types";
import { formatRelatif } from "@/lib/format";
import { usePengguna } from "@/lib/query/hooks";
import { PERMISSIONS, can } from "@/lib/rbac/permissions";
import { LAPIS_META, ROLES, ROLE_IDS } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";

const STATUS = { AKTIF: { tone: "normal", label: "Aktif" }, NONAKTIF: { tone: "netral", label: "Nonaktif" }, MENUNGGU: { tone: "waspada", label: "Menunggu aktivasi" } } as const;

/** Matriks peran × izin: transparansi least privilege untuk audit SPBE. */
function MatriksAkses() {
  const grup = [...new Set(PERMISSIONS.map((p) => p.split(".")[0]))];
  return (
    <div className="glass scrollbar-thin overflow-x-auto rounded-xl">
      <table className="w-full min-w-[1100px] border-separate border-spacing-0 text-xs">
        <thead className="sticky top-0 z-10">
          <tr>
            <th className="sticky left-0 z-20 bg-white/95 p-3 text-left font-medium text-muted-foreground backdrop-blur dark:bg-navy-950/95">Izin</th>
            {ROLE_IDS.map((r) => (
              <th key={r} className="bg-white/90 p-2 align-bottom font-medium backdrop-blur dark:bg-navy-950/90">
                <span className="block w-20 text-[11px] leading-tight">{ROLES[r].label}</span>
                <span className="mt-1 block text-[10px] font-normal text-muted-foreground">{LAPIS_META[ROLES[r].lapis].label.split(" ")[1]}</span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {grup.map((g) => (
            <Fragment key={g}>
              <tr><td colSpan={ROLE_IDS.length + 1} className="sticky left-0 bg-mist-100/80 px-3 py-1.5 font-semibold capitalize dark:bg-white/5">{g}</td></tr>
              {PERMISSIONS.filter((p) => p.startsWith(`${g}.`)).map((p) => (
                <tr key={p} className="hover:bg-white/50 dark:hover:bg-white/[0.03]">
                  <td className="num sticky left-0 border-b border-border/50 bg-white/90 px-3 py-2 backdrop-blur dark:bg-navy-950/90">{p}</td>
                  {ROLE_IDS.map((r) => (
                    <td key={r} className="border-b border-border/50 text-center">
                      {can(r, p) ? <span className="inline-grid size-5 place-items-center rounded-full bg-data-500 text-white"><Check className="size-3" /></span> : <span className="inline-block size-1 rounded-full bg-mist-300 dark:bg-white/15" />}
                    </td>
                  ))}
                </tr>
              ))}
            </Fragment>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function PenggunaPage() {
  const [tab, setTab] = useState<"daftar" | "matriks">("daftar");
  const { data, isLoading } = usePengguna();
  const [rows, setRows] = useState<PenggunaRow[]>([]);
  useEffect(() => { if (data) setRows(data); }, [data]);
  const ubah = (ids: string[], patch: Partial<PenggunaRow>) => { const s = new Set(ids); setRows((r) => r.map((x) => (s.has(x.id) ? { ...x, ...patch } : x))); };

  const kolom = useMemo<ColumnDef<PenggunaRow>[]>(() => [
    { accessorKey: "nama", header: "Nama", cell: ({ row: { original: u } }) => <div><p className="font-medium">{u.nama}</p><p className="text-xs text-muted-foreground">{u.email}</p></div> },
    { id: "peran", accessorFn: (u) => ROLES[u.role].label, header: "Peran", cell: ({ row: { original: u } }) => <div><p>{ROLES[u.role].label}</p><p className="text-xs text-muted-foreground">{LAPIS_META[ROLES[u.role].lapis].label.split(" — ")[0]}</p></div> },
    { id: "lapis", accessorFn: (u) => ROLES[u.role].lapis, header: "Lapis", filterFn: "equals" },
    { accessorKey: "lembaga", header: "Lembaga" },
    { accessorKey: "wilayah", header: "Cakupan", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{getValue<string>()}</span> },
    { accessorKey: "mfa", header: "MFA", cell: ({ getValue }) => (getValue<boolean>() ? <ShieldCheck className="size-4 text-normal" aria-label="MFA aktif" /> : <span className="text-xs text-waspada-ink">Belum</span>) },
    { accessorKey: "terakhirMasuk", header: "Terakhir masuk", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{formatRelatif(getValue<string>())}</span> },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ getValue }) => { const s = STATUS[getValue<PenggunaRow["status"]>()]; return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>; } },
  ], []);

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Pengguna & peran" description="RBAC tiga lapis dengan SSO, MFA, dan prinsip least privilege. Perubahan akun tercatat di audit trail."
        actions={<Segmented layoutId="pengguna-tab" size="sm" value={tab} onChange={setTab} options={[{ value: "daftar", label: "Daftar pengguna" }, { value: "matriks", label: "Matriks akses" }]} />} />
      {tab === "matriks" ? <MatriksAkses /> : (
        <DataTable data={rows} isLoading={isLoading} columns={kolom} getRowId={(r) => r.id} entityLabel="pengguna" hiddenColumns={["lapis"]} searchPlaceholder="Cari nama, email, peran, atau lembaga"
          facets={[
            { columnId: "lapis", label: "Lapis", options: Object.entries(LAPIS_META).map(([value, v]) => ({ value, label: v.label.split(" — ")[0] })) },
            { columnId: "status", label: "Status", options: Object.entries(STATUS).map(([value, v]) => ({ value, label: v.label })) },
          ]}
          bulkActions={[
            { id: "aktifkan", label: "Aktifkan", icon: UserCheck, perm: "pengguna.kelola", tone: "data", enabled: (r) => r.every((u) => u.status !== "AKTIF"), run: (r) => { ubah(r.map((u) => u.id), { status: "AKTIF" }); toast.success(`${r.length} akun diaktifkan`); } },
            { id: "nonaktifkan", label: "Nonaktifkan", icon: UserX, perm: "pengguna.kelola", tone: "danger", enabled: (r) => r.every((u) => u.status === "AKTIF"), confirm: { title: "Nonaktifkan akun terpilih?", description: (n) => `${n} akun tidak dapat masuk dan seluruh sesinya diakhiri. Data yang pernah diinput tetap tersimpan.`, actionLabel: "Nonaktifkan" }, run: (r) => { ubah(r.map((u) => u.id), { status: "NONAKTIF" }); toast.success(`${r.length} akun dinonaktifkan`); } },
            { id: "mfa", label: "Wajibkan MFA", icon: KeyRound, perm: "pengguna.kelola", run: (r) => { ubah(r.map((u) => u.id), { mfa: true }); toast.success(`MFA diwajibkan untuk ${r.length} akun`, { description: "Berlaku pada login berikutnya." }); } },
            { id: "sesi", label: "Akhiri sesi", icon: LogOut, perm: "pengguna.kelola", run: (r) => { toast.success(`Sesi ${r.length} akun diakhiri`); } },
            { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Nama", value: (u) => u.nama }, { header: "Email", value: (u) => u.email }, { header: "Peran", value: (u) => ROLES[u.role].label }, { header: "Lembaga", value: (u) => u.lembaga }, { header: "Status", value: (u) => u.status }], "pengguna") },
          ]} />
      )}
      <p className={cn("text-xs text-muted-foreground", tab === "matriks" ? "block" : "hidden")}>Matriks ini dibaca langsung dari lib/rbac/permissions.ts, sumber yang sama dengan proxy.ts dan API.</p>
    </div>
  );
}
