"use client";

import { motion } from "motion/react";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useSession } from "@/lib/auth/session-context";
import { NAV, activeNav } from "@/lib/nav";
import { spring } from "@/lib/motion/presets";
import { useEntriHarga } from "@/lib/query/hooks";
import { can } from "@/lib/rbac/permissions";
import { ROLES } from "@/lib/rbac/roles";
import { useEwsStore } from "@/lib/stores/ews-store";
import { useUi } from "@/lib/stores/ui-store";
import { cn } from "@/lib/utils";
import { BrandMark } from "./brand-mark";

export function useVisibleNav() {
  const { role } = useSession();
  return NAV.map((g) => ({ ...g, items: g.items.filter((i) => !i.perm || can(role, i.perm)) })).filter((g) => g.items.length > 0);
}

function useBadges() {
  const { role } = useSession();
  const unread = useEwsStore((s) => s.unread);
  const { data } = useEntriHarga();
  const validasi = can(role, "harga.validasi") || can(role, "harga.publikasi") ? (data?.filter((e) => e.status === "DIAJUKAN").length ?? 0) : 0;
  return { ews: unread, validasi };
}

export function NavList({ collapsed = false, layoutId = "sidebar-active", onNavigate }: { collapsed?: boolean; layoutId?: string; onNavigate?: () => void }) {
  const pathname = usePathname();
  const groups = useVisibleNav();
  const active = activeNav(pathname, groups.flatMap((g) => g.items));
  const badges = useBadges();

  return (
    <nav className="scrollbar-thin flex-1 space-y-5 overflow-y-auto px-3 pb-4" aria-label="Navigasi utama">
      {groups.map((g) => (
        <div key={g.grup}>
          {!collapsed && <p className="px-3 pb-1.5 text-xs font-medium text-muted-foreground/80">{g.grup}</p>}
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
                    "relative flex h-10 items-center gap-3 rounded-xl px-3 text-sm transition-colors",
                    collapsed && "justify-center px-0",
                    isActive ? "font-semibold text-navy-900 dark:text-white" : "text-muted-foreground hover:bg-white/50 hover:text-foreground dark:hover:bg-white/5",
                  )}
                >
                  {isActive && (
                    <motion.span layoutId={layoutId} transition={spring.layout} className="absolute inset-0 rounded-xl bg-white shadow-ambient-sm ring-1 ring-navy-900/5 dark:bg-white/10 dark:ring-white/10">
                      <span className="absolute inset-y-2.5 left-0 w-[3px] rounded-r-full bg-siklus" />
                    </motion.span>
                  )}
                  <item.icon className={cn("relative size-[18px] shrink-0", isActive && "text-navy-700 dark:text-data-300")} />
                  {!collapsed && <span className="relative truncate">{item.label}</span>}
                  {badge > 0 && (
                    <span className={cn(
                      "num relative grid h-5 min-w-5 place-items-center rounded-full px-1.5 text-[11px] font-semibold",
                      item.badge === "ews" ? "bg-defisit text-white" : "bg-waspada-soft text-waspada-ink",
                      collapsed ? "absolute top-1 right-1 h-4 min-w-4 px-1 text-[10px]" : "ml-auto",
                    )}>
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

export function Sidebar() {
  const user = useSession();
  const collapsed = useUi((s) => s.sidebarCollapsed);
  const toggle = useUi((s) => s.toggleSidebar);

  return (
    <motion.aside
      initial={false}
      animate={{ width: collapsed ? 84 : 272 }}
      transition={spring.layout}
      className="glass sticky top-3 z-30 m-3 mr-0 hidden h-[calc(100dvh-1.5rem)] shrink-0 flex-col overflow-hidden rounded-xl lg:flex"
    >
      <div className={cn("flex h-[72px] items-center gap-3 px-5", collapsed && "justify-center px-0")}>
        <BrandMark />
        {!collapsed && (
          <div className="leading-tight">
            <p className="font-display text-[15px] font-bold tracking-tight text-navy-900 dark:text-white">Garda Inflasi</p>
            <p className="text-xs text-muted-foreground">Provinsi Aceh</p>
          </div>
        )}
      </div>
      <NavList collapsed={collapsed} />
      <div className={cn("border-t border-border/70 p-3", collapsed && "px-2")}>
        {!collapsed && (
          <div className="mb-2 rounded-xl bg-white/50 p-3 dark:bg-white/5">
            <p className="truncate text-sm font-medium">{user.nama}</p>
            <p className="truncate text-xs text-muted-foreground">{ROLES[user.role].label}</p>
          </div>
        )}
        <button onClick={toggle} className="flex h-9 w-full items-center justify-center gap-2 rounded-xl text-sm text-muted-foreground hover:bg-white/60 hover:text-foreground dark:hover:bg-white/5" aria-label={collapsed ? "Bentangkan sidebar" : "Ciutkan sidebar"}>
          {collapsed ? <PanelLeftOpen className="size-4" /> : <><PanelLeftClose className="size-4" /> Ciutkan</>}
        </button>
      </div>
    </motion.aside>
  );
}
