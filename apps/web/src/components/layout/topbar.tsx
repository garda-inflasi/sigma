"use client";

import { AnimatePresence, motion } from "motion/react";
import { Bell, LogOut, Menu, Moon, Search, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { Kbd } from "@/components/command/command-palette";
import { PulseRing } from "@/components/motion/pulse-ring";
import { MODE_MOCK } from "@/lib/api/client";
import { useSession } from "@/lib/auth/session-context";
import { siklusMeta, toneClass } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { popIn } from "@/lib/motion/presets";
import { NAV_FLAT, activeNav } from "@/lib/nav";
import { useSiklusAktif } from "@/lib/query/hooks";
import { can } from "@/lib/rbac/permissions";
import { ROLES } from "@/lib/rbac/roles";
import { useEwsStore } from "@/lib/stores/ews-store";
import { useUi } from "@/lib/stores/ui-store";
import { cn } from "@/lib/utils";
import { BrandMark } from "./brand-mark";
import { RoleSwitcher } from "./role-switcher";
import { NavList } from "./sidebar";

function useToday() {
  const [d, setD] = useState<string>("");
  useEffect(() => setD(formatTanggal(new Date(), "EEEE, d MMMM yyyy")), []);
  return d;
}

export function Topbar() {
  const pathname = usePathname();
  const user = useSession();
  const today = useToday();
  const setCommandOpen = useUi((s) => s.setCommandOpen);
  const mobileOpen = useUi((s) => s.mobileNavOpen);
  const setMobileOpen = useUi((s) => s.setMobileNavOpen);
  const unread = useEwsStore((s) => s.unread);
  const bacaSemua = useEwsStore((s) => s.bacaSemua);
  const stream = useEwsStore((s) => s.status);
  const { data: siklus } = useSiklusAktif(can(user.role, "dashboard.operasional"));
  const { resolvedTheme, setTheme } = useTheme();
  const title = activeNav(pathname, NAV_FLAT)?.label ?? "Garda Inflasi";
  const meta = siklus ? siklusMeta[siklus.siklus] : null;

  return (
    <header className="sticky top-3 z-20 mx-3 lg:mx-1 lg:mr-3">
      <div className="glass flex h-16 items-center gap-3 rounded-xl px-3 sm:px-4">
        <button onClick={() => setMobileOpen(true)} className="grid size-9 place-items-center rounded-xl hover:bg-secondary lg:hidden" aria-label="Buka navigasi">
          <Menu className="size-5" />
        </button>
        <div className="min-w-0 flex-1">
          <p className="truncate font-display text-[15px] font-semibold text-navy-900 dark:text-white">{title}</p>
          <p className="hidden truncate text-xs text-muted-foreground sm:block">{today}</p>
        </div>

        <button
          onClick={() => setCommandOpen(true)}
          className="glass-subtle hidden h-9 w-64 items-center gap-2 rounded-md px-3 text-sm text-muted-foreground transition-shadow hover:shadow-ambient-sm md:flex"
        >
          <Search className="size-4" />
          <span className="flex-1 text-left">Cari atau jalankan perintah</span>
          <Kbd>⌘K</Kbd>
        </button>

        {meta && (
          <Link href={can(user.role, "intervensi.lihat") ? `/intervensi?siklus=${siklus!.siklus}` : "#"} className={cn("hidden h-9 items-center gap-2 rounded-xl px-3 text-xs font-medium sm:inline-flex", toneClass[meta.tone].soft, toneClass[meta.tone].text)}>
            <PulseRing tone={meta.tone} rings={1} active={siklus!.siklus === "tekanan"} />
            {meta.label}
          </Link>
        )}

        {MODE_MOCK && <span className="hidden rounded-md border border-dashed border-border px-2 py-1 text-[11px] text-muted-foreground 2xl:inline">Data demo</span>}
        <RoleSwitcher />

        {can(user.role, "ews.lihat") && (
          <Link href="/ews" onClick={bacaSemua} className="relative grid size-9 place-items-center rounded-xl hover:bg-secondary" aria-label={`Peringatan dini, ${unread} belum dibaca`}>
            <Bell className="size-[18px]" />
            <AnimatePresence>
              {unread > 0 && (
                <motion.span variants={popIn} initial="hidden" animate="show" exit="exit" className="absolute top-1 right-1">
                  <PulseRing tone="defisit" rings={1}>
                    <span className="num relative grid h-4 min-w-4 place-items-center rounded-full bg-defisit px-1 text-[10px] font-semibold text-white">{unread}</span>
                  </PulseRing>
                </motion.span>
              )}
            </AnimatePresence>
            <span className={cn("absolute bottom-1.5 left-1.5 size-1.5 rounded-full", stream === "terputus" ? "bg-mist-400" : "animate-live bg-normal")} title={`Aliran EWS: ${stream}`} />
          </Link>
        )}

        <DropdownMenu>
          <DropdownMenuTrigger className="grid size-9 place-items-center rounded-xl bg-navy-800 text-xs font-semibold text-white dark:bg-data-600" aria-label="Menu akun">
            {user.nama.split(" ").filter((w) => /^[A-Z]/.test(w)).slice(0, 2).map((w) => w[0]).join("")}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64 rounded-xl p-1.5">
            <DropdownMenuLabel>
              <p className="font-medium">{user.nama}</p>
              <p className="text-xs font-normal text-muted-foreground">{ROLES[user.role].label}</p>
              <p className="text-xs font-normal text-muted-foreground">{user.lembaga}</p>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")} className="rounded-md">
              {resolvedTheme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
              {resolvedTheme === "dark" ? "Mode terang" : "Mode gelap"}
            </DropdownMenuItem>
            <DropdownMenuItem asChild className="rounded-md">
              <Link href="/masuk"><LogOut className="size-4" /> Keluar</Link>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="left" className="glass-strong flex w-[300px] flex-col gap-0 border-0 p-0">
          <SheetTitle className="flex h-[72px] items-center gap-3 px-5">
            <BrandMark /> <span className="font-display font-bold">Garda Inflasi</span>
          </SheetTitle>
          <NavList layoutId="mobile-nav-active" onNavigate={() => setMobileOpen(false)} />
        </SheetContent>
      </Sheet>
    </header>
  );
}
