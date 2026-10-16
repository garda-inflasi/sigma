"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Check, Download, KeyRound, LogOut, MoreHorizontal, Pencil, ShieldCheck, Trash2, UserCheck, UserPlus, UserX } from "lucide-react";
import { Fragment, useMemo, useState } from "react";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { Segmented } from "@/components/garda/segmented";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { exportCsv } from "@/lib/csv";
import type { PenggunaRow } from "@/lib/data/types";
import { formatRelatif } from "@/lib/format";
import {
  useAssignRolePengguna, useCreatePengguna, useDeletePengguna, usePengguna, usePenggunaBulk, useResetPasswordPengguna, useUpdatePengguna,
} from "@/lib/query/hooks";
import { PERMISSIONS, can } from "@/lib/rbac/permissions";
import { LAPIS_META, ROLES, ROLE_IDS, type Role } from "@/lib/rbac/roles";
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

/** Dialog buat/ubah pengguna. `user` null → mode buat. */
function PenggunaDialog({ user, open, onOpenChange }: { user: PenggunaRow | null; open: boolean; onOpenChange: (v: boolean) => void }) {
  const buat = useCreatePengguna();
  const ubah = useUpdatePengguna();
  const [form, setForm] = useState(() => ({
    id: user?.id ?? "",
    nama: user?.nama ?? "",
    email: user?.email ?? "",
    password: "",
    peran: (user?.role ?? "OPS_MAHASISWA") as Role,
    lembaga: user?.lembaga ?? "",
  }));

  const sibuk = buat.isPending || ubah.isPending;
  const lengkap = Boolean(form.nama && form.email && form.lembaga && (user || form.password.length >= 8));

  function simpan() {
    if (user) {
      ubah.mutate(
        { id: user.id, input: { nama: form.nama, email: form.email, peran: form.peran, lembaga: form.lembaga, lapis: ROLES[form.peran].lapis } },
        { onSuccess: () => onOpenChange(false) },
      );
    } else {
      buat.mutate(
        { id: form.id, nama: form.nama, email: form.email, password: form.password, peran: form.peran, lapis: ROLES[form.peran].lapis, lembaga: form.lembaga },
        { onSuccess: () => onOpenChange(false) },
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{user ? "Ubah pengguna" : "Tambah pengguna"}</DialogTitle>
          <DialogDescription>
            {user ? "Perubahan peran mengakhiri sesi aktif pengguna." : "Akun baru dibuat dengan kata sandi sementara; pengguna wajib menggantinya saat masuk pertama."}
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4 py-2">
          {!user && (
            <div className="grid gap-2">
              <Label htmlFor="pg-id">ID pengguna</Label>
              <Input id="pg-id" value={form.id} onChange={(e) => setForm({ ...form, id: e.target.value })} placeholder="USR-0001" />
            </div>
          )}
          <div className="grid gap-2">
            <Label htmlFor="pg-nama">Nama lengkap</Label>
            <Input id="pg-nama" value={form.nama} onChange={(e) => setForm({ ...form, nama: e.target.value })} />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="pg-email">Email</Label>
            <Input id="pg-email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
          </div>
          {!user && (
            <div className="grid gap-2">
              <Label htmlFor="pg-pass">Kata sandi sementara</Label>
              <Input id="pg-pass" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} placeholder="Minimal 8 karakter" />
            </div>
          )}
          <div className="grid gap-2">
            <Label>Peran</Label>
            <Select value={form.peran} onValueChange={(v) => setForm({ ...form, peran: v as Role })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {ROLE_IDS.map((r) => (
                  <SelectItem key={r} value={r}>{ROLES[r].label} — {LAPIS_META[ROLES[r].lapis].label.split(" — ")[0]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="pg-lembaga">Lembaga</Label>
            <Input id="pg-lembaga" value={form.lembaga} onChange={(e) => setForm({ ...form, lembaga: e.target.value })} />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={sibuk}>Batal</Button>
          <SpringButton variant="primary" size="md" onClick={simpan} disabled={!lengkap || sibuk}>
            {sibuk ? "Menyimpan…" : user ? "Simpan perubahan" : "Buat pengguna"}
          </SpringButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** Dialog reset kata sandi oleh admin. */
function ResetPasswordDialog({ user, open, onOpenChange }: { user: PenggunaRow | null; open: boolean; onOpenChange: (v: boolean) => void }) {
  const reset = useResetPasswordPengguna();
  const [password, setPassword] = useState("");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Reset kata sandi</DialogTitle>
          <DialogDescription>
            Kata sandi baru untuk {user?.nama}. Seluruh sesi pengguna akan diakhiri dan mereka harus masuk ulang.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-2 py-2">
          <Label htmlFor="pg-new-pass">Kata sandi baru</Label>
          <Input id="pg-new-pass" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Minimal 8 karakter" />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={reset.isPending}>Batal</Button>
          <SpringButton variant="danger" size="md" disabled={password.length < 8 || reset.isPending}
            onClick={() => reset.mutate({ id: user!.id, password }, { onSuccess: () => { setPassword(""); onOpenChange(false); } })}>
            {reset.isPending ? "Mereset…" : "Reset kata sandi"}
          </SpringButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function PenggunaPage() {
  const [tab, setTab] = useState<"daftar" | "matriks">("daftar");
  const { data, isLoading } = usePengguna();
  const bulkMut = usePenggunaBulk();
  const hapus = useDeletePengguna();
  const assignRole = useAssignRolePengguna();

  const [dialog, setDialog] = useState<{ mode: "buat" | "ubah" | "reset"; user: PenggunaRow | null } | null>(null);

  const kolom = useMemo<ColumnDef<PenggunaRow>[]>(() => [
    { accessorKey: "nama", header: "Nama", cell: ({ row: { original: u } }) => <div><p className="font-medium">{u.nama}</p><p className="text-xs text-muted-foreground">{u.email}</p></div> },
    { id: "peran", accessorFn: (u) => ROLES[u.role].label, header: "Peran", cell: ({ row: { original: u } }) => <div><p>{ROLES[u.role].label}</p><p className="text-xs text-muted-foreground">{LAPIS_META[ROLES[u.role].lapis].label.split(" — ")[0]}</p></div> },
    { id: "lapis", accessorFn: (u) => ROLES[u.role].lapis, header: "Lapis", filterFn: "equals" },
    { accessorKey: "lembaga", header: "Lembaga" },
    { accessorKey: "wilayah", header: "Cakupan", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{getValue<string>()}</span> },
    { accessorKey: "mfa", header: "MFA", cell: ({ getValue }) => (getValue<boolean>() ? <ShieldCheck className="size-4 text-normal" aria-label="MFA aktif" /> : <span className="text-xs text-waspada-ink">Belum</span>) },
    { accessorKey: "terakhirMasuk", header: "Terakhir masuk", cell: ({ getValue }) => { const v = getValue<string>(); return <span className="whitespace-nowrap text-muted-foreground">{v ? formatRelatif(v) : "Belum pernah"}</span>; } },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ getValue }) => { const s = STATUS[getValue<PenggunaRow["status"]>()]; return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>; } },
    {
      id: "_aksi", header: "", enableSorting: false, size: 48,
      cell: ({ row: { original: u } }) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-sm" aria-label={`Aksi untuk ${u.nama}`}><MoreHorizontal /></Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuItem onSelect={() => setDialog({ mode: "ubah", user: u })}><Pencil /> Ubah data</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setDialog({ mode: "reset", user: u })}><KeyRound /> Reset kata sandi</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => assignRole.mutate({ id: u.id, peran: u.role === "ADMIN_SISTEM" ? "OPS_MAHASISWA" : "ADMIN_SISTEM" })}>
              <ShieldCheck /> {u.role === "ADMIN_SISTEM" ? "Cabut admin sistem" : "Jadikan admin sistem"}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => hapus.mutate(u.id)}><Trash2 /> Hapus pengguna</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ], [assignRole, hapus]);

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Pengguna & peran" description="RBAC tiga lapis dengan SSO, MFA, dan prinsip least privilege. Perubahan akun tercatat di audit trail."
        actions={<Segmented layoutId="pengguna-tab" size="sm" value={tab} onChange={setTab} options={[{ value: "daftar", label: "Daftar pengguna" }, { value: "matriks", label: "Matriks akses" }]} />} />
      {tab === "matriks" ? <MatriksAkses /> : (
        <DataTable data={data} isLoading={isLoading} columns={kolom} getRowId={(r) => r.id} entityLabel="pengguna" hiddenColumns={["lapis"]} searchPlaceholder="Cari nama, email, peran, atau lembaga"
          toolbar={
            <SpringButton variant="primary" size="sm" onClick={() => setDialog({ mode: "buat", user: null })}>
              <UserPlus /> Tambah pengguna
            </SpringButton>
          }
          facets={[
            { columnId: "lapis", label: "Lapis", options: Object.entries(LAPIS_META).map(([value, v]) => ({ value, label: v.label.split(" — ")[0] })) },
            { columnId: "status", label: "Status", options: Object.entries(STATUS).map(([value, v]) => ({ value, label: v.label })) },
          ]}
          bulkActions={[
            { id: "aktifkan", label: "Aktifkan", icon: UserCheck, perm: "pengguna.kelola", tone: "data", enabled: (r) => r.every((u) => u.status !== "AKTIF"), run: (r) => bulkMut.mutateAsync({ aksi: "aktifkan", ids: r.map((u) => u.id), label: "Aktifkan" }) },
            { id: "nonaktifkan", label: "Nonaktifkan", icon: UserX, perm: "pengguna.kelola", tone: "danger", enabled: (r) => r.every((u) => u.status === "AKTIF"), confirm: { title: "Nonaktifkan akun terpilih?", description: (n) => `${n} akun tidak dapat masuk dan seluruh sesinya diakhiri. Data yang pernah diinput tetap tersimpan.`, actionLabel: "Nonaktifkan" }, run: (r) => bulkMut.mutateAsync({ aksi: "nonaktifkan", ids: r.map((u) => u.id), label: "Nonaktifkan" }) },
            { id: "mfa", label: "Wajibkan MFA", icon: KeyRound, perm: "pengguna.kelola", run: (r) => bulkMut.mutateAsync({ aksi: "wajibkan_mfa", ids: r.map((u) => u.id), label: "Wajibkan MFA" }) },
            { id: "sesi", label: "Akhiri sesi", icon: LogOut, perm: "pengguna.kelola", run: (r) => bulkMut.mutateAsync({ aksi: "akhiri_sesi", ids: r.map((u) => u.id), label: "Akhiri sesi" }) },
            { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Nama", value: (u) => u.nama }, { header: "Email", value: (u) => u.email }, { header: "Peran", value: (u) => ROLES[u.role].label }, { header: "Lembaga", value: (u) => u.lembaga }, { header: "Status", value: (u) => u.status }], "pengguna") },
          ]} />
      )}
      <p className={cn("text-xs text-muted-foreground", tab === "matriks" ? "block" : "hidden")}>Matriks ini dibaca langsung dari lib/rbac/permissions.ts, sumber yang sama dengan proxy.ts dan API.</p>

      {dialog?.mode === "reset" ? (
        <ResetPasswordDialog user={dialog.user} open onOpenChange={(v) => !v && setDialog(null)} />
      ) : (
        <PenggunaDialog key={dialog?.user?.id ?? "baru"} user={dialog?.user ?? null} open={dialog?.mode === "buat" || dialog?.mode === "ubah"} onOpenChange={(v) => !v && setDialog(null)} />
      )}
    </div>
  );
}