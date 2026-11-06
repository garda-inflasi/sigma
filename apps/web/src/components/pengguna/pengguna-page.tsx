"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Check, Download, KeyRound, LockOpen, LogOut, MoreHorizontal, Pencil, ShieldCheck, Trash2, UserCheck, UserPlus, UserX } from "lucide-react";
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
import { formatRelatif, formatTanggal } from "@/lib/format";
import { masihTerkunci, type PenggunaDetail } from "@/lib/api/pengguna";
import { useCan } from "@/lib/auth/session-context";
import { LencanaTerkunci } from "./lencana-terkunci";
import {
  useAssignRolePengguna, useBukaKunciPengguna, useCreatePengguna, useDeletePengguna, useKatalogAkses, usePengguna, usePenggunaBulk, useResetPasswordPengguna, useUpdatePengguna, usePenggunaDetail,
} from "@/lib/query/hooks";
import { usePaksaKeluarPengguna } from "@/lib/query/sesi-hooks";
import { LAPIS_META, ROLES, ROLE_IDS, type Role } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";
import { z } from "zod";
import { FormBff, PesanField, idGalatField } from "@/components/form/form-bff";
import type { GalatForm } from "@/lib/form/kiriman";
import { useFormBff } from "@/lib/form/use-form-bff";

const STATUS = { AKTIF: { tone: "normal", label: "Aktif" }, NONAKTIF: { tone: "netral", label: "Nonaktif" }, MENUNGGU: { tone: "waspada", label: "Menunggu aktivasi" } } as const;

