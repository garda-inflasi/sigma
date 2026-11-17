"use client";

import { motion } from "motion/react";
import { Check, ChevronsUpDown, MapPin, PanelLeftClose, PanelLeftOpen } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { keteranganSiklus } from "@/lib/api/siklus";
import { useSession } from "@/lib/auth/session-context";
import { siklusMeta, toneClass } from "@/lib/design/tokens";
import { NAV, activeNav } from "@/lib/nav";
import { inisialNama } from "@/lib/nama";
import { spring } from "@/lib/motion/presets";
import { useJumlahUsulanMenunggu, useRingkasanHarga, useSiklusAktif } from "@/lib/query/hooks";
import { can } from "@/lib/rbac/permissions";
import { ROLES } from "@/lib/rbac/roles";
import { useEwsStore } from "@/lib/stores/ews-store";
import { useUi } from "@/lib/stores/ui-store";
import { cn } from "@/lib/utils";
import { BrandMark } from "./brand-mark";

export function useVisibleNav() {
  const sesi = useSession();
  return NAV.map((g) => ({ ...g, items: g.items.filter((i) => !i.perm || can(sesi, i.perm)) })).filter((g) => g.items.length > 0);
}

function useBadges() {
  const sesi = useSession();
  const unread = useEwsStore((s) => s.unread);
  const { data } = useRingkasanHarga(90, can(sesi, "harga.lihat"));
  const validasi = can(sesi, "harga.validasi") || can(sesi, "harga.publikasi") ? (data?.diajukan ?? 0) : 0;
  const bisaSetujuiMaster = can(sesi, "master.setujui");
  const master = useJumlahUsulanMenunggu(bisaSetujuiMaster).data ?? 0;
  return { ews: unread, validasi, master: bisaSetujuiMaster ? master : 0 };
}

