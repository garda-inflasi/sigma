"use client";

import type { ColumnDef } from "@tanstack/react-table";
import {
  Check,
  Download,
  KeyRound,
  LockOpen,
  LogOut,
  MoreHorizontal,
  Pencil,
  ShieldCheck,
  ShieldOff,
  Trash2,
  UserCheck,
  UserPlus,
  UserX,
} from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import {
  HalamanPas,
  IsiPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { MetaItem } from "@/components/garda/inspector";
import { PageHeader } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { ViewTabs } from "@/components/garda/view-tabs";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { PenggunaRow } from "@/lib/data/types";
import { formatRelatif, formatTanggal } from "@/lib/format";
import { masihTerkunci, type PenggunaDetail } from "@/lib/api/pengguna";
import { useCan, useOptionalSession } from "@/lib/auth/session-context";
import { FilterPengguna } from "./filter-pengguna";
import { LencanaTerkunci } from "./lencana-terkunci";
import { ResetMfaDialog } from "./reset-mfa-dialog";
import { RiwayatPeran } from "./riwayat-peran";
import { AksiUndangan, ItemKirimUlangUndangan } from "./aksi-undangan";
import { useEksporPengguna } from "@/lib/query/undangan-hooks";
import { MatriksAksesEditor } from "./matriks-akses";
import {
  IzinLangsungPanel,
  PilihBagianPengguna,
  type BagianDialogPengguna,
} from "./izin-langsung";
import { MODE_MOCK } from "@/lib/api/client";
import {
  adalahKolomUrut,
  jumlahFilterAktif,
  koreksiHalaman,
  kunciCachePengguna,
  parseStateTabel,
  queryUrlPengguna,
  resetFilter,
  toSortingState,
  ubahCari,
  ubahFilter,
  ubahHalaman,
  ubahUkuran,
  ubahUrut,
  type StateTabelPengguna,
} from "@/lib/pengguna/tabel";
import {
  useAssignRolePengguna,
  useBukaKunciPengguna,
  useCreatePengguna,
  useDeletePengguna,
  useKatalogAkses,
  usePenggunaHalaman,
  usePenggunaBulk,
  useResetPasswordPengguna,
  useUpdatePengguna,
  usePenggunaDetail,
} from "@/lib/query/hooks";
import { usePaksaKeluarPengguna } from "@/lib/query/sesi-hooks";
import { LAPIS_META, ROLES, ROLE_IDS, type Role } from "@/lib/rbac/roles";
import { z } from "zod";
import {
  FormDialog,
  PesanField,
  idGalatField,
} from "@/components/form/form-bff";
import { FormAlert, FormField } from "@/components/form/form-field";
import type { GalatForm } from "@/lib/form/kiriman";
import { useFormBff } from "@/lib/form/use-form-bff";

const STATUS = {
  AKTIF: { tone: "normal", label: "Aktif" },
  NONAKTIF: { tone: "netral", label: "Nonaktif" },
  MENUNGGU: { tone: "waspada", label: "Menunggu aktivasi" },
} as const;

/** Matriks peran × izin: transparansi least privilege untuk audit SPBE. Dibaca dari API (sumber izin tunggal). */
function MatriksAkses() {
  const { data: katalog, isLoading } = useKatalogAkses();
  if (isLoading || !katalog)
    return (
      <div
        className="h-64 animate-pulse rounded-lg bg-secondary"
        aria-busy="true"
      />
    );
  const { izin, izinPeran } = katalog;
  const grup = [...new Set(izin.map((p) => p.split(".")[0]))];
  return (
    <div className="scrollbar-thin max-h-[75dvh] overflow-auto rounded-lg border border-border bg-card lg:max-h-none lg:min-h-0 lg:flex-1">
      <table className="w-full min-w-[1100px] border-separate border-spacing-0 text-xs">
        <thead className="sticky top-0 z-10">
          <tr>
            <th className="sticky left-0 z-20 border-b border-border bg-secondary p-3 text-left font-medium text-muted-foreground">
              Izin
            </th>
            {ROLE_IDS.map((r) => (
              <th
                key={r}
                className="border-b border-border bg-secondary p-2 align-bottom font-medium"
              >
                <span className="block w-20 text-[11px] leading-tight">
                  {ROLES[r].label}
                </span>
                <span className="mt-1 block text-[10px] font-normal text-muted-foreground">
                  {LAPIS_META[ROLES[r].lapis].label.split(" ")[1]}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {grup.map((g) => (
            <Fragment key={g}>
              <tr>
                <td
                  colSpan={ROLE_IDS.length + 1}
                  className="sticky left-0 border-b border-border bg-secondary px-3 py-1.5 font-semibold capitalize"
                >
                  {g}
                </td>
              </tr>
              {izin
                .filter((p) => p.startsWith(`${g}.`))
                .map((p) => (
                  <tr key={p} className="group hover:bg-secondary/60">
                    <td className="num sticky left-0 border-b border-border bg-card px-3 py-2 group-hover:bg-secondary">
                      {p}
                    </td>
                    {ROLE_IDS.map((r) => (
                      <td
                        key={r}
                        className="border-b border-border text-center"
                      >
                        {izinPeran[r]?.has(p) ? (
                          <span className="inline-grid size-5 place-items-center rounded-full bg-data-500 text-white">
                            <Check className="size-3" />
                          </span>
                        ) : (
                          <span className="inline-block size-1 rounded-full bg-mist-300 dark:bg-white/15" />
                        )}
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
    nama: z.string().trim().min(1, "Nama wajib diisi."),
    email: z.string().trim().pipe(z.email("Email tidak valid.")),
    password: baru
      ? z.string().min(8, "Kata sandi minimal 8 karakter.")
      : z.string(),
    peran: z.enum(ROLE_IDS),
    lembaga: z.string().trim().min(1, "Lembaga wajib diisi."),
  });
}
const skemaBuatPengguna = skemaPengguna(true);
const skemaUbahPengguna = skemaPengguna(false);
type IsianPengguna = z.input<typeof skemaBuatPengguna>;

/** Isi dialog buat/ubah pengguna. `user` null → mode buat; `PenggunaDetail` (dari server) menambah rincian akun. */
function FormPengguna({
  user,
  gagalMuat,
  onOpenChange,
}: {
  user: PenggunaRow | PenggunaDetail | null;
  gagalMuat: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const buat = useCreatePengguna();
  const ubah = useUpdatePengguna();
  const [form, setForm] = useState<IsianPengguna>(() => ({
    nama: user?.nama ?? "",
    email: user?.email ?? "",
    password: "",
    peran: (user?.role ?? "OPS_MAHASISWA") as Role,
    lembaga: user?.lembaga ?? "",
  }));

  // Toast gagal datang dari hook mutasi; galat juga tampil di dialog (400/422 per field bila server merincinya).
  const kiriman = useFormBff<
    z.output<typeof skemaBuatPengguna>,
    unknown,
    GalatForm,
    IsianPengguna
  >({
    skema: user ? skemaUbahPengguna : skemaBuatPengguna,
    kirim: (n) =>
      user
        ? ubah.mutateAsync({
            id: user.id,
            input: {
              nama: n.nama,
              email: n.email,
              peran: n.peran,
              lembaga: n.lembaga,
              lapis: ROLES[n.peran].lapis,
            },
          })
        : buat.mutateAsync({
            nama: n.nama,
            email: n.email,
            password: n.password,
            peran: n.peran,
            lapis: ROLES[n.peran].lapis,
            lembaga: n.lembaga,
          }),
    sukses: () => onOpenChange(false),
  });
  const sibuk = kiriman.pending;
  const g = kiriman.galatField;
  const atributGalat = (field: keyof IsianPengguna) =>
    g[field]
      ? {
          "aria-invalid": true,
          "aria-describedby": idGalatField(`pg-${field}`),
        }
      : {};

  return (
    <FormDialog onKirim={kiriman.kirimDari(() => form)} sibuk={sibuk}>
      <DialogHeader>
        <DialogTitle>{user ? "Ubah pengguna" : "Tambah pengguna"}</DialogTitle>
        <DialogDescription>
          {user
            ? "Perubahan peran mengakhiri sesi aktif pengguna."
            : "Akun baru dibuat dengan kata sandi sementara; pengguna wajib menggantinya saat masuk pertama."}
        </DialogDescription>
      </DialogHeader>

      <DialogBody>
        <FormField id="pg-nama" label="Nama lengkap" galat={g.nama}>
          <Input
            id="pg-nama"
            value={form.nama}
            onChange={(e) => setForm({ ...form, nama: e.target.value })}
            {...atributGalat("nama")}
          />
        </FormField>
        <FormField id="pg-email" label="Email" galat={g.email}>
          <Input
            id="pg-email"
            type="email"
            value={form.email}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
            {...atributGalat("email")}
          />
        </FormField>
        {!user && (
          <FormField id="pg-pass" label="Kata sandi sementara">
            <Input
              id="pg-pass"
              type="password"
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              placeholder="Minimal 8 karakter"
              {...atributGalat("password")}
            />
            <PesanField field="pg-password" pesan={g.password} />
          </FormField>
        )}
        <FormField id="pg-peran" label="Peran">
          <Select
            value={form.peran}
            onValueChange={(v) => setForm({ ...form, peran: v as Role })}
          >
            <SelectTrigger id="pg-peran">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLE_IDS.map((r) => (
                <SelectItem key={r} value={r}>
                  {ROLES[r].label} —{" "}
                  {LAPIS_META[ROLES[r].lapis].label.split(" — ")[0]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormField>
        <FormField id="pg-lembaga" label="Lembaga" galat={g.lembaga}>
          <Input
            id="pg-lembaga"
            value={form.lembaga}
            onChange={(e) => setForm({ ...form, lembaga: e.target.value })}
            {...atributGalat("lembaga")}
          />
        </FormField>
        <FormAlert>{kiriman.galat?.pesan}</FormAlert>
        {user && gagalMuat && (
          <p className="text-xs text-muted-foreground">
            Data terbaru gagal dimuat; formulir memakai data dari daftar.
          </p>
        )}
        {user && "dibuatPada" in user && user.dibuatPada && (
          <dl
            aria-label="Rincian akun"
            className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-lg bg-secondary p-3"
          >
            <MetaItem label="Status">
              <StatusBadge tone={STATUS[user.status].tone}>
                {STATUS[user.status].label}
              </StatusBadge>
            </MetaItem>
            <MetaItem label="MFA">
              {user.mfa ? "Aktif" : "Belum aktif"}
            </MetaItem>
            <MetaItem label="Terakhir masuk">
              <span className="num">
                {user.terakhirMasuk
                  ? formatRelatif(user.terakhirMasuk)
                  : "Belum pernah"}
              </span>
            </MetaItem>
            <MetaItem label="Dibuat">
              <span className="num">{formatTanggal(user.dibuatPada)}</span>
            </MetaItem>
            <MetaItem label="Cakupan" wide>
              {user.wilayah}
            </MetaItem>
            <MetaItem label="ID pengguna" wide>
              <span className="num break-all">{user.id}</span>
            </MetaItem>
          </dl>
        )}
        {user && <RiwayatPeran userId={user.id} />}
      </DialogBody>

      <DialogFooter>
        <Button
          variant="outline"
          onClick={() => onOpenChange(false)}
          disabled={sibuk}
        >
          Batal
        </Button>
        <SpringButton
          type="submit"
          variant="primary"
          size="md"
          disabled={sibuk}
        >
          {sibuk ? "Menyimpan…" : user ? "Simpan perubahan" : "Buat pengguna"}
        </SpringButton>
      </DialogFooter>
    </FormDialog>
  );
}

/** Dialog buat/ubah pengguna. Mode ubah memuat data segar dari GET /v1/users/{id} sebelum formulir tampil. */
function PenggunaDialog({
  user,
  open,
  onOpenChange,
}: {
  user: PenggunaRow | null;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const detail = usePenggunaDetail(open && user ? user.id : null);
  const siap = !user || detail.isSuccess || detail.isError;
  const [bagian, setBagian] = useState<BagianDialogPengguna>("akun");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {siap ? (
          <>
            {user && <PilihBagianPengguna nilai={bagian} onUbah={setBagian} />}
            {/* Formulir akun tetap terpasang (disembunyikan) saat melihat izin langsung: isiannya tidak hilang. */}
            <div className={user && bagian === "izin" ? "hidden" : "contents"}>
              <FormPengguna
                user={user ? (detail.data ?? user) : null}
                gagalMuat={detail.isError}
                onOpenChange={onOpenChange}
              />
            </div>
            {user && bagian === "izin" && (
              <IzinLangsungPanel
                user={user}
                onTutup={() => onOpenChange(false)}
              />
            )}
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Ubah pengguna</DialogTitle>
              <DialogDescription>Memuat data pengguna…</DialogDescription>
            </DialogHeader>
            <div
              className="h-48 animate-pulse rounded-lg bg-secondary"
              aria-busy="true"
            />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

const skemaResetAdmin = z.object({
  password: z.string().min(8, "Kata sandi minimal 8 karakter."),
});

/** Dialog reset kata sandi oleh admin. */
function ResetPasswordDialog({
  user,
  open,
  onOpenChange,
}: {
  user: PenggunaRow | null;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
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
        <FormDialog
          onKirim={kiriman.kirimDari(() => ({ password }))}
          sibuk={kiriman.pending}
        >
          <DialogHeader>
            <DialogTitle>Reset kata sandi</DialogTitle>
            <DialogDescription>
              Kata sandi baru untuk {user?.nama} ({user?.email}). Seluruh sesi
              akun ini diakhiri di semua perangkat dan pemiliknya harus masuk
              ulang dengan kata sandi baru.
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <FormField id="pg-new-pass" label="Kata sandi baru" galat={galat}>
              <Input
                id="pg-new-pass"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Minimal 8 karakter"
                aria-invalid={galat ? true : undefined}
                aria-describedby={
                  galat ? idGalatField("pg-new-pass") : undefined
                }
              />
            </FormField>
          </DialogBody>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={kiriman.pending}
            >
              Batal
            </Button>
            <SpringButton
              type="submit"
              variant="danger"
              size="md"
              disabled={password.length < 8 || kiriman.pending}
            >
              {kiriman.pending ? "Mereset…" : "Reset kata sandi"}
            </SpringButton>
          </DialogFooter>
        </FormDialog>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Menu aksi satu baris. Tiap aksi tetap berjalan seperti semula; panel bawah menyebut akun terdampak dan akibat aksi
 * yang sedang disorot, karena sebagian aksi langsung berjalan tanpa dialog konfirmasi.
 */
function MenuAksiPengguna({
  user: u,
  bolehKelola,
  idSendiri,
  onDialog,
  onBukaKunci,
  onPaksaKeluar,
  onUbahAdmin,
  onHapus,
}: {
  user: PenggunaRow;
  bolehKelola: boolean;
  idSendiri?: string;
  onDialog: (mode: "ubah" | "reset" | "mfa") => void;
  onBukaKunci: () => void;
  onPaksaKeluar: () => void;
  onUbahAdmin: () => void;
  onHapus: () => void;
}) {
  const [sorot, setSorot] = useState<string>();
  const dampak = (teks: string) => ({
    onFocus: () => setSorot(teks),
    onBlur: () => setSorot(undefined),
  });
  const adalahAdmin = u.role === "ADMIN_SISTEM";

  return (
    <DropdownMenu onOpenChange={(buka) => !buka && setSorot(undefined)}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`Aksi untuk ${u.nama}`}
        >
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuLabel className="min-w-0 font-normal">
          <span className="block truncate text-[13px] font-medium text-foreground">
            {u.nama}
          </span>
          <span className="block truncate text-xs text-muted-foreground">
            {u.email}
          </span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => onDialog("ubah")}>
          <Pencil /> Ubah data
        </DropdownMenuItem>
        {u.status === "MENUNGGU" ? (
          <ItemKirimUlangUndangan user={u} />
        ) : (
          <DropdownMenuItem
            onSelect={() => onDialog("reset")}
            {...dampak(
              `Membuka dialog kata sandi baru untuk ${u.nama}. Setelah disimpan, semua sesinya berakhir.`,
            )}
          >
            <KeyRound /> Reset kata sandi
          </DropdownMenuItem>
        )}
        {bolehKelola && masihTerkunci(u.terkunciSampai) && (
          <DropdownMenuItem
            onSelect={onBukaKunci}
            {...dampak(`Langsung membuka kunci masuk ${u.nama}.`)}
          >
            <LockOpen /> Buka kunci
          </DropdownMenuItem>
        )}
        <DropdownMenuItem
          onSelect={onPaksaKeluar}
          {...dampak(
            `Langsung mengakhiri semua sesi ${u.nama} di semua perangkat. Pemilik akun harus masuk lagi.`,
          )}
        >
          <LogOut /> Paksa keluar
        </DropdownMenuItem>
        {bolehKelola && u.mfa && u.id !== idSendiri && (
          <DropdownMenuItem
            onSelect={() => onDialog("mfa")}
            {...dampak(
              `Membuka konfirmasi untuk melepas autentikator ${u.nama} dan mengakhiri semua sesinya.`,
            )}
          >
            <ShieldOff /> Reset MFA
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onSelect={onUbahAdmin}
          {...dampak(
            adalahAdmin
              ? `Langsung mengubah ${u.nama} menjadi ${ROLES.OPS_MAHASISWA.label} dan mengakhiri semua sesinya.`
              : `Langsung memberi ${u.nama} akses penuh ${ROLES.ADMIN_SISTEM.label} dan mengakhiri semua sesinya.`,
          )}
        >
          <ShieldCheck />{" "}
          {adalahAdmin ? "Cabut admin sistem" : "Jadikan admin sistem"}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          onSelect={onHapus}
          {...dampak(
            `Langsung menghapus akun ${u.nama} secara permanen beserta sesi dan izin langsungnya. Ditolak bila akun masih menjadi asal data; nonaktifkan saja.`,
          )}
        >
          <Trash2 /> Hapus pengguna
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <p
          aria-live="polite"
          className="min-h-[4.25rem] px-2 py-1.5 text-xs leading-snug text-muted-foreground"
        >
          {sorot ??
            "Paksa keluar, ubah admin, dan hapus berjalan seketika tanpa konfirmasi. Sorot aksi untuk melihat dampaknya."}
        </p>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function PenggunaPage() {
  // Tab ikut URL lewat History API, sama seperti keadaan tabel, supaya berpindah tab tidak memuat ulang data server.
  const searchParams = useSearchParams();
  const tab = searchParams.get("tab") === "matriks" ? "matriks" : "daftar";
  const setTab = useCallback((nilai: "daftar" | "matriks") => {
    const q = new URLSearchParams(window.location.search);
    if (nilai === "daftar") q.delete("tab");
    else q.set("tab", nilai);
    const qs = q.toString();
    window.history.replaceState(
      null,
      "",
      `${window.location.pathname}${qs ? `?${qs}` : ""}`,
    );
  }, []);
  // Keadaan tabel hidup di URL (pencarian, filter, urutan, halaman): muat ulang dan tombol kembali memulihkannya.
  const tabelState = useMemo(
    () => parseStateTabel(searchParams),
    [searchParams],
  );
  const { data, isLoading, isPlaceholderData } = usePenggunaHalaman(tabelState);
  const idSendiri = useOptionalSession()?.id;
  const bulkMut = usePenggunaBulk();
  const hapus = useDeletePengguna();
  const assignRole = useAssignRolePengguna();
  const bukaKunci = useBukaKunciPengguna();
  const bolehKelola = useCan("pengguna.kelola");
  const paksaKeluar = usePaksaKeluarPengguna();
  const eksporTerpilih = useEksporPengguna();

  const [dialog, setDialog] = useState<{
    mode: "buat" | "ubah" | "reset" | "mfa";
    user: PenggunaRow | null;
  } | null>(null);

  // push untuk perubahan yang layak jadi langkah "kembali" (halaman, urutan, filter); replace untuk pencarian.
  const ubahTabel = useCallback(
    (berikut: StateTabelPengguna, mode: "push" | "replace" = "push") => {
      const qs = queryUrlPengguna(berikut);
      if (
        qs ===
        queryUrlPengguna(
          parseStateTabel(new URLSearchParams(window.location.search)),
        )
      )
        return;
      const q = new URLSearchParams(qs);
      const tabUrl = new URLSearchParams(window.location.search).get("tab");
      if (tabUrl) q.set("tab", tabUrl);
      const tujuan = q.toString();
      window.history[mode === "push" ? "pushState" : "replaceState"](
        null,
        "",
        `${window.location.pathname}${tujuan ? `?${tujuan}` : ""}`,
      );
    },
    [],
  );

  // Halaman di luar jangkauan (baris terakhir dihapus, URL lama): pindah ke halaman terakhir yang ada.
  const total = data?.total;
  useEffect(() => {
    if (isPlaceholderData || total === undefined) return;
    const halaman = koreksiHalaman(tabelState, total);
    if (halaman !== null)
      ubahTabel(ubahHalaman(tabelState, halaman), "replace");
  }, [isPlaceholderData, total, tabelState, ubahTabel]);

  const kolom = useMemo<ColumnDef<PenggunaRow>[]>(
    () => [
      // Nilai kolom memuat email juga supaya kotak cari ("Cari nama, email, …") menemukan pengguna lewat email.
      {
        id: "nama",
        accessorFn: (u) => `${u.nama} ${u.email}`,
        header: "Nama",
        cell: ({ row: { original: u } }) => (
          <div className="min-w-0">
            <SelTeks className="max-w-[11rem] font-medium">{u.nama}</SelTeks>
            <SelTeks className="max-w-[11rem] text-xs text-muted-foreground">
              {u.email}
            </SelTeks>
          </div>
        ),
      },
      {
        id: "peran",
        accessorFn: (u) => ROLES[u.role].label,
        header: "Peran",
        cell: ({ row: { original: u } }) => (
          <div className="w-36 max-w-full">
            <p className="truncate" title={ROLES[u.role].label}>
              {ROLES[u.role].label}
            </p>
            <p className="truncate text-xs text-muted-foreground">
              {LAPIS_META[ROLES[u.role].lapis].label.split(" — ")[0]}
            </p>
          </div>
        ),
      },
      {
        accessorKey: "lembaga",
        header: "Lembaga",
        cell: ({ getValue }) => (
          <SelTeks className="max-w-[8rem]">{getValue<string>()}</SelTeks>
        ),
      },
      {
        accessorKey: "wilayah",
        header: "Cakupan",
        enableSorting: false,
        cell: ({ getValue }) => (
          <SelTeks className="max-w-[6.5rem] text-muted-foreground">
            {getValue<string>()}
          </SelTeks>
        ),
      },
      {
        accessorKey: "mfa",
        header: "MFA",
        enableSorting: false,
        cell: ({ getValue }) =>
          getValue<boolean>() ? (
            <ShieldCheck
              className="size-4 text-normal"
              aria-label="MFA aktif"
            />
          ) : (
            <span className="text-xs text-waspada-ink">Belum</span>
          ),
      },
      {
        id: "terakhir_masuk",
        accessorFn: (u) => u.terakhirMasuk,
        header: "Terakhir masuk",
        cell: ({ getValue }) => {
          const v = getValue<string>();
          return (
            <span className="whitespace-nowrap text-muted-foreground">
              {v ? formatRelatif(v) : "Belum pernah"}
            </span>
          );
        },
      },
      {
        accessorKey: "status",
        header: "Status",
        cell: ({ row: { original: u }, getValue }) => {
          const s = STATUS[getValue<PenggunaRow["status"]>()];
          return (
            <div className="flex flex-wrap items-center gap-1.5">
              <StatusBadge tone={s.tone}>{s.label}</StatusBadge>
              <LencanaTerkunci sampai={u.terkunciSampai} />
            </div>
          );
        },
      },
      {
        id: "dibuat_pada",
        accessorFn: (u) => u.dibuatPada ?? "",
        header: "Dibuat",
        cell: ({ getValue }) => {
          const v = getValue<string>();
          return (
            <span className="whitespace-nowrap text-muted-foreground">
              {v ? formatTanggal(v) : "–"}
            </span>
          );
        },
      },
      {
        id: "_aksi",
        header: "",
        enableSorting: false,
        size: 48,
        meta: { sematKanan: true },
        cell: ({ row: { original: u } }) => (
          <MenuAksiPengguna
            user={u}
            bolehKelola={bolehKelola}
            idSendiri={idSendiri}
            onDialog={(mode) => setDialog({ mode, user: u })}
            onBukaKunci={() => bukaKunci.mutate({ id: u.id, nama: u.nama })}
            onPaksaKeluar={() => paksaKeluar.mutate({ id: u.id, nama: u.nama })}
            onUbahAdmin={() =>
              assignRole.mutate({
                id: u.id,
                peran:
                  u.role === "ADMIN_SISTEM" ? "OPS_MAHASISWA" : "ADMIN_SISTEM",
              })
            }
            onHapus={() => hapus.mutate(u.id)}
          />
        ),
      },
    ],
    [assignRole, hapus, bukaKunci, bolehKelola, paksaKeluar, idSendiri],
  );

  // API hanya mengembalikan satu halaman: angka dihitung dari baris yang dimuat, dan cakupannya disebut di tiap sel.
  // Sel yang filternya sedang aktif memakai total server karena seluruh hasilnya memenuhi kriteria sel itu.
  const baris = data?.rows ?? [];
  const sebagian = (total ?? 0) > baris.length;
  const cakupan = sebagian
    ? "di halaman ini"
    : jumlahFilterAktif(tabelState) > 0 || tabelState.search
      ? "sesuai filter"
      : "seluruh akun";
  const selRingkas = (
    label: string,
    kunci: "status" | "mfa" | "terkunci",
    nilaiFilter: string,
    cocok: (u: PenggunaRow) => boolean,
    tone: ItemRingkas["tone"],
    arti: string,
  ): ItemRingkas => {
    const aktif = tabelState[kunci] === nilaiFilter;
    const jumlah = aktif ? (total ?? 0) : baris.filter(cocok).length;
    return {
      label,
      nilai: isLoading ? "–" : jumlah.toLocaleString("id-ID"),
      keterangan: aktif ? "Filter aktif" : cakupan,
      tone,
      arti,
      aktif,
      onClick: () =>
        ubahTabel(
          ubahFilter(tabelState, kunci, aktif ? undefined : nilaiFilter),
        ),
    };
  };
  const ringkasan: ItemRingkas[] = [
    selRingkas(
      "Akun aktif",
      "status",
      "AKTIF",
      (u) => u.status === "AKTIF",
      "normal",
      "Akun yang dapat masuk dan bekerja. Klik untuk menyaring.",
    ),
    selRingkas(
      "Menunggu aktivasi",
      "status",
      "MENUNGGU",
      (u) => u.status === "MENUNGGU",
      "waspada",
      "Sudah diundang tetapi belum mengaktifkan akun. Klik untuk menyaring, lalu kirim ulang undangan dari menu baris.",
    ),
    selRingkas(
      "Belum MFA",
      "mfa",
      "false",
      (u) => !u.mfa,
      "waspada",
      "Akun tanpa verifikasi dua langkah. Klik untuk menyaring.",
    ),
    selRingkas(
      "Terkunci",
      "terkunci",
      "true",
      (u) => masihTerkunci(u.terkunciSampai),
      "defisit",
      "Masuk diblokir sementara setelah gagal berulang. Klik untuk menyaring, lalu buka kunci dari menu baris.",
    ),
  ];

  return (
    <HalamanPas>
      <PageHeader
        title="Pengguna & peran"
        description="Kelola akun, peran, dan keamanan masuk. Setiap perubahan akun tercatat di audit trail."
        tabs={
          <ViewTabs
            layoutId="pengguna-tab"
            label="Tampilan pengguna"
            value={tab}
            onChange={setTab}
            tabs={[
              { value: "daftar", label: "Daftar pengguna" },
              { value: "matriks", label: "Matriks akses" },
            ]}
          />
        }
      />
      {tab === "daftar" && (
        <RingkasanKeputusan items={ringkasan} label="Ringkasan akun" />
      )}
      {tab === "matriks" ? (
        <IsiPas>
          {bolehKelola && !MODE_MOCK ? (
            <MatriksAksesEditor />
          ) : (
            <MatriksAkses />
          )}
          <p className="shrink-0 text-xs text-muted-foreground">
            Matriks ini dibaca dari sumber izin yang sama dengan penegakan akses
            di server.
          </p>
        </IsiPas>
      ) : (
        <DataTable
          isiPenuh
          data={data?.rows}
          isLoading={isLoading}
          columns={kolom}
          getRowId={(r) => r.id}
          entityLabel="pengguna"
          searchPlaceholder="Cari nama, email, peran, atau lembaga"
          server={{
            total: data?.total ?? 0,
            page: tabelState.page,
            pageSize: tabelState.limit,
            onPageChange: (p) => ubahTabel(ubahHalaman(tabelState, p)),
            onPageSizeChange: (n) => ubahTabel(ubahUkuran(tabelState, n)),
            search: tabelState.search,
            onSearchChange: (q) =>
              ubahTabel(ubahCari(tabelState, q), "replace"),
            sorting: toSortingState(tabelState),
            onSortToggle: (id) =>
              adalahKolomUrut(id) && ubahTabel(ubahUrut(tabelState, id)),
            kunciKueri: kunciCachePengguna(tabelState),
            isFetching: isPlaceholderData,
          }}
          filterBar={
            <FilterPengguna
              state={tabelState}
              onFilter={(kunci, nilai) =>
                ubahTabel(ubahFilter(tabelState, kunci, nilai))
              }
              onReset={() => ubahTabel(resetFilter(tabelState))}
            />
          }
          toolbar={
            <AksiUndangan
              onBuatSementara={() => setDialog({ mode: "buat", user: null })}
              filter={{
                search: tabelState.search,
                peran: tabelState.peran,
                status: tabelState.status,
                lapis: tabelState.lapis,
                mfa: tabelState.mfa,
                terkunci: tabelState.terkunci,
              }}
            />
          }
          bulkActions={[
            {
              id: "aktifkan",
              label: "Aktifkan",
              icon: UserCheck,
              perm: "pengguna.kelola",
              tone: "data",
              enabled: (r) => r.every((u) => u.status === "NONAKTIF"),
              run: (r) =>
                bulkMut.mutateAsync({
                  aksi: "aktifkan",
                  ids: r.map((u) => u.id),
                  label: "Aktifkan",
                }),
            },
            {
              id: "nonaktifkan",
              label: "Nonaktifkan",
              icon: UserX,
              perm: "pengguna.kelola",
              tone: "danger",
              enabled: (r) => r.every((u) => u.status === "AKTIF"),
              confirm: {
                title: "Nonaktifkan akun terpilih?",
                description: (n) =>
                  `${n} akun tidak dapat masuk dan seluruh sesinya diakhiri. Data yang pernah diinput tetap tersimpan.`,
                actionLabel: "Nonaktifkan",
              },
              run: (r) =>
                bulkMut.mutateAsync({
                  aksi: "nonaktifkan",
                  ids: r.map((u) => u.id),
                  label: "Nonaktifkan",
                }),
            },
            {
              id: "mfa",
              label: "Wajibkan MFA",
              icon: KeyRound,
              perm: "pengguna.kelola",
              run: (r) =>
                bulkMut.mutateAsync({
                  aksi: "wajibkan_mfa",
                  ids: r.map((u) => u.id),
                  label: "Wajibkan MFA",
                }),
            },
            {
              id: "sesi",
              label: "Akhiri sesi",
              icon: LogOut,
              perm: "pengguna.kelola",
              run: (r) =>
                bulkMut.mutateAsync({
                  aksi: "akhiri_sesi",
                  ids: r.map((u) => u.id),
                  label: "Akhiri sesi",
                }),
            },
            {
              id: "ekspor",
              label: "Ekspor CSV",
              icon: Download,
              perm: "pengguna.kelola",
              run: (r) => eksporTerpilih.jalankan({ ids: r.map((u) => u.id) }),
            },
          ]}
        />
      )}

      {dialog?.mode === "mfa" && (
        <ResetMfaDialog
          user={dialog.user}
          open
          onOpenChange={(v) => !v && setDialog(null)}
        />
      )}
      {dialog?.mode === "reset" ? (
        <ResetPasswordDialog
          user={dialog.user}
          open
          onOpenChange={(v) => !v && setDialog(null)}
        />
      ) : (
        <PenggunaDialog
          key={dialog?.user?.id ?? "baru"}
          user={dialog?.user ?? null}
          open={dialog?.mode === "buat" || dialog?.mode === "ubah"}
          onOpenChange={(v) => !v && setDialog(null)}
        />
      )}
    </HalamanPas>
  );
}
