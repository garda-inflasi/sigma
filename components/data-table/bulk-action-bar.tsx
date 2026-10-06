"use client";

import { AnimatePresence, motion } from "motion/react";
import { Loader2, MoreHorizontal, X, type LucideIcon } from "lucide-react";
import { useState } from "react";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
  AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
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
  confirm?: { title: string; description: (n: number) => string; actionLabel: string };
  /** Aksi aktif hanya bila seluruh baris terpilih memenuhi syarat */
  enabled?: (rows: T[]) => boolean;
  run: (rows: T[]) => Promise<unknown> | unknown;
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
export function BulkActionBar<T>({ rows, actions, entityLabel, onClear, placement = "fixed", maxVisible = 3, className }: Props<T>) {
  const role = useOptionalSession()?.role;
  const [busy, setBusy] = useState<string | null>(null);
  const [pending, setPending] = useState<BulkAction<T> | null>(null);
  const allowed = actions.filter((a) => !a.perm || can(role, a.perm));
  const visible = allowed.slice(0, maxVisible);
  const overflow = allowed.slice(maxVisible);

  async function execute(a: BulkAction<T>) {
    setBusy(a.id);
    try {
      await a.run(rows);
      onClear();
    } finally {
      setBusy(null);
      setPending(null);
    }
  }
  const trigger = (a: BulkAction<T>) => (a.confirm ? setPending(a) : execute(a));
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
              "glass-strong z-40 flex items-center gap-1.5 rounded-xl p-1.5 pl-3",
              placement === "fixed" ? "fixed bottom-6 left-1/2 -translate-x-1/2 max-w-[calc(100vw-2rem)]" : "sticky bottom-3 mx-auto w-fit max-w-full",
              className,
            )}
          >
            <div className="flex items-center gap-2 pr-2">
              <AnimatePresence mode="popLayout" initial={false}>
                <motion.span key={rows.length} variants={popIn} initial="hidden" animate="show" exit="exit" className="num grid h-7 min-w-7 place-items-center rounded-lg bg-navy-800 px-1.5 text-sm font-semibold text-white dark:bg-data-500">
                  {rows.length.toLocaleString("id-ID")}
                </motion.span>
              </AnimatePresence>
              <span className="hidden text-sm text-muted-foreground sm:inline">{entityLabel} terpilih</span>
            </div>
            <span className="mx-1 h-6 w-px bg-border" />
            {allowed.length === 0 && <span className="px-2 text-sm text-muted-foreground">Peran Anda tidak memiliki aksi massal di sini</span>}
            {visible.map((a) => (
              <SpringButton
                key={a.id}
                size="sm"
                variant={a.tone === "danger" ? "danger" : a.tone === "data" ? "data" : "soft"}
                disabled={!!busy || !isEnabled(a)}
                title={!isEnabled(a) ? "Tidak berlaku untuk sebagian data terpilih" : undefined}
                onClick={() => trigger(a)}
              >
                {busy === a.id ? <Loader2 className="animate-spin" /> : a.icon && <a.icon />}
                <span className={cn(a.icon && "hidden md:inline")}>{a.label}</span>
              </SpringButton>
            ))}
            {overflow.length > 0 && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <SpringButton size="icon" variant="ghost" aria-label="Aksi lainnya" className="size-8">
                    <MoreHorizontal />
                  </SpringButton>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" side="top" className="w-56 rounded-xl">
                  {overflow.map((a) => (
                    <DropdownMenuItem key={a.id} disabled={!isEnabled(a)} onSelect={() => trigger(a)} className={cn(a.tone === "danger" && "text-defisit-ink")}>
                      {a.icon && <a.icon className="size-4" />} {a.label}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
            <SpringButton size="icon" variant="ghost" onClick={onClear} aria-label="Batalkan pilihan (Esc)" className="size-8">
              <X />
            </SpringButton>
          </motion.div>
        )}
      </AnimatePresence>

      <AlertDialog open={!!pending} onOpenChange={(o: boolean) => !o && setPending(null)}>
        <AlertDialogContent className="rounded-xl">
          <AlertDialogHeader>
            <AlertDialogTitle>{pending?.confirm?.title}</AlertDialogTitle>
            <AlertDialogDescription>{pending?.confirm?.description(rows.length)}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="rounded-xl">Batal</AlertDialogCancel>
            <AlertDialogAction
              className={cn("rounded-xl", pending?.tone === "danger" && "bg-defisit hover:bg-kritis")}
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
