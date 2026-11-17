"use client";

import { AnimatePresence, motion } from "motion/react";
import {
  Bell,
  ChevronRight,
  KeyRound,
  LogOut,
  Menu,
  MonitorSmartphone,
  Moon,
  Search,
  ShieldCheck,
  Sun,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import { useTheme } from "next-themes";
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
import { inisialNama } from "@/lib/nama";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { Kbd } from "@/components/command/command-palette";
import { PulseRing } from "@/components/motion/pulse-ring";
import { MODE_MOCK } from "@/lib/api/client";
import { keteranganSiklus } from "@/lib/api/siklus";
import { keluarDariSesi } from "@/lib/auth/logout-client";
import { useSession } from "@/lib/auth/session-context";
import { siklusMeta, toneClass } from "@/lib/design/tokens";
import { popIn } from "@/lib/motion/presets";
import { remahRoti } from "@/lib/nav";
import {
  LABEL_TAB_PROFIL,
  hrefTabProfil,
  type TabProfil,
} from "@/lib/profil/tab";
import { useSiklusAktif } from "@/lib/query/hooks";
import { can } from "@/lib/rbac/permissions";
import { ROLES } from "@/lib/rbac/roles";
import { useEwsStore } from "@/lib/stores/ews-store";
import { useUi } from "@/lib/stores/ui-store";
import { cn } from "@/lib/utils";
import { BrandMark } from "./brand-mark";
import { RoleSwitcher } from "./role-switcher";
import { NavList } from "./sidebar";

interface ItemMenuProfil {
  tab: TabProfil;
  label: string;
  ikon: LucideIcon;
}

/** Tautan menu akun ke tab /profil, berurutan: profil lebih dulu, lalu tiga bagian keamanan. */
const MENU_PROFIL: readonly ItemMenuProfil[] = [
  { tab: "profil", label: "Profil Saya", ikon: UserRound },
  {
    tab: "ganti-password",
    label: LABEL_TAB_PROFIL["ganti-password"],
    ikon: KeyRound,
  },
  { tab: "mfa", label: LABEL_TAB_PROFIL.mfa, ikon: ShieldCheck },
  {
    tab: "keamanan",
    label: LABEL_TAB_PROFIL.keamanan,
    ikon: MonitorSmartphone,
  },
];

/**
 * Item menu akun yang tampil. Mode demo (tanpa API) tidak punya BFF auth, jadi hanya "Profil Saya" (data sesi,
 * hanya-baca) yang tersedia; tiga bagian keamanan disembunyikan.
 */
export function itemMenuProfil(modeMock: boolean): readonly ItemMenuProfil[] {
  return modeMock ? MENU_PROFIL.filter((m) => m.tab === "profil") : MENU_PROFIL;
}

export function Topbar() {
  const pathname = usePathname();
  const user = useSession();
  const setCommandOpen = useUi((s) => s.setCommandOpen);
  const mobileOpen = useUi((s) => s.mobileNavOpen);
  const setMobileOpen = useUi((s) => s.setMobileNavOpen);
  const unread = useEwsStore((s) => s.unread);
  const bacaSemua = useEwsStore((s) => s.bacaSemua);
  const stream = useEwsStore((s) => s.status);
  const { data: siklus } = useSiklusAktif(can(user, "dashboard.operasional"));
  const { resolvedTheme, setTheme } = useTheme();
  const remah = remahRoti(pathname);
  const meta = siklus ? siklusMeta[siklus.siklus] : null;

  return (
    <header className="sticky top-0 z-20 border-b border-border bg-background/90 backdrop-blur-sm">
      <div className="flex h-14 items-center gap-3 px-3 sm:px-5">
        <button
          type="button"
          onClick={() => setMobileOpen(true)}
          className="grid size-8 place-items-center rounded-md hover:bg-secondary lg:hidden"
          aria-label="Buka navigasi"
        >
          <Menu className="size-5" strokeWidth={1.75} />
        </button>
        <div className="min-w-0 flex-1">
          <nav aria-label="Remah roti" className="flex min-w-0 items-center gap-1.5 text-[13px]">
            {remah.map((r, i) => {
              const terakhir = i === remah.length - 1;
              return (
                <span key={`${r}-${i}`} className="flex min-w-0 items-center gap-1.5">
                  {i > 0 && <ChevronRight className="size-3.5 shrink-0 text-navy-300" strokeWidth={1.75} aria-hidden />}
                  {terakhir ? (
                    <p aria-current="page" className="truncate font-medium text-foreground">
                      {r}
                    </p>
                  ) : (
                    <span className="hidden truncate text-muted-foreground sm:inline">{r}</span>
                  )}
                </span>
              );
            })}
          </nav>
        </div>

        <button
          type="button"
          onClick={() => setCommandOpen(true)}
          className="hidden h-8 w-64 items-center gap-2 rounded-md border border-border bg-card px-2.5 text-[13px] whitespace-nowrap text-muted-foreground shadow-ambient-xs transition-colors hover:bg-secondary md:flex"
        >
          <Search className="size-4" strokeWidth={1.75} />
          <span className="flex-1 truncate text-left">Cari atau jalankan perintah</span>
          <Kbd>⌘K</Kbd>
        </button>

        {meta && (
          <Link
            href={
              can(user, "intervensi.lihat")
                ? `/intervensi?siklus=${siklus!.siklus}`
                : "#"
            }
            title={keteranganSiklus(siklus!) || undefined}
            className={cn(
              "hidden h-7 items-center gap-2 rounded-md px-2.5 text-xs font-medium sm:inline-flex",
              toneClass[meta.tone].soft,
              toneClass[meta.tone].text,
            )}
          >
            <PulseRing
              tone={meta.tone}
              rings={1}
              active={siklus!.siklus === "tekanan"}
            />
            {meta.label}
          </Link>
        )}

        {MODE_MOCK && (
          <span className="hidden rounded-md border border-dashed border-border px-2 py-1 text-[11px] text-muted-foreground 2xl:inline">
            Data demo
          </span>
        )}
        <RoleSwitcher />

        {can(user, "ews.lihat") && (
          <Link
            href="/ews"
            onClick={bacaSemua}
            className="relative grid size-8 place-items-center rounded-md hover:bg-secondary"
            aria-label={`Peringatan dini, ${unread} belum dibaca`}
          >
            <Bell className="size-[18px]" strokeWidth={1.75} />
            <AnimatePresence>
              {unread > 0 && (
                <motion.span
                  variants={popIn}
                  initial="hidden"
                  animate="show"
                  exit="exit"
                  className="absolute top-1 right-1"
                >
                  <PulseRing tone="defisit" rings={1}>
                    <span className="num relative grid h-4 min-w-4 place-items-center rounded-full bg-defisit px-1 text-[10px] font-semibold text-white">
                      {unread}
                    </span>
                  </PulseRing>
                </motion.span>
              )}
            </AnimatePresence>
            <span
              className={cn(
                "absolute bottom-1.5 left-1.5 size-1.5 rounded-full",
                stream === "terputus"
                  ? "bg-mist-400"
                  : "animate-live bg-normal",
              )}
              title={`Aliran EWS: ${stream}`}
            />
          </Link>
        )}

        <DropdownMenu>
          <DropdownMenuTrigger
            className="grid size-8 place-items-center rounded-full bg-navy-800 text-[11px] font-semibold text-white outline-none focus-visible:ring-2 focus-visible:ring-ring/50 dark:bg-white dark:text-navy-900"
            aria-label="Menu akun"
          >
            {inisialNama(user.nama)}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64 rounded-lg p-1.5">
            <DropdownMenuLabel>
              <p className="font-medium">{user.nama}</p>
              <p className="text-xs font-normal text-muted-foreground">
                {ROLES[user.role].label}
              </p>
              <p className="text-xs font-normal text-muted-foreground">
                {user.lembaga}
              </p>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            {itemMenuProfil(MODE_MOCK).map(({ tab, label, ikon: Ikon }) => (
              <DropdownMenuItem key={tab} asChild className="rounded-md">
                <Link href={hrefTabProfil(tab)}>
                  <Ikon className="size-4" /> {label}
                </Link>
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() =>
                setTheme(resolvedTheme === "dark" ? "light" : "dark")
              }
              className="rounded-md"
            >
              {resolvedTheme === "dark" ? (
                <Sun className="size-4" />
              ) : (
                <Moon className="size-4" />
              )}
              {resolvedTheme === "dark" ? "Mode terang" : "Mode gelap"}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => void keluarDariSesi()}
              className="rounded-md"
            >
              <LogOut className="size-4" /> Keluar
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent
          side="left"
          className="flex w-[280px] flex-col gap-0 bg-sidebar p-0"
        >
          <SheetTitle className="flex h-14 items-center gap-2.5 px-4 text-[13px] font-semibold">
            <BrandMark />{" "}
            <span>Garda Inflasi</span>
          </SheetTitle>
          <NavList
            layoutId="mobile-nav-active"
            onNavigate={() => setMobileOpen(false)}
          />
        </SheetContent>
      </Sheet>
    </header>
  );
}