export function NavList({ collapsed = false, layoutId = "sidebar-active", onNavigate }: { collapsed?: boolean; layoutId?: string; onNavigate?: () => void }) {
  const pathname = usePathname();
  const groups = useVisibleNav();
  const active = activeNav(pathname, groups.flatMap((g) => g.items));
  const badges = useBadges();

  return (
    <nav className="scrollbar-thin flex-1 space-y-5 overflow-y-auto px-2.5 pb-4" aria-label="Navigasi utama">
      {groups.map((g) => (
        <div key={g.grup}>
          {!collapsed ? (
            <p className="px-2.5 pb-1.5 text-[10.5px] font-medium tracking-[0.06em] text-muted-foreground uppercase">{g.grup}</p>
          ) : (
            <span aria-hidden className="mx-auto mb-1.5 block h-px w-5 bg-border" />
          )}
          <ul className="space-y-0.5">
            {g.items.map((item) => {
              const isActive = active?.href === item.href;
              const badge = item.badge ? badges[item.badge] : 0;
              const link = (
                <Link
                  href={item.href}
                  onClick={onNavigate}
                  aria-current={isActive ? "page" : undefined}
                  className={cn(
                    "relative flex h-8 items-center gap-2.5 rounded-md px-2.5 text-[13px] transition-colors focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none",
                    collapsed && "justify-center px-0",
                    isActive ? "font-medium text-foreground" : "text-muted-foreground hover:bg-secondary hover:text-foreground",
                  )}
                >
                  {isActive && (
                    <motion.span
                      layoutId={layoutId}
                      transition={spring.layout}
                      className="absolute inset-0 rounded-md border border-border bg-card shadow-ambient-xs"
                    />
                  )}
                  <item.icon strokeWidth={1.75} className={cn("relative size-4 shrink-0", isActive && "text-foreground")} />
                  {!collapsed && <span className="relative truncate">{item.label}</span>}
                  {badge > 0 && (
                    <span
                      className={cn(
                        "num relative grid h-[18px] min-w-[18px] place-items-center rounded-md px-1 text-[11px] font-medium",
                        item.badge === "ews" ? "bg-defisit-soft text-defisit-ink" : "bg-waspada-soft text-waspada-ink",
                        collapsed ? "absolute top-0.5 right-0.5 h-4 min-w-4 px-0.5 text-[10px]" : "ml-auto",
                      )}
                    >
                      {badge > 99 ? "99+" : badge}
                    </span>
                  )}
                </Link>
              );
              return (
                <li key={item.href}>
                  {collapsed ? (
                    <Tooltip>
                      <TooltipTrigger asChild>{link}</TooltipTrigger>
                      <TooltipContent side="right">{item.label}</TooltipContent>
                    </Tooltip>
                  ) : (
                    link
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      ))}
    </nav>
  );
}

/**
 * Pengalih ruang kerja di puncak sidebar: wilayah kerja aktif (Provinsi Aceh) beserta indikator siklus ekonomi.
 * Saat ini hanya satu wilayah, jadi menunya menjelaskan wilayah dan siklus, bukan berpindah.
 */
export function WorkspaceSwitcher({ collapsed = false }: { collapsed?: boolean }) {
  const sesi = useSession();
  const { data } = useSiklusAktif(can(sesi, "dashboard.operasional"));
  const meta = data ? siklusMeta[data.siklus] : null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Ruang kerja: Garda Inflasi, Provinsi Aceh"
        className={cn(
          "flex w-full items-center gap-2.5 rounded-lg border border-transparent p-1.5 text-left transition-colors outline-none hover:border-border hover:bg-card focus-visible:ring-2 focus-visible:ring-ring/50 data-[state=open]:border-border data-[state=open]:bg-card",
          collapsed && "justify-center",
        )}
      >
        <BrandMark />
        {!collapsed && (
          <>
            <span className="min-w-0 flex-1 leading-tight">
              <span className="block truncate text-[13px] font-semibold text-foreground">Garda Inflasi</span>
              <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <MapPin className="size-3" strokeWidth={1.75} aria-hidden />
                <span className="truncate">Provinsi Aceh</span>
                {meta && <span aria-hidden className={cn("size-1.5 shrink-0 rounded-full", toneClass[meta.tone].dot)} />}
              </span>
            </span>
            <ChevronsUpDown className="size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.75} aria-hidden />
          </>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64 rounded-lg p-1.5">
        <DropdownMenuLabel className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">Wilayah kerja</DropdownMenuLabel>
        <DropdownMenuItem className="gap-2 rounded-md" disabled>
          <BrandMark className="size-5" />
          <span className="flex-1">Provinsi Aceh</span>
          <Check className="size-4" strokeWidth={1.75} />
        </DropdownMenuItem>
        {data && meta && (
          <>
            <DropdownMenuSeparator />
            <div className="px-2 py-1.5 text-xs">
              <p className="flex items-center gap-1.5 font-medium">
                <span aria-hidden className={cn("size-1.5 rounded-full", toneClass[meta.tone].dot)} />
                {meta.label}
              </p>
              <p className="mt-0.5 text-muted-foreground">{keteranganSiklus(data) || meta.deskripsi}</p>
            </div>
          </>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild className="rounded-md">
          <Link href="/alur-sistem">Peta alur sistem & data</Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function Sidebar() {
  const user = useSession();
  const collapsed = useUi((s) => s.sidebarCollapsed);
  const toggle = useUi((s) => s.toggleSidebar);

  return (
    <motion.aside
      initial={false}
      animate={{ width: collapsed ? 64 : 248 }}
      transition={spring.layout}
      // Offset `--bar-atas` (ConsoleShell) menyisihkan ruang bagi banner impersonasi di puncak viewport.
      className="sticky top-[var(--bar-atas,0px)] z-30 hidden h-[calc(100dvh-var(--bar-atas,0px))] shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar lg:flex"
    >
      <div className={cn("p-2.5 pb-3", collapsed && "px-2")}>
        <WorkspaceSwitcher collapsed={collapsed} />
      </div>
      <NavList collapsed={collapsed} />
      <div className={cn("space-y-1 border-t border-sidebar-border p-2.5", collapsed && "px-2")}>
        <div className={cn("flex items-center gap-2.5 rounded-lg p-1.5", collapsed && "justify-center")}>
          <span aria-hidden className="grid size-7 shrink-0 place-items-center rounded-full bg-navy-800 text-[11px] font-semibold text-white dark:bg-white dark:text-navy-900">
            {inisialNama(user.nama)}
          </span>
          {!collapsed && (
            <div className="min-w-0 leading-tight">
              <p className="truncate text-[13px] font-medium">{user.nama}</p>
              <p className="truncate text-xs text-muted-foreground">{ROLES[user.role].label}</p>
            </div>
          )}
        </div>
        <button
          type="button"
          onClick={toggle}
          className="flex h-8 w-full items-center justify-center gap-2 rounded-md text-[13px] text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
          aria-label={collapsed ? "Bentangkan sidebar" : "Ciutkan sidebar"}
        >
          {collapsed ? <PanelLeftOpen className="size-4" strokeWidth={1.75} /> : <><PanelLeftClose className="size-4" strokeWidth={1.75} /> Ciutkan</>}
        </button>
      </div>
    </motion.aside>
  );
}
