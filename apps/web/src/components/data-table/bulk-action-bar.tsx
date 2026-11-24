"use client";

import { AnimatePresence, motion } from "motion/react";
import { Loader2, MoreHorizontal, X, type LucideIcon } from "lucide-react";
import { useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { FormField } from "@/components/form/form-field";
import { Textarea } from "@/components/ui/textarea";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SpringButton } from "@/components/garda/spring-button";
import { useOptionalSession } from "@/lib/auth/session-context";
import { bulkBar, popIn } from "@/lib/motion/presets";
import { can, type Permission } from "@/lib/rbac/permissions";
import { cn } from "@/lib/utils";

export interface BulkAction<T> {
  id: string;
  label: string;
  icon?: LucideIcon;
  perm?: Permission;
  tone?: "default" | "danger" | "data";
  /** `alasan` menambah isian teks wajib di dialog (minimal `min` karakter) yang diteruskan ke `run`. */
  confirm?: {
    title: string;
    description: (n: number, rows: T[]) => string;
    actionLabel: string;
    alasan?: { label: string; min: number };
  };
  /** Aksi aktif hanya bila seluruh baris terpilih memenuhi syarat */
  enabled?: (rows: T[]) => boolean;
  run: (rows: T[], alasan?: string) => Promise<unknown> | unknown;
}

interface Props<T> {
  rows: T[];
  actions: BulkAction<T>[];
  entityLabel: string;
  onClear: () => void;
  /** Posisi: fixed (halaman) atau sticky di dalam kontainer (mobile) */
  placement?: "fixed" | "inline";
  maxVisible?: number;
  className?: string;
}

/**
 * Bilah aksi massal mengambang. Muncul saat ≥1 baris dicentang; aksi
 * difilter oleh izin peran; aksi destruktif memerlukan konfirmasi.
 */
export function BulkActionBar<T>({
  rows,
  actions,
  entityLabel,
  onClear,
  placement = "fixed",
  maxVisible = 3,
  className,
}: Props<T>) {
  const sesi = useOptionalSession();
  const [busy, setBusy] = useState<string | null>(null);
  const [pending, setPending] = useState<BulkAction<T> | null>(null);
  const [alasan, setAlasan] = useState("");
  const allowed = actions.filter((a) => !a.perm || can(sesi, a.perm));
  const visible = allowed.slice(0, maxVisible);
  const overflow = allowed.slice(maxVisible);

  async function execute(a: BulkAction<T>) {
    setBusy(a.id);
    try {
      await a.run(rows, a.confirm?.alasan ? alasan.trim() : undefined);
      onClear();
    } finally {
      setBusy(null);
      setPending(null);
    }
  }
  const trigger = (a: BulkAction<T>) => {
    if (!a.confirm) return execute(a);
    setAlasan("");
    setPending(a);
  };
  const alasanKurang =
    !!pending?.confirm?.alasan &&
    alasan.trim().length < pending.confirm.alasan.min;
  const isEnabled = (a: BulkAction<T>) => !a.enabled || a.enabled(rows);

  return (
    <>
      <AnimatePresence>
        {rows.length > 0 && (
          <motion.div
            variants={bulkBar}
            initial="hidden"
            animate="show"
            exit="exit"
            role="toolbar"
            aria-label={`Aksi untuk ${rows.length} ${entityLabel} terpilih`}
            className={cn(
              "no-scrollbar z-40 flex w-max items-center gap-1.5 overflow-x-auto rounded-lg border border-white/10 bg-navy-800 p-1.5 pl-3 text-white shadow-ambient-lg",
              placement === "fixed"
                ? "fixed bottom-4 left-1/2 max-w-[calc(100vw-2rem)] -translate-x-1/2 sm:bottom-6"
                : "sticky bottom-3 mx-auto max-w-full",
              className,
            )}
          >
            <div className="flex items-center gap-2 pr-2">
              <AnimatePresence mode="popLayout" initial={false}>
                <motion.span
                  key={rows.length}
                  variants={popIn}
                  initial="hidden"
                  animate="show"
                  exit="exit"
                  className="num grid h-6 min-w-6 place-items-center rounded-md bg-white px-1.5 text-[13px] font-semibold text-navy-900"
                >
                  {rows.length.toLocaleString("id-ID")}
                </motion.span>
              </AnimatePresence>
              <span className="hidden text-[13px] text-white/70 sm:inline">
                {entityLabel} terpilih
              </span>
            </div>
            <span className="mx-1 h-5 w-px bg-white/20" />
            {allowed.length === 0 && (
              <span className="px-2 text-[13px] text-white/70">
                Peran Anda tidak memiliki aksi massal di sini
              </span>
            )}
            {visible.map((a) => (
              <SpringButton
                key={a.id}
                size="sm"
                variant={
                  a.tone === "danger"
                    ? "danger"
                    : a.tone === "data"
                      ? "data"
                      : "ghost"
                }
                className={cn(
                  (!a.tone || a.tone === "default") &&
                    "bg-white/10 text-white hover:bg-white/20 hover:text-white",
                )}
                disabled={!!busy || !isEnabled(a)}
                title={
                  !isEnabled(a)
                    ? "Tidak berlaku untuk sebagian data terpilih"
                    : undefined
                }
                onClick={() => trigger(a)}
              >
                {busy === a.id ? (
                  <Loader2 className="animate-spin" />
                ) : (
                  a.icon && <a.icon />
                )}
                <span className={cn(a.icon && "hidden md:inline")}>
                  {a.label}
                </span>
              </SpringButton>
            ))}
            {overflow.length > 0 && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <SpringButton
                    size="icon"
                    variant="ghost"
                    aria-label="Aksi lainnya"
                    className="size-8 text-white/80 hover:bg-white/10 hover:text-white"
                  >
                    <MoreHorizontal />
                  </SpringButton>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" side="top" className="w-56">
                  {overflow.map((a) => (
                    <DropdownMenuItem
                      key={a.id}
                      disabled={!isEnabled(a)}
                      onSelect={() => trigger(a)}
                      className={cn(a.tone === "danger" && "text-defisit-ink")}
                    >
                      {a.icon && <a.icon className="size-4" />} {a.label}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
            <SpringButton
              size="icon"
              variant="ghost"
              onClick={onClear}
              aria-label="Batalkan pilihan (Esc)"
              className="size-8 text-white/80 hover:bg-white/10 hover:text-white"
            >
              <X />
            </SpringButton>
          </motion.div>
        )}
      </AnimatePresence>

      <AlertDialog
        open={!!pending}
        onOpenChange={(o: boolean) => !o && setPending(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{pending?.confirm?.title}</AlertDialogTitle>
            <AlertDialogDescription>
              {pending?.confirm?.description(rows.length, rows)}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {pending?.confirm?.alasan && (
            <FormField id="bulk-alasan" label={pending.confirm.alasan.label}>
              <Textarea
                id="bulk-alasan"
                value={alasan}
                onChange={(e) => setAlasan(e.target.value)}
                rows={3}
                maxLength={1000}
              />
            </FormField>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              disabled={alasanKurang}
              className={cn(
                pending?.tone === "danger" &&
                  "bg-kritis text-white hover:bg-kritis/90",
              )}
              onClick={(e: React.MouseEvent) => {
                e.preventDefault();
                if (pending) execute(pending);
              }}
            >
              {busy ? <Loader2 className="size-4 animate-spin" /> : null}
              {pending?.confirm?.actionLabel}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
