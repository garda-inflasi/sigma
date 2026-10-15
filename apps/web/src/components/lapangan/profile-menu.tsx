"use client";

import { ChevronDown, LayoutDashboard, LogOut, Moon, Sun, UserCog } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useSession } from "@/lib/auth/session-context";
import { DEV_ROLE_COOKIE } from "@/lib/auth/resolve";
import { LAPIS_META, ROLES, ROLE_IDS, type Lapis, type Role } from "@/lib/rbac/roles";

/** Menu profil untuk halaman lapangan — di-dynamic import dengan ssr:false agar tidak memicu hydration mismatch. */
export default function ProfileMenu({ user }: { user: ReturnType<typeof useSession> }) {
  const router = useRouter();
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => setMounted(true), []);

  function gantiRole(r: string) {
    document.cookie = `${DEV_ROLE_COOKIE}=${r}; path=/; max-age=2592000; samesite=lax`;
    router.push(ROLES[r as Role].beranda);
    router.refresh();
  }

  const perLapis = (Object.keys(LAPIS_META) as Lapis[]).map((l) => ({
    l,
    roles: ROLE_IDS.filter((r) => ROLES[r].lapis === l),
  }));

  const inisial = user.nama
    .split(" ")
    .filter((w) => /^[A-Z]/.test(w))
    .slice(0, 2)
    .map((w) => w[0])
    .join("");

  // Server & initial client render: placeholder statis tanpa Radix UI
  // Setelah mount: ganti dengan DropdownMenu interaktif
  if (!mounted) {
    return (
      <div
        className="flex h-9 w-[42px] shrink-0 items-center justify-center rounded-xl bg-navy-800 text-xs font-semibold text-white dark:bg-data-600"
        aria-label="Menu akun"
      >
        {inisial}
      </div>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="flex h-9 shrink-0 items-center gap-1.5 rounded-xl bg-navy-800 pl-2 pr-2.5 text-xs font-semibold text-white dark:bg-data-600"
        aria-label="Menu akun"
      >
        {inisial}
        <ChevronDown className="size-3 opacity-70" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64 rounded-xl p-1.5">
        <DropdownMenuLabel>
          <p className="font-medium">{user.nama}</p>
          <p className="text-xs font-normal text-muted-foreground">{ROLES[user.role].label}</p>
          <p className="text-xs font-normal text-muted-foreground">{user.lembaga}</p>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />

        {/* Buka konsol — link ke halaman web */}
        <DropdownMenuItem asChild className="rounded-md">
          <Link href={ROLES[user.role].beranda}>
            <LayoutDashboard className="size-4" /> Buka konsol
          </Link>
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
          className="rounded-md"
        >
          {resolvedTheme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
          {resolvedTheme === "dark" ? "Mode terang" : "Mode gelap"}
        </DropdownMenuItem>

        <DropdownMenuItem asChild className="rounded-md">
          <Link href="/masuk">
            <LogOut className="size-4" /> Keluar
          </Link>
        </DropdownMenuItem>

        {process.env.NODE_ENV !== "production" && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
              <UserCog className="size-3.5" />
              Ganti peran (dev)
            </DropdownMenuLabel>
            <DropdownMenuRadioGroup value={user.role} onValueChange={gantiRole}>
              {perLapis.map(({ l, roles }, i) => (
                <div key={l}>
                  {i > 0 && <DropdownMenuSeparator />}
                  <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
                    {LAPIS_META[l].label}
                  </DropdownMenuLabel>
                  {roles.map((r) => (
                    <DropdownMenuRadioItem key={r} value={r} className="rounded-lg">
                      <div>
                        <p className="text-sm">{ROLES[r].label}</p>
                        <p className="text-xs text-muted-foreground">{ROLES[r].lembaga}</p>
                      </div>
                    </DropdownMenuRadioItem>
                  ))}
                </div>
              ))}
            </DropdownMenuRadioGroup>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