/** Matriks peran × izin: transparansi least privilege untuk audit SPBE. Dibaca dari API (sumber izin tunggal). */
function MatriksAkses() {
  const { data: katalog, isLoading } = useKatalogAkses();
  if (isLoading || !katalog) return <div className="glass h-64 animate-pulse rounded-xl" aria-busy="true" />;
  const { izin, izinPeran } = katalog;
  const grup = [...new Set(izin.map((p) => p.split(".")[0]))];
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
              {izin.filter((p) => p.startsWith(`${g}.`)).map((p) => (
                <tr key={p} className="hover:bg-white/50 dark:hover:bg-white/[0.03]">
                  <td className="num sticky left-0 border-b border-border/50 bg-white/90 px-3 py-2 backdrop-blur dark:bg-navy-950/90">{p}</td>
                  {ROLE_IDS.map((r) => (
                    <td key={r} className="border-b border-border/50 text-center">
                      {izinPeran[r]?.has(p) ? <span className="inline-grid size-5 place-items-center rounded-full bg-data-500 text-white"><Check className="size-3" /></span> : <span className="inline-block size-1 rounded-full bg-mist-300 dark:bg-white/15" />}
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

/** Skema dialog pengguna; kata sandi sementara hanya diisi saat membuat akun. */
function skemaPengguna(baru: boolean) {
  return z.object({
    id: z.string().trim(),
    nama: z.string().trim().min(1, "Nama wajib diisi."),
    email: z.string().trim().pipe(z.email("Email tidak valid.")),
    password: baru ? z.string().min(8, "Kata sandi minimal 8 karakter.") : z.string(),
    peran: z.enum(ROLE_IDS),
    lembaga: z.string().trim().min(1, "Lembaga wajib diisi."),
  });
}
const skemaBuatPengguna = skemaPengguna(true);
const skemaUbahPengguna = skemaPengguna(false);
type IsianPengguna = z.input<typeof skemaBuatPengguna>;

/** Isi dialog buat/ubah pengguna. `user` null → mode buat; `PenggunaDetail` (dari server) menambah rincian akun. */
function FormPengguna({ user, gagalMuat, onOpenChange }: { user: PenggunaRow | PenggunaDetail | null; gagalMuat: boolean; onOpenChange: (v: boolean) => void }) {
  const buat = useCreatePengguna();
  const ubah = useUpdatePengguna();
  const [form, setForm] = useState<IsianPengguna>(() => ({
    id: user?.id ?? "",
    nama: user?.nama ?? "",
    email: user?.email ?? "",
    password: "",
    peran: (user?.role ?? "OPS_MAHASISWA") as Role,
    lembaga: user?.lembaga ?? "",
  }));

  // Toast gagal datang dari hook mutasi; galat juga tampil di dialog (400/422 per field bila server merincinya).
  const kiriman = useFormBff<z.output<typeof skemaBuatPengguna>, unknown, GalatForm, IsianPengguna>({
    skema: user ? skemaUbahPengguna : skemaBuatPengguna,
    kirim: (n) =>
      user
        ? ubah.mutateAsync({ id: user.id, input: { nama: n.nama, email: n.email, peran: n.peran, lembaga: n.lembaga, lapis: ROLES[n.peran].lapis } })
        : buat.mutateAsync({ id: n.id, nama: n.nama, email: n.email, password: n.password, peran: n.peran, lapis: ROLES[n.peran].lapis, lembaga: n.lembaga }),
    sukses: () => onOpenChange(false),
  });
  const sibuk = kiriman.pending;
  const g = kiriman.galatField;
  const atributGalat = (field: keyof IsianPengguna) =>
    g[field] ? { "aria-invalid": true, "aria-describedby": idGalatField(`pg-${field}`) } : {};

  return (
    <FormBff onKirim={kiriman.kirimDari(() => form)} sibuk={sibuk} className="contents">
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
              <Input id="pg-id" value={form.id} onChange={(e) => setForm({ ...form, id: e.target.value })} placeholder="USR-0001" {...atributGalat("id")} />
              <PesanField field="pg-id" pesan={g.id} />
            </div>
          )}
          <div className="grid gap-2">
            <Label htmlFor="pg-nama">Nama lengkap</Label>
            <Input id="pg-nama" value={form.nama} onChange={(e) => setForm({ ...form, nama: e.target.value })} {...atributGalat("nama")} />
            <PesanField field="pg-nama" pesan={g.nama} />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="pg-email">Email</Label>
            <Input id="pg-email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} {...atributGalat("email")} />
            <PesanField field="pg-email" pesan={g.email} />
          </div>
          {!user && (
            <div className="grid gap-2">
              <Label htmlFor="pg-pass">Kata sandi sementara</Label>
              <Input id="pg-pass" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} placeholder="Minimal 8 karakter" {...atributGalat("password")} />
              <PesanField field="pg-password" pesan={g.password} />
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
            <Input id="pg-lembaga" value={form.lembaga} onChange={(e) => setForm({ ...form, lembaga: e.target.value })} {...atributGalat("lembaga")} />
            <PesanField field="pg-lembaga" pesan={g.lembaga} />
          </div>
          {kiriman.galat && <p role="alert" className="text-sm text-destructive">{kiriman.galat.pesan}</p>}
          {user && gagalMuat && <p className="text-xs text-muted-foreground">Data terbaru gagal dimuat; formulir memakai data dari daftar.</p>}
          {user && "dibuatPada" in user && (
            <dl aria-label="Rincian akun" className="grid gap-1.5 rounded-xl bg-mist-100/70 p-3 text-sm dark:bg-white/5">
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">ID pengguna</dt><dd className="num">{user.id}</dd></div>
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Cakupan</dt><dd>{user.wilayah}</dd></div>
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Status</dt><dd>{STATUS[user.status].label}</dd></div>
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">MFA</dt><dd>{user.mfa ? "Aktif" : "Belum aktif"}</dd></div>
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Terakhir masuk</dt><dd>{user.terakhirMasuk ? formatRelatif(user.terakhirMasuk) : "Belum pernah"}</dd></div>
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Dibuat</dt><dd>{formatTanggal(user.dibuatPada)}</dd></div>
            </dl>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={sibuk}>Batal</Button>
          <SpringButton type="submit" variant="primary" size="md" disabled={sibuk}>
            {sibuk ? "Menyimpan…" : user ? "Simpan perubahan" : "Buat pengguna"}
          </SpringButton>
        </DialogFooter>
    </FormBff>
  );
}

/** Dialog buat/ubah pengguna. Mode ubah memuat data segar dari GET /v1/users/{id} sebelum formulir tampil. */
function PenggunaDialog({ user, open, onOpenChange }: { user: PenggunaRow | null; open: boolean; onOpenChange: (v: boolean) => void }) {
  const detail = usePenggunaDetail(open && user ? user.id : null);
  const siap = !user || detail.isSuccess || detail.isError;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {siap ? (
          <FormPengguna user={user ? (detail.data ?? user) : null} gagalMuat={detail.isError} onOpenChange={onOpenChange} />
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Ubah pengguna</DialogTitle>
              <DialogDescription>Memuat data pengguna…</DialogDescription>
            </DialogHeader>
            <div className="glass h-48 animate-pulse rounded-xl" aria-busy="true" />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

const skemaResetAdmin = z.object({ password: z.string().min(8, "Kata sandi minimal 8 karakter.") });

/** Dialog reset kata sandi oleh admin. */
function ResetPasswordDialog({ user, open, onOpenChange }: { user: PenggunaRow | null; open: boolean; onOpenChange: (v: boolean) => void }) {
  const reset = useResetPasswordPengguna();
  const [password, setPassword] = useState("");
  const kiriman = useFormBff<z.infer<typeof skemaResetAdmin>>({
    skema: skemaResetAdmin,
    kirim: (n) => reset.mutateAsync({ id: user!.id, password: n.password }),
    sukses: () => {
      setPassword("");
      onOpenChange(false);
    },
  });
  const galat = kiriman.galatField.password ?? kiriman.galat?.pesan;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <FormBff onKirim={kiriman.kirimDari(() => ({ password }))} sibuk={kiriman.pending} className="contents">
          <DialogHeader>
            <DialogTitle>Reset kata sandi</DialogTitle>
            <DialogDescription>
              Kata sandi baru untuk {user?.nama}. Seluruh sesi pengguna akan diakhiri dan mereka harus masuk ulang.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-2 py-2">
            <Label htmlFor="pg-new-pass">Kata sandi baru</Label>
            <Input
              id="pg-new-pass"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Minimal 8 karakter"
              aria-invalid={galat ? true : undefined}
              aria-describedby={galat ? idGalatField("pg-new-pass") : undefined}
            />
            <PesanField field="pg-new-pass" pesan={galat} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={kiriman.pending}>Batal</Button>
            <SpringButton type="submit" variant="danger" size="md" disabled={password.length < 8 || kiriman.pending}>
              {kiriman.pending ? "Mereset…" : "Reset kata sandi"}
            </SpringButton>
          </DialogFooter>
        </FormBff>
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
  const bukaKunci = useBukaKunciPengguna();
  const bolehKelola = useCan("pengguna.kelola");
  const paksaKeluar = usePaksaKeluarPengguna();

  const [dialog, setDialog] = useState<{ mode: "buat" | "ubah" | "reset"; user: PenggunaRow | null } | null>(null);

  const kolom = useMemo<ColumnDef<PenggunaRow>[]>(() => [
    // Nilai kolom memuat email juga supaya kotak cari ("Cari nama, email, …") menemukan pengguna lewat email.
    { id: "nama", accessorFn: (u) => `${u.nama} ${u.email}`, header: "Nama", cell: ({ row: { original: u } }) => <div><p className="font-medium">{u.nama}</p><p className="text-xs text-muted-foreground">{u.email}</p></div> },
    { id: "peran", accessorFn: (u) => ROLES[u.role].label, header: "Peran", cell: ({ row: { original: u } }) => <div><p>{ROLES[u.role].label}</p><p className="text-xs text-muted-foreground">{LAPIS_META[ROLES[u.role].lapis].label.split(" — ")[0]}</p></div> },
    { id: "lapis", accessorFn: (u) => ROLES[u.role].lapis, header: "Lapis", filterFn: "equals" },
    { accessorKey: "lembaga", header: "Lembaga" },
    { accessorKey: "wilayah", header: "Cakupan", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{getValue<string>()}</span> },
    { accessorKey: "mfa", header: "MFA", cell: ({ getValue }) => (getValue<boolean>() ? <ShieldCheck className="size-4 text-normal" aria-label="MFA aktif" /> : <span className="text-xs text-waspada-ink">Belum</span>) },
    { accessorKey: "terakhirMasuk", header: "Terakhir masuk", cell: ({ getValue }) => { const v = getValue<string>(); return <span className="whitespace-nowrap text-muted-foreground">{v ? formatRelatif(v) : "Belum pernah"}</span>; } },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ row: { original: u }, getValue }) => { const s = STATUS[getValue<PenggunaRow["status"]>()]; return <div className="flex flex-wrap items-center gap-1.5"><StatusBadge tone={s.tone}>{s.label}</StatusBadge><LencanaTerkunci sampai={u.terkunciSampai} /></div>; } },
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
            {bolehKelola && masihTerkunci(u.terkunciSampai) && (
              <DropdownMenuItem onSelect={() => bukaKunci.mutate({ id: u.id, nama: u.nama })}><LockOpen /> Buka kunci</DropdownMenuItem>
            )}
            <DropdownMenuItem onSelect={() => paksaKeluar.mutate({ id: u.id, nama: u.nama })}><LogOut /> Paksa keluar</DropdownMenuItem>
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
  ], [assignRole, hapus, bukaKunci, bolehKelola, paksaKeluar]);

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
      <p className={cn("text-xs text-muted-foreground", tab === "matriks" ? "block" : "hidden")}>Matriks ini dibaca dari API (GET /v1/roles dan /v1/permissions), sumber izin yang sama dengan penegakan di backend.</p>

      {dialog?.mode === "reset" ? (
        <ResetPasswordDialog user={dialog.user} open onOpenChange={(v) => !v && setDialog(null)} />
      ) : (
        <PenggunaDialog key={dialog?.user?.id ?? "baru"} user={dialog?.user ?? null} open={dialog?.mode === "buat" || dialog?.mode === "ubah"} onOpenChange={(v) => !v && setDialog(null)} />
      )}
    </div>
  );
}