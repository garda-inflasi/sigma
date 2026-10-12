"use client";

import { motion } from "motion/react";
import { ClipboardList, History, LayoutDashboard, PenLine } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { BrandMark } from "@/components/layout/brand-mark";
import { useSession } from "@/lib/auth/session-context";
import { namaWilayah } from "@/lib/data/generators";
import { spring } from "@/lib/motion/presets";
import { useSyncEngine } from "@/lib/offline/sync-engine";
import { can } from "@/lib/rbac/permissions";
import { ROLES } from "@/lib/rbac/roles";
import { pendingCount, useFieldQueue } from "@/lib/stores/field-queue-store";
import { cn } from "@/lib/utils";
import dynamic from "next/dynamic";
import { OfflineBanner, OfflineIndicator } from "./offline-indicator";

const ProfileMenu = dynamic(() => import("./profile-menu"), { ssr: false });

const NAV = [
  { href: "/lapangan/input", label: "Input harga", icon: PenLine },
  { href: "/lapangan/riwayat", label: "Riwayat", icon: History, badge: "pending" as const },
  { href: "/lapangan/tugas", label: "Tugas", icon: ClipboardList },
];

/** Cangkang PWA tim lapangan: offline-first, satu tangan, target sentuh ≥44px. */
export function FieldShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const user = useSession();
  const items = useFieldQueue((s) => s.items);
  useSyncEngine();

  useEffect(() => {
    useFieldQueue.persist.rehydrate();
    if (process.env.NODE_ENV === "production" && "serviceWorker" in navigator) {
      navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {});
    }
  }, []);

  const pending = pendingCount(items);

  return (
    <div className="mx-auto flex min-h-dvh max-w-md flex-col">
      <header className="sticky top-0 z-30 px-4 pt-[max(env(safe-area-inset-top),0.75rem)]">
        <div className="glass flex h-14 items-center gap-3 rounded-xl px-3">
          <BrandMark className="size-8 shrink-0" />
          <div className="min-w-0 flex-1 leading-tight max-sm:hidden">
            <p className="truncate text-sm font-semibold">Halo, {user.nama.split(" ")[0]}</p>
            <p className="truncate text-xs text-muted-foreground">
              {user.kabupatenKode ? namaWilayah(user.kabupatenKode) : "Seluruh Aceh"}
            </p>
          </div>
          <OfflineIndicator />
          {can(user.role, "dashboard.operasional") && (
            <Link
              href="/operasional"
              className="grid size-9 place-items-center rounded-xl hover:bg-secondary"
              aria-label="Buka konsol"
            >
              <LayoutDashboard className="size-4" />
            </Link>
          )}
          <ProfileMenu user={user} />
        </div>
      </header>
      <OfflineBanner />
      <main id="konten" className="flex-1 px-4 pt-4 pb-44">
        {children}
      </main>

      <nav
        className="fixed inset-x-0 bottom-0 z-40 mx-auto max-w-md px-4 pb-safe"
        aria-label="Navigasi lapangan"
      >
        <div className="glass-strong grid grid-cols-3 gap-1 rounded-xl p-1.5">
          {NAV.map((n) => {
            const active = pathname.startsWith(n.href);
            return (
              <Link
                key={n.href}
                href={n.href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex h-14 flex-col items-center justify-center gap-0.5 rounded-xl text-[11px] font-medium",
                  active ? "text-white" : "text-muted-foreground",
                )}
              >
                {active && (
                  <motion.span
                    layoutId="lapangan-nav"
                    transition={spring.layout}
                    className="absolute inset-0 rounded-xl bg-navy-800 shadow-ambient dark:bg-data-600"
                  />
                )}
                <n.icon className="relative size-5" />
                <span className="relative">{n.label}</span>
                {n.badge === "pending" && pending > 0 && (
                  <span className="num absolute top-1.5 right-[calc(50%-22px)] grid h-4 min-w-4 place-items-center rounded-full bg-waspada px-1 text-[10px] font-semibold text-white">
                    {pending}
                  </span>
                )}
              </Link>
            );
          })}
        </div>
      </nav>
    </div>
  );
}
